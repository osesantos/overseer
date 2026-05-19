package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/shared/errs"
	"github.com/dnlopes/overseer/internal/testutil"
	"github.com/dnlopes/overseer/internal/testutil/mocks"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func assertSessionOrder(t *testing.T, sessions []domain.Session, name string, wantOrder int) {
	t.Helper()
	for _, s := range sessions {
		if s.Name == name {
			if s.Order != wantOrder {
				t.Fatalf("%q: Order = %d, want %d", name, s.Order, wantOrder)
			}
			return
		}
	}
	t.Fatalf("session %q not found in response", name)
}

// --- Create ---

func TestSessionService_Create_HappyPath(t *testing.T) {
	overseerID := uuid.New()
	repo := &mocks.MockSessionRepository{}
	tmux := &mocks.MockTmuxAdapter{CreateSessionResult: "tmux-alpha"}
	git := &mocks.MockGitAdapter{}
	svc := NewSessionService(repo, tmux, git, testLogger())

	resp, err := svc.Create(context.Background(), CreateSessionRequest{Name: "alpha", ProjectID: overseerID})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.Session.Name != "alpha" {
		t.Fatalf("Create() Session.Name = %q, want %q", resp.Session.Name, "alpha")
	}
	if resp.Session.ProjectID != overseerID {
		t.Fatalf("Create() Session.ProjectID = %v, want %v", resp.Session.ProjectID, overseerID)
	}
	if resp.Session.Order != 1 {
		t.Fatalf("Create() Session.Order = %d, want 1", resp.Session.Order)
	}
	if repo.ListCalls != 1 {
		t.Fatalf("SessionRepository.List calls = %d, want 1", repo.ListCalls)
	}
	if tmux.CreateSessionCalls != 1 {
		t.Fatalf("Tmux.CreateSession calls = %d, want 1", tmux.CreateSessionCalls)
	}
	if git.CreateWorktreeCalls != 1 {
		t.Fatalf("Git.CreateWorktree calls = %d, want 1", git.CreateWorktreeCalls)
	}
	if repo.SaveCalls != 1 {
		t.Fatalf("SessionRepository.Save calls = %d, want 1", repo.SaveCalls)
	}
	if repo.SavedSession.Name != "alpha" || repo.SavedSession.ProjectID != overseerID || repo.SavedSession.Order != 1 {
		t.Fatalf("SessionRepository.Save session = %#v, want alpha/<overseerID>/order 1", repo.SavedSession)
	}
}

func TestSessionService_Create_WithoutProjectAllowed(t *testing.T) {
	repo := &mocks.MockSessionRepository{}
	svc := NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, testLogger())

	resp, err := svc.Create(context.Background(), CreateSessionRequest{Name: "orphan", ProjectID: uuid.Nil})

	if err != nil {
		t.Fatalf("Create() error = %v, want nil for project-less session", err)
	}
	if resp.Session.ProjectID != uuid.Nil {
		t.Fatalf("Create() Session.ProjectID = %v, want uuid.Nil", resp.Session.ProjectID)
	}
}

func TestSessionService_Create_EmptyName(t *testing.T) {
	repo := &mocks.MockSessionRepository{}
	tmux := &mocks.MockTmuxAdapter{}
	git := &mocks.MockGitAdapter{}
	svc := NewSessionService(repo, tmux, git, testLogger())

	_, err := svc.Create(context.Background(), CreateSessionRequest{Name: "", ProjectID: uuid.New()})

	if !errors.Is(err, domain.ErrSessionEmptyName) {
		t.Fatalf("Create() error = %v, want %v", err, domain.ErrSessionEmptyName)
	}
	if repo.ListCalls != 0 || tmux.CreateSessionCalls != 0 || git.CreateWorktreeCalls != 0 || repo.SaveCalls != 0 {
		t.Fatalf("mocks called on validation failure: list=%d tmux=%d git=%d save=%d", repo.ListCalls, tmux.CreateSessionCalls, git.CreateWorktreeCalls, repo.SaveCalls)
	}
}

func TestSessionService_Create_DuplicateNameWithinSameProject(t *testing.T) {
	overseerID := uuid.New()
	existing := testutil.MakeSession("alpha", overseerID)
	repo := &mocks.MockSessionRepository{ListResult: []domain.Session{existing}}
	svc := NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, testLogger())

	_, err := svc.Create(context.Background(), CreateSessionRequest{Name: "alpha", ProjectID: overseerID})

	if !errors.Is(err, domain.ErrSessionAlreadyExists) {
		t.Fatalf("Create() error = %v, want %v", err, domain.ErrSessionAlreadyExists)
	}
	if repo.SaveCalls != 0 {
		t.Fatalf("SessionRepository.Save calls = %d, want 0 on duplicate", repo.SaveCalls)
	}
}

func TestSessionService_Create_DuplicateNameAcrossProjectsAllowed(t *testing.T) {
	overseerID := uuid.New()
	otherID := uuid.New()
	existing := testutil.MakeSession("alpha", otherID)
	repo := &mocks.MockSessionRepository{ListResult: []domain.Session{existing}}
	svc := NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, testLogger())

	_, err := svc.Create(context.Background(), CreateSessionRequest{Name: "alpha", ProjectID: overseerID})

	if err != nil {
		t.Fatalf("Create() error = %v, want nil (same name in different project is allowed)", err)
	}
}

func TestSessionService_Create_DuplicateNameAmongUnassignedSessions(t *testing.T) {
	existing := testutil.MakeSession("solo", uuid.Nil)
	repo := &mocks.MockSessionRepository{ListResult: []domain.Session{existing}}
	svc := NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, testLogger())

	_, err := svc.Create(context.Background(), CreateSessionRequest{Name: "solo", ProjectID: uuid.Nil})

	if !errors.Is(err, domain.ErrSessionAlreadyExists) {
		t.Fatalf("Create() error = %v, want %v (duplicate in unassigned bucket)", err, domain.ErrSessionAlreadyExists)
	}
}

func TestSessionService_Create_OrderIncrement(t *testing.T) {
	overseerID := uuid.New()
	otherID := uuid.New()
	first := testutil.MakeSession("alpha", overseerID)
	first.Order = 1
	second := testutil.MakeSession("beta", overseerID)
	second.Order = 2
	otherProject := testutil.MakeSession("gamma", otherID)
	otherProject.Order = 9
	repo := &mocks.MockSessionRepository{ListResult: []domain.Session{first, second, otherProject}}
	svc := NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, testLogger())

	resp, err := svc.Create(context.Background(), CreateSessionRequest{Name: "gamma", ProjectID: overseerID})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.Session.Order != 3 {
		t.Fatalf("Create() Session.Order = %d, want 3", resp.Session.Order)
	}
	if repo.SavedSession.Order != 3 {
		t.Fatalf("SessionRepository.Save Order = %d, want 3", repo.SavedSession.Order)
	}
}

func TestSessionService_Create_TmuxError(t *testing.T) {
	tmuxErr := errors.New("tmux unavailable")
	repo := &mocks.MockSessionRepository{}
	tmux := &mocks.MockTmuxAdapter{CreateSessionErr: tmuxErr}
	git := &mocks.MockGitAdapter{}
	svc := NewSessionService(repo, tmux, git, testLogger())

	_, err := svc.Create(context.Background(), CreateSessionRequest{Name: "alpha", ProjectID: uuid.New()})

	if !errors.Is(err, tmuxErr) {
		t.Fatalf("Create() error = %v, want wrapped %v", err, tmuxErr)
	}
	if repo.SaveCalls != 0 {
		t.Fatalf("SessionRepository.Save calls = %d, want 0", repo.SaveCalls)
	}
	if git.CreateWorktreeCalls != 0 {
		t.Fatalf("Git.CreateWorktree calls = %d, want 0", git.CreateWorktreeCalls)
	}
}

func TestSessionService_Create_FirstSessionOrder(t *testing.T) {
	overseerID := uuid.New()
	otherID := uuid.New()
	otherProject := testutil.MakeSession("alpha", otherID)
	otherProject.Order = 4
	repo := &mocks.MockSessionRepository{ListResult: []domain.Session{otherProject}}
	svc := NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, testLogger())

	resp, err := svc.Create(context.Background(), CreateSessionRequest{Name: "alpha", ProjectID: overseerID})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.Session.Order != 1 {
		t.Fatalf("Create() Session.Order = %d, want 1", resp.Session.Order)
	}
}

// --- Rename ---

func TestSessionService_Rename_HappyPath(t *testing.T) {
	original := testutil.MakeSession("alpha", uuid.New())
	repo := &mocks.MockSessionRepository{
		GetResult:  original,
		ListResult: []domain.Session{original},
	}
	svc := NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, testLogger())

	resp, err := svc.Rename(context.Background(), RenameSessionRequest{ID: original.ID, NewName: "beta"})

	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if resp.Session.Name != "beta" {
		t.Fatalf("Rename() Session.Name = %q, want %q", resp.Session.Name, "beta")
	}
	if repo.GetCalls != 1 {
		t.Fatalf("SessionRepository.Get calls = %d, want 1", repo.GetCalls)
	}
	if repo.ListCalls != 1 {
		t.Fatalf("SessionRepository.List calls = %d, want 1", repo.ListCalls)
	}
	if repo.SaveCalls != 1 {
		t.Fatalf("SessionRepository.Save calls = %d, want 1", repo.SaveCalls)
	}
	if repo.SavedSession.Name != "beta" {
		t.Fatalf("SessionRepository.Save Session.Name = %q, want %q", repo.SavedSession.Name, "beta")
	}
}

func TestSessionService_Rename_EmptyName(t *testing.T) {
	original := testutil.MakeSession("alpha", uuid.New())
	repo := &mocks.MockSessionRepository{GetResult: original}
	svc := NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, testLogger())

	_, err := svc.Rename(context.Background(), RenameSessionRequest{ID: original.ID, NewName: ""})

	if !errors.Is(err, domain.ErrSessionEmptyName) {
		t.Fatalf("Rename() error = %v, want %v", err, domain.ErrSessionEmptyName)
	}
	if repo.SaveCalls != 0 {
		t.Fatalf("SessionRepository.Save calls = %d, want 0 on validation failure", repo.SaveCalls)
	}
}

func TestSessionService_Rename_NotFound(t *testing.T) {
	repo := &mocks.MockSessionRepository{GetErr: domain.ErrSessionNotFound}
	svc := NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, testLogger())

	_, err := svc.Rename(context.Background(), RenameSessionRequest{ID: uuid.New(), NewName: "beta"})

	if !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("Rename() error = %v, want %v", err, domain.ErrSessionNotFound)
	}
	if repo.SaveCalls != 0 {
		t.Fatalf("SessionRepository.Save calls = %d, want 0 when not found", repo.SaveCalls)
	}
}

func TestSessionService_Rename_DuplicateNameInSameProject(t *testing.T) {
	overseerID := uuid.New()
	original := testutil.MakeSession("alpha", overseerID)
	conflicting := testutil.MakeSession("beta", overseerID)
	repo := &mocks.MockSessionRepository{
		GetResult:  original,
		ListResult: []domain.Session{original, conflicting},
	}
	svc := NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, testLogger())

	_, err := svc.Rename(context.Background(), RenameSessionRequest{ID: original.ID, NewName: "beta"})

	if !errors.Is(err, domain.ErrSessionAlreadyExists) {
		t.Fatalf("Rename() error = %v, want %v", err, domain.ErrSessionAlreadyExists)
	}
	if repo.SaveCalls != 0 {
		t.Fatalf("SessionRepository.Save calls = %d, want 0 on duplicate", repo.SaveCalls)
	}
}

func TestSessionService_Rename_UpdatedAtChanges(t *testing.T) {
	original := testutil.MakeSession("alpha", uuid.New())
	original.UpdatedAt = time.Now().Add(-time.Minute)
	beforeRename := original.UpdatedAt

	repo := &mocks.MockSessionRepository{
		GetResult:  original,
		ListResult: []domain.Session{original},
	}
	svc := NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, testLogger())

	_, err := svc.Rename(context.Background(), RenameSessionRequest{ID: original.ID, NewName: "beta"})

	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if !repo.SavedSession.UpdatedAt.After(beforeRename) {
		t.Fatalf("SavedSession.UpdatedAt = %v, want after %v", repo.SavedSession.UpdatedAt, beforeRename)
	}
}

// --- List ---

func TestSessionService_List_Empty(t *testing.T) {
	repo := &mocks.MockSessionRepository{}
	svc := NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, testLogger())

	resp, err := svc.List(context.Background(), ListSessionsRequest{})

	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(resp.Sessions) != 0 {
		t.Fatalf("List() len(Sessions) = %d, want 0", len(resp.Sessions))
	}
	if repo.ListCalls != 1 {
		t.Fatalf("SessionRepository.List calls = %d, want 1", repo.ListCalls)
	}
}

func TestSessionService_List_SortsByOrderWithinSameProject(t *testing.T) {
	projectID := uuid.New()
	s1 := testutil.MakeSession("alpha", projectID)
	s1.Order = 2
	s2 := testutil.MakeSession("beta", projectID)
	s2.Order = 1
	s3 := testutil.MakeSession("gamma", projectID)
	s3.Order = 3
	repo := &mocks.MockSessionRepository{ListResult: []domain.Session{s1, s2, s3}}
	svc := NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, testLogger())

	resp, err := svc.List(context.Background(), ListSessionsRequest{})

	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(resp.Sessions) != 3 {
		t.Fatalf("List() len(Sessions) = %d, want 3", len(resp.Sessions))
	}
	if resp.Sessions[0].Name != "beta" {
		t.Fatalf("Sessions[0].Name = %q, want %q", resp.Sessions[0].Name, "beta")
	}
	if resp.Sessions[1].Name != "alpha" {
		t.Fatalf("Sessions[1].Name = %q, want %q", resp.Sessions[1].Name, "alpha")
	}
	if resp.Sessions[2].Name != "gamma" {
		t.Fatalf("Sessions[2].Name = %q, want %q", resp.Sessions[2].Name, "gamma")
	}
}

func TestSessionService_List_GroupsByProjectID(t *testing.T) {
	lowID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	highID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	highSession := testutil.MakeSession("alpha", highID)
	highSession.Order = 1
	lowSession := testutil.MakeSession("beta", lowID)
	lowSession.Order = 1
	repo := &mocks.MockSessionRepository{ListResult: []domain.Session{highSession, lowSession}}
	svc := NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, testLogger())

	resp, err := svc.List(context.Background(), ListSessionsRequest{})

	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(resp.Sessions) != 2 {
		t.Fatalf("List() len(Sessions) = %d, want 2", len(resp.Sessions))
	}
	if resp.Sessions[0].ProjectID != lowID {
		t.Fatalf("Sessions[0].ProjectID = %v, want lowID", resp.Sessions[0].ProjectID)
	}
	if resp.Sessions[1].ProjectID != highID {
		t.Fatalf("Sessions[1].ProjectID = %v, want highID", resp.Sessions[1].ProjectID)
	}
}

func TestSessionService_List_OrderWithinGroup(t *testing.T) {
	projectID := uuid.New()
	s1 := testutil.MakeSession("first", projectID)
	s1.Order = 10
	s2 := testutil.MakeSession("second", projectID)
	s2.Order = 3
	s3 := testutil.MakeSession("third", projectID)
	s3.Order = 7
	repo := &mocks.MockSessionRepository{ListResult: []domain.Session{s1, s2, s3}}
	svc := NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, testLogger())

	resp, err := svc.List(context.Background(), ListSessionsRequest{})

	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	sessions := resp.Sessions
	if sessions[0].Order != 3 || sessions[1].Order != 7 || sessions[2].Order != 10 {
		t.Fatalf("Sessions not sorted by Order ASC: got %d,%d,%d, want 3,7,10",
			sessions[0].Order, sessions[1].Order, sessions[2].Order)
	}
}

// --- Reorder ---

func TestSessionService_Reorder_MoveDown(t *testing.T) {
	projectID := uuid.New()
	a := testutil.MakeSession("A", projectID)
	a.Order = 1
	b := testutil.MakeSession("B", projectID)
	b.Order = 2
	c := testutil.MakeSession("C", projectID)
	c.Order = 3

	repo := &mocks.MockSessionRepository{
		GetResult:  b,
		ListResult: []domain.Session{a, b, c},
	}
	svc := NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, testLogger())

	resp, err := svc.Reorder(context.Background(), ReorderSessionRequest{ID: b.ID, Direction: 1})

	if err != nil {
		t.Fatalf("Reorder() error = %v", err)
	}
	if repo.SaveCalls != 2 {
		t.Fatalf("SaveCalls = %d, want 2", repo.SaveCalls)
	}
	if len(resp.Sessions) != 3 {
		t.Fatalf("len(resp.Sessions) = %d, want 3", len(resp.Sessions))
	}
	assertSessionOrder(t, resp.Sessions, "A", 1)
	assertSessionOrder(t, resp.Sessions, "C", 2)
	assertSessionOrder(t, resp.Sessions, "B", 3)
}

func TestSessionService_Reorder_MoveUp(t *testing.T) {
	projectID := uuid.New()
	a := testutil.MakeSession("A", projectID)
	a.Order = 1
	b := testutil.MakeSession("B", projectID)
	b.Order = 2
	c := testutil.MakeSession("C", projectID)
	c.Order = 3

	repo := &mocks.MockSessionRepository{
		GetResult:  b,
		ListResult: []domain.Session{a, b, c},
	}
	svc := NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, testLogger())

	resp, err := svc.Reorder(context.Background(), ReorderSessionRequest{ID: b.ID, Direction: -1})

	if err != nil {
		t.Fatalf("Reorder() error = %v", err)
	}
	if repo.SaveCalls != 2 {
		t.Fatalf("SaveCalls = %d, want 2", repo.SaveCalls)
	}
	if len(resp.Sessions) != 3 {
		t.Fatalf("len(resp.Sessions) = %d, want 3", len(resp.Sessions))
	}
	assertSessionOrder(t, resp.Sessions, "B", 1)
	assertSessionOrder(t, resp.Sessions, "A", 2)
	assertSessionOrder(t, resp.Sessions, "C", 3)
}

func TestSessionService_Reorder_BoundaryFirst_Up(t *testing.T) {
	projectID := uuid.New()
	a := testutil.MakeSession("A", projectID)
	a.Order = 1
	b := testutil.MakeSession("B", projectID)
	b.Order = 2
	c := testutil.MakeSession("C", projectID)
	c.Order = 3

	repo := &mocks.MockSessionRepository{
		GetResult:  a,
		ListResult: []domain.Session{a, b, c},
	}
	svc := NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, testLogger())

	_, err := svc.Reorder(context.Background(), ReorderSessionRequest{ID: a.ID, Direction: -1})

	if !errors.Is(err, errs.ErrNoOp) {
		t.Fatalf("Reorder() error = %v, want %v", err, errs.ErrNoOp)
	}
	if repo.SaveCalls != 0 {
		t.Fatalf("SaveCalls = %d, want 0 (Save must not be called on boundary)", repo.SaveCalls)
	}
}

func TestSessionService_Reorder_BoundaryLast_Down(t *testing.T) {
	projectID := uuid.New()
	a := testutil.MakeSession("A", projectID)
	a.Order = 1
	b := testutil.MakeSession("B", projectID)
	b.Order = 2
	c := testutil.MakeSession("C", projectID)
	c.Order = 3

	repo := &mocks.MockSessionRepository{
		GetResult:  c,
		ListResult: []domain.Session{a, b, c},
	}
	svc := NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, testLogger())

	_, err := svc.Reorder(context.Background(), ReorderSessionRequest{ID: c.ID, Direction: 1})

	if !errors.Is(err, errs.ErrNoOp) {
		t.Fatalf("Reorder() error = %v, want %v", err, errs.ErrNoOp)
	}
	if repo.SaveCalls != 0 {
		t.Fatalf("SaveCalls = %d, want 0 (Save must not be called on boundary)", repo.SaveCalls)
	}
}

func TestSessionService_Reorder_SingleSession(t *testing.T) {
	a := testutil.MakeSession("A", uuid.New())
	a.Order = 1

	repo := &mocks.MockSessionRepository{
		GetResult:  a,
		ListResult: []domain.Session{a},
	}
	svc := NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, testLogger())

	_, err := svc.Reorder(context.Background(), ReorderSessionRequest{ID: a.ID, Direction: 1})

	if !errors.Is(err, errs.ErrNoOp) {
		t.Fatalf("Reorder() error = %v, want %v", err, errs.ErrNoOp)
	}
	if repo.SaveCalls != 0 {
		t.Fatalf("SaveCalls = %d, want 0 (Save must not be called for single-session group)", repo.SaveCalls)
	}
}

func TestSessionService_Reorder_NotFound(t *testing.T) {
	repo := &mocks.MockSessionRepository{
		GetErr: domain.ErrSessionNotFound,
	}
	svc := NewSessionService(repo, &mocks.MockTmuxAdapter{}, &mocks.MockGitAdapter{}, testLogger())

	_, err := svc.Reorder(context.Background(), ReorderSessionRequest{ID: uuid.New(), Direction: 1})

	if !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("Reorder() error = %v, want %v", err, domain.ErrSessionNotFound)
	}
	if repo.SaveCalls != 0 {
		t.Fatalf("SaveCalls = %d, want 0", repo.SaveCalls)
	}
}
