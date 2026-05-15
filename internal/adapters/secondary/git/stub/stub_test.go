package stub_test

import (
	"context"
	"testing"

	"github.com/dnlopes/overseer/internal/adapters/secondary/git/stub"
	"github.com/dnlopes/overseer/internal/core/domain/session"
)

// Compile-time interface satisfaction check.
var _ session.GitAdapter = (*stub.Stub)(nil)

func TestCreateWorktree_IncrementsCalls(t *testing.T) {
	s := &stub.Stub{}

	if err := s.CreateWorktree(context.Background(), "main", "/tmp/worktree"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.CreateWorktreeCalls != 1 {
		t.Errorf("CreateWorktreeCalls = %d, want 1", s.CreateWorktreeCalls)
	}
}

func TestRemoveWorktree_IncrementsCalls(t *testing.T) {
	s := &stub.Stub{}

	if err := s.RemoveWorktree(context.Background(), "/tmp/worktree"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.RemoveWorktreeCalls != 1 {
		t.Errorf("RemoveWorktreeCalls = %d, want 1", s.RemoveWorktreeCalls)
	}
}
