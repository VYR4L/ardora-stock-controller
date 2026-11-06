package tabs

import (
	"strings"

	"github.com/ardora/stockcontroller/internal/backend/models"
)

type stockLevel uint8

const (
	stockHealthy stockLevel = iota
	stockLow
	stockDepleted
)

func productStockLevel(product models.Product) stockLevel {
	if product.Stock <= 0 {
		return stockDepleted
	}
	if product.Stock <= product.MinStock {
		return stockLow
	}
	return stockHealthy
}

func filterProducts(products []models.Product, query string) []models.Product {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return products
	}
	filtered := make([]models.Product, 0, len(products))
	for _, product := range products {
		if strings.Contains(strings.ToLower(product.Name), query) {
			filtered = append(filtered, product)
		}
	}
	return filtered
}

func filterGames(games []models.Game, query string) []models.Game {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return games
	}
	filtered := make([]models.Game, 0, len(games))
	for _, game := range games {
		if strings.Contains(strings.ToLower(game.Name), query) {
			filtered = append(filtered, game)
		}
	}
	return filtered
}
