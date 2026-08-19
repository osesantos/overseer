package inspector

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/components"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/core/service"
)

// streamView previews a tmux pane that streams content in real time (Agent or
// Shell). Behaviour is identical between the two; only the targeted tmux
// session and the placeholder strings differ.
type streamView struct {
	kind            viewKind
	label           string
	previewKind     service.PreviewKind
	notReadyMessage string

	sessionID     uuid.UUID
	generation    int // incremented on each Init(); stale msgs from old chains are dropped
	width, height int
	content       string
	ready         bool
	err           error
	service       service.SessionService
	styles        *styles.Styles
	pollInterval  time.Duration

	// agentCount and agentIndex are only meaningful for the swarm agent view:
	// agentCount drives the "Agent 2/4" label and bounds paging, agentIndex is
	// the 1-based pane currently shown.
	agentCount int
	agentIndex int
}

func newAgentView(svc service.SessionService, s *styles.Styles, pollInterval time.Duration) *streamView {
	return &streamView{
		kind:            viewKindAgent,
		label:           "Agent",
		previewKind:     service.PreviewKindAgent,
		notReadyMessage: "Agent session not started — press ⏎ to launch",
		service:         svc,
		styles:          s,
		pollInterval:    pollInterval,
	}
}

func newShellView(svc service.SessionService, s *styles.Styles, pollInterval time.Duration) *streamView {
	return &streamView{
		kind:            viewKindShell,
		label:           "Shell",
		previewKind:     service.PreviewKindShell,
		notReadyMessage: "Shell session not started — press ⏎ to launch",
		service:         svc,
		styles:          s,
		pollInterval:    pollInterval,
	}
}

func newEditorView(svc service.SessionService, s *styles.Styles, pollInterval time.Duration) *streamView {
	return &streamView{
		kind:            viewKindEditor,
		label:           "Editor",
		previewKind:     service.PreviewKindEditor,
		notReadyMessage: "Editor not started — press e to launch nvim",
		service:         svc,
		styles:          s,
		pollInterval:    pollInterval,
	}
}


// newSwarmAgentView previews one pane of a swarm at a time. The board shows what
// agents choose to say; this tab is how the operator sees what an agent is
// actually doing when it stops making sense.
func newSwarmAgentView(svc service.SessionService, s *styles.Styles, pollInterval time.Duration) *streamView {
	return &streamView{
		kind:            viewKindSwarmAgent,
		label:           "Agent",
		previewKind:     service.PreviewKindAgent,
		notReadyMessage: "Agent pane not started",
		service:         svc,
		styles:          s,
		pollInterval:    pollInterval,
		agentIndex:      1,
	}
}

func (v *streamView) Label() string {
	if v.kind == viewKindSwarmAgent && v.agentCount > 0 {
		return fmt.Sprintf("%s %d/%d", v.label, v.agentIndex, v.agentCount)
	}
	return v.label
}

// cycleAgent moves the swarm agent view to another pane, wrapping at both ends.
// Returns false when the view has nothing to cycle, so the caller can leave the
// keypress unhandled rather than pointlessly restarting a poll.
func (v *streamView) cycleAgent(delta int) bool {
	if v.kind != viewKindSwarmAgent || v.agentCount <= 1 {
		return false
	}
	next := ((v.agentIndex-1+delta)%v.agentCount + v.agentCount) % v.agentCount
	v.agentIndex = next + 1
	v.content = ""
	v.ready = false
	v.err = nil
	return true
}

func (v *streamView) Init() tea.Cmd {
	v.generation++
	return v.capture()
}

func (v *streamView) Update(msg tea.Msg) (View, tea.Cmd) {
	captured, ok := msg.(previewCapturedMsg)
	if !ok {
		return v, nil
	}
	if captured.kind != v.kind {
		return v, nil
	}
	if captured.sessionID != v.sessionID {
		return v, nil
	}
	if v.sessionID == uuid.Nil {
		return v, nil
	}
	// Drop messages produced by a superseded capture chain. This prevents the
	// old polling goroutine from scheduling a duplicate chain after Init() was
	// called mid-flight (e.g. on a force refresh or session switch).
	if captured.generation != v.generation {
		return v, nil
	}
	if captured.err != nil {
		v.err = captured.err
	} else {
		v.content = captured.content
		v.ready = captured.sessionReady
		v.err = nil
	}
	return v, v.scheduleNext()
}

func (v *streamView) Body() string {
	if v.sessionID == uuid.Nil {
		return components.CenteredContent(v.styles, v.styles.EmptyState.Title.Render("Select a session to preview"), v.width, v.height)
	}
	if v.err != nil {
		return components.CenteredContent(v.styles, v.styles.EmptyState.Title.Render("Preview error: "+v.err.Error()), v.width, v.height)
	}
	if !v.ready {
		return components.CenteredContent(v.styles, v.styles.EmptyState.Title.Render(v.notReadyMessage), v.width, v.height)
	}
	return v.content
}

func (v *streamView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

func (v *streamView) SetSession(sess domain.Session) {
	v.sessionID = sess.ID
	v.agentCount = sess.AgentCount()
	v.agentIndex = 1
	v.content = ""
	v.ready = false
	v.err = nil
}

// capture builds a Cmd that fetches the current pane content for the view's
// sessionID. The sessionID, width, and height are captured at Cmd creation
// time so messages produced after a session change or resize are dropped by
// the staleness check in Update, terminating the old polling chain. Width
// and height are forwarded so the service can resize the tmux pane before
// capturing, forcing the agent app to redraw at the preview's canvas size.
func (v *streamView) capture() tea.Cmd {
	sessID := v.sessionID
	svc := v.service
	kind := v.kind
	previewKind := v.previewKind
	width := v.width
	height := v.height
	gen := v.generation
	agentIndex := v.agentIndex
	return func() tea.Msg {
		if sessID == uuid.Nil {
			return previewCapturedMsg{kind: kind, sessionID: sessID, generation: gen}
		}
		resp, err := svc.PreviewSession(context.Background(), service.PreviewSessionRequest{
			ID:         sessID,
			Kind:       previewKind,
			AgentIndex: agentIndex,
			Width:      width,
			Height:     height,
		})
		return previewCapturedMsg{
			kind:         kind,
			sessionID:    sessID,
			generation:   gen,
			content:      resp.Content,
			sessionReady: resp.SessionReady,
			err:          err,
		}
	}
}

func (v *streamView) scheduleNext() tea.Cmd {
	next := v.capture()
	return tea.Tick(v.pollInterval, func(time.Time) tea.Msg {
		return next()
	})
}
