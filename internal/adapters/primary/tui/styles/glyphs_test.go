package styles_test

import (
	"reflect"
	"testing"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
)

func TestNewGlyphs_EmojiMode_AllFieldsPopulated(t *testing.T) {
	g := styles.NewGlyphs(false)
	v := reflect.ValueOf(g)
	for i := 0; i < v.NumField(); i++ {
		name := v.Type().Field(i).Name
		if v.Field(i).String() == "" {
			t.Errorf("NewGlyphs(false): %s is empty", name)
		}
	}
}

func TestNewGlyphs_FallbackMode_AllFieldsPopulated(t *testing.T) {
	g := styles.NewGlyphs(true)
	v := reflect.ValueOf(g)
	for i := 0; i < v.NumField(); i++ {
		name := v.Type().Field(i).Name
		if v.Field(i).String() == "" {
			t.Errorf("NewGlyphs(true): %s is empty", name)
		}
	}
}

func TestNewGlyphs_EmojiAndFallback_DifferPerSemanticField(t *testing.T) {
	emoji := styles.NewGlyphs(false)
	fallback := styles.NewGlyphs(true)

	allowSameAsFallback := map[string]struct{}{
		"PRStateClosed": {},
		"PRStateMerged": {},
		"PRStateDraft":  {},
	}

	ev := reflect.ValueOf(emoji)
	fv := reflect.ValueOf(fallback)
	for i := 0; i < ev.NumField(); i++ {
		name := ev.Type().Field(i).Name
		if _, ok := allowSameAsFallback[name]; ok {
			continue
		}
		if ev.Field(i).String() == fv.Field(i).String() {
			t.Errorf("%s: emoji and fallback are identical (%q); expected different glyphs", name, ev.Field(i).String())
		}
	}
}

func TestNewGlyphs_FallbackMode_AllPRStatesShareSingleDot(t *testing.T) {
	g := styles.NewGlyphs(true)
	if g.PRStateOpen != g.PRStateClosed || g.PRStateOpen != g.PRStateMerged || g.PRStateOpen != g.PRStateDraft {
		t.Errorf("fallback PR state glyphs must all share the same dot; got open=%q closed=%q merged=%q draft=%q",
			g.PRStateOpen, g.PRStateClosed, g.PRStateMerged, g.PRStateDraft)
	}
}

func TestGlyphs_LabelGlyph_KnownDefaultCodes_ReturnNonEmpty(t *testing.T) {
	codes := []string{"WIP", "testing", "ready", "done", "draft"}

	for _, disableEmoji := range []bool{false, true} {
		g := styles.NewGlyphs(disableEmoji)
		for _, code := range codes {
			if g.LabelGlyph(code) == "" {
				t.Errorf("LabelGlyph(%q) with disableEmoji=%v: got empty, want non-empty default", code, disableEmoji)
			}
		}
	}
}

func TestGlyphs_LabelGlyph_UnknownCode_ReturnsEmpty(t *testing.T) {
	g := styles.NewGlyphs(false)
	if got := g.LabelGlyph("blocked"); got != "" {
		t.Errorf("LabelGlyph(%q) = %q, want empty (user-defined codes have no built-in default)", "blocked", got)
	}
}

func TestGlyphs_LabelGlyph_EmojiAndFallback_DifferPerKnownCode(t *testing.T) {
	emoji := styles.NewGlyphs(false)
	fallback := styles.NewGlyphs(true)

	for _, code := range []string{"WIP", "testing", "ready", "done", "draft"} {
		if emoji.LabelGlyph(code) == fallback.LabelGlyph(code) {
			t.Errorf("LabelGlyph(%q): emoji and fallback are identical (%q); expected different glyphs", code, emoji.LabelGlyph(code))
		}
	}
}
