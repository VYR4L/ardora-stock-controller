package icons

import "github.com/egoist/mygo/ui"

func parse(body string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`))
}

var (
	Home       = parse(`<path d="M3 11.5 12 4l9 7.5"/><path d="M5.5 10v10h13V10"/><path d="M9.5 20v-6h5v6"/>`)
	Sales      = parse(`<path d="M5 4h14v16H5z"/><path d="M8 8h8M8 12h5M8 16h3"/><path d="M16 14v4M14 16h4"/>`)
	Tables     = parse(`<rect x="4" y="5" width="16" height="10" rx="2"/><path d="M7 15v4M17 15v4M8 9h8"/>`)
	Stock      = parse(`<path d="m4 7 8-4 8 4-8 4z"/><path d="M4 7v10l8 4 8-4V7M12 11v10"/>`)
	Games      = parse(`<path d="M8 8h8a5 5 0 0 1 4.7 6.7l-1 2.8a2.2 2.2 0 0 1-3.8.6L14.3 16H9.7l-1.6 2.1a2.2 2.2 0 0 1-3.8-.6l-1-2.8A5 5 0 0 1 8 8Z"/><path d="M8 11v4M6 13h4M16.5 12.5h.01M18 14h.01"/>`)
	Settings   = parse(`<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1-2.8 2.8-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.6v.2h-4V21a1.7 1.7 0 0 0-1-1.6 1.7 1.7 0 0 0-1.9.3l-.1.1L4.2 17l.1-.1a1.7 1.7 0 0 0 .3-1.9A1.7 1.7 0 0 0 3 14H2.8v-4H3a1.7 1.7 0 0 0 1.6-1 1.7 1.7 0 0 0-.3-1.9L4.2 7 7 4.2l.1.1A1.7 1.7 0 0 0 9 4.6a1.7 1.7 0 0 0 1-1.6v-.2h4V3a1.7 1.7 0 0 0 1 1.6 1.7 1.7 0 0 0 1.9-.3l.1-.1L19.8 7l-.1.1a1.7 1.7 0 0 0-.3 1.9 1.7 1.7 0 0 0 1.6 1h.2v4H21a1.7 1.7 0 0 0-1.6 1Z"/>`)
	Check      = parse(`<path d="m5 12 4 4L19 6"/>`)
	Warning    = parse(`<path d="M12 3 2.8 20h18.4L12 3Z"/><path d="M12 9v4M12 17h.01"/>`)
	Error      = parse(`<circle cx="12" cy="12" r="9"/><path d="m9 9 6 6M15 9l-6 6"/>`)
	Info       = parse(`<circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7h.01"/>`)
	Empty      = parse(`<path d="M4 7h16v12H4z"/><path d="m4 7 3-3h10l3 3M9 12h6"/>`)
	ArrowRight = parse(`<path d="M5 12h14M14 7l5 5-5 5"/>`)
	Image      = parse(`<rect x="3" y="4" width="18" height="16" rx="2"/><circle cx="9" cy="10" r="2"/><path d="m4 18 5-5 3 3 2-2 6 4"/>`)
)
