package services

import (
	"context"
	"database/sql"
	"fmt"
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
	now := time.Now().Format(database.TimeLayout)
	active := 0
	if g.Active {
		active = 1
	}

	res, err := s.db.ExecContext(ctx,
		"INSERT INTO games (name, min_players, max_players, notes, image, active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		g.Name, g.MinPlayers, g.MaxPlayers, g.Notes, g.Image, active, now, now,
	)
	if err != nil {
		return fmt.Errorf("create game: %w", err)
	}

	id, err := res.LastInsertId()
	if err == nil {
		g.ID = id
		g.CreatedAt, _ = time.ParseInLocation(database.TimeLayout, now, time.Local)
		g.UpdatedAt = g.CreatedAt
	}
	return nil
}

func (s *GameService) Update(ctx context.Context, g *models.Game) error {
	now := time.Now().Format(database.TimeLayout)
	active := 0
	if g.Active {
		active = 1
	}

	_, err := s.db.ExecContext(ctx,
		"UPDATE games SET name = ?, min_players = ?, max_players = ?, notes = ?, image = ?, active = ?, updated_at = ? WHERE id = ?",
		g.Name, g.MinPlayers, g.MaxPlayers, g.Notes, g.Image, active, now, g.ID,
	)
	if err != nil {
		return fmt.Errorf("update game: %w", err)
	}
	g.UpdatedAt, _ = time.ParseInLocation(database.TimeLayout, now, time.Local)
	return nil
}
