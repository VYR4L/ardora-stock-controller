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
	state *frontend.State

	loading bool
	games   []models.Game
	err     error
	query   string
	pageMsg string

	showModal bool
	formGame  models.Game
	formImage string
	formMsg   string
	saving    bool
}

func NewGamesTab(s *frontend.State) *GamesTab {
	t := &GamesTab{state: s}
	t.loadGames()
	return t
}

func (t *GamesTab) loadGames() {
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
			components.FeedbackBanner(c, t.pageMsg, components.ToneSuccess)
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
	})
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
			components.SectionHeader(c, "Novo jogo", "Cadastre um título e sua faixa de jogadores.")
			if t.formMsg != "" {
				components.FeedbackBanner(c, t.formMsg, components.ToneDanger)
			}

			ui.Field(c, "Nome *", func() {
				ui.TextInput(c, &t.formGame.Name).Disabled(t.saving)
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

			ui.Field(c, "Capa do jogo", func() {
				ui.Row(c).Gap(10).Wrap().AlignItems(ui.Center).Children(func() {
					name := "Nenhum arquivo selecionado"
					if t.formImage != "" {
						name = filepath.Base(t.formImage)
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
		imageName := ""
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
		err := t.state.GameService.Create(context.Background(), &game)
		t.state.Window.Update(func() {
			t.saving = false
			if err != nil {
				t.formMsg = "Não foi possível criar o jogo: " + err.Error()
				return
			}
			t.showModal = false
			t.pageMsg = "Jogo cadastrado com sucesso."
			t.loadGames()
		})
	}()
}
