package services

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/ardora/stockcontroller/internal/backend/database"
	"github.com/ardora/stockcontroller/internal/backend/models"
)

// TableGameService tracks the game and active customers at each table.
type TableGameService struct {
	db *sql.DB
}

func NewTableGameService(db *sql.DB) *TableGameService {
	return &TableGameService{db: db}
}

// ListOverviews returns tables with seated active customers and current games.
func (s *TableGameService) ListOverviews(ctx context.Context, onlyActive bool) ([]models.TableOverview, error) {
	tables, err := NewTableService(s.db).List(ctx, onlyActive)
	if err != nil {
		return nil, err
	}
	result := make([]models.TableOverview, 0, len(tables))
	for _, table := range tables {
		overview := models.TableOverview{Table: table}
		session, err := s.Current(ctx, table.ID)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		if err == nil {
			overview.CurrentGame = session
		}
		clients, err := s.SeatedClients(ctx, table.ID)
		if err != nil {
			return nil, err
		}
		overview.Clients = clients
		result = append(result, overview)
	}
	return result, nil
}

// SeatedClients returns customers at the table who still have an active visit.
func (s *TableGameService) SeatedClients(ctx context.Context, tableID int64) ([]models.Client, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.document, c.active_card, c.checked_in_at, c.created_at, c.updated_at
		FROM tables_clients tc
		JOIN clients c ON c.id = tc.client_id
		JOIN visits v ON v.client_id = c.id AND v.closed_at IS NULL
		WHERE tc.table_id = ? AND tc.left_at IS NULL
		ORDER BY tc.joined_at ASC`, tableID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var clients []models.Client
	for rows.Next() {
		client, err := scanClient(rows)
		if err != nil {
			return nil, err
		}
		clients = append(clients, *client)
	}
	return clients, rows.Err()
}

// Current returns the current game session for a table.
func (s *TableGameService) Current(ctx context.Context, tableID int64) (*models.TableGameSession, error) {
	return scanGameSession(s.db.QueryRowContext(ctx, tableSessionSelect+" WHERE s.table_id = ? AND s.ended_at IS NULL", tableID))
}

// History returns newest-first game sessions for a table.
func (s *TableGameService) History(ctx context.Context, tableID int64) ([]models.TableGameSession, error) {
	rows, err := s.db.QueryContext(ctx, tableSessionSelect+" WHERE s.table_id = ? ORDER BY s.started_at DESC, s.id DESC", tableID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []models.TableGameSession
	for rows.Next() {
		session, err := scanGameSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *session)
	}
	return sessions, rows.Err()
}

// StartCatalog starts a session using an active catalog game.
func (s *TableGameService) StartCatalog(ctx context.Context, tableID, gameID int64) error {
	return s.start(ctx, tableID, &gameID, "", nil)
}

// StartBrought starts a session for a game brought by a customer.
func (s *TableGameService) StartBrought(ctx context.Context, tableID int64, name string, ownerID *int64) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("brought game name is required")
	}
	return s.start(ctx, tableID, nil, name, ownerID)
}

func (s *TableGameService) start(ctx context.Context, tableID int64, gameID *int64, broughtName string, ownerID *int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var tableActive int
	if err := tx.QueryRowContext(ctx, "SELECT active FROM tables WHERE id = ?", tableID).Scan(&tableActive); err != nil {
		return fmt.Errorf("get table: %w", err)
	}
	if tableActive != 1 {
		return fmt.Errorf("table is inactive")
	}
	var occupantCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM tables_clients tc
		JOIN visits v ON v.client_id = tc.client_id AND v.closed_at IS NULL
		WHERE tc.table_id = ? AND tc.left_at IS NULL`, tableID).Scan(&occupantCount); err != nil {
		return err
	}
	if occupantCount == 0 {
		return fmt.Errorf("seat an active customer before starting a game")
	}
	if gameID != nil {
		var active int
		if err := tx.QueryRowContext(ctx, "SELECT active FROM games WHERE id = ?", *gameID).Scan(&active); err != nil {
			return fmt.Errorf("get game: %w", err)
		}
		if active != 1 {
			return fmt.Errorf("game is inactive")
		}
	}
	if ownerID != nil {
		var seated int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM tables_clients tc
			JOIN visits v ON v.client_id = tc.client_id AND v.closed_at IS NULL
			WHERE tc.table_id = ? AND tc.client_id = ? AND tc.left_at IS NULL`, tableID, *ownerID).Scan(&seated); err != nil {
			return err
		}
		if seated == 0 {
			return fmt.Errorf("the selected owner is not seated at this table")
		}
	}

	now := time.Now().Format(database.TimeLayout)
	if _, err := tx.ExecContext(ctx, "UPDATE table_game_sessions SET ended_at = ? WHERE table_id = ? AND ended_at IS NULL", now, tableID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO table_game_sessions (table_id, game_id, brought_game_name, brought_by_client_id, started_at)
		VALUES (?, ?, ?, ?, ?)`, tableID, gameID, broughtName, ownerID, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE tables SET game_id = ? WHERE id = ?", gameID, tableID); err != nil {
		return err
	}
	return tx.Commit()
}

// Stop ends the current game while preserving its history.
func (s *TableGameService) Stop(ctx context.Context, tableID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().Format(database.TimeLayout)
	result, err := tx.ExecContext(ctx, "UPDATE table_game_sessions SET ended_at = ? WHERE table_id = ? AND ended_at IS NULL", now, tableID)
	if err != nil {
		return err
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return fmt.Errorf("table has no active game")
	}
	if _, err := tx.ExecContext(ctx, "UPDATE tables SET game_id = NULL WHERE id = ?", tableID); err != nil {
		return err
	}
	return tx.Commit()
}

const tableSessionSelect = `SELECT s.id, s.table_id, s.game_id, COALESCE(g.name, ''),
	s.brought_game_name, s.brought_by_client_id, COALESCE(c.name, ''), s.started_at, s.ended_at
	FROM table_game_sessions s
	LEFT JOIN games g ON g.id = s.game_id
	LEFT JOIN clients c ON c.id = s.brought_by_client_id`

func scanGameSession(row scanner) (*models.TableGameSession, error) {
	var session models.TableGameSession
	var gameID, ownerID sql.NullInt64
	var started string
	var ended sql.NullString
	if err := row.Scan(&session.ID, &session.TableID, &gameID, &session.GameName,
		&session.BroughtGameName, &ownerID, &session.BroughtByName, &started, &ended); err != nil {
		return nil, err
	}
	if gameID.Valid {
		value := gameID.Int64
		session.GameID = &value
	}
	if ownerID.Valid {
		value := ownerID.Int64
		session.BroughtByClientID = &value
	}
	session.StartedAt, _ = time.ParseInLocation(database.TimeLayout, started, time.Local)
	if ended.Valid && ended.String != "" {
		value, _ := time.ParseInLocation(database.TimeLayout, ended.String, time.Local)
		session.EndedAt = &value
	}
	return &session, nil
}

func scanClient(row scanner) (*models.Client, error) {
	var client models.Client
	var document, checked sql.NullString
	var card sql.NullInt64
	var created, updated string
	if err := row.Scan(&client.ID, &client.Name, &document, &card, &checked, &created, &updated); err != nil {
		return nil, err
	}
	if document.Valid {
		value := document.String
		client.Document = &value
	}
	if card.Valid {
		value := int(card.Int64)
		client.ActiveCard = &value
	}
	if checked.Valid && checked.String != "" {
		value, _ := time.ParseInLocation(database.TimeLayout, checked.String, time.Local)
		client.CheckedInAt = &value
	}
	client.CreatedAt, _ = time.ParseInLocation(database.TimeLayout, created, time.Local)
	client.UpdatedAt, _ = time.ParseInLocation(database.TimeLayout, updated, time.Local)
	return &client, nil
}
