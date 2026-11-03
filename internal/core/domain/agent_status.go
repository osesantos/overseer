package domain

import (
	"context"
	"errors"
	"maps"
	"time"

	"github.com/google/uuid"
)

// AgentType identifies which agent runs in a session. It is set on Session
// at creation time, copied from the chosen Launcher. Detection strategies
// route by AgentType so a session running Claude Code is parsed differently
// than one running OpenCode.
type AgentType string

const (
	AgentTypeUnknown    AgentType = "unknown"
	AgentTypeClaudeCode AgentType = "claude-code"
	AgentTypeOpenCode   AgentType = "opencode"
)

// AgentStatusKind enumerates the runtime states an agent can be in. The
// "Dead" status means the agent's tmux session is missing or crashed; the
// shell session is ignored for status purposes. "Unknown" is the safe
// fallback when the detector cannot classify the state — never a hard
// failure.
type AgentStatusKind string

const (
	AgentStatusUnknown AgentStatusKind = "unknown"
	AgentStatusRunning AgentStatusKind = "running"
	AgentStatusWaiting AgentStatusKind = "waiting"
	// AgentStatusAsking marks an agent that finished its turn by asking the
	// operator something in prose ("Shall I…?", "say yes to write it"). Unlike
	// Waiting, no dialog blocks it; it only looks idle.
	AgentStatusAsking AgentStatusKind = "asking"
	AgentStatusIdle   AgentStatusKind = "idle"
	AgentStatusDead   AgentStatusKind = "dead"
	// AgentStatusSwarm marks a session whose agents are not tracked individually.
	// A swarm has one pane per agent, and this API carries a single status per
	// session, so there is nothing honest to report — but it is not Unknown
	// either: Unknown means detection failed, whereas this means detection does
	// not apply.
	AgentStatusSwarm AgentStatusKind = "swarm"
	// AgentStatusReviewPending marks a review session whose PR holds a review the
	// operator staged on GitHub but has not submitted. It is never detected from a
	// pane; WithPendingReviews lays it over the detected status.
	AgentStatusReviewPending AgentStatusKind = "review-pending"
)

// outranksPendingReview reports whether k beats a pending review: the agent is
// still working or blocked on a dialog. A prose question (Asking) does not: once
// the review is staged, submitting it is the operator's next move.
func outranksPendingReview(k AgentStatusKind) bool {
	switch k {
	case AgentStatusRunning, AgentStatusWaiting, AgentStatusSwarm:
		return true
	}
	return false
}

// WithPendingReviews returns a copy of statuses in which every review session
// whose PR carries an unsubmitted review by the viewer reads
// AgentStatusReviewPending, unless its agent is busy.
func WithPendingReviews(statuses map[uuid.UUID]AgentStatus, sessions []Session, inbox ReviewInbox) map[uuid.UUID]AgentStatus {
	pending := make(map[PullRequestRef]bool)
	for _, pr := range inbox.PullRequests {
		if pr.ViewerReviewPending {
			pending[pr.Ref()] = true
		}
	}
	out := make(map[uuid.UUID]AgentStatus, len(statuses))
	maps.Copy(out, statuses)
	for _, sess := range sessions {
		current := statuses[sess.ID]
		if !sess.IsReview() || !pending[sess.ReviewPR] || outranksPendingReview(current.Kind) {
			continue
		}
		out[sess.ID] = AgentStatus{Kind: AgentStatusReviewPending, DetectedAt: current.DetectedAt, Source: "github/pending-review"}
	}
	return out
}

// AgentStatus is a value object describing what an agent is doing right
// now. DetectedAt records when the snapshot was taken; Source identifies
// the detector that produced it (e.g. "claude-code/pane-parser"); Reason
// is an optional human-readable hint (e.g. "matched 'esc to interrupt'").
type AgentStatus struct {
	Kind       AgentStatusKind
	DetectedAt time.Time
	Source     string
	Reason     string
}

// AgentStatusDetector is the port for per-(agent, strategy) detection
// implementations. Examples: ClaudeCodePaneDetector, ClaudeCodeHookDetector,
// OpenCodePaneDetector. Per-agent detectors only resolve Running / Waiting
// / Idle / Unknown; Dead detection lives in the service layer so detector
// authors never need to think about process liveness.
type AgentStatusDetector interface {
	AgentType() AgentType
	Detect(ctx context.Context, session Session) (AgentStatus, error)
}

// AgentStatusDetectorRegistry resolves an AgentType to its detector. Used
// by AgentStatusService.PollAll to route each session to the right
// detector implementation.
type AgentStatusDetectorRegistry interface {
	DetectorFor(agentType AgentType) (AgentStatusDetector, bool)
}

// AgentStatus sentinel errors.
var (
	ErrAgentTypeRequired = errors.New("agent type is required")
	ErrAgentTypeUnknown  = errors.New("agent type is not recognized")
)
