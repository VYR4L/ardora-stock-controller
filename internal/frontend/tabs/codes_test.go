package tabs

import (
	"testing"
	"time"

	"github.com/ardora/stockcontroller/internal/backend/models"
	"github.com/ardora/stockcontroller/internal/frontend/components"
)

func TestCodesBatchQuantityValidation(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		next    int
		want    int
		wantErr bool
	}{
		{name: "valid", raw: "2", next: 1, want: 2},
		{name: "last code fits", raw: "1", next: 999, want: 1},
		{name: "zero", raw: "0", next: 1, wantErr: true},
		{name: "not numeric", raw: "dois", next: 1, wantErr: true},
		{name: "exceeds remaining", raw: "2", next: 999, wantErr: true},
		{name: "capacity exhausted", raw: "1", next: 1000, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseBatchQuantity(test.raw, test.next)
			if test.wantErr {
				if err == nil {
					t.Fatalf("parseBatchQuantity(%q, %d) = %d, want error", test.raw, test.next, got)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("parseBatchQuantity(%q, %d) = %d, %v; want %d", test.raw, test.next, got, err, test.want)
			}
		})
	}
}

func TestCodesSearchStateLabelsAndBatchMapping(t *testing.T) {
	codes := []models.AccessCode{
		{ID: 1, Value: "001", State: models.AccessCodeAvailable},
		{ID: 7, Value: "007", State: models.AccessCodeInUse},
		{ID: 8, Value: "008", State: models.AccessCodeArchived},
	}
	for _, query := range []string{"7", "007"} {
		filtered := filterAccessCodes(codes, query)
		if len(filtered) != 1 || filtered[0].ID != 7 {
			t.Fatalf("filterAccessCodes(%q) = %+v", query, filtered)
		}
	}
	labels := []struct {
		state models.AccessCodeState
		label string
		tone  components.Tone
	}{
		{models.AccessCodeAvailable, "Disponível", components.ToneSuccess},
		{models.AccessCodeInUse, "Em uso", components.ToneAccent},
		{models.AccessCodeArchived, "Arquivado", components.ToneNeutral},
	}
	for _, item := range labels {
		label, tone := accessCodeStatePresentation(item.state)
		if label != item.label || tone != item.tone {
			t.Fatalf("state %q = %q/%v", item.state, label, tone)
		}
	}

	batch := models.AccessCodeBatch{ID: 42, FirstNumber: 7, LastNumber: 12, Quantity: 6, CreatedAt: time.Date(2026, 11, 11, 10, 0, 0, 0, time.Local)}
	tab := &CodesTab{batches: []models.AccessCodeBatch{batch}}
	if got := tab.batchIDForOption(batchOption(batch)); got != 42 {
		t.Fatalf("batch option mapped to %d, want 42", got)
	}
}
