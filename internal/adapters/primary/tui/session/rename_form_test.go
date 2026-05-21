package session

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/testutil"
)

func TestRenameForm_PrefillsInputWithCurrentName(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	svc := newCreateFormSessionService(t)

	form := NewRenameSessionForm(styles.New(), svc, sess, 100)

	if got := form.input.Value(); got != "alpha" {
		t.Fatalf("input prefilled with %q, want %q", got, "alpha")
	}
}

func TestRenameForm_EscapeClosesPopup(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	form := NewRenameSessionForm(styles.New(), newCreateFormSessionService(t), sess, 100)

	_, cmd := form.Update(escKeyPress())

	if cmd == nil {
		t.Fatalf("Update(esc) command = nil, want close emit")
	}
	if _, ok := cmd().(shared.RenamePopupCloseMsg); !ok {
		t.Fatalf("Update(esc) msg type = %T, want shared.RenamePopupCloseMsg", cmd())
	}
}

func TestRenameForm_SubmitWithUnchangedNameClosesPopup(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	form := NewRenameSessionForm(styles.New(), newCreateFormSessionService(t), sess, 100)

	_, cmd := form.Update(formKeyPress("enter"))

	if cmd == nil {
		t.Fatalf("Update(enter) command = nil, want close emit")
	}
	if _, ok := cmd().(shared.RenamePopupCloseMsg); !ok {
		t.Fatalf("Update(enter) msg type = %T, want shared.RenamePopupCloseMsg (no-op rename)", cmd())
	}
}

func TestRenameForm_SubmitEmptyNameShowsError(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	form := NewRenameSessionForm(styles.New(), newCreateFormSessionService(t), sess, 100)

	form.input.SetValue("   ")
	updated, cmd := form.Update(formKeyPress("enter"))

	if cmd != nil {
		t.Fatalf("Update(enter) cmd = %v, want nil (validation rejects empty name)", cmd)
	}
	got := updated.(RenameFormModel).errMsg
	if !strings.Contains(got, "required") {
		t.Fatalf("errMsg = %q, want it to mention name is required", got)
	}
}

func TestRenameForm_SessionSubmitCallsRenameAndEmitsRenamedMsg(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	svc, repo, _, _, _ := newCreateFormSessionServiceWithMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	repo.EXPECT().List(mock.Anything).Return([]domain.Session{sess}, nil).Once()
	repo.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()

	form := NewRenameSessionForm(styles.New(), svc, sess, 100)
	form.input.SetValue("beta")

	_, cmd := form.Update(formKeyPress("enter"))

	if cmd == nil {
		t.Fatalf("Update(enter) cmd = nil, want rename command")
	}
	msg, ok := cmd().(shared.SessionRenamedMsg)
	if !ok {
		t.Fatalf("Update(enter) msg type = %T, want shared.SessionRenamedMsg", cmd())
	}
	if msg.Session.Name != "beta" {
		t.Fatalf("rename result Session.Name = %q, want %q", msg.Session.Name, "beta")
	}
}

func TestRenameForm_SessionSubmitServiceErrorEmitsErrMsg(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	svc, repo, _, _, _ := newCreateFormSessionServiceWithMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).
		Return(domain.Session{}, errors.New("boom")).Once()

	form := NewRenameSessionForm(styles.New(), svc, sess, 100)
	form.input.SetValue("beta")

	_, cmd := form.Update(formKeyPress("enter"))

	if cmd == nil {
		t.Fatalf("Update(enter) cmd = nil, want error command")
	}
	errMsg, ok := cmd().(shared.SessionRenameErrMsg)
	if !ok {
		t.Fatalf("Update(enter) msg type = %T, want shared.SessionRenameErrMsg", cmd())
	}
	if errMsg.Err == nil {
		t.Fatalf("err msg has nil Err, want non-nil")
	}
}

func TestRenameForm_ProjectSubmitCallsRenameAndEmitsRenamedMsg(t *testing.T) {
	project := testutil.MakeProject("/repo/overseer", "old")
	svc, repo := newProjectsServiceWithMocks(t)
	repo.EXPECT().Get(mock.Anything, project.ID).Return(project, nil).Once()
	repo.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()

	form := NewRenameProjectForm(styles.New(), svc, project.ID, project.Name, 100)
	form.input.SetValue("new")

	_, cmd := form.Update(formKeyPress("enter"))

	if cmd == nil {
		t.Fatalf("Update(enter) cmd = nil, want rename command")
	}
	msg, ok := cmd().(shared.ProjectRenamedMsg)
	if !ok {
		t.Fatalf("Update(enter) msg type = %T, want shared.ProjectRenamedMsg", cmd())
	}
	if msg.Project.Name != "new" {
		t.Fatalf("rename result Project.Name = %q, want %q", msg.Project.Name, "new")
	}
}

func TestRenameForm_ProjectSubmitServiceErrorEmitsErrMsg(t *testing.T) {
	project := testutil.MakeProject("/repo/overseer", "old")
	svc, repo := newProjectsServiceWithMocks(t)
	repo.EXPECT().Get(mock.Anything, project.ID).
		Return(domain.Project{}, errors.New("boom")).Once()

	form := NewRenameProjectForm(styles.New(), svc, project.ID, project.Name, 100)
	form.input.SetValue("new")

	_, cmd := form.Update(formKeyPress("enter"))

	if cmd == nil {
		t.Fatalf("Update(enter) cmd = nil, want error command")
	}
	errMsg, ok := cmd().(shared.ProjectRenameErrMsg)
	if !ok {
		t.Fatalf("Update(enter) msg type = %T, want shared.ProjectRenameErrMsg", cmd())
	}
	if errMsg.Err == nil {
		t.Fatalf("err msg has nil Err, want non-nil")
	}
}

func TestRenameForm_ViewShowsSessionTitleAndCurrentName(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	form := NewRenameSessionForm(styles.New(), newCreateFormSessionService(t), sess, 100)

	view := form.View().Content

	if !strings.Contains(view, "Rename Session") {
		t.Fatalf("view missing 'Rename Session' title: %q", view)
	}
	if !strings.Contains(view, "alpha") {
		t.Fatalf("view missing current name 'alpha': %q", view)
	}
}

func TestRenameForm_ViewShowsGroupTitleForProject(t *testing.T) {
	svc, _ := newProjectsServiceWithMocks(t)
	form := NewRenameProjectForm(styles.New(), svc, uuid.New(), "overseer", 100)

	view := form.View().Content

	if !strings.Contains(view, "Rename Group") {
		t.Fatalf("view missing 'Rename Group' title: %q", view)
	}
	if !strings.Contains(view, "overseer") {
		t.Fatalf("view missing current group name 'overseer': %q", view)
	}
}

func TestRenameForm_ErrMsgClearsRenamedMsgPath(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	form := NewRenameSessionForm(styles.New(), newCreateFormSessionService(t), sess, 100)

	_, cmd := form.Update(shared.SessionRenamedMsg{Session: sess})

	if cmd == nil {
		t.Fatalf("Update(SessionRenamedMsg) cmd = nil, want close popup")
	}
	if _, ok := cmd().(shared.RenamePopupCloseMsg); !ok {
		t.Fatalf("Update(SessionRenamedMsg) msg type = %T, want shared.RenamePopupCloseMsg", cmd())
	}
}

func TestRenameForm_KeyTypingUpdatesInput(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	form := NewRenameSessionForm(styles.New(), newCreateFormSessionService(t), sess, 100)

	updated, _ := form.Update(formKeyPress("X"))

	if got := updated.(RenameFormModel).input.Value(); got != "alphaX" {
		t.Fatalf("input value after typing 'X' = %q, want %q", got, "alphaX")
	}
}
