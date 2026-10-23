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

	svc := NewPullRequestService(port, noSettings(t), testLogger())
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

	svc := NewPullRequestService(port, noSettings(t), testLogger())
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

	svc := NewPullRequestService(port, noSettings(t), testLogger())
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

	svc := NewPullRequestService(port, noSettings(t), testLogger())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.GetForBranch(ctx, GetPullRequestForBranchRequest{RepoPath: "/repo", Branch: "main"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("GetForBranch() error = %v, want context.Canceled chain", err)
	}
}

// noSettings is a settings repo with nothing persisted, for tests whose subject
// is something else.
func noSettings(t *testing.T) *mocks.MockReviewSettingsRepository {
	t.Helper()
	r := mocks.NewMockReviewSettingsRepository(t)
	r.EXPECT().Load(mock.Anything).Return(domain.ReviewSettings{}, nil).Maybe()
	return r
}

func TestPullRequestService_ListRequestedReviews_AppliesDismissedAndHandled(t *testing.T) {
	port := mocks.NewMockPullRequestPort(t)
	port.EXPECT().ListRequestedReviews(mock.Anything).Return(domain.ReviewInbox{Viewer: "me", PullRequests: []domain.PullRequest{
		{Number: 1, Repo: "o/r"}, {Number: 2, Repo: "o/r"}, {Number: 3, Repo: "o/r"},
	}}, nil).Once()
	settings := mocks.NewMockReviewSettingsRepository(t)
	settings.EXPECT().Load(mock.Anything).Return(domain.ReviewSettings{
		Dismissed: []domain.PullRequestRef{{Repo: "o/r", Number: 1}},
		Handled:   []domain.PullRequestRef{{Repo: "o/r", Number: 2}},
	}, nil).Once()

	svc := NewPullRequestService(port, settings, testLogger())
	resp, err := svc.ListRequestedReviews(context.Background(), ListRequestedReviewsRequest{})

	if err != nil {
		t.Fatalf("ListRequestedReviews() error = %v", err)
	}
	prs := resp.Inbox.PullRequests
	if len(prs) != 3 {
		t.Fatalf("PullRequests = %+v, want all three kept", prs)
	}
	if !prs[0].Dismissed || prs[1].Dismissed || prs[2].Dismissed {
		t.Fatalf("Dismissed: #1=%v #2=%v #3=%v, want only #1 marked", prs[0].Dismissed, prs[1].Dismissed, prs[2].Dismissed)
	}
	if prs[0].ReviewedByViewer || !prs[1].ReviewedByViewer || prs[2].ReviewedByViewer {
		t.Fatalf("ReviewedByViewer: #1=%v #2=%v #3=%v, want only the handled #2 marked", prs[0].ReviewedByViewer, prs[1].ReviewedByViewer, prs[2].ReviewedByViewer)
	}
}

func TestPullRequestService_ResolveAutoReview_PersistedToggleWinsOverConfig(t *testing.T) {
	off := false
	settings := mocks.NewMockReviewSettingsRepository(t)
	settings.EXPECT().Load(mock.Anything).Return(domain.ReviewSettings{AutoReview: &off}, nil).Once()

	svc := NewPullRequestService(mocks.NewMockPullRequestPort(t), settings, testLogger())
	resp, err := svc.ResolveAutoReview(context.Background(), ResolveAutoReviewRequest{Default: true})

	if err != nil || resp.On {
		t.Fatalf("ResolveAutoReview() = %v, %v; want off from the persisted toggle", resp.On, err)
	}
}

func TestPullRequestService_ResolveAutoReview_FallsBackToConfig(t *testing.T) {
	svc := NewPullRequestService(mocks.NewMockPullRequestPort(t), noSettings(t), testLogger())
	resp, err := svc.ResolveAutoReview(context.Background(), ResolveAutoReviewRequest{Default: true})
	if err != nil || !resp.On {
		t.Fatalf("ResolveAutoReview() = %v, %v; want the config default", resp.On, err)
	}
}

func TestPullRequestService_SetAutoReview_Persists(t *testing.T) {
	settings := mocks.NewMockReviewSettingsRepository(t)
	settings.EXPECT().SetAutoReview(mock.Anything, false).Return(nil).Once()

	svc := NewPullRequestService(mocks.NewMockPullRequestPort(t), settings, testLogger())
	if _, err := svc.SetAutoReview(context.Background(), SetAutoReviewRequest{On: false}); err != nil {
		t.Fatalf("SetAutoReview() error = %v", err)
	}
}

func TestPullRequestService_Dismiss_PersistsTheRef(t *testing.T) {
	settings := mocks.NewMockReviewSettingsRepository(t)
	ref := domain.PullRequestRef{Repo: "o/r", Number: 7}
	settings.EXPECT().Dismiss(mock.Anything, ref).Return(nil).Once()

	svc := NewPullRequestService(mocks.NewMockPullRequestPort(t), settings, testLogger())
	if _, err := svc.Dismiss(context.Background(), DismissPullRequestRequest{Ref: ref}); err != nil {
		t.Fatalf("Dismiss() error = %v", err)
	}
}

func TestPullRequestService_Dismiss_RejectsAnEmptyRef(t *testing.T) {
	svc := NewPullRequestService(mocks.NewMockPullRequestPort(t), mocks.NewMockReviewSettingsRepository(t), testLogger())
	if _, err := svc.Dismiss(context.Background(), DismissPullRequestRequest{}); !errors.Is(err, domain.ErrPullRequestInvalidNumber) {
		t.Fatalf("Dismiss() error = %v, want ErrPullRequestInvalidNumber", err)
	}
}

func TestPullRequestService_ListRequestedReviews_PassesTheInboxThrough(t *testing.T) {
	port := mocks.NewMockPullRequestPort(t)
	want := domain.ReviewInbox{Viewer: "me", PullRequests: []domain.PullRequest{{Number: 1, Repo: "o/r"}}}
	port.EXPECT().ListRequestedReviews(mock.Anything).Return(want, nil).Once()

	svc := NewPullRequestService(port, noSettings(t), testLogger())
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

	svc := NewPullRequestService(port, noSettings(t), testLogger())
	_, err := svc.ListRequestedReviews(context.Background(), ListRequestedReviewsRequest{})

	if err == nil || !strings.Contains(err.Error(), "gh down") {
		t.Fatalf("ListRequestedReviews() error = %v, want wrapped port error", err)
	}
}

func TestPullRequestService_OpenInBrowser_PassesTheURLThrough(t *testing.T) {
	port := mocks.NewMockPullRequestPort(t)
	port.EXPECT().OpenInBrowser(mock.Anything, "https://github.com/o/r/pull/7").Return(nil).Once()

	svc := NewPullRequestService(port, mocks.NewMockReviewSettingsRepository(t), testLogger())
	if _, err := svc.OpenInBrowser(context.Background(), OpenPullRequestInBrowserRequest{URL: "https://github.com/o/r/pull/7"}); err != nil {
		t.Fatalf("OpenInBrowser() error = %v", err)
	}
}
