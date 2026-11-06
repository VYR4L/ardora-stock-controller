package services

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ardora/stockcontroller/internal/backend/database"
	"github.com/ardora/stockcontroller/internal/backend/models"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func createProduct(t *testing.T, db *sql.DB, name string, price int64, stock int) int64 {
	t.Helper()
	now := time.Now().Format(database.TimeLayout)
	result, err := db.Exec(`INSERT INTO products(name, category, price_cents, stock, active, created_at, updated_at) VALUES(?, 'Bebidas', ?, ?, 1, ?, ?)`, name, price, stock, now, now)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := result.LastInsertId()
	return id
}

func TestVisitSnapshotsEntryAndConsumption(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	config := NewConfigService(db)
	if err := config.SetEntryPriceCents(ctx, 2500); err != nil {
		t.Fatal(err)
	}
	visits := NewVisitService(db)
	visit, err := visits.OpenVisit(ctx, "Ana", nil, 7)
	if err != nil {
		t.Fatal(err)
	}
	if visit.EntryPriceCents != 2500 {
		t.Fatalf("entry = %d", visit.EntryPriceCents)
	}
	if err := config.SetEntryPriceCents(ctx, 3000); err != nil {
		t.Fatal(err)
	}
	productID := createProduct(t, db, "Café <forte>", 900, 2)
	if err := NewSalesService(db).BillProduct(ctx, visit.ClientID, productID, 2); err != nil {
		t.Fatal(err)
	}
	preview, err := visits.Preview(ctx, visit.ID, 500)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Visit.EntryPriceCents != 2500 || preview.Visit.ConsumptionSubtotalCents != 1800 || preview.Visit.FinalTotalCents != 3800 {
		t.Fatalf("unexpected preview: %+v", preview.Visit)
	}
	if len(preview.Lines) != 1 || preview.Lines[0].Description != "Café <forte>" {
		t.Fatalf("unexpected lines: %+v", preview.Lines)
	}
	if _, err := visits.OpenVisit(ctx, "Ana", nil, 8); err == nil {
		t.Fatal("expected duplicate active visit error")
	}
	if _, err := visits.OpenVisit(ctx, "Bia", nil, 7); err == nil {
		t.Fatal("expected card in use error")
	}
}

func TestBillingRequiresVisitAndAvailableStock(t *testing.T) {
	db := testDB(t)
	productID := createProduct(t, db, "Suco", 700, 1)
	if err := NewSalesService(db).BillProduct(context.Background(), 999, productID, 1); err == nil {
		t.Fatal("expected missing visit error")
	}
	visit, err := NewVisitService(db).OpenVisit(context.Background(), "Ana", nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewSalesService(db).BillProduct(context.Background(), visit.ClientID, productID, 2); err == nil {
		t.Fatal("expected stock error")
	}
	var stock int
	if err := db.QueryRow("SELECT stock FROM products WHERE id=?", productID).Scan(&stock); err != nil {
		t.Fatal(err)
	}
	if stock != 1 {
		t.Fatalf("stock changed to %d", stock)
	}
}

func TestCheckoutIsAtomicIdempotentAndHistoricallyStable(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if err := NewConfigService(db).SetEntryPriceCents(ctx, 1200); err != nil {
		t.Fatal(err)
	}
	visits := NewVisitService(db)
	visit, err := visits.OpenVisit(ctx, "Ana", nil, 3)
	if err != nil {
		t.Fatal(err)
	}
	closed, err := visits.Checkout(ctx, visit.ID, 200, models.PaymentPix)
	if err != nil {
		t.Fatal(err)
	}
	if closed.Visit.FinalTotalCents != 1000 || closed.Visit.ReceiptIdentifier != "REC-000001" {
		t.Fatalf("unexpected checkout: %+v", closed.Visit)
	}
	again, err := visits.Checkout(ctx, visit.ID, 0, models.PaymentCash)
	if err != nil {
		t.Fatal(err)
	}
	if again.Visit.FinalTotalCents != 1000 || again.Visit.PaymentMethod != models.PaymentPix {
		t.Fatalf("idempotency failed: %+v", again.Visit)
	}
	if err := NewConfigService(db).SetEntryPriceCents(ctx, 9900); err != nil {
		t.Fatal(err)
	}
	preserved, err := visits.Preview(ctx, visit.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if preserved.Visit.EntryPriceCents != 1200 || preserved.Visit.DiscountCents != 200 {
		t.Fatalf("history changed: %+v", preserved.Visit)
	}
}

func TestDiscountBoundsAndEmptyConsumptionCheckout(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if err := NewConfigService(db).SetEntryPriceCents(ctx, 1000); err != nil {
		t.Fatal(err)
	}
	visit, err := NewVisitService(db).OpenVisit(ctx, "Ana", nil, 9)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewVisitService(db).Preview(ctx, visit.ID, -1); err == nil {
		t.Fatal("expected negative discount error")
	}
	if _, err := NewVisitService(db).Checkout(ctx, visit.ID, 1001, models.PaymentCash); err == nil {
		t.Fatal("expected excessive discount error")
	}
	closed, err := NewVisitService(db).Checkout(ctx, visit.ID, 1000, models.PaymentCash)
	if err != nil {
		t.Fatal(err)
	}
	if len(closed.Lines) != 0 || closed.Visit.ConsumptionSubtotalCents != 0 || closed.Visit.FinalTotalCents != 0 {
		t.Fatalf("unexpected empty-consumption checkout: %+v", closed)
	}
}

func TestCheckoutRollsBackOnPersistenceFailure(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	visits := NewVisitService(db)
	visit, err := visits.OpenVisit(ctx, "Ana", nil, 4)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_client_checkout BEFORE UPDATE OF active_card ON clients BEGIN SELECT RAISE(ABORT, 'forced'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := visits.Checkout(ctx, visit.ID, 0, models.PaymentCash); err == nil {
		t.Fatal("expected checkout failure")
	}
	active, err := visits.Get(ctx, visit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if active.ClosedAt != nil {
		t.Fatal("visit closed despite rollback")
	}
	var card sql.NullInt64
	if err := db.QueryRow("SELECT active_card FROM clients WHERE id=?", visit.ClientID).Scan(&card); err != nil {
		t.Fatal(err)
	}
	if !card.Valid || card.Int64 != 4 {
		t.Fatalf("card not preserved: %+v", card)
	}
}

func TestReceiptEscapesContentAndPrintFailureKeepsVisitClosed(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	visit, err := NewVisitService(db).OpenVisit(ctx, "<Ana>", nil, 5)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewVisitService(db).Checkout(ctx, visit.ID, 0, models.PaymentDebit); err != nil {
		t.Fatal(err)
	}
	receipts := NewReceiptService(db, t.TempDir())
	path, err := receipts.Save(ctx, visit.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "<Ana>") || !strings.Contains(string(content), "&lt;Ana&gt;") {
		t.Fatal("receipt did not escape customer name")
	}
	secondPath, err := receipts.Save(ctx, visit.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	secondContent, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != string(secondContent) {
		t.Fatal("reprint changed preserved receipt")
	}
	receipts.opener = func(string) error { return errors.New("cancelled") }
	printPath, err := receipts.Open(ctx, visit.ID, true)
	if err == nil || printPath == "" {
		t.Fatal("expected retryable print error and saved path")
	}
	stored, err := NewVisitService(db).Get(ctx, visit.ID)
	if err != nil || stored.ClosedAt == nil {
		t.Fatal("print failure changed visit state")
	}
}
