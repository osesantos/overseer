// Package git provides git adapter implementations of the domain.GitAdapter port.
package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dnlopes/overseer/internal/core/domain"
)

// Compile-time interface check.
var _ domain.GitAdapter = (*Stub)(nil)

type Stub struct {
	CreateWorktreeCalls int
	RemoveWorktreeCalls int
}

func (s *Stub) CreateWorktree(_ context.Context, _, _ string) error {
	s.CreateWorktreeCalls++
	return nil
}

func (s *Stub) RemoveWorktree(_ context.Context, _ string) error {
	s.RemoveWorktreeCalls++
	return nil
}

func (s *Stub) IsGitRepo(_ context.Context, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: path does not exist: %s", domain.ErrProjectNotGitRepo, path)
		}
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: not a directory: %s", domain.ErrProjectNotGitRepo, path)
	}
	gitPath := filepath.Join(path, ".git")
	if _, err := os.Stat(gitPath); err != nil {
		return fmt.Errorf("%w: missing .git in %s", domain.ErrProjectNotGitRepo, path)
	}
	return nil
}
