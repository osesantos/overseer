package session

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/testutil"
)

var errAdapter = errors.New("adapter blew up")

func escKeyPress() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyEsc}
}

func makeTmuxSession(id string) domain.TmuxSession {
	now := time.Now()
	return domain.TmuxSession{ID: id, CreatedAt: now, UpdatedAt: now}
}

func TestDeleteForm_EscapeCancelsPopup(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	form := NewDeleteForm(styles.New(), newCreateFormSessionService(t), sess)

	_, cmd := form.Update(escKeyPress())

	if cmd == nil {
		t.Fatalf("Update(esc) command = nil, want cancel emit")
	}
	if _, ok := cmd().(shared.NewSessionDeletePopupCloseMsg); !ok {
		t.Fatalf("Update(esc) msg type = %T, want shared.NewSessionDeletePopupCloseMsg", cmd())
	}
}

func TestDeleteForm_NCancelsPopup(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	form := NewDeleteForm(styles.New(), newCreateFormSessionService(t), sess)

	_, cmd := form.Update(formKeyPress("n"))

	if cmd == nil {
		t.Fatalf("Update(n) command = nil, want cancel emit")
	}
	if _, ok := cmd().(shared.NewSessionDeletePopupCloseMsg); !ok {
		t.Fatalf("Update(n) msg type = %T, want shared.NewSessionDeletePopupCloseMsg", cmd())
	}
}

func TestDeleteForm_EnterAlsoConfirms(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	svc, repo, _, tmux, _ := newCreateFormSessionServiceWithMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, sess.ID.String()).
		Return(makeTmuxSession(sess.ID.String()), nil).Once()
	tmux.EXPECT().KillSession(mock.Anything, sess.ID.String()).Return(nil).Once()
	expectAgentAndEditorTmuxGone(tmux, sess.ID.String())
	repo.EXPECT().Delete(mock.Anything, sess.ID).Return(nil).Once()

	form := NewDeleteForm(styles.New(), svc, sess)

	_, cmd := form.Update(formKeyPress("enter"))

	if cmd == nil {
		t.Fatalf("Update(enter) command = nil, want delete command")
	}
	if _, ok := cmd().(shared.SessionDeletedMsg); !ok {
		t.Fatalf("Update(enter) msg type = %T, want shared.SessionDeletedMsg", cmd())
	}
}

func TestDeleteForm_ViewUsesDangerStyling(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	form := NewDeleteForm(styles.New(), newCreateFormSessionService(t), sess)

	view := form.View().Content
	wantANSI := "\x1b[38;2;239;68;68m"
	if !strings.Contains(view, wantANSI) {
		t.Fatalf("View() missing danger-red ANSI sequence %q in output: %q", wantANSI, view)
	}
}

func TestDeleteForm_YConfirmCallsServiceAndEmitsDeletedMsg(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	svc, repo, _, tmux, _ := newCreateFormSessionServiceWithMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, sess.ID.String()).
		Return(makeTmuxSession(sess.ID.String()), nil).Once()
	tmux.EXPECT().KillSession(mock.Anything, sess.ID.String()).Return(nil).Once()
	expectAgentAndEditorTmuxGone(tmux, sess.ID.String())
	repo.EXPECT().Delete(mock.Anything, sess.ID).Return(nil).Once()

	form := NewDeleteForm(styles.New(), svc, sess)

	_, cmd := form.Update(formKeyPress("y"))

	if cmd == nil {
		t.Fatalf("Update(y) command = nil, want delete command")
	}
	msg := cmd()
	if _, ok := msg.(shared.SessionDeletedMsg); !ok {
		t.Fatalf("Update(y) msg type = %T, want shared.SessionDeletedMsg", msg)
	}
}

func TestDeleteForm_YConfirmServiceErrorEmitsErrMsg(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	svc, repo, _, _, _ := newCreateFormSessionServiceWithMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, errAdapter).Once()

	form := NewDeleteForm(styles.New(), svc, sess)

	_, cmd := form.Update(formKeyPress("y"))

	if cmd == nil {
		t.Fatalf("Update(y) command = nil, want error command")
	}
	errMsg, ok := cmd().(shared.SessionDeleteErrMsg)
	if !ok {
		t.Fatalf("Update(y) msg type = %T, want shared.SessionDeleteErrMsg", cmd())
	}
	if errMsg.Err == nil {
		t.Fatalf("Update(y) errMsg.Err = nil, want non-nil")
	}
}

func TestDeleteForm_ViewMentionsSessionNameAndWorktreeConsequences(t *testing.T) {
	sess := testutil.MakeSessionWithWorktree("alpha", uuid.New(), "/data/worktrees/abc", "overseer/alpha")
	form := NewDeleteForm(styles.New(), newCreateFormSessionService(t), sess)

	view := form.View().Content
	if !strings.Contains(view, "alpha") {
		t.Fatalf("View() missing session name 'alpha': %q", view)
	}
	if !strings.Contains(view, "worktree") {
		t.Fatalf("View() missing worktree warning for project-backed session: %q", view)
	}
}

func TestDeleteForm_ViewMentionsTmuxConsequencesForProjectlessSession(t *testing.T) {
	sess := testutil.MakeSession("orphan", uuid.New())
	form := NewDeleteForm(styles.New(), newCreateFormSessionService(t), sess)

	view := form.View().Content
	if strings.Contains(view, "worktree") {
		t.Fatalf("View() mentioned worktree for project-less session: %q", view)
	}
	if !strings.Contains(view, "tmux") {
		t.Fatalf("View() missing tmux warning: %q", view)
	}
}

func TestDeleteForm_SwarmSession_KeepsBoardUnlessToggled(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 3)
	svc := newCreateFormSessionService(t)

	form := NewDeleteForm(styles.New(), svc, sess)
	if form.purgeBoard {
		t.Fatal("purgeBoard defaults to on; the transcript must survive a plain delete")
	}

	view := form.View().Content
	if !strings.Contains(view, "kept on disk") {
		t.Fatalf("View() does not state the board is kept: %q", view)
	}
	if !strings.Contains(view, "p: toggle board purge") {
		t.Fatalf("View() does not offer the purge toggle: %q", view)
	}
}

func TestDeleteForm_SwarmSession_PToggleFlipsAndIsVisible(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 3)
	svc := newCreateFormSessionService(t)

	updated, _ := NewDeleteForm(styles.New(), svc, sess).Update(formKeyPress("p"))
	form := updated.(DeleteFormModel)

	if !form.purgeBoard {
		t.Fatal("p did not enable the board purge")
	}
	// The purge is irreversible, so its state has to be readable on screen.
	if view := form.View().Content; !strings.Contains(view, "PURGED") {
		t.Fatalf("View() does not warn that the board will be purged: %q", view)
	}

	updated, _ = form.Update(formKeyPress("p"))
	if updated.(DeleteFormModel).purgeBoard {
		t.Fatal("p is not a toggle — it did not turn the purge back off")
	}
}

func TestDeleteForm_NonSwarmSession_HidesThePurgeOption(t *testing.T) {
	sess := testutil.MakeSession("solo", uuid.New())
	svc := newCreateFormSessionService(t)

	form := NewDeleteForm(styles.New(), svc, sess)

	view := form.View().Content
	if strings.Contains(view, "Swarm board") || strings.Contains(view, "toggle board purge") {
		t.Fatalf("View() offers a board purge on a session that has no board: %q", view)
	}

	// And the key must not silently arm anything.
	updated, _ := form.Update(formKeyPress("p"))
	if updated.(DeleteFormModel).purgeBoard {
		t.Fatal("p armed the purge on a non-swarm session")
	}
}
