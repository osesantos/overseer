package styles

import (
	"image/color"
	"testing"

	"github.com/lucasb-eyer/go-colorful"
)

var omarchyPalette = map[string]string{
	"mode":       "dark",
	"accent":     "#f38d70",
	"background": "#2c2525",
	"foreground": "#e6d9db",
	"red":        "#fd6883",
}

func hex(c color.Color) string {
	v, _ := colorful.MakeColor(c)
	return v.Hex()
}

func TestOmarchyThemeMapsThePalette(t *testing.T) {
	th := ResolveTheme("", omarchyPalette)
	if got := hex(th.Primary); got != "#f38d70" {
		t.Errorf("Primary: want accent #f38d70, got %s", got)
	}
	if got := hex(th.Text); got != "#e6d9db" {
		t.Errorf("Text: want foreground #e6d9db, got %s", got)
	}
	if got := hex(th.Danger); got != "#fd6883" {
		t.Errorf("Danger: want red #fd6883, got %s", got)
	}
	if got := hex(th.Warning); got != "#f38d70" {
		t.Errorf("Warning: a missing yellow should fall back to accent, got %s", got)
	}
	if hex(ResolveTheme("omarchy", omarchyPalette).Primary) != hex(th.Primary) {
		t.Error(`theme "omarchy" should resolve like an empty theme`)
	}
}

func TestResolveTheme_FallsBackToDark(t *testing.T) {
	dark := hex(DarkTheme().Primary)
	tests := map[string]map[string]string{
		"no omarchy": nil,
		"malformed":  {"accent": "[", "background": "[", "foreground": "["},
		"incomplete": {"accent": "#ffffff"},
	}
	for name, palette := range tests {
		t.Run(name, func(t *testing.T) {
			if got := hex(ResolveTheme("", palette).Primary); got != dark {
				t.Errorf("want dark primary %s, got %s", dark, got)
			}
		})
	}
}

func TestNamedThemeBeatsOmarchy(t *testing.T) {
	want := hex(NordTheme().Primary)
	if got := hex(ResolveTheme("nord", omarchyPalette).Primary); got != want {
		t.Errorf("named theme should win: want %s, got %s", want, got)
	}
}

func TestRepaint_RebuildsStylesInPlace(t *testing.T) {
	s := newStyles("", false, nil)
	before := hex(s.TitleBar.Base.GetBackground())

	s.Repaint(omarchyPalette)

	after := hex(s.TitleBar.Base.GetBackground())
	if after == before || after != "#f38d70" {
		t.Errorf("title bar background: want accent #f38d70 after repaint, got %s (was %s)", after, before)
	}
}
