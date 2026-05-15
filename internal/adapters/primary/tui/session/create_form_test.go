package session

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	domainsession "github.com/dnlopes/overseer/internal/core/domain/session"
	servicesession "github.com/dnlopes/overseer/internal/core/service/session"
	"github.com/dnlopes/overseer/internal/testutil/mocks"
)

func newCreateFormUseCase(sessions []domainsession.Session) *servicesession.CreateUseCase {
	repo := &mocks.MockSessionRepository{ListResult: sessions}
	tmux := &mocks.MockTmuxAdapter{}
	git := &mocks.MockGitAdapter{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return servicesession.NewCreateUseCase(repo, tmux, git, logger)
}

func newCreateFormFixture() CreateFormModel {
	uc := newCreateFormUseCase([]domainsession.Session{})
	return NewCreateForm(styles.New(), uc)
}

func TestCreateForm_HappyPath(t *testing.T) {
	m := newCreateFormFixture()

	m.nameInput.SetValue("my-session")
	m.projectInput.SetValue("my-project")

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a cmd after Enter with valid inputs")
	}

	msg := cmd()
	created, ok := msg.(sessionCreatedMsg)
	if !ok {
		t.Fatalf("expected sessionCreatedMsg, got %T", msg)
	}
	if created.session.Name != "my-session" {
		t.Errorf("session.Name: want %q, got %q", "my-session", created.session.Name)
	}
	if created.session.ProjectName != "my-project" {
		t.Errorf("session.ProjectName: want %q, got %q", "my-project", created.session.ProjectName)
	}

	_, ok = result.(CreateFormModel)
	if !ok {
		t.Fatalf("expected CreateFormModel back, got %T", result)
	}
}

func TestCreateForm_EmptyName(t *testing.T) {
	m := newCreateFormFixture()

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("expected no cmd when name is empty")
	}

	form, ok := result.(CreateFormModel)
	if !ok {
		t.Fatalf("expected CreateFormModel back, got %T", result)
	}
	if form.errMsg == "" {
		t.Error("expected errMsg to be set for empty name")
	}
	if !strings.Contains(form.errMsg, "required") {
		t.Errorf("errMsg should mention 'required', got %q", form.errMsg)
	}
}

func TestCreateForm_Esc(t *testing.T) {
	m := newCreateFormFixture()

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

func TestCreateForm_TabCycles(t *testing.T) {
	m := newCreateFormFixture()

	if m.focusIndex != 0 {
		t.Fatalf("initial focusIndex: want 0, got %d", m.focusIndex)
	}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if cmd != nil {
		t.Fatal("Tab should not produce a cmd")
	}
	form := result.(CreateFormModel)
	if form.focusIndex != 1 {
		t.Fatalf("focusIndex after Tab: want 1, got %d", form.focusIndex)
	}
	if form.projectInput.Focused() != true {
		t.Error("projectInput should be focused after Tab")
	}
	if form.nameInput.Focused() != false {
		t.Error("nameInput should be blurred after Tab")
	}

	result2, _ := form.Update(tea.KeyMsg{Type: tea.KeyTab})
	form2 := result2.(CreateFormModel)
	if form2.focusIndex != 0 {
		t.Fatalf("focusIndex after second Tab: want 0, got %d", form2.focusIndex)
	}
	if form2.nameInput.Focused() != true {
		t.Error("nameInput should be focused after second Tab")
	}
}

func TestCreateForm_EmptyProject(t *testing.T) {
	m := newCreateFormFixture()
	m.nameInput.SetValue("my-session")

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("expected no cmd when project is empty")
	}

	form, ok := result.(CreateFormModel)
	if !ok {
		t.Fatalf("expected CreateFormModel back, got %T", result)
	}
	if form.errMsg == "" {
		t.Error("expected errMsg to be set for empty project")
	}
	if !strings.Contains(form.errMsg, "required") {
		t.Errorf("errMsg should mention 'required', got %q", form.errMsg)
	}
}

func TestCreateForm_ViewContainsHelp(t *testing.T) {
	m := newCreateFormFixture()
	view := m.View()
	if !strings.Contains(view, "Tab") || !strings.Contains(view, "Enter") || !strings.Contains(view, "Esc") {
		t.Errorf("View should contain help hints, got: %q", view)
	}
}
