package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/dnlopes/overseer/internal/core/domain"
)

type PullRequestService struct {
	port     domain.PullRequestPort
	settings domain.ReviewSettingsRepository
	logger   *slog.Logger
}

func NewPullRequestService(port domain.PullRequestPort, settings domain.ReviewSettingsRepository, logger *slog.Logger) *PullRequestService {
	return &PullRequestService{port: port, settings: settings, logger: logger}
}

type GetPullRequestForBranchRequest struct {
	RepoPath string
	Branch   string
}

type GetPullRequestForBranchResponse struct {
	PullRequest domain.PullRequest
}

func (s *PullRequestService) GetForBranch(ctx context.Context, req GetPullRequestForBranchRequest) (GetPullRequestForBranchResponse, error) {
	pr, err := s.port.GetForBranch(ctx, req.RepoPath, req.Branch)
	if err != nil {
		if errors.Is(err, domain.ErrPullRequestNotFound) {
			return GetPullRequestForBranchResponse{}, err
		}
		return GetPullRequestForBranchResponse{}, fmt.Errorf("fetch pull request for branch %s: %w", req.Branch, err)
	}
	return GetPullRequestForBranchResponse{PullRequest: pr}, nil
}

type ListRequestedReviewsRequest struct{}

type ListRequestedReviewsResponse struct {
	Inbox domain.ReviewInbox
}

// ListRequestedReviews is the inbox with the PRs the operator dismissed marked
// as such, and the PRs whose review session was closed by hand marked as
// reviewed, so auto-review never acts on them again.
func (s *PullRequestService) ListRequestedReviews(ctx context.Context, _ ListRequestedReviewsRequest) (ListRequestedReviewsResponse, error) {
	inbox, err := s.port.ListRequestedReviews(ctx)
	if err != nil {
		return ListRequestedReviewsResponse{}, fmt.Errorf("fetch requested reviews: %w", err)
	}
	settings, err := s.settings.Load(ctx)
	if err != nil {
		return ListRequestedReviewsResponse{}, fmt.Errorf("load review settings: %w", err)
	}
	for i := range inbox.PullRequests {
		ref := inbox.PullRequests[i].Ref()
		inbox.PullRequests[i].Dismissed = slices.Contains(settings.Dismissed, ref)
		if slices.Contains(settings.Handled, ref) {
			inbox.PullRequests[i].ReviewedByViewer = true
		}
	}
	s.logger.InfoContext(ctx, "review inbox fetched", "viewer", inbox.Viewer, "count", len(inbox.PullRequests),
		"dismissed", len(settings.Dismissed), "handled", len(settings.Handled))
	return ListRequestedReviewsResponse{Inbox: inbox}, nil
}

type ResolveAutoReviewRequest struct {
	// Default is the config value, used until the operator toggles it.
	Default bool
}

type ResolveAutoReviewResponse struct {
	On bool
}

// ResolveAutoReview returns the persisted toggle when the operator set one,
// otherwise the config default.
func (s *PullRequestService) ResolveAutoReview(ctx context.Context, req ResolveAutoReviewRequest) (ResolveAutoReviewResponse, error) {
	settings, err := s.settings.Load(ctx)
	if err != nil {
		return ResolveAutoReviewResponse{}, fmt.Errorf("load review settings: %w", err)
	}
	if settings.AutoReview != nil {
		return ResolveAutoReviewResponse{On: *settings.AutoReview}, nil
	}
	return ResolveAutoReviewResponse{On: req.Default}, nil
}

type SetAutoReviewRequest struct {
	On bool
}

type SetAutoReviewResponse struct{}

// SetAutoReview persists the toggle so it survives restarts.
func (s *PullRequestService) SetAutoReview(ctx context.Context, req SetAutoReviewRequest) (SetAutoReviewResponse, error) {
	if err := s.settings.SetAutoReview(ctx, req.On); err != nil {
		return SetAutoReviewResponse{}, fmt.Errorf("persist auto-review toggle: %w", err)
	}
	s.logger.InfoContext(ctx, "auto-review toggled", "on", req.On)
	return SetAutoReviewResponse{}, nil
}

type DismissPullRequestRequest struct {
	Ref domain.PullRequestRef
}

type DismissPullRequestResponse struct{}

// Dismiss hides a PR from the inbox for good.
func (s *PullRequestService) Dismiss(ctx context.Context, req DismissPullRequestRequest) (DismissPullRequestResponse, error) {
	if req.Ref.Repo == "" || req.Ref.Number <= 0 {
		return DismissPullRequestResponse{}, domain.ErrPullRequestInvalidNumber
	}
	if err := s.settings.Dismiss(ctx, req.Ref); err != nil {
		return DismissPullRequestResponse{}, fmt.Errorf("dismiss pull request %s#%d: %w", req.Ref.Repo, req.Ref.Number, err)
	}
	s.logger.InfoContext(ctx, "pull request dismissed", "repo", req.Ref.Repo, "pr", req.Ref.Number)
	return DismissPullRequestResponse{}, nil
}

type OpenPullRequestInBrowserRequest struct {
	URL string
}

type OpenPullRequestInBrowserResponse struct{}

func (s *PullRequestService) OpenInBrowser(ctx context.Context, req OpenPullRequestInBrowserRequest) (OpenPullRequestInBrowserResponse, error) {
	if err := s.port.OpenInBrowser(ctx, req.URL); err != nil {
		return OpenPullRequestInBrowserResponse{}, fmt.Errorf("open pull request in browser: %w", err)
	}
	return OpenPullRequestInBrowserResponse{}, nil
}
