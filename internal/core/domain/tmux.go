package domain

import (
	"context"
	"errors"
	"time"
)

type TmuxSession struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TmuxAdapter interface {
	GetSession(ctx context.Context, tmuxID string) (TmuxSession, error)
	ListSessions(ctx context.Context) ([]TmuxSession, error)
	CreateSession(ctx context.Context, name, startDir, shellCommand string) (tmuxID string, err error)
	KillSession(ctx context.Context, tmuxID string) error
	AttachSession(ctx context.Context, tmuxID string) error
}

var ErrTmuxSessionNotFound = errors.New("tmux session not found")
