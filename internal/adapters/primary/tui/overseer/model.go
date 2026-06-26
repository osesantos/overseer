package overseer

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
)

const (
	// inputPrompt is the prefix shown in the text input.
	inputPrompt = "> "
	// inputHeight is the number of rows the text input row occupies.
	inputHeight = 1
	// borderFrameH is the vertical frame consumed by the panel border.
	borderFrameH = 2
)

// SessionSnapshot is a lightweight view into a session's static state that
// the dashboard builds at submit time. PaneOutput is intentionally absent:
// overseerChatCmd fetches it fresh via PreviewSession inside the request
// goroutine so it is never stale.
type SessionSnapshot struct {
	SessionID   uuid.UUID
	SessionName string
	ProjectName string
	Branch      string
	AgentType   domain.AgentType
	Status      domain.AgentStatusKind
}

// Model is the Overseer chat panel. It holds a scrollable message history
// (viewport), a single-line text input, and a thinking-spinner shown while
// the agent is processing. It is embedded directly in dashboard.Model and
// becomes visible when the user presses ctrl+o.
//
// The panel itself never calls the OverseerService directly. On submit it
// emits shared.OverseerSubmitMsg; the dashboard receives that message,
// attaches a live session context snapshot, and fires the service command.
// This avoids the stale-closure bug that arises when service/snapshot
// references are captured at panel construction time.
type Model struct {
	// history accumulates all chat messages for the viewport.
	history []domain.OverseerMessage

	viewport viewport.Model
	input    textinput.Model
	spinner  spinner.Model
	thinking bool
	width    int
	height   int
	styles   *styles.Styles
}

// New constructs a Model.
func New(s *styles.Styles) Model {
	ti := textinput.New()
	ti.Placeholder = "Ask the Overseer Agent…"
	ti.Prompt = inputPrompt
	ti.SetStyles(s.Form.Input)
	ti.Focus()

	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot))

	vp := viewport.New()

	return Model{
		viewport: vp,
		input:    ti,
		spinner:  sp,
		styles:   s,
	}
}

// SetSize updates the panel dimensions. Called by the dashboard whenever the
// terminal is resized or the panel is toggled open.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.input.SetWidth(width - len(inputPrompt) - borderFrameH)

	vpHeight := max(height-borderFrameH-inputHeight, 1)
	m.viewport.SetWidth(width - borderFrameH)
	m.viewport.SetHeight(vpHeight)
	m.refreshViewport()
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.thinking {
			return m, nil
		}
		switch msg.String() {
		case "enter":
			return m.submit()
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd

	case spinner.TickMsg:
		if m.thinking {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case shared.OverseerChatResponseMsg:
		m.thinking = false
		if msg.Err != nil {
			m.appendMessage(domain.OverseerMessage{
				Role:      domain.OverseerRoleAgent,
				Content:   "Error: " + msg.Err.Error(),
				Timestamp: time.Now(),
			})
			return m, nil
		}
		m.appendMessage(domain.OverseerMessage{
			Role:      domain.OverseerRoleAgent,
			Content:   msg.Text,
			Timestamp: time.Now(),
		})
		return m, nil

	case shared.OverseerPromptSentMsg:
		var note string
		if msg.Err != nil {
			note = "Failed to send prompt to " + msg.SessionName + ": " + msg.Err.Error()
		} else {
			note = "Prompt sent to " + msg.SessionName + "."
		}
		m.appendMessage(domain.OverseerMessage{
			Role:      domain.OverseerRoleAgent,
			Content:   note,
			Timestamp: time.Now(),
		})
		return m, nil
	}

	return m, nil
}

func (m Model) View() tea.View {
	s := m.styles
	border := s.Border.Focused.
		Width(m.width - borderFrameH).
		Height(m.height - borderFrameH)

	var b strings.Builder
	b.WriteString(m.viewport.View())
	b.WriteByte('\n')
	if m.thinking {
		b.WriteString(s.Chat.ThinkingPrefix.Render(m.spinner.View()))
		b.WriteString(s.Chat.ThinkingText.Render(" thinking…"))
	} else {
		b.WriteString(m.input.View())
	}

	return tea.NewView(border.Render(b.String()))
}

// appendMessage adds msg to history and refreshes the viewport.
func (m *Model) appendMessage(msg domain.OverseerMessage) {
	m.history = append(m.history, msg)
	m.refreshViewport()
}

// AppendMessage is the exported version used by the dashboard to inject
// system notices (e.g. after a confirmation is accepted).
func (m *Model) AppendMessage(msg domain.OverseerMessage) {
	m.appendMessage(msg)
}

// refreshViewport re-renders the message history into the viewport and scrolls
// to the bottom.
func (m *Model) refreshViewport() {
	s := m.styles
	var b strings.Builder
	for _, msg := range m.history {
		switch msg.Role {
		case domain.OverseerRoleUser:
			b.WriteString(s.Chat.UserLabel.Render("You  "))
			b.WriteString(s.Chat.UserText.Render(msg.Content))
		case domain.OverseerRoleAgent:
			b.WriteString(s.Chat.AgentLabel.Render("Agent"))
			b.WriteString(s.Chat.AgentText.Render(" " + msg.Content))
		}
		b.WriteByte('\n')
	}
	m.viewport.SetContent(b.String())
	m.viewport.GotoBottom()
}

// submit reads the current input value, appends a user message, starts the
// spinner, and emits OverseerSubmitMsg for the dashboard to handle.
func (m Model) submit() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.input.Value())
	if text == "" {
		return m, nil
	}
	m.input.SetValue("")
	m.appendMessage(domain.OverseerMessage{
		Role:      domain.OverseerRoleUser,
		Content:   text,
		Timestamp: time.Now(),
	})
	m.thinking = true
	return m, tea.Batch(
		m.spinner.Tick,
		shared.Emit(shared.OverseerSubmitMsg{UserMessage: text}),
	)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// panelBorder returns a copy of the focused border style sized to the given
// outer dimensions. Exported for tests.
func panelBorder(s *styles.Styles, width, height int) lipgloss.Style {
	return s.Border.Focused.Width(width - borderFrameH).Height(height - borderFrameH)
}
