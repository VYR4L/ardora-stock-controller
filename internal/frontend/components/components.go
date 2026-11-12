package components

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/ardora/stockcontroller/internal/frontend/icons"
	"github.com/ardora/stockcontroller/internal/frontend/style"
)

type Action struct {
	Label    string
	Disabled bool
	Run      func()
}

type Tone uint8

const (
	ToneNeutral Tone = iota
	ToneAccent
	ToneSuccess
	ToneWarning
	ToneDanger
)

func Page(c *ui.Context, body func()) ui.Element {
	return ui.Column(c).Gap(20).FillWidth().Children(body)
}

func PageHeader(c *ui.Context, eyebrow, title, subtitle string, action *Action) ui.Element {
	tokens := style.FromContext(c)
	return ui.Row(c).Gap(16).Wrap().AlignItems(ui.Center).Children(func() {
		ui.Column(c).Gap(4).Grow(1).MinWidth(280).Children(func() {
			if eyebrow != "" {
				ui.Text(c, eyebrow).FontSize(12).FontWeight(700).TextColor(tokens.Accent)
			}
			ui.Text(c, title).FontSize(26).FontWeight(760).TextColor(tokens.Text)
			if subtitle != "" {
				ui.Text(c, subtitle).FontSize(14).TextColor(tokens.Muted)
			}
		})
		if action != nil && action.Label != "" {
			button := ui.PrimaryButton(c, action.Label).Disabled(action.Disabled)
			if button.Clicked() && action.Run != nil {
				action.Run()
			}
		}
	})
}

func Panel(c *ui.Context, title, subtitle string, minWidth float32, body func()) ui.Element {
	tokens := style.FromContext(c)
	panel := ui.Column(c).Gap(16).Padding(20).Background(tokens.Surface).Border(1, tokens.Border).Radius(14)
	if minWidth > 0 {
		panel = panel.MinWidth(minWidth)
	}
	return panel.Children(func() {
		if title != "" {
			SectionHeader(c, title, subtitle)
		}
		body()
	})
}

func SectionHeader(c *ui.Context, title, subtitle string) ui.Element {
	tokens := style.FromContext(c)
	return ui.Column(c).Gap(3).Children(func() {
		ui.Text(c, title).FontSize(17).FontWeight(700).TextColor(tokens.Text)
		if subtitle != "" {
			ui.Text(c, subtitle).FontSize(13).TextColor(tokens.Muted)
		}
	})
}

func MetricCard(c *ui.Context, label string, value int, detail string, icon *ui.SVG, action *Action) ui.Element {
	tokens := style.FromContext(c)
	return ui.Column(c).Gap(14).Padding(18).MinWidth(220).Grow(1).Background(tokens.Surface).Border(1, tokens.Border).Radius(14).Shadow(0, 4, 16, 0, ui.Hex("#000000").Alpha(0.06)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
			if icon != nil {
				ui.Box(c).Size(36, 36).Radius(10).Background(tokens.AccentSoft).Children(func() {
					ui.Icon(c, icon).Size(18, 18).Margin(9).TextColor(tokens.Accent)
				})
			}
			ui.Text(c, label).FontSize(13).FontWeight(650).TextColor(tokens.Muted)
		})
		ui.Text(c, strconv.Itoa(value)).FontSize(34).FontWeight(780).TextColor(tokens.Text)
		if detail != "" {
			ui.Text(c, detail).FontSize(13).TextColor(tokens.Muted)
		}
		if action != nil && action.Label != "" {
			button := ui.Button(c, action.Label).Disabled(action.Disabled)
			if button.Clicked() && action.Run != nil {
				action.Run()
			}
		}
	})
}

func StatusBadge(c *ui.Context, label string, tone Tone) ui.Element {
	bg, fg, icon := toneStyle(c, tone)
	return ui.Row(c).Gap(5).Padding(4, 8).Radius(999).Background(bg).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icon).Size(13, 13).TextColor(fg)
		ui.Text(c, label).FontSize(12).FontWeight(650).TextColor(fg)
	})
}

func FormActions(c *ui.Context, primary Action, secondary *Action) ui.Element {
	return ui.Row(c).Gap(10).Wrap().AlignItems(ui.Center).Children(func() {
		button := ui.PrimaryButton(c, primary.Label).Disabled(primary.Disabled)
		if button.Clicked() && primary.Run != nil {
			primary.Run()
		}
		if secondary != nil {
			button := ui.Button(c, secondary.Label).Disabled(secondary.Disabled)
			if button.Clicked() && secondary.Run != nil {
				secondary.Run()
			}
		}
	})
}

func FeedbackBanner(c *ui.Context, message string, tone Tone) ui.Element {
	if message == "" {
		return ui.Box(c)
	}
	bg, fg, icon := toneStyle(c, tone)
	return ui.Row(c).Gap(8).Padding(10, 12).Radius(10).Background(bg).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icon).Size(16, 16).TextColor(fg)
		ui.Text(c, message).FontSize(13).TextColor(fg).Grow(1)
	})
}

func LoadingState(c *ui.Context, title, detail string) ui.Element {
	tokens := style.FromContext(c)
	return stateCard(c, func() {
		ui.Spinner(c)
		ui.Text(c, title).FontWeight(700).TextColor(tokens.Text)
		if detail != "" {
			ui.Text(c, detail).FontSize(13).TextColor(tokens.Muted)
		}
	})
}

func EmptyState(c *ui.Context, title, detail string, action *Action) ui.Element {
	tokens := style.FromContext(c)
	return stateCard(c, func() {
		ui.Icon(c, icons.Empty).Size(28, 28).TextColor(tokens.Muted)
		ui.Text(c, title).FontSize(16).FontWeight(700).TextColor(tokens.Text)
		if detail != "" {
			ui.Text(c, detail).FontSize(13).TextColor(tokens.Muted)
		}
		if action != nil && action.Label != "" {
			button := ui.PrimaryButton(c, action.Label).Disabled(action.Disabled)
			if button.Clicked() && action.Run != nil {
				action.Run()
			}
		}
	})
}

func ErrorState(c *ui.Context, title, detail string, retry *Action) ui.Element {
	tokens := style.FromContext(c)
	return stateCard(c, func() {
		ui.Icon(c, icons.Error).Size(28, 28).TextColor(tokens.Danger)
		ui.Text(c, title).FontSize(16).FontWeight(700).TextColor(tokens.Text)
		if detail != "" {
			ui.Text(c, detail).FontSize(13).TextColor(tokens.Muted)
		}
		if retry != nil && retry.Label != "" {
			button := ui.Button(c, retry.Label).Disabled(retry.Disabled)
			if button.Clicked() && retry.Run != nil {
				retry.Run()
			}
		}
	})
}

func stateCard(c *ui.Context, body func()) ui.Element {
	tokens := style.FromContext(c)
	return ui.Column(c).Gap(10).Padding(28).FillWidth().MinHeight(180).AlignItems(ui.Center).Justify(ui.Center).Background(tokens.Surface).Border(1, tokens.Border).Radius(14).Children(body)
}

func toneStyle(c *ui.Context, tone Tone) (ui.Color, ui.Color, *ui.SVG) {
	tokens := style.FromContext(c)
	switch tone {
	case ToneAccent:
		return tokens.AccentSoft, tokens.Accent, icons.Info
	case ToneSuccess:
		return tokens.SuccessSoft, tokens.Success, icons.Check
	case ToneWarning:
		return tokens.WarningSoft, tokens.Warning, icons.Warning
	case ToneDanger:
		return tokens.DangerSoft, tokens.Danger, icons.Error
	default:
		return tokens.NeutralSoft, tokens.Muted, icons.Info
	}
}
