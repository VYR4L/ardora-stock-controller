package services

import (
	"context"
	"encoding/csv"
	"os"
	"testing"
	"time"

	"github.com/ardora/stockcontroller/internal/backend/database"
)

func TestBackupIncludesVisitReceiptColumnsAndLegacyRows(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	generateAccessCodes(t, db, 6)
	visit, err := NewVisitService(db).OpenVisit(ctx, "Ana", nil, "006")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Format(database.TimeLayout)
	if _, err := db.Exec(`INSERT INTO history(kind, description, quantity, unit_price_cents, total_cents, created_at) VALUES('sale', 'Legado', 1, 100, 100, ?)`, now); err != nil {
		t.Fatal(err)
	}
	path, err := NewBackupService(db, t.TempDir()).GenerateCSV(ctx, time.Now().Format(database.DayLayout))
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 || records[0][0] != "Visit ID" || records[0][1] != "Receipt" {
		t.Fatalf("unexpected backup records: %#v", records)
	}
	if records[1][0] != "1" || records[2][0] != "" {
		t.Fatalf("visit and legacy identifiers not preserved: visit=%q legacy=%q (visit %d)", records[1][0], records[2][0], visit.ID)
	}
}
