package components_test

import (
	"strings"
	"testing"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/components"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
)

func TestPanel_FocusedRoundedBorder(t *testing.T) {
	s := styles.New()
	out := components.Panel(s, "hello", true)

	roundedChars := "╭╮╰╯"
	found := false
	for _, ch := range roundedChars {
		if strings.ContainsRune(out, ch) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Panel(focused=true) output missing rounded border chars (╭╮╰╯), got: %q", out)
	}
}

func TestPanel_BlurredRoundedBorder(t *testing.T) {
	s := styles.New()
	out := components.Panel(s, "hello", false)

	roundedChars := "╭╮╰╯"
	found := false
	for _, ch := range roundedChars {
		if strings.ContainsRune(out, ch) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Panel(focused=false) output missing rounded border chars (╭╮╰╯), got: %q", out)
	}
}

func TestPanel_ContentVisible(t *testing.T) {
	s := styles.New()
	content := "unique-content-xyz"
	out := components.Panel(s, content, true)

	if !strings.Contains(out, content) {
		t.Errorf("Panel output missing content %q, got: %q", content, out)
	}
}
