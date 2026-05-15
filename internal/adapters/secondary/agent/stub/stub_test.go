package stub_test

import (
	"context"
	"testing"

	"github.com/dnlopes/overseer/internal/adapters/secondary/agent/stub"
	"github.com/dnlopes/overseer/internal/core/domain/session"
)

// Compile-time interface satisfaction check.
var _ session.AgentLauncher = (*stub.Stub)(nil)

func TestLaunch_IncrementsCalls(t *testing.T) {
	s := &stub.Stub{}

	pid, err := s.Launch(context.Background(), "harness", "/workdir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.LaunchCalls != 1 {
		t.Errorf("LaunchCalls = %d, want 1", s.LaunchCalls)
	}
	if pid != 12345 {
		t.Errorf("pid = %d, want 12345", pid)
	}
}
