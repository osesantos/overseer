package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEligibleForAutoTriage(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	fresh := Issue{Key: "RDODCF-1", Project: "RDODCF", StatusCategory: IssueStatusNew, CreatedAt: now.Add(-time.Hour)}
	launchers := map[string]bool{"RDODCF": true}
	with := func(edit func(*Issue)) Issue {
		i := fresh
		edit(&i)
		return i
	}

	tests := []struct {
		name  string
		issue Issue
		taken map[string]bool
		want  bool
	}{
		{name: "fresh unassigned ticket", issue: fresh, want: true},
		{name: "in progress is still open", issue: with(func(i *Issue) { i.StatusCategory = IssueStatusInProgress }), want: true},
		{name: "assigned", issue: with(func(i *Issue) { i.Assignee = "Vera Cardoso" })},
		{name: "done", issue: with(func(i *Issue) { i.StatusCategory = IssueStatusDone })},
		{name: "dismissed", issue: with(func(i *Issue) { i.Dismissed = true })},
		{name: "handled", issue: with(func(i *Issue) { i.Handled = true })},
		{name: "older than max age", issue: with(func(i *Issue) { i.CreatedAt = now.Add(-8 * 24 * time.Hour) })},
		{name: "unknown creation time", issue: with(func(i *Issue) { i.CreatedAt = time.Time{} })},
		{name: "no launcher for its project", issue: with(func(i *Issue) { i.Key, i.Project = "RDINC-1", "RDINC" })},
		{name: "already has a triage session", issue: fresh, taken: map[string]bool{"RDODCF-1": true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EligibleForAutoTriage(tt.issue, now, 7*24*time.Hour, launchers, tt.taken); got != tt.want {
				t.Fatalf("EligibleForAutoTriage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTriageFinished(t *testing.T) {
	triage, _ := NewSession("RDODCF-1 x", uuid.New())
	_ = triage.AssignTriageTarget("RDODCF-1")
	plain, _ := NewSession("plain", uuid.New())
	done := Issue{Key: "RDODCF-1", StatusCategory: IssueStatusDone}

	tests := []struct {
		name  string
		sess  Session
		issue Issue
		want  bool
	}{
		{name: "triage session, ticket done", sess: triage, issue: done, want: true},
		{name: "triage session, ticket open", sess: triage, issue: Issue{Key: "RDODCF-1", StatusCategory: IssueStatusInProgress}},
		{name: "triage session, another ticket done", sess: triage, issue: Issue{Key: "RDODCF-2", StatusCategory: IssueStatusDone}},
		{name: "ordinary session", sess: plain, issue: done},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TriageFinished(tt.sess, tt.issue); got != tt.want {
				t.Fatalf("TriageFinished() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIssueProjectKey(t *testing.T) {
	tests := map[string]string{"RDODCF-7699": "RDODCF", "RDINC-1": "RDINC", "NODASH": "NODASH"}
	for key, want := range tests {
		if got := IssueProjectKey(key); got != want {
			t.Fatalf("IssueProjectKey(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestSession_AssignTriageTarget(t *testing.T) {
	sess, _ := NewSession("RDODCF-1", uuid.New())

	if err := sess.AssignTriageTarget("  "); !errors.Is(err, ErrIssueEmptyKey) {
		t.Fatalf("AssignTriageTarget(blank) error = %v, want ErrIssueEmptyKey", err)
	}
	if sess.IsTriage() {
		t.Fatal("IsTriage() = true after a rejected assignment")
	}
	if err := sess.AssignTriageTarget("RDODCF-1"); err != nil {
		t.Fatalf("AssignTriageTarget() error = %v", err)
	}
	if !sess.IsTriage() || sess.TriageIssue != "RDODCF-1" {
		t.Fatalf("IsTriage() = %v, TriageIssue = %q", sess.IsTriage(), sess.TriageIssue)
	}
}
