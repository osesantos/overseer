package shared

import (
	"os/exec"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/core/domain"
)

type SessionCreatedMsg struct{ Session domain.Session }

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

type SessionEditorLaunchedMsg struct{ Err error }

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
