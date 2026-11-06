package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/ardora/stockcontroller/internal/frontend/components"
	"github.com/ardora/stockcontroller/internal/frontend/style"
)

func TestRenderAppShellAtSupportedSizes(t *testing.T) {
	sizes := [][2]int{{1024, 768}, {800, 600}}
	for _, dark := range []bool{false, true} {
		for _, size := range sizes {
			name := fmt.Sprintf("dark=%t/%dx%d", dark, size[0], size[1])
			t.Run(name, func(t *testing.T) {
				selected := "Home"
				image := ui.Render(func(c *ui.Context) {
					c.SetTheme(style.NewTheme(dark))
					renderAppShell(c, &selected, func() {
						components.Page(c, func() {
							components.PageHeader(c, "VISÃO GERAL", "Painel de teste", "Verificação do shell em tamanho suportado.", &components.Action{Label: "Ação principal"})
							components.EmptyState(c, "Conteúdo de teste", "A navegação e a ação principal devem permanecer visíveis.", nil)
						})
					})
				}, size[0], size[1], 1)
				if image == nil || image.Bounds().Dx() != size[0] || image.Bounds().Dy() != size[1] {
					t.Fatalf("unexpected rendered image bounds: %v", image)
				}
				writeMainPreview(t, name, image)
			})
		}
	}
}

func writeMainPreview(t *testing.T, name string, rendered *image.RGBA) {
	directory := os.Getenv("ARDORA_PREVIEW_DIR")
	if directory == "" {
		return
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("create preview directory: %v", err)
	}
	filename := "shell_" + strings.NewReplacer("/", "_", "=", "-", "\\", "_").Replace(name) + ".png"
	file, err := os.Create(filepath.Join(directory, filename))
	if err != nil {
		t.Fatalf("create preview: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, rendered); err != nil {
		t.Fatalf("encode preview: %v", err)
	}
}
