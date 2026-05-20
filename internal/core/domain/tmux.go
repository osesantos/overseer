package domain

import (
	"context"
	"errors"
	"os/exec"
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
	// AttachCommand returns a runnable *exec.Cmd that attaches the caller's
	// terminal to the named tmux session when executed. It does NOT take over
	// the terminal itself — the caller is responsible for running it (typically
	// via tea.ExecProcess from inside a Bubble Tea alt-screen TUI).
	AttachCommand(ctx context.Context, tmuxID string) (*exec.Cmd, error)
}

var ErrTmuxSessionNotFound = errors.New("tmux session not found")
