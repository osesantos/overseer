// Package stub provides a stub implementation of the git adapter.
// This is a stub adapter. It satisfies the port interface with canned responses.
// Replace with real implementation when integrating real git.
package stub

import (
	"context"

	"github.com/dnlopes/overseer/internal/core/domain/session"
)

// Compile-time interface check.
var _ session.GitAdapter = (*Stub)(nil)

// Stub is a stub implementation of session.GitAdapter.
// It records call counts for test assertions and returns nil for all operations.
type Stub struct {
	CreateWorktreeCalls int
	RemoveWorktreeCalls int
}

// CreateWorktree records the call and returns nil without touching any real git worktree.
func (s *Stub) CreateWorktree(_ context.Context, _, _ string) error {
	s.CreateWorktreeCalls++
	return nil
}

// RemoveWorktree records the call and returns nil without touching any real git worktree.
func (s *Stub) RemoveWorktree(_ context.Context, _ string) error {
	s.RemoveWorktreeCalls++
	return nil
}
