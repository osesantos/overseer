package omarchy

import (
	"os"
	"path/filepath"
	"testing"
)

func writeColors(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "theme"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "theme", "colors.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPalette_ReadsFlatKeys(t *testing.T) {
	dir := writeColors(t, "mode = \"dark\"\naccent = \"#f38d70\"\n")

	p, ok := Palette(dir)

	if !ok || p["accent"] != "#f38d70" || p["mode"] != "dark" {
		t.Errorf("Palette() = %v, %v; want accent and mode parsed", p, ok)
	}
}

func TestPalette_MissingOrEmptyFile_ReturnsFalse(t *testing.T) {
	if _, ok := Palette(t.TempDir()); ok {
		t.Error("Palette() on a dir without colors.toml should return false")
	}
	if _, ok := Palette(""); ok {
		t.Error("Palette(\"\") should return false")
	}
}

func TestDir_HonoursXDGStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/custom/state")

	if got, want := Dir(), "/custom/state/omarchy/current"; got != want {
		t.Errorf("Dir() = %q, want %q", got, want)
	}
}
