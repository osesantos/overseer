package dashboard

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/testutil"
)

func openIssue(key string) domain.Issue {
	return domain.Issue{Key: key, Project: domain.IssueProjectKey(key), Summary: "Deployment Failure: thing", Status: "Backlog",
		StatusCategory: domain.IssueStatusNew, CreatedAt: time.Now()}
}

func TestDashboard_IssuesLoadedMsg_StoresTheInboxAndAutoTriagesWhenOn(t *testing.T) {
	m := sizedDashboard(t)

	updated, cmd := m.Update(shared.IssuesLoadedMsg{Inbox: domain.TriageInbox{Issues: []domain.Issue{openIssue("RDODCF-1")}}})
	m = updated.(Model)

	if len(m.triageInbox.Issues) != 1 || !m.triageLoaded {
		t.Fatalf("triageInbox = %+v, loaded = %v", m.triageInbox, m.triageLoaded)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want an auto-triage command while auto-triage is on")
	}
}

func TestDashboard_AutoTriageToggle_StopsPollsFromStartingTriages(t *testing.T) {
	m := sizedDashboard(t)

	updated, _ := m.Update(shared.AutoTriageToggleMsg{})
	m = updated.(Model)
	if m.triage.AutoTriage {
		t.Fatal("toggle did not turn auto-triage off")
	}
	if _, cmd := m.Update(shared.IssuesLoadedMsg{Inbox: domain.TriageInbox{Issues: []domain.Issue{openIssue("RDODCF-1")}}}); cmd != nil {
		t.Fatal("a poll started auto-triage while it is off")
	}
}

func TestDashboard_ReapFinishedTriage_OnlyForDoneIssuesWithATriageSession(t *testing.T) {
	m := sizedDashboard(t)
	triaging := testutil.MakeSession("RDODCF-7 thing", uuid.New())
	_ = triaging.AssignTriageTarget("RDODCF-7")
	review := testutil.MakeSession("PR #7 thing", uuid.New())
	_ = review.AssignReviewTarget(domain.PullRequestRef{Repo: "o/r", Number: 7})
	updated, _ := m.Update(shared.SessionsLoadedMsg{Sessions: []domain.Session{triaging, review}})
	m = updated.(Model)

	done := openIssue("RDODCF-7")
	done.StatusCategory = domain.IssueStatusDone
	if cmds := m.reapFinishedTriageCmds(domain.TriageInbox{Issues: []domain.Issue{done}}); len(cmds) != 1 {
		t.Fatalf("done issue produced %d delete commands, want 1", len(cmds))
	}
	if cmds := m.reapFinishedTriageCmds(domain.TriageInbox{Issues: []domain.Issue{openIssue("RDODCF-7")}}); len(cmds) != 0 {
		t.Fatal("open issue produced a delete command")
	}
	other := openIssue("RDODCF-8")
	other.StatusCategory = domain.IssueStatusDone
	if cmds := m.reapFinishedTriageCmds(domain.TriageInbox{Issues: []domain.Issue{other}}); len(cmds) != 0 {
		t.Fatal("a done issue without a triage session produced a delete command")
	}
}

func TestDashboard_TriageSessionReaped_ToastsAndReloads(t *testing.T) {
	m := sizedDashboard(t)
	triaging := testutil.MakeSession("RDODCF-7 thing", uuid.New())
	_ = triaging.AssignTriageTarget("RDODCF-7")

	updated, cmd := m.Update(shared.TriageSessionReapedMsg{Session: triaging})
	m = updated.(Model)

	if !strings.Contains(m.toast, "RDODCF-7 is done") || cmd == nil {
		t.Fatalf("toast = %q, cmd nil = %v; want a deletion notice and a reload", m.toast, cmd == nil)
	}
}

func TestDashboard_IssueOpen_WithoutLauncher_Toasts(t *testing.T) {
	m := sizedDashboard(t)

	updated, _ := m.Update(shared.IssueOpenMsg{Issue: openIssue("RDINC-3")})

	if toast := updated.(Model).toast; !strings.Contains(toast, "no triage launcher for RDINC") {
		t.Fatalf("toast = %q", toast)
	}
}

func TestDashboard_ColonTriage_OpensThePage_AndQReturns(t *testing.T) {
	m := sizedDashboard(t)
	updated, _ := m.Update(shared.IssuesLoadedMsg{Inbox: domain.TriageInbox{Issues: []domain.Issue{openIssue("RDODCF-1")}}})
	m = updated.(Model)
	m.triage.AutoTriage = false

	m = typeCommand(t, m, "triage")
	if m.page != pageTriage {
		t.Fatal(":triage did not open the triage page")
	}
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "Support tickets") || !strings.Contains(view, "RDODCF-1") {
		t.Fatalf("triage page not rendered as the body:\n%s", view)
	}

	updated, _ = m.Update(letterKey("q"))
	if updated.(Model).page != pageSessions {
		t.Fatal("q on the triage page did not return to the sessions page")
	}
}
