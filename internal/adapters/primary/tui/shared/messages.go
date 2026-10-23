package shared

import (
	"os/exec"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/core/domain"
)

// SwarmBootstrappedMsg reports the outcome of briefing a new swarm's agents.
type SwarmBootstrappedMsg struct {
	SessionID uuid.UUID
	Err       error
}

// SessionCreatedMsg reports a newly created session. SwarmGoal is set only for
// swarm sessions and carries the operator's initial prompt through to
// bootstrapping — the goal is not persisted on the Session, it becomes the first
// message on the swarm's board.
type SessionCreatedMsg struct {
	Session   domain.Session
	SwarmGoal string
}

type SessionSelectedMsg struct{ Session domain.Session }

type SessionSelectionClearedMsg struct{}

type SessionsLoadedMsg struct {
	Sessions []domain.Session
	Err      error
}

type SessionReorderedMsg struct {
	Sessions []domain.Session
	FocusID  string
	Err      error
}

type SessionLabelCycledMsg struct {
	Sessions []domain.Session
	FocusID  string
	Err      error
}

type NewSessionPopupCloseMsg struct{}

type SessionCreateErrMsg struct{ Err error }

type SessionAttachReadyMsg struct {
	Command *exec.Cmd
	Err     error
}

type SessionAttachedMsg struct{ Err error }

type AgentEnterSentMsg struct{ Err error }

type PreviewSessionKilledMsg struct{ Err error }

type KillPreviewPopupCloseMsg struct{}

type SessionDeleteRequestedMsg struct{ Session domain.Session }

type SessionDeletedMsg struct{}

type SessionDeleteErrMsg struct{ Err error }

type NewSessionDeletePopupCloseMsg struct{}

type SessionRenameRequestedMsg struct{ Session domain.Session }

type ProjectRenameRequestedMsg struct {
	ProjectID   uuid.UUID
	CurrentName string
}

type SessionRenamedMsg struct{ Session domain.Session }

type SessionRenameErrMsg struct{ Err error }

type ProjectRenamedMsg struct{ Project domain.Project }

type ProjectRenameErrMsg struct{ Err error }

type RenamePopupCloseMsg struct{}

type HelpPopupCloseMsg struct{}

type ProjectsLoadedMsg struct {
	Projects []domain.Project
	Err      error
}

type ProjectRegisteredMsg struct{ Project domain.Project }

type ProjectRegisterErrMsg struct{ Err error }

type JobsTickMsg struct{ JobID string }

type JobsBatchMsg struct{ Cmds []tea.Cmd }

// OmarchyPaletteMsg carries the palette read from the active Omarchy theme; nil when Omarchy is absent.
type OmarchyPaletteMsg struct{ Palette map[string]string }

// PullRequestsLoadedMsg carries one poll of the review inbox. The dashboard
// decides whether to auto-review it (AutoReviewCompletedMsg follows when it does).
type PullRequestsLoadedMsg struct {
	Inbox domain.ReviewInbox
	Err   error
}

// PullRequestOpenMsg is emitted by the PRs page when Enter lands on a PR: jump
// to its review session, starting one first when none exists.
type PullRequestOpenMsg struct{ PR domain.PullRequest }

// PullRequestReviewMsg is emitted by the PRs page to start a review session by
// hand, bypassing the auto-review eligibility rules (bots, own PRs, reviewed).
type PullRequestReviewMsg struct{ PR domain.PullRequest }

// PullRequestOpenInBrowserMsg is emitted by the PRs page, or by o on the sessions
// page, to open the PR's GitHub page in the browser.
type PullRequestOpenInBrowserMsg struct{ PR domain.PullRequest }

// PullRequestBrowserOpenedMsg reports the outcome of opening a PR in the browser.
type PullRequestBrowserOpenedMsg struct{ Err error }

// AutoReviewToggleMsg is emitted by the PRs page to flip auto-review on or off
// for the rest of the session; the config value is only the starting point.
type AutoReviewToggleMsg struct{}

// AutoReviewSavedMsg reports whether the toggle was persisted.
type AutoReviewSavedMsg struct{ Err error }

// AutoReviewCompletedMsg reports the review sessions one inbox poll started.
type AutoReviewCompletedMsg struct {
	Created []domain.Session
	Err     error
}

// PullRequestDismissMsg is emitted by the PRs page to hide a PR from the inbox
// for good.
type PullRequestDismissMsg struct{ PR domain.PullRequest }

// PullRequestDismissedMsg reports the outcome of a dismissal.
type PullRequestDismissedMsg struct {
	Ref domain.PullRequestRef
	Err error
}

// ReviewSessionStartedMsg reports a review session started from the PRs page.
type ReviewSessionStartedMsg struct {
	Session domain.Session
	Err     error
}

// ReviewSessionReapedMsg reports the automatic deletion of a review session whose
// PR was merged or closed.
type ReviewSessionReapedMsg struct {
	Session domain.Session
	Err     error
}

// PageCloseMsg asks the dashboard to return to the sessions page.
type PageCloseMsg struct{}

// SessionFocusRequestedMsg moves the session list cursor to the given session
// and selects it, as if the operator had navigated there.
type SessionFocusRequestedMsg struct{ SessionID uuid.UUID }

type PRStatusUpdatedMsg struct {
	SessionID uuid.UUID
	PR        domain.PullRequest
	Err       error
}

type AgentStatusesUpdatedMsg struct {
	Statuses map[uuid.UUID]domain.AgentStatus
	Err      error
}

type BranchesLoadedMsg struct {
	ProjectID     uuid.UUID
	Branches      []domain.BranchInfo
	DefaultBranch string
	LoadedAt      time.Time
	Err           error
}

type BranchCacheTickMsg struct{}

type ProjectCurrentBranchLoadedMsg struct {
	ProjectID uuid.UUID
	Branch    string
	Err       error
}

// ProjectDiscoveryCompletedMsg is emitted by the dashboard when the startup
// repo-discovery background job finishes. Count is the number of repositories
// newly registered during the scan. MissingPaths lists every configured
// discovery path that does not exist on disk; an empty slice means all paths
// were accessible.
type ProjectDiscoveryCompletedMsg struct {
	Count        int
	MissingPaths []string
	Err          error
}

// --- Overseer chat panel messages ---

// OverseerTogglePanelMsg is emitted when the user presses ctrl+o. The
// dashboard uses it to flip chatPanelVisible and resize child panels.
type OverseerTogglePanelMsg struct{}

// OverseerSubmitMsg is emitted by the chat panel when the user presses Enter.
// The dashboard receives it, attaches a live session context snapshot, and
// fires the OverseerService.Chat command. Keeping the service call in the
// dashboard ensures it always reads from the current cachedSessions rather
// than from a stale closure captured at panel construction time.
type OverseerSubmitMsg struct {
	UserMessage string
}

// OverseerChatResponseMsg carries the result of a completed Chat call from
// the OverseerService. The overseer panel appends Text to history and, when
// Action is non-nil, asks the dashboard to open the confirmation popup.
type OverseerChatResponseMsg struct {
	Text   string
	Action *domain.OverseerAction
	Err    error
}

// OverseerConfirmActionMsg is emitted by the confirmation popup when the user
// presses y/Enter. The dashboard forwards it to the session service as a
// SendAgentPrompt call.
type OverseerConfirmActionMsg struct {
	Action domain.OverseerAction
}

// OverseerCancelActionMsg is emitted by the confirmation popup when the user
// presses Esc/n. The dashboard closes the popup without sending anything.
type OverseerCancelActionMsg struct{}

// OverseerPromptSentMsg is emitted after SendAgentPrompt completes. The
// overseer panel uses it to append a system note to the chat history.
type OverseerPromptSentMsg struct {
	SessionName string
	Err         error
}

// OverseerCommandMsg is emitted by the chat panel when the user submits a
// line that starts with '/'. The dashboard parses the raw text, resolves
// session names against cachedSessions, and executes the command without
// calling the LLM.
type OverseerCommandMsg struct {
	Raw string
}

// OverseerCommandResultMsg carries the outcome of an operator command back
// to the chat panel, which renders it as a dimmed system message.
type OverseerCommandResultMsg struct {
	Text    string
	IsError bool
}

