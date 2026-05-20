package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

type PRState string

const (
	PRStateOpen   PRState = "OPEN"
	PRStateClosed PRState = "CLOSED"
	PRStateMerged PRState = "MERGED"
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

type PullRequest struct {
	Number    int
	Title     string
	URL       string
	Branch    string
	State     PRState
	Author    string
	Stats     PRStats
	Checks    PRChecks
	UpdatedAt time.Time
	FetchedAt time.Time
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
	case PRStateOpen, PRStateClosed, PRStateMerged:
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
	GetForBranch(ctx context.Context, repoPath, branch string) (PullRequest, error)
}

var (
	ErrPullRequestNotFound      = errors.New("pull request not found")
	ErrPullRequestInvalidNumber = errors.New("pull request number must be positive")
	ErrPullRequestEmptyBranch   = errors.New("pull request branch cannot be empty")
	ErrPullRequestInvalidState  = errors.New("pull request state must be OPEN, CLOSED, or MERGED")
)
