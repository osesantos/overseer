// Package git provides git adapter implementations of the domain.GitAdapter port.
package git

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/dnlopes/overseer/internal/core/domain"
)

var _ domain.GitAdapter = (*Adapter)(nil)

type Adapter struct {
	gitBin string
	logger *slog.Logger
}

func New(logger *slog.Logger) (*Adapter, error) {
	path, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("git: not found on PATH: %w", err)
	}
	return &Adapter{gitBin: path, logger: logger}, nil
}

// CreateWorktree runs `git -C <repoPath> worktree add -b <featureBranch>
// <worktreePath> <baseBranch>`, creating featureBranch from baseBranch at
// worktreePath. No --track / --no-track flag is passed: git's defaults
// handle the upstream automatically — local refs get no upstream (correct
// for new features), remote-tracking refs get auto-upstream (correct for
// continuing work on a published branch).
func (a *Adapter) CreateWorktree(_ context.Context, repoPath, baseBranch, featureBranch, worktreePath string) error {
	if _, err := a.runIn(repoPath, "worktree", "add", "-b", featureBranch, worktreePath, baseBranch); err != nil {
		return fmt.Errorf("git: create worktree %q: %w", worktreePath, err)
	}
	a.logger.Debug("git worktree created",
		"repo", repoPath,
		"base_branch", baseBranch,
		"feature_branch", featureBranch,
		"worktree", worktreePath,
	)
	return nil
}

// PullBranch fast-forwards branch from origin so that a worktree forked from
// it starts at the latest remote tip. When branch is currently checked out in
// repoPath we use `git pull --ff-only origin <branch>`, which is the only
// way git allows updating a checked-out branch. When branch is NOT checked
// out we use `git fetch origin <branch>:<branch>` to update the ref directly
// without touching the work tree.
func (a *Adapter) PullBranch(_ context.Context, repoPath, branch string) error {
	head, err := a.runIn(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("git: pull branch %q (resolve HEAD): %w", branch, err)
	}
	var pullErr error
	if strings.TrimSpace(head) == branch {
		_, pullErr = a.runIn(repoPath, "pull", "--ff-only", "origin", branch)
	} else {
		_, pullErr = a.runIn(repoPath, "fetch", "origin", branch+":"+branch)
	}
	if pullErr != nil {
		return fmt.Errorf("git: pull branch %q: %w", branch, pullErr)
	}
	a.logger.Debug("git branch pulled from origin", "repo", repoPath, "branch", branch)
	return nil
}

func (a *Adapter) RemoveWorktree(_ context.Context, repoPath, worktreePath string) error {
	if _, err := a.runIn(repoPath, "worktree", "remove", "--force", worktreePath); err != nil {
		return fmt.Errorf("git: remove worktree %q: %w", worktreePath, err)
	}
	a.logger.Debug("git worktree removed",
		"repo", repoPath,
		"worktree", worktreePath,
	)
	return nil
}

func (a *Adapter) IsGitRepo(_ context.Context, path string) error {
	stdout, err := a.runIn(path, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		if errors.Is(err, errGitNotARepo) {
			return fmt.Errorf("%w: %s", domain.ErrProjectNotGitRepo, path)
		}
		return fmt.Errorf("git: is-git-repo %q: %w", path, err)
	}
	if strings.TrimSpace(stdout) != "true" {
		return fmt.Errorf("%w: %s", domain.ErrProjectNotGitRepo, path)
	}
	return nil
}

func (a *Adapter) GetDefaultBranch(_ context.Context, repoPath string) (string, error) {
	if stdout, err := a.runIn(repoPath, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		branch := strings.TrimPrefix(strings.TrimSpace(stdout), "origin/")
		if branch != "" {
			a.logger.Debug("git default branch resolved from origin/HEAD",
				"repo", repoPath, "branch", branch,
			)
			return branch, nil
		}
	}

	for _, candidate := range []string{"main", "master"} {
		if _, err := a.runIn(repoPath, "rev-parse", "--verify", "--quiet", "refs/heads/"+candidate); err == nil {
			a.logger.Debug("git default branch resolved from local fallback",
				"repo", repoPath, "branch", candidate,
			)
			return candidate, nil
		}
	}

	return "", fmt.Errorf("%w: %s", domain.ErrProjectNoDefaultBranch, repoPath)
}

// ListBranches enumerates local + remote-tracking branches in repoPath via
// `git for-each-ref`, returning each branch's short name, scope (local vs
// remote), and committer date. The remote HEAD symbolic ref
// (origin/HEAD -> origin/main) is omitted; only concrete branches show up.
//
// Output format from git is one line per ref:
//
//	<shortname>|<objecttype>|<committerdate-iso>
//
// The pipe is unambiguous because git ref names cannot contain '|'.
func (a *Adapter) ListBranches(_ context.Context, repoPath string) ([]domain.BranchInfo, error) {
	const format = "%(refname:short)|%(objecttype)|%(committerdate:iso-strict)"

	localOut, err := a.runIn(repoPath, "for-each-ref", "--format="+format, "refs/heads")
	if err != nil {
		return nil, fmt.Errorf("git: for-each-ref refs/heads: %w", err)
	}
	remoteOut, err := a.runIn(repoPath, "for-each-ref", "--format="+format, "refs/remotes")
	if err != nil {
		return nil, fmt.Errorf("git: for-each-ref refs/remotes: %w", err)
	}

	branches := make([]domain.BranchInfo, 0)
	branches = appendParsedBranches(branches, localOut, domain.BranchScopeLocal)
	branches = appendParsedBranches(branches, remoteOut, domain.BranchScopeRemote)
	return branches, nil
}

// CurrentBranch reads HEAD's short branch name via `git rev-parse
// --abbrev-ref HEAD`. Returns "HEAD" if the repo is in a detached HEAD
// state; callers may treat that as a special-case display.
func (a *Adapter) CurrentBranch(_ context.Context, repoPath string) (string, error) {
	stdout, err := a.runIn(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git: current branch %q: %w", repoPath, err)
	}
	return strings.TrimSpace(stdout), nil
}

func appendParsedBranches(dst []domain.BranchInfo, stdout string, scope domain.BranchScope) []domain.BranchInfo {
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		if name == "" || strings.HasSuffix(name, "/HEAD") {
			continue
		}
		if parts[1] != "commit" {
			continue
		}
		when, err := time.Parse(time.RFC3339, strings.TrimSpace(parts[2]))
		if err != nil {
			when = time.Time{}
		}
		dst = append(dst, domain.BranchInfo{Name: name, Scope: scope, CommitterDate: when})
	}
	return dst
}

var errGitNotARepo = errors.New("git: not a repository")

func (a *Adapter) runIn(repoPath string, args ...string) (string, error) {
	full := append([]string{"-C", repoPath}, args...)
	cmd := exec.Command(a.gitBin, full...)
	stdout, err := cmd.Output()
	if err == nil {
		return string(stdout), nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		stderr := strings.TrimSpace(string(exitErr.Stderr))
		if strings.Contains(stderr, "not a git repository") || strings.Contains(stderr, "cannot change to") {
			return "", errGitNotARepo
		}
		return "", fmt.Errorf("%w: %s", err, stderr)
	}
	return "", err
}
