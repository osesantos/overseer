package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/shared/paths"
)

// SwarmService owns the use-cases of a swarm session's communication board:
// posting to it, reading it, waking idle agents, and handing the agents their
// bootstrap contract.
//
// Waking agents is the part that makes a swarm actually work. Agent CLIs do not
// loop — they finish a turn and sit idle at their prompt — so nothing would ever
// make agent 3 read what agent 2 just wrote. Overseer therefore has to poke
// them, which it does by typing into their tmux panes.
//
// Post only *records* that a nudge is due; FlushNudges performs it. Splitting
// the two keeps timing out of the service (the caller decides how often to
// flush) and coalesces a burst of posts into a single round of pokes, which is
// the main defence against a swarm nudging itself into an infinite conversation.
type SwarmService struct {
	board       domain.SwarmBoardRepository
	descriptors domain.SwarmDescriptorWriter
	sessions    domain.SessionRepository
	tmux        domain.TmuxAdapter
	resolver    paths.Resolver
	maxMessages int
	logger      *slog.Logger

	mu sync.Mutex
	// pending maps a session to the author of the most recent post it has seen.
	// Presence means "this board moved since the last flush"; the author is
	// excluded from the next round because they have just spoken.
	pending map[uuid.UUID]string
	// pendingSubmit counts how many more times a session's agent panes should be
	// sent a bare Enter to submit a briefing that was typed into them. See
	// briefingSubmitAttempts.
	pendingSubmit map[uuid.UUID]int
}

// briefingSubmitAttempts is how many flush rounds will try to submit a freshly
// typed briefing.
//
// An agent pane is created and briefed within milliseconds, but the agent CLI
// behind it takes seconds to start accepting input — so the briefing text lands
// in its prompt while the Enter that should submit it is swallowed by the startup
// sequence, leaving the agent holding a prompt it never sent.
//
// Rather than model each CLI's readiness (which needs per-pane status detection
// Overseer does not have yet), the flush job simply re-sends Enter a few times.
// Enter is safe to repeat: it submits a queued prompt and is a harmless newline
// otherwise. At the default 2s debounce this covers roughly the first six
// seconds; a slower cold start is what the operator's broadcast lever is for.
const briefingSubmitAttempts = 3

// NewSwarmService wires the swarm use-cases. maxMessages is the per-session
// board cap that trips the runaway-chatter circuit breaker; a value of zero or
// less disables the cap.
func NewSwarmService(
	board domain.SwarmBoardRepository,
	descriptors domain.SwarmDescriptorWriter,
	sessions domain.SessionRepository,
	tmux domain.TmuxAdapter,
	resolver paths.Resolver,
	maxMessages int,
	logger *slog.Logger,
) *SwarmService {
	return &SwarmService{
		board:       board,
		descriptors: descriptors,
		sessions:    sessions,
		tmux:        tmux,
		resolver:    resolver,
		maxMessages: maxMessages,
		logger:        logger,
		pending:       make(map[uuid.UUID]string),
		pendingSubmit: make(map[uuid.UUID]int),
	}
}

// --- Post ---

type PostSwarmMessageRequest struct {
	SessionID uuid.UUID
	Author    string
	Role      domain.SwarmRole
	Content   string
}

type PostSwarmMessageResponse struct {
	Message domain.SwarmMessage
}

// Post validates and appends a message to the session's board, then marks the
// session as needing a nudge. It returns as soon as the message is durable —
// waking the other agents is FlushNudges' job — so an agent's HTTP call is never
// blocked behind a round of tmux writes.
func (s *SwarmService) Post(ctx context.Context, req PostSwarmMessageRequest) (PostSwarmMessageResponse, error) {
	sess, err := s.requireSwarm(ctx, req.SessionID)
	if err != nil {
		return PostSwarmMessageResponse{}, err
	}

	msg, err := domain.NewSwarmMessage(req.SessionID, req.Author, req.Role, req.Content)
	if err != nil {
		return PostSwarmMessageResponse{}, err
	}

	if s.maxMessages > 0 {
		count, err := s.board.Count(ctx, sess.ID)
		if err != nil {
			return PostSwarmMessageResponse{}, fmt.Errorf("count board messages: %w", err)
		}
		if count >= s.maxMessages {
			s.logger.WarnContext(ctx, "swarm board cap reached, rejecting post",
				slog.String("session_id", sess.ID.String()),
				slog.Int("count", count),
				slog.Int("cap", s.maxMessages),
			)
			return PostSwarmMessageResponse{}, domain.ErrSwarmBoardCapReached
		}
	}

	stored, err := s.board.Append(ctx, msg)
	if err != nil {
		return PostSwarmMessageResponse{}, fmt.Errorf("append board message: %w", err)
	}

	s.mu.Lock()
	s.pending[sess.ID] = msg.Author
	s.mu.Unlock()

	s.logger.InfoContext(ctx, "swarm message posted",
		slog.String("session_id", sess.ID.String()),
		slog.String("author", msg.Author),
		slog.Int("seq", stored.Seq),
	)
	return PostSwarmMessageResponse{Message: stored}, nil
}

// --- ListMessages ---

type ListSwarmMessagesRequest struct {
	SessionID uuid.UUID
	Since     int
}

type ListSwarmMessagesResponse struct {
	Messages []domain.SwarmMessage
	// LatestSeq is the cursor the caller should send next. It never goes
	// backwards: an empty result returns the cursor that came in.
	LatestSeq int
}

// ListMessages returns the session's board entries newer than Since. Callers
// (the TUI's board tab, and agents polling over HTTP) use LatestSeq as their
// watermark so each read only transfers what they have not seen.
func (s *SwarmService) ListMessages(ctx context.Context, req ListSwarmMessagesRequest) (ListSwarmMessagesResponse, error) {
	messages, err := s.board.ListSince(ctx, req.SessionID, req.Since)
	if err != nil {
		return ListSwarmMessagesResponse{}, fmt.Errorf("list board messages: %w", err)
	}

	latest := req.Since
	for _, msg := range messages {
		if msg.Seq > latest {
			latest = msg.Seq
		}
	}
	return ListSwarmMessagesResponse{Messages: messages, LatestSeq: latest}, nil
}

// --- FlushNudges ---

type FlushSwarmNudgesRequest struct{}

type FlushSwarmNudgesResponse struct {
	// Nudged counts the agent panes successfully poked across all sessions.
	Nudged int
}

// FlushNudges wakes the agents of every session whose board has moved since the
// last flush, then clears the pending set. Callers run it on a timer: the
// interval is the debounce window, so a burst of posts costs exactly one round
// of pokes.
//
// The most recent author is skipped — they have just spoken and are presumably
// still working. A pane that cannot be reached is logged and skipped rather than
// failing the flush, so one dead agent never silences the rest of the swarm.
func (s *SwarmService) FlushNudges(ctx context.Context, _ FlushSwarmNudgesRequest) (FlushSwarmNudgesResponse, error) {
	s.mu.Lock()
	due := s.pending
	s.pending = make(map[uuid.UUID]string)
	s.mu.Unlock()

	nudged := 0
	for sessionID, lastAuthor := range due {
		sess, err := s.sessions.Get(ctx, sessionID)
		if err != nil {
			s.logger.WarnContext(ctx, "skipping swarm nudge, session lookup failed",
				slog.String("session_id", sessionID.String()),
				slog.String("error", err.Error()),
			)
			continue
		}
		if !sess.IsSwarm() {
			continue
		}
		nudged += s.nudgeSession(ctx, sess, lastAuthor)
	}

	return FlushSwarmNudgesResponse{Nudged: nudged}, nil
}

// nudgeSession types the read-the-board instruction into every agent pane of
// sess except skipAuthor's, returning how many panes were reached.
func (s *SwarmService) nudgeSession(ctx context.Context, sess domain.Session, skipAuthor string) int {
	nudged := 0
	for index := 1; index <= sess.AgentCount(); index++ {
		if domain.SwarmAgentAuthor(index) == skipAuthor {
			continue
		}

		tmuxID := sess.AgentTmuxID(index)
		if err := s.tmux.SendText(ctx, tmuxID, swarmNudgePrompt); err != nil {
			s.logger.WarnContext(ctx, "could not nudge swarm agent",
				slog.String("session_id", sess.ID.String()),
				slog.String("tmux_id", tmuxID),
				slog.String("error", err.Error()),
			)
			continue
		}
		if err := s.tmux.SendKeys(ctx, tmuxID, "Enter"); err != nil {
			s.logger.WarnContext(ctx, "could not submit swarm nudge",
				slog.String("session_id", sess.ID.String()),
				slog.String("tmux_id", tmuxID),
				slog.String("error", err.Error()),
			)
			continue
		}
		nudged++
	}
	return nudged
}

// --- Bootstrap ---

type BootstrapSwarmRequest struct {
	SessionID uuid.UUID
	BoardURL  string
	Goal      string
}

// Bootstrap prepares a freshly created swarm: it writes the contract the agents
// read, records the goal as the board's first message so it shows up in history,
// and introduces each agent to its own identity.
//
// The order matters. The descriptor is written first and a failure aborts, so
// agents are never told to read a contract that does not exist.
func (s *SwarmService) Bootstrap(ctx context.Context, req BootstrapSwarmRequest) error {
	sess, err := s.requireSwarm(ctx, req.SessionID)
	if err != nil {
		return err
	}

	desc, err := domain.NewSwarmDescriptor(sess.ID, req.BoardURL, sess.SwarmSize, req.Goal)
	if err != nil {
		return err
	}

	if err := s.descriptors.Write(ctx, desc); err != nil {
		return fmt.Errorf("write swarm descriptor: %w", err)
	}

	goalMsg, err := domain.NewSwarmMessage(sess.ID, swarmHumanAuthor, domain.SwarmRoleHuman, desc.Goal)
	if err != nil {
		return err
	}
	if _, err := s.board.Append(ctx, goalMsg); err != nil {
		return fmt.Errorf("post swarm goal: %w", err)
	}

	descriptorPath := s.resolver.SwarmDescriptorFile(sess.ID)
	for index := 1; index <= sess.AgentCount(); index++ {
		tmuxID := sess.AgentTmuxID(index)
		prompt := swarmBootstrapPrompt(index, sess.SwarmSize, descriptorPath)

		if err := s.tmux.SendText(ctx, tmuxID, prompt); err != nil {
			s.logger.WarnContext(ctx, "could not brief swarm agent",
				slog.String("session_id", sess.ID.String()),
				slog.String("tmux_id", tmuxID),
				slog.String("error", err.Error()),
			)
			continue
		}
	}

	// Submitting is left to FlushNudges. The agent CLI is still starting up at
	// this point and would swallow an Enter sent now, stranding the briefing in
	// its prompt — see briefingSubmitAttempts.
	s.mu.Lock()
	s.pendingSubmit[sess.ID] = briefingSubmitAttempts
	s.mu.Unlock()

	s.logger.InfoContext(ctx, "swarm briefed, awaiting submit",
		slog.String("session_id", sess.ID.String()),
		slog.Int("agents", sess.SwarmSize),
		slog.Int("submit_attempts", briefingSubmitAttempts),
	)
	return nil
}

// --- SubmitBriefings ---

type SubmitSwarmBriefingsRequest struct{}

type SubmitSwarmBriefingsResponse struct {
	// Submitted counts the panes sent an Enter across all sessions.
	Submitted int
}

// SubmitBriefings sends a bare Enter to the agent panes of every session with a
// briefing still awaiting submission, consuming one attempt per session. Called
// by the same job that flushes nudges.
func (s *SwarmService) SubmitBriefings(ctx context.Context, _ SubmitSwarmBriefingsRequest) (SubmitSwarmBriefingsResponse, error) {
	s.mu.Lock()
	due := make(map[uuid.UUID]int, len(s.pendingSubmit))
	for sessionID, attempts := range s.pendingSubmit {
		due[sessionID] = attempts
		if attempts <= 1 {
			delete(s.pendingSubmit, sessionID)
		} else {
			s.pendingSubmit[sessionID] = attempts - 1
		}
	}
	s.mu.Unlock()

	submitted := 0
	for sessionID := range due {
		sess, err := s.sessions.Get(ctx, sessionID)
		if err != nil {
			s.logger.WarnContext(ctx, "skipping briefing submit, session lookup failed",
				slog.String("session_id", sessionID.String()),
				slog.String("error", err.Error()),
			)
			continue
		}
		for _, tmuxID := range sess.AgentTmuxIDs() {
			if err := s.tmux.SendKeys(ctx, tmuxID, "Enter"); err != nil {
				s.logger.WarnContext(ctx, "could not submit swarm briefing",
					slog.String("session_id", sess.ID.String()),
					slog.String("tmux_id", tmuxID),
					slog.String("error", err.Error()),
				)
				continue
			}
			submitted++
		}
	}

	return SubmitSwarmBriefingsResponse{Submitted: submitted}, nil
}

// requireSwarm loads a session and rejects it unless it is a swarm, so every
// swarm use-case fails the same recognisable way on an ordinary session.
func (s *SwarmService) requireSwarm(ctx context.Context, sessionID uuid.UUID) (domain.Session, error) {
	sess, err := s.sessions.Get(ctx, sessionID)
	if err != nil {
		return domain.Session{}, err
	}
	if !sess.IsSwarm() {
		return domain.Session{}, domain.ErrSessionNotASwarm
	}
	return sess, nil
}

// swarmHumanAuthor is the board author used for posts that came from the
// operator rather than an agent.
const swarmHumanAuthor = "human"

// swarmNudgePrompt is typed into an idle agent's pane when the board has moved.
// It is deliberately short and repeatable — the agent already knows the protocol
// from its bootstrap briefing.
const swarmNudgePrompt = "New messages on the swarm board. Read them from your cursor, " +
	"act on anything relevant to your part, and post back only if you have something substantive to add. " +
	"Keep any post short — one bold claim line, then evidence, under ~12 lines."

// swarmBootstrapPrompt is the one-off briefing that gives an agent its identity
// and points it at the contract. The contract carries the goal and the wire
// calls, which keeps this short enough to type into a pane reliably.
func swarmBootstrapPrompt(index, total int, descriptorPath string) string {
	return fmt.Sprintf(
		"You are %s of %d agents in a swarm. Read %s for the goal, the board protocol, and the "+
			"method and conventions — follow both. Read the board first, claim an axis nobody holds, "+
			"and post as %s. Try to break your own claims before posting them and say what survived. "+
			"Be terse: one bold claim line, then evidence, under ~12 lines. Silence is fine. "+
			"Claim a file on the board before editing it — you share a working directory. "+
			"When the goal is answered, post a verdict and stop.",
		domain.SwarmAgentAuthor(index), total, descriptorPath, domain.SwarmAgentAuthor(index),
	)
}
