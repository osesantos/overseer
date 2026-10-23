package jira

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dnlopes/overseer/internal/core/domain"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("readFixture(%q) error = %v", name, err)
	}
	return data
}

// fakeCommander answers by the first argument after "workitem" and, for view,
// by the issue key; it is safe for the adapter's concurrent views.
type fakeCommander struct {
	mu     sync.Mutex
	search []byte
	views  map[string][]byte
	calls  [][]string
}

func (f *fakeCommander) Run(_ context.Context, args ...string) ([]byte, []byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, args)
	switch args[2] {
	case "search":
		return f.search, nil, nil
	case "view":
		if out, ok := f.views[args[3]]; ok {
			return out, nil, nil
		}
	}
	return nil, []byte("✗ Error: Issue does not exist"), errors.New("exit status 1")
}

func TestAdapter_ListIssues_SearchesKeysThenViewsEach(t *testing.T) {
	cmd := &fakeCommander{
		search: readFixture(t, "search_keys.json"),
		views: map[string][]byte{
			"RDODCF-4652": readFixture(t, "view_unassigned.json"),
			"RDINC-94264": readFixture(t, "view_assigned.json"),
		},
	}

	issues, err := NewWithCommander(cmd, testLogger()).ListIssues(context.Background(), "project = RDODCF")
	if err != nil {
		t.Fatalf("ListIssues() error = %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("ListIssues() returned %d issues, want 2", len(issues))
	}

	first := issues[0]
	wantCreated := time.Date(2026, 7, 2, 10, 59, 33, 557_000_000, time.UTC)
	if first.Key != "RDODCF-4652" || first.Project != "RDODCF" || first.Status != "Backlog" ||
		first.StatusCategory != domain.IssueStatusNew || first.Assignee != "" || first.Priority != "Normal" ||
		!first.CreatedAt.Equal(wantCreated) || first.URL != "https://outsystemsrd.atlassian.net/browse/RDODCF-4652" ||
		!strings.HasPrefix(first.Summary, "Deployment Failure:") {
		t.Fatalf("first issue = %+v", first)
	}
	if second := issues[1]; second.Key != "RDINC-94264" || second.Assignee != "Rafael Seara" || second.StatusCategory != "undefined" {
		t.Fatalf("second issue = %+v", second)
	}

	search := cmd.calls[0]
	if !slices.Contains(search, "--jql") || !slices.Contains(search, "project = RDODCF") || !slices.Contains(search, "--json") {
		t.Fatalf("search args = %v", search)
	}
}

func TestAdapter_ListIssues_FailedViewKeepsAKeyOnlyIssue(t *testing.T) {
	cmd := &fakeCommander{search: readFixture(t, "search_keys.json"), views: map[string][]byte{
		"RDODCF-4652": readFixture(t, "view_unassigned.json"),
	}}

	issues, err := NewWithCommander(cmd, testLogger()).ListIssues(context.Background(), "q")
	if err != nil {
		t.Fatalf("ListIssues() error = %v", err)
	}
	stub := issues[1]
	if stub.Key != "RDINC-94264" || stub.Project != "RDINC" || !stub.CreatedAt.IsZero() || stub.StatusCategory == domain.IssueStatusDone {
		t.Fatalf("stub = %+v, want key-only, zero CreatedAt, not done", stub)
	}
}

func TestAdapter_ListIssues_SearchFailure_ReturnsError(t *testing.T) {
	cmd := &fakeCommander{}

	if _, err := NewWithCommander(cmd, testLogger()).ListIssues(context.Background(), "q"); err == nil {
		t.Fatal("ListIssues() error = nil, want the search failure")
	}
}

func TestAdapter_ListIssues_NullHits_ReturnsError(t *testing.T) {
	cmd := &fakeCommander{search: readFixture(t, "search_nulls.json")}

	if _, err := NewWithCommander(cmd, testLogger()).ListIssues(context.Background(), "q"); !errors.Is(err, domain.ErrIssueEmptyKey) {
		t.Fatalf("ListIssues() error = %v, want ErrIssueEmptyKey", err)
	}
}

func TestAdapter_GetIssues_ViewsOnlyTheGivenKeys(t *testing.T) {
	cmd := &fakeCommander{views: map[string][]byte{"RDODCF-4652": readFixture(t, "view_unassigned.json")}}

	issues, err := NewWithCommander(cmd, testLogger()).GetIssues(context.Background(), []string{"RDODCF-4652"})
	if err != nil {
		t.Fatalf("GetIssues() error = %v", err)
	}
	if len(issues) != 1 || issues[0].StatusCategory != domain.IssueStatusNew || len(cmd.calls) != 1 {
		t.Fatalf("issues = %+v, calls = %v", issues, cmd.calls)
	}
}

func TestAdapter_OpenInBrowser_UsesViewWeb(t *testing.T) {
	cmd := &fakeCommander{views: map[string][]byte{"RDODCF-1": nil}}

	if err := NewWithCommander(cmd, testLogger()).OpenInBrowser(context.Background(), "RDODCF-1"); err != nil {
		t.Fatalf("OpenInBrowser() error = %v", err)
	}
	if got := cmd.calls[0]; !slices.Equal(got, []string{"jira", "workitem", "view", "RDODCF-1", "--web"}) {
		t.Fatalf("args = %v", got)
	}
}
