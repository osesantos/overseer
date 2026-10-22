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
	port       domain.PullRequestPort
	dismissals domain.PullRequestDismissalRepository
	logger     *slog.Logger
}

func NewPullRequestService(port domain.PullRequestPort, dismissals domain.PullRequestDismissalRepository, logger *slog.Logger) *PullRequestService {
	return &PullRequestService{port: port, dismissals: dismissals, logger: logger}
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

// ListRequestedReviews is the inbox minus the PRs the operator dismissed, so
// neither the page nor auto-review ever sees a dismissed PR.
func (s *PullRequestService) ListRequestedReviews(ctx context.Context, _ ListRequestedReviewsRequest) (ListRequestedReviewsResponse, error) {
	inbox, err := s.port.ListRequestedReviews(ctx)
	if err != nil {
		return ListRequestedReviewsResponse{}, fmt.Errorf("fetch requested reviews: %w", err)
	}
	dismissed, err := s.dismissals.List(ctx)
	if err != nil {
		return ListRequestedReviewsResponse{}, fmt.Errorf("list dismissed pull requests: %w", err)
	}
	hidden := make(map[domain.PullRequestRef]bool, len(dismissed))
	for _, ref := range dismissed {
		hidden[ref] = true
	}
	inbox.PullRequests = slices.DeleteFunc(inbox.PullRequests, func(pr domain.PullRequest) bool { return hidden[pr.Ref()] })
	s.logger.InfoContext(ctx, "review inbox fetched", "viewer", inbox.Viewer, "count", len(inbox.PullRequests), "dismissed", len(dismissed))
	return ListRequestedReviewsResponse{Inbox: inbox}, nil
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
	if err := s.dismissals.Add(ctx, req.Ref); err != nil {
		return DismissPullRequestResponse{}, fmt.Errorf("dismiss pull request %s#%d: %w", req.Ref.Repo, req.Ref.Number, err)
	}
	s.logger.InfoContext(ctx, "pull request dismissed", "repo", req.Ref.Repo, "pr", req.Ref.Number)
	return DismissPullRequestResponse{}, nil
}
