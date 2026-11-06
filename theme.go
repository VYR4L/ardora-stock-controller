package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/ardora/stockcontroller/internal/frontend/style"
)

func getArdoraTheme(dark bool) *ui.Theme {
	return style.NewTheme(dark)
}
