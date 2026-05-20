package tmux_test

import (
	"context"
	"strings"
	"testing"

	"github.com/dnlopes/overseer/internal/adapters/secondary/tmux"
	"github.com/dnlopes/overseer/internal/core/domain"
)

// Compile-time interface satisfaction check.
var _ domain.TmuxAdapter = (*tmux.Stub)(nil)

func TestStub_CreateSession_IncrementsCalls(t *testing.T) {
	s := &tmux.Stub{}

	id, err := s.CreateSession(context.Background(), "my-session", "/tmp/wt", "bash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.CreateSessionCalls != 1 {
		t.Errorf("CreateSessionCalls = %d, want 1", s.CreateSessionCalls)
	}
	if !strings.HasPrefix(id, "tmux-stub-my-session-") {
		t.Errorf("id %q does not have expected prefix", id)
	}
	if s.LastStartDir != "/tmp/wt" {
		t.Errorf("LastStartDir = %q, want %q", s.LastStartDir, "/tmp/wt")
	}
	if s.LastShellCommand != "bash" {
		t.Errorf("LastShellCommand = %q, want %q", s.LastShellCommand, "bash")
	}
}

func TestStub_KillSession_IncrementsCalls(t *testing.T) {
	s := &tmux.Stub{}

	if err := s.KillSession(context.Background(), "some-id"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.KillSessionCalls != 1 {
		t.Errorf("KillSessionCalls = %d, want 1", s.KillSessionCalls)
	}
}

func TestStub_GetSession_IncrementsCallsAndEchoesID(t *testing.T) {
	s := &tmux.Stub{}

	got, err := s.GetSession(context.Background(), "echo-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.GetSessionCalls != 1 {
		t.Errorf("GetSessionCalls = %d, want 1", s.GetSessionCalls)
	}
	if got.ID != "echo-id" {
		t.Errorf("GetSession.ID = %q, want %q", got.ID, "echo-id")
	}
}

func TestStub_ListSessions_IncrementsCallsAndReturnsEmpty(t *testing.T) {
	s := &tmux.Stub{}

	got, err := s.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ListSessionsCalls != 1 {
		t.Errorf("ListSessionsCalls = %d, want 1", s.ListSessionsCalls)
	}
	if len(got) != 0 {
		t.Errorf("ListSessions len = %d, want 0", len(got))
	}
}

func TestStub_AttachSession_IncrementsCalls(t *testing.T) {
	s := &tmux.Stub{}

	if err := s.AttachSession(context.Background(), "some-id"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.AttachSessionCalls != 1 {
		t.Errorf("AttachSessionCalls = %d, want 1", s.AttachSessionCalls)
	}
}
