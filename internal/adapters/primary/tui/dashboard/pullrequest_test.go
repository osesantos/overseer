package dashboard

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"

	"github.com/osesantos/skipper/internal/adapters/primary/tui/jobs"
	"github.com/osesantos/skipper/internal/adapters/primary/tui/shared"
	"github.com/osesantos/skipper/internal/core/domain"
)

// prSession is a session in a registered project with an open PR; the project
// matters because the PR page runs gh in its path.
func prSession(t *testing.T) (domain.Session, domain.PullRequest, domain.Project) {
	t.Helper()
	project, err := domain.NewProject("/repo/api", "api")
	if err != nil {
		t.Fatalf("NewProject() error = %v", err)
	}
	sess, err := domain.NewSession("api", project.ID)
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	pr, err := domain.NewPullRequest(535, "feat: cron schedules", "feature/cron", domain.PRStateOpen)
	if err != nil {
		t.Fatalf("NewPullRequest() error = %v", err)
	}
	return sess, pr, project
}

func withProject(t *testing.T, m Model, project domain.Project) Model {
	t.Helper()
	updated, _ := m.Update(shared.ProjectsLoadedMsg{Projects: []domain.Project{project}})
	return updated.(Model)
}

func TestDashboard_PRStatusUpdatedMsg_ReachesTheInspector(t *testing.T) {
	sess, pr, _ := prSession(t)
	m := selectInDashboard(t, sizedDashboard(t), sess)

	updated, _ := m.Update(shared.PRStatusUpdatedMsg{SessionID: sess.ID, PR: pr})
	m = updated.(Model)

	if !m.inspector.HasPullRequest() {
		t.Fatal("the inspector did not receive the PR status")
	}
}

func TestDashboard_SessionSelected_ReplaysTheCachedPRStatus(t *testing.T) {
	sess, pr, _ := prSession(t)
	m := sizedDashboard(t)
	updated, _ := m.Update(shared.PRStatusUpdatedMsg{SessionID: sess.ID, PR: pr})
	m = updated.(Model)

	m = selectInDashboard(t, m, sess)

	if !m.inspector.HasPullRequest() {
		t.Fatal("selecting the session did not replay its cached PR status to the inspector")
	}
}

func TestDashboard_P_SelectsThePullRequestTab(t *testing.T) {
	sess, pr, _ := prSession(t)
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
	sess, _, _ := prSession(t)
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

func prPageDashboard(t *testing.T) (Model, domain.Session, domain.PullRequest) {
	t.Helper()
	sess, pr, project := prSession(t)
	m := selectInDashboard(t, withProject(t, sizedDashboard(t), project), sess)
	updated, _ := m.Update(shared.PRStatusUpdatedMsg{SessionID: sess.ID, PR: pr})
	m = updated.(Model)
	updated, _ = m.Update(letterKey("p"))
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	return updated.(Model), sess, pr
}

func TestDashboard_EnterOnThePRTab_OpensThePage(t *testing.T) {
	m, _, _ := prPageDashboard(t)

	if m.page != pagePullRequest {
		t.Fatalf("page = %v, want the pull request page", m.page)
	}
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "#535") || !strings.Contains(v, "Loading") {
		t.Fatalf("page view:\n%s", v)
	}
}

func TestDashboard_PRCommand_OpensThePageForTheSelectedSession(t *testing.T) {
	sess, pr, project := prSession(t)
	m := selectInDashboard(t, withProject(t, sizedDashboard(t), project), sess)
	updated, _ := m.Update(shared.PRStatusUpdatedMsg{SessionID: sess.ID, PR: pr})
	m = updated.(Model)

	updated, _ = m.executeCommand("/pr")
	m = updated.(Model)

	if m.page != pagePullRequest {
		t.Fatalf("page = %v, want the pull request page", m.page)
	}
}

func TestDashboard_PRCommand_WithoutAPR_ReportsAnError(t *testing.T) {
	sess, _, _ := prSession(t)
	m := selectInDashboard(t, sizedDashboard(t), sess)

	updated, cmd := m.executeCommand("/pr")
	m = updated.(Model)

	if m.page != pageSessions || cmd == nil {
		t.Fatalf("page = %v, cmd nil = %v; want to stay on sessions with a result", m.page, cmd == nil)
	}
	if res, ok := cmd().(shared.OverseerCommandResultMsg); !ok || !res.IsError {
		t.Fatalf("result = %#v, want an error result", cmd())
	}
}

func TestDashboard_PullRequestDetailsLoaded_FillsThePage(t *testing.T) {
	m, sess, pr := prPageDashboard(t)
	details := domain.PullRequestDetails{PullRequest: pr, Files: []domain.PRFile{{Path: "src/Scheduler.cs", Additions: 3}}}

	updated, _ := m.Update(shared.PullRequestDetailsLoadedMsg{SessionID: sess.ID, Details: details})
	m = updated.(Model)

	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "src/Scheduler.cs") || strings.Contains(v, "Loading") {
		t.Fatalf("page view:\n%s", v)
	}
}

func TestDashboard_PullRequestDetailsLoaded_ForAnotherSession_IsIgnored(t *testing.T) {
	m, _, pr := prPageDashboard(t)
	details := domain.PullRequestDetails{PullRequest: pr, Files: []domain.PRFile{{Path: "src/Other.cs"}}}

	updated, _ := m.Update(shared.PullRequestDetailsLoadedMsg{SessionID: uuid.New(), Details: details})
	m = updated.(Model)

	if v := ansi.Strip(m.View().Content); strings.Contains(v, "src/Other.cs") {
		t.Fatalf("another session's details landed on the page:\n%s", v)
	}
}

func TestDashboard_PullRequestDetailsLoaded_Error_IsShown(t *testing.T) {
	m, sess, _ := prPageDashboard(t)

	updated, _ := m.Update(shared.PullRequestDetailsLoadedMsg{SessionID: sess.ID, Err: errors.New("gh exit 1")})
	m = updated.(Model)

	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "gh exit 1") {
		t.Fatalf("page view:\n%s", v)
	}
}

func TestDashboard_QAndEscOnThePRPage_ReturnToSessions(t *testing.T) {
	m, _, _ := prPageDashboard(t)
	updated, cmd := m.Update(letterKey("q"))
	m = updated.(Model)
	if m.page != pageSessions || isQuit(t, cmd) {
		t.Fatalf("after q: page = %v, quit = %v", m.page, isQuit(t, cmd))
	}

	m, _, _ = prPageDashboard(t)
	updated, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("esc on the page emitted nothing")
	}
	updated, _ = m.Update(cmd())
	if m = updated.(Model); m.page != pageSessions {
		t.Fatalf("after esc: page = %v", m.page)
	}
}

func TestDashboard_CtrlCOnThePRPage_StillQuits(t *testing.T) {
	m, _, _ := prPageDashboard(t)
	if _, cmd := m.Update(ctrlC()); !isQuit(t, cmd) {
		t.Fatal("ctrl+c on the page did not quit")
	}
}

func TestDashboard_R_OnThePRPage_Refetches(t *testing.T) {
	m, _, _ := prPageDashboard(t)
	updated, cmd := m.Update(letterKey("r"))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("r emitted nothing")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)

	if !strings.Contains(m.toast, "refreshing PR #535") {
		t.Fatalf("toast = %q, want the refresh notice", m.toast)
	}
}

func TestDashboard_NewerPRStatus_WhileThePageIsOpen_Refetches(t *testing.T) {
	m, sess, pr := prPageDashboard(t)
	pr.UpdatedAt = time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	updated, _ := m.Update(shared.PullRequestDetailsLoadedMsg{SessionID: sess.ID, Details: domain.PullRequestDetails{PullRequest: pr}})
	m = updated.(Model)

	same := pr
	updated, _ = m.Update(shared.PRStatusUpdatedMsg{SessionID: sess.ID, PR: same})
	if m = updated.(Model); m.toast != "" {
		t.Fatalf("an unchanged status refetched: toast %q", m.toast)
	}

	newer := pr
	newer.UpdatedAt = pr.UpdatedAt.Add(time.Minute)
	updated, _ = m.Update(shared.PRStatusUpdatedMsg{SessionID: sess.ID, PR: newer})
	if m = updated.(Model); !strings.Contains(m.toast, "refreshing PR #535") {
		t.Fatalf("a newer status did not refetch: toast %q", m.toast)
	}
}

func TestDashboard_NewerPRStatus_BeforeDetailsLoaded_DoesNotDoubleFetch(t *testing.T) {
	m, sess, pr := prPageDashboard(t)
	pr.UpdatedAt = time.Now()

	updated, _ := m.Update(shared.PRStatusUpdatedMsg{SessionID: sess.ID, PR: pr})

	if m = updated.(Model); m.toast != "" {
		t.Fatalf("refetched while the first fetch is in flight: toast %q", m.toast)
	}
}

func TestDashboard_PRPageSearchPrompt_KeepsQAndColonAsText(t *testing.T) {
	m, sess, pr := prPageDashboard(t)
	updated, _ := m.Update(shared.PullRequestDetailsLoadedMsg{SessionID: sess.ID, Details: domain.PullRequestDetails{PullRequest: pr, Diff: "diff --git a/x b/x\n@@ -1 +1 @@\n-old\n+new\n"}})
	m = updated.(Model)
	for _, k := range []tea.KeyPressMsg{letterKey("]"), letterKey("/"), letterKey("q"), letterKey(":")} {
		updated, _ = m.Update(k)
		m = updated.(Model)
	}

	if m.page != pagePullRequest || m.cmdlineActive || !m.prPage.CapturesInput() {
		t.Fatalf("page=%v cmdline=%v capturing=%v: a shortcut escaped the search prompt", m.page, m.cmdlineActive, m.prPage.CapturesInput())
	}
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "/q:") {
		t.Fatalf("prompt does not show the typed text:\n%s", v)
	}
}

func TestDashboard_ReloadOnTheInboxPages_FiresTheJobOnce(t *testing.T) {
	tests := []struct {
		name  string
		msg   tea.Msg
		jobID string
		toast string
	}{
		{"prs", shared.PullRequestsReloadMsg{}, PullRequestInboxJobID, "reloading PR inbox"},
		{"triage", shared.IssuesReloadMsg{}, TriageInboxJobID, "reloading triage inbox"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var runs int
			m := sizedDashboard(t)
			m.scheduler = jobs.New(jobs.Job{ID: tt.jobID, Interval: time.Hour, Run: func() tea.Cmd { runs++; return nil }})

			updated, cmd := m.Update(tt.msg)
			m = updated.(Model)

			if runs != 1 || cmd == nil || m.toast != tt.toast {
				t.Fatalf("runs = %d, cmd nil = %v, toast = %q", runs, cmd == nil, m.toast)
			}
		})
	}
}
