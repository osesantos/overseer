package git_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/dnlopes/overseer/internal/adapters/secondary/git"
	"github.com/dnlopes/overseer/internal/core/domain"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func seedRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitCommands := [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "test@overseer.local"},
		{"config", "user.name", "Overseer Test"},
		{"commit", "--allow-empty", "-q", "-m", "init"},
	}
	for _, args := range gitCommands {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestAdapter_NewReturnsAdapterWhenGitPresent(t *testing.T) {
	a, err := git.New(discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if a == nil {
		t.Fatal("New() returned nil adapter")
	}
}

func TestAdapter_CreateWorktreeCreatesBranchedWorktree(t *testing.T) {
	a, err := git.New(discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	repo := seedRepo(t)
	worktree := filepath.Join(t.TempDir(), "wt-alpha")

	if err := a.CreateWorktree(context.Background(), repo, "main", "overseer/alpha", worktree); err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}

	if info, err := os.Stat(worktree); err != nil {
		t.Fatalf("worktree path missing: %v", err)
	} else if !info.IsDir() {
		t.Fatalf("worktree path is not a directory: %v", info.Mode())
	}

	head, err := exec.Command("git", "-C", worktree, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD in worktree: %v", err)
	}
	if got, want := string(head), "overseer/alpha\n"; got != want {
		t.Fatalf("worktree HEAD = %q, want %q", got, want)
	}
}

func TestAdapter_CreateWorktreeFailsForMissingBaseBranch(t *testing.T) {
	a, err := git.New(discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	repo := seedRepo(t)
	worktree := filepath.Join(t.TempDir(), "wt-alpha")

	err = a.CreateWorktree(context.Background(), repo, "no-such-branch", "overseer/alpha", worktree)
	if err == nil {
		t.Fatalf("CreateWorktree() error = nil, want non-nil for missing base branch")
	}
}

func TestAdapter_RemoveWorktreeRemovesIt(t *testing.T) {
	a, err := git.New(discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	repo := seedRepo(t)
	worktree := filepath.Join(t.TempDir(), "wt-alpha")

	if err := a.CreateWorktree(context.Background(), repo, "main", "overseer/alpha", worktree); err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}
	if err := a.RemoveWorktree(context.Background(), repo, worktree); err != nil {
		t.Fatalf("RemoveWorktree() error = %v", err)
	}

	if _, err := os.Stat(worktree); !os.IsNotExist(err) {
		t.Fatalf("worktree path still exists or unexpected error: %v", err)
	}
}

func TestAdapter_IsGitRepoReturnsNilForRealRepo(t *testing.T) {
	a, err := git.New(discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	repo := seedRepo(t)
	if err := a.IsGitRepo(context.Background(), repo); err != nil {
		t.Fatalf("IsGitRepo() error = %v, want nil", err)
	}
}

func TestAdapter_IsGitRepoReturnsSentinelForNonRepo(t *testing.T) {
	a, err := git.New(discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	dir := t.TempDir()
	err = a.IsGitRepo(context.Background(), dir)
	if !errors.Is(err, domain.ErrProjectNotGitRepo) {
		t.Fatalf("IsGitRepo() error = %v, want wrapped %v", err, domain.ErrProjectNotGitRepo)
	}
}

func TestAdapter_IsGitRepoReturnsSentinelForMissingPath(t *testing.T) {
	a, err := git.New(discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	err = a.IsGitRepo(context.Background(), filepath.Join(t.TempDir(), "does-not-exist"))
	if !errors.Is(err, domain.ErrProjectNotGitRepo) {
		t.Fatalf("IsGitRepo() error = %v, want wrapped %v", err, domain.ErrProjectNotGitRepo)
	}
}

func TestAdapter_GetDefaultBranchFallsBackToMain(t *testing.T) {
	a, err := git.New(discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	repo := seedRepo(t)

	branch, err := a.GetDefaultBranch(context.Background(), repo)
	if err != nil {
		t.Fatalf("GetDefaultBranch() error = %v", err)
	}
	if branch != "main" {
		t.Fatalf("GetDefaultBranch() = %q, want %q (no origin → local main fallback)", branch, "main")
	}
}

func TestAdapter_GetDefaultBranchFallsBackToMaster(t *testing.T) {
	a, err := git.New(discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	dir := t.TempDir()
	gitCommands := [][]string{
		{"init", "-q", "-b", "master"},
		{"config", "user.email", "test@overseer.local"},
		{"config", "user.name", "Overseer Test"},
		{"commit", "--allow-empty", "-q", "-m", "init"},
	}
	for _, args := range gitCommands {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	branch, err := a.GetDefaultBranch(context.Background(), dir)
	if err != nil {
		t.Fatalf("GetDefaultBranch() error = %v", err)
	}
	if branch != "master" {
		t.Fatalf("GetDefaultBranch() = %q, want %q (no origin → local master fallback)", branch, "master")
	}
}

func TestAdapter_GetDefaultBranchReturnsSentinelWhenNothingResolves(t *testing.T) {
	a, err := git.New(discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	dir := t.TempDir()
	gitCommands := [][]string{
		{"init", "-q", "-b", "trunk"},
		{"config", "user.email", "test@overseer.local"},
		{"config", "user.name", "Overseer Test"},
		{"commit", "--allow-empty", "-q", "-m", "init"},
	}
	for _, args := range gitCommands {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	_, err = a.GetDefaultBranch(context.Background(), dir)
	if !errors.Is(err, domain.ErrProjectNoDefaultBranch) {
		t.Fatalf("GetDefaultBranch() error = %v, want wrapped %v", err, domain.ErrProjectNoDefaultBranch)
	}
}

func seedRepoWithRemote(t *testing.T) string {
	t.Helper()
	upstream := t.TempDir()
	if out, err := exec.Command("git", "init", "--bare", "-q", "-b", "main", upstream).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	repo := t.TempDir()
	cmds := [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "test@overseer.local"},
		{"config", "user.name", "Overseer Test"},
		{"commit", "--allow-empty", "-q", "-m", "init"},
		{"remote", "add", "origin", upstream},
		{"push", "-q", "origin", "main"},
		{"fetch", "-q", "origin"},
	}
	for _, args := range cmds {
		full := append([]string{"-C", repo}, args...)
		if out, err := exec.Command("git", full...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return repo
}

func TestAdapter_PullBranch_WithRemote_FastForwards(t *testing.T) {
	a, err := git.New(discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Create a bare upstream, a primary clone (the repo under test), and a
	// second clone that simulates another developer pushing new work.
	upstream := t.TempDir()
	if out, err := exec.Command("git", "init", "--bare", "-q", "-b", "main", upstream).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}

	setupClone := func(dir string, remote string) {
		cmds := [][]string{
			{"init", "-q", "-b", "main"},
			{"config", "user.email", "test@overseer.local"},
			{"config", "user.name", "Overseer Test"},
			{"commit", "--allow-empty", "-q", "-m", "init"},
			{"remote", "add", "origin", remote},
			{"push", "-q", "origin", "main"},
		}
		for _, args := range cmds {
			if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
	}

	repo := t.TempDir()
	setupClone(repo, upstream)

	// Second clone pushes a new commit to origin, simulating remote work.
	other := t.TempDir()
	otherCmds := [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "test@overseer.local"},
		{"config", "user.name", "Overseer Test"},
		{"remote", "add", "origin", upstream},
		{"fetch", "-q", "origin"},
		{"checkout", "-q", "-b", "main", "--track", "origin/main"},
		{"commit", "--allow-empty", "-q", "-m", "remote work"},
		{"push", "-q", "origin", "main"},
	}
	for _, args := range otherCmds {
		if out, err := exec.Command("git", append([]string{"-C", other}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v in other clone: %v\n%s", args, err, out)
		}
	}

	localBefore, _ := exec.Command("git", "-C", repo, "rev-parse", "main").Output()

	if err := a.PullBranch(context.Background(), repo, "main"); err != nil {
		t.Fatalf("PullBranch() error = %v", err)
	}

	localAfter, _ := exec.Command("git", "-C", repo, "rev-parse", "main").Output()
	if string(localBefore) == string(localAfter) {
		t.Fatal("PullBranch() did not advance local main; expected local to move forward after remote commit")
	}
}

func TestAdapter_PullBranch_NoRemote_ReturnsError(t *testing.T) {
	a, err := git.New(discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	repo := seedRepo(t) // no origin configured

	err = a.PullBranch(context.Background(), repo, "main")
	if err == nil {
		t.Fatal("PullBranch() error = nil, want non-nil for repo with no remote")
	}
}

func TestAdapter_ListBranches_LocalRepo_ReturnsHEADBranchAsLocal(t *testing.T) {
	a, err := git.New(discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	repo := seedRepo(t)

	branches, err := a.ListBranches(context.Background(), repo)
	if err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}
	if len(branches) != 1 {
		t.Fatalf("ListBranches() len = %d, want 1 (only main)", len(branches))
	}
	if branches[0].Name != "main" {
		t.Fatalf("ListBranches()[0].Name = %q, want %q", branches[0].Name, "main")
	}
	if branches[0].Scope != domain.BranchScopeLocal {
		t.Fatalf("ListBranches()[0].Scope = %v, want local", branches[0].Scope)
	}
	if branches[0].CommitterDate.IsZero() {
		t.Fatal("ListBranches()[0].CommitterDate is zero, want non-zero")
	}
}

func TestAdapter_ListBranches_RepoWithRemote_ReturnsLocalAndRemote(t *testing.T) {
	a, err := git.New(discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	repo := seedRepoWithRemote(t)

	branches, err := a.ListBranches(context.Background(), repo)
	if err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}

	hasLocal := slices.ContainsFunc(branches, func(b domain.BranchInfo) bool {
		return b.Name == "main" && b.Scope == domain.BranchScopeLocal
	})
	hasRemote := slices.ContainsFunc(branches, func(b domain.BranchInfo) bool {
		return b.Name == "origin/main" && b.Scope == domain.BranchScopeRemote
	})
	if !hasLocal {
		t.Errorf("ListBranches() missing local main branch: %+v", branches)
	}
	if !hasRemote {
		t.Errorf("ListBranches() missing remote origin/main: %+v", branches)
	}

	hasHEAD := slices.ContainsFunc(branches, func(b domain.BranchInfo) bool {
		return b.Name == "origin/HEAD"
	})
	if hasHEAD {
		t.Errorf("ListBranches() unexpectedly included origin/HEAD symbolic ref: %+v", branches)
	}
}

func TestAdapter_ListBranches_NotARepo_ReturnsError(t *testing.T) {
	a, err := git.New(discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = a.ListBranches(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("ListBranches() error = nil, want non-nil for non-repo")
	}
}

func TestAdapter_CurrentBranch_ReturnsHEADName(t *testing.T) {
	a, err := git.New(discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	repo := seedRepo(t)

	branch, err := a.CurrentBranch(context.Background(), repo)
	if err != nil {
		t.Fatalf("CurrentBranch() error = %v", err)
	}
	if branch != "main" {
		t.Fatalf("CurrentBranch() = %q, want %q", branch, "main")
	}
}

func TestAdapter_CurrentBranch_NotARepo_ReturnsError(t *testing.T) {
	a, err := git.New(discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = a.CurrentBranch(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("CurrentBranch() error = nil, want non-nil for non-repo")
	}
}
