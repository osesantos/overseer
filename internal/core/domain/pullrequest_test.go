package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewPullRequest_CreatesPullRequest(t *testing.T) {
	before := time.Now()

	pr, err := NewPullRequest(42, "Add feature", "feature/branch", PRStateOpen)

	if err != nil {
		t.Fatalf("NewPullRequest() error = %v", err)
	}
	if pr.Number != 42 {
		t.Fatalf("NewPullRequest() Number = %d, want 42", pr.Number)
	}
	if pr.Title != "Add feature" {
		t.Fatalf("NewPullRequest() Title = %q, want %q", pr.Title, "Add feature")
	}
	if pr.Branch != "feature/branch" {
		t.Fatalf("NewPullRequest() Branch = %q, want %q", pr.Branch, "feature/branch")
	}
	if pr.State != PRStateOpen {
		t.Fatalf("NewPullRequest() State = %q, want %q", pr.State, PRStateOpen)
	}
	if pr.FetchedAt.Before(before) {
		t.Fatalf("NewPullRequest() FetchedAt = %v, before creation start %v", pr.FetchedAt, before)
	}
}

func TestNewPullRequest_TrimsTitleAndBranch(t *testing.T) {
	pr, err := NewPullRequest(1, "  Hello  ", "  main  ", PRStateOpen)

	if err != nil {
		t.Fatalf("NewPullRequest() error = %v", err)
	}
	if pr.Title != "Hello" {
		t.Fatalf("NewPullRequest() Title = %q, want %q", pr.Title, "Hello")
	}
	if pr.Branch != "main" {
		t.Fatalf("NewPullRequest() Branch = %q, want %q", pr.Branch, "main")
	}
}

func TestNewPullRequest_LeavesStatsAndChecksZeroByDefault(t *testing.T) {
	pr, err := NewPullRequest(1, "Title", "branch", PRStateOpen)

	if err != nil {
		t.Fatalf("NewPullRequest() error = %v", err)
	}
	if pr.Stats != (PRStats{}) {
		t.Fatalf("NewPullRequest() Stats = %+v, want zero-value", pr.Stats)
	}
	if pr.Checks != (PRChecks{}) {
		t.Fatalf("NewPullRequest() Checks = %+v, want zero-value", pr.Checks)
	}
}

func TestNewPullRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		number  int
		title   string
		branch  string
		state   PRState
		wantErr error
	}{
		{name: "zero number", number: 0, title: "t", branch: "b", state: PRStateOpen, wantErr: ErrPullRequestInvalidNumber},
		{name: "negative number", number: -1, title: "t", branch: "b", state: PRStateOpen, wantErr: ErrPullRequestInvalidNumber},
		{name: "empty branch", number: 1, title: "t", branch: "", state: PRStateOpen, wantErr: ErrPullRequestEmptyBranch},
		{name: "blank branch", number: 1, title: "t", branch: "   ", state: PRStateOpen, wantErr: ErrPullRequestEmptyBranch},
		{name: "invalid state", number: 1, title: "t", branch: "b", state: PRState("WEIRD"), wantErr: ErrPullRequestInvalidState},
		{name: "empty state", number: 1, title: "t", branch: "b", state: PRState(""), wantErr: ErrPullRequestInvalidState},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPullRequest(tt.number, tt.title, tt.branch, tt.state)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewPullRequest() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewPullRequest_AcceptsAllValidStates(t *testing.T) {
	states := []PRState{PRStateOpen, PRStateClosed, PRStateMerged}
	for _, st := range states {
		t.Run(string(st), func(t *testing.T) {
			pr, err := NewPullRequest(1, "t", "b", st)
			if err != nil {
				t.Fatalf("NewPullRequest(state=%q) error = %v", st, err)
			}
			if pr.State != st {
				t.Fatalf("NewPullRequest() State = %q, want %q", pr.State, st)
			}
		})
	}
}

func TestNewPullRequest_AcceptsEmptyTitle(t *testing.T) {
	pr, err := NewPullRequest(1, "", "branch", PRStateOpen)
	if err != nil {
		t.Fatalf("NewPullRequest() error = %v, want nil for empty title", err)
	}
	if pr.Title != "" {
		t.Fatalf("NewPullRequest() Title = %q, want empty", pr.Title)
	}
}

func TestPRStats_TotalChanges_AdditionsPlusDeletions(t *testing.T) {
	s := PRStats{Additions: 10, Deletions: 4, ChangedFiles: 3}
	if got := s.TotalChanges(); got != 14 {
		t.Fatalf("TotalChanges() = %d, want 14", got)
	}
}

func TestPRChecks_OverallConclusion(t *testing.T) {
	tests := []struct {
		name string
		c    PRChecks
		want CheckConclusion
	}{
		{name: "no checks at all", c: PRChecks{}, want: CheckConclusionPending},
		{name: "all passing", c: PRChecks{Total: 3, Passing: 3}, want: CheckConclusionSuccess},
		{name: "any failing", c: PRChecks{Total: 3, Passing: 2, Failing: 1}, want: CheckConclusionFailure},
		{name: "any pending without failures", c: PRChecks{Total: 3, Passing: 2, Pending: 1}, want: CheckConclusionPending},
		{name: "all skipped", c: PRChecks{Total: 2, Skipped: 2}, want: CheckConclusionSkipped},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.OverallConclusion(); got != tt.want {
				t.Fatalf("OverallConclusion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPullRequestSentinelErrors_AreDistinct(t *testing.T) {
	all := []error{
		ErrPullRequestNotFound,
		ErrPullRequestInvalidNumber,
		ErrPullRequestEmptyBranch,
		ErrPullRequestInvalidState,
	}
	seen := map[string]struct{}{}
	for _, e := range all {
		msg := e.Error()
		if _, dup := seen[msg]; dup {
			t.Fatalf("duplicate sentinel error message: %q", msg)
		}
		seen[msg] = struct{}{}
		if !strings.Contains(msg, "pull request") {
			t.Fatalf("sentinel error %q should mention 'pull request'", msg)
		}
	}
}
