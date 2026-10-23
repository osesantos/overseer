package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

// IssueStatusCategory is Jira's coarse status bucket, which stays stable while
// each project names its statuses differently.
type IssueStatusCategory string

const (
	IssueStatusNew        IssueStatusCategory = "new"
	IssueStatusInProgress IssueStatusCategory = "indeterminate"
	IssueStatusDone       IssueStatusCategory = "done"
)

// Issue is a support ticket (RDODCF, RDINC) that a triage session can be
// started for.
type Issue struct {
	Key            string
	Project        string
	Summary        string
	Status         string
	StatusCategory IssueStatusCategory
	// Assignee is the display name; empty when unassigned.
	Assignee string
	Priority string
	URL      string
	// CreatedAt is zero when the tracker could not say; such an issue is never auto-triaged.
	CreatedAt time.Time
	// Dismissed: the operator hid this issue from the inbox; it is never auto-triaged.
	Dismissed bool
	// Handled: the operator closed this issue's triage session by hand; it is never auto-triaged again.
	Handled bool
}

// IssueProjectKey is the project part of an issue key: "RDODCF" for "RDODCF-7699".
func IssueProjectKey(key string) string {
	project, _, _ := strings.Cut(key, "-")
	return project
}

// TriageCommandKeyToken is the placeholder in a triage launcher command that
// the issue key replaces.
const TriageCommandKeyToken = "{{key}}"

// TriageInbox is every issue the triage query returns, plus the issues that
// triage sessions still track after they left that query.
type TriageInbox struct {
	Issues []Issue
}

// EligibleForAutoTriage reports whether a triage session should be started for
// issue: not done, unassigned, created within maxAge, not dismissed or handled,
// with a launcher for its project and no triage session yet.
func EligibleForAutoTriage(issue Issue, now time.Time, maxAge time.Duration, launchers map[string]bool, taken map[string]bool) bool {
	return issue.StatusCategory != IssueStatusDone &&
		issue.Assignee == "" &&
		!issue.Dismissed &&
		!issue.Handled &&
		!issue.CreatedAt.IsZero() &&
		now.Sub(issue.CreatedAt) <= maxAge &&
		launchers[issue.Project] &&
		!taken[issue.Key]
}

// TriageFinished reports whether sess triages issue and issue is done. Such a
// session is deleted.
func TriageFinished(sess Session, issue Issue) bool {
	return sess.IsTriage() && sess.TriageIssue == issue.Key && issue.StatusCategory == IssueStatusDone
}

type IssueTrackerPort interface {
	// ListIssues runs a tracker query and returns every matching issue.
	ListIssues(ctx context.Context, query string) ([]Issue, error)
	// GetIssues fetches the given issues by key, whatever their status.
	GetIssues(ctx context.Context, keys []string) ([]Issue, error)
	// OpenInBrowser opens the issue page in the operator's default browser.
	OpenInBrowser(ctx context.Context, key string) error
}

// TriageSettings is what the operator decided about the triage inbox, kept
// across restarts: an auto-triage override (nil means use the config value),
// issues hidden from the inbox, and issues whose triage session was closed by hand.
type TriageSettings struct {
	AutoTriage *bool    `json:",omitempty"`
	Dismissed  []string `json:",omitempty"`
	Handled    []string `json:",omitempty"`
}

// TriageSettingsRepository persists TriageSettings; every mutation is atomic.
type TriageSettingsRepository interface {
	Load(ctx context.Context) (TriageSettings, error)
	Dismiss(ctx context.Context, key string) error
	MarkHandled(ctx context.Context, key string) error
	SetAutoTriage(ctx context.Context, on bool) error
}

var ErrIssueEmptyKey = errors.New("issue key cannot be empty")
