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
	// CapturePane returns the current visible content of the named tmux
	// session's active pane, with ANSI escape sequences preserved so callers
	// can render colored output. Returns ErrTmuxSessionNotFound if the session
	// does not exist.
	CapturePane(ctx context.Context, tmuxID string) (string, error)
	// ResizeWindow resizes the named tmux session's window to the given
	// dimensions in cells. The running terminal application receives SIGWINCH
	// and redraws on the new canvas. Returns ErrTmuxSessionNotFound if the
	// session does not exist.
	ResizeWindow(ctx context.Context, tmuxID string, width, height int) error
	// SendKeys sends the named key(s) to the given tmux session's active pane
	// without attaching to it. key is a tmux key name such as "Enter".
	// Returns ErrTmuxSessionNotFound if the session does not exist.
	SendKeys(ctx context.Context, tmuxID string, key string) error
	// SendText sends a literal text string to the named session's active pane
	// using tmux send-keys -l (literal mode), which bypasses key-name
	// interpretation so arbitrary text — including characters like < > / " —
	// is delivered verbatim. The caller is responsible for sending a
	// subsequent SendKeys("Enter") if the text should be submitted.
	// Returns ErrTmuxSessionNotFound if the session does not exist.
	SendText(ctx context.Context, tmuxID string, text string) error
	// EnsureExtendedKeys sets the tmux server option `extended-keys on` so that
	// modifier key sequences (e.g. Shift+Enter) are preserved and forwarded to
	// inner applications instead of being collapsed to their unmodified form.
	// The option is server-scoped and only needs to be set once per tmux server
	// process. It is a no-op on tmux servers that do not support the option.
	EnsureExtendedKeys(ctx context.Context) error
}

var ErrTmuxSessionNotFound = errors.New("tmux session not found")
