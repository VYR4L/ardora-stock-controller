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

func TestFreshDatabaseAppliesAccessCodeSchema(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "fresh-codes.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, table := range []string{"access_code_batches", "access_codes"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("table %s missing", table)
		}
	}

	var accessCodeColumn int
	rows, err := db.Query("PRAGMA table_info(visits)")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		if name == "access_code_id" {
			accessCodeColumn++
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if accessCodeColumn != 1 {
		t.Fatalf("visits.access_code_id columns = %d, want 1", accessCodeColumn)
	}
}

func TestUpgradeBackfillsLegacyAccessCodes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-codes.db")
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	db, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(migrations[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(migrations[1]); err != nil {
		t.Fatal(err)
	}

	stamp := "2026-11-11 10:00:00"
	anaResult, err := db.Exec(`INSERT INTO clients(name, active_card, checked_in_at, created_at, updated_at) VALUES('Ana', 7, ?, ?, ?)`, stamp, stamp, stamp)
	if err != nil {
		t.Fatal(err)
	}
	anaID, _ := anaResult.LastInsertId()
	biaResult, err := db.Exec(`INSERT INTO clients(name, created_at, updated_at) VALUES('Bia', ?, ?)`, stamp, stamp)
	if err != nil {
		t.Fatal(err)
	}
	biaID, _ := biaResult.LastInsertId()
	carlaResult, err := db.Exec(`INSERT INTO clients(name, created_at, updated_at) VALUES('Carla', ?, ?)`, stamp, stamp)
	if err != nil {
		t.Fatal(err)
	}
	carlaID, _ := carlaResult.LastInsertId()

	if _, err := db.Exec(`INSERT INTO visits(client_id, client_name, card_number, opened_at) VALUES(?, 'Ana', 7, ?)`, anaID, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO visits(client_id, client_name, card_number, opened_at, closed_at) VALUES(?, 'Ana', 7, ?, ?)`, anaID, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO visits(client_id, client_name, card_number, opened_at, closed_at) VALUES(?, 'Bia', 8, ?, ?)`, biaID, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version = 2"); err != nil {
		t.Fatal(err)
	}

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}

	var batchFirst, batchLast, batchQuantity, legacy int
	if err := db.QueryRow(`SELECT first_number, last_number, quantity, legacy FROM access_code_batches`).Scan(&batchFirst, &batchLast, &batchQuantity, &legacy); err != nil {
		t.Fatal(err)
	}
	if batchFirst != 7 || batchLast != 8 || batchQuantity != 2 || legacy != 1 {
		t.Fatalf("legacy batch = %d-%d quantity %d legacy %d", batchFirst, batchLast, batchQuantity, legacy)
	}

	var code7ID int64
	var code7Value string
	var code7InUse int
	if err := db.QueryRow(`
		SELECT ac.id, ac.value, EXISTS(
			SELECT 1 FROM visits v WHERE v.access_code_id = ac.id AND v.closed_at IS NULL
		)
		FROM access_codes ac WHERE ac.number = 7`).Scan(&code7ID, &code7Value, &code7InUse); err != nil {
		t.Fatal(err)
	}
	if code7Value != "007" || code7InUse != 1 {
		t.Fatalf("code 7 = value %q in_use %d", code7Value, code7InUse)
	}

	var code8Value string
	var code8InUse int
	if err := db.QueryRow(`
		SELECT ac.value, EXISTS(
			SELECT 1 FROM visits v WHERE v.access_code_id = ac.id AND v.closed_at IS NULL
		)
		FROM access_codes ac WHERE ac.number = 8`).Scan(&code8Value, &code8InUse); err != nil {
		t.Fatal(err)
	}
	if code8Value != "008" || code8InUse != 0 {
		t.Fatalf("code 8 = value %q in_use %d", code8Value, code8InUse)
	}

	rows, err := db.Query(`SELECT access_code_id FROM visits WHERE card_number = 7 ORDER BY closed_at IS NULL, id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var linked []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		linked = append(linked, id)
	}
	if len(linked) != 2 || linked[0] != code7ID || linked[1] != code7ID {
		t.Fatalf("card 7 visit links = %v, want [%d %d]", linked, code7ID, code7ID)
	}

	if _, err := db.Exec(`INSERT INTO visits(client_id, client_name, card_number, access_code_id, opened_at) VALUES(?, 'Carla', 7, ?, ?)`, carlaID, code7ID, stamp); err == nil {
		t.Fatal("expected unique open-code constraint")
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
