package components

import (
	"github.com/egoist/mygo/ui"

	"github.com/ardora/stockcontroller/internal/frontend/style"
)

// ConfirmationModal presents one explicit confirmation for destructive actions.
func ConfirmationModal(c *ui.Context, open *bool, title, detail, confirmLabel string, danger, busy bool, confirm, cancel func()) {
	ui.Modal(c, open, func() {
		ui.Column(c).Gap(16).Width(520).MaxWidthPercent(90).Padding(8).Children(func() {
			SectionHeader(c, title, detail)
			ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
				if ui.Button(c, "Cancelar").Disabled(busy).Clicked() && cancel != nil {
					cancel()
				}
				button := ui.PrimaryButton(c, confirmLabel).Disabled(busy)
				if danger {
					button = button.Background(style.FromContext(c).Danger).TextColor(ui.Hex("#ffffff"))
				}
				if button.Clicked() && confirm != nil {
					confirm()
				}
			})
		})
	})
}
