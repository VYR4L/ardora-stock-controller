package tabs

import (
	"context"
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/ardora/stockcontroller/internal/frontend"
	"github.com/ardora/stockcontroller/internal/frontend/components"
	"github.com/ardora/stockcontroller/internal/frontend/icons"
)

type HomeTab struct {
	state *frontend.State

	// Example dashboard metrics
	totalProducts int
	totalGames    int
	loading       bool
	err           error
}

func NewHomeTab(s *frontend.State) *HomeTab {
	t := &HomeTab{state: s}
	t.loadStats()
	return t
}

func (t *HomeTab) loadStats() {
	t.loading = true
	go func() {
		products, productsErr := t.state.ProductService.List(context.Background(), false)
		games, gamesErr := t.state.GameService.List(context.Background(), false)

		t.state.Window.Update(func() {
			t.totalProducts = len(products)
			t.totalGames = len(games)
			switch {
			case productsErr != nil:
				t.err = fmt.Errorf("carregar produtos: %w", productsErr)
			case gamesErr != nil:
				t.err = fmt.Errorf("carregar jogos: %w", gamesErr)
			default:
				t.err = nil
			}
			t.loading = false
		})
	}()
}

func (t *HomeTab) View(c *ui.Context) {
	components.Page(c, func() {
		components.PageHeader(c, "VISÃO GERAL", "Olá! Vamos organizar a casa?", "Acompanhe o catálogo e acesse rapidamente as rotinas da luderia.", nil)

		if t.loading {
			components.LoadingState(c, "Montando seu resumo", "Consultando produtos e jogos cadastrados.")
			return
		}
		if t.err != nil {
			components.ErrorState(c, "Não foi possível carregar o resumo", t.err.Error(), &components.Action{Label: "Tentar novamente", Run: t.loadStats})
			return
		}

		ui.Row(c).Gap(16).Wrap().Children(func() {
			components.MetricCard(c, "Produtos cadastrados", t.totalProducts, "Itens disponíveis no controle de estoque", icons.Stock, &components.Action{Label: "Abrir estoque", Run: func() { t.navigate("Stock") }})
			components.MetricCard(c, "Jogos cadastrados", t.totalGames, "Títulos disponíveis no catálogo", icons.Games, &components.Action{Label: "Abrir jogos", Run: func() { t.navigate("Games") }})
		})

		components.Panel(c, "Atalhos do dia", "Acesse as rotinas mais usadas pela equipe.", 0, func() {
			ui.Row(c).Gap(12).Wrap().Children(func() {
				if ui.PrimaryButton(c, "Registrar venda").Clicked() {
					t.navigate("Sales")
				}
				if ui.Button(c, "Ver mesas").Clicked() {
					t.navigate("Tables")
				}
				if ui.Button(c, "Configurações").Clicked() {
					t.navigate("Settings")
				}
			})
		})
	})
}

func (t *HomeTab) navigate(destination string) {
	if t.state != nil && t.state.Navigate != nil {
		t.state.Navigate(destination)
	}
}
