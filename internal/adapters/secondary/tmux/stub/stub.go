// Package stub provides a stub implementation of the tmux adapter.
// This is a stub adapter. It satisfies the port interface with canned responses.
// Replace with real implementation when integrating real tmux.
package stub

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/core/domain/session"
)

// Compile-time interface check.
var _ session.TmuxAdapter = (*Stub)(nil)

// Stub is a stub implementation of session.TmuxAdapter.
// It satisfies the port interface with canned responses and records call counts for testing.
type Stub struct {
	CreateSessionCalls int
	KillSessionCalls   int
}

// CreateSession returns a deterministic canned tmux session ID of the form "tmux-stub-<name>-<uuid8>".
func (s *Stub) CreateSession(_ context.Context, name string) (string, error) {
	s.CreateSessionCalls++
	id := uuid.New().String()[:8]
	return fmt.Sprintf("tmux-stub-%s-%s", name, id), nil
}

// KillSession records the call and returns nil without touching any real tmux session.
func (s *Stub) KillSession(_ context.Context, _ string) error {
	s.KillSessionCalls++
	return nil
}
