package services

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestAccessCodePrintUsesSamePayloadForQRAndCode128(t *testing.T) {
	qrCode, barCode, err := encodeAccessCodeBarcodes("001")
	if err != nil {
		t.Fatal(err)
	}
	if qrCode.Content() != "001" || barCode.Content() != "001" {
		t.Fatalf("payloads = QR %q Code128 %q", qrCode.Content(), barCode.Content())
	}
}

func TestAccessCodePrintSavesRepeatableA4SheetWithoutAllocatingCodes(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	codes := NewAccessCodeService(db)
	batch, err := codes.GenerateBatch(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	statsBefore, err := codes.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	printer := NewAccessCodePrintService(db, t.TempDir())

	path, err := printer.SaveBatch(ctx, batch.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	html := string(first)
	if strings.Count(html, `class="label"`) != 2 {
		t.Fatalf("label count = %d", strings.Count(html, `class="label"`))
	}
	for _, fragment := range []string{"@page{size:A4", "grid-template-columns:repeat(3", "001", "002", "ARDORA", "data:image/png;base64,"} {
		if !strings.Contains(html, fragment) {
			t.Fatalf("sheet missing %q", fragment)
		}
	}

	secondPath, err := printer.SaveBatch(ctx, batch.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	if path != secondPath || string(first) != string(second) {
		t.Fatal("reprint changed the saved label sheet")
	}
	statsAfter, err := codes.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if statsAfter.Total != statsBefore.Total || statsAfter.NextNumber != statsBefore.NextNumber {
		t.Fatalf("printing changed code allocation: before=%+v after=%+v", statsBefore, statsAfter)
	}
	batches, err := codes.ListBatches(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || batches[0].LastPrintedAt == nil {
		t.Fatalf("printed batch metadata = %+v", batches)
	}
}

func TestAccessCodePrintReturnsSavedPathWhenOpeningFails(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	batch, err := NewAccessCodeService(db).GenerateBatch(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	printer := NewAccessCodePrintService(db, t.TempDir())
	printer.opener = func(string) error { return errors.New("cancelled") }

	path, err := printer.OpenBatch(ctx, batch.ID, true)
	if err == nil || path == "" {
		t.Fatalf("OpenBatch() = %q, %v; want saved path and error", path, err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("saved sheet unavailable after open failure: %v", statErr)
	}
}
