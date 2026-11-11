package tabs

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/ardora/stockcontroller/internal/backend/models"
	"github.com/ardora/stockcontroller/internal/frontend"
	"github.com/ardora/stockcontroller/internal/frontend/components"
	"github.com/ardora/stockcontroller/internal/frontend/icons"
	"github.com/ardora/stockcontroller/internal/frontend/style"
)

type GamesTab struct {
	state   *frontend.State
	actions gameActions
	update  func(func())

	loading bool
	games   []models.Game
	err     error
	query   string
	pageMsg string
	pageOK  bool

	showModal bool
	formGame  models.Game
	formImage string
	formMsg   string
	saving    bool

	showRemove   bool
	removeTarget models.Game
	removePlan   models.RemovalPlan
	removeBusy   bool
}

type gameActions struct {
	create      func(context.Context, *models.Game) error
	update      func(context.Context, *models.Game) error
	planRemoval func(context.Context, int64) (models.RemovalPlan, error)
	remove      func(context.Context, int64) (models.RemovalPlan, error)
}

func NewGamesTab(s *frontend.State) *GamesTab {
	t := &GamesTab{state: s}
	if s != nil {
		if s.GameService != nil {
			t.actions = gameActions{
				create:      s.GameService.Create,
				update:      s.GameService.Update,
				planRemoval: s.GameService.PlanRemoval,
				remove:      s.GameService.Remove,
			}
		}
		if s.Window != nil {
			t.update = s.Window.Update
		}
	}
	t.loadGames()
	return t
}

func (t *GamesTab) loadGames() {
	if t.state == nil || t.state.GameService == nil || t.state.Window == nil {
		return
	}
	t.loading = true
	go func() {
		games, err := t.state.GameService.List(context.Background(), false)
		t.state.Window.Update(func() {
			t.games = games
			t.err = err
			t.loading = false
		})
	}()
}

func (t *GamesTab) View(c *ui.Context) {
	components.Page(c, func() {
		components.PageHeader(c, "CATÁLOGO", "Jogos", "Encontre rapidamente os títulos e confira a faixa de jogadores.", &components.Action{Label: "Novo jogo", Run: t.openCreate})

		if t.pageMsg != "" {
			components.FeedbackBanner(c, t.pageMsg, feedbackTone(t.pageOK))
		}
		if t.loading {
			components.LoadingState(c, "Carregando jogos", "Preparando o catálogo da luderia.")
		} else if t.err != nil {
			components.ErrorState(c, "Não foi possível carregar os jogos", t.err.Error(), &components.Action{Label: "Tentar novamente", Run: t.loadGames})
		} else {
			t.renderContent(c)
		}
	})

	t.renderModal(c)
	t.renderRemoveModal(c)
}

func (t *GamesTab) renderContent(c *ui.Context) {
	active := 0
	for _, game := range t.games {
		if game.Active {
			active++
		}
	}

	ui.Row(c).Gap(12).Wrap().Children(func() {
		components.MetricCard(c, "Jogos", len(t.games), "Títulos cadastrados", icons.Games, nil)
		components.MetricCard(c, "Ativos", active, "Disponíveis no catálogo", icons.Check, nil)
	})

	components.Panel(c, "Catálogo de jogos", "Busque pelo nome e confira a faixa recomendada de jogadores.", 0, func() {
		filtered := filterGames(t.games, t.query)
		ui.Row(c).Gap(12).Wrap().AlignItems(ui.Center).Children(func() {
			ui.SearchField(c, &t.query).Width(300).Placeholder("Buscar por nome...").Label("Buscar jogos por nome")
			ui.Spacer(c)
			ui.Textf(c, "%d resultado(s)", len(filtered)).FontSize(12).TextColor(style.FromContext(c).Muted)
		})

		switch {
		case len(t.games) == 0:
			components.EmptyState(c, "Nenhum jogo cadastrado", "Cadastre o primeiro título do catálogo.", &components.Action{Label: "Cadastrar jogo", Run: t.openCreate})
		case len(filtered) == 0:
			components.EmptyState(c, "Nenhum jogo encontrado", "Tente buscar por outro nome.", nil)
		default:
			ui.Row(c).Gap(10).Wrap().Children(func() {
				for _, game := range filtered {
					t.renderGame(c, game)
				}
			})
		}
	})
}

func (t *GamesTab) renderGame(c *ui.Context, game models.Game) {
	tokens := style.FromContext(c)
	imagePath := ""
	if t.state != nil && t.state.ImageService != nil {
		imagePath = t.state.ImageService.GetPath(game.Image)
	}
	ui.Column(c).Gap(12).Padding(14).MinWidth(220).Width(250).Grow(1).Background(tokens.SurfaceSubtle).Border(1, tokens.Border).Radius(12).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			components.Thumbnail(c, imagePath, "Capa de "+game.Name, 58)
			ui.Column(c).Gap(4).Grow(1).Children(func() {
				ui.Text(c, game.Name).FontSize(15).FontWeight(700).TextColor(tokens.Text)
				ui.Textf(c, "%d–%d jogadores", game.MinPlayers, game.MaxPlayers).FontSize(12).TextColor(tokens.Muted)
			})
		})
		if game.Active {
			components.StatusBadge(c, "Ativo", components.ToneSuccess)
		} else {
			components.StatusBadge(c, "Inativo", components.ToneNeutral)
		}
		ui.Row(c).Gap(8).Wrap().Children(func() {
			if ui.Button(c, "Editar").Clicked() {
				t.openEdit(game)
			}
			if ui.Button(c, "Excluir").Clicked() {
				t.requestRemoval(game)
			}
		})
	})
}

func (t *GamesTab) openEdit(game models.Game) {
	t.formGame = game
	t.formImage = ""
	t.formMsg = ""
	t.showModal = true
}

func (t *GamesTab) openCreate() {
	t.formGame = models.Game{Active: true, MinPlayers: 1, MaxPlayers: 4}
	t.formImage = ""
	t.formMsg = ""
	t.showModal = true
}

func (t *GamesTab) renderModal(c *ui.Context) {
	ui.Modal(c, &t.showModal, func() {
		ui.Column(c).Gap(18).Width(620).MaxWidthPercent(88).Padding(8).Children(func() {
			title, subtitle := "Novo jogo", "Cadastre um título e sua faixa de jogadores."
			if t.formGame.ID != 0 {
				title, subtitle = "Editar jogo", "Atualize os dados do jogo ou reative seu cadastro."
			}
			components.SectionHeader(c, title, subtitle)
			if t.formMsg != "" {
				components.FeedbackBanner(c, t.formMsg, components.ToneDanger)
			}

			ui.Field(c, "Nome *", func() {
				ui.TextInput(c, &t.formGame.Name).Disabled(t.saving)
			})
			ui.Field(c, "Observações", func() {
				ui.TextArea(c, &t.formGame.Notes).Lines(2, 5).Disabled(t.saving)
			})
			ui.Row(c).Gap(12).Wrap().Children(func() {
				ui.Field(c, "Mínimo de jogadores", func() {
					minimum := float64(t.formGame.MinPlayers)
					if ui.NumberInput(c, &minimum, 1, 99, 1).Disabled(t.saving).Changed() {
						t.formGame.MinPlayers = int(minimum)
					}
				}).MinWidth(200).Grow(1)
				ui.Field(c, "Máximo de jogadores", func() {
					maximum := float64(t.formGame.MaxPlayers)
					if ui.NumberInput(c, &maximum, 1, 99, 1).Disabled(t.saving).Changed() {
						t.formGame.MaxPlayers = int(maximum)
					}
				}).MinWidth(200).Grow(1)
			})
			ui.Checkbox(c, &t.formGame.Active, "Ativo").Disabled(t.saving)

			ui.Field(c, "Capa do jogo", func() {
				ui.Row(c).Gap(10).Wrap().AlignItems(ui.Center).Children(func() {
					name := "Nenhum arquivo selecionado"
					if t.formImage != "" {
						name = filepath.Base(t.formImage)
					} else if t.formGame.Image != "" {
						name = filepath.Base(t.formGame.Image) + " (capa atual)"
					}
					ui.Text(c, name).FontSize(12).TextColor(style.FromContext(c).Muted).Grow(1)
					if ui.Button(c, "Escolher imagem").Disabled(t.saving).Clicked() {
						t.chooseImage()
					}
				})
			})

			components.FormActions(c,
				components.Action{Label: savingLabel(t.saving), Disabled: t.saving, Run: t.saveForm},
				&components.Action{Label: "Cancelar", Disabled: t.saving, Run: func() { t.showModal = false }},
			)
		})
	})
}

func (t *GamesTab) chooseImage() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:  t.state.Window,
			Title:   "Escolher capa do jogo",
			Filters: []mygo.FileFilter{{Name: "Imagens", Extensions: []string{"jpg", "jpeg", "png", "webp"}}},
		})
		t.state.Window.Update(func() {
			if err != nil {
				t.formMsg = "Não foi possível abrir a imagem: " + err.Error()
			} else if len(paths) > 0 {
				t.formImage = paths[0]
				t.formMsg = ""
			}
		})
	}()
}

func (t *GamesTab) saveForm() {
	if strings.TrimSpace(t.formGame.Name) == "" {
		t.formMsg = "Informe o nome do jogo."
		return
	}
	if t.formGame.MaxPlayers < t.formGame.MinPlayers {
		t.formMsg = "O máximo de jogadores deve ser igual ou maior que o mínimo."
		return
	}
	if t.saving {
		return
	}

	t.saving = true
	t.formMsg = ""
	go func() {
		imageName := t.formGame.Image
		if t.formImage != "" {
			name, err := t.state.ImageService.StoreCopy(t.formImage)
			if err != nil {
				t.state.Window.Update(func() {
					t.saving = false
					t.formMsg = fmt.Sprintf("Não foi possível salvar a imagem: %v", err)
				})
				return
			}
			imageName = name
		}

		game := t.formGame
		game.Name = strings.TrimSpace(game.Name)
		game.Image = imageName
		var err error
		if game.ID == 0 {
			err = t.actions.create(context.Background(), &game)
		} else {
			err = t.actions.update(context.Background(), &game)
		}
		t.runUpdate(func() {
			t.saving = false
			if err != nil {
				t.formMsg = "Não foi possível salvar o jogo: " + err.Error()
				return
			}
			t.showModal = false
			t.pageOK = true
			if t.formGame.ID == 0 {
				t.pageMsg = "Jogo cadastrado com sucesso."
			} else {
				t.pageMsg = "Jogo atualizado com sucesso."
			}
			t.loadGames()
		})
	}()
}

func (t *GamesTab) requestRemoval(game models.Game) {
	if t.removeBusy || t.actions.planRemoval == nil {
		return
	}
	t.removeBusy, t.pageMsg = true, ""
	go func() {
		plan, err := t.actions.planRemoval(context.Background(), game.ID)
		t.runUpdate(func() {
			t.removeBusy = false
			if err != nil {
				t.pageOK, t.pageMsg = false, "Não foi possível verificar o jogo: "+err.Error()
				return
			}
			if plan.Kind == models.RemovalBlocked {
				t.pageOK, t.pageMsg, t.showRemove = false, plan.Message, false
				return
			}
			t.removeTarget, t.removePlan, t.showRemove = game, plan, true
		})
	}()
}

func (t *GamesTab) renderRemoveModal(c *ui.Context) {
	copy := removalConfirmation(t.removePlan)
	components.ConfirmationModal(c, &t.showRemove, "Remover jogo", t.removePlan.Message, copy.ConfirmLabel, copy.Danger, t.removeBusy, t.confirmRemoval, func() { t.showRemove = false })
}

func (t *GamesTab) confirmRemoval() {
	if t.removeBusy || t.actions.remove == nil || t.removeTarget.ID == 0 {
		return
	}
	t.removeBusy = true
	go func() {
		plan, err := t.actions.remove(context.Background(), t.removeTarget.ID)
		t.runUpdate(func() {
			t.removeBusy = false
			if err != nil {
				t.pageOK, t.pageMsg = false, "Não foi possível remover o jogo: "+err.Error()
				return
			}
			t.showRemove, t.pageOK = false, true
			if plan.Kind == models.RemovalDelete {
				t.pageMsg = "Jogo excluído definitivamente."
			} else {
				t.pageMsg = "Jogo arquivado. Edite-o para reativar quando necessário."
			}
			t.loadGames()
		})
	}()
}

func (t *GamesTab) runUpdate(fn func()) {
	if t.update != nil {
		t.update(fn)
		return
	}
	fn()
}
