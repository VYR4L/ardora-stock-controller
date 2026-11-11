package tabs

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/ardora/stockcontroller/internal/backend/models"
	"github.com/ardora/stockcontroller/internal/backend/services"
	"github.com/ardora/stockcontroller/internal/frontend"
	"github.com/ardora/stockcontroller/internal/frontend/components"
	"github.com/ardora/stockcontroller/internal/frontend/style"
)

type SalesTab struct {
	state *frontend.State

	clientName     string
	clientDocument string
	clientCard     string
	registerMsg    string
	registerOK     bool
	registerBusy   bool

	loading   bool
	loadErr   error
	active    []models.Visit
	products  []models.Product
	completed []models.Visit

	billVisit   string
	billProduct string
	billQty     string
	billMsg     string
	billOK      bool
	billBusy    bool

	showCheckout    bool
	checkoutVisitID int64
	discount        string
	payment         string
	checkoutMsg     string
	checkoutBusy    bool
	preview         *models.CheckoutSummary

	showReceipt bool
	receipt     *models.CheckoutSummary
	receiptMsg  string
	printing    bool

	completedVisit string
}

func NewSalesTab(s *frontend.State) *SalesTab {
	t := &SalesTab{state: s, billQty: "1", discount: "R$ 0,00", payment: "Selecione..."}
	t.Refresh()
	return t
}

func (t *SalesTab) Refresh() {
	if t.state == nil || t.state.Window == nil || t.state.VisitService == nil {
		return
	}
	t.loading = true
	go func() {
		active, err := t.state.VisitService.ListActive(context.Background())
		products, productsErr := t.state.ProductService.List(context.Background(), true)
		completed, completedErr := t.state.VisitService.ListCompleted(context.Background(), 25)
		if err == nil {
			err = productsErr
		}
		if err == nil {
			err = completedErr
		}
		t.state.Window.Update(func() {
			t.active = active
			t.products = products
			t.completed = completed
			t.loadErr = err
			t.loading = false
			t.normalizeSelections()
		})
	}()
}

func (t *SalesTab) View(c *ui.Context) {
	components.Page(c, func() {
		components.PageHeader(c, "ATENDIMENTO", "Vendas e comandas", "Registre entradas, lance produtos e feche a conta do cliente em um único fluxo.", nil)

		ui.Row(c).Gap(16).Wrap().AlignItems(ui.Start).Children(func() {
			components.Panel(c, "Registrar entrada", "Identifique o cliente e associe um cartão disponível.", 300, func() {
				if t.registerMsg != "" {
					components.FeedbackBanner(c, t.registerMsg, feedbackTone(t.registerOK))
				}
				ui.Field(c, "Nome *", func() { ui.TextInput(c, &t.clientName).Disabled(t.registerBusy) })
				ui.Field(c, "CPF ou documento", func() { ui.TextInput(c, &t.clientDocument).Disabled(t.registerBusy) })
				ui.Field(c, "Cartão (0–999) *", func() { ui.TextInput(c, &t.clientCard).Disabled(t.registerBusy) })
				components.FormActions(c, components.Action{Label: busyLabel(t.registerBusy, "Registrar entrada"), Disabled: t.registerBusy, Run: t.registerClient}, nil)
			}).Grow(1)

			components.Panel(c, "Lançar produto", "Selecione uma comanda aberta e um produto disponível.", 340, func() {
				if t.billMsg != "" {
					components.FeedbackBanner(c, t.billMsg, feedbackTone(t.billOK))
				}
				ui.Field(c, "Cliente / comanda *", func() {
					ui.Select(c, &t.billVisit, t.activeOptions()).Disabled(t.billBusy || t.loading)
				})
				ui.Field(c, "Produto *", func() {
					ui.Select(c, &t.billProduct, t.productOptions()).Disabled(t.billBusy || t.loading)
				})
				ui.Field(c, "Quantidade *", func() { ui.TextInput(c, &t.billQty).Disabled(t.billBusy) })
				components.FormActions(c, components.Action{Label: busyLabel(t.billBusy, "Lançar na comanda"), Disabled: t.billBusy || t.loading, Run: t.billProductCharge}, nil)
			}).Grow(1)
		})

		switch {
		case t.loading:
			components.LoadingState(c, "Carregando comandas", "Atualizando clientes e produtos disponíveis.")
		case t.loadErr != nil:
			components.ErrorState(c, "Não foi possível carregar as comandas", t.loadErr.Error(), &components.Action{Label: "Tentar novamente", Run: t.Refresh})
		default:
			t.renderActiveVisits(c)
			t.renderCompletedVisits(c)
		}

		if t.showCheckout {
			t.renderCheckoutModal(c)
		}
		if t.showReceipt {
			t.renderReceiptModal(c)
		}
	})
}

func (t *SalesTab) renderActiveVisits(c *ui.Context) {
	components.Panel(c, "Comandas abertas", "Valores de entrada são preservados desde o check-in.", 0, func() {
		if len(t.active) == 0 {
			components.EmptyState(c, "Nenhuma comanda aberta", "Registre a entrada do primeiro cliente para começar.", nil)
			return
		}
		for _, visit := range t.active {
			visit := visit
			ui.Row(c).Gap(12).Wrap().AlignItems(ui.Center).Padding(10, 0).BorderWidth(0, 0, 0, 1).BorderColor(style.FromContext(c).Border).Children(func() {
				ui.Column(c).Gap(2).Grow(1).MinWidth(220).Children(func() {
					ui.Textf(c, "Cartão %03d · %s", visit.CardNumber, visit.ClientName).FontWeight(700)
					ui.Textf(c, "Entrada %s · desde %s", services.FormatBRLCents(visit.EntryPriceCents), visit.OpenedAt.Format("02/01 15:04")).FontSize(12).TextColor(style.FromContext(c).Muted)
				})
				components.StatusBadge(c, "Aberta", components.ToneAccent)
				if ui.PrimaryButton(c, "Fechar comanda").Clicked() {
					t.openCheckout(visit.ID)
				}
			})
		}
	})
}

func (t *SalesTab) renderCompletedVisits(c *ui.Context) {
	components.Panel(c, "Recibos recentes", "Consulte e reimprima uma comanda já fechada sem alterar seus valores.", 0, func() {
		if len(t.completed) == 0 {
			ui.Text(c, "Nenhuma comanda fechada ainda.").TextColor(style.FromContext(c).Muted)
			return
		}
		ui.Row(c).Gap(12).Wrap().AlignItems(ui.End).Children(func() {
			ui.Field(c, "Comanda fechada", func() {
				ui.Select(c, &t.completedVisit, t.completedOptions())
			}).MinWidth(340).Grow(1)
			if ui.Button(c, "Abrir recibo").Clicked() {
				t.reopenReceipt()
			}
		})
	})
}

func (t *SalesTab) renderCheckoutModal(c *ui.Context) {
	ui.Modal(c, &t.showCheckout, func() {
		ui.Column(c).Gap(16).Width(660).MaxWidthPercent(90).Padding(8).Children(func() {
			components.SectionHeader(c, "Fechar comanda", "Revise os itens, aplique o desconto e confirme o pagamento.")
			if t.checkoutMsg != "" {
				components.FeedbackBanner(c, t.checkoutMsg, components.ToneDanger)
			}
			if t.preview == nil {
				components.LoadingState(c, "Calculando total", "Buscando os lançamentos desta visita.")
			} else {
				t.renderSummary(c, t.preview)
			}
			ui.Row(c).Gap(12).Wrap().Children(func() {
				ui.Field(c, "Desconto", func() { ui.TextInput(c, &t.discount).Disabled(t.checkoutBusy) }).MinWidth(180).Grow(1)
				ui.Field(c, "Forma de pagamento *", func() {
					ui.Select(c, &t.payment, []string{"Selecione...", "Dinheiro", "Pix", "Crédito", "Débito", "Outro"}).Disabled(t.checkoutBusy)
				}).MinWidth(220).Grow(1)
			})
			ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
				if ui.Button(c, "Atualizar total").Disabled(t.checkoutBusy).Clicked() {
					t.loadCheckoutPreview()
				}
				if ui.Button(c, "Cancelar").Disabled(t.checkoutBusy).Clicked() {
					t.showCheckout = false
				}
				if ui.PrimaryButton(c, busyLabel(t.checkoutBusy, "Confirmar fechamento")).Disabled(t.checkoutBusy || t.preview == nil).Clicked() {
					t.confirmCheckout()
				}
			})
		})
	})
}

func (t *SalesTab) renderReceiptModal(c *ui.Context) {
	ui.Modal(c, &t.showReceipt, func() {
		ui.Column(c).Gap(16).Width(660).MaxWidthPercent(90).Padding(8).Children(func() {
			components.SectionHeader(c, "Recibo não fiscal", "A comanda foi preservada e pode ser impressa novamente.")
			if t.receiptMsg != "" {
				components.FeedbackBanner(c, t.receiptMsg, components.ToneDanger)
			}
			if t.receipt != nil {
				t.renderSummary(c, t.receipt)
				ui.Textf(c, "Pagamento: %s · Recibo %s", services.PaymentMethodLabel(t.receipt.Visit.PaymentMethod), t.receipt.Visit.ReceiptIdentifier).FontSize(12).TextColor(style.FromContext(c).Muted)
			}
			ui.Row(c).Gap(10).Justify(ui.End).Children(func() {
				if ui.Button(c, "Fechar").Disabled(t.printing).Clicked() {
					t.showReceipt = false
				}
				if ui.PrimaryButton(c, busyLabel(t.printing, "Imprimir")).Disabled(t.printing || t.receipt == nil).Clicked() {
					t.printReceipt()
				}
			})
		})
	})
}

func (t *SalesTab) renderSummary(c *ui.Context, summary *models.CheckoutSummary) {
	visit := summary.Visit
	ui.Column(c).Gap(6).Children(func() {
		ui.Textf(c, "%s · Cartão %03d", visit.ClientName, visit.CardNumber).FontSize(17).FontWeight(750)
		ui.Textf(c, "Entrada: %s", services.FormatBRLCents(visit.EntryPriceCents))
		for _, line := range summary.Lines {
			line := line
			ui.Row(c).Gap(8).Children(func() {
				ui.Textf(c, "%dx %s", line.Quantity, line.Description).Grow(1)
				ui.Text(c, services.FormatBRLCents(line.TotalCents)).FontWeight(650)
			})
		}
		ui.Textf(c, "Consumo: %s", services.FormatBRLCents(visit.ConsumptionSubtotalCents)).TextColor(style.FromContext(c).Muted)
		ui.Textf(c, "Total bruto: %s", services.FormatBRLCents(visit.GrossTotalCents)).FontWeight(650)
		ui.Textf(c, "Desconto: %s", services.FormatBRLCents(visit.DiscountCents)).TextColor(style.FromContext(c).Muted)
		ui.Textf(c, "Total final: %s", services.FormatBRLCents(visit.FinalTotalCents)).FontSize(20).FontWeight(800).TextColor(style.FromContext(c).Accent)
	})
}

func (t *SalesTab) registerClient() {
	name := strings.TrimSpace(t.clientName)
	cardText := strings.TrimSpace(t.clientCard)
	if name == "" {
		t.setRegisterError("Informe o nome do cliente.")
		return
	}
	codeValue, err := services.NormalizeAccessCode(cardText)
	if err != nil {
		t.setRegisterError("Informe um cartão válido entre 0 e 999.")
		return
	}
	if t.registerBusy {
		return
	}
	var document *string
	if value := strings.TrimSpace(t.clientDocument); value != "" {
		document = &value
	}
	t.registerBusy = true
	t.registerMsg = ""
	go func() {
		visit, err := t.state.VisitService.OpenVisit(context.Background(), name, document, codeValue)
		t.state.Window.Update(func() {
			t.registerBusy = false
			if err != nil {
				t.setRegisterError("Não foi possível registrar a entrada: " + err.Error())
				return
			}
			t.registerOK = true
			t.registerMsg = fmt.Sprintf("Entrada registrada. Valor desta comanda: %s.", services.FormatBRLCents(visit.EntryPriceCents))
			t.clientName, t.clientDocument, t.clientCard = "", "", ""
			t.Refresh()
		})
	}()
}

func (t *SalesTab) billProductCharge() {
	visitID := t.visitIDForOption(t.billVisit)
	productID := t.productIDForOption(t.billProduct)
	quantity, err := strconv.Atoi(strings.TrimSpace(t.billQty))
	if visitID == 0 || productID == 0 || err != nil || quantity <= 0 {
		t.setBillError("Selecione cliente e produto e informe uma quantidade válida.")
		return
	}
	if t.billBusy {
		return
	}
	t.billBusy, t.billMsg = true, ""
	clientID := int64(0)
	for _, visit := range t.active {
		if visit.ID == visitID {
			clientID = visit.ClientID
			break
		}
	}
	go func() {
		err := t.state.SalesService.BillProduct(context.Background(), clientID, productID, quantity)
		t.state.Window.Update(func() {
			t.billBusy = false
			if err != nil {
				t.setBillError("Não foi possível lançar o produto: " + err.Error())
				return
			}
			t.billOK, t.billMsg, t.billQty = true, "Produto lançado na comanda.", "1"
			t.Refresh()
		})
	}()
}

func (t *SalesTab) openCheckout(visitID int64) {
	t.checkoutVisitID = visitID
	t.discount = "R$ 0,00"
	t.payment = "Selecione..."
	t.checkoutMsg = ""
	t.preview = nil
	t.showCheckout = true
	t.loadCheckoutPreview()
}

func (t *SalesTab) loadCheckoutPreview() {
	discount, err := services.ParseBRLCents(t.discount)
	if err != nil {
		t.checkoutMsg = "Informe um desconto válido: " + err.Error()
		return
	}
	t.checkoutBusy = true
	t.checkoutMsg = ""
	go func() {
		preview, err := t.state.VisitService.Preview(context.Background(), t.checkoutVisitID, discount)
		t.state.Window.Update(func() {
			t.checkoutBusy = false
			if err != nil {
				t.checkoutMsg = "Não foi possível calcular a comanda: " + err.Error()
				return
			}
			t.preview = preview
			t.discount = services.FormatBRLCents(preview.Visit.DiscountCents)
		})
	}()
}

func (t *SalesTab) confirmCheckout() {
	discount, err := services.ParseBRLCents(t.discount)
	if err != nil {
		t.checkoutMsg = "Informe um desconto válido: " + err.Error()
		return
	}
	payment := paymentFromLabel(t.payment)
	if payment == "" {
		t.checkoutMsg = "Selecione a forma de pagamento."
		return
	}
	t.checkoutBusy, t.checkoutMsg = true, ""
	go func() {
		summary, err := t.state.VisitService.Checkout(context.Background(), t.checkoutVisitID, discount, payment)
		if err == nil {
			_, err = t.state.ReceiptService.Save(context.Background(), summary.Visit.ID, false)
		}
		t.state.Window.Update(func() {
			t.checkoutBusy = false
			if err != nil && summary == nil {
				t.checkoutMsg = "Não foi possível fechar a comanda: " + err.Error()
				return
			}
			t.showCheckout = false
			t.receipt = summary
			t.receiptMsg = ""
			if err != nil {
				t.receiptMsg = "A comanda foi fechada, mas o arquivo do recibo não pôde ser criado: " + err.Error()
			}
			t.showReceipt = true
			t.Refresh()
		})
	}()
}

func (t *SalesTab) reopenReceipt() {
	visitID := t.completedIDForOption(t.completedVisit)
	if visitID == 0 {
		return
	}
	go func() {
		summary, err := t.state.VisitService.Preview(context.Background(), visitID, 0)
		t.state.Window.Update(func() {
			if err != nil {
				t.loadErr = err
				return
			}
			t.receipt = summary
			t.receiptMsg = ""
			t.showReceipt = true
		})
	}()
}

func (t *SalesTab) printReceipt() {
	if t.receipt == nil || t.printing {
		return
	}
	t.printing, t.receiptMsg = true, ""
	visitID := t.receipt.Visit.ID
	go func() {
		path, err := t.state.ReceiptService.Open(context.Background(), visitID, true)
		t.state.Window.Update(func() {
			t.printing = false
			if err != nil {
				t.receiptMsg = fmt.Sprintf("Não foi possível abrir a impressão. O recibo permanece salvo em %s: %v", path, err)
			}
		})
	}()
}

func (t *SalesTab) activeOptions() []string {
	options := []string{"Selecione..."}
	for _, visit := range t.active {
		options = append(options, visitOption(visit))
	}
	return options
}

func (t *SalesTab) productOptions() []string {
	options := []string{"Selecione..."}
	for _, product := range t.products {
		options = append(options, productOption(product))
	}
	return options
}

func (t *SalesTab) completedOptions() []string {
	options := []string{"Selecione..."}
	for _, visit := range t.completed {
		options = append(options, completedOption(visit))
	}
	return options
}

func (t *SalesTab) normalizeSelections() {
	if t.visitIDForOption(t.billVisit) == 0 {
		t.billVisit = "Selecione..."
	}
	if t.productIDForOption(t.billProduct) == 0 {
		t.billProduct = "Selecione..."
	}
	if t.completedIDForOption(t.completedVisit) == 0 {
		t.completedVisit = "Selecione..."
	}
}

func visitOption(visit models.Visit) string {
	return fmt.Sprintf("Cartão %03d · %s (#%d)", visit.CardNumber, visit.ClientName, visit.ID)
}

func productOption(product models.Product) string {
	return fmt.Sprintf("%s · %s · estoque %d (#%d)", product.Name, services.FormatBRLCents(product.PriceCents), product.Stock, product.ID)
}

func completedOption(visit models.Visit) string {
	return fmt.Sprintf("%s · %s · %s", visit.ReceiptIdentifier, visit.ClientName, services.FormatBRLCents(visit.FinalTotalCents))
}

func (t *SalesTab) visitIDForOption(option string) int64 {
	for _, visit := range t.active {
		if visitOption(visit) == option {
			return visit.ID
		}
	}
	return 0
}

func (t *SalesTab) productIDForOption(option string) int64 {
	for _, product := range t.products {
		if productOption(product) == option {
			return product.ID
		}
	}
	return 0
}

func (t *SalesTab) completedIDForOption(option string) int64 {
	for _, visit := range t.completed {
		if completedOption(visit) == option {
			return visit.ID
		}
	}
	return 0
}

func paymentFromLabel(label string) models.PaymentMethod {
	switch label {
	case "Dinheiro":
		return models.PaymentCash
	case "Pix":
		return models.PaymentPix
	case "Crédito":
		return models.PaymentCredit
	case "Débito":
		return models.PaymentDebit
	case "Outro":
		return models.PaymentOther
	default:
		return ""
	}
}

func (t *SalesTab) setRegisterError(message string) {
	t.registerOK = false
	t.registerMsg = message
}

func (t *SalesTab) setBillError(message string) {
	t.billOK = false
	t.billMsg = message
}

func feedbackTone(success bool) components.Tone {
	if success {
		return components.ToneSuccess
	}
	return components.ToneDanger
}

func busyLabel(busy bool, label string) string {
	if busy {
		return "Processando..."
	}
	return label
}
