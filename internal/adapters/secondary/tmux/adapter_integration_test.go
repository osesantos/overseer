//go:build integration

package tmux_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/secondary/tmux"
	"github.com/dnlopes/overseer/internal/core/domain"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func uniqueSessionName(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("overseer-it-%s-%s", t.Name(), uuid.NewString()[:8])
}

func newAdapter(t *testing.T) *tmux.Adapter {
	t.Helper()
	a, err := tmux.New(discardLogger())
	if err != nil {
		t.Skipf("tmux not available: %v", err)
	}
	return a
}

func TestIntegration_Adapter_CreateGetKill(t *testing.T) {
	a := newAdapter(t)
	ctx := context.Background()
	name := uniqueSessionName(t)

	tmuxID, err := a.CreateSession(ctx, name, "", "")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	t.Cleanup(func() { _ = a.KillSession(ctx, tmuxID) })

	if tmuxID != name {
		t.Errorf("CreateSession() tmuxID = %q, want %q", tmuxID, name)
	}

	got, err := a.GetSession(ctx, tmuxID)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if got.ID != name {
		t.Errorf("GetSession() ID = %q, want %q", got.ID, name)
	}
	if got.CreatedAt.IsZero() {
		t.Errorf("GetSession() CreatedAt is zero, want a populated time")
	}
	if time.Since(got.CreatedAt) > time.Minute {
		t.Errorf("GetSession() CreatedAt = %v, want a recent time", got.CreatedAt)
	}

	if err := a.KillSession(ctx, tmuxID); err != nil {
		t.Fatalf("KillSession() error = %v", err)
	}

	_, err = a.GetSession(ctx, tmuxID)
	if !errors.Is(err, domain.ErrTmuxSessionNotFound) {
		t.Errorf("GetSession() after kill error = %v, want %v", err, domain.ErrTmuxSessionNotFound)
	}
}

func TestIntegration_Adapter_ListSessions_IncludesCreated(t *testing.T) {
	a := newAdapter(t)
	ctx := context.Background()
	name := uniqueSessionName(t)

	tmuxID, err := a.CreateSession(ctx, name, "", "")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	t.Cleanup(func() { _ = a.KillSession(ctx, tmuxID) })

	all, err := a.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}

	found := false
	for _, s := range all {
		if s.ID == tmuxID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ListSessions() did not include the created session %q", tmuxID)
	}
}

func TestIntegration_Adapter_KillSession_UnknownReturnsNotFound(t *testing.T) {
	a := newAdapter(t)

	err := a.KillSession(context.Background(), uniqueSessionName(t))
	if !errors.Is(err, domain.ErrTmuxSessionNotFound) {
		t.Errorf("KillSession() unknown error = %v, want %v", err, domain.ErrTmuxSessionNotFound)
	}
}

func TestIntegration_Adapter_CreateSession_DuplicateReturnsError(t *testing.T) {
	a := newAdapter(t)
	ctx := context.Background()
	name := uniqueSessionName(t)

	tmuxID, err := a.CreateSession(ctx, name, "", "")
	if err != nil {
		t.Fatalf("CreateSession() first error = %v", err)
	}
	t.Cleanup(func() { _ = a.KillSession(ctx, tmuxID) })

	if _, err := a.CreateSession(ctx, name, "", ""); err == nil {
		t.Errorf("CreateSession() duplicate error = nil, want error from tmux duplicate-name rejection")
	}
}

func TestIntegration_Adapter_CreateSession_StartDirIsApplied(t *testing.T) {
	a := newAdapter(t)
	ctx := context.Background()
	name := uniqueSessionName(t)

	wantDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks() temp dir error = %v", err)
	}

	tmuxID, err := a.CreateSession(ctx, name, wantDir, "")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	t.Cleanup(func() { _ = a.KillSession(ctx, tmuxID) })

	out, err := exec.Command("tmux", "display-message", "-p", "-t", tmuxID, "#{session_path}").Output()
	if err != nil {
		t.Fatalf("tmux display-message error = %v", err)
	}
	gotDir, err := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatalf("EvalSymlinks() session path error = %v", err)
	}
	if gotDir != wantDir {
		t.Errorf("session_path = %q, want %q", gotDir, wantDir)
	}
}

func TestIntegration_Adapter_CreateSession_ShellCommandIsApplied(t *testing.T) {
	a := newAdapter(t)
	ctx := context.Background()
	name := uniqueSessionName(t)

	tmuxID, err := a.CreateSession(ctx, name, "", "sleep 30")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	t.Cleanup(func() { _ = a.KillSession(ctx, tmuxID) })

	out, err := exec.Command("tmux", "display-message", "-p", "-t", tmuxID, "#{pane_start_command}").Output()
	if err != nil {
		t.Fatalf("tmux display-message error = %v", err)
	}
	got := strings.Trim(strings.TrimSpace(string(out)), `"`)
	if got != "sleep 30" {
		t.Errorf("pane_start_command = %q, want %q", got, "sleep 30")
	}
}
