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

// TableService handles CRUD for tables.
type TableService struct {
	db *sql.DB
}

func NewTableService(db *sql.DB) *TableService {
	return &TableService{db: db}
}

func (s *TableService) List(ctx context.Context, onlyActive bool) ([]models.Table, error) {
	query := "SELECT id, name, game_id, active, created_at FROM tables"
	if onlyActive {
		query += " WHERE active = 1"
	}
	query += " ORDER BY id ASC"

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()

	var list []models.Table
	for rows.Next() {
		var t models.Table
		var active int
		var created string
		if err := rows.Scan(&t.ID, &t.Name, &t.GameID, &active, &created); err != nil {
			return nil, fmt.Errorf("scan table: %w", err)
		}
		t.Active = active == 1
		t.CreatedAt, _ = time.ParseInLocation(database.TimeLayout, created, time.Local)
		list = append(list, t)
	}
	return list, rows.Err()
}

func (s *TableService) Create(ctx context.Context, t *models.Table) error {
	name, err := validateTable(t)
	if err != nil {
		return err
	}
	now := time.Now().Format(database.TimeLayout)
	active := 0
	if t.Active {
		active = 1
	}

	res, err := s.db.ExecContext(ctx,
		"INSERT INTO tables (name, game_id, active, created_at) VALUES (?, ?, ?, ?)",
		name, t.GameID, active, now,
	)
	if err != nil {
		return fmt.Errorf("create table: %w", err)
	}

	id, err := res.LastInsertId()
	if err == nil {
		t.Name = name
		t.ID = id
		t.CreatedAt, _ = time.ParseInLocation(database.TimeLayout, now, time.Local)
	}
	return nil
}

func (s *TableService) Update(ctx context.Context, t *models.Table) error {
	name, err := validateTable(t)
	if err != nil {
		return err
	}
	active := 0
	if t.Active {
		active = 1
	}

	result, err := s.db.ExecContext(ctx,
		"UPDATE tables SET name = ?, game_id = ?, active = ? WHERE id = ?",
		name, t.GameID, active, t.ID,
	)
	if err != nil {
		return fmt.Errorf("update table: %w", err)
	}
	if err := requireOneRow(result, "table"); err != nil {
		return err
	}
	t.Name = name
	return nil
}

// PlanRemoval reports whether a table can be deleted, archived, or is in use.
func (s *TableService) PlanRemoval(ctx context.Context, id int64) (models.RemovalPlan, error) {
	return planTableRemoval(ctx, s.db, id)
}

// Remove recomputes and applies the safe table removal policy atomically.
func (s *TableService) Remove(ctx context.Context, id int64) (models.RemovalPlan, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return models.RemovalPlan{}, err
	}
	defer tx.Rollback()
	plan, err := planTableRemoval(ctx, tx, id)
	if err != nil {
		return models.RemovalPlan{}, err
	}
	if plan.Kind == models.RemovalBlocked {
		return plan, fmt.Errorf("%s", plan.Message)
	}
	var result sql.Result
	switch plan.Kind {
	case models.RemovalDelete:
		result, err = tx.ExecContext(ctx, "DELETE FROM tables WHERE id = ?", id)
	case models.RemovalArchive:
		result, err = tx.ExecContext(ctx, "UPDATE tables SET active = 0 WHERE id = ?", id)
	}
	if err != nil {
		return plan, fmt.Errorf("remove table: %w", err)
	}
	if err := requireOneRow(result, "table"); err != nil {
		return plan, err
	}
	if err := tx.Commit(); err != nil {
		return plan, err
	}
	return plan, nil
}

func validateTable(t *models.Table) (string, error) {
	name := strings.TrimSpace(t.Name)
	if name == "" {
		return "", fmt.Errorf("table name is required")
	}
	return name, nil
}

func planTableRemoval(ctx context.Context, query rowQuerier, id int64) (models.RemovalPlan, error) {
	var exists, hasGame int
	if err := query.QueryRowContext(ctx, "SELECT COUNT(*), COUNT(game_id) FROM tables WHERE id = ?", id).Scan(&exists, &hasGame); err != nil {
		return models.RemovalPlan{}, err
	}
	if exists != 1 {
		return models.RemovalPlan{}, fmt.Errorf("table not found")
	}
	var openSeats, openGames int
	if err := query.QueryRowContext(ctx, "SELECT COUNT(*) FROM tables_clients WHERE table_id = ? AND left_at IS NULL", id).Scan(&openSeats); err != nil {
		return models.RemovalPlan{}, err
	}
	if err := query.QueryRowContext(ctx, "SELECT COUNT(*) FROM table_game_sessions WHERE table_id = ? AND ended_at IS NULL", id).Scan(&openGames); err != nil {
		return models.RemovalPlan{}, err
	}
	if openSeats > 0 || openGames > 0 || hasGame > 0 {
		return models.RemovalPlan{Kind: models.RemovalBlocked, Message: "A mesa está ocupada ou possui um jogo em andamento. Encerre o atendimento antes de removê-la."}, nil
	}
	var references int
	if err := query.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM history WHERE table_id = ?) +
		(SELECT COUNT(*) FROM tables_clients WHERE table_id = ?) +
		(SELECT COUNT(*) FROM table_game_sessions WHERE table_id = ?)`, id, id, id).Scan(&references); err != nil {
		return models.RemovalPlan{}, err
	}
	if references > 0 {
		return models.RemovalPlan{Kind: models.RemovalArchive, Message: "Esta mesa possui histórico e será arquivada."}, nil
	}
	return models.RemovalPlan{Kind: models.RemovalDelete, Message: "Esta mesa nunca foi utilizada e será excluída definitivamente."}, nil
}
