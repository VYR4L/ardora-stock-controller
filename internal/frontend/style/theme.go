package style

import "github.com/egoist/mygo/ui"

// Tokens contains colors used by composed Ardora components in addition to
// the widget colors exposed by ui.Theme.
type Tokens struct {
	Canvas        ui.Color
	Surface       ui.Color
	SurfaceSubtle ui.Color
	SurfaceRaised ui.Color
	Border        ui.Color
	Text          ui.Color
	Muted         ui.Color
	Accent        ui.Color
	AccentSoft    ui.Color
	AccentText    ui.Color
	Success       ui.Color
	SuccessSoft   ui.Color
	Warning       ui.Color
	WarningSoft   ui.Color
	Danger        ui.Color
	DangerSoft    ui.Color
	NeutralSoft   ui.Color
	Sidebar       ui.Color
}

// NewTheme returns the widget theme used by the application.
func NewTheme(dark bool) *ui.Theme {
	tokens := TokensFor(dark)
	var theme *ui.Theme
	if dark {
		theme = ui.DarkTheme()
		theme.SurfaceHover = ui.Hex("#31434B")
		theme.SurfacePressed = ui.Hex("#3A4D55")
		theme.AccentHover = ui.Hex("#FFC592")
		theme.AccentPressed = ui.Hex("#E89859")
		theme.Selection = ui.Hex("#F3AD72").Alpha(0.30)
		theme.Focus = ui.Hex("#FFC592").Alpha(0.80)
		theme.Scrollbar = ui.Hex("#D7E0E4").Alpha(0.38)
	} else {
		theme = ui.LightTheme()
		theme.SurfaceHover = ui.Hex("#E9E2DB")
		theme.SurfacePressed = ui.Hex("#DED5CC")
		theme.AccentHover = ui.Hex("#314047")
		theme.AccentPressed = ui.Hex("#26343A")
		theme.Selection = ui.Hex("#FCC399").Alpha(0.48)
		theme.Focus = ui.Hex("#B95F25").Alpha(0.70)
		theme.Scrollbar = ui.Hex("#3D4C54").Alpha(0.30)
	}

	theme.Background = tokens.Canvas
	theme.Surface = tokens.SurfaceSubtle
	theme.Border = tokens.Border
	theme.Text = tokens.Text
	theme.TextMuted = tokens.Muted
	theme.Accent = tokens.Accent
	theme.AccentText = tokens.AccentText
	theme.Success = tokens.Success
	theme.Warning = tokens.Warning
	theme.Danger = tokens.Danger
	theme.Radius = 10
	theme.Spacing = 4
	theme.FontSize = 14
	return theme
}

// TokensFor returns Ardora's semantic palette for an appearance.
func TokensFor(dark bool) Tokens {
	if dark {
		return Tokens{
			Canvas:        ui.Hex("#131C20"),
			Surface:       ui.Hex("#1C292F"),
			SurfaceSubtle: ui.Hex("#26353C"),
			SurfaceRaised: ui.Hex("#304149"),
			Border:        ui.Hex("#405159"),
			Text:          ui.Hex("#F7F3EF"),
			Muted:         ui.Hex("#B7C1C6"),
			Accent:        ui.Hex("#F3AD72"),
			AccentSoft:    ui.Hex("#F3AD72").Alpha(0.16),
			AccentText:    ui.Hex("#182126"),
			Success:       ui.Hex("#63D6A6"),
			SuccessSoft:   ui.Hex("#2B7B5D").Alpha(0.34),
			Warning:       ui.Hex("#FFC067"),
			WarningSoft:   ui.Hex("#A76522").Alpha(0.34),
			Danger:        ui.Hex("#FF8A8A"),
			DangerSoft:    ui.Hex("#A63E46").Alpha(0.34),
			NeutralSoft:   ui.Hex("#D7E0E4").Alpha(0.10),
			Sidebar:       ui.Hex("#18252A"),
		}
	}

	return Tokens{
		Canvas:        ui.Hex("#F7F4F0"),
		Surface:       ui.Hex("#FFFFFF"),
		SurfaceSubtle: ui.Hex("#F1ECE6"),
		SurfaceRaised: ui.Hex("#FFFFFF"),
		Border:        ui.Hex("#DDD4CB"),
		Text:          ui.Hex("#253238"),
		Muted:         ui.Hex("#66737A"),
		Accent:        ui.Hex("#3D4C54"),
		AccentSoft:    ui.Hex("#FCC399").Alpha(0.45),
		AccentText:    ui.Hex("#FFFFFF"),
		Success:       ui.Hex("#1F7551"),
		SuccessSoft:   ui.Hex("#DDF2E8"),
		Warning:       ui.Hex("#9A5516"),
		WarningSoft:   ui.Hex("#FFF0D7"),
		Danger:        ui.Hex("#B83A3A"),
		DangerSoft:    ui.Hex("#FBE2E2"),
		NeutralSoft:   ui.Hex("#E9EEF0"),
		Sidebar:       ui.Hex("#EAE3DC"),
	}
}

// FromContext returns the semantic palette matching the active context theme.
func FromContext(c *ui.Context) Tokens {
	return TokensFor(c.Theme().Dark)
}
