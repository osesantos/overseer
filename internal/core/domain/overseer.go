package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// OverseerRole identifies the author of an OverseerMessage.
type OverseerRole string

const (
	OverseerRoleUser  OverseerRole = "user"
	OverseerRoleAgent OverseerRole = "agent"
)

// OverseerMessage is a single entry in the Overseer chat history. Role
// disambiguates whether the user or the agent produced the content.
type OverseerMessage struct {
	Role      OverseerRole
	Content   string
	Timestamp time.Time
}

// OverseerActionType enumerates the actions the Overseer Agent may request.
type OverseerActionType string

const (
	OverseerActionSendPrompt OverseerActionType = "send_prompt"
)

// OverseerAction is an action intent parsed from the agent's response. It is
// never executed automatically; the TUI must present a confirmation dialog and
// wait for the user to approve before calling SessionService.SendAgentPrompt.
type OverseerAction struct {
	Type        OverseerActionType
	SessionID   uuid.UUID
	SessionName string
	Project     string
	Prompt      string
}

// OverseerResponse is the structured result of a single Chat call. Text is the
// human-readable content to append to the chat history. Action, when non-nil,
// is a parsed action that requires user confirmation before execution.
type OverseerResponse struct {
	Text   string
	Action *OverseerAction
}

// OverseerSessionContext is a runtime snapshot of one session, injected into
// every Chat call so the agent can make informed decisions without querying
// Overseer's internal state directly.
type OverseerSessionContext struct {
	SessionID   uuid.UUID
	SessionName string
	ProjectName string
	Branch      string
	AgentType   AgentType
	Status      AgentStatusKind
	PaneOutput  string // last N lines of the agent pane, ANSI-stripped
}

// OverseerAgentPort is the outbound port for the meta-agent backend.
// Implementations invoke an LLM (e.g. `claude -p`) and return a structured
// response. The context timeout controls the maximum wall-clock time allowed
// for the LLM call.
type OverseerAgentPort interface {
	// Chat sends userMsg to the underlying agent, enriched with a snapshot
	// of all current sessions as context. Returns a parsed response
	// containing plain text, an action, or both.
	Chat(ctx context.Context, userMsg string, sessions []OverseerSessionContext) (OverseerResponse, error)
}

// Overseer sentinel errors.
var (
	ErrOverseerAgentNotFound = errors.New("overseer: claude binary not found on PATH")
	ErrOverseerAgentFailed   = errors.New("overseer: agent invocation failed")
)
