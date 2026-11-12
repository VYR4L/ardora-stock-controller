package tabs

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ardora/stockcontroller/internal/backend/models"
)

func TestProductStockLevel(t *testing.T) {
	tests := []struct {
		name    string
		product models.Product
		want    stockLevel
	}{
		{name: "depleted at zero", product: models.Product{Stock: 0, MinStock: 2}, want: stockDepleted},
		{name: "depleted below zero", product: models.Product{Stock: -1, MinStock: 2}, want: stockDepleted},
		{name: "low at minimum", product: models.Product{Stock: 2, MinStock: 2}, want: stockLow},
		{name: "low below minimum", product: models.Product{Stock: 1, MinStock: 2}, want: stockLow},
		{name: "healthy above minimum", product: models.Product{Stock: 3, MinStock: 2}, want: stockHealthy},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := productStockLevel(test.product); got != test.want {
				t.Fatalf("productStockLevel() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestProductEditPreservesIdentityAndUsesUpdate(t *testing.T) {
	updates := make(chan struct{}, 1)
	var creates, saves atomic.Int32
	product := models.Product{ID: 42, Name: "Café", Category: "Bebidas", Image: "cafe.png", PriceCents: 900, Active: false}
	tab := &StockTab{
		actions: stockActions{
			create: func(context.Context, *models.Product) error { creates.Add(1); return nil },
			update: func(_ context.Context, item *models.Product, expectedStock int) error {
				saves.Add(1)
				if item.ID != 42 || item.Image != "cafe.png" || !item.Active || expectedStock != product.Stock {
					t.Fatalf("updated product = %+v", item)
				}
				return nil
			},
		},
		update: func(fn func()) { fn(); updates <- struct{}{} },
	}
	tab.openEdit(product)
	if tab.formItem.ID != 42 || tab.formItem.Image != "cafe.png" || tab.formImage != "" {
		t.Fatalf("edit form lost persisted data: %+v image=%q", tab.formItem, tab.formImage)
	}
	tab.formItem.Active = true
	tab.saveForm()
	select {
	case <-updates:
	case <-time.After(time.Second):
		t.Fatal("product update did not finish")
	}
	if creates.Load() != 0 || saves.Load() != 1 {
		t.Fatalf("product persistence create=%d update=%d", creates.Load(), saves.Load())
	}
}

func TestGameEditPreservesIdentityAndUsesUpdate(t *testing.T) {
	updates := make(chan struct{}, 1)
	var creates, saves atomic.Int32
	game := models.Game{ID: 17, Name: "Catan", Image: "catan.png", MinPlayers: 3, MaxPlayers: 4, Active: false}
	tab := &GamesTab{
		actions: gameActions{
			create: func(context.Context, *models.Game) error { creates.Add(1); return nil },
			update: func(_ context.Context, item *models.Game) error {
				saves.Add(1)
				if item.ID != 17 || item.Image != "catan.png" || !item.Active {
					t.Fatalf("updated game = %+v", item)
				}
				return nil
			},
		},
		update: func(fn func()) { fn(); updates <- struct{}{} },
	}
	tab.openEdit(game)
	if tab.formGame.ID != 17 || tab.formGame.Image != "catan.png" || tab.formImage != "" {
		t.Fatalf("edit form lost persisted data: %+v image=%q", tab.formGame, tab.formImage)
	}
	tab.formGame.Active = true
	tab.saveForm()
	select {
	case <-updates:
	case <-time.After(time.Second):
		t.Fatal("game update did not finish")
	}
	if creates.Load() != 0 || saves.Load() != 1 {
		t.Fatalf("game persistence create=%d update=%d", creates.Load(), saves.Load())
	}
}

func TestCatalogRemovalConfirmationCopy(t *testing.T) {
	deleteCopy := removalConfirmation(models.RemovalPlan{Kind: models.RemovalDelete})
	if deleteCopy.ConfirmLabel != "Excluir definitivamente" || !deleteCopy.Danger {
		t.Fatalf("delete confirmation = %+v", deleteCopy)
	}
	archiveCopy := removalConfirmation(models.RemovalPlan{Kind: models.RemovalArchive})
	if archiveCopy.ConfirmLabel != "Arquivar" || archiveCopy.Danger {
		t.Fatalf("archive confirmation = %+v", archiveCopy)
	}
}

func TestTableRemovalBlockedShowsMessageWithoutConfirmation(t *testing.T) {
	updates := make(chan struct{}, 1)
	plan := models.RemovalPlan{Kind: models.RemovalBlocked, Message: "Mesa ocupada."}
	tab := &TablesTab{
		actions: tableActions{
			planRemoval: func(context.Context, int64) (models.RemovalPlan, error) { return plan, nil },
		},
		update: func(fn func()) { fn(); updates <- struct{}{} },
	}
	table := models.Table{ID: 9, Name: "Mesa 9", Active: true}
	tab.requestTableRemoval(table)
	select {
	case <-updates:
	case <-time.After(time.Second):
		t.Fatal("table removal plan did not finish")
	}
	if tab.showRemove || tab.pageMsg != "Mesa ocupada." {
		t.Fatalf("blocked state show=%t message=%q", tab.showRemove, tab.pageMsg)
	}
}

func TestTableEditPreservesIdentityAndUsesUpdate(t *testing.T) {
	updates := make(chan struct{}, 1)
	var saves atomic.Int32
	gameID := int64(5)
	tab := &TablesTab{
		actions: tableActions{update: func(_ context.Context, table *models.Table) error {
			saves.Add(1)
			if table.ID != 9 || table.GameID == nil || *table.GameID != gameID || !table.Active {
				t.Fatalf("updated table = %+v", table)
			}
			return nil
		}},
		update: func(fn func()) { fn(); updates <- struct{}{} },
	}
	table := models.Table{ID: 9, Name: "Mesa 9", GameID: &gameID, Active: false}
	tab.openTableEdit(table)
	if tab.editTable.ID != 9 || tab.editTable.GameID == nil || *tab.editTable.GameID != gameID {
		t.Fatalf("edit form lost table identity: %+v", tab.editTable)
	}
	tab.editTable.Active = true
	tab.saveTableEdit()
	select {
	case <-updates:
	case <-time.After(time.Second):
		t.Fatal("table update did not finish")
	}
	if saves.Load() != 1 {
		t.Fatalf("table update calls = %d", saves.Load())
	}
}

func TestTableRemovalRaceBlockedClosesConfirmation(t *testing.T) {
	updates := make(chan struct{}, 1)
	plan := models.RemovalPlan{Kind: models.RemovalBlocked, Message: "A mesa passou a estar ocupada."}
	tab := &TablesTab{
		showRemove:   true,
		removeTarget: models.Table{ID: 9, Name: "Mesa 9"},
		actions: tableActions{remove: func(context.Context, int64) (models.RemovalPlan, error) {
			return plan, errors.New(plan.Message)
		}},
		update: func(fn func()) { fn(); updates <- struct{}{} },
	}
	tab.confirmTableRemoval()
	select {
	case <-updates:
	case <-time.After(time.Second):
		t.Fatal("table removal did not finish")
	}
	if tab.showRemove || tab.pageMsg != plan.Message {
		t.Fatalf("raced blocked state show=%t message=%q", tab.showRemove, tab.pageMsg)
	}
}

func TestCatalogFilters(t *testing.T) {
	products := []models.Product{{Name: "Café Especial"}, {Name: "Refrigerante"}, {Name: "Água com gás"}}
	games := []models.Game{{Name: "Catan"}, {Name: "Código Secreto"}, {Name: "Azul"}}

	productTests := []struct {
		name  string
		query string
		want  int
	}{
		{name: "empty returns all", query: "", want: 3},
		{name: "case insensitive", query: "CAFÉ", want: 1},
		{name: "unicode case", query: "ÁGUA", want: 1},
		{name: "trimmed", query: "  refrigerante ", want: 1},
		{name: "no match", query: "suco", want: 0},
	}
	for _, test := range productTests {
		t.Run("products/"+test.name, func(t *testing.T) {
			if got := len(filterProducts(products, test.query)); got != test.want {
				t.Fatalf("filterProducts() returned %d items, want %d", got, test.want)
			}
		})
	}

	gameTests := []struct {
		name  string
		query string
		want  int
	}{
		{name: "empty returns all", query: "", want: 3},
		{name: "case insensitive", query: "CATAN", want: 1},
		{name: "unicode case", query: "CÓDIGO", want: 1},
		{name: "no match", query: "xadrez", want: 0},
	}
	for _, test := range gameTests {
		t.Run("games/"+test.name, func(t *testing.T) {
			if got := len(filterGames(games, test.query)); got != test.want {
				t.Fatalf("filterGames() returned %d items, want %d", got, test.want)
			}
		})
	}
}
