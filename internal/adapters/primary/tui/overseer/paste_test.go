package overseer

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
)

func TestChat_PasteReachesTheInput(t *testing.T) {
	// tea.PasteMsg is its own message type, not a KeyPressMsg. The Update switch
	// only handled key presses, so a paste fell through and was silently dropped —
	// you simply could not paste into the chat.
	m := New(styles.New())
	m.SetSize(80, 20)

	updated, _ := m.Update(tea.PasteMsg{Content: "some pasted text"})
	m = updated.(Model)

	if got := m.input.Value(); !strings.Contains(got, "some pasted text") {
		t.Fatalf("input value = %q, want the pasted text", got)
	}
}

func TestChat_PasteAppendsToWhatYouAlreadyTyped(t *testing.T) {
	m := New(styles.New())
	m.SetSize(80, 20)

	updated, _ := m.Update(tea.KeyPressMsg{Text: "/send ", Code: 's'})
	m = updated.(Model)
	before := m.input.Value()

	updated, _ = m.Update(tea.PasteMsg{Content: "pasted"})
	m = updated.(Model)

	if got := m.input.Value(); got == before || !strings.Contains(got, "pasted") {
		t.Fatalf("input value = %q, want %q plus the pasted text", got, before)
	}
}

func TestChat_PasteIsIgnoredWhileThinking(t *testing.T) {
	// Input is frozen while the agent works; paste must respect that like typing.
	m := New(styles.New())
	m.SetSize(80, 20)
	m.thinking = true

	updated, _ := m.Update(tea.PasteMsg{Content: "nope"})

	if got := updated.(Model).input.Value(); got != "" {
		t.Fatalf("input value = %q, want empty while thinking", got)
	}
}
