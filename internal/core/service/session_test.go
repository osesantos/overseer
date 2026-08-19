package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os/exec"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/shared/errs"
	"github.com/dnlopes/overseer/internal/shared/paths"
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

func newSessionMocks(t *testing.T) (*mocks.MockSessionRepository, *mocks.MockProjectRepository, *mocks.MockTmuxAdapter, *mocks.MockGitAdapter) {
	t.Helper()
	return mocks.NewMockSessionRepository(t),
		mocks.NewMockProjectRepository(t),
		mocks.NewMockTmuxAdapter(t),
		mocks.NewMockGitAdapter(t)
}

// expectAgentAndEditorTmuxGone stubs the Delete teardown's inspection of the
// -agent and -editor tmux sessions when they no longer exist, so the shell-only
// teardown expectations in a test stay focused on the shell session.
func expectAgentAndEditorTmuxGone(tmux *mocks.MockTmuxAdapter, baseID string) {
	tmux.EXPECT().GetSession(mock.Anything, baseID+"-agent").
		Return(domain.TmuxSession{}, domain.ErrTmuxSessionNotFound).Once()
	tmux.EXPECT().GetSession(mock.Anything, baseID+"-editor").
		Return(domain.TmuxSession{}, domain.ErrTmuxSessionNotFound).Once()
}

// newTestSessionService builds a service with a throwaway board mock, for the
// majority of tests that never touch a swarm. Swarm tests use
// newTestSessionServiceWithBoard so they can assert on board calls.
func newTestSessionService(
	repo domain.SessionRepository,
	projects domain.ProjectRepository,
	tmux domain.TmuxAdapter,
	git domain.GitAdapter,
	logger *slog.Logger,
) *SessionService {
	launcher, _ := domain.NewLauncher("OpenCode", "opencode", domain.AgentTypeOpenCode)
	return NewSessionService(repo, projects, tmux, git, noopBoard{}, paths.NewResolver(""), launcher, "nvim", logger)
}

func newTestSessionServiceWithBoard(
	repo domain.SessionRepository,
	projects domain.ProjectRepository,
	tmux domain.TmuxAdapter,
	git domain.GitAdapter,
	board domain.SwarmBoardRepository,
	logger *slog.Logger,
) *SessionService {
	launcher, _ := domain.NewLauncher("OpenCode", "opencode", domain.AgentTypeOpenCode)
	return NewSessionService(repo, projects, tmux, git, board, paths.NewResolver(""), launcher, "nvim", logger)
}

// noopBoard satisfies the board port for tests that never exercise a swarm, so
// they neither need board expectations nor risk a nil dereference.
type noopBoard struct{}

func (noopBoard) Append(context.Context, domain.SwarmMessage) (domain.SwarmMessage, error) {
	return domain.SwarmMessage{}, nil
}
func (noopBoard) ListSince(context.Context, uuid.UUID, int) ([]domain.SwarmMessage, error) {
	return nil, nil
}
func (noopBoard) Count(context.Context, uuid.UUID) (int, error) { return 0, nil }
func (noopBoard) Purge(context.Context, uuid.UUID) error        { return nil }

func expectProjectLookup(t *testing.T, projects *mocks.MockProjectRepository, projectID uuid.UUID, name string) string {
	t.Helper()
	repoPath := "/repo/" + name
	project := testutil.MakeProject(repoPath, name)
	project.ID = projectID
	projects.EXPECT().Get(mock.Anything, projectID).Return(project, nil).Once()
	return repoPath
}

func worktreeCreateReq(name string, projectID uuid.UUID, baseBranch string) CreateSessionRequest {
	return CreateSessionRequest{
		Name:           name,
		ProjectID:      projectID,
		CreateWorktree: true,
		BaseBranch:     baseBranch,
	}
}

// expectSwarmAgentTmuxGone stubs the Delete teardown's inspection of a swarm's
// agent panes plus the editor pane when none of them still exist.
func expectSwarmAgentTmuxGone(tmux *mocks.MockTmuxAdapter, sess domain.Session) {
	for _, tmuxID := range sess.AgentTmuxIDs() {
		tmux.EXPECT().GetSession(mock.Anything, tmuxID).
			Return(domain.TmuxSession{}, domain.ErrTmuxSessionNotFound).Once()
	}
	tmux.EXPECT().GetSession(mock.Anything, sess.ID.String()+"-editor").
		Return(domain.TmuxSession{}, domain.ErrTmuxSessionNotFound).Once()
}

func TestSessionService_Create_Swarm_CreatesOneTmuxSessionPerAgent(t *testing.T) {
	overseerID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	board := mocks.NewMockSwarmBoardRepository(t)

	expectProjectLookup(t, projects, overseerID, "overseer")
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.UUIDString(), mock.Anything, "").
		Return("tmux-shell", nil).Once()
	for index := 1; index <= 3; index++ {
		tmux.EXPECT().CreateSession(mock.Anything, testutil.SwarmAgentTmuxIDString(index), mock.Anything, "opencode").
			Return("tmux-agent", nil).Once()
	}

	var saved domain.Session
	repo.EXPECT().Save(mock.Anything, mock.Anything).
		Run(func(_ context.Context, s domain.Session) { saved = s }).
		Return(nil).Once()
	projects.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()

	svc := newTestSessionServiceWithBoard(repo, projects, tmux, git, board, testLogger())
	resp, err := svc.Create(context.Background(), CreateSessionRequest{
		Name:      "hive",
		ProjectID: overseerID,
		SwarmSize: 3,
	})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !resp.Session.IsSwarm() {
		t.Fatal("Create() Session.IsSwarm() = false, want true")
	}
	if resp.Session.SwarmSize != 3 {
		t.Fatalf("Create() Session.SwarmSize = %d, want 3", resp.Session.SwarmSize)
	}
	if saved.SwarmSize != 3 {
		t.Fatalf("persisted SwarmSize = %d, want 3", saved.SwarmSize)
	}
}

func TestSessionService_Create_Swarm_NeverCreatesABareAgentPane(t *testing.T) {
	overseerID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	board := mocks.NewMockSwarmBoardRepository(t)

	expectProjectLookup(t, projects, overseerID, "overseer")
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()

	var createdTmuxIDs []string
	tmux.EXPECT().CreateSession(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, name, _, _ string) { createdTmuxIDs = append(createdTmuxIDs, name) }).
		Return("tmux", nil).Times(3)
	repo.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()
	projects.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()

	svc := newTestSessionServiceWithBoard(repo, projects, tmux, git, board, testLogger())
	resp, err := svc.Create(context.Background(), CreateSessionRequest{
		Name:      "hive",
		ProjectID: overseerID,
		SwarmSize: 2,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	bare := resp.Session.ID.String() + "-agent"
	if slices.Contains(createdTmuxIDs, bare) {
		t.Fatalf("Create() created the bare agent pane %q for a swarm; panes = %v", bare, createdTmuxIDs)
	}
	for index := 1; index <= 2; index++ {
		want := resp.Session.AgentTmuxID(index)
		if !slices.Contains(createdTmuxIDs, want) {
			t.Fatalf("Create() did not create swarm pane %q; panes = %v", want, createdTmuxIDs)
		}
	}
}

func TestSessionService_Create_Swarm_RejectsOutOfRangeSize(t *testing.T) {
	tests := []struct {
		name string
		size int
	}{
		{name: "one is not a swarm", size: 1},
		{name: "above maximum", size: domain.SwarmMaxAgents + 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, projects, tmux, git := newSessionMocks(t)
			board := mocks.NewMockSwarmBoardRepository(t)

			svc := newTestSessionServiceWithBoard(repo, projects, tmux, git, board, testLogger())
			_, err := svc.Create(context.Background(), CreateSessionRequest{
				Name:      "hive",
				ProjectID: uuid.New(),
				SwarmSize: tt.size,
			})

			if !errors.Is(err, domain.ErrSessionSwarmSizeOutOfRange) {
				t.Fatalf("Create() error = %v, want %v", err, domain.ErrSessionSwarmSizeOutOfRange)
			}
		})
	}
}

func TestSessionService_Create_SwarmSizeOne_StaysASingleAgentSession(t *testing.T) {
	// A size of 1 is rejected outright rather than silently downgraded, so the
	// only way to get a single-agent session is to leave SwarmSize at zero.
	overseerID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	board := mocks.NewMockSwarmBoardRepository(t)

	expectProjectLookup(t, projects, overseerID, "overseer")
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.UUIDString(), mock.Anything, "").
		Return("tmux-shell", nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.AgentTmuxIDString(), mock.Anything, "opencode").
		Return("tmux-agent", nil).Once()
	repo.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()
	projects.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()

	svc := newTestSessionServiceWithBoard(repo, projects, tmux, git, board, testLogger())
	resp, err := svc.Create(context.Background(), CreateSessionRequest{
		Name:      "solo",
		ProjectID: overseerID,
	})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.Session.IsSwarm() {
		t.Fatal("Create() with no SwarmSize produced a swarm")
	}
}

func TestSessionService_Delete_Swarm_KillsEveryAgentPaneAndPurgesBoard(t *testing.T) {
	pinWorktreeRoot(t)
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 3)
	repo, projects, tmux, git := newSessionMocks(t)
	board := mocks.NewMockSwarmBoardRepository(t)

	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()

	// Shell, then every swarm agent pane, then the editor.
	for _, tmuxID := range append(
		append([]string{sess.ID.String()}, sess.AgentTmuxIDs()...),
		sess.ID.String()+"-editor",
	) {
		tmux.EXPECT().GetSession(mock.Anything, tmuxID).
			Return(domain.TmuxSession{ID: tmuxID}, nil).Once()
		tmux.EXPECT().KillSession(mock.Anything, tmuxID).Return(nil).Once()
	}

	board.EXPECT().Purge(mock.Anything, sess.ID).Return(nil).Once()
	repo.EXPECT().Delete(mock.Anything, sess.ID).Return(nil).Once()

	svc := newTestSessionServiceWithBoard(repo, projects, tmux, git, board, testLogger())
	if _, err := svc.Delete(context.Background(), DeleteSessionRequest{ID: sess.ID}); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func TestSessionService_Delete_Swarm_BoardPurgeFailureIsNotFatal(t *testing.T) {
	pinWorktreeRoot(t)
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 2)
	repo, projects, tmux, git := newSessionMocks(t)
	board := mocks.NewMockSwarmBoardRepository(t)

	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, sess.ID.String()).
		Return(domain.TmuxSession{}, domain.ErrTmuxSessionNotFound).Once()
	expectSwarmAgentTmuxGone(tmux, sess)
	board.EXPECT().Purge(mock.Anything, sess.ID).Return(errors.New("disk on fire")).Once()
	repo.EXPECT().Delete(mock.Anything, sess.ID).Return(nil).Once()

	svc := newTestSessionServiceWithBoard(repo, projects, tmux, git, board, testLogger())
	if _, err := svc.Delete(context.Background(), DeleteSessionRequest{ID: sess.ID}); err != nil {
		t.Fatalf("Delete() error = %v, want nil — a stale board must not block session teardown", err)
	}
}

func TestSessionService_Delete_NonSwarm_LeavesTheBoardAlone(t *testing.T) {
	pinWorktreeRoot(t)
	sess := testutil.MakeSession("solo", uuid.New())
	repo, projects, tmux, git := newSessionMocks(t)
	board := mocks.NewMockSwarmBoardRepository(t)

	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, sess.ID.String()).
		Return(domain.TmuxSession{}, domain.ErrTmuxSessionNotFound).Once()
	expectAgentAndEditorTmuxGone(tmux, sess.ID.String())
	repo.EXPECT().Delete(mock.Anything, sess.ID).Return(nil).Once()

	// No board.EXPECT().Purge — the mock fails the test if Purge is called.
	svc := newTestSessionServiceWithBoard(repo, projects, tmux, git, board, testLogger())
	if _, err := svc.Delete(context.Background(), DeleteSessionRequest{ID: sess.ID}); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func TestSessionService_AttachAgent_Swarm_TargetsRequestedPane(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 4)
	wantTmuxID := sess.AgentTmuxID(3)
	repo, projects, tmux, git := newSessionMocks(t)
	board := mocks.NewMockSwarmBoardRepository(t)

	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, wantTmuxID).
		Return(domain.TmuxSession{ID: wantTmuxID}, nil).Once()
	tmux.EXPECT().AttachCommand(mock.Anything, wantTmuxID).
		Return(exec.Command("true"), nil).Once()

	svc := newTestSessionServiceWithBoard(repo, projects, tmux, git, board, testLogger())
	resp, err := svc.AttachAgent(context.Background(), AttachAgentRequest{ID: sess.ID, AgentIndex: 3})

	if err != nil {
		t.Fatalf("AttachAgent() error = %v", err)
	}
	if resp.Command == nil {
		t.Fatal("AttachAgent() Command = nil")
	}
}

func TestSessionService_PreviewSession_Swarm_CapturesRequestedAgentPane(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 3)
	wantTmuxID := sess.AgentTmuxID(2)
	repo, projects, tmux, git := newSessionMocks(t)
	board := mocks.NewMockSwarmBoardRepository(t)

	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().CapturePane(mock.Anything, wantTmuxID).Return("agent two output", nil).Once()

	svc := newTestSessionServiceWithBoard(repo, projects, tmux, git, board, testLogger())
	resp, err := svc.PreviewSession(context.Background(), PreviewSessionRequest{
		ID:         sess.ID,
		Kind:       PreviewKindAgent,
		AgentIndex: 2,
	})

	if err != nil {
		t.Fatalf("PreviewSession() error = %v", err)
	}
	if resp.Content != "agent two output" {
		t.Fatalf("PreviewSession() Content = %q", resp.Content)
	}
}

func TestSessionService_KillPreviewSession_Swarm_KillsRequestedAgentPane(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 3)
	wantTmuxID := sess.AgentTmuxID(3)
	repo, projects, tmux, git := newSessionMocks(t)
	board := mocks.NewMockSwarmBoardRepository(t)

	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, wantTmuxID).
		Return(domain.TmuxSession{ID: wantTmuxID}, nil).Once()
	tmux.EXPECT().KillSession(mock.Anything, wantTmuxID).Return(nil).Once()

	svc := newTestSessionServiceWithBoard(repo, projects, tmux, git, board, testLogger())
	_, err := svc.KillPreviewSession(context.Background(), KillPreviewSessionRequest{
		ID:         sess.ID,
		Kind:       PreviewKindAgent,
		AgentIndex: 3,
	})

	if err != nil {
		t.Fatalf("KillPreviewSession() error = %v", err)
	}
}

func TestSessionService_SendAgentPrompt_Swarm_TargetsRequestedPane(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 3)
	wantTmuxID := sess.AgentTmuxID(2)
	repo, projects, tmux, git := newSessionMocks(t)
	board := mocks.NewMockSwarmBoardRepository(t)

	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().SendText(mock.Anything, wantTmuxID, "read the board").Return(nil).Once()
	tmux.EXPECT().SendKeys(mock.Anything, wantTmuxID, "Enter").Return(nil).Once()

	svc := newTestSessionServiceWithBoard(repo, projects, tmux, git, board, testLogger())
	_, err := svc.SendAgentPrompt(context.Background(), SendAgentPromptRequest{
		ID:         sess.ID,
		AgentIndex: 2,
		Prompt:     "read the board",
	})

	if err != nil {
		t.Fatalf("SendAgentPrompt() error = %v", err)
	}
}

func TestSessionService_SendAgentEnter_Swarm_TargetsRequestedPane(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 3)
	wantTmuxID := sess.AgentTmuxID(1)
	repo, projects, tmux, git := newSessionMocks(t)
	board := mocks.NewMockSwarmBoardRepository(t)

	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().SendKeys(mock.Anything, wantTmuxID, "Enter").Return(nil).Once()

	svc := newTestSessionServiceWithBoard(repo, projects, tmux, git, board, testLogger())
	_, err := svc.SendAgentEnter(context.Background(), SendAgentEnterRequest{ID: sess.ID, AgentIndex: 1})

	if err != nil {
		t.Fatalf("SendAgentEnter() error = %v", err)
	}
}

func TestSessionService_SendAgentEnterAll_Swarm_ReachesEveryPane(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 4)
	repo, projects, tmux, git := newSessionMocks(t)
	board := mocks.NewMockSwarmBoardRepository(t)

	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	for _, tmuxID := range sess.AgentTmuxIDs() {
		tmux.EXPECT().SendKeys(mock.Anything, tmuxID, "Enter").Return(nil).Once()
	}

	svc := newTestSessionServiceWithBoard(repo, projects, tmux, git, board, testLogger())
	resp, err := svc.SendAgentEnterAll(context.Background(), SendAgentEnterAllRequest{ID: sess.ID})

	if err != nil {
		t.Fatalf("SendAgentEnterAll() error = %v", err)
	}
	if resp.Delivered != 4 {
		t.Fatalf("SendAgentEnterAll() Delivered = %d, want 4", resp.Delivered)
	}
}

func TestSessionService_SendAgentEnterAll_NonSwarm_ReachesTheSinglePane(t *testing.T) {
	sess := testutil.MakeSession("solo", uuid.New())
	repo, projects, tmux, git := newSessionMocks(t)
	board := mocks.NewMockSwarmBoardRepository(t)

	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().SendKeys(mock.Anything, sess.ID.String()+"-agent", "Enter").Return(nil).Once()

	svc := newTestSessionServiceWithBoard(repo, projects, tmux, git, board, testLogger())
	resp, err := svc.SendAgentEnterAll(context.Background(), SendAgentEnterAllRequest{ID: sess.ID})

	if err != nil {
		t.Fatalf("SendAgentEnterAll() error = %v", err)
	}
	if resp.Delivered != 1 {
		t.Fatalf("SendAgentEnterAll() Delivered = %d, want 1 — /enter must work on ordinary sessions too", resp.Delivered)
	}
}

func TestSessionService_SendAgentEnterAll_DeadPaneDoesNotStopTheOthers(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 3)
	repo, projects, tmux, git := newSessionMocks(t)
	board := mocks.NewMockSwarmBoardRepository(t)

	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().SendKeys(mock.Anything, sess.AgentTmuxID(1), "Enter").
		Return(domain.ErrTmuxSessionNotFound).Once()
	tmux.EXPECT().SendKeys(mock.Anything, sess.AgentTmuxID(2), "Enter").Return(nil).Once()
	tmux.EXPECT().SendKeys(mock.Anything, sess.AgentTmuxID(3), "Enter").Return(nil).Once()

	svc := newTestSessionServiceWithBoard(repo, projects, tmux, git, board, testLogger())
	resp, err := svc.SendAgentEnterAll(context.Background(), SendAgentEnterAllRequest{ID: sess.ID})

	if err != nil {
		t.Fatalf("SendAgentEnterAll() error = %v, want a dead pane to be tolerated", err)
	}
	if resp.Delivered != 2 {
		t.Fatalf("SendAgentEnterAll() Delivered = %d, want 2 (the reachable panes)", resp.Delivered)
	}
}

func TestSessionService_SendAgentEnterAll_UnknownSession(t *testing.T) {
	missingID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	board := mocks.NewMockSwarmBoardRepository(t)
	repo.EXPECT().Get(mock.Anything, missingID).
		Return(domain.Session{}, domain.ErrSessionNotFound).Once()

	svc := newTestSessionServiceWithBoard(repo, projects, tmux, git, board, testLogger())
	_, err := svc.SendAgentEnterAll(context.Background(), SendAgentEnterAllRequest{ID: missingID})

	if !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("SendAgentEnterAll() error = %v, want %v", err, domain.ErrSessionNotFound)
	}
}

func TestSessionService_SendAgentEnter_NonSwarm_IgnoresAgentIndex(t *testing.T) {
	sess := testutil.MakeSession("solo", uuid.New())
	wantTmuxID := sess.ID.String() + "-agent"
	repo, projects, tmux, git := newSessionMocks(t)
	board := mocks.NewMockSwarmBoardRepository(t)

	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().SendKeys(mock.Anything, wantTmuxID, "Enter").Return(nil).Once()

	svc := newTestSessionServiceWithBoard(repo, projects, tmux, git, board, testLogger())
	_, err := svc.SendAgentEnter(context.Background(), SendAgentEnterRequest{ID: sess.ID, AgentIndex: 7})

	if err != nil {
		t.Fatalf("SendAgentEnter() error = %v", err)
	}
}

func TestSessionService_Create_WorktreeMode_HappyPath(t *testing.T) {
	overseerID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)

	repoPath := expectProjectLookup(t, projects, overseerID, "overseer")
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	git.EXPECT().PullBranch(mock.Anything, repoPath, "main").Return(nil).Once()
	git.EXPECT().CreateWorktree(mock.Anything, repoPath, "main", mock.Anything, mock.Anything).Return(nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.UUIDString(), mock.Anything, "").Return("tmux-alpha", nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.AgentTmuxIDString(), mock.Anything, "opencode").Return("tmux-alpha-agent", nil).Once()

	var savedSession domain.Session
	repo.EXPECT().Save(mock.Anything, mock.Anything).
		Run(func(_ context.Context, s domain.Session) { savedSession = s }).
		Return(nil).Once()
	projects.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.Create(context.Background(), worktreeCreateReq("alpha", overseerID, "main"))

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
	if !resp.Session.HasWorktree() {
		t.Fatalf("Create() Session.HasWorktree() = false, want true for worktree-mode session")
	}
	wantBranch := paths.SessionFeatureBranch(resp.Session.ID)
	if resp.Session.Branch != wantBranch {
		t.Fatalf("Create() Session.Branch = %q, want %q", resp.Session.Branch, wantBranch)
	}
	wantPath := paths.NewResolver("").SessionWorktreePath(resp.Session.ID)
	if resp.Session.WorktreePath != wantPath {
		t.Fatalf("Create() Session.WorktreePath = %q, want %q", resp.Session.WorktreePath, wantPath)
	}
	if savedSession.WorktreePath != wantPath || savedSession.Branch != wantBranch {
		t.Fatalf("SessionRepository.Save session = %#v, want worktree+branch populated", savedSession)
	}
}

func TestSessionService_Create_ProjectMode_SkipsGit_PersistsWithoutWorktree(t *testing.T) {
	overseerID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)

	repoPath := expectProjectLookup(t, projects, overseerID, "overseer")
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.UUIDString(), repoPath, "").Return("tmux-alpha", nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.AgentTmuxIDString(), repoPath, "opencode").Return("tmux-alpha-agent", nil).Once()

	var savedSession domain.Session
	repo.EXPECT().Save(mock.Anything, mock.Anything).
		Run(func(_ context.Context, s domain.Session) { savedSession = s }).
		Return(nil).Once()
	projects.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.Create(context.Background(), CreateSessionRequest{
		Name:           "alpha",
		ProjectID:      overseerID,
		CreateWorktree: false,
	})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.Session.HasWorktree() {
		t.Fatalf("Create() HasWorktree() = true, want false for project-mode session")
	}
	if resp.Session.Branch != "" {
		t.Fatalf("Create() Session.Branch = %q, want empty for project-mode", resp.Session.Branch)
	}
	if resp.Session.WorktreePath != "" {
		t.Fatalf("Create() Session.WorktreePath = %q, want empty for project-mode", resp.Session.WorktreePath)
	}
	if savedSession.HasWorktree() {
		t.Fatalf("saved session HasWorktree() = true, want false")
	}
}

func TestSessionService_Create_EmptyName(t *testing.T) {
	repo, projects, tmux, git := newSessionMocks(t)
	svc := newTestSessionService(repo, projects, tmux, git, testLogger())

	_, err := svc.Create(context.Background(), worktreeCreateReq("", uuid.New(), "main"))

	if !errors.Is(err, domain.ErrSessionEmptyName) {
		t.Fatalf("Create() error = %v, want %v", err, domain.ErrSessionEmptyName)
	}
}

func TestSessionService_Create_WorktreeMode_EmptyBaseBranch_ResolvesProjectDefault(t *testing.T) {
	projID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	repoPath := expectProjectLookup(t, projects, projID, "overseer")
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	git.EXPECT().GetDefaultBranch(mock.Anything, repoPath).Return("trunk", nil).Once()
	git.EXPECT().PullBranch(mock.Anything, repoPath, "trunk").Return(nil).Once()
	git.EXPECT().CreateWorktree(mock.Anything, repoPath, "trunk", mock.Anything, mock.Anything).Return(nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.UUIDString(), mock.Anything, "").Return("tmux-alpha", nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.AgentTmuxIDString(), mock.Anything, "opencode").Return("tmux-alpha-agent", nil).Once()
	repo.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()
	projects.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.Create(context.Background(), CreateSessionRequest{Name: "alpha", ProjectID: projID, CreateWorktree: true})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestSessionService_Create_WorktreeMode_DefaultBranchResolveError(t *testing.T) {
	projID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	repoPath := expectProjectLookup(t, projects, projID, "overseer")
	gitErr := errors.New("no remote configured")
	git.EXPECT().GetDefaultBranch(mock.Anything, repoPath).Return("", gitErr).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.Create(context.Background(), CreateSessionRequest{Name: "alpha", ProjectID: projID, CreateWorktree: true})

	if !errors.Is(err, gitErr) {
		t.Fatalf("Create() error = %v, want wrapped %v", err, gitErr)
	}
}

func TestSessionService_Create_EmptyProjectID(t *testing.T) {
	repo, projects, tmux, git := newSessionMocks(t)
	svc := newTestSessionService(repo, projects, tmux, git, testLogger())

	_, err := svc.Create(context.Background(), worktreeCreateReq("alpha", uuid.Nil, "main"))

	if !errors.Is(err, domain.ErrSessionEmptyProjectID) {
		t.Fatalf("Create() error = %v, want %v", err, domain.ErrSessionEmptyProjectID)
	}
}

func TestSessionService_Create_WorktreeMode_UsesUserProvidedBranch(t *testing.T) {
	projID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	repoPath := expectProjectLookup(t, projects, projID, "overseer")
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	git.EXPECT().PullBranch(mock.Anything, repoPath, "main").Return(nil).Once()
	git.EXPECT().CreateWorktree(mock.Anything, repoPath, "main", "my-feature", mock.Anything).Return(nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.UUIDString(), mock.Anything, "").Return("tmux-alpha", nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.AgentTmuxIDString(), mock.Anything, "opencode").Return("tmux-alpha-agent", nil).Once()

	var savedSession domain.Session
	repo.EXPECT().Save(mock.Anything, mock.Anything).
		Run(func(_ context.Context, s domain.Session) { savedSession = s }).
		Return(nil).Once()
	projects.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.Create(context.Background(), CreateSessionRequest{
		Name:           "alpha",
		ProjectID:      projID,
		CreateWorktree: true,
		BaseBranch:     "main",
		Branch:         "my-feature",
	})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.Session.Branch != "my-feature" {
		t.Fatalf("Create() Session.Branch = %q, want %q", resp.Session.Branch, "my-feature")
	}
	if savedSession.Branch != "my-feature" {
		t.Fatalf("SessionRepository.Save Branch = %q, want %q", savedSession.Branch, "my-feature")
	}
}

func TestSessionService_Create_WorktreeMode_BlankBranchGeneratesDefault(t *testing.T) {
	projID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	repoPath := expectProjectLookup(t, projects, projID, "overseer")
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	git.EXPECT().PullBranch(mock.Anything, repoPath, "main").Return(nil).Once()
	git.EXPECT().CreateWorktree(mock.Anything, repoPath, "main", mock.Anything, mock.Anything).Return(nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.UUIDString(), mock.Anything, "").Return("tmux-alpha", nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.AgentTmuxIDString(), mock.Anything, "opencode").Return("tmux-alpha-agent", nil).Once()
	repo.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()
	projects.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.Create(context.Background(), CreateSessionRequest{
		Name:           "alpha",
		ProjectID:      projID,
		CreateWorktree: true,
		BaseBranch:     "main",
		Branch:         "   ",
	})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	want := paths.SessionFeatureBranch(resp.Session.ID)
	if resp.Session.Branch != want {
		t.Fatalf("Create() Session.Branch = %q, want generated default %q", resp.Session.Branch, want)
	}
}

func TestSessionService_Create_DuplicateNameWithinSameProject(t *testing.T) {
	overseerID := uuid.New()
	existing := testutil.MakeSession("alpha", overseerID)
	repo, projects, tmux, git := newSessionMocks(t)
	expectProjectLookup(t, projects, overseerID, "overseer")
	repo.EXPECT().List(mock.Anything).Return([]domain.Session{existing}, nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.Create(context.Background(), worktreeCreateReq("alpha", overseerID, "main"))

	if !errors.Is(err, domain.ErrSessionAlreadyExists) {
		t.Fatalf("Create() error = %v, want %v", err, domain.ErrSessionAlreadyExists)
	}
}

func TestSessionService_Create_DuplicateNameAcrossProjectsAllowed(t *testing.T) {
	overseerID := uuid.New()
	otherID := uuid.New()
	existing := testutil.MakeSession("alpha", otherID)
	repo, projects, tmux, git := newSessionMocks(t)
	repoPath := expectProjectLookup(t, projects, overseerID, "overseer")
	repo.EXPECT().List(mock.Anything).Return([]domain.Session{existing}, nil).Once()
	git.EXPECT().PullBranch(mock.Anything, repoPath, "main").Return(nil).Once()
	git.EXPECT().CreateWorktree(mock.Anything, repoPath, "main", mock.Anything, mock.Anything).Return(nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.UUIDString(), mock.Anything, "").Return("tmux-alpha", nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.AgentTmuxIDString(), mock.Anything, "opencode").Return("tmux-alpha-agent", nil).Once()
	repo.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()
	projects.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.Create(context.Background(), worktreeCreateReq("alpha", overseerID, "main"))

	if err != nil {
		t.Fatalf("Create() error = %v, want nil (same name in different project is allowed)", err)
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
	repo, projects, tmux, git := newSessionMocks(t)
	repoPath := expectProjectLookup(t, projects, overseerID, "overseer")
	repo.EXPECT().List(mock.Anything).
		Return([]domain.Session{first, second, otherProject}, nil).Once()
	git.EXPECT().PullBranch(mock.Anything, repoPath, "main").Return(nil).Once()
	git.EXPECT().CreateWorktree(mock.Anything, repoPath, "main", mock.Anything, mock.Anything).Return(nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.UUIDString(), mock.Anything, "").Return("tmux-gamma", nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.AgentTmuxIDString(), mock.Anything, "opencode").Return("tmux-gamma-agent", nil).Once()

	var savedSession domain.Session
	repo.EXPECT().Save(mock.Anything, mock.Anything).
		Run(func(_ context.Context, s domain.Session) { savedSession = s }).
		Return(nil).Once()
	projects.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.Create(context.Background(), worktreeCreateReq("gamma", overseerID, "main"))

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.Session.Order != 3 {
		t.Fatalf("Create() Session.Order = %d, want 3", resp.Session.Order)
	}
	if savedSession.Order != 3 {
		t.Fatalf("SessionRepository.Save Order = %d, want 3", savedSession.Order)
	}
}

func TestSessionService_Create_AgentTmuxErrorKillsShellAndPropagates(t *testing.T) {
	tmuxErr := errors.New("tmux out of capacity")
	projID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	repoPath := expectProjectLookup(t, projects, projID, "overseer")
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	git.EXPECT().PullBranch(mock.Anything, repoPath, "main").Return(nil).Once()
	git.EXPECT().CreateWorktree(mock.Anything, repoPath, "main", mock.Anything, mock.Anything).Return(nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.UUIDString(), mock.Anything, "").
		Return("tmux-alpha", nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.AgentTmuxIDString(), mock.Anything, "opencode").
		Return("", tmuxErr).Once()
	tmux.EXPECT().KillSession(mock.Anything, testutil.UUIDString()).Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.Create(context.Background(), worktreeCreateReq("alpha", projID, "main"))

	if err == nil || !errors.Is(err, tmuxErr) {
		t.Fatalf("Create() error = %v, want wrapped %v", err, tmuxErr)
	}
}

func TestSessionService_Create_TmuxError(t *testing.T) {
	tmuxErr := errors.New("tmux unavailable")
	projID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	repoPath := expectProjectLookup(t, projects, projID, "overseer")
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	git.EXPECT().PullBranch(mock.Anything, repoPath, "main").Return(nil).Once()
	git.EXPECT().CreateWorktree(mock.Anything, repoPath, "main", mock.Anything, mock.Anything).Return(nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.UUIDString(), mock.Anything, "").Return("", tmuxErr).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.Create(context.Background(), worktreeCreateReq("alpha", projID, "main"))

	if !errors.Is(err, tmuxErr) {
		t.Fatalf("Create() error = %v, want wrapped %v", err, tmuxErr)
	}
}

func TestSessionService_Create_GitError(t *testing.T) {
	gitErr := errors.New("git refused")
	projID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	repoPath := expectProjectLookup(t, projects, projID, "overseer")
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	git.EXPECT().PullBranch(mock.Anything, repoPath, "main").Return(nil).Once()
	git.EXPECT().CreateWorktree(mock.Anything, repoPath, "main", mock.Anything, mock.Anything).Return(gitErr).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.Create(context.Background(), worktreeCreateReq("alpha", projID, "main"))

	if !errors.Is(err, gitErr) {
		t.Fatalf("Create() error = %v, want wrapped %v", err, gitErr)
	}
}

func TestSessionService_Create_WorktreeMode_PullFails_ContinuesWithWorktreeCreation(t *testing.T) {
	projID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	repoPath := expectProjectLookup(t, projects, projID, "overseer")
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	git.EXPECT().PullBranch(mock.Anything, repoPath, "main").Return(errors.New("no remote configured")).Once()
	git.EXPECT().CreateWorktree(mock.Anything, repoPath, "main", mock.Anything, mock.Anything).Return(nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.UUIDString(), mock.Anything, "").Return("tmux-alpha", nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.AgentTmuxIDString(), mock.Anything, "opencode").Return("tmux-alpha-agent", nil).Once()
	repo.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()
	projects.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.Create(context.Background(), worktreeCreateReq("alpha", projID, "main"))

	if err != nil {
		t.Fatalf("Create() error = %v, want nil (pull failure is best-effort)", err)
	}
}

func TestSessionService_Create_ProjectLookupErrorBubblesUp(t *testing.T) {
	lookupErr := errors.New("project lookup failed")
	projID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	projects.EXPECT().Get(mock.Anything, projID).Return(domain.Project{}, lookupErr).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.Create(context.Background(), worktreeCreateReq("alpha", projID, "main"))

	if !errors.Is(err, lookupErr) {
		t.Fatalf("Create() error = %v, want wrapped %v", err, lookupErr)
	}
}

func TestSessionService_Create_WithAgentCommand(t *testing.T) {
	projID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	repoPath := expectProjectLookup(t, projects, projID, "overseer")
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	git.EXPECT().PullBranch(mock.Anything, repoPath, "main").Return(nil).Once()
	git.EXPECT().CreateWorktree(mock.Anything, repoPath, "main", mock.Anything, mock.Anything).Return(nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.UUIDString(), mock.Anything, "").
		Return("tmux-alpha", nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.AgentTmuxIDString(), mock.Anything, "claude").
		Return("tmux-alpha-agent", nil).Once()

	var savedSession domain.Session
	repo.EXPECT().Save(mock.Anything, mock.Anything).
		Run(func(_ context.Context, s domain.Session) { savedSession = s }).
		Return(nil).Once()
	projects.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.Create(context.Background(), CreateSessionRequest{
		Name:           "alpha",
		ProjectID:      projID,
		CreateWorktree: true,
		BaseBranch:     "main",
		AgentCommand:   "claude",
	})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.Session.AgentCommand != "claude" {
		t.Fatalf("Create() Session.AgentCommand = %q, want %q", resp.Session.AgentCommand, "claude")
	}
	if savedSession.AgentCommand != "claude" {
		t.Fatalf("SessionRepository.Save session.AgentCommand = %q, want %q", savedSession.AgentCommand, "claude")
	}
}

func TestSessionService_Create_RejectsInvalidAgentCommand(t *testing.T) {
	repo, projects, tmux, git := newSessionMocks(t)
	svc := newTestSessionService(repo, projects, tmux, git, testLogger())

	_, err := svc.Create(context.Background(), CreateSessionRequest{
		Name:           "alpha",
		ProjectID:      uuid.New(),
		CreateWorktree: true,
		BaseBranch:     "main",
		AgentCommand:   "   ",
	})

	if !errors.Is(err, domain.ErrSessionEmptyAgentCommand) {
		t.Fatalf("Create() error = %v, want %v", err, domain.ErrSessionEmptyAgentCommand)
	}
}

func TestCreateSession_CapturesAgentTypeFromLauncher(t *testing.T) {
	projID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	repoPath := expectProjectLookup(t, projects, projID, "overseer")
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	git.EXPECT().PullBranch(mock.Anything, repoPath, "main").Return(nil).Once()
	git.EXPECT().CreateWorktree(mock.Anything, repoPath, "main", mock.Anything, mock.Anything).Return(nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.UUIDString(), mock.Anything, "").
		Return("tmux-alpha", nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.AgentTmuxIDString(), mock.Anything, "claude").
		Return("tmux-alpha-agent", nil).Once()

	var savedSession domain.Session
	repo.EXPECT().Save(mock.Anything, mock.Anything).
		Run(func(_ context.Context, s domain.Session) { savedSession = s }).
		Return(nil).Once()
	projects.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.Create(context.Background(), CreateSessionRequest{
		Name:           "alpha",
		ProjectID:      projID,
		CreateWorktree: true,
		BaseBranch:     "main",
		AgentCommand:   "claude",
		AgentType:      domain.AgentTypeClaudeCode,
	})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if resp.Session.AgentType != domain.AgentTypeClaudeCode {
		t.Fatalf("Create() Session.AgentType = %q, want %q", resp.Session.AgentType, domain.AgentTypeClaudeCode)
	}
	if savedSession.AgentType != domain.AgentTypeClaudeCode {
		t.Fatalf("SessionRepository.Save session.AgentType = %q, want %q", savedSession.AgentType, domain.AgentTypeClaudeCode)
	}
}

func TestSessionService_Rename_HappyPath(t *testing.T) {
	original := testutil.MakeSession("alpha", uuid.New())
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, original.ID).Return(original, nil).Once()
	repo.EXPECT().List(mock.Anything).Return([]domain.Session{original}, nil).Once()

	var savedSession domain.Session
	repo.EXPECT().Save(mock.Anything, mock.Anything).
		Run(func(_ context.Context, s domain.Session) { savedSession = s }).
		Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.Rename(context.Background(), RenameSessionRequest{ID: original.ID, NewName: "beta"})

	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if resp.Session.Name != "beta" {
		t.Fatalf("Rename() Session.Name = %q, want %q", resp.Session.Name, "beta")
	}
	if savedSession.Name != "beta" {
		t.Fatalf("SessionRepository.Save Session.Name = %q, want %q", savedSession.Name, "beta")
	}
}

func TestSessionService_Rename_EmptyName(t *testing.T) {
	original := testutil.MakeSession("alpha", uuid.New())
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, original.ID).Return(original, nil).Once()
	repo.EXPECT().List(mock.Anything).Return([]domain.Session{original}, nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.Rename(context.Background(), RenameSessionRequest{ID: original.ID, NewName: ""})

	if !errors.Is(err, domain.ErrSessionEmptyName) {
		t.Fatalf("Rename() error = %v, want %v", err, domain.ErrSessionEmptyName)
	}
}

func TestSessionService_Rename_NotFound(t *testing.T) {
	repo, projects, tmux, git := newSessionMocks(t)
	missingID := uuid.New()
	repo.EXPECT().Get(mock.Anything, missingID).
		Return(domain.Session{}, domain.ErrSessionNotFound).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.Rename(context.Background(), RenameSessionRequest{ID: missingID, NewName: "beta"})

	if !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("Rename() error = %v, want %v", err, domain.ErrSessionNotFound)
	}
}

func TestSessionService_Rename_DuplicateNameInSameProject(t *testing.T) {
	overseerID := uuid.New()
	original := testutil.MakeSession("alpha", overseerID)
	conflicting := testutil.MakeSession("beta", overseerID)
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, original.ID).Return(original, nil).Once()
	repo.EXPECT().List(mock.Anything).
		Return([]domain.Session{original, conflicting}, nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.Rename(context.Background(), RenameSessionRequest{ID: original.ID, NewName: "beta"})

	if !errors.Is(err, domain.ErrSessionAlreadyExists) {
		t.Fatalf("Rename() error = %v, want %v", err, domain.ErrSessionAlreadyExists)
	}
}

func TestSessionService_List_Empty(t *testing.T) {
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.List(context.Background(), ListSessionsRequest{})

	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(resp.Sessions) != 0 {
		t.Fatalf("List() len(Sessions) = %d, want 0", len(resp.Sessions))
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
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().List(mock.Anything).Return([]domain.Session{s1, s2, s3}, nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
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
}

func TestSessionService_ListBranches_HappyPath(t *testing.T) {
	overseerID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	repoPath := expectProjectLookup(t, projects, overseerID, "overseer")

	when := time.Now()
	branches := []domain.BranchInfo{
		{Name: "main", Scope: domain.BranchScopeLocal, CommitterDate: when},
		{Name: "origin/feat/x", Scope: domain.BranchScopeRemote, CommitterDate: when.Add(-time.Hour)},
	}
	git.EXPECT().ListBranches(mock.Anything, repoPath).Return(branches, nil).Once()
	git.EXPECT().GetDefaultBranch(mock.Anything, repoPath).Return("main", nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.ListBranches(context.Background(), ListBranchesRequest{ProjectID: overseerID})

	if err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}
	if len(resp.Branches) != 2 {
		t.Fatalf("ListBranches() len = %d, want 2", len(resp.Branches))
	}
	if resp.Branches[0].Name != "main" {
		t.Fatalf("ListBranches()[0].Name = %q, want %q", resp.Branches[0].Name, "main")
	}
	if resp.DefaultBranch != "main" {
		t.Fatalf("ListBranches() DefaultBranch = %q, want %q", resp.DefaultBranch, "main")
	}
}

func TestSessionService_ListBranches_DefaultBranchResolveError_NotFatal(t *testing.T) {
	overseerID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	repoPath := expectProjectLookup(t, projects, overseerID, "overseer")
	git.EXPECT().ListBranches(mock.Anything, repoPath).Return(nil, nil).Once()
	git.EXPECT().GetDefaultBranch(mock.Anything, repoPath).Return("", errors.New("no default")).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.ListBranches(context.Background(), ListBranchesRequest{ProjectID: overseerID})

	if err != nil {
		t.Fatalf("ListBranches() error = %v, want nil (default-branch error is non-fatal)", err)
	}
	if resp.DefaultBranch != "" {
		t.Fatalf("ListBranches() DefaultBranch = %q, want empty on resolve error", resp.DefaultBranch)
	}
}

func TestSessionService_ListBranches_ProjectLookupErrorBubblesUp(t *testing.T) {
	overseerID := uuid.New()
	lookupErr := errors.New("project store down")
	repo, projects, tmux, git := newSessionMocks(t)
	projects.EXPECT().Get(mock.Anything, overseerID).Return(domain.Project{}, lookupErr).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.ListBranches(context.Background(), ListBranchesRequest{ProjectID: overseerID})

	if !errors.Is(err, lookupErr) {
		t.Fatalf("ListBranches() error = %v, want wrapped %v", err, lookupErr)
	}
}

func TestSessionService_ListBranches_GitErrorBubblesUp(t *testing.T) {
	overseerID := uuid.New()
	gitErr := errors.New("git refused")
	repo, projects, tmux, git := newSessionMocks(t)
	repoPath := expectProjectLookup(t, projects, overseerID, "overseer")
	git.EXPECT().ListBranches(mock.Anything, repoPath).Return(nil, gitErr).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.ListBranches(context.Background(), ListBranchesRequest{ProjectID: overseerID})

	if !errors.Is(err, gitErr) {
		t.Fatalf("ListBranches() error = %v, want wrapped %v", err, gitErr)
	}
}

func TestSessionService_ProjectCurrentBranch_HappyPath(t *testing.T) {
	overseerID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	repoPath := expectProjectLookup(t, projects, overseerID, "overseer")
	git.EXPECT().CurrentBranch(mock.Anything, repoPath).Return("feat/foo", nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.ProjectCurrentBranch(context.Background(), ProjectCurrentBranchRequest{ProjectID: overseerID})

	if err != nil {
		t.Fatalf("ProjectCurrentBranch() error = %v", err)
	}
	if resp.Branch != "feat/foo" {
		t.Fatalf("ProjectCurrentBranch() Branch = %q, want %q", resp.Branch, "feat/foo")
	}
}

func TestSessionService_Reorder_MoveDown(t *testing.T) {
	projectID := uuid.New()
	a := testutil.MakeSession("A", projectID)
	a.Order = 1
	b := testutil.MakeSession("B", projectID)
	b.Order = 2
	c := testutil.MakeSession("C", projectID)
	c.Order = 3

	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, b.ID).Return(b, nil).Once()
	repo.EXPECT().List(mock.Anything).Return([]domain.Session{a, b, c}, nil).Once()
	repo.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Twice()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.Reorder(context.Background(), ReorderSessionRequest{ID: b.ID, Direction: 1})

	if err != nil {
		t.Fatalf("Reorder() error = %v", err)
	}
	assertSessionOrder(t, resp.Sessions, "A", 1)
	assertSessionOrder(t, resp.Sessions, "C", 2)
	assertSessionOrder(t, resp.Sessions, "B", 3)
}

func TestSessionService_Reorder_BoundaryFirst_Up(t *testing.T) {
	projectID := uuid.New()
	a := testutil.MakeSession("A", projectID)
	a.Order = 1
	b := testutil.MakeSession("B", projectID)
	b.Order = 2

	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, a.ID).Return(a, nil).Once()
	repo.EXPECT().List(mock.Anything).Return([]domain.Session{a, b}, nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.Reorder(context.Background(), ReorderSessionRequest{ID: a.ID, Direction: -1})

	if !errors.Is(err, errs.ErrNoOp) {
		t.Fatalf("Reorder() error = %v, want %v", err, errs.ErrNoOp)
	}
}

func TestSessionService_Reorder_SingleSession(t *testing.T) {
	a := testutil.MakeSession("A", uuid.New())
	a.Order = 1

	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, a.ID).Return(a, nil).Once()
	repo.EXPECT().List(mock.Anything).Return([]domain.Session{a}, nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.Reorder(context.Background(), ReorderSessionRequest{ID: a.ID, Direction: 1})

	if !errors.Is(err, errs.ErrNoOp) {
		t.Fatalf("Reorder() error = %v, want %v", err, errs.ErrNoOp)
	}
}

func TestSessionService_Reorder_NotFound(t *testing.T) {
	repo, projects, tmux, git := newSessionMocks(t)
	missingID := uuid.New()
	repo.EXPECT().Get(mock.Anything, missingID).
		Return(domain.Session{}, domain.ErrSessionNotFound).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.Reorder(context.Background(), ReorderSessionRequest{ID: missingID, Direction: 1})

	if !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("Reorder() error = %v, want %v", err, domain.ErrSessionNotFound)
	}
}

func TestSessionService_Reorder_UpdatedAt_UsesNowNotNeighbour(t *testing.T) {
	projectID := uuid.New()
	oldTime := time.Now().Add(-1 * time.Hour)

	a := testutil.MakeSession("A", projectID)
	a.Order = 1
	a.UpdatedAt = oldTime

	b := testutil.MakeSession("B", projectID)
	b.Order = 2
	b.UpdatedAt = oldTime

	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, b.ID).Return(b, nil).Once()
	repo.EXPECT().List(mock.Anything).Return([]domain.Session{a, b}, nil).Once()

	before := time.Now()
	repo.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Twice()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.Reorder(context.Background(), ReorderSessionRequest{ID: b.ID, Direction: -1})
	if err != nil {
		t.Fatalf("Reorder() error = %v", err)
	}

	for _, sess := range resp.Sessions {
		if sess.ID == b.ID {
			if !sess.UpdatedAt.After(before) && !sess.UpdatedAt.Equal(before) {
				t.Errorf("moved session UpdatedAt = %v, want >= %v (time.Now() at call time)", sess.UpdatedAt, before)
			}
			return
		}
	}
	t.Fatal("moved session B not found in response")
}

func TestSessionService_AttachShell_HappyPath(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, sess.ID.String()).
		Return(domain.TmuxSession{ID: sess.ID.String()}, nil).Once()
	wantCmd := exec.Command("tmux", "attach-session", "-t", sess.ID.String())
	tmux.EXPECT().AttachCommand(mock.Anything, sess.ID.String()).Return(wantCmd, nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.AttachShell(context.Background(), AttachShellRequest{ID: sess.ID})

	if err != nil {
		t.Fatalf("AttachShell() error = %v", err)
	}
	if resp.Command != wantCmd {
		t.Fatalf("AttachShell() Command = %v, want %v", resp.Command, wantCmd)
	}
}

func TestSessionService_AttachShell_SessionNotFound(t *testing.T) {
	missingID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, missingID).
		Return(domain.Session{}, domain.ErrSessionNotFound).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.AttachShell(context.Background(), AttachShellRequest{ID: missingID})

	if !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("AttachShell() error = %v, want %v", err, domain.ErrSessionNotFound)
	}
}

func TestSessionService_AttachShell_TmuxSessionMissing_RecreatesAtWorktreePath(t *testing.T) {
	worktreePath := "/abs/worktree/alpha"
	sess := testutil.MakeSessionWithWorktree("alpha", uuid.New(), worktreePath, "overseer/alpha")
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, sess.ID.String()).
		Return(domain.TmuxSession{}, domain.ErrTmuxSessionNotFound).Once()
	tmux.EXPECT().CreateSession(mock.Anything, sess.ID.String(), worktreePath, "").
		Return(sess.ID.String(), nil).Once()
	wantCmd := exec.Command("tmux", "attach-session", "-t", sess.ID.String())
	tmux.EXPECT().AttachCommand(mock.Anything, sess.ID.String()).Return(wantCmd, nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.AttachShell(context.Background(), AttachShellRequest{ID: sess.ID})

	if err != nil {
		t.Fatalf("AttachShell() error = %v, want nil", err)
	}
	if resp.Command != wantCmd {
		t.Fatalf("AttachShell() Command = %v, want %v", resp.Command, wantCmd)
	}
}

func TestSessionService_AttachShell_ProjectMode_TmuxMissing_RecreatesAtProjectPath(t *testing.T) {
	overseerID := uuid.New()
	sess := testutil.MakeSession("alpha", overseerID)
	project := testutil.MakeProject("/repo/overseer", "overseer")
	project.ID = overseerID

	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, sess.ID.String()).
		Return(domain.TmuxSession{}, domain.ErrTmuxSessionNotFound).Once()
	projects.EXPECT().Get(mock.Anything, overseerID).Return(project, nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, sess.ID.String(), project.Path, "").
		Return(sess.ID.String(), nil).Once()
	wantCmd := exec.Command("tmux", "attach-session", "-t", sess.ID.String())
	tmux.EXPECT().AttachCommand(mock.Anything, sess.ID.String()).Return(wantCmd, nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.AttachShell(context.Background(), AttachShellRequest{ID: sess.ID})
	if err != nil {
		t.Fatalf("AttachShell() error = %v, want nil for project-mode recreate", err)
	}
}

func TestSessionService_AttachAgent_HappyPath(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	if err := sess.AssignAgentCommand("opencode"); err != nil {
		t.Fatalf("seed AssignAgentCommand: %v", err)
	}
	agentTmuxID := sess.ID.String() + "-agent"
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, agentTmuxID).
		Return(domain.TmuxSession{ID: agentTmuxID}, nil).Once()
	wantCmd := exec.Command("tmux", "attach-session", "-t", agentTmuxID)
	tmux.EXPECT().AttachCommand(mock.Anything, agentTmuxID).Return(wantCmd, nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.AttachAgent(context.Background(), AttachAgentRequest{ID: sess.ID})

	if err != nil {
		t.Fatalf("AttachAgent() error = %v", err)
	}
	if resp.Command != wantCmd {
		t.Fatalf("AttachAgent() Command = %v, want %v", resp.Command, wantCmd)
	}
}

func TestSessionService_AttachAgent_AgentTmuxMissing_RecreatesWithCommand(t *testing.T) {
	worktreePath := "/abs/worktree/alpha"
	sess := testutil.MakeSessionWithWorktree("alpha", uuid.New(), worktreePath, "overseer/alpha")
	if err := sess.AssignAgentCommand("opencode --config /tmp/cfg"); err != nil {
		t.Fatalf("seed AssignAgentCommand: %v", err)
	}
	agentTmuxID := sess.ID.String() + "-agent"
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, agentTmuxID).
		Return(domain.TmuxSession{}, domain.ErrTmuxSessionNotFound).Once()
	tmux.EXPECT().CreateSession(mock.Anything, agentTmuxID, worktreePath, "opencode --config /tmp/cfg").
		Return(agentTmuxID, nil).Once()
	wantCmd := exec.Command("tmux", "attach-session", "-t", agentTmuxID)
	tmux.EXPECT().AttachCommand(mock.Anything, agentTmuxID).Return(wantCmd, nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.AttachAgent(context.Background(), AttachAgentRequest{ID: sess.ID})

	if err != nil {
		t.Fatalf("AttachAgent() error = %v, want nil after recreate", err)
	}
	if resp.Command != wantCmd {
		t.Fatalf("AttachAgent() Command = %v, want %v", resp.Command, wantCmd)
	}
}

func TestSessionService_AttachAgent_NoSessionCommandAndNoDefaultLauncher_ReturnsSentinel(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()

	svc := NewSessionService(repo, projects, tmux, git, noopBoard{}, paths.NewResolver(""), domain.Launcher{}, "nvim", testLogger())
	_, err := svc.AttachAgent(context.Background(), AttachAgentRequest{ID: sess.ID})

	if !errors.Is(err, domain.ErrSessionNoAgentCommandAvailable) {
		t.Fatalf("AttachAgent() error = %v, want ErrSessionNoAgentCommandAvailable", err)
	}
}

func TestSessionService_AttachAgent_SessionNotFound(t *testing.T) {
	missingID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, missingID).
		Return(domain.Session{}, domain.ErrSessionNotFound).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.AttachAgent(context.Background(), AttachAgentRequest{ID: missingID})

	if !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("AttachAgent() error = %v, want %v", err, domain.ErrSessionNotFound)
	}
}

func TestSessionService_PreviewSession_Shell_ReturnsContent(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().CapturePane(mock.Anything, sess.ID.String()).Return("shell content", nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.PreviewSession(context.Background(), PreviewSessionRequest{ID: sess.ID, Kind: PreviewKindShell})

	if err != nil {
		t.Fatalf("PreviewSession() error = %v", err)
	}
	if !resp.SessionReady {
		t.Errorf("PreviewSession() SessionReady = false, want true")
	}
	if resp.Content != "shell content" {
		t.Errorf("PreviewSession() Content = %q, want %q", resp.Content, "shell content")
	}
}

func TestSessionService_PreviewSession_TmuxSessionMissing_ReturnsNotReady(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().CapturePane(mock.Anything, sess.ID.String()+"-agent").
		Return("", domain.ErrTmuxSessionNotFound).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.PreviewSession(context.Background(), PreviewSessionRequest{ID: sess.ID, Kind: PreviewKindAgent})

	if err != nil {
		t.Fatalf("PreviewSession() error = %v, want nil for not-found", err)
	}
	if resp.SessionReady {
		t.Errorf("PreviewSession() SessionReady = true, want false")
	}
}

func TestSessionService_KillPreviewSession_Shell(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, sess.ID.String()).
		Return(domain.TmuxSession{ID: sess.ID.String()}, nil).Once()
	tmux.EXPECT().KillSession(mock.Anything, sess.ID.String()).Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.KillPreviewSession(context.Background(), KillPreviewSessionRequest{ID: sess.ID, Kind: PreviewKindShell})

	if err != nil {
		t.Fatalf("KillPreviewSession() error = %v, want nil", err)
	}
}

func TestSessionService_KillPreviewSession_Agent(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, sess.ID.String()+"-agent").
		Return(domain.TmuxSession{ID: sess.ID.String() + "-agent"}, nil).Once()
	tmux.EXPECT().KillSession(mock.Anything, sess.ID.String()+"-agent").Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.KillPreviewSession(context.Background(), KillPreviewSessionRequest{ID: sess.ID, Kind: PreviewKindAgent})

	if err != nil {
		t.Fatalf("KillPreviewSession() error = %v, want nil", err)
	}
}

func TestSessionService_KillPreviewSession_AlreadyGone(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, sess.ID.String()).
		Return(domain.TmuxSession{}, domain.ErrTmuxSessionNotFound).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.KillPreviewSession(context.Background(), KillPreviewSessionRequest{ID: sess.ID, Kind: PreviewKindShell})

	if err != nil {
		t.Fatalf("KillPreviewSession() error = %v, want nil for already-gone session", err)
	}
}

func TestSessionService_KillPreviewSession_SessionNotFound(t *testing.T) {
	missingID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, missingID).
		Return(domain.Session{}, domain.ErrSessionNotFound).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.KillPreviewSession(context.Background(), KillPreviewSessionRequest{ID: missingID, Kind: PreviewKindShell})

	if !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("KillPreviewSession() error = %v, want %v", err, domain.ErrSessionNotFound)
	}
}

func pinWorktreeRoot(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	return paths.NewResolver("").WorktreeRoot()
}

func TestSessionService_Delete_HappyPath_WorktreeSession(t *testing.T) {
	pinWorktreeRoot(t)
	overseerID := uuid.New()
	sess := testutil.MakeSessionWithWorktree(
		"alpha",
		overseerID,
		paths.NewResolver("").SessionWorktreePath(uuid.New()),
		"overseer/alpha",
	)
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	project := testutil.MakeProject("/repo/overseer", "overseer")
	project.ID = overseerID
	projects.EXPECT().Get(mock.Anything, overseerID).Return(project, nil).Once()
	git.EXPECT().RemoveWorktree(mock.Anything, "/repo/overseer", sess.WorktreePath).Return(nil).Once()
	agentTmuxID := sess.ID.String() + "-agent"
	editorTmuxID := sess.ID.String() + "-editor"
	// All three backing tmux sessions exist and are torn down.
	tmux.EXPECT().GetSession(mock.Anything, sess.ID.String()).
		Return(domain.TmuxSession{ID: sess.ID.String()}, nil).Once()
	tmux.EXPECT().KillSession(mock.Anything, sess.ID.String()).Return(nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, agentTmuxID).
		Return(domain.TmuxSession{ID: agentTmuxID}, nil).Once()
	tmux.EXPECT().KillSession(mock.Anything, agentTmuxID).Return(nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, editorTmuxID).
		Return(domain.TmuxSession{ID: editorTmuxID}, nil).Once()
	tmux.EXPECT().KillSession(mock.Anything, editorTmuxID).Return(nil).Once()
	repo.EXPECT().Delete(mock.Anything, sess.ID).Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.Delete(context.Background(), DeleteSessionRequest{ID: sess.ID})

	if err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}
}

func TestSessionService_Delete_HappyPath_ProjectMode_NoGitCall(t *testing.T) {
	pinWorktreeRoot(t)
	sess := testutil.MakeSession("alpha", uuid.New())
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, sess.ID.String()).
		Return(domain.TmuxSession{ID: sess.ID.String()}, nil).Once()
	tmux.EXPECT().KillSession(mock.Anything, sess.ID.String()).Return(nil).Once()
	expectAgentAndEditorTmuxGone(tmux, sess.ID.String())
	repo.EXPECT().Delete(mock.Anything, sess.ID).Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.Delete(context.Background(), DeleteSessionRequest{ID: sess.ID})

	if err != nil {
		t.Fatalf("Delete() error = %v, want nil for project-mode session", err)
	}
}

func TestSessionService_Delete_NotFound(t *testing.T) {
	missingID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, missingID).
		Return(domain.Session{}, domain.ErrSessionNotFound).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.Delete(context.Background(), DeleteSessionRequest{ID: missingID})

	if !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("Delete() error = %v, want %v", err, domain.ErrSessionNotFound)
	}
}

func TestSessionService_Delete_WorktreePathOutsideRoot_Refused(t *testing.T) {
	pinWorktreeRoot(t)
	sess := testutil.MakeSessionWithWorktree(
		"malicious",
		uuid.New(),
		"/etc/passwd",
		"overseer/malicious",
	)
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.Delete(context.Background(), DeleteSessionRequest{ID: sess.ID})

	if !errors.Is(err, domain.ErrSessionWorktreePathOutsideRoot) {
		t.Fatalf("Delete() error = %v, want %v", err, domain.ErrSessionWorktreePathOutsideRoot)
	}
}

func TestSessionService_Delete_TmuxSessionAlreadyGone_StillDeletes(t *testing.T) {
	pinWorktreeRoot(t)
	sess := testutil.MakeSession("orphan", uuid.New())
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, sess.ID.String()).
		Return(domain.TmuxSession{}, domain.ErrTmuxSessionNotFound).Once()
	expectAgentAndEditorTmuxGone(tmux, sess.ID.String())
	repo.EXPECT().Delete(mock.Anything, sess.ID).Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.Delete(context.Background(), DeleteSessionRequest{ID: sess.ID})

	if err != nil {
		t.Fatalf("Delete() error = %v, want nil when tmux session is already gone", err)
	}
}

func TestSessionService_Delete_GitWorktreeAlreadyGone_StillDeletes(t *testing.T) {
	pinWorktreeRoot(t)
	overseerID := uuid.New()
	sess := testutil.MakeSessionWithWorktree(
		"alpha",
		overseerID,
		paths.NewResolver("").SessionWorktreePath(uuid.New()),
		"overseer/alpha",
	)
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	project := testutil.MakeProject("/repo/overseer", "overseer")
	project.ID = overseerID
	projects.EXPECT().Get(mock.Anything, overseerID).Return(project, nil).Once()
	git.EXPECT().RemoveWorktree(mock.Anything, "/repo/overseer", sess.WorktreePath).
		Return(domain.ErrGitWorktreeNotFound).Once()
	tmux.EXPECT().GetSession(mock.Anything, sess.ID.String()).
		Return(domain.TmuxSession{}, domain.ErrTmuxSessionNotFound).Once()
	expectAgentAndEditorTmuxGone(tmux, sess.ID.String())
	repo.EXPECT().Delete(mock.Anything, sess.ID).Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.Delete(context.Background(), DeleteSessionRequest{ID: sess.ID})

	if err != nil {
		t.Fatalf("Delete() error = %v, want nil when git worktree is already gone", err)
	}
}

func TestSessionService_AttachEditor_EditorTmuxExists_ReusesWithoutRecreate(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	editorTmuxID := sess.ID.String() + "-editor"
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, editorTmuxID).
		Return(domain.TmuxSession{ID: editorTmuxID}, nil).Once()
	wantCmd := exec.Command("tmux", "attach-session", "-t", editorTmuxID)
	tmux.EXPECT().AttachCommand(mock.Anything, editorTmuxID).Return(wantCmd, nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.AttachEditor(context.Background(), AttachEditorRequest{ID: sess.ID})

	if err != nil {
		t.Fatalf("AttachEditor() error = %v", err)
	}
	if resp.Command != wantCmd {
		t.Fatalf("AttachEditor() Command = %v, want %v", resp.Command, wantCmd)
	}
}

func TestSessionService_AttachEditor_EditorTmuxMissing_CreatesRunningConfiguredEditor(t *testing.T) {
	worktreePath := "/abs/worktree/alpha"
	sess := testutil.MakeSessionWithWorktree("alpha", uuid.New(), worktreePath, "overseer/alpha")
	editorTmuxID := sess.ID.String() + "-editor"
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, editorTmuxID).
		Return(domain.TmuxSession{}, domain.ErrTmuxSessionNotFound).Once()
	tmux.EXPECT().CreateSession(mock.Anything, editorTmuxID, worktreePath, "nvim .").
		Return(editorTmuxID, nil).Once()
	wantCmd := exec.Command("tmux", "attach-session", "-t", editorTmuxID)
	tmux.EXPECT().AttachCommand(mock.Anything, editorTmuxID).Return(wantCmd, nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.AttachEditor(context.Background(), AttachEditorRequest{ID: sess.ID})

	if err != nil {
		t.Fatalf("AttachEditor() error = %v, want nil after create", err)
	}
	if resp.Command != wantCmd {
		t.Fatalf("AttachEditor() Command = %v, want %v", resp.Command, wantCmd)
	}
}

func TestSessionService_AttachEditor_EmptyEditorCommand_FallsBackToNvim(t *testing.T) {
	worktreePath := "/abs/worktree/alpha"
	sess := testutil.MakeSessionWithWorktree("alpha", uuid.New(), worktreePath, "overseer/alpha")
	editorTmuxID := sess.ID.String() + "-editor"
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, editorTmuxID).
		Return(domain.TmuxSession{}, domain.ErrTmuxSessionNotFound).Once()
	tmux.EXPECT().CreateSession(mock.Anything, editorTmuxID, worktreePath, "nvim .").
		Return(editorTmuxID, nil).Once()
	wantCmd := exec.Command("tmux", "attach-session", "-t", editorTmuxID)
	tmux.EXPECT().AttachCommand(mock.Anything, editorTmuxID).Return(wantCmd, nil).Once()

	launcher, _ := domain.NewLauncher("OpenCode", "opencode", domain.AgentTypeOpenCode)
	svc := NewSessionService(repo, projects, tmux, git, noopBoard{}, paths.NewResolver(""), launcher, "", testLogger())
	if _, err := svc.AttachEditor(context.Background(), AttachEditorRequest{ID: sess.ID}); err != nil {
		t.Fatalf("AttachEditor() error = %v", err)
	}
}

func TestSessionService_AttachEditor_SessionNotFound(t *testing.T) {
	sessID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, sessID).Return(domain.Session{}, domain.ErrSessionNotFound).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.AttachEditor(context.Background(), AttachEditorRequest{ID: sessID})

	if !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("AttachEditor() error = %v, want ErrSessionNotFound", err)
	}
}

func TestSessionService_CycleLabel_FromEmptyAssignsFirst(t *testing.T) {
	sess := testutil.MakeSession("alpha", uuid.New())
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()

	var saved domain.Session
	repo.EXPECT().Save(mock.Anything, mock.Anything).
		Run(func(_ context.Context, s domain.Session) { saved = s }).
		Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.CycleLabel(context.Background(), CycleSessionLabelRequest{
		ID:     sess.ID,
		Labels: domain.DefaultLabels,
	})

	if err != nil {
		t.Fatalf("CycleLabel() error = %v", err)
	}
	if resp.Session.Label != "WIP" {
		t.Fatalf("response Label = %q, want %q", resp.Session.Label, "WIP")
	}
	if saved.Label != "WIP" {
		t.Fatalf("saved Label = %q, want %q", saved.Label, "WIP")
	}
}

func TestSessionService_CycleLabel_NotFound(t *testing.T) {
	missingID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	repo.EXPECT().Get(mock.Anything, missingID).
		Return(domain.Session{}, domain.ErrSessionNotFound).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.CycleLabel(context.Background(), CycleSessionLabelRequest{
		ID:     missingID,
		Labels: domain.DefaultLabels,
	})

	if !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("CycleLabel() error = %v, want %v", err, domain.ErrSessionNotFound)
	}
}
