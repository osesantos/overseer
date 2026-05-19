package service

import (
	"context"
	"errors"
	"testing"

	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/testutil"
	"github.com/dnlopes/overseer/internal/testutil/mocks"
)

// --- Register ---

func TestProjectService_Register_HappyPath(t *testing.T) {
	repo := &mocks.MockProjectRepository{GetByPathErr: domain.ErrProjectNotFound}
	git := &mocks.MockGitAdapter{}
	svc := NewProjectService(repo, git, testLogger())

	resp, err := svc.Register(context.Background(), RegisterProjectRequest{Path: "/repo/overseer", Name: "Overseer"})

	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if resp.Project.Name != "Overseer" {
		t.Fatalf("Register() Project.Name = %q, want %q", resp.Project.Name, "Overseer")
	}
	if resp.Project.Path != "/repo/overseer" {
		t.Fatalf("Register() Project.Path = %q, want %q", resp.Project.Path, "/repo/overseer")
	}
	if git.IsGitRepoCalls != 1 {
		t.Fatalf("Git.IsGitRepo calls = %d, want 1", git.IsGitRepoCalls)
	}
	if git.IsGitRepoLastArg != "/repo/overseer" {
		t.Fatalf("Git.IsGitRepo arg = %q, want %q", git.IsGitRepoLastArg, "/repo/overseer")
	}
	if repo.SaveCalls != 1 {
		t.Fatalf("ProjectRepository.Save calls = %d, want 1", repo.SaveCalls)
	}
}

func TestProjectService_Register_DerivesNameFromPath(t *testing.T) {
	repo := &mocks.MockProjectRepository{GetByPathErr: domain.ErrProjectNotFound}
	git := &mocks.MockGitAdapter{}
	svc := NewProjectService(repo, git, testLogger())

	resp, err := svc.Register(context.Background(), RegisterProjectRequest{Path: "/repo/widgets", Name: ""})

	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if resp.Project.Name != "widgets" {
		t.Fatalf("Register() Project.Name = %q, want %q (derived)", resp.Project.Name, "widgets")
	}
}

func TestProjectService_Register_RejectsEmptyPath(t *testing.T) {
	repo := &mocks.MockProjectRepository{}
	git := &mocks.MockGitAdapter{}
	svc := NewProjectService(repo, git, testLogger())

	_, err := svc.Register(context.Background(), RegisterProjectRequest{Path: "", Name: ""})

	if !errors.Is(err, domain.ErrProjectEmptyPath) {
		t.Fatalf("Register() error = %v, want %v", err, domain.ErrProjectEmptyPath)
	}
	if git.IsGitRepoCalls != 0 {
		t.Fatalf("Git.IsGitRepo calls = %d, want 0 on validation failure", git.IsGitRepoCalls)
	}
	if repo.SaveCalls != 0 {
		t.Fatalf("ProjectRepository.Save calls = %d, want 0 on validation failure", repo.SaveCalls)
	}
}

func TestProjectService_Register_RejectsRelativePath(t *testing.T) {
	repo := &mocks.MockProjectRepository{}
	git := &mocks.MockGitAdapter{}
	svc := NewProjectService(repo, git, testLogger())

	_, err := svc.Register(context.Background(), RegisterProjectRequest{Path: "repos/x", Name: ""})

	if !errors.Is(err, domain.ErrProjectPathNotAbsolute) {
		t.Fatalf("Register() error = %v, want %v", err, domain.ErrProjectPathNotAbsolute)
	}
}

func TestProjectService_Register_RejectsNonGitRepo(t *testing.T) {
	repo := &mocks.MockProjectRepository{GetByPathErr: domain.ErrProjectNotFound}
	git := &mocks.MockGitAdapter{IsGitRepoErr: domain.ErrProjectNotGitRepo}
	svc := NewProjectService(repo, git, testLogger())

	_, err := svc.Register(context.Background(), RegisterProjectRequest{Path: "/not/a/repo", Name: ""})

	if !errors.Is(err, domain.ErrProjectNotGitRepo) {
		t.Fatalf("Register() error = %v, want %v", err, domain.ErrProjectNotGitRepo)
	}
	if repo.SaveCalls != 0 {
		t.Fatalf("ProjectRepository.Save calls = %d, want 0 when not a git repo", repo.SaveCalls)
	}
}

func TestProjectService_Register_RejectsDuplicatePath(t *testing.T) {
	existing := testutil.MakeProject("/repo/overseer", "OldName")
	repo := &mocks.MockProjectRepository{GetByPathResult: existing, GetByPathErr: nil}
	git := &mocks.MockGitAdapter{}
	svc := NewProjectService(repo, git, testLogger())

	_, err := svc.Register(context.Background(), RegisterProjectRequest{Path: "/repo/overseer", Name: "NewName"})

	if !errors.Is(err, domain.ErrProjectAlreadyExists) {
		t.Fatalf("Register() error = %v, want %v", err, domain.ErrProjectAlreadyExists)
	}
	if repo.SaveCalls != 0 {
		t.Fatalf("ProjectRepository.Save calls = %d, want 0 on duplicate", repo.SaveCalls)
	}
}

func TestProjectService_Register_GetByPathInfrastructureError(t *testing.T) {
	infraErr := errors.New("disk failed")
	repo := &mocks.MockProjectRepository{GetByPathErr: infraErr}
	git := &mocks.MockGitAdapter{}
	svc := NewProjectService(repo, git, testLogger())

	_, err := svc.Register(context.Background(), RegisterProjectRequest{Path: "/repo/overseer", Name: ""})

	if !errors.Is(err, infraErr) {
		t.Fatalf("Register() error = %v, want wrapped %v", err, infraErr)
	}
}

// --- List ---

func TestProjectService_List_Empty(t *testing.T) {
	repo := &mocks.MockProjectRepository{}
	svc := NewProjectService(repo, &mocks.MockGitAdapter{}, testLogger())

	resp, err := svc.List(context.Background(), ListProjectsRequest{})

	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(resp.Projects) != 0 {
		t.Fatalf("List() len = %d, want 0", len(resp.Projects))
	}
	if repo.ListCalls != 1 {
		t.Fatalf("ProjectRepository.List calls = %d, want 1", repo.ListCalls)
	}
}

func TestProjectService_List_SortsByName(t *testing.T) {
	a := testutil.MakeProject("/r/zeta", "Zeta")
	b := testutil.MakeProject("/r/alpha", "Alpha")
	c := testutil.MakeProject("/r/mike", "Mike")
	repo := &mocks.MockProjectRepository{ListResult: []domain.Project{a, b, c}}
	svc := NewProjectService(repo, &mocks.MockGitAdapter{}, testLogger())

	resp, err := svc.List(context.Background(), ListProjectsRequest{})

	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(resp.Projects) != 3 {
		t.Fatalf("List() len = %d, want 3", len(resp.Projects))
	}
	wantOrder := []string{"Alpha", "Mike", "Zeta"}
	for i, want := range wantOrder {
		if resp.Projects[i].Name != want {
			t.Fatalf("Projects[%d].Name = %q, want %q", i, resp.Projects[i].Name, want)
		}
	}
}

