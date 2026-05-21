package shared_test

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
)

func keyPressRune(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Text: string(r), Code: r}
}

func keyPressEsc() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyEsc}
}

func sampleGroups() []shared.HelpPopupGroup {
	return []shared.HelpPopupGroup{
		{
			Title: "Sessions",
			Bindings: []key.Binding{
				key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new session")),
				key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete session")),
			},
		},
		{
			Title: "General",
			Bindings: []key.Binding{
				key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help menu")),
				key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
			},
		},
	}
}

func TestHelpPopup_EscEmitsClose(t *testing.T) {
	popup := shared.NewHelpPopupModel(styles.New(), sampleGroups(), 100)

	_, cmd := popup.Update(keyPressEsc())

	if cmd == nil {
		t.Fatalf("Update(esc) command = nil, want close emit")
	}
	if _, ok := cmd().(shared.HelpPopupCloseMsg); !ok {
		t.Fatalf("Update(esc) msg type = %T, want shared.HelpPopupCloseMsg", cmd())
	}
}

func TestHelpPopup_QuestionMarkEmitsClose(t *testing.T) {
	popup := shared.NewHelpPopupModel(styles.New(), sampleGroups(), 100)

	_, cmd := popup.Update(keyPressRune('?'))

	if cmd == nil {
		t.Fatalf("Update(?) command = nil, want close emit")
	}
	if _, ok := cmd().(shared.HelpPopupCloseMsg); !ok {
		t.Fatalf("Update(?) msg type = %T, want shared.HelpPopupCloseMsg", cmd())
	}
}

func TestHelpPopup_QEmitsClose(t *testing.T) {
	popup := shared.NewHelpPopupModel(styles.New(), sampleGroups(), 100)

	_, cmd := popup.Update(keyPressRune('q'))

	if cmd == nil {
		t.Fatalf("Update(q) command = nil, want close emit")
	}
	if _, ok := cmd().(shared.HelpPopupCloseMsg); !ok {
		t.Fatalf("Update(q) msg type = %T, want shared.HelpPopupCloseMsg", cmd())
	}
}

func TestHelpPopup_UnrelatedKeyIsNoop(t *testing.T) {
	popup := shared.NewHelpPopupModel(styles.New(), sampleGroups(), 100)

	_, cmd := popup.Update(keyPressRune('x'))

	if cmd != nil {
		t.Fatalf("Update(x) command = %v, want nil", cmd())
	}
}

func TestHelpPopup_ViewRendersAllBindings(t *testing.T) {
	popup := shared.NewHelpPopupModel(styles.New(), sampleGroups(), 100)

	out := popup.View().Content

	for _, want := range []string{"new session", "delete session", "help menu", "quit"} {
		if !strings.Contains(out, want) {
			t.Errorf("View missing binding description %q\noutput:\n%s", want, out)
		}
	}
}

func TestHelpPopup_ViewRendersGroupTitles(t *testing.T) {
	popup := shared.NewHelpPopupModel(styles.New(), sampleGroups(), 100)

	out := popup.View().Content

	for _, want := range []string{"Sessions", "General"} {
		if !strings.Contains(out, want) {
			t.Errorf("View missing group title %q\noutput:\n%s", want, out)
		}
	}
}

func TestHelpPopup_ViewHasModalBorder(t *testing.T) {
	popup := shared.NewHelpPopupModel(styles.New(), sampleGroups(), 100)

	out := popup.View().Content

	roundedChars := "╭╮╰╯"
	found := false
	for _, ch := range roundedChars {
		if strings.ContainsRune(out, ch) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("View missing rounded modal border chars (╭╮╰╯), got: %q", out)
	}
}

func TestHelpPopup_ViewIncludesTitle(t *testing.T) {
	popup := shared.NewHelpPopupModel(styles.New(), sampleGroups(), 100)

	out := popup.View().Content

	if !strings.Contains(out, "Keyboard Shortcuts") {
		t.Errorf("View missing popup title\noutput:\n%s", out)
	}
}

func TestHelpPopup_DisabledBindingsHidden(t *testing.T) {
	disabled := key.NewBinding(key.WithKeys("z"), key.WithHelp("z", "secret action"))
	disabled.SetEnabled(false)
	groups := []shared.HelpPopupGroup{
		{
			Title: "Sessions",
			Bindings: []key.Binding{
				key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new session")),
				disabled,
			},
		},
	}
	popup := shared.NewHelpPopupModel(styles.New(), groups, 100)

	out := popup.View().Content

	if strings.Contains(out, "secret action") {
		t.Errorf("View should hide disabled binding description, got:\n%s", out)
	}
	if !strings.Contains(out, "new session") {
		t.Errorf("View should still show enabled binding\noutput:\n%s", out)
	}
}

func TestHelpPopup_EmptyGroupIsSkipped(t *testing.T) {
	groups := []shared.HelpPopupGroup{
		{
			Title:    "Empty",
			Bindings: nil,
		},
		{
			Title: "Sessions",
			Bindings: []key.Binding{
				key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new session")),
			},
		},
	}
	popup := shared.NewHelpPopupModel(styles.New(), groups, 100)

	out := popup.View().Content

	if strings.Contains(out, "Empty") {
		t.Errorf("View should not render empty group title, got:\n%s", out)
	}
}
