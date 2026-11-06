package tabs

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/ardora/stockcontroller/internal/backend/models"
	"github.com/ardora/stockcontroller/internal/frontend"
	"github.com/ardora/stockcontroller/internal/frontend/style"
)

func TestRepresentativeTabStatesRenderAtSupportedSizes(t *testing.T) {
	state := &frontend.State{Navigate: func(string) {}}
	views := map[string]func(*ui.Context){
		"home":         (&HomeTab{state: state, totalProducts: 18, totalGames: 42}).View,
		"home_loading": (&HomeTab{state: state, loading: true}).View,
		"home_error":   (&HomeTab{state: state, err: errors.New("falha simulada")}).View,
		"stock": (&StockTab{state: state, products: []models.Product{
			{ID: 1, Name: "Café especial", Category: "Bebidas", PriceCents: 900, Stock: 2, MinStock: 3, Active: true},
			{ID: 2, Name: "Refrigerante", Category: "Bebidas", PriceCents: 700, Stock: 0, MinStock: 4, Active: false},
		}}).View,
		"stock_empty": (&StockTab{state: state}).View,
		"stock_error": (&StockTab{state: state, err: errors.New("falha simulada")}).View,
		"stock_modal": (&StockTab{state: state, showModal: true, formItem: models.Product{Active: true, Category: "Outros"}, formMsg: "Informe o nome do produto."}).View,
		"games": (&GamesTab{state: state, games: []models.Game{
			{ID: 1, Name: "Catan", MinPlayers: 3, MaxPlayers: 4, Active: true},
			{ID: 2, Name: "Código Secreto", MinPlayers: 2, MaxPlayers: 8, Active: false},
		}}).View,
		"games_modal": (&GamesTab{state: state, showModal: true, formGame: models.Game{Active: true, MinPlayers: 1, MaxPlayers: 4}, formMsg: "Informe o nome do jogo."}).View,
		"sales":       (&SalesTab{state: state, billQty: "1"}).View,
		"sales_feedback": (&SalesTab{
			state: state, billQty: "1", registerMsg: "Entrada registrada com sucesso.", registerOK: true, billMsg: "Informe um ID de produto válido.",
		}).View,
		"sales_checkout": (&SalesTab{
			state: state, billQty: "1", showCheckout: true, discount: "R$ 5,00", payment: "Pix",
			preview: &models.CheckoutSummary{Visit: models.Visit{ID: 1, ClientName: "Ana", CardNumber: 7, EntryPriceCents: 2500, ConsumptionSubtotalCents: 1800, GrossTotalCents: 4300, DiscountCents: 500, FinalTotalCents: 3800}, Lines: []models.VisitLine{{Description: "Café", Quantity: 2, UnitPriceCents: 900, TotalCents: 1800}}},
		}).View,
		"sales_receipt": (&SalesTab{
			state: state, billQty: "1", showReceipt: true,
			receipt: &models.CheckoutSummary{Visit: models.Visit{ID: 1, ClientName: "Ana", CardNumber: 7, EntryPriceCents: 2500, GrossTotalCents: 2500, FinalTotalCents: 2500, PaymentMethod: models.PaymentPix, ReceiptIdentifier: "REC-000001"}},
		}).View,
		"tables": (&TablesTab{state: state, overviews: []models.TableOverview{
			{Table: models.Table{ID: 1, Name: "Mesa 1", Active: true}, Clients: []models.Client{{ID: 1, Name: "Ana"}}, CurrentGame: &models.TableGameSession{BroughtGameName: "Azul", BroughtByName: "Ana"}},
		}}).View,
		"tables_catalog": (&TablesTab{state: state, overviews: []models.TableOverview{
			{Table: models.Table{ID: 2, Name: "Mesa 2", Active: true}, Clients: []models.Client{{ID: 2, Name: "Bia"}}, CurrentGame: &models.TableGameSession{GameID: int64Pointer(2), GameName: "Catan"}},
		}}).View,
		"tables_brought_modal": (&TablesTab{state: state, showGameModal: true, gameSource: "Jogo trazido pelo cliente", broughtOwner: "Não informado", overviews: []models.TableOverview{
			{Table: models.Table{ID: 1, Name: "Mesa 1", Active: true}, Clients: []models.Client{{ID: 1, Name: "Ana"}}},
		}, gameTableID: 1}).View,
		"tables_empty":   (&TablesTab{state: state}).View,
		"tables_error":   (&TablesTab{state: state, err: errors.New("falha simulada")}).View,
		"settings":       (&SettingsTab{state: state, entryPrice: "R$ 25,00", businessName: "Ardora Luderia"}).View,
		"settings_error": (&SettingsTab{state: state, msg: "O relatório não pôde ser enviado: falha simulada"}).View,
	}

	for name, view := range views {
		for _, dark := range []bool{false, true} {
			for _, size := range [][2]int{{1024, 768}, {800, 600}} {
				t.Run(fmt.Sprintf("%s/dark=%t/%dx%d", name, dark, size[0], size[1]), func(t *testing.T) {
					image := ui.Render(func(c *ui.Context) {
						c.SetTheme(style.NewTheme(dark))
						ui.Scroll(c).Fill().Padding(24).Children(func() { view(c) })
					}, size[0], size[1], 1)
					if image == nil || image.Bounds().Dx() != size[0] || image.Bounds().Dy() != size[1] {
						t.Fatalf("unexpected rendered image bounds: %v", image)
					}
					writeTabPreview(t, fmt.Sprintf("%s_dark=%t_%dx%d", name, dark, size[0], size[1]), image)
				})
			}
		}
	}
}

func TestSalesValidatesDiscountAndPaymentBeforeCheckout(t *testing.T) {
	tab := &SalesTab{discount: "-1", payment: "Pix"}
	tab.confirmCheckout()
	if !strings.Contains(tab.checkoutMsg, "desconto válido") {
		t.Fatalf("unexpected discount message: %q", tab.checkoutMsg)
	}
	tab.discount = "R$ 0,00"
	tab.payment = "Selecione..."
	tab.confirmCheckout()
	if !strings.Contains(tab.checkoutMsg, "forma de pagamento") {
		t.Fatalf("unexpected payment message: %q", tab.checkoutMsg)
	}
}

func TestSalesAndTableSelectionMappings(t *testing.T) {
	visit := models.Visit{ID: 11, ClientID: 22, ClientName: "Ana", CardNumber: 7}
	product := models.Product{ID: 33, Name: "Café", PriceCents: 900, Stock: 2}
	sales := &SalesTab{active: []models.Visit{visit}, products: []models.Product{product}}
	if sales.visitIDForOption(visitOption(visit)) != 11 || sales.productIDForOption(productOption(product)) != 33 {
		t.Fatal("sales selection did not map to persisted identifiers")
	}
	tables := &TablesTab{visits: []models.Visit{visit}, overviews: []models.TableOverview{{Table: models.Table{ID: 44, Name: "Mesa 1"}}}}
	if tables.clientIDForOption(clientVisitOption(visit)) != 22 || tables.tableIDForOption(tableOption(tables.overviews[0].Table)) != 44 {
		t.Fatal("table selection did not map to persisted identifiers")
	}
}

func int64Pointer(value int64) *int64 { return &value }

func TestSalesKeyboardFocusOrder(t *testing.T) {
	tab := &SalesTab{state: &frontend.State{}, billQty: "1"}
	tester := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(style.NewTheme(false))
		tab.View(c)
	}, 800, 600)

	tester.Key(0, ui.KeyTab)
	if !tester.Focused("Nome *") {
		t.Fatal("first Tab did not focus the client name field")
	}
	tester.Key(0, ui.KeyTab)
	if !tester.Focused("CPF ou documento") {
		t.Fatal("second Tab did not focus the document field")
	}
}

func writeTabPreview(t *testing.T, name string, rendered *image.RGBA) {
	directory := os.Getenv("ARDORA_PREVIEW_DIR")
	if directory == "" {
		return
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create preview directory: %v", err)
	}
	filename := strings.NewReplacer("/", "_", "=", "-", "\\", "_").Replace(name) + ".png"
	file, err := os.Create(filepath.Join(directory, filename))
	if err != nil {
		t.Fatalf("create preview: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, rendered); err != nil {
		t.Fatalf("encode preview: %v", err)
	}
}
