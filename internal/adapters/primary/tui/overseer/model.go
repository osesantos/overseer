package overseer

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
)

const (
	// agentPrompt is the input prefix shown in Agent mode.
	agentPrompt = "» "
	// operatorPrompt is the input prefix shown in Operator mode (line starts with /).
	operatorPrompt = "$ "
	// inputHeight is the number of rows the text input row occupies.
	inputHeight = 1
	// borderFrameH is the vertical frame consumed by the panel border.
	borderFrameH = 2
	// borderSideW is the two columns the left and right border glyphs occupy.
	//
	// lipgloss treats a bordered style's Width as the *total* including the
	// border, so the usable interior is Width-2. The viewport used to be sized to
	// the full Width, leaving it two columns wider than the box it draws into —
	// which quietly wrapped any long agent line and became obvious once messages
	// gained a full-width separator.
	borderSideW = 2
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
// The panel itself never calls OverseerService directly. On submit it emits
// either OverseerSubmitMsg (agent mode) or OverseerCommandMsg (operator mode,
// any line starting with '/'); the dashboard handles execution in both cases.
type Model struct {
	// history accumulates all chat messages for the viewport.
	history []domain.OverseerMessage
	// rendered holds each history entry already converted to styled text, index
	// for index. Markdown rendering is far too costly to redo for the whole
	// transcript on every incoming message, so each entry is rendered once and
	// only re-rendered when a resize changes the wrap width.
	rendered []string

	viewport viewport.Model
	input    textinput.Model
	spinner  spinner.Model
	markdown styles.Markdown
	thinking bool
	width    int
	height   int
	styles   *styles.Styles
}

// New constructs a Model.
func New(s *styles.Styles) Model {
	ti := textinput.New()
	ti.Placeholder = "Ask the Overseer Agent… or type /help for commands"
	ti.Prompt = agentPrompt
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
	// Prompt width depends on current mode; use the longer one to be safe.
	m.input.SetWidth(max(width-len(operatorPrompt)-borderFrameH-borderSideW, 1))

	vpHeight := max(height-borderFrameH-inputHeight, 1)
	vpWidth := max(width-borderFrameH-borderSideW, 1)
	m.viewport.SetWidth(vpWidth)
	m.viewport.SetHeight(vpHeight)

	// Markdown is wrapped at a fixed width, so a width change invalidates every
	// cached body. Height changes do not.
	if m.markdown.Width() != vpWidth {
		m.markdown = m.styles.NewMarkdown(vpWidth)
		m.rerenderAll()
	}
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
		if key.Matches(msg, submitKeyBinding) {
			return m.submit()
		}
		if key.Matches(msg, scrollUpKeyBinding, scrollDownKeyBinding, scrollPageUpBinding, scrollPageDownBinding) {
			// Route scroll keys to the viewport; do not pass to text input.
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		// All other keys go to the text input.
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.syncPrompt()
		return m, cmd

	case tea.PasteMsg:
		// Bracketed paste arrives as its own message type, not a KeyPressMsg, so it
		// needs forwarding explicitly — textinput knows how to consume it. Frozen
		// while thinking, exactly like typing.
		if m.thinking {
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.syncPrompt()
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
			Role:      domain.OverseerRoleSystem,
			Content:   note,
			Timestamp: time.Now(),
		})
		return m, nil

	case shared.OverseerCommandResultMsg:
		m.thinking = false
		m.appendMessage(domain.OverseerMessage{
			Role:      domain.OverseerRoleSystem,
			Content:   msg.Text,
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
	m.rendered = append(m.rendered, m.renderBody(msg))
	m.refreshViewport()
}

// renderBody converts one message's content to styled text. Agent and user prose
// goes through markdown; system notices stay literal, because they are Overseer's
// own one-line notes and markdown would only mangle paths and errors in them.
func (m Model) renderBody(msg domain.OverseerMessage) string {
	if msg.Role == domain.OverseerRoleSystem {
		return m.styles.Chat.SystemText.Render("  ○ " + msg.Content)
	}
	return m.markdown.Render(msg.Content)
}

// rerenderAll rebuilds every cached body, after a resize changed the wrap width.
func (m *Model) rerenderAll() {
	m.rendered = make([]string, len(m.history))
	for i, msg := range m.history {
		m.rendered[i] = m.renderBody(msg)
	}
}

// refreshViewport re-renders the message history into the viewport.
// It scrolls to the bottom only when the viewport was already at the bottom
// before the update, so a user who has scrolled up to read history is not
// yanked back on every new message.
func (m *Model) refreshViewport() {
	s := m.styles
	separator := s.Chat.Separator.Render(strings.Repeat("─", max(m.viewport.Width(), 1)))

	var b strings.Builder
	for i, msg := range m.history {
		body := ""
		if i < len(m.rendered) {
			body = m.rendered[i]
		}

		switch msg.Role {
		case domain.OverseerRoleUser:
			b.WriteString(s.Chat.UserLabel.Render("You"))
			b.WriteByte('\n')
			b.WriteString(body)
		case domain.OverseerRoleAgent:
			b.WriteString(s.Chat.AgentLabel.Render("Agent"))
			b.WriteByte('\n')
			b.WriteString(body)
		case domain.OverseerRoleSystem:
			b.WriteString(body)
		}
		b.WriteByte('\n')
		b.WriteString(separator)
		b.WriteByte('\n')
	}
	atBottom := m.viewport.AtBottom()
	m.viewport.SetContent(b.String())
	if atBottom {
		m.viewport.GotoBottom()
	}
}

// syncPrompt updates the input prompt prefix based on the current input
// value. Lines starting with '/' show the operator prompt; everything else
// shows the agent prompt.
func (m *Model) syncPrompt() {
	if strings.HasPrefix(m.input.Value(), "/") {
		m.input.Prompt = operatorPrompt
	} else {
		m.input.Prompt = agentPrompt
	}
}

// submit reads the current input value and routes it to agent mode or
// operator mode depending on whether the line starts with '/'.
func (m Model) submit() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.input.Value())
	if text == "" {
		return m, nil
	}
	m.input.SetValue("")
	m.input.Prompt = agentPrompt // reset for next message

	if strings.HasPrefix(text, "/") {
		// Operator mode: emit the raw command; dashboard parses and executes.
		m.appendMessage(domain.OverseerMessage{
			Role:      domain.OverseerRoleUser,
			Content:   text,
			Timestamp: time.Now(),
		})
		m.thinking = true
		return m, tea.Batch(
			m.spinner.Tick,
			shared.Emit(shared.OverseerCommandMsg{Raw: text}),
		)
	}

	// Agent mode: existing LLM flow.
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

// InputValue exposes the composed-but-unsent text. Used by the dashboard's tests
// to assert where a paste landed.
func (m Model) InputValue() string { return m.input.Value() }
