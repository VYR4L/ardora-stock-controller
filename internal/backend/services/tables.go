package services

import (
	"context"
	"database/sql"
	"fmt"
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
	now := time.Now().Format(database.TimeLayout)
	active := 0
	if t.Active {
		active = 1
	}

	res, err := s.db.ExecContext(ctx,
		"INSERT INTO tables (name, game_id, active, created_at) VALUES (?, ?, ?, ?)",
		t.Name, t.GameID, active, now,
	)
	if err != nil {
		return fmt.Errorf("create table: %w", err)
	}

	id, err := res.LastInsertId()
	if err == nil {
		t.ID = id
		t.CreatedAt, _ = time.ParseInLocation(database.TimeLayout, now, time.Local)
	}
	return nil
}

func (s *TableService) Update(ctx context.Context, t *models.Table) error {
	active := 0
	if t.Active {
		active = 1
	}

	_, err := s.db.ExecContext(ctx,
		"UPDATE tables SET name = ?, game_id = ?, active = ? WHERE id = ?",
		t.Name, t.GameID, active, t.ID,
	)
	if err != nil {
		return fmt.Errorf("update table: %w", err)
	}
	return nil
}
