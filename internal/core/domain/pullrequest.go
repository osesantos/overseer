package domain

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"
)

type PRState string

const (
	PRStateOpen   PRState = "OPEN"
	PRStateClosed PRState = "CLOSED"
	PRStateMerged PRState = "MERGED"
	PRStateDraft  PRState = "DRAFT"
)

type CheckConclusion string

const (
	CheckConclusionSuccess   CheckConclusion = "SUCCESS"
	CheckConclusionFailure   CheckConclusion = "FAILURE"
	CheckConclusionPending   CheckConclusion = "PENDING"
	CheckConclusionSkipped   CheckConclusion = "SKIPPED"
	CheckConclusionCancelled CheckConclusion = "CANCELLED"
)

type PRStats struct {
	Additions    int
	Deletions    int
	ChangedFiles int
}

func (s PRStats) TotalChanges() int { return s.Additions + s.Deletions }

type PRChecks struct {
	Total   int
	Passing int
	Failing int
	Pending int
	Skipped int
}

func (c PRChecks) OverallConclusion() CheckConclusion {
	switch {
	case c.Failing > 0:
		return CheckConclusionFailure
	case c.Pending > 0:
		return CheckConclusionPending
	case c.Total > 0 && c.Skipped == c.Total:
		return CheckConclusionSkipped
	case c.Passing > 0 && c.Failing == 0 && c.Pending == 0:
		return CheckConclusionSuccess
	default:
		return CheckConclusionPending
	}
}

// PRComments counts review threads with input from someone other than the viewer.
// AwaitingViewer is the subset of Unresolved where someone else spoke last.
type PRComments struct {
	Resolved       int
	Unresolved     int
	AwaitingViewer int
}

func (c PRComments) Total() int { return c.Resolved + c.Unresolved }

// PRReviewThread is one review thread; Authors lists human comment authors in order, bots excluded.
type PRReviewThread struct {
	Resolved bool
	Authors  []string
}

// SummariseReviewThreads ignores threads only the viewer took part in.
func SummariseReviewThreads(threads []PRReviewThread, viewer string) PRComments {
	var c PRComments
	for _, th := range threads {
		if !slices.ContainsFunc(th.Authors, func(a string) bool { return a != viewer }) {
			continue
		}
		if th.Resolved {
			c.Resolved++
			continue
		}
		c.Unresolved++
		if th.Authors[len(th.Authors)-1] != viewer {
			c.AwaitingViewer++
		}
	}
	return c
}

type PullRequest struct {
	Number int
	// Repo is the GitHub "owner/name"; empty when fetched for a local branch.
	Repo        string
	Title       string
	URL         string
	Branch      string
	State       PRState
	Author      string
	AuthorIsBot bool
	// ReviewedByViewer: a review by the viewer exists on GitHub, or the viewer
	// closed a review session for this PR by hand. Either way, no auto-review.
	ReviewedByViewer bool
	// Dismissed: the operator hid this PR from the inbox; it is never auto-reviewed.
	Dismissed bool
	Stats     PRStats
	Checks    PRChecks
	Comments  PRComments
	CreatedAt time.Time
	UpdatedAt time.Time
	FetchedAt time.Time
}

// PullRequestRef identifies a PR across repositories; it is what a review session
// records so the same PR is never picked up twice.
type PullRequestRef struct {
	Repo   string
	Number int
}

func (p PullRequest) Ref() PullRequestRef { return PullRequestRef{Repo: p.Repo, Number: p.Number} }

// RepoName is the repository name without its owner, which is how a registered
// Project is named when it was cloned under its GitHub name.
func (p PullRequest) RepoName() string {
	_, name, _ := strings.Cut(p.Repo, "/")
	if name == "" {
		return p.Repo
	}
	return name
}

// ReviewFinished reports whether sess exists to review pr and pr no longer needs
// one: merged or closed. Such a session is deleted, worktree included.
func ReviewFinished(sess Session, pr PullRequest) bool {
	return sess.IsReview() && (pr.State == PRStateMerged || pr.State == PRStateClosed)
}

// ReviewCommandNumberToken is the placeholder in a review launcher command
// that the PR number replaces.
const ReviewCommandNumberToken = "{{number}}"

// ReviewInbox is every open PR with a review requested from Viewer.
type ReviewInbox struct {
	Viewer       string
	PullRequests []PullRequest
}

// EligibleForAutoReview reports whether a review session should be started for
// pr: an open, non-draft PR by another human, in a registered project, with no
// review session yet, no review by the viewer and not dismissed.
func EligibleForAutoReview(pr PullRequest, viewer string, projectNames map[string]bool, taken map[PullRequestRef]bool) bool {
	return pr.State == PRStateOpen &&
		!pr.AuthorIsBot &&
		pr.Author != viewer &&
		!pr.ReviewedByViewer &&
		!pr.Dismissed &&
		projectNames[pr.RepoName()] &&
		!taken[pr.Ref()]
}

func NewPullRequest(number int, title, branch string, state PRState) (PullRequest, error) {
	if number <= 0 {
		return PullRequest{}, ErrPullRequestInvalidNumber
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return PullRequest{}, ErrPullRequestEmptyBranch
	}
	switch state {
	case PRStateOpen, PRStateClosed, PRStateMerged, PRStateDraft:
	default:
		return PullRequest{}, ErrPullRequestInvalidState
	}
	return PullRequest{
		Number:    number,
		Title:     strings.TrimSpace(title),
		Branch:    branch,
		State:     state,
		FetchedAt: time.Now(),
	}, nil
}

type PullRequestPort interface {
	// GetForBranch fetches the PR for a selector `gh pr view` understands: a head
	// branch name or a PR number.
	GetForBranch(ctx context.Context, repoPath, branch string) (PullRequest, error)
	// ListRequestedReviews fetches every open PR with a review requested from the
	// authenticated user, directly or through a team.
	ListRequestedReviews(ctx context.Context) (ReviewInbox, error)
	// OpenInBrowser opens the PR page in the operator's default browser.
	OpenInBrowser(ctx context.Context, prURL string) error
}

// ReviewSettings is what the operator decided about the inbox, kept across
// restarts: an auto-review override (nil means use the config value), PRs hidden
// from the inbox, and PRs whose review session was closed by hand and must not
// be opened again.
type ReviewSettings struct {
	AutoReview *bool            `json:",omitempty"`
	Dismissed  []PullRequestRef `json:",omitempty"`
	Handled    []PullRequestRef `json:",omitempty"`
}

// ReviewSettingsRepository persists ReviewSettings; every mutation is atomic.
type ReviewSettingsRepository interface {
	Load(ctx context.Context) (ReviewSettings, error)
	Dismiss(ctx context.Context, ref PullRequestRef) error
	MarkHandled(ctx context.Context, ref PullRequestRef) error
	SetAutoReview(ctx context.Context, on bool) error
}

var (
	ErrPullRequestNotFound      = errors.New("pull request not found")
	ErrPullRequestInvalidNumber = errors.New("pull request number must be positive")
	ErrPullRequestEmptyBranch   = errors.New("pull request branch cannot be empty")
	ErrPullRequestEmptyRepo     = errors.New("pull request repo cannot be empty")
	ErrPullRequestInvalidState  = errors.New("pull request state must be OPEN, CLOSED, MERGED, or DRAFT")
)
