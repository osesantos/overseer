package dashboard

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/jobs"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/adapters/secondary/swarmboard"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/core/service"
	"github.com/dnlopes/overseer/internal/shared/paths"
	"github.com/dnlopes/overseer/internal/testutil"
	"github.com/dnlopes/overseer/internal/testutil/mocks"
)

func newTestDashboard(t *testing.T) Model {
	t.Helper()

	repo := mocks.NewMockSessionRepository(t)
	projects := mocks.NewMockProjectRepository(t)
	tmux := mocks.NewMockTmuxAdapter(t)
	git := mocks.NewMockGitAdapter(t)
	defaultLauncher, _ := domain.NewLauncher("OpenCode", "opencode", domain.AgentTypeOpenCode)

	sessSvc := service.NewSessionService(repo, projects, tmux, git, swarmboard.New(paths.NewResolver(""), slog.Default()), mocks.NewMockReviewSettingsRepository(t), mocks.NewMockTriageSettingsRepository(t), paths.NewResolver(""), defaultLauncher, "nvim", slog.Default())
	projSvc := service.NewProjectService(projects, git, slog.Default())
	prSvc := service.NewPullRequestService(mocks.NewMockPullRequestPort(t), mocks.NewMockReviewSettingsRepository(t), slog.Default())

	return New(
		styles.New(),
		*sessSvc,
		*projSvc,
		*prSvc,
		*service.NewIssueService(mocks.NewMockIssueTrackerPort(t), mocks.NewMockTriageSettingsRepository(t), slog.Default()),
		nil, // overseerService — not needed for these tests
		nil, // swarmService — not needed for these tests
		jobs.Model{},
		[]domain.Launcher{defaultLauncher},
		domain.DefaultLabels,
		60, 15,
		500*time.Millisecond,
		nil, // discoveryPaths
		"",  // swarmBoardURL — swarm disabled
		domain.SwarmMaxAgents,
		ReviewOptions{Command: `claude "/ose-pr-review {{number}}"`, AutoReview: true, MaxPerPoll: 2},
		TriageOptions{Commands: map[string]string{"RDODCF": `claude "/ose-rdodcf-triage {{key}}"`}, ProjectName: "vault", AutoTriage: true, MaxPerPoll: 1, MaxAge: 7 * 24 * time.Hour},
	)
}

func TestDashboard_SessionSelectedMsg_ForwardsToInspector(t *testing.T) {
	m := newTestDashboard(t)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)

	before := ansi.Strip(m.View().Content)
	if !strings.Contains(before, "Select a session to preview") {
		t.Fatalf("setup: expected inspector to show 'Select a session to preview' before selection, got:\n%s", before)
	}

	sess := testutil.MakeSession("alpha", uuid.New())
	updated, _ = m.Update(shared.SessionSelectedMsg{Session: sess})
	m = updated.(Model)

	after := ansi.Strip(m.View().Content)
	if strings.Contains(after, "Select a session to preview") {
		t.Errorf("inspector still shows 'Select a session to preview' after SessionSelectedMsg; the message was not forwarded to the inspector. View:\n%s", after)
	}
}

func TestDashboard_SessionSelectionClearedMsg_ForwardsToInspector(t *testing.T) {
	m := newTestDashboard(t)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)

	sess := testutil.MakeSession("alpha", uuid.New())
	updated, _ = m.Update(shared.SessionSelectedMsg{Session: sess})
	m = updated.(Model)

	before := ansi.Strip(m.View().Content)
	if strings.Contains(before, "Select a session to preview") {
		t.Fatalf("setup: expected inspector to show a session after selection, got:\n%s", before)
	}

	updated, _ = m.Update(shared.SessionSelectionClearedMsg{})
	m = updated.(Model)

	after := ansi.Strip(m.View().Content)
	if !strings.Contains(after, "Select a session to preview") {
		t.Errorf("inspector should show 'Select a session to preview' after SessionSelectionClearedMsg; the message was not forwarded to the inspector. View:\n%s", after)
	}
}

func TestDashboard_AgentStatusesUpdatedMsg_RendersAggregateInSessionsTitle(t *testing.T) {
	m := newTestDashboardNoEmoji(t)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 30})
	m = updated.(Model)

	statuses := map[uuid.UUID]domain.AgentStatus{
		uuid.New(): {Kind: domain.AgentStatusRunning},
		uuid.New(): {Kind: domain.AgentStatusRunning},
		uuid.New(): {Kind: domain.AgentStatusRunning},
		uuid.New(): {Kind: domain.AgentStatusWaiting},
		uuid.New(): {Kind: domain.AgentStatusDead},
	}

	updated, _ = m.Update(shared.AgentStatusesUpdatedMsg{Statuses: statuses})
	m = updated.(Model)

	view := ansi.Strip(m.View().Content)
	want := "Sessions (● 3 running ◐ 1 waiting ■ 1 dead)"
	if !strings.Contains(view, want) {
		t.Errorf("Sessions box title missing aggregate %q in view:\n%s", want, view)
	}

	lines := strings.Split(view, "\n")
	titleLine := ""
	for _, line := range lines {
		if strings.Contains(line, "Sessions (") {
			titleLine = line
			break
		}
	}
	if titleLine == "" {
		t.Fatalf("Sessions box border line with aggregate not found in view:\n%s", view)
	}
	if !strings.Contains(titleLine, want) {
		t.Errorf("aggregate %q should sit immediately after the Sessions title on the border line:\n%s", want, titleLine)
	}
}

func TestDashboard_AgentStatusesUpdatedMsg_OmitsZeroCounts(t *testing.T) {
	m := newTestDashboardNoEmoji(t)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 30})
	m = updated.(Model)

	statuses := map[uuid.UUID]domain.AgentStatus{
		uuid.New(): {Kind: domain.AgentStatusRunning},
	}

	updated, _ = m.Update(shared.AgentStatusesUpdatedMsg{Statuses: statuses})
	m = updated.(Model)

	view := ansi.Strip(m.View().Content)
	want := "Sessions (● 1 running)"
	if !strings.Contains(view, want) {
		t.Errorf("Sessions box title missing %q in view:\n%s", want, view)
	}
	for _, segment := range []string{"◐ 0", "■ 0", "○ 0", "? 0", "waiting", "dead", "idle", "unknown"} {
		if strings.Contains(view, segment) {
			t.Errorf("Sessions box title should omit zero-count segments, found %q in view:\n%s", segment, view)
		}
	}
}

func TestDashboard_AgentStatusesUpdatedMsg_NoStatuses_TitleIsPlainSessions(t *testing.T) {
	m := newTestDashboardNoEmoji(t)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 30})
	m = updated.(Model)

	view := ansi.Strip(m.View().Content)
	if strings.Contains(view, "Sessions (") {
		t.Errorf("plain 'Sessions' title (no parens) expected before any AgentStatusesUpdatedMsg, got:\n%s", view)
	}
	if !strings.Contains(view, "─ Sessions ") {
		t.Errorf("Sessions title not found in view:\n%s", view)
	}
}

func newTestDashboardNoEmoji(t *testing.T) Model {
	t.Helper()

	repo := mocks.NewMockSessionRepository(t)
	projects := mocks.NewMockProjectRepository(t)
	tmux := mocks.NewMockTmuxAdapter(t)
	git := mocks.NewMockGitAdapter(t)
	defaultLauncher, _ := domain.NewLauncher("OpenCode", "opencode", domain.AgentTypeOpenCode)

	sessSvc := service.NewSessionService(repo, projects, tmux, git, swarmboard.New(paths.NewResolver(""), slog.Default()), mocks.NewMockReviewSettingsRepository(t), mocks.NewMockTriageSettingsRepository(t), paths.NewResolver(""), defaultLauncher, "nvim", slog.Default())
	projSvc := service.NewProjectService(projects, git, slog.Default())

	return New(
		styles.NewWithTheme("dark", true),
		*sessSvc,
		*projSvc,
		*service.NewPullRequestService(mocks.NewMockPullRequestPort(t), mocks.NewMockReviewSettingsRepository(t), slog.Default()),
		*service.NewIssueService(mocks.NewMockIssueTrackerPort(t), mocks.NewMockTriageSettingsRepository(t), slog.Default()),
		nil, // overseerService — not needed for these tests
		nil, // swarmService — not needed for these tests
		jobs.Model{},
		[]domain.Launcher{defaultLauncher},
		domain.DefaultLabels,
		60, 15,
		500*time.Millisecond,
		nil, // discoveryPaths
		"",  // swarmBoardURL — swarm disabled
		domain.SwarmMaxAgents,
		ReviewOptions{Command: `claude "/ose-pr-review {{number}}"`, AutoReview: true, MaxPerPoll: 2},
		TriageOptions{Commands: map[string]string{"RDODCF": `claude "/ose-rdodcf-triage {{key}}"`}, ProjectName: "vault", AutoTriage: true, MaxPerPoll: 1, MaxAge: 7 * 24 * time.Hour},
	)
}

func TestDashboard_PullRequestsLoadedMsg_StoresTheInboxAndAutoReviewsWhenOn(t *testing.T) {
	m := sizedDashboard(t)
	inbox := domain.ReviewInbox{Viewer: "me", PullRequests: []domain.PullRequest{{Repo: "o/r", Number: 5}}}

	updated, cmd := m.Update(shared.PullRequestsLoadedMsg{Inbox: inbox})
	m = updated.(Model)

	if len(m.reviewInbox.PullRequests) != 1 {
		t.Fatalf("reviewInbox = %+v, want the loaded inbox", m.reviewInbox)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want an auto-review command while auto-review is on")
	}
}

func TestDashboard_AutoReviewToggle_StopsPollsFromStartingReviews(t *testing.T) {
	m := sizedDashboard(t)

	updated, _ := m.Update(shared.AutoReviewToggleMsg{})
	m = updated.(Model)
	if m.review.AutoReview {
		t.Fatal("toggle did not turn auto-review off")
	}

	_, cmd := m.Update(shared.PullRequestsLoadedMsg{Inbox: domain.ReviewInbox{Viewer: "me", PullRequests: []domain.PullRequest{{Repo: "o/r", Number: 5}}}})
	if cmd != nil {
		t.Fatal("a poll started auto-review while it is off")
	}
}

func TestDashboard_AutoReviewCompleted_ReloadsSessionsOnlyWhenSomethingWasCreated(t *testing.T) {
	m := sizedDashboard(t)

	if _, cmd := m.Update(shared.AutoReviewCompletedMsg{}); cmd != nil {
		t.Fatal("cmd != nil, want no reload when nothing was created")
	}
	if _, cmd := m.Update(shared.AutoReviewCompletedMsg{Created: []domain.Session{testutil.MakeSession("PR #5", uuid.New())}}); cmd == nil {
		t.Fatal("cmd = nil, want a sessions reload after a review session was created")
	}
}

func TestDashboard_ColonOpensTheCommandLine_AndEscCloses(t *testing.T) {
	m := sizedDashboard(t)

	updated, _ := m.Update(letterKey(":"))
	m = updated.(Model)
	if !m.cmdlineActive {
		t.Fatal(": did not open the command line")
	}
	lines := strings.Split(strings.TrimRight(ansi.Strip(m.View().Content), "\n"), "\n")
	if last := lines[len(lines)-1]; !strings.HasPrefix(strings.TrimSpace(last), ":") {
		t.Fatalf("command line not rendered on the bottom line, got %q", last)
	}

	// q is text now, not quit.
	updated, cmd := m.Update(letterKey("q"))
	m = updated.(Model)
	if isQuit(t, cmd) {
		t.Fatal("q quit the app while typing a command")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updated.(Model)
	if m.cmdlineActive {
		t.Fatal("esc did not close the command line")
	}
}

func TestDashboard_ColonPrs_OpensThePage_AndSessionsReturns(t *testing.T) {
	m := sizedDashboard(t)
	updated, _ := m.Update(shared.PullRequestsLoadedMsg{Inbox: domain.ReviewInbox{Viewer: "me", PullRequests: []domain.PullRequest{
		{Repo: "o/r", Number: 5, Title: "feat: thing", Author: "ana", State: domain.PRStateOpen},
	}}})
	m = updated.(Model)

	m = typeCommand(t, m, "prs")
	if m.page != pagePRs {
		t.Fatal(":prs did not open the PRs page")
	}
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "waiting on me") || !strings.Contains(view, "#5") {
		t.Fatalf("PRs page not rendered as the body:\n%s", view)
	}
	if strings.Contains(view, "Select a session to preview") {
		t.Fatal("the inspector is still rendered under the PRs page")
	}

	m = typeCommand(t, m, "sessions")
	if m.page != pageSessions {
		t.Fatal(":sessions did not return to the sessions page")
	}
}

func TestDashboard_UnknownCommand_ShowsAToastWhenTheChatIsHidden(t *testing.T) {
	m := sizedDashboard(t)

	m = typeCommand(t, m, "nope")

	if !strings.Contains(m.toast, "unknown command") {
		t.Fatalf("toast = %q, want the unknown-command error", m.toast)
	}
}

func TestDashboard_ColonIsIgnoredWhileAPopupIsOpen(t *testing.T) {
	m := sizedDashboard(t)
	updated, _ := m.Update(letterKey("?"))
	m = updated.(Model)
	if m.activePopup != popupHelp {
		t.Fatal("precondition: help popup not open")
	}

	updated, _ = m.Update(letterKey(":"))
	m = updated.(Model)

	if m.cmdlineActive {
		t.Fatal(": opened the command line under a popup")
	}
}

func TestDashboard_PRsPage_EscReturnsToSessions(t *testing.T) {
	m := sizedDashboard(t)
	m = typeCommand(t, m, "prs")

	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)

	if m.page != pageSessions {
		t.Fatal("esc on the PRs page did not return to sessions")
	}
}

func TestDashboard_O_OpensTheSelectedSessionPRInTheBrowser(t *testing.T) {
	m := sizedDashboard(t)
	sess := testutil.MakeSession("feature", uuid.New())
	m = selectInDashboard(t, m, sess)

	updated, cmd := m.Update(letterKey("o"))
	m = updated.(Model)
	if cmd == nil || !strings.Contains(m.toast, "no pull request") {
		t.Fatalf("toast = %q, want a no-PR notice before the PR status arrives", m.toast)
	}

	pr := domain.PullRequest{Repo: "o/r", Number: 5, URL: "https://github.com/o/r/pull/5"}
	updated, _ = m.Update(shared.PRStatusUpdatedMsg{SessionID: sess.ID, PR: pr})
	m = updated.(Model)
	_, cmd = m.Update(letterKey("o"))
	if cmd == nil {
		t.Fatal("o returned no command")
	}
	if got, ok := cmd().(shared.PullRequestOpenInBrowserMsg); !ok || got.PR.URL != pr.URL {
		t.Fatalf("o emitted %+v, want PullRequestOpenInBrowserMsg for %s", got, pr.URL)
	}
}

func TestDashboard_OpenPRWithASession_FocusesIt(t *testing.T) {
	m := sizedDashboard(t)
	sess := testutil.MakeSession("PR #5 feat: thing", uuid.New())
	_ = sess.AssignReviewTarget(domain.PullRequestRef{Repo: "o/r", Number: 5})
	m = selectInDashboard(t, m, sess)
	m = typeCommand(t, m, "prs")

	updated, cmd := m.Update(shared.PullRequestOpenMsg{PR: domain.PullRequest{Repo: "o/r", Number: 5}})
	m = updated.(Model)

	if m.page != pageSessions {
		t.Fatal("opening a PR with a session did not switch back to the sessions page")
	}
	if focus, ok := cmd().(shared.SessionFocusRequestedMsg); !ok || focus.SessionID != sess.ID {
		t.Fatalf("got %+v, want a focus request for the review session", cmd())
	}
}

func TestDashboard_OpenPRWithoutALocalRepo_ShowsAToast(t *testing.T) {
	m := sizedDashboard(t)

	updated, cmd := m.Update(shared.PullRequestOpenMsg{PR: domain.PullRequest{Repo: "o/missing", Number: 5}})
	m = updated.(Model)

	if cmd == nil || !strings.Contains(m.toast, "no local repo") {
		t.Fatalf("toast = %q, want a no-local-repo notice", m.toast)
	}
}

// typeCommand opens the command line, types text and submits it.
func typeCommand(t *testing.T, m Model, text string) Model {
	t.Helper()
	updated, _ := m.Update(letterKey(":"))
	m = updated.(Model)
	for _, r := range text {
		updated, _ = m.Update(letterKey(string(r)))
		m = updated.(Model)
	}
	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ = m.Update(msg)
			m = updated.(Model)
		}
	}
	return m
}

func TestDashboard_PRStatusMerged_ReapsTheReviewSession(t *testing.T) {
	m := sizedDashboard(t)
	review := testutil.MakeSession("PR #7 thing", uuid.New())
	_ = review.AssignReviewTarget(domain.PullRequestRef{Repo: "o/r", Number: 7})
	plain := testutil.MakeSession("work", uuid.New())
	updated, _ := m.Update(shared.SessionsLoadedMsg{Sessions: []domain.Session{review, plain}})
	m = updated.(Model)

	if cmd := m.reapFinishedReviewCmd(review.ID, domain.PullRequest{State: domain.PRStateMerged}); cmd == nil {
		t.Fatal("merged PR on a review session did not produce a delete command")
	}
	if cmd := m.reapFinishedReviewCmd(review.ID, domain.PullRequest{State: domain.PRStateOpen}); cmd != nil {
		t.Fatal("open PR on a review session produced a delete command")
	}
	if cmd := m.reapFinishedReviewCmd(plain.ID, domain.PullRequest{State: domain.PRStateMerged}); cmd != nil {
		t.Fatal("merged PR on an ordinary session produced a delete command")
	}
}

func TestDashboard_ReviewSessionReaped_ToastsAndReloads(t *testing.T) {
	m := sizedDashboard(t)
	review := testutil.MakeSession("PR #7 thing", uuid.New())
	_ = review.AssignReviewTarget(domain.PullRequestRef{Repo: "o/r", Number: 7})

	updated, cmd := m.Update(shared.ReviewSessionReapedMsg{Session: review})
	m = updated.(Model)

	if !strings.Contains(m.toast, "PR #7") || cmd == nil {
		t.Fatalf("toast = %q, cmd nil = %v; want a deletion notice and a reload", m.toast, cmd == nil)
	}
}

func TestDashboard_PullRequestDismissed_MarksItInTheInbox(t *testing.T) {
	m := sizedDashboard(t)
	updated, _ := m.Update(shared.PullRequestsLoadedMsg{Inbox: domain.ReviewInbox{Viewer: "me", PullRequests: []domain.PullRequest{
		{Repo: "o/r", Number: 5, Title: "keep", Author: "ana", State: domain.PRStateOpen},
		{Repo: "o/r", Number: 6, Title: "drop", Author: "ana", State: domain.PRStateOpen},
	}}})
	m = updated.(Model)

	updated, cmd := m.Update(shared.PullRequestDismissedMsg{Ref: domain.PullRequestRef{Repo: "o/r", Number: 6}})
	m = updated.(Model)

	prs := m.reviewInbox.PullRequests
	if len(prs) != 2 || prs[0].Dismissed || !prs[1].Dismissed {
		t.Fatalf("inbox = %+v, want #6 marked dismissed and #5 untouched", prs)
	}
	if cmd == nil || !strings.Contains(m.toast, "#6") {
		t.Fatalf("toast = %q, want a hidden notice for #6", m.toast)
	}
}

func TestDashboard_PRsPage_QReturnsToSessions_CtrlCQuits(t *testing.T) {
	m := sizedDashboard(t)
	m = typeCommand(t, m, "prs")

	updated, cmd := m.Update(letterKey("q"))
	m = updated.(Model)
	if isQuit(t, cmd) || m.page != pageSessions {
		t.Fatal("q on the PRs page must return to the sessions page, not quit")
	}

	m = typeCommand(t, m, "prs")
	if _, cmd := m.Update(ctrlC()); !isQuit(t, cmd) {
		t.Fatal("ctrl+c must still quit from the PRs page")
	}
}

func TestDashboard_PRsPage_SaysLoadingUntilTheFirstPollAnswers(t *testing.T) {
	m := sizedDashboard(t)
	m = typeCommand(t, m, "prs")
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "Loading pull requests") {
		t.Fatalf("page before the first poll:\n%s", view)
	}

	updated, _ := m.Update(shared.PullRequestsLoadedMsg{Inbox: domain.ReviewInbox{Viewer: "me"}})
	m = updated.(Model)
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "Nothing waiting on you") {
		t.Fatalf("page after an empty poll:\n%s", view)
	}
}
