// Package paths resolves where Ardora keeps its local data.
//
// Everything lives under %LOCALAPPDATA%\Ardora on Windows (the user cache
// directory on other systems), so the application keeps working offline and
// survives the user deleting the original files they imported.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// AppName is the folder name used under the local application data dir.
const AppName = "Ardora"

// Layout holds the absolute directories and files used by the application.
type Layout struct {
	Root       string // %LOCALAPPDATA%\Ardora
	Images     string // copies of product and game pictures
	Backups    string // daily CSV reports
	Receipts   string // printable non-fiscal receipts
	CodeSheets string // printable QR and barcode command sheets
	Database   string // SQLite database file
}

// Resolve returns the layout without touching the disk.
func Resolve() (Layout, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		dir, err := os.UserCacheDir()
		if err != nil {
			return Layout{}, fmt.Errorf("localizar AppData: %w", err)
		}
		base = dir
	}
	root := filepath.Join(base, AppName)
	return Layout{
		Root:       root,
		Images:     filepath.Join(root, "images"),
		Backups:    filepath.Join(root, "backups"),
		Receipts:   filepath.Join(root, "receipts"),
		CodeSheets: filepath.Join(root, "code-sheets"),
		Database:   filepath.Join(root, "ardora.db"),
	}, nil
}

// Ensure creates every directory of the layout when missing.
func (l Layout) Ensure() error {
	for _, dir := range []string{l.Root, l.Images, l.Backups, l.Receipts, l.CodeSheets} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("criar %s: %w", dir, err)
		}
	}
	return nil
}
