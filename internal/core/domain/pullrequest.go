package domain

import (
	"context"
	"errors"
	"fmt"
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

// PRReviewState is a reviewer's latest verdict, in GitHub's vocabulary, plus
// PRReviewStateRequested for a reviewer who was asked and has not answered.
type PRReviewState string

const (
	PRReviewStateApproved         PRReviewState = "APPROVED"
	PRReviewStateChangesRequested PRReviewState = "CHANGES_REQUESTED"
	PRReviewStateCommented        PRReviewState = "COMMENTED"
	PRReviewStateDismissed        PRReviewState = "DISMISSED"
	PRReviewStatePending          PRReviewState = "PENDING"
	PRReviewStateRequested        PRReviewState = "REQUESTED"
)

// PRReviewer is one person or team on the PR's review list with their latest review.
type PRReviewer struct {
	Login string
	State PRReviewState
}

// PRReviewDecision is GitHub's overall review verdict; empty when the base
// branch requires no review.
type PRReviewDecision string

const (
	PRReviewDecisionApproved         PRReviewDecision = "APPROVED"
	PRReviewDecisionChangesRequested PRReviewDecision = "CHANGES_REQUESTED"
	PRReviewDecisionReviewRequired   PRReviewDecision = "REVIEW_REQUIRED"
)

// PRMergeState is GitHub's mergeStateStatus: whether the merge button would work.
type PRMergeState string

const (
	PRMergeStateClean    PRMergeState = "CLEAN"
	PRMergeStateHasHooks PRMergeState = "HAS_HOOKS"
	PRMergeStateUnstable PRMergeState = "UNSTABLE"
	PRMergeStateBlocked  PRMergeState = "BLOCKED"
	PRMergeStateBehind   PRMergeState = "BEHIND"
	PRMergeStateDirty    PRMergeState = "DIRTY"
	PRMergeStateDraft    PRMergeState = "DRAFT"
	PRMergeStateUnknown  PRMergeState = "UNKNOWN"
)

// MergeReadiness is whether GitHub would merge the PR as it stands. Known is
// false until GitHub has computed the merge state, or when the PR was fetched
// without it. Reasons explain a block, or warn on a PR that is still mergeable.
type MergeReadiness struct {
	Known   bool
	Ready   bool
	Reasons []string
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
// Path, Line, Outdated, DiffHunk and Comments are only filled by GetDetails.
type PRReviewThread struct {
	Resolved bool
	Outdated bool
	Path     string
	Line     int
	DiffHunk string
	Authors  []string
	Comments []PRComment
}

// PRComment is one message on a pull request: an issue comment, a review
// summary (ReviewState set) or a comment inside a review thread.
type PRComment struct {
	Author      string
	AuthorIsBot bool
	Body        string
	CreatedAt   time.Time
	ReviewState string
}

// PRFile is one changed file with its line counts.
type PRFile struct {
	Path       string
	Additions  int
	Deletions  int
	ChangeType string
}

// PullRequestDetails is everything the full-screen page shows: the PR, its
// files, the raw unified diff, the conversation (issue comments and review
// summaries, oldest first) and the review threads.
type PullRequestDetails struct {
	PullRequest  PullRequest
	Files        []PRFile
	Diff         string
	Conversation []PRComment
	Threads      []PRReviewThread
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
	Repo   string
	Title  string
	URL    string
	Branch string
	// Body is the description in markdown; BaseBranch and Labels come from the
	// same gh call and are shown by the inspector's PR tab.
	Body        string
	BaseBranch  string
	Labels      []string
	State       PRState
	Author      string
	AuthorIsBot bool
	// ReviewedByViewer: a review by the viewer exists on GitHub, or the viewer
	// closed a review session for this PR by hand. Either way, no auto-review.
	ReviewedByViewer bool
	// ViewerReviewPending: the viewer has a review staged on GitHub but not submitted.
	ViewerReviewPending bool
	// ViewerReviewRequested: GitHub is waiting on a review from the viewer, directly
	// or through a team. Filled by the per-minute status poll only.
	ViewerReviewRequested bool
	// ViewerReview is the viewer's latest review state; empty when they never
	// reviewed. Filled by the per-minute status poll only.
	ViewerReview PRReviewState
	// Dismissed: the operator hid this PR from the inbox; it is never auto-reviewed.
	Dismissed bool
	Stats     PRStats
	Checks    PRChecks
	Comments  PRComments
	// Reviewers is only filled by GetDetails; the inbox leaves ReviewDecision and
	// MergeState empty too.
	ReviewDecision PRReviewDecision
	MergeState     PRMergeState
	Reviewers      []PRReviewer
	CreatedAt      time.Time
	UpdatedAt      time.Time
	FetchedAt      time.Time
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

// MergeReadiness reads the merge state GitHub computed and spells out why a
// blocked PR is blocked: the review decision first, then the checks.
func (p PullRequest) MergeReadiness() MergeReadiness {
	r := MergeReadiness{Known: true}
	switch p.MergeState {
	case "", PRMergeStateUnknown:
		return MergeReadiness{}
	case PRMergeStateClean, PRMergeStateHasHooks:
		r.Ready = true
	case PRMergeStateUnstable:
		r.Ready = true
		r.Reasons = p.Checks.reasons()
	case PRMergeStateDraft:
		r.Reasons = []string{"draft"}
	case PRMergeStateDirty:
		r.Reasons = []string{"merge conflicts"}
	case PRMergeStateBehind:
		r.Reasons = []string{"behind " + p.BaseBranch}
	default:
		switch p.ReviewDecision {
		case PRReviewDecisionChangesRequested:
			r.Reasons = []string{"changes requested"}
		case PRReviewDecisionReviewRequired:
			r.Reasons = []string{"review required"}
		}
		r.Reasons = append(r.Reasons, p.Checks.reasons()...)
		if len(r.Reasons) == 0 {
			r.Reasons = []string{"blocked by branch rules"}
		}
	}
	return r
}

// ViewerVerdict is where the viewer stands on the PR: a staged review first,
// then an open request (a re-request outranks an older review), then their
// latest review. Empty when none applies.
func (p PullRequest) ViewerVerdict() PRReviewState {
	switch {
	case p.ViewerReviewPending || p.ViewerReview == PRReviewStatePending:
		return PRReviewStatePending
	case p.ViewerReviewRequested:
		return PRReviewStateRequested
	}
	return p.ViewerReview
}

// reasons names failing and pending checks, with counts.
func (c PRChecks) reasons() []string {
	var out []string
	if c.Failing > 0 {
		out = append(out, fmt.Sprintf("%d check%s failing", c.Failing, plural(c.Failing)))
	}
	if c.Pending > 0 {
		out = append(out, fmt.Sprintf("%d check%s pending", c.Pending, plural(c.Pending)))
	}
	return out
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// ReviewFinished reports whether sess exists to review pr and pr no longer needs
// one: merged or closed. Such a session is deleted, worktree included.
func ReviewFinished(sess Session, pr PullRequest) bool {
	return sess.IsReview() && (pr.State == PRStateMerged || pr.State == PRStateClosed)
}

// ReviewCommandNumberToken is the placeholder in a review launcher command
// that the PR number replaces.
const ReviewCommandNumberToken = "{{number}}"

// ReviewInbox is every open PR with a review requested from Viewer, plus the
// open PRs Viewer already reviewed (GitHub clears the request on review).
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
	// authenticated user, directly or through a team, plus the open PRs they
	// already reviewed.
	ListRequestedReviews(ctx context.Context) (ReviewInbox, error)
	// OpenInBrowser opens the PR page in the operator's default browser.
	OpenInBrowser(ctx context.Context, prURL string) error
	// GetDetails fetches what the PR page shows (files, diff, conversation,
	// review threads) for the PR number, run from repoPath.
	GetDetails(ctx context.Context, repoPath string, number int) (PullRequestDetails, error)
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
	Restore(ctx context.Context, ref PullRequestRef) error
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
