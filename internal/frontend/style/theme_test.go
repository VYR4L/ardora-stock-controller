package style

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestPaletteContrast(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			tokens := TokensFor(dark)
			pairs := []struct {
				name       string
				foreground ui.Color
				background ui.Color
			}{
				{name: "canvas text", foreground: tokens.Text, background: tokens.Canvas},
				{name: "surface text", foreground: tokens.Text, background: tokens.Surface},
				{name: "muted text", foreground: tokens.Muted, background: tokens.Surface},
				{name: "primary action", foreground: tokens.AccentText, background: tokens.Accent},
				{name: "success status", foreground: tokens.Success, background: composite(tokens.SuccessSoft, tokens.Surface)},
				{name: "warning status", foreground: tokens.Warning, background: composite(tokens.WarningSoft, tokens.Surface)},
				{name: "danger status", foreground: tokens.Danger, background: composite(tokens.DangerSoft, tokens.Surface)},
			}
			for _, pair := range pairs {
				if ratio := contrast(pair.foreground, pair.background); ratio < 4.5 {
					t.Errorf("%s contrast %.2f is below 4.5", pair.name, ratio)
				}
			}
		})
	}
}

func composite(foreground, background ui.Color) ui.Color {
	a := float64(foreground.A) / 255
	return ui.RGB(
		uint8(math.Round(float64(foreground.R)*a+float64(background.R)*(1-a))),
		uint8(math.Round(float64(foreground.G)*a+float64(background.G)*(1-a))),
		uint8(math.Round(float64(foreground.B)*a+float64(background.B)*(1-a))),
	)
}

func contrast(a, b ui.Color) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func luminance(color ui.Color) float64 {
	channel := func(value uint8) float64 {
		v := float64(value) / 255
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(color.R) + 0.7152*channel(color.G) + 0.0722*channel(color.B)
}
