package session

import (
	"log/slog"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/service"
	"github.com/dnlopes/overseer/internal/testutil/mocks"
)

func TestCreateForm_TabFocusesProjectInput(t *testing.T) {
	model := NewCreateForm(styles.New(), newCreateFormSessionService(nil))

	updated, _ := model.Update(formKeyPress("tab"))
	updated, _ = updated.(CreateFormModel).Update(formKeyPress("overseer"))

	form := updated.(CreateFormModel)
	if got := form.projectInput.Value(); got != "overseer" {
		t.Fatalf("project input value = %q, want %q", got, "overseer")
	}
	if got := form.nameInput.Value(); got != "" {
		t.Fatalf("name input value = %q, want empty after project typing", got)
	}
}

func TestCreateForm_SubmitCreatesSessionWithProject(t *testing.T) {
	repo := &mocks.MockSessionRepository{}
	model := NewCreateForm(styles.New(), newCreateFormSessionService(repo))

	updated, _ := model.Update(formKeyPress("alpha"))
	updated, _ = updated.(CreateFormModel).Update(formKeyPress("tab"))
	updated, _ = updated.(CreateFormModel).Update(formKeyPress("overseer"))
	_, cmd := updated.(CreateFormModel).Update(formKeyPress("enter"))

	if cmd == nil {
		t.Fatalf("submit command = nil, want create command")
	}
	msg, ok := cmd().(shared.SessionCreatedMsg)
	if !ok {
		t.Fatalf("submit msg type = %T, want shared.SessionCreatedMsg", cmd())
	}
	if msg.Session.Name != "alpha" || msg.Session.ProjectName != "overseer" {
		t.Fatalf("created session = %+v, want alpha/overseer", msg.Session)
	}
}

func newCreateFormSessionService(repo *mocks.MockSessionRepository) service.SessionService {
	if repo == nil {
		repo = &mocks.MockSessionRepository{}
	}
	return *service.NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, slog.Default())
}

func formKeyPress(value string) tea.KeyPressMsg {
	if value == "tab" {
		return tea.KeyPressMsg{Code: tea.KeyTab}
	}
	if value == "enter" {
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	}
	return tea.KeyPressMsg{Text: value, Code: []rune(value)[0]}
}
