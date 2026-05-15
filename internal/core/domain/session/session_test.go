package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewCreatesSession(t *testing.T) {
	before := time.Now()

	s, err := New("alpha", "overseer")

	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if s.ID == uuid.Nil {
		t.Fatal("New() ID is nil")
	}
	if s.Name != "alpha" {
		t.Fatalf("New() Name = %q, want %q", s.Name, "alpha")
	}
	if s.ProjectName != "overseer" {
		t.Fatalf("New() ProjectName = %q, want %q", s.ProjectName, "overseer")
	}
	if s.Order != 0 {
		t.Fatalf("New() Order = %d, want 0", s.Order)
	}
	if s.CreatedAt.Before(before) {
		t.Fatalf("New() CreatedAt = %v, before creation start %v", s.CreatedAt, before)
	}
	if s.UpdatedAt.Before(before) {
		t.Fatalf("New() UpdatedAt = %v, before creation start %v", s.UpdatedAt, before)
	}
	if !s.CreatedAt.Equal(s.UpdatedAt) {
		t.Fatalf("New() CreatedAt = %v, UpdatedAt = %v, want equal", s.CreatedAt, s.UpdatedAt)
	}
}

func TestNewTrimsNameAndProject(t *testing.T) {
	s, err := New("  alpha  ", "  overseer  ")

	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if s.Name != "alpha" {
		t.Fatalf("New() Name = %q, want %q", s.Name, "alpha")
	}
	if s.ProjectName != "overseer" {
		t.Fatalf("New() ProjectName = %q, want %q", s.ProjectName, "overseer")
	}
}

func TestNewValidation(t *testing.T) {
	long := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tests := []struct {
		name    string
		session string
		project string
		wantErr error
	}{
		{name: "empty name", session: "", project: "overseer", wantErr: ErrEmptyName},
		{name: "blank name", session: "   ", project: "overseer", wantErr: ErrEmptyName},
		{name: "name too long", session: long, project: "overseer", wantErr: ErrNameTooLong},
		{name: "empty project", session: "alpha", project: "", wantErr: ErrEmptyProject},
		{name: "blank project", session: "alpha", project: "   ", wantErr: ErrEmptyProject},
		{name: "project too long", session: "alpha", project: long, wantErr: ErrProjectTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.session, tt.project)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("New() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewAcceptsOneHundredCharacterFields(t *testing.T) {
	exactly100 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	s, err := New(exactly100, exactly100)

	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if s.Name != exactly100 {
		t.Fatalf("New() Name length = %d, want 100", len(s.Name))
	}
	if s.ProjectName != exactly100 {
		t.Fatalf("New() ProjectName length = %d, want 100", len(s.ProjectName))
	}
}

func TestRenameUpdatesNameAndUpdatedAt(t *testing.T) {
	s, err := New("alpha", "overseer")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	originalCreatedAt := s.CreatedAt
	originalUpdatedAt := s.UpdatedAt
	time.Sleep(time.Nanosecond)

	err = s.Rename("beta")

	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if s.Name != "beta" {
		t.Fatalf("Rename() Name = %q, want %q", s.Name, "beta")
	}
	if !s.CreatedAt.Equal(originalCreatedAt) {
		t.Fatalf("Rename() CreatedAt = %v, want unchanged %v", s.CreatedAt, originalCreatedAt)
	}
	if !s.UpdatedAt.After(originalUpdatedAt) {
		t.Fatalf("Rename() UpdatedAt = %v, want after %v", s.UpdatedAt, originalUpdatedAt)
	}
}

func TestRenameTrimsName(t *testing.T) {
	s, err := New("alpha", "overseer")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = s.Rename("  beta  ")

	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if s.Name != "beta" {
		t.Fatalf("Rename() Name = %q, want %q", s.Name, "beta")
	}
}

func TestRenameValidation(t *testing.T) {
	long := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tests := []struct {
		name    string
		newName string
		wantErr error
	}{
		{name: "empty name", newName: "", wantErr: ErrEmptyName},
		{name: "blank name", newName: "   ", wantErr: ErrEmptyName},
		{name: "name too long", newName: long, wantErr: ErrNameTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := New("alpha", "overseer")
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			originalName := s.Name
			originalUpdatedAt := s.UpdatedAt

			err = s.Rename(tt.newName)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Rename() error = %v, want %v", err, tt.wantErr)
			}
			if s.Name != originalName {
				t.Fatalf("Rename() changed Name to %q, want unchanged %q", s.Name, originalName)
			}
			if !s.UpdatedAt.Equal(originalUpdatedAt) {
				t.Fatalf("Rename() changed UpdatedAt to %v, want unchanged %v", s.UpdatedAt, originalUpdatedAt)
			}
		})
	}
}

func TestString(t *testing.T) {
	s, err := New("alpha", "overseer")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	got := s.String()

	if got != "[overseer] alpha" {
		t.Fatalf("String() = %q, want %q", got, "[overseer] alpha")
	}
	if got == s.ID.String() || got == "[overseer] alpha "+s.ID.String() {
		t.Fatal("String() includes UUID")
	}
}

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "empty name", err: ErrEmptyName, want: "session name cannot be empty"},
		{name: "name too long", err: ErrNameTooLong, want: "session name exceeds 100 characters"},
		{name: "empty project", err: ErrEmptyProject, want: "project name cannot be empty"},
		{name: "project too long", err: ErrProjectTooLong, want: "project name exceeds 100 characters"},
		{name: "not found", err: ErrNotFound, want: "session not found"},
		{name: "already exists", err: ErrAlreadyExists, want: "session already exists"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatal("error is nil")
			}
			if tt.err.Error() != tt.want {
				t.Fatalf("error message = %q, want %q", tt.err.Error(), tt.want)
			}
		})
	}
}

func TestPortInterfaces(t *testing.T) {
	ctx := context.Background()
	id := uuid.New()
	s, err := New("alpha", "overseer")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	var repo Repository = fakeRepository{}
	if err := repo.Save(ctx, s); err != nil {
		t.Fatalf("Repository.Save() error = %v", err)
	}
	if _, err := repo.Get(ctx, id); err != nil {
		t.Fatalf("Repository.Get() error = %v", err)
	}
	if _, err := repo.List(ctx); err != nil {
		t.Fatalf("Repository.List() error = %v", err)
	}
	if err := repo.Delete(ctx, id); err != nil {
		t.Fatalf("Repository.Delete() error = %v", err)
	}

	var tmux TmuxAdapter = fakeTmuxAdapter{}
	if _, err := tmux.CreateSession(ctx, "alpha"); err != nil {
		t.Fatalf("TmuxAdapter.CreateSession() error = %v", err)
	}
	if err := tmux.KillSession(ctx, "tmux-alpha"); err != nil {
		t.Fatalf("TmuxAdapter.KillSession() error = %v", err)
	}

	var git GitAdapter = fakeGitAdapter{}
	if err := git.CreateWorktree(ctx, "main", "/tmp/alpha"); err != nil {
		t.Fatalf("GitAdapter.CreateWorktree() error = %v", err)
	}
	if err := git.RemoveWorktree(ctx, "/tmp/alpha"); err != nil {
		t.Fatalf("GitAdapter.RemoveWorktree() error = %v", err)
	}

	var launcher AgentLauncher = fakeAgentLauncher{}
	if _, err := launcher.Launch(ctx, "claude", "/tmp/alpha"); err != nil {
		t.Fatalf("AgentLauncher.Launch() error = %v", err)
	}
}

type fakeRepository struct{}

func (fakeRepository) Save(context.Context, Session) error { return nil }
func (fakeRepository) Get(context.Context, uuid.UUID) (Session, error) {
	return New("alpha", "overseer")
}
func (fakeRepository) List(context.Context) ([]Session, error) { return []Session{}, nil }
func (fakeRepository) Delete(context.Context, uuid.UUID) error { return nil }

type fakeTmuxAdapter struct{}

func (fakeTmuxAdapter) CreateSession(context.Context, string) (string, error) {
	return "tmux-alpha", nil
}
func (fakeTmuxAdapter) KillSession(context.Context, string) error { return nil }

type fakeGitAdapter struct{}

func (fakeGitAdapter) CreateWorktree(context.Context, string, string) error { return nil }
func (fakeGitAdapter) RemoveWorktree(context.Context, string) error         { return nil }

type fakeAgentLauncher struct{}

func (fakeAgentLauncher) Launch(context.Context, string, string) (int, error) { return 1, nil }
