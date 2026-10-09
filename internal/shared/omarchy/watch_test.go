package omarchy

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatch_ThemeDirSwap_Signals(t *testing.T) {
	dir := t.TempDir()
	ch := Watch(dir)
	if ch == nil {
		t.Fatal("Watch() returned nil for an existing dir")
	}
	next := filepath.Join(dir, "next-theme")
	if err := os.MkdirAll(next, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.Rename(next, filepath.Join(dir, "theme")); err != nil {
		t.Fatal(err)
	}

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Error("no signal after the theme directory was swapped in")
	}
}

func TestWatch_UnrelatedFile_NoSignal(t *testing.T) {
	dir := t.TempDir()
	ch := Watch(dir)

	if err := os.WriteFile(filepath.Join(dir, "other.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-ch:
		t.Error("unrelated file change should not signal a theme switch")
	case <-time.After(500 * time.Millisecond):
	}
}
