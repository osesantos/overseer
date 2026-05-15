package session

import (
	"log/slog"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	domainsession "github.com/dnlopes/overseer/internal/core/domain/session"
	servicesession "github.com/dnlopes/overseer/internal/core/service/session"
	"github.com/dnlopes/overseer/internal/testutil/mocks"
)

func newTestSession(name string) domainsession.Session {
	s, err := domainsession.New(name, "test-project")
	if err != nil {
		panic(err)
	}
	return s
}

func newRenameFormFixture(current domainsession.Session) (RenameFormModel, *mocks.MockSessionRepository) {
	mock := &mocks.MockSessionRepository{
		GetResult:  current,
		ListResult: []domainsession.Session{current},
	}
	uc := servicesession.NewRenameUseCase(mock, slog.Default())
	m := NewRenameForm(styles.New(), uc, current)
	return m, mock
}

func TestRenameForm_HappyPath(t *testing.T) {
	current := newTestSession("old-name")
	m, _ := newRenameFormFixture(current)

	if m.nameInput.Value() != "old-name" {
		t.Fatalf("expected input pre-filled with %q, got %q", "old-name", m.nameInput.Value())
	}

	m.nameInput.SetValue("new-name")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a cmd after Enter with valid name")
	}

	msg := cmd()
	renamed, ok := msg.(sessionRenamedMsg)
	if !ok {
		t.Fatalf("expected sessionRenamedMsg, got %T", msg)
	}
	if renamed.NewName != "new-name" {
		t.Errorf("expected NewName=%q, got %q", "new-name", renamed.NewName)
	}
}

func TestRenameForm_EmptyName(t *testing.T) {
	current := newTestSession("old-name")
	m, _ := newRenameFormFixture(current)

	m.nameInput.SetValue("")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("expected no cmd for empty name")
	}

	form, ok := updated.(RenameFormModel)
	if !ok {
		t.Fatalf("expected RenameFormModel back, got %T", updated)
	}
	if form.errMsg == "" {
		t.Error("expected errMsg to be set when name is empty")
	}
}

func TestRenameForm_Esc(t *testing.T) {
	current := newTestSession("old-name")
	m, _ := newRenameFormFixture(current)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected a cmd after Esc")
	}

	msg := cmd()
	_, ok := msg.(cancelFormMsg)
	if !ok {
		t.Fatalf("expected cancelFormMsg, got %T", msg)
	}
}
