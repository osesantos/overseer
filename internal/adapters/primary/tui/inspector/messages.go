package inspector

import "github.com/google/uuid"

type viewKind int

const (
	viewKindAgent viewKind = iota
	viewKindShell
	viewKindEditor
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

// RevealEditorMsg asks the inspector to reveal its gated Editor tab and make
// it the active view. The dashboard emits this when the user presses "e", in
// tandem with the AttachEditor call that opens nvim. The tab stays revealed
// until the selected session changes.
type RevealEditorMsg struct{}
