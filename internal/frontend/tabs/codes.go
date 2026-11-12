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
	"github.com/ardora/stockcontroller/internal/frontend/icons"
	"github.com/ardora/stockcontroller/internal/frontend/style"
)

type CodesTab struct {
	state *frontend.State

	loading bool
	err     error
	stats   models.AccessCodeStats
	batches []models.AccessCodeBatch
	codes   []models.AccessCode

	quantity string
	query    string
	msg      string
	msgOK    bool
	busy     bool

	printingID int64
	codeBusyID int64
}

func NewCodesTab(state *frontend.State) *CodesTab {
	tab := &CodesTab{state: state, quantity: "1"}
	tab.Refresh()
	return tab
}

func (t *CodesTab) Refresh() {
	if t.state == nil || t.state.Window == nil || t.state.AccessCodeService == nil {
		return
	}
	t.loading = true
	go func() {
		ctx := context.Background()
		stats, err := t.state.AccessCodeService.Stats(ctx)
		batches, batchesErr := t.state.AccessCodeService.ListBatches(ctx)
		codes, codesErr := t.state.AccessCodeService.ListCodes(ctx, "")
		if err == nil {
			err = batchesErr
		}
		if err == nil {
			err = codesErr
		}
		t.state.Window.Update(func() {
			t.stats, t.batches, t.codes = stats, batches, codes
			t.err, t.loading = err, false
		})
	}()
}

func (t *CodesTab) View(c *ui.Context) {
	components.Page(c, func() {
		components.PageHeader(c, "ATENDIMENTO", "Códigos", "Gere, imprima e acompanhe as comandas físicas reutilizáveis.", nil)
		if t.msg != "" {
			components.FeedbackBanner(c, t.msg, feedbackTone(t.msgOK))
		}

		ui.Row(c).Gap(12).Wrap().Children(func() {
			components.MetricCard(c, "Códigos", t.stats.Total, "Etiquetas cadastradas", icons.Codes, nil)
			components.MetricCard(c, "Disponíveis", t.stats.Available, "Prontos para check-in", icons.Check, nil)
			components.MetricCard(c, "Em uso", t.stats.InUse, "Vinculados a visitas", icons.Info, nil)
			components.MetricCard(c, "Arquivados", t.stats.Archived, "Fora de circulação", icons.Empty, nil)
		})

		components.Panel(c, "Gerar lote", "A sequência continua após o maior número já emitido.", 0, func() {
			nextLabel := "Capacidade esgotada"
			if t.stats.NextNumber <= 999 {
				nextLabel = fmt.Sprintf("Próximo código: %03d", t.stats.NextNumber)
			}
			ui.Text(c, nextLabel).FontWeight(650).TextColor(style.FromContext(c).Text)
			ui.Row(c).Gap(12).Wrap().AlignItems(ui.End).Children(func() {
				ui.Field(c, "Quantidade *", func() {
					ui.TextInput(c, &t.quantity).Disabled(t.busy || t.stats.NextNumber > 999)
				}).MinWidth(180).Grow(1)
				if ui.PrimaryButton(c, busyLabel(t.busy, "Gerar códigos")).Disabled(t.busy || t.loading || t.stats.NextNumber > 999).Clicked() {
					t.generateBatch()
				}
			})
		})

		switch {
		case t.loading:
			components.LoadingState(c, "Carregando códigos", "Atualizando lotes e disponibilidade.")
		case t.err != nil:
			components.ErrorState(c, "Não foi possível carregar os códigos", t.err.Error(), &components.Action{Label: "Tentar novamente", Run: t.Refresh})
		default:
			t.renderBatches(c)
			t.renderCodes(c)
		}
	})
}

func (t *CodesTab) renderBatches(c *ui.Context) {
	components.Panel(c, "Lotes", "Cada lote pode ser impresso novamente sem reservar novos números.", 0, func() {
		if len(t.batches) == 0 {
			components.EmptyState(c, "Nenhum lote gerado", "Informe uma quantidade para criar as primeiras comandas.", nil)
			return
		}
		for _, batch := range t.batches {
			batch := batch
			ui.Row(c).Gap(12).Wrap().AlignItems(ui.Center).Padding(10, 0).BorderWidth(0, 0, 0, 1).BorderColor(style.FromContext(c).Border).Children(func() {
				ui.Column(c).Gap(2).Grow(1).MinWidth(220).Children(func() {
					title := fmt.Sprintf("%03d–%03d · %d códigos", batch.FirstNumber, batch.LastNumber, batch.Quantity)
					if batch.Legacy {
						title = "Legado · " + title
					}
					ui.Text(c, title).FontWeight(700)
					detail := "Gerado em " + batch.CreatedAt.Format("02/01/2006 15:04")
					if batch.LastPrintedAt != nil {
						detail += " · impresso em " + batch.LastPrintedAt.Format("02/01/2006 15:04")
					}
					ui.Text(c, detail).FontSize(12).TextColor(style.FromContext(c).Muted)
				})
				if ui.Button(c, busyLabel(t.printingID == batch.ID, "Imprimir")).Disabled(t.printingID != 0).Clicked() {
					t.printBatch(batch.ID)
				}
			})
		}
	})
}

func (t *CodesTab) renderCodes(c *ui.Context) {
	components.Panel(c, "Códigos cadastrados", "Consulte a disponibilidade e retire etiquetas perdidas de circulação.", 0, func() {
		filtered := filterAccessCodes(t.codes, t.query)
		ui.Row(c).Gap(12).Wrap().AlignItems(ui.Center).Children(func() {
			ui.SearchField(c, &t.query).Width(300).Placeholder("Buscar número...").Label("Buscar códigos por número")
			ui.Spacer(c)
			ui.Textf(c, "%d resultado(s)", len(filtered)).FontSize(12).TextColor(style.FromContext(c).Muted)
		})
		if len(t.codes) == 0 {
			components.EmptyState(c, "Nenhum código cadastrado", "Gere um lote para começar.", nil)
			return
		}
		if len(filtered) == 0 {
			components.EmptyState(c, "Nenhum código encontrado", "Tente outro número.", nil)
			return
		}
		ui.Row(c).Gap(10).Wrap().Children(func() {
			for _, code := range filtered {
				code := code
				ui.Column(c).Gap(10).Padding(14).Width(190).Background(style.FromContext(c).SurfaceSubtle).Border(1, style.FromContext(c).Border).Radius(12).Children(func() {
					ui.Text(c, code.Value).FontSize(26).FontWeight(800).TextColor(style.FromContext(c).Text)
					label, tone := accessCodeStatePresentation(code.State)
					components.StatusBadge(c, label, tone)
					switch code.State {
					case models.AccessCodeAvailable:
						if ui.Button(c, "Arquivar").Disabled(t.codeBusyID != 0).Clicked() {
							t.changeCodeState(code.ID, false)
						}
					case models.AccessCodeArchived:
						if ui.Button(c, "Reativar").Disabled(t.codeBusyID != 0).Clicked() {
							t.changeCodeState(code.ID, true)
						}
					}
				})
			}
		})
	})
}

func (t *CodesTab) generateBatch() {
	quantity, err := parseBatchQuantity(t.quantity, t.stats.NextNumber)
	if err != nil {
		t.msgOK, t.msg = false, err.Error()
		return
	}
	if t.busy || t.state == nil || t.state.AccessCodeService == nil {
		return
	}
	t.busy, t.msg = true, ""
	go func() {
		batch, err := t.state.AccessCodeService.GenerateBatch(context.Background(), quantity)
		t.state.Window.Update(func() {
			t.busy = false
			if err != nil {
				t.msgOK, t.msg = false, "Não foi possível gerar o lote: "+err.Error()
				return
			}
			t.msgOK, t.msg = true, fmt.Sprintf("Lote %03d–%03d gerado com sucesso.", batch.FirstNumber, batch.LastNumber)
			t.Refresh()
		})
	}()
}

func (t *CodesTab) printBatch(batchID int64) {
	if t.printingID != 0 || t.state == nil || t.state.AccessCodePrintService == nil {
		return
	}
	t.printingID, t.msg = batchID, ""
	go func() {
		path, err := t.state.AccessCodePrintService.OpenBatch(context.Background(), batchID, true)
		t.state.Window.Update(func() {
			t.printingID = 0
			if err != nil {
				t.msgOK, t.msg = false, fmt.Sprintf("Não foi possível abrir a impressão. A folha permanece salva em %s: %v", path, err)
				return
			}
			t.msgOK, t.msg = true, "Folha de códigos aberta para impressão."
			t.Refresh()
		})
	}()
}

func (t *CodesTab) changeCodeState(codeID int64, reactivate bool) {
	if t.codeBusyID != 0 || t.state == nil || t.state.AccessCodeService == nil {
		return
	}
	t.codeBusyID, t.msg = codeID, ""
	go func() {
		var err error
		if reactivate {
			err = t.state.AccessCodeService.Reactivate(context.Background(), codeID)
		} else {
			err = t.state.AccessCodeService.Archive(context.Background(), codeID)
		}
		t.state.Window.Update(func() {
			t.codeBusyID = 0
			if err != nil {
				t.msgOK, t.msg = false, "Não foi possível atualizar o código: "+err.Error()
				return
			}
			t.msgOK, t.msg = true, "Código atualizado com sucesso."
			t.Refresh()
		})
	}()
}

func parseBatchQuantity(raw string, nextNumber int) (int, error) {
	quantity, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || quantity <= 0 || quantity > 999 {
		return 0, fmt.Errorf("Informe uma quantidade entre 1 e 999.")
	}
	if nextNumber < 1 || nextNumber > 999 || nextNumber+quantity-1 > 999 {
		return 0, fmt.Errorf("A quantidade ultrapassa o limite do código 999.")
	}
	return quantity, nil
}

func filterAccessCodes(codes []models.AccessCode, query string) []models.AccessCode {
	query = strings.TrimSpace(query)
	if query == "" {
		return codes
	}
	value, err := services.NormalizeAccessCode(query)
	if err != nil {
		return []models.AccessCode{}
	}
	filtered := make([]models.AccessCode, 0, 1)
	for _, code := range codes {
		if code.Value == value {
			filtered = append(filtered, code)
		}
	}
	return filtered
}

func accessCodeStatePresentation(state models.AccessCodeState) (string, components.Tone) {
	switch state {
	case models.AccessCodeAvailable:
		return "Disponível", components.ToneSuccess
	case models.AccessCodeInUse:
		return "Em uso", components.ToneAccent
	default:
		return "Arquivado", components.ToneNeutral
	}
}

func batchOption(batch models.AccessCodeBatch) string {
	return fmt.Sprintf("Lote %06d · %03d–%03d", batch.ID, batch.FirstNumber, batch.LastNumber)
}

func (t *CodesTab) batchIDForOption(option string) int64 {
	for _, batch := range t.batches {
		if batchOption(batch) == option {
			return batch.ID
		}
	}
	return 0
}
