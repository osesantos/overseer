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

	svc := NewPullRequestService(port, noDismissals(t), testLogger())
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

	svc := NewPullRequestService(port, noDismissals(t), testLogger())
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

	svc := NewPullRequestService(port, noDismissals(t), testLogger())
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

	svc := NewPullRequestService(port, noDismissals(t), testLogger())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.GetForBranch(ctx, GetPullRequestForBranchRequest{RepoPath: "/repo", Branch: "main"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("GetForBranch() error = %v, want context.Canceled chain", err)
	}
}

// noDismissals is a dismissal repo that never hides anything, for tests whose
// subject is something else.
func noDismissals(t *testing.T) *mocks.MockPullRequestDismissalRepository {
	t.Helper()
	d := mocks.NewMockPullRequestDismissalRepository(t)
	d.EXPECT().List(mock.Anything).Return(nil, nil).Maybe()
	return d
}

func TestPullRequestService_ListRequestedReviews_DropsDismissedPRs(t *testing.T) {
	port := mocks.NewMockPullRequestPort(t)
	port.EXPECT().ListRequestedReviews(mock.Anything).Return(domain.ReviewInbox{Viewer: "me", PullRequests: []domain.PullRequest{
		{Number: 1, Repo: "o/r"}, {Number: 2, Repo: "o/r"},
	}}, nil).Once()
	dismissals := mocks.NewMockPullRequestDismissalRepository(t)
	dismissals.EXPECT().List(mock.Anything).Return([]domain.PullRequestRef{{Repo: "o/r", Number: 1}}, nil).Once()

	svc := NewPullRequestService(port, dismissals, testLogger())
	resp, err := svc.ListRequestedReviews(context.Background(), ListRequestedReviewsRequest{})

	if err != nil {
		t.Fatalf("ListRequestedReviews() error = %v", err)
	}
	if len(resp.Inbox.PullRequests) != 1 || resp.Inbox.PullRequests[0].Number != 2 {
		t.Fatalf("PullRequests = %+v, want only #2", resp.Inbox.PullRequests)
	}
}

func TestPullRequestService_Dismiss_PersistsTheRef(t *testing.T) {
	dismissals := mocks.NewMockPullRequestDismissalRepository(t)
	ref := domain.PullRequestRef{Repo: "o/r", Number: 7}
	dismissals.EXPECT().Add(mock.Anything, ref).Return(nil).Once()

	svc := NewPullRequestService(mocks.NewMockPullRequestPort(t), dismissals, testLogger())
	if _, err := svc.Dismiss(context.Background(), DismissPullRequestRequest{Ref: ref}); err != nil {
		t.Fatalf("Dismiss() error = %v", err)
	}
}

func TestPullRequestService_Dismiss_RejectsAnEmptyRef(t *testing.T) {
	svc := NewPullRequestService(mocks.NewMockPullRequestPort(t), mocks.NewMockPullRequestDismissalRepository(t), testLogger())
	if _, err := svc.Dismiss(context.Background(), DismissPullRequestRequest{}); !errors.Is(err, domain.ErrPullRequestInvalidNumber) {
		t.Fatalf("Dismiss() error = %v, want ErrPullRequestInvalidNumber", err)
	}
}

func TestPullRequestService_ListRequestedReviews_PassesTheInboxThrough(t *testing.T) {
	port := mocks.NewMockPullRequestPort(t)
	want := domain.ReviewInbox{Viewer: "me", PullRequests: []domain.PullRequest{{Number: 1, Repo: "o/r"}}}
	port.EXPECT().ListRequestedReviews(mock.Anything).Return(want, nil).Once()

	svc := NewPullRequestService(port, noDismissals(t), testLogger())
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

	svc := NewPullRequestService(port, noDismissals(t), testLogger())
	_, err := svc.ListRequestedReviews(context.Background(), ListRequestedReviewsRequest{})

	if err == nil || !strings.Contains(err.Error(), "gh down") {
		t.Fatalf("ListRequestedReviews() error = %v, want wrapped port error", err)
	}
}
