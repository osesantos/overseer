package dashboard

import (
	"testing"

	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/core/domain"
)

func prSession(t *testing.T) (domain.Session, domain.PullRequest) {
	t.Helper()
	sess, err := domain.NewSession("api", uuid.New())
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	pr, err := domain.NewPullRequest(535, "feat: cron schedules", "feature/cron", domain.PRStateOpen)
	if err != nil {
		t.Fatalf("NewPullRequest() error = %v", err)
	}
	return sess, pr
}

func TestDashboard_PRStatusUpdatedMsg_ReachesTheInspector(t *testing.T) {
	sess, pr := prSession(t)
	m := selectInDashboard(t, sizedDashboard(t), sess)

	updated, _ := m.Update(shared.PRStatusUpdatedMsg{SessionID: sess.ID, PR: pr})
	m = updated.(Model)

	if !m.inspector.HasPullRequest() {
		t.Fatal("the inspector did not receive the PR status")
	}
}

func TestDashboard_SessionSelected_ReplaysTheCachedPRStatus(t *testing.T) {
	sess, pr := prSession(t)
	m := sizedDashboard(t)
	updated, _ := m.Update(shared.PRStatusUpdatedMsg{SessionID: sess.ID, PR: pr})
	m = updated.(Model)

	m = selectInDashboard(t, m, sess)

	if !m.inspector.HasPullRequest() {
		t.Fatal("selecting the session did not replay its cached PR status to the inspector")
	}
}

func TestDashboard_P_SelectsThePullRequestTab(t *testing.T) {
	sess, pr := prSession(t)
	m := selectInDashboard(t, sizedDashboard(t), sess)
	updated, _ := m.Update(shared.PRStatusUpdatedMsg{SessionID: sess.ID, PR: pr})
	m = updated.(Model)

	updated, _ = m.Update(letterKey("p"))
	m = updated.(Model)

	if got := m.inspector.ActiveViewLabel(); got != "PR" {
		t.Fatalf("active tab = %q, want PR", got)
	}
}

func TestDashboard_P_WithoutAPR_Toasts(t *testing.T) {
	sess, _ := prSession(t)
	m := selectInDashboard(t, sizedDashboard(t), sess)

	updated, _ := m.Update(letterKey("p"))
	m = updated.(Model)

	if m.toast != "no pull request for this session" {
		t.Fatalf("toast = %q, want the no-PR notice", m.toast)
	}
	if got := m.inspector.ActiveViewLabel(); got != "Agent" {
		t.Fatalf("active tab = %q, want Agent untouched", got)
	}
}
