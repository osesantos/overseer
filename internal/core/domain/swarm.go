package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// SwarmMinAgents and SwarmMaxAgents bound how many agents a swarm session
	// may run. The floor is 2 because a "swarm" of one is just an ordinary
	// session; the ceiling keeps the board readable, caps the token cost of a
	// single goal, and matches the number of distinct author colours the TUI
	// can render.
	SwarmMinAgents = 2
	SwarmMaxAgents = 8

	swarmAuthorMaxLen  = 100
	swarmContentMaxLen = 8000
)

// SwarmRole discriminates who produced a SwarmMessage. It drives how the TUI
// renders the post and lets the nudge logic tell an agent's own contribution
// apart from an instruction typed by the human.
type SwarmRole string

const (
	SwarmRoleAgent  SwarmRole = "agent"
	SwarmRoleHuman  SwarmRole = "human"
	SwarmRoleSystem SwarmRole = "system"
)

// SwarmMessage is one post on a swarm session's communication board.
//
// Seq is a per-session monotonic cursor assigned by the repository on append —
// NewSwarmMessage deliberately leaves it at zero. Both the agents and the TUI
// use Seq as a "since" watermark so they fetch only what they have not seen,
// which is what keeps a busy board cheap to poll.
type SwarmMessage struct {
	Seq       int
	SessionID uuid.UUID
	Author    string
	Role      SwarmRole
	Content   string
	CreatedAt time.Time
}

// NewSwarmMessage constructs a validated board post. Author and Content are
// trimmed; both are required. Seq is left zero for the repository to assign.
func NewSwarmMessage(sessionID uuid.UUID, author string, role SwarmRole, content string) (SwarmMessage, error) {
	author = strings.TrimSpace(author)
	content = strings.TrimSpace(content)

	if sessionID == uuid.Nil {
		return SwarmMessage{}, ErrSwarmMessageEmptySessionID
	}
	if author == "" {
		return SwarmMessage{}, ErrSwarmMessageEmptyAuthor
	}
	if len(author) > swarmAuthorMaxLen {
		return SwarmMessage{}, ErrSwarmMessageAuthorTooLong
	}
	if !role.valid() {
		return SwarmMessage{}, ErrSwarmMessageUnknownRole
	}
	if content == "" {
		return SwarmMessage{}, ErrSwarmMessageEmptyContent
	}
	if len(content) > swarmContentMaxLen {
		return SwarmMessage{}, ErrSwarmMessageContentTooLong
	}

	return SwarmMessage{
		SessionID: sessionID,
		Author:    author,
		Role:      role,
		Content:   content,
		CreatedAt: time.Now(),
	}, nil
}

func (r SwarmRole) valid() bool {
	switch r {
	case SwarmRoleAgent, SwarmRoleHuman, SwarmRoleSystem:
		return true
	}
	return false
}

// SwarmAgentAuthor returns the canonical board author name for the agent at the
// given 1-based index ("agent-3"). Agents are told this name at bootstrap and
// sign their posts with it, so the board can attribute a post and the nudge
// logic can skip the author when waking everyone else.
func SwarmAgentAuthor(index int) string {
	return fmt.Sprintf("agent-%d", index)
}

// SwarmDescriptor is the bootstrap contract handed to a swarm's agents: where
// the board lives, who else is on it, and what they are collectively trying to
// achieve. Agents read it once at startup, which keeps the prompt Overseer has
// to inject into each pane short.
//
// It carries only semantics — how the contract is rendered on disk, and which
// wire calls an agent should make, is the adapter's business.
type SwarmDescriptor struct {
	SessionID  uuid.UUID
	BoardURL   string
	AgentCount int
	Roster     []string
	Goal       string
}

// NewSwarmDescriptor constructs a validated bootstrap descriptor and derives the
// roster from agentCount, so the roster can never disagree with the number of
// panes Overseer actually spawned.
func NewSwarmDescriptor(sessionID uuid.UUID, boardURL string, agentCount int, goal string) (SwarmDescriptor, error) {
	boardURL = strings.TrimSpace(boardURL)
	goal = strings.TrimSpace(goal)

	if sessionID == uuid.Nil {
		return SwarmDescriptor{}, ErrSwarmMessageEmptySessionID
	}
	if boardURL == "" {
		return SwarmDescriptor{}, ErrSwarmDescriptorEmptyBoardURL
	}
	if agentCount < SwarmMinAgents || agentCount > SwarmMaxAgents {
		return SwarmDescriptor{}, ErrSessionSwarmSizeOutOfRange
	}
	if goal == "" {
		return SwarmDescriptor{}, ErrSwarmDescriptorEmptyGoal
	}

	roster := make([]string, 0, agentCount)
	for index := 1; index <= agentCount; index++ {
		roster = append(roster, SwarmAgentAuthor(index))
	}

	return SwarmDescriptor{
		SessionID:  sessionID,
		BoardURL:   boardURL,
		AgentCount: agentCount,
		Roster:     roster,
		Goal:       goal,
	}, nil
}

// Swarm ports.

// SwarmBoardRepository persists the message board of a swarm session. Append is
// the only writer, so it owns Seq assignment — implementations must guarantee
// that Seq is strictly increasing per session even under concurrent appends
// from several agent processes.
type SwarmBoardRepository interface {
	// Append stores msg, assigning the next Seq for its session, and returns
	// the stored message.
	Append(ctx context.Context, msg SwarmMessage) (SwarmMessage, error)
	// ListSince returns the session's messages with Seq greater than since,
	// in ascending Seq order. A since of zero returns the whole board.
	ListSince(ctx context.Context, sessionID uuid.UUID, since int) ([]SwarmMessage, error)
	// Count reports how many messages the session's board holds. Used to trip
	// the runaway-chatter circuit breaker.
	Count(ctx context.Context, sessionID uuid.UUID) (int, error)
	// Purge removes the session's board entirely. Called when the session is
	// deleted.
	Purge(ctx context.Context, sessionID uuid.UUID) error
}

// SwarmDescriptorWriter persists the bootstrap descriptor where a swarm's agents
// can read it. Kept separate from SwarmBoardRepository because bootstrapping
// happens once per session while the board is written continuously.
type SwarmDescriptorWriter interface {
	Write(ctx context.Context, desc SwarmDescriptor) error
}

// Swarm sentinel errors.
var (
	ErrSwarmMessageEmptySessionID = errors.New("swarm message session id cannot be empty")
	ErrSwarmMessageEmptyAuthor    = errors.New("swarm message author cannot be empty")
	ErrSwarmMessageAuthorTooLong  = errors.New("swarm message author exceeds 100 characters")
	ErrSwarmMessageEmptyContent   = errors.New("swarm message content cannot be empty")
	ErrSwarmMessageContentTooLong = errors.New("swarm message content exceeds 8000 characters")
	ErrSwarmMessageUnknownRole    = errors.New("swarm message role must be agent, human or system")
	ErrSwarmBoardCapReached       = errors.New("swarm board message cap reached")
	ErrSessionNotASwarm           = errors.New("session is not a swarm")

	ErrSwarmDescriptorEmptyBoardURL = errors.New("swarm descriptor board url cannot be empty")
	ErrSwarmDescriptorEmptyGoal     = errors.New("swarm descriptor goal cannot be empty")
)
