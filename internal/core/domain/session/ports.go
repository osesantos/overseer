package session

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Save(ctx context.Context, s Session) error
	Get(ctx context.Context, id uuid.UUID) (Session, error)
	List(ctx context.Context) ([]Session, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type TmuxAdapter interface {
	CreateSession(ctx context.Context, name string) (tmuxID string, err error)
	KillSession(ctx context.Context, tmuxID string) error
}

type GitAdapter interface {
	CreateWorktree(ctx context.Context, baseBranch, path string) error
	RemoveWorktree(ctx context.Context, path string) error
}

type AgentLauncher interface {
	Launch(ctx context.Context, harness, workdir string) (pid int, err error)
}
