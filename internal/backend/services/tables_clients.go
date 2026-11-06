package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ardora/stockcontroller/internal/backend/database"
)

// TablesClientsService handles assigning clients to tables.
type TablesClientsService struct {
	db *sql.DB
}

func NewTablesClientsService(db *sql.DB) *TablesClientsService {
	return &TablesClientsService{db: db}
}

// AssignClient adds a client to a table.
func (s *TablesClientsService) AssignClient(ctx context.Context, tableID, clientID int64) error {
	now := time.Now().Format(database.TimeLayout)

	var activeVisitID int64
	if err := s.db.QueryRowContext(ctx,
		"SELECT id FROM visits WHERE client_id = ? AND closed_at IS NULL", clientID,
	).Scan(&activeVisitID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("client has no active visit")
		}
		return err
	}

	// Verify client is not already at this table
	var existingID int64
	err := s.db.QueryRowContext(ctx, "SELECT id FROM tables_clients WHERE table_id = ? AND client_id = ? AND left_at IS NULL", tableID, clientID).Scan(&existingID)
	if err == nil {
		return fmt.Errorf("client already at table")
	} else if err != sql.ErrNoRows {
		return err
	}

	_, err = s.db.ExecContext(ctx,
		"INSERT INTO tables_clients (table_id, client_id, joined_at) VALUES (?, ?, ?)",
		tableID, clientID, now,
	)
	return err
}

// RemoveClient marks a client as having left a table.
func (s *TablesClientsService) RemoveClient(ctx context.Context, tableID, clientID int64) error {
	now := time.Now().Format(database.TimeLayout)
	res, err := s.db.ExecContext(ctx,
		"UPDATE tables_clients SET left_at = ? WHERE table_id = ? AND client_id = ? AND left_at IS NULL",
		now, tableID, clientID,
	)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("client not at table")
	}
	return nil
}
