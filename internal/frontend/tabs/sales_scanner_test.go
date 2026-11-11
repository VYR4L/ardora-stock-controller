package tabs

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ardora/stockcontroller/internal/backend/models"
)

func TestSalesScannerValidatesCheckInWithoutOpeningVisit(t *testing.T) {
	updates := make(chan struct{}, 1)
	var opens atomic.Int32
	tab := &SalesTab{
		clientCard: " 7\r\n",
		actions: salesActions{
			resolveAvailable: func(context.Context, string) (*models.AccessCode, error) {
				return &models.AccessCode{ID: 7, Value: "007", State: models.AccessCodeAvailable}, nil
			},
			openVisit: func(context.Context, string, *string, string) (*models.Visit, error) {
				opens.Add(1)
				return nil, nil
			},
		},
		update: func(fn func()) { fn(); updates <- struct{}{} },
	}

	tab.validateRegistrationCode()
	<-updates
	if tab.clientCard != "007" || !tab.registerOK || tab.registerMsg != "Comanda 007 disponível." {
		t.Fatalf("validated state = code %q ok=%t message=%q", tab.clientCard, tab.registerOK, tab.registerMsg)
	}
	if opens.Load() != 0 {
		t.Fatalf("scanner opened %d visits before confirmation", opens.Load())
	}
}

func TestSalesAvailableCodeSelectionUsesScannerValidation(t *testing.T) {
	updates := make(chan struct{}, 1)
	code := models.AccessCode{ID: 7, Value: "007", State: models.AccessCodeAvailable}
	tab := &SalesTab{
		availableCodes: []models.AccessCode{code},
		availableCode:  availableCodeOption(code),
		actions: salesActions{
			resolveAvailable: func(_ context.Context, raw string) (*models.AccessCode, error) {
				if raw != "007" {
					t.Fatalf("selected code validation received %q", raw)
				}
				return &code, nil
			},
		},
		update: func(fn func()) { fn(); updates <- struct{}{} },
	}

	tab.selectAvailableCode()
	select {
	case <-updates:
	case <-time.After(time.Second):
		t.Fatal("selected code did not trigger validation")
	}
	if tab.clientCard != "007" || !tab.registerOK || tab.registerMsg != "Comanda 007 disponível." {
		t.Fatalf("selected code state = code %q ok=%t message=%q", tab.clientCard, tab.registerOK, tab.registerMsg)
	}
}

func TestSalesScannerRegisterButtonAssignsValidatedCodeOnce(t *testing.T) {
	updates := make(chan struct{}, 1)
	var opens atomic.Int32
	tab := &SalesTab{
		clientName: "Ana",
		clientCard: "001",
		actions: salesActions{
			openVisit: func(_ context.Context, name string, document *string, code string) (*models.Visit, error) {
				opens.Add(1)
				if name != "Ana" || document != nil || code != "001" {
					t.Fatalf("open visit arguments = %q, %v, %q", name, document, code)
				}
				return &models.Visit{ID: 1, AccessCodeValue: "001", EntryPriceCents: 2500}, nil
			},
		},
		update: func(fn func()) { fn(); updates <- struct{}{} },
	}

	tab.registerClient()
	<-updates
	if opens.Load() != 1 || tab.clientCard != "" || tab.clientName != "" {
		t.Fatalf("after register opens=%d code=%q name=%q", opens.Load(), tab.clientCard, tab.clientName)
	}
}

func TestSalesScannerBillsOnlyOneInFlightSubmission(t *testing.T) {
	product := models.Product{ID: 9, Name: "Café", PriceCents: 700, Stock: 4, Active: true}
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	updated := make(chan struct{}, 1)
	var calls atomic.Int32
	tab := &SalesTab{
		products:    []models.Product{product},
		billProduct: productOption(product),
		billQty:     "1",
		actions: salesActions{
			billProductByCode: func(context.Context, string, int64, int) (*models.Visit, error) {
				calls.Add(1)
				started <- struct{}{}
				<-release
				return &models.Visit{ID: 3, AccessCodeValue: "001"}, nil
			},
		},
		update: func(fn func()) { fn(); updated <- struct{}{} },
	}

	tab.billScannedCode()
	if calls.Load() != 0 || tab.billMsg == "" {
		t.Fatalf("missing-code submission calls=%d message=%q", calls.Load(), tab.billMsg)
	}
	tab.billCode = "001"
	tab.billProduct = "Selecione..."
	tab.billScannedCode()
	if calls.Load() != 0 {
		t.Fatalf("missing-product submission billed %d times", calls.Load())
	}
	tab.billProduct = productOption(product)
	tab.billQty = "0"
	tab.billScannedCode()
	if calls.Load() != 0 {
		t.Fatalf("non-positive quantity submission billed %d times", calls.Load())
	}
	tab.billQty = "1"
	tab.billScannedCode()
	<-started
	tab.billScannedCode()
	close(release)
	<-updated
	if calls.Load() != 1 {
		t.Fatalf("in-flight scans billed %d times", calls.Load())
	}
	if tab.billCode != "" || tab.billProduct != productOption(product) {
		t.Fatalf("successful scan code=%q product=%q", tab.billCode, tab.billProduct)
	}
}

func TestSalesScannerCheckoutLookupOpensPreviewWithoutClosing(t *testing.T) {
	updates := make(chan struct{}, 1)
	var lookups atomic.Int32
	tab := &SalesTab{
		checkoutCode: "1",
		actions: salesActions{
			activeForCode: func(context.Context, string) (*models.Visit, error) {
				lookups.Add(1)
				return &models.Visit{ID: 44, ClientName: "Ana", AccessCodeValue: "001"}, nil
			},
		},
		update: func(fn func()) { fn(); updates <- struct{}{} },
	}

	tab.locateCheckoutCode()
	<-updates
	if lookups.Load() != 1 || !tab.showCheckout || tab.checkoutVisitID != 44 {
		t.Fatalf("checkout lookup count=%d show=%t visit=%d", lookups.Load(), tab.showCheckout, tab.checkoutVisitID)
	}
	if tab.receipt != nil || tab.showReceipt {
		t.Fatal("lookup closed the visit or opened a receipt")
	}
}
