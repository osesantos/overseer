package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"

	"github.com/dnlopes/overseer/internal/core/domain"
)

type ProjectService struct {
	repo   domain.ProjectRepository
	git    domain.GitAdapter
	logger *slog.Logger
}

func NewProjectService(repo domain.ProjectRepository, git domain.GitAdapter, logger *slog.Logger) *ProjectService {
	return &ProjectService{repo: repo, git: git, logger: logger}
}

// --- Register ---

type RegisterProjectRequest struct {
	Path string
	Name string
}

type RegisterProjectResponse struct {
	Project domain.Project
}

func (s *ProjectService) Register(ctx context.Context, req RegisterProjectRequest) (RegisterProjectResponse, error) {
	project, err := domain.NewProject(req.Path, req.Name)
	if err != nil {
		return RegisterProjectResponse{}, err
	}

	if _, err := s.repo.GetByPath(ctx, project.Path); err == nil {
		return RegisterProjectResponse{}, domain.ErrProjectAlreadyExists
	} else if !errors.Is(err, domain.ErrProjectNotFound) {
		return RegisterProjectResponse{}, fmt.Errorf("lookup project by path: %w", err)
	}

	if err := s.git.IsGitRepo(ctx, project.Path); err != nil {
		if errors.Is(err, domain.ErrProjectNotGitRepo) {
			return RegisterProjectResponse{}, err
		}
		return RegisterProjectResponse{}, fmt.Errorf("verify git repo: %w", err)
	}

	if err := s.repo.Save(ctx, project); err != nil {
		return RegisterProjectResponse{}, fmt.Errorf("save project: %w", err)
	}

	s.logger.InfoContext(ctx, "project registered",
		slog.String("id", project.ID.String()),
		slog.String("path", project.Path),
		slog.String("name", project.Name),
	)
	return RegisterProjectResponse{Project: project}, nil
}

// --- List ---

type ListProjectsRequest struct{}

type ListProjectsResponse struct {
	Projects []domain.Project
}

func (s *ProjectService) List(ctx context.Context, _ ListProjectsRequest) (ListProjectsResponse, error) {
	projects, err := s.repo.List(ctx)
	if err != nil {
		return ListProjectsResponse{}, err
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name < projects[j].Name
	})

	return ListProjectsResponse{Projects: projects}, nil
}
