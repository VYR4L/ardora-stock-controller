package tabs

import (
	"testing"

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
