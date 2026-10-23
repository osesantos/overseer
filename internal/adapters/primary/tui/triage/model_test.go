package triage

import (
	"fmt"
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
	triaging, _ := domain.NewSession("RDODCF-7699 x", uuid.New())
	_ = triaging.AssignTriageTarget("RDODCF-7699")

	m := New(styles.New())
	m.SetSize(140, 20)
	m.SetData(Data{
		Loaded: true,
		Inbox: domain.TriageInbox{Issues: []domain.Issue{
			{Key: "RDODCF-7699", Project: "RDODCF", Summary: "Deployment Failure: ai-acp-service", Status: "Backlog", StatusCategory: domain.IssueStatusNew, Priority: "Normal", CreatedAt: now},
			{Key: "RDODCF-4652", Project: "RDODCF", Summary: "Old timermanager failure", Status: "Backlog", StatusCategory: domain.IssueStatusNew, Priority: "Normal", CreatedAt: now.Add(-48 * time.Hour), Dismissed: true},
			{Key: "RDINC-94264", Project: "RDINC", Summary: "Cannot terminate revisions", Status: "Waiting for customer", StatusCategory: "undefined", Assignee: "Rafael Seara", Priority: "High", CreatedAt: now},
			{Key: "RDODCF-1", Project: "RDODCF", Summary: "Already solved", Status: "Solved", StatusCategory: domain.IssueStatusDone, CreatedAt: now},
		}},
		Sessions:      []domain.Session{triaging},
		Launchers:     map[string]bool{"RDODCF": true},
		AgentStatuses: map[uuid.UUID]domain.AgentStatus{triaging.ID: {Kind: domain.AgentStatusRunning}},
		AutoTriage:    true,
	})
	return m, triaging
}

func enter() tea.KeyPressMsg { return tea.KeyPressMsg{Code: tea.KeyEnter} }
func letter(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func TestModel_View_GroupsByProjectAndMarksSessionState(t *testing.T) {
	m, _ := fixture(t)

	view := ansi.Strip(m.View().Content)

	for _, want := range []string{"Support tickets", "2 open", "(1 dismissed hidden, h shows them)", "auto-triage on",
		"RDINC (1)", "RDODCF (1)", "RDODCF-7699", "triage session", "RDINC-94264", "Rafael Seara", "Waiting for customer", "no launcher"} {
		if !strings.Contains(view, want) {
			t.Errorf("view is missing %q:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"RDODCF-4652", "RDODCF-1 "} {
		if strings.Contains(view, unwanted) {
			t.Errorf("view lists %q:\n%s", unwanted, view)
		}
	}
}

func TestModel_HiddenToggle_ShowsDismissedTickets(t *testing.T) {
	m, _ := fixture(t)

	updated, _ := m.Update(letter('h'))
	view := ansi.Strip(updated.(Model).View().Content)

	if !strings.Contains(view, "RDODCF-4652") || !strings.Contains(view, "dismissed") {
		t.Fatalf("h did not show the dismissed ticket:\n%s", view)
	}
}

func TestModel_View_BeforeTheFirstPoll_SaysLoading(t *testing.T) {
	m := New(styles.New())
	m.SetSize(80, 10)
	m.SetData(Data{})

	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "Loading tickets") {
		t.Fatalf("view = %q, want a loading notice", view)
	}
}

func TestModel_View_LoadError_IsShownInsteadOfLoading(t *testing.T) {
	m := New(styles.New())
	m.SetSize(80, 10)
	m.SetData(Data{LoadError: "acli: not logged in"})

	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "Could not load the inbox: acli: not logged in") {
		t.Fatalf("view = %q", view)
	}
}

func TestModel_Keys_EmitTheirMessages(t *testing.T) {
	// Rows: RDINC group, RDINC-94264, RDODCF group, RDODCF-7699.
	m, _ := fixture(t)
	updated, _ := m.Update(letter('j'))
	m = updated.(Model)

	tests := []struct {
		name string
		key  tea.KeyPressMsg
		want any
	}{
		{name: "enter opens", key: enter(), want: shared.IssueOpenMsg{}},
		{name: "r triages", key: letter('r'), want: shared.IssueTriageMsg{}},
		{name: "d dismisses", key: letter('d'), want: shared.IssueDismissMsg{}},
		{name: "o opens the browser", key: letter('o'), want: shared.IssueOpenInBrowserMsg{}},
		{name: "a toggles auto-triage", key: letter('a'), want: shared.AutoTriageToggleMsg{}},
		{name: "esc closes", key: tea.KeyPressMsg{Code: tea.KeyEscape}, want: shared.PageCloseMsg{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cmd := m.Update(tt.key)
			if cmd == nil {
				t.Fatal("cmd = nil")
			}
			msg := cmd()
			if fmt.Sprintf("%T", msg) != fmt.Sprintf("%T", tt.want) {
				t.Fatalf("msg = %T, want %T", msg, tt.want)
			}
			if open, ok := msg.(shared.IssueOpenMsg); ok && open.Issue.Key != "RDINC-94264" {
				t.Fatalf("opened %q, want the selected RDINC-94264", open.Issue.Key)
			}
		})
	}
}

func TestModel_GroupRow_EnterDoesNothing(t *testing.T) {
	m, _ := fixture(t)

	if _, cmd := m.Update(enter()); cmd != nil {
		if _, isOpen := cmd().(shared.IssueOpenMsg); isOpen {
			t.Fatal("enter on a project row opened an issue")
		}
	}
}
