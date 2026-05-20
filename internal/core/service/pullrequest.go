package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/dnlopes/overseer/internal/core/domain"
)

type PullRequestService struct {
	port   domain.PullRequestPort
	logger *slog.Logger
}

func NewPullRequestService(port domain.PullRequestPort, logger *slog.Logger) *PullRequestService {
	return &PullRequestService{port: port, logger: logger}
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
