package inspector

import (
	"errors"

	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/core/domain"
)

// errSwarmDisabled is returned when the board is used while swarm mode is off,
// which means no SwarmService was wired in.
var errSwarmDisabled = errors.New("swarm mode is disabled")

type viewKind int

const (
	viewKindAgent viewKind = iota
	viewKindShell
	viewKindEditor
	// viewKindSwarmAgent previews one pane of a swarm, selected by index. It is
	// a separate kind from viewKindAgent so a capture meant for the swarm tab
	// can never be consumed by the single-agent tab or vice versa.
	viewKindSwarmAgent
	// viewKindBoard is the swarm's Agents Board — a message transcript rather
	// than a tmux pane, so it does not produce previewCapturedMsg at all.
	viewKindBoard
)

// previewCapturedMsg carries the result of a single tmux capture-pane call.
// kind identifies which view produced the message so inactive views ignore
// messages that arrive after a view switch. sessionID lets a view drop
// messages that arrived after the selected session changed, breaking the
// stale polling chain. generation lets a view drop messages produced by a
// superseded capture chain (e.g. from before a ForceRefreshMsg).
type previewCapturedMsg struct {
	kind         viewKind
	sessionID    uuid.UUID
	generation   int
	content      string
	sessionReady bool
	err          error
}

// ForceRefreshMsg asks the inspector to perform an immediate preview capture
// without waiting for the next scheduled poll tick. Callers (e.g. the
// dashboard after a successful SendAgentPrompt) emit this so the user sees
// the sent prompt land in the preview pane right away.
type ForceRefreshMsg struct{}

// swarmBoardLoadedMsg carries a delta of board messages. messages holds only
// entries newer than the cursor the request was made with; latestSeq is the
// cursor to send next. generation and sessionID play the same staleness role as
// in previewCapturedMsg.
type swarmBoardLoadedMsg struct {
	sessionID  uuid.UUID
	generation int
	messages   []domain.SwarmMessage
	latestSeq  int
	err        error
}

// swarmBoardPostedMsg reports the outcome of an operator post so the board can
// surface a failure and pull the new message in immediately.
type swarmBoardPostedMsg struct {
	sessionID uuid.UUID
	err       error
}

// RevealEditorMsg asks the inspector to reveal its gated Editor tab and make
// it the active view. The dashboard emits this when the user presses "e", in
// tandem with the AttachEditor call that opens nvim. The tab stays revealed
// until the selected session changes.
type RevealEditorMsg struct{}
