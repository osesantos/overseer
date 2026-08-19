package inspector

import (
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/components"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/core/service"
)

const tabStripHeight = 1

var (
	ToggleViewKeyBinding = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "toggle view"))
)

// Model is the dashboard's right panel. It owns a fixed set of preview views
// rendered as a tab strip on top of the active view's body. Only the active view
// polls; views become quiescent when inactive because the inspector stops
// forwarding messages to them.
//
// Which tabs are visible depends on the selected session. An ordinary session
// shows [Agent] [Shell]; a swarm shows [Agents Board] [Agent N/M] [Shell],
// because a swarm has no single agent pane and the board is the primary way to
// follow it. The Editor tab is appended to either set once the user presses "e"
// (RevealEditorMsg), and re-hidden when the selection changes.
//
// activeIx indexes views, not the visible subset, so message routing and
// rendering stay direct; visibleIxs() is the single place that decides which
// tabs exist and in what order.
type Model struct {
	views         []View
	activeIx      int
	editorVisible bool
	isSwarm       bool
	width         int
	height        int
	focused       bool
	styles        *styles.Styles
}

// Fixed positions within views. Editor stays last so RevealEditorMsg and the
// size loop need no knowledge of the swarm tabs.
const (
	ixAgent = iota
	ixBoard
	ixSwarmAgent
	ixShell
	ixEditor
)

func New(
	s *styles.Styles,
	sessionService service.SessionService,
	swarmService *service.SwarmService,
	previewRefreshInterval time.Duration,
) Model {
	views := make([]View, 5)
	views[ixAgent] = newAgentView(sessionService, s, previewRefreshInterval)
	views[ixBoard] = newBoardView(swarmService, s, previewRefreshInterval)
	views[ixSwarmAgent] = newSwarmAgentView(sessionService, s, previewRefreshInterval)
	views[ixShell] = newShellView(sessionService, s, previewRefreshInterval)
	views[ixEditor] = newEditorView(sessionService, s, previewRefreshInterval)

	return Model{
		views:    views,
		activeIx: ixAgent,
		styles:   s,
	}
}

// visibleIxs returns the views to render as tabs, in tab-strip order.
func (m Model) visibleIxs() []int {
	out := make([]int, 0, len(m.views))
	if m.isSwarm {
		out = append(out, ixBoard, ixSwarmAgent)
	} else {
		out = append(out, ixAgent)
	}
	out = append(out, ixShell)
	if m.editorVisible {
		out = append(out, ixEditor)
	}
	return out
}

func (m Model) visibleCount() int { return len(m.visibleIxs()) }

// CapturesInput reports whether the active view has taken over the keyboard —
// currently only the board's compose mode. The dashboard consults this before
// claiming any key of its own, so a message being typed is never cut short by a
// global shortcut.
func (m Model) CapturesInput() bool {
	board, ok := m.views[ixBoard].(*boardView)
	return ok && m.activeIx == ixBoard && board.CapturesInput()
}

// CanCompose reports whether the compose key would do something here — the board
// tab is active and the session is a swarm. The dashboard uses it to decide
// whether to forward that key, so "i" stays free everywhere else in the UI.
func (m Model) CanCompose() bool {
	board, ok := m.views[ixBoard].(*boardView)
	return ok && m.activeIx == ixBoard && board.isSwarm
}

// ActivePreviewKind reports which tmux pane the active tab is showing, so the
// dashboard can act on it (attach, kill) without matching on tab labels. The
// boolean is false for the Agents Board, which has no pane behind it — that is
// what stops "x" on the board silently killing the shell.
func (m Model) ActivePreviewKind() (service.PreviewKind, int, bool) {
	switch m.activeIx {
	case ixAgent:
		return service.PreviewKindAgent, 0, true
	case ixSwarmAgent:
		index := 1
		if sv, ok := m.views[ixSwarmAgent].(*streamView); ok {
			index = sv.agentIndex
		}
		return service.PreviewKindAgent, index, true
	case ixShell:
		return service.PreviewKindShell, 0, true
	case ixEditor:
		return service.PreviewKindEditor, 0, true
	default:
		return 0, 0, false
	}
}

func (m Model) Init() tea.Cmd {
	return m.views[m.activeIx].Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case shared.SessionSelectedMsg:
		m.isSwarm = msg.Session.IsSwarm()
		m.editorVisible = false
		m.activeIx = m.visibleIxs()[0]
		for i := range m.views {
			m.views[i].SetSession(msg.Session)
		}
		return m, m.views[m.activeIx].Init()

	case shared.SessionSelectionClearedMsg:
		m.isSwarm = false
		m.editorVisible = false
		m.activeIx = ixAgent
		for i := range m.views {
			m.views[i].SetSession(domain.Session{})
		}
		return m, nil

	case RevealEditorMsg:
		m.editorVisible = true
		m.activeIx = ixEditor
		return m, m.views[m.activeIx].Init()

	case tea.KeyPressMsg:
		// Compose mode owns every key while it is open, including tab.
		if m.CapturesInput() {
			return m.forwardToActive(msg)
		}
		if key.Matches(msg, ToggleViewKeyBinding) {
			vis := m.visibleIxs()
			pos := max(slices.Index(vis, m.activeIx), 0)
			m.activeIx = vis[(pos+1)%len(vis)]
			return m, m.views[m.activeIx].Init()
		}
		if key.Matches(msg, SwarmAgentNextKeyBinding, SwarmAgentPrevKeyBinding) {
			return m.cycleSwarmAgent(msg)
		}
		return m.forwardToActive(msg)

	case previewCapturedMsg:
		updated, cmd := m.views[m.activeIx].Update(msg)
		m.views[m.activeIx] = updated
		return m, cmd

	case swarmBoardLoadedMsg, swarmBoardPostedMsg:
		// Routed to the board explicitly rather than to activeIx: the board must
		// still absorb the tail of its polling chain after the user tabs away,
		// otherwise a stale fetch would land on whichever view is now active.
		updated, cmd := m.views[ixBoard].Update(msg)
		m.views[ixBoard] = updated
		return m, cmd

	case ForceRefreshMsg:
		// Trigger an immediate capture, superseding the current polling chain.
		// Init() increments the view's generation so in-flight msgs from the
		// old chain are dropped without re-scheduling.
		return m, m.views[m.activeIx].Init()
	}
	return m, nil
}

func (m Model) View() tea.View {
	innerW, _ := components.TitledPanelInnerSize(m.styles, m.focused, m.width, m.height)
	tabsRow := m.renderTabStrip(innerW)
	body := m.views[m.activeIx].Body()
	content := lipgloss.JoinVertical(lipgloss.Left, tabsRow, body)
	return components.PanelWithTitle(m.styles, content, "Preview", m.focused, m.width, m.height)
}

// forwardToActive hands a message to the active view and stores the result.
func (m Model) forwardToActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.views[m.activeIx].Update(msg)
	m.views[m.activeIx] = updated
	return m, cmd
}

// cycleSwarmAgent pages the swarm Agent tab to another pane. It only acts when
// that tab is active, so "[" and "]" stay free everywhere else.
func (m Model) cycleSwarmAgent(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.activeIx != ixSwarmAgent {
		return m, nil
	}
	view, ok := m.views[ixSwarmAgent].(*streamView)
	if !ok {
		return m, nil
	}

	delta := 1
	if key.Matches(msg, SwarmAgentPrevKeyBinding) {
		delta = -1
	}
	if !view.cycleAgent(delta) {
		return m, nil
	}
	return m, view.Init()
}

func (m Model) renderTabStrip(width int) string {
	vis := m.visibleIxs()
	labels := make([]string, 0, len(vis))
	// The visible set is not a prefix of views once swarm tabs are involved, so
	// this must iterate the index slice rather than a range.
	for _, ix := range vis {
		v := m.views[ix]
		if ix == m.activeIx {
			labels = append(labels, m.styles.Tab.Active.Render(v.Label()))
		} else {
			labels = append(labels, m.styles.Tab.Inactive.Render(v.Label()))
		}
	}
	row := strings.Join(labels, "")
	if pad := width - lipgloss.Width(row); pad > 0 {
		row += strings.Repeat(" ", pad)
	}
	return row
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	innerW, innerH := components.TitledPanelInnerSize(m.styles, m.focused, width, height)
	bodyH := max(innerH-tabStripHeight, 1)
	for i := range m.views {
		m.views[i].SetSize(innerW, bodyH)
	}
}

func (m *Model) SetFocus(focused bool) {
	m.focused = focused
}

func (m Model) ActiveViewLabel() string {
	return m.views[m.activeIx].Label()
}
