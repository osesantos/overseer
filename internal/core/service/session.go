package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/shared/errs"
	"github.com/dnlopes/overseer/internal/shared/paths"
)

type SessionService struct {
	repo            domain.SessionRepository
	projects        domain.ProjectRepository
	tmux            domain.TmuxAdapter
	git             domain.GitAdapter
	board           domain.SwarmBoardRepository
	pathsResolver   paths.Resolver
	defaultLauncher domain.Launcher
	editorCommand   string
	logger          *slog.Logger
}

// NewSessionService wires the session use-cases. defaultLauncher is used by
// AttachAgent when a session's AgentCommand is empty (pre-launcher sessions);
// editorCommand is the single editor launched by AttachEditor, falling back to
// "nvim" when empty. board is used only to purge a swarm session's message
// board during teardown — the swarm use-cases themselves live in SwarmService.
func NewSessionService(
	repo domain.SessionRepository,
	projects domain.ProjectRepository,
	tmux domain.TmuxAdapter,
	git domain.GitAdapter,
	board domain.SwarmBoardRepository,
	resolver paths.Resolver,
	defaultLauncher domain.Launcher,
	editorCommand string,
	logger *slog.Logger,
) *SessionService {
	return &SessionService{
		repo:            repo,
		projects:        projects,
		tmux:            tmux,
		git:             git,
		board:           board,
		pathsResolver:   resolver,
		defaultLauncher: defaultLauncher,
		editorCommand:   editorCommand,
		logger:          logger,
	}
}

// --- Create ---

type CreateSessionRequest struct {
	Name           string
	ProjectID      uuid.UUID
	CreateWorktree bool
	BaseBranch     string
	Branch         string
	AgentCommand   string
	AgentType      domain.AgentType
	// SwarmSize turns the session into a swarm of that many agents. Leave it at
	// zero for an ordinary single-agent session; a value of one is rejected
	// rather than silently downgraded.
	SwarmSize int
}

type CreateSessionResponse struct {
	Session domain.Session
}

// Create wires up a new session in one of two modes selected by
// CreateWorktree:
//
//   - CreateWorktree=true (Mode 1, default): forks BaseBranch into a fresh
//     git worktree on a new branch (Branch — auto-generated when empty),
//     then registers tmux sessions for the shell and the agent rooted at
//     the worktree path.
//   - CreateWorktree=false (Mode 2): no git work. The session attaches to
//     the project's own working directory; tmux is rooted there. Branch,
//     BaseBranch, and worktree paths are all empty on the persisted row.
//
// Both modes share name validation, duplicate detection, Order assignment,
// tmux/persist tail, and project recency bump.
func (s *SessionService) Create(ctx context.Context, req CreateSessionRequest) (CreateSessionResponse, error) {
	sess, err := domain.NewSession(req.Name, req.ProjectID)
	if err != nil {
		return CreateSessionResponse{}, err
	}

	if req.AgentCommand != "" {
		if err := sess.AssignAgentCommand(req.AgentCommand); err != nil {
			return CreateSessionResponse{}, err
		}
	}

	if req.AgentType != "" {
		if err := sess.AssignAgentType(req.AgentType); err != nil {
			return CreateSessionResponse{}, err
		}
	}

	if req.SwarmSize != 0 {
		if err := sess.AssignSwarmSize(req.SwarmSize); err != nil {
			return CreateSessionResponse{}, err
		}
	}

	project, err := s.projects.Get(ctx, req.ProjectID)
	if err != nil {
		return CreateSessionResponse{}, fmt.Errorf("lookup project: %w", err)
	}

	var resolvedBaseBranch string
	if req.CreateWorktree {
		resolvedBaseBranch = strings.TrimSpace(req.BaseBranch)
		if resolvedBaseBranch == "" {
			resolvedBaseBranch, err = s.git.GetDefaultBranch(ctx, project.Path)
			if err != nil {
				return CreateSessionResponse{}, fmt.Errorf("resolve default branch: %w", err)
			}
		}

		branch := strings.TrimSpace(req.Branch)
		if branch == "" {
			branch = paths.SessionFeatureBranch(sess.ID)
		}
		if err := sess.AssignWorktree(
			s.pathsResolver.SessionWorktreePath(sess.ID),
			branch,
		); err != nil {
			return CreateSessionResponse{}, fmt.Errorf("assign worktree: %w", err)
		}
	}

	nextOrder, err := s.assignNextOrderForProject(ctx, sess)
	if err != nil {
		return CreateSessionResponse{}, err
	}
	sess.Order = nextOrder

	if sess.HasWorktree() {
		if err := s.git.PullBranch(ctx, project.Path, resolvedBaseBranch); err != nil {
			s.logger.Warn("could not pull base branch before forking; worktree may be outdated",
				"base_branch", resolvedBaseBranch, "err", err)
		}
		if err := s.git.CreateWorktree(ctx, project.Path, resolvedBaseBranch, sess.Branch, sess.WorktreePath); err != nil {
			return CreateSessionResponse{}, fmt.Errorf("create git worktree: %w", err)
		}
	}

	sess, err = s.spinUpTmuxAndPersist(ctx, sess, project)
	if err != nil {
		return CreateSessionResponse{}, err
	}
	return CreateSessionResponse{Session: sess}, nil
}

// spinUpTmuxAndPersist runs the post-git tail: it creates the shell + agent
// tmux sessions, saves the session row, and bumps the project's UpdatedAt
// for recency ordering. The tmux sessions are rooted at the session's
// working directory — the worktree path in Mode 1, the project path in
// Mode 2. Each agent tmux session uses sess.AgentCommand, falling back to
// the default launcher when empty — matching the lazy launch semantics in
// ensureTmuxSession.
//
// A swarm gets one pane per agent (AgentTmuxIDs); if any of them fails to start,
// every pane created so far is torn down so a half-built swarm never survives.
func (s *SessionService) spinUpTmuxAndPersist(ctx context.Context, sess domain.Session, project domain.Project) (domain.Session, error) {
	workDir := sessionWorkingDir(sess, project)

	if _, err := s.tmux.CreateSession(ctx, sess.ID.String(), workDir, ""); err != nil {
		return domain.Session{}, fmt.Errorf("create tmux session: %w", err)
	}

	agentCmd := sess.AgentCommand
	if agentCmd == "" {
		agentCmd = s.defaultLauncher.Command
	}

	created := make([]string, 0, sess.AgentCount())
	for _, agentTmuxID := range sess.AgentTmuxIDs() {
		if _, err := s.tmux.CreateSession(ctx, agentTmuxID, workDir, agentCmd); err != nil {
			for _, orphan := range created {
				_ = s.tmux.KillSession(ctx, orphan)
			}
			_ = s.tmux.KillSession(ctx, sess.ID.String())
			return domain.Session{}, fmt.Errorf("create agent tmux session %q: %w", agentTmuxID, err)
		}
		created = append(created, agentTmuxID)
	}

	if err := s.repo.Save(ctx, sess); err != nil {
		return domain.Session{}, fmt.Errorf("save session: %w", err)
	}

	project.UpdatedAt = sess.CreatedAt
	if err := s.projects.Save(ctx, project); err != nil {
		s.logger.WarnContext(ctx, "bump project UpdatedAt failed; recency ordering may lag",
			slog.String("project_id", project.ID.String()),
			slog.String("error", err.Error()),
		)
	}

	return sess, nil
}

// sessionWorkingDir returns the directory a session's tmux + editor live in:
// the worktree for Mode 1, the project path for Mode 2.
func sessionWorkingDir(sess domain.Session, project domain.Project) string {
	if sess.HasWorktree() {
		return sess.WorktreePath
	}
	return project.Path
}

// resolveSessionWorkingDir returns the session's working directory, skipping
// the project lookup entirely for Mode 1 sessions whose worktree path is
// already self-contained.
func (s *SessionService) resolveSessionWorkingDir(ctx context.Context, sess domain.Session) (string, error) {
	if sess.HasWorktree() {
		return sess.WorktreePath, nil
	}
	project, err := s.projects.Get(ctx, sess.ProjectID)
	if err != nil {
		return "", fmt.Errorf("lookup project: %w", err)
	}
	return project.Path, nil
}

// assignNextOrderForProject computes the next Order value for a session
// inside its project, and returns ErrSessionAlreadyExists if a sibling
// session shares the same name.
func (s *SessionService) assignNextOrderForProject(ctx context.Context, sess domain.Session) (int, error) {
	existing, err := s.repo.List(ctx)
	if err != nil {
		return 0, fmt.Errorf("list sessions: %w", err)
	}
	nextOrder := 1
	for _, candidate := range existing {
		if candidate.ProjectID != sess.ProjectID {
			continue
		}
		if candidate.Name == sess.Name {
			return 0, domain.ErrSessionAlreadyExists
		}
		if candidate.Order >= nextOrder {
			nextOrder = candidate.Order + 1
		}
	}
	return nextOrder, nil
}

// --- Rename ---

type RenameSessionRequest struct {
	ID      uuid.UUID
	NewName string
}

type RenameSessionResponse struct {
	Session domain.Session
}

func (s *SessionService) Rename(ctx context.Context, req RenameSessionRequest) (RenameSessionResponse, error) {
	sess, err := s.repo.Get(ctx, req.ID)
	if err != nil {
		return RenameSessionResponse{}, err
	}

	existing, err := s.repo.List(ctx)
	if err != nil {
		return RenameSessionResponse{}, fmt.Errorf("list sessions: %w", err)
	}

	for _, candidate := range existing {
		if candidate.ID == sess.ID {
			continue
		}
		if candidate.ProjectID == sess.ProjectID && candidate.Name == req.NewName {
			return RenameSessionResponse{}, domain.ErrSessionAlreadyExists
		}
	}

	if err := sess.Rename(req.NewName); err != nil {
		return RenameSessionResponse{}, err
	}

	if err := s.repo.Save(ctx, sess); err != nil {
		return RenameSessionResponse{}, fmt.Errorf("save session: %w", err)
	}

	return RenameSessionResponse{Session: sess}, nil
}

// --- CycleLabel ---

type CycleSessionLabelRequest struct {
	ID     uuid.UUID
	Labels []domain.Label
}

type CycleSessionLabelResponse struct {
	Session domain.Session
}

// CycleLabel advances the session's status label through the configured
// cycle (see [domain.NextLabelCode]). Driven by the l keybinding in the
// TUI: empty → first → … → last → empty. The Labels slice is the
// configured cycle order — typically [domain.DefaultLabels] or whatever
// the user provided in config. An empty Labels slice clears whatever
// label the session held.
func (s *SessionService) CycleLabel(ctx context.Context, req CycleSessionLabelRequest) (CycleSessionLabelResponse, error) {
	sess, err := s.repo.Get(ctx, req.ID)
	if err != nil {
		return CycleSessionLabelResponse{}, err
	}

	next := domain.NextLabelCode(sess.Label, req.Labels)
	if err := sess.AssignLabel(next); err != nil {
		return CycleSessionLabelResponse{}, err
	}

	if err := s.repo.Save(ctx, sess); err != nil {
		return CycleSessionLabelResponse{}, fmt.Errorf("save session: %w", err)
	}

	return CycleSessionLabelResponse{Session: sess}, nil
}

// --- List ---

type ListSessionsRequest struct{}

type ListSessionsResponse struct {
	Sessions []domain.Session
}

func (s *SessionService) List(ctx context.Context, _ ListSessionsRequest) (ListSessionsResponse, error) {
	sessions, err := s.repo.List(ctx)
	if err != nil {
		return ListSessionsResponse{}, err
	}

	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].ProjectID == sessions[j].ProjectID {
			return sessions[i].Order < sessions[j].Order
		}
		return sessions[i].ProjectID.String() < sessions[j].ProjectID.String()
	})

	return ListSessionsResponse{Sessions: sessions}, nil
}

// --- ListBranches ---

type ListBranchesRequest struct {
	ProjectID uuid.UUID
}

type ListBranchesResponse struct {
	Branches      []domain.BranchInfo
	DefaultBranch string
}

// ListBranches reads the local and remote-tracking branches of the supplied
// project plus the project's default branch (best-effort: empty if the
// adapter can't resolve one). Used by the TUI's branch picker to populate
// its cache and pin the default at the top of the list.
func (s *SessionService) ListBranches(ctx context.Context, req ListBranchesRequest) (ListBranchesResponse, error) {
	project, err := s.projects.Get(ctx, req.ProjectID)
	if err != nil {
		return ListBranchesResponse{}, fmt.Errorf("lookup project: %w", err)
	}
	branches, err := s.git.ListBranches(ctx, project.Path)
	if err != nil {
		return ListBranchesResponse{}, fmt.Errorf("list branches: %w", err)
	}
	defaultBranch, _ := s.git.GetDefaultBranch(ctx, project.Path)
	return ListBranchesResponse{Branches: branches, DefaultBranch: defaultBranch}, nil
}

// --- ProjectCurrentBranch ---

type ProjectCurrentBranchRequest struct {
	ProjectID uuid.UUID
}

type ProjectCurrentBranchResponse struct {
	Branch string
}

// ProjectCurrentBranch reads the branch HEAD currently points at in the
// project's working directory. Used by Mode 2 sessions to surface the
// live branch in the details panel without persisting it.
func (s *SessionService) ProjectCurrentBranch(ctx context.Context, req ProjectCurrentBranchRequest) (ProjectCurrentBranchResponse, error) {
	project, err := s.projects.Get(ctx, req.ProjectID)
	if err != nil {
		return ProjectCurrentBranchResponse{}, fmt.Errorf("lookup project: %w", err)
	}
	branch, err := s.git.CurrentBranch(ctx, project.Path)
	if err != nil {
		return ProjectCurrentBranchResponse{}, fmt.Errorf("read current branch: %w", err)
	}
	return ProjectCurrentBranchResponse{Branch: branch}, nil
}

// --- Reorder ---

type ReorderSessionRequest struct {
	ID        uuid.UUID
	Direction int
}

type ReorderSessionResponse struct {
	Sessions []domain.Session
}

func (s *SessionService) Reorder(ctx context.Context, req ReorderSessionRequest) (ReorderSessionResponse, error) {
	target, err := s.repo.Get(ctx, req.ID)
	if err != nil {
		return ReorderSessionResponse{}, err
	}

	all, err := s.repo.List(ctx)
	if err != nil {
		return ReorderSessionResponse{}, fmt.Errorf("list sessions: %w", err)
	}

	projectSessions := make([]domain.Session, 0, len(all))
	for _, sess := range all {
		if sess.ProjectID == target.ProjectID {
			projectSessions = append(projectSessions, sess)
		}
	}
	sort.Slice(projectSessions, func(i, j int) bool {
		return projectSessions[i].Order < projectSessions[j].Order
	})

	if len(projectSessions) <= 1 {
		return ReorderSessionResponse{}, errs.ErrNoOp
	}

	idx := -1
	for i, sess := range projectSessions {
		if sess.ID == target.ID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return ReorderSessionResponse{}, fmt.Errorf("session %s not found in project list", target.ID)
	}

	if (idx == 0 && req.Direction == -1) || (idx == len(projectSessions)-1 && req.Direction == 1) {
		return ReorderSessionResponse{}, errs.ErrNoOp
	}

	neighbor := idx + req.Direction

	projectSessions[idx].Order, projectSessions[neighbor].Order = projectSessions[neighbor].Order, projectSessions[idx].Order
	projectSessions[idx].UpdatedAt = time.Now()

	if err := s.repo.Save(ctx, projectSessions[idx]); err != nil {
		return ReorderSessionResponse{}, fmt.Errorf("save target session: %w", err)
	}
	if err := s.repo.Save(ctx, projectSessions[neighbor]); err != nil {
		return ReorderSessionResponse{}, fmt.Errorf("save neighbor session: %w", err)
	}

	sort.Slice(projectSessions, func(i, j int) bool {
		return projectSessions[i].Order < projectSessions[j].Order
	})

	s.logger.InfoContext(ctx, "session reordered",
		slog.String("id", target.ID.String()),
		slog.Int("direction", req.Direction),
	)

	return ReorderSessionResponse{Sessions: projectSessions}, nil
}

// --- AttachShell ---

type AttachShellRequest struct {
	ID uuid.UUID
}

type AttachShellResponse struct {
	Command *exec.Cmd
}

func (s *SessionService) AttachShell(ctx context.Context, req AttachShellRequest) (AttachShellResponse, error) {
	sess, err := s.repo.Get(ctx, req.ID)
	if err != nil {
		return AttachShellResponse{}, err
	}

	tmuxID := sess.ID.String()
	if err := s.ensureTmuxSession(ctx, tmuxID, sess, ""); err != nil {
		return AttachShellResponse{}, err
	}

	cmd, err := s.tmux.AttachCommand(ctx, tmuxID)
	if err != nil {
		return AttachShellResponse{}, fmt.Errorf("attach tmux session: %w", err)
	}

	s.logger.InfoContext(ctx, "shell attach prepared",
		slog.String("id", tmuxID),
	)

	return AttachShellResponse{Command: cmd}, nil
}

// --- AttachAgent ---

type AttachAgentRequest struct {
	ID uuid.UUID
	// AgentIndex selects which pane of a swarm to attach to (1-based). It is
	// ignored by single-agent sessions.
	AgentIndex int
}

type AttachAgentResponse struct {
	Command *exec.Cmd
}

func (s *SessionService) AttachAgent(ctx context.Context, req AttachAgentRequest) (AttachAgentResponse, error) {
	sess, err := s.repo.Get(ctx, req.ID)
	if err != nil {
		return AttachAgentResponse{}, err
	}

	agentTmuxID := sess.AgentTmuxID(req.AgentIndex)
	agentCmd := sess.AgentCommand
	if agentCmd == "" {
		agentCmd = s.defaultLauncher.Command
	}
	if agentCmd == "" {
		return AttachAgentResponse{}, domain.ErrSessionNoAgentCommandAvailable
	}
	if err := s.ensureTmuxSession(ctx, agentTmuxID, sess, agentCmd); err != nil {
		return AttachAgentResponse{}, err
	}

	cmd, err := s.tmux.AttachCommand(ctx, agentTmuxID)
	if err != nil {
		return AttachAgentResponse{}, fmt.Errorf("attach tmux session: %w", err)
	}

	s.logger.InfoContext(ctx, "agent attach prepared",
		slog.String("id", agentTmuxID),
		slog.String("agent_command", agentCmd),
	)

	return AttachAgentResponse{Command: cmd}, nil
}

// --- AttachEditor ---

type AttachEditorRequest struct {
	ID uuid.UUID
}

type AttachEditorResponse struct {
	Command *exec.Cmd
}

// AttachEditor prepares an attach into the session's dedicated editor tmux
// session (<uuid>-editor), lazily creating it running the configured editor
// (falling back to "nvim" when unset) rooted at the session's working
// directory. Like AttachShell/AttachAgent it returns a runnable *exec.Cmd the
// TUI hands to tea.ExecProcess; the editor tab persists afterwards so a
// subsequent Enter re-attaches into the same nvim.
func (s *SessionService) AttachEditor(ctx context.Context, req AttachEditorRequest) (AttachEditorResponse, error) {
	sess, err := s.repo.Get(ctx, req.ID)
	if err != nil {
		return AttachEditorResponse{}, err
	}

	editorCmd := strings.TrimSpace(s.editorCommand)
	if editorCmd == "" {
		editorCmd = defaultEditorCommand
	}
	editorTmuxID := previewTmuxID(sess, PreviewKindEditor, 0)
	if err := s.ensureTmuxSession(ctx, editorTmuxID, sess, editorCmd+" ."); err != nil {
		return AttachEditorResponse{}, err
	}

	cmd, err := s.tmux.AttachCommand(ctx, editorTmuxID)
	if err != nil {
		return AttachEditorResponse{}, fmt.Errorf("attach tmux session: %w", err)
	}

	s.logger.InfoContext(ctx, "editor attach prepared",
		slog.String("id", editorTmuxID),
		slog.String("editor_command", editorCmd),
	)

	return AttachEditorResponse{Command: cmd}, nil
}

// --- SendAgentEnter ---

type SendAgentEnterRequest struct {
	ID uuid.UUID
	// AgentIndex selects which pane of a swarm to poke (1-based). Ignored by
	// single-agent sessions.
	AgentIndex int
}

type SendAgentEnterResponse struct{}

// SendAgentEnter delivers an Enter keypress to the agent tmux pane of the
// given session without attaching to it. The TUI keeps running. If the agent
// pane does not exist the call returns domain.ErrTmuxSessionNotFound.
func (s *SessionService) SendAgentEnter(ctx context.Context, req SendAgentEnterRequest) (SendAgentEnterResponse, error) {
	sess, err := s.repo.Get(ctx, req.ID)
	if err != nil {
		return SendAgentEnterResponse{}, err
	}
	agentTmuxID := sess.AgentTmuxID(req.AgentIndex)
	if err := s.tmux.SendKeys(ctx, agentTmuxID, "Enter"); err != nil {
		return SendAgentEnterResponse{}, fmt.Errorf("send enter to agent: %w", err)
	}
	s.logger.InfoContext(ctx, "sent enter to agent",
		slog.String("id", sess.ID.String()),
		slog.String("tmux_id", agentTmuxID),
	)
	return SendAgentEnterResponse{}, nil
}

// --- SendAgentEnterAll ---

type SendAgentEnterAllRequest struct {
	ID uuid.UUID
}

type SendAgentEnterAllResponse struct {
	// Delivered counts the panes that accepted the keystroke.
	Delivered int
}

// SendAgentEnterAll delivers an Enter keypress to every agent pane of a session
// — one pane for an ordinary session, all N for a swarm.
//
// This is the operator's "submit whatever is queued" lever. A bare Enter is safe
// to broadcast: it submits a pending prompt, and is a harmless newline on an
// agent that is idle or already working. An unreachable pane is logged and
// skipped rather than failing the call, so one dead agent cannot stop the others
// being woken.
func (s *SessionService) SendAgentEnterAll(ctx context.Context, req SendAgentEnterAllRequest) (SendAgentEnterAllResponse, error) {
	sess, err := s.repo.Get(ctx, req.ID)
	if err != nil {
		return SendAgentEnterAllResponse{}, err
	}

	delivered := 0
	for _, tmuxID := range sess.AgentTmuxIDs() {
		if err := s.tmux.SendKeys(ctx, tmuxID, "Enter"); err != nil {
			s.logger.WarnContext(ctx, "could not send enter to agent pane",
				slog.String("session_id", sess.ID.String()),
				slog.String("tmux_id", tmuxID),
				slog.String("error", err.Error()),
			)
			continue
		}
		delivered++
	}

	s.logger.InfoContext(ctx, "sent enter to agent panes",
		slog.String("id", sess.ID.String()),
		slog.Int("delivered", delivered),
		slog.Int("agents", sess.AgentCount()),
	)
	return SendAgentEnterAllResponse{Delivered: delivered}, nil
}

// --- SendAgentPrompt ---

type SendAgentPromptRequest struct {
	ID uuid.UUID
	// AgentIndex selects which pane of a swarm receives the prompt (1-based).
	// Ignored by single-agent sessions.
	AgentIndex int
	Prompt     string
}

type SendAgentPromptResponse struct{}

// SendAgentPrompt delivers a literal text prompt followed by an Enter keypress
// to the agent tmux pane of the given session without attaching to it. The
// text is sent in literal mode so characters like < > / " are delivered
// verbatim. If the agent pane does not exist the call returns
// domain.ErrTmuxSessionNotFound.
func (s *SessionService) SendAgentPrompt(ctx context.Context, req SendAgentPromptRequest) (SendAgentPromptResponse, error) {
	sess, err := s.repo.Get(ctx, req.ID)
	if err != nil {
		return SendAgentPromptResponse{}, err
	}
	agentTmuxID := sess.AgentTmuxID(req.AgentIndex)
	if err := s.tmux.SendText(ctx, agentTmuxID, req.Prompt); err != nil {
		return SendAgentPromptResponse{}, fmt.Errorf("send prompt to agent: %w", err)
	}
	if err := s.tmux.SendKeys(ctx, agentTmuxID, "Enter"); err != nil {
		return SendAgentPromptResponse{}, fmt.Errorf("send enter after prompt: %w", err)
	}
	s.logger.InfoContext(ctx, "sent prompt to agent",
		slog.String("id", sess.ID.String()),
		slog.String("tmux_id", agentTmuxID),
		slog.Int("prompt_len", len(req.Prompt)),
	)
	return SendAgentPromptResponse{}, nil
}

// --- PreviewSession ---

// PreviewKind selects which tmux session attached to an Overseer session is
// captured by PreviewSession.
type PreviewKind int

const (
	PreviewKindShell PreviewKind = iota
	PreviewKindAgent
	PreviewKindEditor
)

// defaultEditorCommand is the editor run in a session's Editor tmux session
// when no editorCommand is configured.
const defaultEditorCommand = "nvim"

const tmuxSuffixEditor = "-editor"

// previewTmuxID resolves the tmux session backing the requested target of a
// session. The shell uses the bare UUID and the editor appends "-editor"; agent
// panes go through Session.AgentTmuxID so swarm sessions address the pane at
// agentIndex while single-agent sessions ignore it.
func previewTmuxID(sess domain.Session, kind PreviewKind, agentIndex int) string {
	switch kind {
	case PreviewKindAgent:
		return sess.AgentTmuxID(agentIndex)
	case PreviewKindEditor:
		return sess.ID.String() + tmuxSuffixEditor
	default:
		return sess.ID.String()
	}
}

type PreviewSessionRequest struct {
	ID   uuid.UUID
	Kind PreviewKind
	// AgentIndex selects which pane of a swarm to capture when Kind is
	// PreviewKindAgent (1-based). Ignored otherwise.
	AgentIndex int
	Width      int
	Height     int
}

// PreviewSessionResponse carries a snapshot of the targeted tmux pane.
// SessionReady is false when the tmux session does not yet exist (typically
// the agent session before its first attach); callers should render a
// placeholder rather than treat this as an error.
type PreviewSessionResponse struct {
	Content      string
	SessionReady bool
}

func (s *SessionService) PreviewSession(ctx context.Context, req PreviewSessionRequest) (PreviewSessionResponse, error) {
	sess, err := s.repo.Get(ctx, req.ID)
	if err != nil {
		return PreviewSessionResponse{}, err
	}

	tmuxID := previewTmuxID(sess, req.Kind, req.AgentIndex)

	if req.Width > 0 && req.Height > 0 {
		if err := s.tmux.ResizeWindow(ctx, tmuxID, req.Width, req.Height); err != nil {
			if errors.Is(err, domain.ErrTmuxSessionNotFound) {
				return PreviewSessionResponse{SessionReady: false}, nil
			}
			return PreviewSessionResponse{}, fmt.Errorf("resize pane: %w", err)
		}
	}

	content, err := s.tmux.CapturePane(ctx, tmuxID)
	if errors.Is(err, domain.ErrTmuxSessionNotFound) {
		return PreviewSessionResponse{SessionReady: false}, nil
	}
	if err != nil {
		return PreviewSessionResponse{}, fmt.Errorf("capture pane: %w", err)
	}
	return PreviewSessionResponse{Content: content, SessionReady: true}, nil
}

// --- KillPreviewSession ---

type KillPreviewSessionRequest struct {
	ID   uuid.UUID
	Kind PreviewKind
	// AgentIndex selects which pane of a swarm to kill when Kind is
	// PreviewKindAgent (1-based). Ignored otherwise.
	AgentIndex int
}

type KillPreviewSessionResponse struct{}

func (s *SessionService) KillPreviewSession(ctx context.Context, req KillPreviewSessionRequest) (KillPreviewSessionResponse, error) {
	sess, err := s.repo.Get(ctx, req.ID)
	if err != nil {
		return KillPreviewSessionResponse{}, err
	}

	tmuxID := previewTmuxID(sess, req.Kind, req.AgentIndex)

	if err := s.killTmuxIfExists(ctx, tmuxID); err != nil {
		return KillPreviewSessionResponse{}, fmt.Errorf("kill preview session: %w", err)
	}

	s.logger.InfoContext(ctx, "preview session killed",
		slog.String("id", sess.ID.String()),
		slog.String("tmux_id", tmuxID),
	)
	return KillPreviewSessionResponse{}, nil
}

// --- Delete ---

type DeleteSessionRequest struct {
	ID uuid.UUID
}

type DeleteSessionResponse struct{}

// Delete tears down a session in three steps, in this order:
//
//  1. If the session has a worktree (Mode 1), the path is verified to live
//     inside paths.WorktreeRoot() (defence in depth against a tampered DB
//     row) and the git worktree is removed. Mode 2 sessions skip git
//     entirely. If the owning project no longer exists in the repository, or
//     the worktree was already removed by a prior interrupted delete, git
//     removal is skipped with a warning — the rest of the teardown still
//     proceeds.
//  2. Every backing tmux session is killed, if it still exists: the shell, each
//     agent pane (one for a single-agent session, N for a swarm) and the editor.
//     A missing tmux session is not an error: the user may have killed it
//     manually or the tmux server may have restarted.
//  3. A swarm's message board is purged. A failure here is logged and swallowed
//     — stale board data on disk must not strand a session row the user asked
//     to delete.
//  4. The session row is deleted from the repository last, so any failure in
//     steps 1 or 2 leaves a retriable session row instead of an orphaned
//     worktree or tmux session paired with no DB record.
func (s *SessionService) Delete(ctx context.Context, req DeleteSessionRequest) (DeleteSessionResponse, error) {
	sess, err := s.repo.Get(ctx, req.ID)
	if err != nil {
		return DeleteSessionResponse{}, err
	}

	if sess.HasWorktree() {
		if !sess.WorktreeIsInsideRoot(s.pathsResolver.WorktreeRoot()) {
			s.logger.ErrorContext(ctx, "session worktree path outside managed root, refusing to delete",
				slog.String("id", sess.ID.String()),
				slog.String("worktree_path", sess.WorktreePath),
				slog.String("worktree_root", s.pathsResolver.WorktreeRoot()),
			)
			return DeleteSessionResponse{}, domain.ErrSessionWorktreePathOutsideRoot
		}
		if err := s.removeWorktreeForSession(ctx, sess); err != nil {
			return DeleteSessionResponse{}, err
		}
	}

	// Tear down every backing tmux session: the shell (bare UUID), each agent
	// pane, and the editor (-editor). A missing session is not an error
	// (killTmuxIfExists no-ops), so partially-created sessions clean up.
	tmuxIDs := append([]string{sess.ID.String()}, sess.AgentTmuxIDs()...)
	tmuxIDs = append(tmuxIDs, sess.ID.String()+tmuxSuffixEditor)
	for _, tmuxID := range tmuxIDs {
		if err := s.killTmuxIfExists(ctx, tmuxID); err != nil {
			return DeleteSessionResponse{}, err
		}
	}

	if sess.IsSwarm() {
		if err := s.board.Purge(ctx, sess.ID); err != nil {
			s.logger.WarnContext(ctx, "purge swarm board failed; board data left on disk",
				slog.String("session_id", sess.ID.String()),
				slog.String("error", err.Error()),
			)
		}
	}

	if err := s.repo.Delete(ctx, sess.ID); err != nil {
		return DeleteSessionResponse{}, fmt.Errorf("delete session: %w", err)
	}

	s.logger.InfoContext(ctx, "session deleted",
		slog.String("id", sess.ID.String()),
		slog.String("name", sess.Name),
	)
	return DeleteSessionResponse{}, nil
}

func (s *SessionService) removeWorktreeForSession(ctx context.Context, sess domain.Session) error {
	project, err := s.projects.Get(ctx, sess.ProjectID)
	if err != nil {
		if errors.Is(err, domain.ErrProjectNotFound) {
			s.logger.WarnContext(ctx, "owning project missing, skipping git worktree removal",
				slog.String("session_id", sess.ID.String()),
				slog.String("project_id", sess.ProjectID.String()),
				slog.String("worktree_path", sess.WorktreePath),
			)
			return nil
		}
		return fmt.Errorf("lookup project: %w", err)
	}
	if err := s.git.RemoveWorktree(ctx, project.Path, sess.WorktreePath); err != nil {
		if errors.Is(err, domain.ErrGitWorktreeNotFound) {
			s.logger.WarnContext(ctx, "git worktree already gone, nothing to remove",
				slog.String("session_id", sess.ID.String()),
				slog.String("worktree_path", sess.WorktreePath),
			)
			return nil
		}
		return fmt.Errorf("remove git worktree: %w", err)
	}
	return nil
}

func (s *SessionService) killTmuxIfExists(ctx context.Context, tmuxID string) error {
	if _, err := s.tmux.GetSession(ctx, tmuxID); err != nil {
		if errors.Is(err, domain.ErrTmuxSessionNotFound) {
			s.logger.InfoContext(ctx, "tmux session already gone, nothing to kill",
				slog.String("id", tmuxID),
			)
			return nil
		}
		return fmt.Errorf("inspect tmux session: %w", err)
	}
	if err := s.tmux.KillSession(ctx, tmuxID); err != nil {
		return fmt.Errorf("kill tmux session: %w", err)
	}
	return nil
}

func (s *SessionService) ensureTmuxSession(ctx context.Context, tmuxID string, sess domain.Session, shellCommand string) error {
	_, err := s.tmux.GetSession(ctx, tmuxID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, domain.ErrTmuxSessionNotFound) {
		return fmt.Errorf("inspect tmux session: %w", err)
	}

	startDir, err := s.resolveSessionWorkingDir(ctx, sess)
	if err != nil {
		return err
	}

	if _, err := s.tmux.CreateSession(ctx, tmuxID, startDir, shellCommand); err != nil {
		return fmt.Errorf("recreate tmux session: %w", err)
	}
	s.logger.InfoContext(ctx, "tmux session recreated",
		slog.String("id", tmuxID),
	)
	return nil
}
