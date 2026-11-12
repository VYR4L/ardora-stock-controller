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

// GameService handles CRUD for games.
type GameService struct {
	db *sql.DB
}

func NewGameService(db *sql.DB) *GameService {
	return &GameService{db: db}
}

func (s *GameService) List(ctx context.Context, onlyActive bool) ([]models.Game, error) {
	query := "SELECT id, name, min_players, max_players, notes, image, active, created_at, updated_at FROM games"
	if onlyActive {
		query += " WHERE active = 1"
	}
	query += " ORDER BY name ASC"

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list games: %w", err)
	}
	defer rows.Close()

	var list []models.Game
	for rows.Next() {
		var g models.Game
		var active int
		var created, updated string
		if err := rows.Scan(&g.ID, &g.Name, &g.MinPlayers, &g.MaxPlayers, &g.Notes, &g.Image, &active, &created, &updated); err != nil {
			return nil, fmt.Errorf("scan game: %w", err)
		}
		g.Active = active == 1
		g.CreatedAt, _ = time.ParseInLocation(database.TimeLayout, created, time.Local)
		g.UpdatedAt, _ = time.ParseInLocation(database.TimeLayout, updated, time.Local)
		list = append(list, g)
	}
	return list, rows.Err()
}

func (s *GameService) Create(ctx context.Context, g *models.Game) error {
	name, err := validateGame(g)
	if err != nil {
		return err
	}
	now := time.Now().Format(database.TimeLayout)
	active := 0
	if g.Active {
		active = 1
	}

	res, err := s.db.ExecContext(ctx,
		"INSERT INTO games (name, min_players, max_players, notes, image, active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		name, g.MinPlayers, g.MaxPlayers, g.Notes, g.Image, active, now, now,
	)
	if err != nil {
		return fmt.Errorf("create game: %w", err)
	}

	id, err := res.LastInsertId()
	if err == nil {
		g.Name = name
		g.ID = id
		g.CreatedAt, _ = time.ParseInLocation(database.TimeLayout, now, time.Local)
		g.UpdatedAt = g.CreatedAt
	}
	return nil
}

func (s *GameService) Update(ctx context.Context, g *models.Game) error {
	name, err := validateGame(g)
	if err != nil {
		return err
	}
	now := time.Now().Format(database.TimeLayout)
	active := 0
	if g.Active {
		active = 1
	}

	result, err := s.db.ExecContext(ctx,
		"UPDATE games SET name = ?, min_players = ?, max_players = ?, notes = ?, image = ?, active = ?, updated_at = ? WHERE id = ?",
		name, g.MinPlayers, g.MaxPlayers, g.Notes, g.Image, active, now, g.ID,
	)
	if err != nil {
		return fmt.Errorf("update game: %w", err)
	}
	if err := requireOneRow(result, "game"); err != nil {
		return err
	}
	g.Name = name
	g.UpdatedAt, _ = time.ParseInLocation(database.TimeLayout, now, time.Local)
	return nil
}

// PlanRemoval reports whether a game can be deleted or must be archived.
func (s *GameService) PlanRemoval(ctx context.Context, id int64) (models.RemovalPlan, error) {
	return planGameRemoval(ctx, s.db, id)
}

// Remove recomputes and applies the safe game removal policy atomically.
func (s *GameService) Remove(ctx context.Context, id int64) (models.RemovalPlan, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return models.RemovalPlan{}, err
	}
	defer tx.Rollback()
	plan, err := planGameRemoval(ctx, tx, id)
	if err != nil {
		return models.RemovalPlan{}, err
	}
	var result sql.Result
	switch plan.Kind {
	case models.RemovalDelete:
		result, err = tx.ExecContext(ctx, "DELETE FROM games WHERE id = ?", id)
	case models.RemovalArchive:
		result, err = tx.ExecContext(ctx, "UPDATE games SET active = 0, updated_at = ? WHERE id = ?", time.Now().Format(database.TimeLayout), id)
	default:
		return plan, fmt.Errorf("%s", plan.Message)
	}
	if err != nil {
		return plan, fmt.Errorf("remove game: %w", err)
	}
	if err := requireOneRow(result, "game"); err != nil {
		return plan, err
	}
	if err := tx.Commit(); err != nil {
		return plan, err
	}
	return plan, nil
}

func validateGame(g *models.Game) (string, error) {
	name := strings.TrimSpace(g.Name)
	if name == "" {
		return "", fmt.Errorf("game name is required")
	}
	if g.MinPlayers < 1 || g.MaxPlayers < g.MinPlayers {
		return "", fmt.Errorf("game player range is invalid")
	}
	return name, nil
}

func planGameRemoval(ctx context.Context, query rowQuerier, id int64) (models.RemovalPlan, error) {
	var exists int
	if err := query.QueryRowContext(ctx, "SELECT COUNT(*) FROM games WHERE id = ?", id).Scan(&exists); err != nil {
		return models.RemovalPlan{}, err
	}
	if exists != 1 {
		return models.RemovalPlan{}, fmt.Errorf("game not found")
	}
	var references int
	if err := query.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM history WHERE game_id = ?) +
		(SELECT COUNT(*) FROM table_game_sessions WHERE game_id = ?) +
		(SELECT COUNT(*) FROM tables WHERE game_id = ?)`, id, id, id).Scan(&references); err != nil {
		return models.RemovalPlan{}, err
	}
	if references > 0 {
		return models.RemovalPlan{Kind: models.RemovalArchive, Message: "Este jogo possui histórico ou está associado a uma mesa e será arquivado."}, nil
	}
	return models.RemovalPlan{Kind: models.RemovalDelete, Message: "Este jogo nunca foi utilizado e será excluído definitivamente."}, nil
}
