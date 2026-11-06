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

type StockTab struct {
	state *frontend.State

	loading  bool
	products []models.Product
	err      error
	query    string
	pageMsg  string

	showModal bool
	formItem  models.Product
	formImage string
	formMsg   string
	saving    bool
}

func NewStockTab(s *frontend.State) *StockTab {
	t := &StockTab{state: s}
	t.loadProducts()
	return t
}

func (t *StockTab) loadProducts() {
	t.loading = true
	go func() {
		products, err := t.state.ProductService.List(context.Background(), false)
		t.state.Window.Update(func() {
			t.products = products
			t.err = err
			t.loading = false
		})
	}()
}

func (t *StockTab) View(c *ui.Context) {
	components.Page(c, func() {
		components.PageHeader(c, "CATÁLOGO", "Estoque", "Acompanhe preços, disponibilidade e níveis mínimos dos produtos.", &components.Action{Label: "Novo produto", Run: t.openCreate})

		if t.pageMsg != "" {
			components.FeedbackBanner(c, t.pageMsg, components.ToneSuccess)
		}

		if t.loading {
			components.LoadingState(c, "Carregando produtos", "Preparando seu catálogo de estoque.")
		} else if t.err != nil {
			components.ErrorState(c, "Não foi possível carregar o estoque", t.err.Error(), &components.Action{Label: "Tentar novamente", Run: t.loadProducts})
		} else {
			t.renderContent(c)
		}
	})

	t.renderModal(c)
}

func (t *StockTab) renderContent(c *ui.Context) {
	active, attention := 0, 0
	for _, product := range t.products {
		if product.Active {
			active++
		}
		if productStockLevel(product) != stockHealthy {
			attention++
		}
	}

	ui.Row(c).Gap(12).Wrap().Children(func() {
		components.MetricCard(c, "Produtos", len(t.products), "Itens cadastrados", icons.Stock, nil)
		components.MetricCard(c, "Ativos", active, "Disponíveis para venda", icons.Check, nil)
		components.MetricCard(c, "Pedem atenção", attention, "No mínimo ou esgotados", icons.Warning, nil)
	})

	components.Panel(c, "Lista de produtos", "Busque pelo nome e confira os indicadores de estoque.", 0, func() {
		filtered := filterProducts(t.products, t.query)
		ui.Row(c).Gap(12).Wrap().AlignItems(ui.Center).Children(func() {
			ui.SearchField(c, &t.query).Width(300).Placeholder("Buscar por nome...").Label("Buscar produtos por nome")
			ui.Spacer(c)
			ui.Textf(c, "%d resultado(s)", len(filtered)).FontSize(12).TextColor(style.FromContext(c).Muted)
		})

		switch {
		case len(t.products) == 0:
			components.EmptyState(c, "Nenhum produto cadastrado", "Cadastre o primeiro item para começar a controlar o estoque.", &components.Action{Label: "Cadastrar produto", Run: t.openCreate})
		case len(filtered) == 0:
			components.EmptyState(c, "Nenhum produto encontrado", "Tente buscar por outro nome.", nil)
		default:
			ui.Column(c).Gap(8).Children(func() {
				for _, product := range filtered {
					t.renderProduct(c, product)
				}
			})
		}
	})
}

func (t *StockTab) renderProduct(c *ui.Context, product models.Product) {
	tokens := style.FromContext(c)
	imagePath := ""
	if t.state != nil && t.state.ImageService != nil {
		imagePath = t.state.ImageService.GetPath(product.Image)
	}

	ui.Row(c).Gap(14).Wrap().AlignItems(ui.Center).Padding(12).Background(tokens.SurfaceSubtle).Border(1, tokens.Border).Radius(12).Children(func() {
		components.Thumbnail(c, imagePath, "Imagem de "+product.Name, 54)
		ui.Column(c).Gap(3).Grow(1).MinWidth(190).Children(func() {
			ui.Text(c, product.Name).FontSize(15).FontWeight(700).TextColor(tokens.Text)
			category := product.Category
			if strings.TrimSpace(category) == "" {
				category = "Sem categoria"
			}
			ui.Text(c, category).FontSize(12).TextColor(tokens.Muted)
		})
		ui.Column(c).Gap(2).MinWidth(100).Children(func() {
			ui.Text(c, "Preço").FontSize(11).TextColor(tokens.Muted)
			ui.Textf(c, "R$ %.2f", float64(product.PriceCents)/100).FontWeight(700).TextColor(tokens.Text)
		})
		ui.Column(c).Gap(2).MinWidth(110).Children(func() {
			ui.Text(c, "Disponível / mínimo").FontSize(11).TextColor(tokens.Muted)
			ui.Textf(c, "%d / %d", product.Stock, product.MinStock).FontWeight(700).TextColor(tokens.Text)
		})
		ui.Row(c).Gap(6).Wrap().Children(func() {
			switch productStockLevel(product) {
			case stockDepleted:
				components.StatusBadge(c, "Esgotado", components.ToneDanger)
			case stockLow:
				components.StatusBadge(c, "Estoque baixo", components.ToneWarning)
			default:
				components.StatusBadge(c, "Estoque saudável", components.ToneSuccess)
			}
			if !product.Active {
				components.StatusBadge(c, "Inativo", components.ToneNeutral)
			}
		})
	})
}

func (t *StockTab) openCreate() {
	t.formItem = models.Product{Active: true, Category: "Outros"}
	t.formImage = ""
	t.formMsg = ""
	t.showModal = true
}

func (t *StockTab) renderModal(c *ui.Context) {
	ui.Modal(c, &t.showModal, func() {
		ui.Column(c).Gap(18).Width(680).MaxWidthPercent(88).Padding(8).Children(func() {
			components.SectionHeader(c, "Novo produto", "Preencha os dados que aparecerão no estoque.")
			if t.formMsg != "" {
				components.FeedbackBanner(c, t.formMsg, components.ToneDanger)
			}

			ui.Field(c, "Nome *", func() {
				ui.TextInput(c, &t.formItem.Name).Disabled(t.saving)
			})
			ui.Field(c, "Categoria *", func() {
				ui.TextInput(c, &t.formItem.Category).Disabled(t.saving)
			})
			ui.Row(c).Gap(12).Wrap().Children(func() {
				ui.Field(c, "Preço (R$)", func() {
					price := float64(t.formItem.PriceCents) / 100
					if ui.NumberInput(c, &price, 0, 99999, 0.1).Disabled(t.saving).Changed() {
						t.formItem.PriceCents = int64(price * 100)
					}
				}).MinWidth(160).Grow(1)
				ui.Field(c, "Estoque atual", func() {
					stock := float64(t.formItem.Stock)
					if ui.NumberInput(c, &stock, 0, 99999, 1).Disabled(t.saving).Changed() {
						t.formItem.Stock = int(stock)
					}
				}).MinWidth(160).Grow(1)
				ui.Field(c, "Estoque mínimo", func() {
					minimum := float64(t.formItem.MinStock)
					if ui.NumberInput(c, &minimum, 0, 99999, 1).Disabled(t.saving).Changed() {
						t.formItem.MinStock = int(minimum)
					}
				}).MinWidth(160).Grow(1)
			})

			ui.Field(c, "Imagem", func() {
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

func (t *StockTab) chooseImage() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:  t.state.Window,
			Title:   "Escolher imagem do produto",
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

func (t *StockTab) saveForm() {
	if strings.TrimSpace(t.formItem.Name) == "" {
		t.formMsg = "Informe o nome do produto."
		return
	}
	if strings.TrimSpace(t.formItem.Category) == "" {
		t.formMsg = "Informe a categoria do produto."
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

		item := t.formItem
		item.Name = strings.TrimSpace(item.Name)
		item.Category = strings.TrimSpace(item.Category)
		item.Image = imageName
		err := t.state.ProductService.Create(context.Background(), &item)
		t.state.Window.Update(func() {
			t.saving = false
			if err != nil {
				t.formMsg = "Não foi possível criar o produto: " + err.Error()
				return
			}
			t.showModal = false
			t.pageMsg = "Produto cadastrado com sucesso."
			t.loadProducts()
		})
	}()
}

func savingLabel(saving bool) string {
	if saving {
		return "Salvando..."
	}
	return "Salvar"
}
