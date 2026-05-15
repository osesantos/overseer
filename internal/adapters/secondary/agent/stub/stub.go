// Package stub provides a stub implementation of the agent launcher adapter.
// This is a stub adapter. It satisfies the port interface with canned responses.
// Replace with real implementation when integrating real agent launching.
package stub

import (
	"context"

	"github.com/dnlopes/overseer/internal/core/domain/session"
)

// Compile-time interface check.
var _ session.AgentLauncher = (*Stub)(nil)

// Stub is a stub implementation of session.AgentLauncher.
// It records call counts for test assertions and returns a fixed canned PID.
type Stub struct {
	LaunchCalls int
}

// Launch records the call and returns a canned PID of 12345 without spawning any process.
func (s *Stub) Launch(_ context.Context, _, _ string) (int, error) {
	s.LaunchCalls++
	return 12345, nil
}
