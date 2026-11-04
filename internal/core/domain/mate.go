package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// MateRole identifies the author of an MateMessage.
type MateRole string

const (
	MateRoleUser   MateRole = "user"
	MateRoleAgent  MateRole = "agent"
	MateRoleSystem MateRole = "system" // operator command feedback
)

// MateMessage is a single entry in the First Mate chat history. Role
// disambiguates whether the user, the agent, or the system produced the
// content.
type MateMessage struct {
	Role      MateRole
	Content   string
	Timestamp time.Time
}

// MateActionType enumerates the actions the First Mate may request.
type MateActionType string

const (
	MateActionSendPrompt MateActionType = "send_prompt"
)

// MateAction is an action intent parsed from the agent's response. It is
// never executed automatically; the TUI must present a confirmation dialog and
// wait for the user to approve before calling SessionService.SendAgentPrompt.
type MateAction struct {
	Type        MateActionType
	SessionID   uuid.UUID
	SessionName string
	Project     string
	Prompt      string
}

// MateResponse is the structured result of a single Chat call. Text is the
// human-readable content to append to the chat history. Action, when non-nil,
// is a parsed action that requires user confirmation before execution.
type MateResponse struct {
	Text   string
	Action *MateAction
}

// MateSessionContext is a runtime snapshot of one session, injected into
// every Chat call so the agent can make informed decisions without querying
// Skipper's internal state directly.
type MateSessionContext struct {
	SessionID   uuid.UUID
	SessionName string
	ProjectName string
	Branch      string
	AgentType   AgentType
	Status      AgentStatusKind
	PaneOutput  string // last N lines of the agent pane, ANSI-stripped
}

// MateAgentPort is the outbound port for the meta-agent backend.
// Implementations invoke an LLM (e.g. `claude -p`) and return a structured
// response. The context timeout controls the maximum wall-clock time allowed
// for the LLM call.
type MateAgentPort interface {
	// Chat sends userMsg to the underlying agent, enriched with a snapshot
	// of all current sessions as context. Returns a parsed response
	// containing plain text, an action, or both.
	Chat(ctx context.Context, userMsg string, sessions []MateSessionContext) (MateResponse, error)
}

// First Mate sentinel errors.
var (
	ErrMateAgentNotFound = errors.New("mate: claude binary not found on PATH")
	ErrMateAgentFailed   = errors.New("mate: agent invocation failed")
)
