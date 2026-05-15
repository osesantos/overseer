package help_test

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/help"
	"github.com/muesli/termenv"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}

func TestRegistry_RegisterAndRetrieve(t *testing.T) {
	reg := help.NewRegistry()
	bindings := []key.Binding{
		key.NewBinding(key.WithKeys("j"), key.WithHelp("j", "down")),
		key.NewBinding(key.WithKeys("k"), key.WithHelp("k", "up")),
	}
	reg.RegisterPane("sessions", bindings)

	got := reg.BindingsFor("sessions")
	if len(got) < 2 {
		t.Fatalf("expected at least 2 bindings, got %d", len(got))
	}

	foundJ := false
	for _, b := range got {
		if b.Help().Key == "j" {
			foundJ = true
			break
		}
	}
	if !foundJ {
		t.Error("expected 'j' binding in BindingsFor(sessions)")
	}
}

func TestRegistry_GlobalsAlwaysPresent(t *testing.T) {
	reg := help.NewRegistry()

	got := reg.BindingsFor("nonexistent-pane")
	if len(got) == 0 {
		t.Fatal("expected global bindings for unknown pane, got none")
	}

	foundQuit := false
	for _, b := range got {
		if b.Help().Key == "q" {
			foundQuit = true
			break
		}
	}
	if !foundQuit {
		t.Error("expected 'q' quit binding in globals")
	}
}

func TestHelp_ActivePaneOnly(t *testing.T) {
	reg := help.NewRegistry()
	reg.RegisterPane("sessions", []key.Binding{
		key.NewBinding(key.WithKeys("j"), key.WithHelp("j", "down")),
	})
	reg.RegisterPane("preview", []key.Binding{
		key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "scroll up")),
	})

	bar := help.NewHelpBar(reg)
	bar.SetActivePane("sessions")

	out := bar.View()

	if !strings.Contains(out, "j") {
		t.Errorf("expected 'j' in view for sessions pane, got:\n%s", out)
	}
	if strings.Contains(out, "pgup") {
		t.Errorf("expected 'pgup' absent when preview pane is inactive, got:\n%s", out)
	}
}

func TestHelp_Toggle(t *testing.T) {
	reg := help.NewRegistry()
	reg.RegisterPane("sessions", []key.Binding{
		key.NewBinding(key.WithKeys("j"), key.WithHelp("j", "down")),
	})

	bar := help.NewHelpBar(reg)
	bar.SetActivePane("sessions")

	shortOut := bar.View()

	updated, _ := bar.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	bar = updated.(help.Model)

	fullOut := bar.View()

	if shortOut == fullOut {
		t.Errorf("expected view to change after toggling help\nshort: %q\nfull:  %q", shortOut, fullOut)
	}
}
