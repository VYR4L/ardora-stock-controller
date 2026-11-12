package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLayoutIncludesCodeSheetsDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)
	layout, err := Resolve()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, AppName, "code-sheets")
	if layout.CodeSheets != want {
		t.Fatalf("CodeSheets = %q, want %q", layout.CodeSheets, want)
	}
	if err := layout.Ensure(); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(want); err != nil || !info.IsDir() {
		t.Fatalf("code sheets directory not created: %v, %v", info, err)
	}
}
