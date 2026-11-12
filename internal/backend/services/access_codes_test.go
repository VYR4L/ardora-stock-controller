package services

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/ardora/stockcontroller/internal/backend/database"
	"github.com/ardora/stockcontroller/internal/backend/models"
)

func TestAccessCodeNormalization(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "trim and pad", raw: " 7\r\n", want: "007"},
		{name: "preserve zero", raw: "000", want: "000"},
		{name: "reject negative", raw: "-1", wantErr: true},
		{name: "reject overflow", raw: "1000", wantErr: true},
		{name: "reject letters", raw: "ABC", wantErr: true},
		{name: "reject empty", raw: " \r\n", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeAccessCode(test.raw)
			if test.wantErr {
				if err == nil {
					t.Fatalf("NormalizeAccessCode(%q) = %q, want error", test.raw, got)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("NormalizeAccessCode(%q) = %q, %v; want %q", test.raw, got, err, test.want)
			}
		})
	}
}

func TestAccessCodeBatchGenerationContinuesAndValidates(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	service := NewAccessCodeService(db)

	first, err := service.GenerateBatch(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if first.FirstNumber != 1 || first.LastNumber != 2 || first.Quantity != 2 || first.Legacy {
		t.Fatalf("first batch = %+v", first)
	}
	second, err := service.GenerateBatch(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if second.FirstNumber != 3 || second.LastNumber != 4 {
		t.Fatalf("second batch = %+v", second)
	}
	codes, err := service.ListBatchCodes(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != 2 || codes[0].Value != "001" || codes[1].Value != "002" {
		t.Fatalf("first batch codes = %+v", codes)
	}

	if _, err := service.GenerateBatch(ctx, 0); err == nil {
		t.Fatal("expected zero quantity error")
	}
	var batchCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM access_code_batches").Scan(&batchCount); err != nil {
		t.Fatal(err)
	}
	if batchCount != 2 {
		t.Fatalf("invalid generation changed batch count to %d", batchCount)
	}
}

func TestAccessCodeBatchGenerationStopsAtLimitAtomically(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	now := "2026-11-11 10:00:00"
	result, err := db.Exec(`INSERT INTO access_code_batches(first_number, last_number, quantity, created_at) VALUES(998, 998, 1, ?)`, now)
	if err != nil {
		t.Fatal(err)
	}
	batchID, _ := result.LastInsertId()
	if _, err := db.Exec(`INSERT INTO access_codes(batch_id, number, value, active, created_at) VALUES(?, 998, '998', 1, ?)`, batchID, now); err != nil {
		t.Fatal(err)
	}

	service := NewAccessCodeService(db)
	last, err := service.GenerateBatch(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if last.FirstNumber != 999 || last.LastNumber != 999 {
		t.Fatalf("last batch = %+v", last)
	}
	var batchesBefore, codesBefore int
	if err := db.QueryRow("SELECT COUNT(*) FROM access_code_batches").Scan(&batchesBefore); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM access_codes").Scan(&codesBefore); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GenerateBatch(ctx, 2); err == nil {
		t.Fatal("expected capacity error")
	}
	var batchesAfter, codesAfter int
	db.QueryRow("SELECT COUNT(*) FROM access_code_batches").Scan(&batchesAfter)
	db.QueryRow("SELECT COUNT(*) FROM access_codes").Scan(&codesAfter)
	if batchesAfter != batchesBefore || codesAfter != codesBefore {
		t.Fatalf("overflow changed rows: batches %d→%d codes %d→%d", batchesBefore, batchesAfter, codesBefore, codesAfter)
	}
}

func TestAccessCodeConcurrentGenerationUsesUniqueContiguousRanges(t *testing.T) {
	db := testDB(t)
	service := NewAccessCodeService(db)
	ctx := context.Background()

	type outcome struct {
		batchFirst int
		batchLast  int
		err        error
	}
	start := make(chan struct{})
	outcomes := make(chan outcome, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for _, quantity := range []int{2, 3} {
		quantity := quantity
		go func() {
			ready.Done()
			<-start
			batch, err := service.GenerateBatch(ctx, quantity)
			if err != nil {
				outcomes <- outcome{err: err}
				return
			}
			outcomes <- outcome{batchFirst: batch.FirstNumber, batchLast: batch.LastNumber}
		}()
	}
	ready.Wait()
	close(start)

	got := []outcome{<-outcomes, <-outcomes}
	for _, item := range got {
		if item.err != nil {
			t.Fatal(item.err)
		}
	}
	sort.Slice(got, func(i, j int) bool { return got[i].batchFirst < got[j].batchFirst })
	if got[0].batchFirst != 1 || got[0].batchLast+1 != got[1].batchFirst || got[1].batchLast != 5 {
		t.Fatalf("generated ranges = %d-%d and %d-%d", got[0].batchFirst, got[0].batchLast, got[1].batchFirst, got[1].batchLast)
	}
	rows, err := db.Query("SELECT value FROM access_codes ORDER BY number")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			t.Fatal(err)
		}
		values = append(values, value)
	}
	want := []string{"001", "002", "003", "004", "005"}
	if fmt.Sprint(values) != fmt.Sprint(want) {
		t.Fatalf("values = %v, want %v", values, want)
	}
}

func TestAccessCodeLifecycleDerivesStateAndReleasesAfterCheckout(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	service := NewAccessCodeService(db)
	batch, err := service.GenerateBatch(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	codes, err := service.ListBatchCodes(ctx, batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().Format(database.TimeLayout)
	clientResult, err := db.Exec(`INSERT INTO clients(name, active_card, checked_in_at, created_at, updated_at) VALUES('Ana', 1, ?, ?, ?)`, stamp, stamp, stamp)
	if err != nil {
		t.Fatal(err)
	}
	clientID, _ := clientResult.LastInsertId()
	visitResult, err := db.Exec(`INSERT INTO visits(client_id, client_name, card_number, access_code_id, opened_at) VALUES(?, 'Ana', 1, ?, ?)`, clientID, codes[0].ID, stamp)
	if err != nil {
		t.Fatal(err)
	}
	visitID, _ := visitResult.LastInsertId()

	inUse, err := service.Resolve(ctx, "1")
	if err != nil {
		t.Fatal(err)
	}
	if inUse.Value != "001" || inUse.State != models.AccessCodeInUse {
		t.Fatalf("resolved in-use code = %+v", inUse)
	}
	if _, err := service.ResolveAvailable(ctx, "001"); err == nil {
		t.Fatal("expected in-use code to be unavailable")
	}
	if err := service.Archive(ctx, inUse.ID); err == nil {
		t.Fatal("expected archiving in-use code to fail")
	}

	if _, err := NewVisitService(db).Checkout(ctx, visitID, 0, models.PaymentCash); err != nil {
		t.Fatal(err)
	}
	released, err := service.ResolveAvailable(ctx, "001")
	if err != nil {
		t.Fatal(err)
	}
	if released.State != models.AccessCodeAvailable {
		t.Fatalf("released code state = %q", released.State)
	}

	if err := service.Archive(ctx, codes[1].ID); err != nil {
		t.Fatal(err)
	}
	archived, err := service.Resolve(ctx, "002")
	if err != nil {
		t.Fatal(err)
	}
	if archived.State != models.AccessCodeArchived || archived.Active {
		t.Fatalf("archived code = %+v", archived)
	}
	if _, err := service.ResolveAvailable(ctx, "002"); err == nil {
		t.Fatal("expected archived code to be unavailable")
	}
	if err := service.Reactivate(ctx, archived.ID); err != nil {
		t.Fatal(err)
	}
	if reactivated, err := service.ResolveAvailable(ctx, "2"); err != nil || reactivated.State != models.AccessCodeAvailable {
		t.Fatalf("reactivated code = %+v, %v", reactivated, err)
	}
}

func TestAccessCodeListsSearchAndStatsUseCanonicalValues(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	service := NewAccessCodeService(db)
	if _, err := service.GenerateBatch(ctx, 7); err != nil {
		t.Fatal(err)
	}

	batches, err := service.ListBatches(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || batches[0].FirstNumber != 1 || batches[0].LastNumber != 7 {
		t.Fatalf("batches = %+v", batches)
	}
	for _, query := range []string{"7", "007"} {
		codes, err := service.ListCodes(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		if len(codes) != 1 || codes[0].Value != "007" {
			t.Fatalf("ListCodes(%q) = %+v", query, codes)
		}
	}
	stats, err := service.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Total != 7 || stats.Available != 7 || stats.InUse != 0 || stats.Archived != 0 || stats.NextNumber != 8 {
		t.Fatalf("stats = %+v", stats)
	}
	if _, err := service.Resolve(ctx, "999"); err != sql.ErrNoRows {
		t.Fatalf("unknown code error = %v, want sql.ErrNoRows", err)
	}
}
