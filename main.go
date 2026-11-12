package main

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/ardora/stockcontroller/internal/backend/database"
	"github.com/ardora/stockcontroller/internal/backend/paths"
	"github.com/ardora/stockcontroller/internal/backend/services"
	"github.com/ardora/stockcontroller/internal/frontend"
	"github.com/ardora/stockcontroller/internal/frontend/components"
	"github.com/ardora/stockcontroller/internal/frontend/icons"
	"github.com/ardora/stockcontroller/internal/frontend/style"
	"github.com/ardora/stockcontroller/internal/frontend/tabs"
)

type App struct {
	loading bool
	err     error
	db      *sql.DB
	layout  paths.Layout

	configService *services.ConfigService
	themeDark     bool

	// Services
	imageService           *services.ImageService
	gameService            *services.GameService
	productService         *services.ProductService
	salesService           *services.SalesService
	visitService           *services.VisitService
	receiptService         *services.ReceiptService
	accessCodeService      *services.AccessCodeService
	accessCodePrintService *services.AccessCodePrintService
	tableService           *services.TableService
	tablesClientsService   *services.TablesClientsService
	tableGameService       *services.TableGameService

	// UI State
	window      *mygo.Window
	selectedTab string
	lastTab     string

	// Tabs
	homeTab     *tabs.HomeTab
	gamesTab    *tabs.GamesTab
	stockTab    *tabs.StockTab
	salesTab    *tabs.SalesTab
	codesTab    *tabs.CodesTab
	tablesTab   *tabs.TablesTab
	settingsTab *tabs.SettingsTab
}

func main() {
	app := &App{
		loading:     true,
		selectedTab: "Home",
	}

	mygo.App.WhenReady(func() {
		app.window = mygo.NewWindow(mygo.WindowOptions{
			Title:   "Ardora",
			Width:   1024,
			Height:  768,
			Content: ui.View(app.view),
		})

		// Background initialization
		go func() {
			err := app.init()
			app.window.Update(func() {
				if err != nil {
					app.err = err
				} else {
					app.loading = false
				}
			})
		}()
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func (a *App) init() error {
	// Simulate loading time so the user can see the splash screen
	time.Sleep(500 * time.Millisecond)

	layout, err := paths.Resolve()
	if err != nil {
		return err
	}
	if err := layout.Ensure(); err != nil {
		return err
	}
	a.layout = layout

	db, err := database.Open(context.Background(), layout.Database)
	if err != nil {
		return err
	}
	a.db = db

	a.configService = services.NewConfigService(db)
	a.imageService = services.NewImageService(layout.Images)
	a.gameService = services.NewGameService(db)
	a.productService = services.NewProductService(db)
	a.salesService = services.NewSalesService(db)
	a.visitService = services.NewVisitService(db)
	a.receiptService = services.NewReceiptService(db, layout.Receipts)
	a.accessCodeService = services.NewAccessCodeService(db)
	a.accessCodePrintService = services.NewAccessCodePrintService(db, layout.CodeSheets)
	a.tableService = services.NewTableService(db)
	a.tablesClientsService = services.NewTablesClientsService(db)
	a.tableGameService = services.NewTableGameService(db)
	backupService := services.NewBackupService(db, layout.Backups)

	dark, _ := a.configService.Get(context.Background(), "theme_dark")
	a.themeDark = (dark == "true")

	// Init tabs
	state := &frontend.State{
		Window:                 a.window,
		Layout:                 a.layout,
		ImageService:           a.imageService,
		GameService:            a.gameService,
		ProductService:         a.productService,
		SalesService:           a.salesService,
		VisitService:           a.visitService,
		ReceiptService:         a.receiptService,
		AccessCodeService:      a.accessCodeService,
		AccessCodePrintService: a.accessCodePrintService,
		TableService:           a.tableService,
		TablesClientsService:   a.tablesClientsService,
		TableGameService:       a.tableGameService,
		Navigate: func(destination string) {
			a.selectedTab = destination
		},
	}

	a.homeTab = tabs.NewHomeTab(state)
	a.gamesTab = tabs.NewGamesTab(state)
	a.stockTab = tabs.NewStockTab(state)
	a.salesTab = tabs.NewSalesTab(state)
	a.codesTab = tabs.NewCodesTab(state)
	a.tablesTab = tabs.NewTablesTab(state)
	a.settingsTab = tabs.NewSettingsTab(state, a.configService, backupService)

	return nil
}

func (a *App) view(c *ui.Context) {
	c.SetTheme(getArdoraTheme(a.themeDark))
	tokens := style.FromContext(c)

	if a.loading {
		ui.Column(c).Fill().Padding(40).Background(tokens.Canvas).Justify(ui.Center).Children(func() {
			components.LoadingState(c, "Preparando a Ardora", "Carregando dados e configurações da luderia...").MaxWidth(480).AlignSelf(ui.Center)
		})
		return
	}

	if a.err != nil {
		ui.Column(c).Fill().Padding(40).Background(tokens.Canvas).Justify(ui.Center).Children(func() {
			components.ErrorState(c, "Não foi possível iniciar", a.err.Error(), nil).MaxWidth(520).AlignSelf(ui.Center)
		})
		return
	}

	renderAppShell(c, &a.selectedTab, func() {
		a.viewSelectedTab(c)
	})
}

func renderAppShell(c *ui.Context, selectedTab *string, content func()) ui.Element {
	tokens := style.FromContext(c)
	return ui.Row(c).Fill().Background(tokens.Canvas).Children(func() {
		ui.Column(c).FillHeight().Width(208).Padding(12).Gap(14).Background(tokens.Sidebar).BorderWidth(0, 1, 0, 0).BorderColor(tokens.Border).Children(func() {
			ui.Row(c).Gap(10).Padding(10, 8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(38, 38).Radius(11).Background(tokens.Accent).Children(func() {
					ui.Icon(c, icons.Games).Size(20, 20).Margin(9).TextColor(tokens.AccentText)
				})
				ui.Column(c).Gap(1).Children(func() {
					ui.Text(c, "ARDORA").FontSize(16).FontWeight(800).TextColor(tokens.Text)
					ui.Text(c, "Gestão da luderia").FontSize(11).TextColor(tokens.Muted)
				})
			})

			ui.Text(c, "NAVEGAÇÃO").FontSize(10).FontWeight(750).TextColor(tokens.Muted).Margin(4, 10, 0, 10)
			ui.Sidebar(c, selectedTab, func() {
				ui.SidebarItem(c, "Home", icons.Home, "Visão geral")
				ui.SidebarItem(c, "Sales", icons.Sales, "Vendas")
				ui.SidebarItem(c, "Codes", icons.Codes, "Códigos")
				ui.SidebarItem(c, "Tables", icons.Tables, "Mesas")
				ui.SidebarItem(c, "Stock", icons.Stock, "Estoque")
				ui.SidebarItem(c, "Games", icons.Games, "Jogos")
				ui.SidebarItem(c, "Settings", icons.Settings, "Configurações")
			}).Grow(1).Background(ui.Transparent)

			ui.Column(c).Gap(2).Padding(10).Radius(10).Background(tokens.NeutralSoft).Children(func() {
				ui.Text(c, "Ardora Luderia").FontSize(12).FontWeight(700).TextColor(tokens.Text)
				ui.Text(c, "Controle local e offline").FontSize(10).TextColor(tokens.Muted)
			})
		})

		ui.Column(c).Grow(1).FillHeight().Children(func() {
			ui.Scroll(c).Grow(1).Padding(24).Children(func() {
				content()
			})
		})
	})
}

func (a *App) viewSelectedTab(c *ui.Context) {
	if a.selectedTab != a.lastTab {
		a.lastTab = a.selectedTab
		switch a.selectedTab {
		case "Sales":
			a.salesTab.Refresh()
		case "Codes":
			a.codesTab.Refresh()
		case "Tables":
			a.tablesTab.Refresh()
		case "Stock":
			a.stockTab.Refresh()
		}
	}
	switch a.selectedTab {
	case "Home":
		a.homeTab.View(c)
	case "Games":
		a.gamesTab.View(c)
	case "Stock":
		a.stockTab.View(c)
	case "Sales":
		a.salesTab.View(c)
	case "Codes":
		a.codesTab.View(c)
	case "Tables":
		a.tablesTab.View(c)
	case "Settings":
		a.settingsTab.View(c)
	default:
		components.EmptyState(c, "Área indisponível", "Selecione uma opção na navegação.", nil)
	}
}
