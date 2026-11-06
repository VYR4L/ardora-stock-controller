package database

import (
	"context"
	"database/sql"
	"net/url"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestFreshDatabaseAppliesVisitSchema(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != len(migrations) {
		t.Fatalf("version = %d, want %d", version, len(migrations))
	}
	for _, table := range []string{"visits", "table_game_sessions"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("table %s missing", table)
		}
	}
}

func TestUpgradeBackfillsActiveVisitsAndGames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade.db")
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	db, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(migrations[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version = 1"); err != nil {
		t.Fatal(err)
	}
	stamp := "2026-10-12 10:00:00"
	clientResult, err := db.Exec(`INSERT INTO clients(name, active_card, checked_in_at, created_at, updated_at) VALUES('Ana', 7, ?, ?, ?)`, stamp, stamp, stamp)
	if err != nil {
		t.Fatal(err)
	}
	clientID, _ := clientResult.LastInsertId()
	gameResult, err := db.Exec(`INSERT INTO games(name, active, created_at, updated_at) VALUES('Catan', 1, ?, ?)`, stamp, stamp)
	if err != nil {
		t.Fatal(err)
	}
	gameID, _ := gameResult.LastInsertId()
	tableResult, err := db.Exec(`INSERT INTO tables(name, game_id, active, created_at) VALUES('Mesa 1', ?, 1, ?)`, gameID, stamp)
	if err != nil {
		t.Fatal(err)
	}
	tableID, _ := tableResult.LastInsertId()
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var gotClient int64
	var entry int64
	if err := db.QueryRow("SELECT client_id, entry_price_cents FROM visits WHERE closed_at IS NULL").Scan(&gotClient, &entry); err != nil {
		t.Fatal(err)
	}
	if gotClient != clientID || entry != 0 {
		t.Fatalf("visit backfill = client %d entry %d", gotClient, entry)
	}
	var gotTable, gotGame int64
	if err := db.QueryRow("SELECT table_id, game_id FROM table_game_sessions WHERE ended_at IS NULL").Scan(&gotTable, &gotGame); err != nil {
		t.Fatal(err)
	}
	if gotTable != tableID || gotGame != gameID {
		t.Fatalf("game backfill = table %d game %d", gotTable, gotGame)
	}
}
