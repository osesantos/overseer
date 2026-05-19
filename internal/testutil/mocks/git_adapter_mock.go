package mocks

import "context"

type MockGitAdapter struct {
	CreateWorktreeCalls int
	CreateWorktreeErr   error

	RemoveWorktreeCalls int
	RemoveWorktreeErr   error

	IsGitRepoCalls   int
	IsGitRepoLastArg string
	IsGitRepoErr     error
}

func (m *MockGitAdapter) CreateWorktree(ctx context.Context, baseBranch, path string) error {
	m.CreateWorktreeCalls++
	return m.CreateWorktreeErr
}

func (m *MockGitAdapter) RemoveWorktree(ctx context.Context, path string) error {
	m.RemoveWorktreeCalls++
	return m.RemoveWorktreeErr
}

func (m *MockGitAdapter) IsGitRepo(ctx context.Context, path string) error {
	m.IsGitRepoCalls++
	m.IsGitRepoLastArg = path
	return m.IsGitRepoErr
}
