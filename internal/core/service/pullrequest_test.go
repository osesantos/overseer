package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/testutil/mocks"
)

func TestPullRequestService_GetForBranch_HappyPath(t *testing.T) {
	port := mocks.NewMockPullRequestPort(t)
	wantPR, _ := domain.NewPullRequest(7, "Add background jobs", "feature/jobs", domain.PRStateOpen)
	port.EXPECT().
		GetForBranch(mock.Anything, "/repo", "feature/jobs").
		Return(wantPR, nil).Once()

	svc := NewPullRequestService(port, testLogger())
	resp, err := svc.GetForBranch(context.Background(), GetPullRequestForBranchRequest{
		RepoPath: "/repo",
		Branch:   "feature/jobs",
	})

	if err != nil {
		t.Fatalf("GetForBranch() error = %v", err)
	}
	if resp.PullRequest.Number != 7 {
		t.Fatalf("GetForBranch() Number = %d, want 7", resp.PullRequest.Number)
	}
	if resp.PullRequest.Branch != "feature/jobs" {
		t.Fatalf("GetForBranch() Branch = %q, want %q", resp.PullRequest.Branch, "feature/jobs")
	}
}

func TestPullRequestService_GetForBranch_NotFound_ReturnsSentinelUnwrapped(t *testing.T) {
	port := mocks.NewMockPullRequestPort(t)
	port.EXPECT().
		GetForBranch(mock.Anything, "/repo", "main").
		Return(domain.PullRequest{}, domain.ErrPullRequestNotFound).Once()

	svc := NewPullRequestService(port, testLogger())
	_, err := svc.GetForBranch(context.Background(), GetPullRequestForBranchRequest{
		RepoPath: "/repo",
		Branch:   "main",
	})

	if !errors.Is(err, domain.ErrPullRequestNotFound) {
		t.Fatalf("GetForBranch() error = %v, want ErrPullRequestNotFound", err)
	}
}

func TestPullRequestService_GetForBranch_PortError_IsWrapped(t *testing.T) {
	port := mocks.NewMockPullRequestPort(t)
	boom := errors.New("gh exit 1")
	port.EXPECT().
		GetForBranch(mock.Anything, "/repo", "main").
		Return(domain.PullRequest{}, boom).Once()

	svc := NewPullRequestService(port, testLogger())
	_, err := svc.GetForBranch(context.Background(), GetPullRequestForBranchRequest{
		RepoPath: "/repo",
		Branch:   "main",
	})

	if err == nil {
		t.Fatal("GetForBranch() error = nil, want wrapped error")
	}
	if !errors.Is(err, boom) {
		t.Fatalf("GetForBranch() error chain missing root cause; got %v", err)
	}
	if err.Error() == boom.Error() {
		t.Fatalf("GetForBranch() error = %q is not wrapped (identical to root cause)", err.Error())
	}
}

func TestPullRequestService_GetForBranch_ContextCancelled_PropagatesError(t *testing.T) {
	port := mocks.NewMockPullRequestPort(t)
	port.EXPECT().
		GetForBranch(mock.Anything, "/repo", "main").
		Return(domain.PullRequest{}, context.Canceled).Once()

	svc := NewPullRequestService(port, testLogger())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.GetForBranch(ctx, GetPullRequestForBranchRequest{RepoPath: "/repo", Branch: "main"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("GetForBranch() error = %v, want context.Canceled chain", err)
	}
}

func TestPullRequestService_ListRequestedReviews_PassesTheInboxThrough(t *testing.T) {
	port := mocks.NewMockPullRequestPort(t)
	want := domain.ReviewInbox{Viewer: "me", PullRequests: []domain.PullRequest{{Number: 1, Repo: "o/r"}}}
	port.EXPECT().ListRequestedReviews(mock.Anything).Return(want, nil).Once()

	svc := NewPullRequestService(port, testLogger())
	resp, err := svc.ListRequestedReviews(context.Background(), ListRequestedReviewsRequest{})

	if err != nil {
		t.Fatalf("ListRequestedReviews() error = %v", err)
	}
	if resp.Inbox.Viewer != "me" || len(resp.Inbox.PullRequests) != 1 {
		t.Fatalf("Inbox = %+v", resp.Inbox)
	}
}

func TestPullRequestService_ListRequestedReviews_WrapsPortErrors(t *testing.T) {
	port := mocks.NewMockPullRequestPort(t)
	port.EXPECT().ListRequestedReviews(mock.Anything).Return(domain.ReviewInbox{}, errors.New("gh down")).Once()

	svc := NewPullRequestService(port, testLogger())
	_, err := svc.ListRequestedReviews(context.Background(), ListRequestedReviewsRequest{})

	if err == nil || !strings.Contains(err.Error(), "gh down") {
		t.Fatalf("ListRequestedReviews() error = %v, want wrapped port error", err)
	}
}
