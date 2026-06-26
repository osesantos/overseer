package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	"github.com/google/uuid"

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

// --- Rename ---

type RenameProjectRequest struct {
	ID      uuid.UUID
	NewName string
}

type RenameProjectResponse struct {
	Project domain.Project
}

func (s *ProjectService) Rename(ctx context.Context, req RenameProjectRequest) (RenameProjectResponse, error) {
	project, err := s.repo.Get(ctx, req.ID)
	if err != nil {
		return RenameProjectResponse{}, err
	}

	if err := project.Rename(req.NewName); err != nil {
		return RenameProjectResponse{}, err
	}

	if err := s.repo.Save(ctx, project); err != nil {
		return RenameProjectResponse{}, fmt.Errorf("save project: %w", err)
	}

	s.logger.InfoContext(ctx, "project renamed",
		slog.String("id", project.ID.String()),
		slog.String("name", project.Name),
	)
	return RenameProjectResponse{Project: project}, nil
}

// --- Discover ---

type DiscoverProjectsRequest struct {
	// Paths is the list of root directories to scan. Each directory is
	// inspected one level deep — only immediate subdirectories are checked.
	// Paths must already be expanded (no ~ prefixes).
	Paths []string
}

type DiscoverProjectsResponse struct {
	// Registered is the count of repositories newly registered during this
	// scan. Already-known paths are skipped silently.
	Registered int
	// MissingPaths contains every entry from Paths that does not exist on
	// disk so callers can surface a warning to the user.
	MissingPaths []string
}

// Discover scans each root directory in req.Paths one level deep, checks
// every immediate subdirectory for a git repository, and registers any that
// are not yet known to Overseer. It is intentionally permissive: individual
// subdirectory errors (non-git dirs, permission denied) are logged and
// skipped so a single bad entry never aborts the whole scan.
func (s *ProjectService) Discover(ctx context.Context, req DiscoverProjectsRequest) (DiscoverProjectsResponse, error) {
	var resp DiscoverProjectsResponse
	for _, root := range req.Paths {
		entries, err := os.ReadDir(root)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				resp.MissingPaths = append(resp.MissingPaths, root)
				continue
			}
			s.logger.WarnContext(ctx, "project discovery: read dir failed", "path", root, "error", err)
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			fullPath := filepath.Join(root, entry.Name())
			if _, err := s.Register(ctx, RegisterProjectRequest{Path: fullPath}); err != nil {
				switch {
				case errors.Is(err, domain.ErrProjectAlreadyExists),
					errors.Is(err, domain.ErrProjectNotGitRepo):
					// expected — skip silently
				default:
					s.logger.WarnContext(ctx, "project discovery: register failed", "path", fullPath, "error", err)
				}
				continue
			}
			resp.Registered++
		}
	}
	s.logger.InfoContext(ctx, "project discovery complete",
		slog.Int("registered", resp.Registered),
		slog.Int("missing_paths", len(resp.MissingPaths)),
	)
	return resp, nil
}
