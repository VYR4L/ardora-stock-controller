package services

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ardora/stockcontroller/internal/backend/database"
	"github.com/ardora/stockcontroller/internal/backend/models"
)

func createTableAndGame(t *testing.T, db *sql.DB) (int64, int64) {
	t.Helper()
	now := time.Now().Format(database.TimeLayout)
	gameResult, err := db.Exec(`INSERT INTO games(name, active, created_at, updated_at) VALUES('Catan', 1, ?, ?)`, now, now)
	if err != nil {
		t.Fatal(err)
	}
	gameID, _ := gameResult.LastInsertId()
	tableResult, err := db.Exec(`INSERT INTO tables(name, active, created_at) VALUES('Mesa 1', 1, ?)`, now)
	if err != nil {
		t.Fatal(err)
	}
	tableID, _ := tableResult.LastInsertId()
	return tableID, gameID
}

func TestTableGameCatalogBroughtSwitchAndStop(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	generateAccessCodes(t, db, 8)
	tableID, gameID := createTableAndGame(t, db)
	service := NewTableGameService(db)
	if err := service.StartCatalog(ctx, tableID, gameID); err == nil {
		t.Fatal("expected empty table error")
	}
	visit, err := NewVisitService(db).OpenVisit(ctx, "Ana", nil, "008")
	if err != nil {
		t.Fatal(err)
	}
	if err := NewTablesClientsService(db).AssignClient(ctx, tableID, visit.ClientID); err != nil {
		t.Fatal(err)
	}
	if err := service.StartCatalog(ctx, tableID, gameID); err != nil {
		t.Fatal(err)
	}
	current, err := service.Current(ctx, tableID)
	if err != nil || current.GameID == nil {
		t.Fatalf("catalog session: %+v %v", current, err)
	}
	originalSessionID := current.ID
	if err := NewGameService(db).Update(ctx, &models.Game{ID: gameID, Name: "Catan: Nova edição", MinPlayers: 1, MaxPlayers: 4, Active: true}); err != nil {
		t.Fatal(err)
	}
	current, err = service.Current(ctx, tableID)
	if err != nil || current.ID != originalSessionID || current.GameID == nil || *current.GameID != gameID || current.GameName != "Catan: Nova edição" {
		t.Fatalf("renamed catalog session: %+v %v", current, err)
	}
	if err := service.StartBrought(ctx, tableID, "Azul do João", &visit.ClientID); err != nil {
		t.Fatal(err)
	}
	current, err = service.Current(ctx, tableID)
	if err != nil || current.GameID != nil || current.BroughtGameName != "Azul do João" || current.BroughtByName != "Ana" {
		t.Fatalf("brought session: %+v %v", current, err)
	}
	history, err := service.History(ctx, tableID)
	if err != nil || len(history) != 2 || history[1].EndedAt == nil {
		t.Fatalf("history: %+v %v", history, err)
	}
	if _, err := db.Exec(`INSERT INTO table_game_sessions(table_id, game_id, started_at) VALUES(?, ?, ?)`, tableID, gameID, time.Now().Format(database.TimeLayout)); err == nil {
		t.Fatal("expected unique active session constraint")
	}
	if err := service.Stop(ctx, tableID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Current(ctx, tableID); err != sql.ErrNoRows {
		t.Fatalf("current after stop: %v", err)
	}
}

func TestBroughtGameOwnerMustBeSeated(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	generateAccessCodes(t, db, 2)
	tableID, _ := createTableAndGame(t, db)
	ana, err := NewVisitService(db).OpenVisit(ctx, "Ana", nil, "001")
	if err != nil {
		t.Fatal(err)
	}
	bia, err := NewVisitService(db).OpenVisit(ctx, "Bia", nil, "002")
	if err != nil {
		t.Fatal(err)
	}
	if err := NewTablesClientsService(db).AssignClient(ctx, tableID, ana.ClientID); err != nil {
		t.Fatal(err)
	}
	if err := NewTableGameService(db).StartBrought(ctx, tableID, "Dixit", &bia.ClientID); err == nil {
		t.Fatal("expected invalid owner error")
	}
	if _, err := NewTableGameService(db).Current(ctx, tableID); err != sql.ErrNoRows {
		t.Fatal("invalid start changed table state")
	}
}
