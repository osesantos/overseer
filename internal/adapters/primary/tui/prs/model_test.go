package prs

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
)

func fixture(t *testing.T) (Model, domain.Session) {
	t.Helper()
	now := time.Now()
	reviewed, _ := domain.NewSession("PR #92 fix", uuid.New())
	_ = reviewed.AssignReviewTarget(domain.PullRequestRef{Repo: "OutSystems/signer", Number: 92})

	m := New(styles.New())
	m.SetSize(120, 20)
	m.SetData(Data{
		Loaded: true,
		Inbox: domain.ReviewInbox{Viewer: "me", PullRequests: []domain.PullRequest{
			{Repo: "OutSystems/signer", Number: 92, Title: "fix(RRCT-1): error info", Author: "joao", State: domain.PRStateOpen, UpdatedAt: now},
			{Repo: "OutSystems/signer", Number: 93, Title: "chore: bump", Author: "dependabot", AuthorIsBot: true, State: domain.PRStateOpen, UpdatedAt: now.Add(-time.Hour)},
			{Repo: "OutSystems/elsewhere", Number: 4, Title: "feat: thing", Author: "ana", State: domain.PRStateOpen, UpdatedAt: now},
		}},
		Sessions:      []domain.Session{reviewed},
		ProjectNames:  map[string]bool{"signer": true},
		AgentStatuses: map[uuid.UUID]domain.AgentStatus{reviewed.ID: {Kind: domain.AgentStatusRunning}},
	})
	return m, reviewed
}

func enter() tea.KeyPressMsg { return tea.KeyPressMsg{Code: tea.KeyEnter} }
func letter(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func TestModel_View_GroupsByRepoAndMarksSessionState(t *testing.T) {
	m, _ := fixture(t)

	view := ansi.Strip(m.View().Content)

	for _, want := range []string{"waiting on me", "2 open", "(1 bot PRs hidden, b shows them)", "OutSystems/signer (1)", "OutSystems/elsewhere (1)", "#92", "review session", "#4", "no local repo"} {
		if !strings.Contains(view, want) {
			t.Errorf("view is missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "#93") {
		t.Errorf("bot PR #93 is listed:\n%s", view)
	}
}

func TestModel_View_FocusedRow_IsOneStyleRun(t *testing.T) {
	m, _ := fixture(t)
	updated, _ := m.Update(letter('j'))
	m = updated.(Model)

	var focused string
	for _, line := range strings.Split(m.View().Content, "\n") {
		if strings.Contains(ansi.Strip(line), "#4") {
			focused = line
		}
	}
	if focused == "" {
		t.Fatal("focused PR row not found")
	}
	// One selection style over plain text: no inner reset before the end of the row.
	body := strings.TrimRight(focused, " ")
	if inner := strings.Count(body, "\x1b[0m"); inner > 1 {
		t.Fatalf("focused row carries %d style resets, want a single run:\n%q", inner, focused)
	}
}

func TestModel_View_EmptyInbox(t *testing.T) {
	m := New(styles.New())
	m.SetSize(80, 10)
	m.SetData(Data{Loaded: true, Inbox: domain.ReviewInbox{Viewer: "me"}})

	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "Nothing waiting on you") {
		t.Fatalf("view = %q", view)
	}
}

func TestModel_View_BeforeTheFirstPoll_SaysLoading(t *testing.T) {
	m := New(styles.New())
	m.SetSize(80, 10)
	m.SetData(Data{})

	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "Loading pull requests") || strings.Contains(view, "Nothing waiting") {
		t.Fatalf("view = %q, want a loading notice", view)
	}
}

func TestModel_View_LoadError_IsShownInsteadOfLoading(t *testing.T) {
	m := New(styles.New())
	m.SetSize(80, 10)
	m.SetData(Data{LoadError: "gh: HTTP 504"})

	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "Could not load the inbox: gh: HTTP 504") {
		t.Fatalf("view = %q, want the load error", view)
	}
}

func TestModel_EnterOnARepoRow_DoesNothing(t *testing.T) {
	m, _ := fixture(t)

	_, cmd := m.Update(enter())

	if cmd != nil {
		t.Fatal("enter on a repo group emitted a command")
	}
}

func TestModel_EnterOnAPR_EmitsOpen(t *testing.T) {
	m, _ := fixture(t)
	updated, _ := m.Update(letter('j'))
	m = updated.(Model)

	_, cmd := m.Update(enter())

	msg, ok := cmd().(shared.PullRequestOpenMsg)
	if !ok || msg.PR.Number != 4 {
		t.Fatalf("got %T %+v, want PullRequestOpenMsg for #4 (first repo alphabetically)", cmd(), cmd())
	}
}

func TestModel_ROnAPR_EmitsReview(t *testing.T) {
	m, _ := fixture(t)
	updated, _ := m.Update(letter('j'))
	m = updated.(Model)

	_, cmd := m.Update(letter('r'))

	if msg, ok := cmd().(shared.PullRequestReviewMsg); !ok || msg.PR.Number != 4 {
		t.Fatalf("got %+v, want PullRequestReviewMsg for #4", cmd())
	}
}

func TestModel_Esc_EmitsClose(t *testing.T) {
	m, _ := fixture(t)

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if _, ok := cmd().(shared.PageCloseMsg); !ok {
		t.Fatalf("got %+v, want PageCloseMsg", cmd())
	}
}

func TestModel_DOnAPR_EmitsDismiss(t *testing.T) {
	m, _ := fixture(t)
	updated, _ := m.Update(letter('j'))
	m = updated.(Model)

	_, cmd := m.Update(letter('d'))

	if msg, ok := cmd().(shared.PullRequestDismissMsg); !ok || msg.PR.Number != 4 {
		t.Fatalf("got %+v, want PullRequestDismissMsg for #4", cmd())
	}
}

func TestModel_GAndShiftG_JumpToTopAndBottom(t *testing.T) {
	m, _ := fixture(t)

	updated, _ := m.Update(letter('G'))
	m = updated.(Model)
	if pr, ok := m.selectedPR(); !ok || pr.Number != 92 {
		t.Fatalf("after G selected = %+v, want the last row (#92)", pr)
	}

	updated, _ = m.Update(letter('g'))
	m = updated.(Model)
	if _, ok := m.selectedPR(); ok {
		t.Fatal("after g the cursor should be on the first repo group, not a PR")
	}
}

func TestModel_OOnAPR_EmitsOpenInBrowser(t *testing.T) {
	m, _ := fixture(t)
	updated, _ := m.Update(letter('j'))
	m = updated.(Model)

	_, cmd := m.Update(letter('o'))

	if msg, ok := cmd().(shared.PullRequestOpenInBrowserMsg); !ok || msg.PR.Number != 4 {
		t.Fatalf("got %+v, want PullRequestOpenInBrowserMsg for #4", cmd())
	}
}

func TestModel_A_EmitsAutoReviewToggle_AndHeaderShowsTheState(t *testing.T) {
	m, _ := fixture(t)

	_, cmd := m.Update(letter('a'))
	if _, ok := cmd().(shared.AutoReviewToggleMsg); !ok {
		t.Fatalf("got %+v, want AutoReviewToggleMsg", cmd())
	}

	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "auto-review off") {
		t.Fatalf("header does not show auto-review off:\n%s", view)
	}
	data := m.data
	data.AutoReview = true
	m.SetData(data)
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "auto-review on") {
		t.Fatalf("header does not show auto-review on:\n%s", view)
	}
}

func TestModel_B_TogglesBotPRs(t *testing.T) {
	m, _ := fixture(t)

	updated, _ := m.Update(letter('b'))
	m = updated.(Model)
	view := ansi.Strip(m.View().Content)
	for _, want := range []string{"3 open", "(1 from bots, b hides them)", "#93", "bot"} {
		if !strings.Contains(view, want) {
			t.Errorf("after b the view is missing %q:\n%s", want, view)
		}
	}

	updated, _ = m.Update(letter('b'))
	m = updated.(Model)
	if view := ansi.Strip(m.View().Content); strings.Contains(view, "#93") {
		t.Fatalf("second b did not hide the bot PR again:\n%s", view)
	}
}
