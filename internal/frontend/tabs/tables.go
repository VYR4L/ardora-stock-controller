package tabs

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/ardora/stockcontroller/internal/backend/models"
	"github.com/ardora/stockcontroller/internal/frontend"
	"github.com/ardora/stockcontroller/internal/frontend/components"
	"github.com/ardora/stockcontroller/internal/frontend/icons"
	"github.com/ardora/stockcontroller/internal/frontend/style"
)

type TablesTab struct {
	state *frontend.State

	loading   bool
	overviews []models.TableOverview
	games     []models.Game
	visits    []models.Visit
	err       error

	formName  string
	createMsg string
	createOK  bool
	creating  bool

	assignTable, assignClient string
	assignMsg                 string
	assignOK, assigning       bool

	showGameModal             bool
	gameTableID               int64
	gameSource, selectedGame  string
	broughtGame, broughtOwner string
	gameMsg                   string
	gameBusy                  bool
}

func NewTablesTab(s *frontend.State) *TablesTab {
	t := &TablesTab{state: s, gameSource: "Catálogo da luderia", selectedGame: "Selecione...", broughtOwner: "Não informado"}
	t.Refresh()
	return t
}

func (t *TablesTab) Refresh() {
	if t.state == nil || t.state.Window == nil || t.state.TableGameService == nil {
		return
	}
	t.loading = true
	go func() {
		overviews, err := t.state.TableGameService.ListOverviews(context.Background(), false)
		games, gamesErr := t.state.GameService.List(context.Background(), true)
		visits, visitsErr := t.state.VisitService.ListActive(context.Background())
		if err == nil {
			err = gamesErr
		}
		if err == nil {
			err = visitsErr
		}
		t.state.Window.Update(func() {
			t.overviews, t.games, t.visits, t.err, t.loading = overviews, games, visits, err, false
			t.normalizeSelections()
		})
	}()
}

func (t *TablesTab) View(c *ui.Context) {
	components.Page(c, func() {
		components.PageHeader(c, "SALÃO", "Mesas", "Veja quem está jogando, use o catálogo ou registre um jogo trazido pelo cliente.", nil)
		ui.Row(c).Gap(16).Wrap().AlignItems(ui.Start).Children(func() {
			components.Panel(c, "Nova mesa", "Crie uma mesa usando o nome ou número visto pela equipe.", 270, func() {
				if t.createMsg != "" {
					components.FeedbackBanner(c, t.createMsg, feedbackTone(t.createOK))
				}
				ui.Field(c, "Nome ou número *", func() { ui.TextInput(c, &t.formName).Disabled(t.creating) })
				components.FormActions(c, components.Action{Label: busyLabel(t.creating, "Criar mesa"), Disabled: t.creating, Run: t.createTable}, nil)
			})
			components.Panel(c, "Adicionar cliente", "Apenas clientes com uma comanda aberta podem ser alocados.", 360, func() {
				if t.assignMsg != "" {
					components.FeedbackBanner(c, t.assignMsg, feedbackTone(t.assignOK))
				}
				ui.Field(c, "Mesa *", func() { ui.Select(c, &t.assignTable, t.tableOptions()).Disabled(t.assigning || t.loading) })
				ui.Field(c, "Cliente *", func() { ui.Select(c, &t.assignClient, t.clientOptions()).Disabled(t.assigning || t.loading) })
				components.FormActions(c, components.Action{Label: busyLabel(t.assigning, "Alocar cliente"), Disabled: t.assigning || t.loading, Run: t.assignClientToTable}, nil)
			})
		})
		switch {
		case t.loading:
			components.LoadingState(c, "Carregando mesas", "Organizando a visão do salão.")
		case t.err != nil:
			components.ErrorState(c, "Não foi possível carregar as mesas", t.err.Error(), &components.Action{Label: "Tentar novamente", Run: t.Refresh})
		case len(t.overviews) == 0:
			components.EmptyState(c, "Nenhuma mesa cadastrada", "Crie a primeira mesa para montar a visão do salão.", nil)
		default:
			components.Panel(c, "Visão do salão", "Cada cartão reúne clientes e o jogo atual da mesa.", 0, func() {
				ui.Row(c).Gap(12).Wrap().AlignItems(ui.Start).Children(func() {
					for _, overview := range t.overviews {
						t.renderTable(c, overview)
					}
				})
			})
		}
		if t.showGameModal {
			t.renderGameModal(c)
		}
	})
}

func (t *TablesTab) renderTable(c *ui.Context, overview models.TableOverview) {
	tokens := style.FromContext(c)
	gameName, source := "Nenhum jogo", "Mesa disponível"
	if session := overview.CurrentGame; session != nil {
		if session.GameID != nil {
			gameName, source = session.GameName, "Catálogo da luderia"
		} else {
			gameName, source = session.BroughtGameName, "Trazido pelo cliente"
			if session.BroughtByName != "" {
				source += " · " + session.BroughtByName
			}
		}
	}
	ui.Column(c).Gap(12).Padding(16).MinWidth(240).Width(280).Grow(1).Background(tokens.SurfaceSubtle).Border(1, tokens.Border).Radius(13).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(34, 34).Radius(9).Background(tokens.AccentSoft).Children(func() { ui.Icon(c, icons.Tables).Size(18, 18).Margin(8).TextColor(tokens.Accent) })
			ui.Column(c).Gap(1).Grow(1).Children(func() {
				ui.Text(c, overview.Table.Name).FontSize(16).FontWeight(750).TextColor(tokens.Text)
				ui.Textf(c, "Mesa #%d", overview.Table.ID).FontSize(11).TextColor(tokens.Muted)
			})
		})
		ui.Column(c).Gap(3).Children(func() {
			ui.Text(c, "Jogo atual").FontSize(11).TextColor(tokens.Muted)
			ui.Text(c, gameName).FontWeight(650).TextColor(tokens.Text)
			ui.Text(c, source).FontSize(11).TextColor(tokens.Muted)
		})
		ui.Column(c).Gap(3).Children(func() {
			ui.Text(c, "Jogadores").FontSize(11).TextColor(tokens.Muted)
			if len(overview.Clients) == 0 {
				ui.Text(c, "Nenhum cliente alocado").FontSize(12).TextColor(tokens.Muted)
			}
			for _, client := range overview.Clients {
				label := client.Name
				if client.ActiveCard != nil {
					label = fmt.Sprintf("Cartão %03d · %s", *client.ActiveCard, client.Name)
				}
				ui.Text(c, label).FontSize(12).TextColor(tokens.Text)
			}
		})
		if overview.CurrentGame != nil {
			components.StatusBadge(c, "Em jogo", components.ToneAccent)
		} else if len(overview.Clients) > 0 {
			components.StatusBadge(c, "Aguardando jogo", components.ToneWarning)
		} else {
			components.StatusBadge(c, "Disponível", components.ToneSuccess)
		}
		ui.Row(c).Gap(8).Wrap().Children(func() {
			if ui.Button(c, "Definir jogo").Disabled(len(overview.Clients) == 0 || t.gameBusy).Clicked() {
				t.openGameModal(overview.Table.ID)
			}
			if overview.CurrentGame != nil && ui.Button(c, "Encerrar jogo").Disabled(t.gameBusy).Clicked() {
				t.stopGame(overview.Table.ID)
			}
		})
	})
}

func (t *TablesTab) renderGameModal(c *ui.Context) {
	ui.Modal(c, &t.showGameModal, func() {
		ui.Column(c).Gap(16).Width(560).MaxWidthPercent(90).Padding(8).Children(func() {
			components.SectionHeader(c, "Definir jogo da mesa", "Escolha o catálogo ou um jogo trazido pelo cliente.")
			if t.gameMsg != "" {
				components.FeedbackBanner(c, t.gameMsg, components.ToneDanger)
			}
			ui.Field(c, "Origem do jogo", func() {
				ui.Select(c, &t.gameSource, []string{"Catálogo da luderia", "Jogo trazido pelo cliente"}).Disabled(t.gameBusy)
			})
			if t.gameSource == "Jogo trazido pelo cliente" {
				ui.Field(c, "Nome do jogo *", func() { ui.TextInput(c, &t.broughtGame).Disabled(t.gameBusy) })
				ui.Field(c, "Quem trouxe", func() { ui.Select(c, &t.broughtOwner, t.ownerOptions()).Disabled(t.gameBusy) })
			} else {
				ui.Field(c, "Jogo do catálogo *", func() { ui.Select(c, &t.selectedGame, t.gameOptions()).Disabled(t.gameBusy) })
			}
			components.FormActions(c, components.Action{Label: busyLabel(t.gameBusy, "Salvar jogo"), Disabled: t.gameBusy, Run: t.saveGame}, &components.Action{Label: "Cancelar", Disabled: t.gameBusy, Run: func() { t.showGameModal = false }})
		})
	})
}

func (t *TablesTab) createTable() {
	name := strings.TrimSpace(t.formName)
	if name == "" {
		t.createOK, t.createMsg = false, "Informe o nome ou número da mesa."
		return
	}
	if t.creating {
		return
	}
	t.creating, t.createMsg = true, ""
	go func() {
		table := models.Table{Name: name, Active: true}
		err := t.state.TableService.Create(context.Background(), &table)
		t.state.Window.Update(func() {
			t.creating = false
			if err != nil {
				t.createOK, t.createMsg = false, "Não foi possível criar a mesa: "+err.Error()
				return
			}
			t.createOK, t.createMsg, t.formName = true, "Mesa criada com sucesso.", ""
			t.Refresh()
		})
	}()
}

func (t *TablesTab) assignClientToTable() {
	tableID, clientID := t.tableIDForOption(t.assignTable), t.clientIDForOption(t.assignClient)
	if tableID == 0 || clientID == 0 {
		t.assignOK, t.assignMsg = false, "Selecione a mesa e o cliente."
		return
	}
	if t.assigning {
		return
	}
	t.assigning, t.assignMsg = true, ""
	go func() {
		err := t.state.TablesClientsService.AssignClient(context.Background(), tableID, clientID)
		t.state.Window.Update(func() {
			t.assigning = false
			if err != nil {
				t.assignOK, t.assignMsg = false, "Não foi possível alocar o cliente: "+err.Error()
				return
			}
			t.assignOK, t.assignMsg, t.assignClient = true, "Cliente alocado à mesa.", "Selecione..."
			t.Refresh()
		})
	}()
}

func (t *TablesTab) openGameModal(tableID int64) {
	t.gameTableID, t.gameSource, t.selectedGame = tableID, "Catálogo da luderia", "Selecione..."
	t.broughtGame, t.broughtOwner, t.gameMsg, t.showGameModal = "", "Não informado", "", true
}

func (t *TablesTab) saveGame() {
	if t.gameBusy {
		return
	}
	t.gameBusy, t.gameMsg = true, ""
	go func() {
		var err error
		if t.gameSource == "Jogo trazido pelo cliente" {
			ownerID := t.ownerIDForOption(t.broughtOwner)
			var owner *int64
			if ownerID != 0 {
				owner = &ownerID
			}
			err = t.state.TableGameService.StartBrought(context.Background(), t.gameTableID, t.broughtGame, owner)
		} else if gameID := t.gameIDForOption(t.selectedGame); gameID == 0 {
			err = fmt.Errorf("selecione um jogo do catálogo")
		} else {
			err = t.state.TableGameService.StartCatalog(context.Background(), t.gameTableID, gameID)
		}
		t.state.Window.Update(func() {
			t.gameBusy = false
			if err != nil {
				t.gameMsg = "Não foi possível definir o jogo: " + err.Error()
				return
			}
			t.showGameModal = false
			t.Refresh()
		})
	}()
}

func (t *TablesTab) stopGame(tableID int64) {
	if t.gameBusy {
		return
	}
	t.gameBusy = true
	go func() {
		err := t.state.TableGameService.Stop(context.Background(), tableID)
		t.state.Window.Update(func() {
			t.gameBusy = false
			if err != nil {
				t.err = err
				return
			}
			t.Refresh()
		})
	}()
}

func (t *TablesTab) tableOptions() []string {
	options := []string{"Selecione..."}
	for _, item := range t.overviews {
		if item.Table.Active {
			options = append(options, tableOption(item.Table))
		}
	}
	return options
}
func (t *TablesTab) clientOptions() []string {
	options := []string{"Selecione..."}
	for _, visit := range t.visits {
		options = append(options, clientVisitOption(visit))
	}
	return options
}
func (t *TablesTab) gameOptions() []string {
	options := []string{"Selecione..."}
	for _, game := range t.games {
		options = append(options, gameOption(game))
	}
	return options
}
func (t *TablesTab) ownerOptions() []string {
	options := []string{"Não informado"}
	for _, overview := range t.overviews {
		if overview.Table.ID == t.gameTableID {
			for _, client := range overview.Clients {
				options = append(options, ownerOption(client))
			}
		}
	}
	return options
}
func (t *TablesTab) normalizeSelections() {
	if t.tableIDForOption(t.assignTable) == 0 {
		t.assignTable = "Selecione..."
	}
	if t.clientIDForOption(t.assignClient) == 0 {
		t.assignClient = "Selecione..."
	}
}

func tableOption(table models.Table) string { return fmt.Sprintf("%s (#%d)", table.Name, table.ID) }
func clientVisitOption(visit models.Visit) string {
	return fmt.Sprintf("Cartão %03d · %s (#%d)", visit.CardNumber, visit.ClientName, visit.ClientID)
}
func gameOption(game models.Game) string      { return fmt.Sprintf("%s (#%d)", game.Name, game.ID) }
func ownerOption(client models.Client) string { return fmt.Sprintf("%s (#%d)", client.Name, client.ID) }

func (t *TablesTab) tableIDForOption(option string) int64 {
	for _, item := range t.overviews {
		if tableOption(item.Table) == option {
			return item.Table.ID
		}
	}
	return 0
}
func (t *TablesTab) clientIDForOption(option string) int64 {
	for _, visit := range t.visits {
		if clientVisitOption(visit) == option {
			return visit.ClientID
		}
	}
	return 0
}
func (t *TablesTab) gameIDForOption(option string) int64 {
	for _, game := range t.games {
		if gameOption(game) == option {
			return game.ID
		}
	}
	return 0
}
func (t *TablesTab) ownerIDForOption(option string) int64 {
	for _, item := range t.overviews {
		for _, client := range item.Clients {
			if ownerOption(client) == option {
				return client.ID
			}
		}
	}
	return 0
}
