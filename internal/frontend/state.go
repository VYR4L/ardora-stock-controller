package frontend

import (
	"github.com/ardora/stockcontroller/internal/backend/paths"
	"github.com/ardora/stockcontroller/internal/backend/services"
	"github.com/egoist/mygo"
)

// State holds all dependencies needed by the UI tabs.
type State struct {
	Window               *mygo.Window
	Layout               paths.Layout
	ImageService         *services.ImageService
	GameService          *services.GameService
	ProductService       *services.ProductService
	SalesService         *services.SalesService
	VisitService         *services.VisitService
	ReceiptService       *services.ReceiptService
	TableService         *services.TableService
	TablesClientsService *services.TablesClientsService
	TableGameService     *services.TableGameService
	Navigate             func(destination string)
}
