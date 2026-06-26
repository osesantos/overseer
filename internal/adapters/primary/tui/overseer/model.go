package overseer

import (
	"context"
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
	"github.com/dnlopes/overseer/internal/core/service"
)

const (
	// inputPrompt is the prefix shown in the text input.
	inputPrompt = "> "
	// inputHeight is the number of rows the text input row occupies.
	inputHeight = 1
	// borderFrameH is the vertical frame consumed by the panel border.
	borderFrameH = 2
)

// SessionSnapshot is a lightweight view into a session's state that the Model
// collects at submit time to pass to OverseerService.Chat as context.
type SessionSnapshot struct {
	SessionID   uuid.UUID
	SessionName string
	ProjectName string
	Branch      string
	AgentType   domain.AgentType
	Status      domain.AgentStatusKind
	PaneOutput  string
}

// Model is the Overseer chat panel. It holds a scrollable message history
// (viewport), a single-line text input, and a thinking-spinner shown while
// the agent is processing. It is embedded directly in dashboard.Model and
// becomes visible when the user presses ctrl+o.
type Model struct {
	// history accumulates all chat messages for the viewport.
	history []domain.OverseerMessage

	viewport  viewport.Model
	input     textinput.Model
	spinner   spinner.Model
	thinking  bool
	width     int
	height    int
	styles    *styles.Styles
	svc       *service.OverseerService
	snapshots func() []SessionSnapshot // injected at construction
}

// New constructs a Model. snapshots is a function called at submit time to
// capture the current session state; the dashboard provides this closure so
// the chat panel never needs to hold a reference to the session list.
func New(s *styles.Styles, svc *service.OverseerService, snapshots func() []SessionSnapshot) Model {
	ti := textinput.New()
	ti.Placeholder = "Ask the Overseer Agent…"
	ti.Prompt = inputPrompt
	ti.SetStyles(s.Form.Input)
	ti.Focus()

	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot))

	vp := viewport.New()

	return Model{
		viewport:  vp,
		input:     ti,
		spinner:   sp,
		styles:    s,
		svc:       svc,
		snapshots: snapshots,
	}
}

// SetSize updates the panel dimensions. Called by the dashboard whenever the
// terminal is resized or the panel is toggled open. height is the full height
// available to the panel (border + history + input row).
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.input.SetWidth(width - len(inputPrompt) - borderFrameH)

	// History viewport fills the space above the input row inside the border.
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
			// Disable input while the agent is thinking.
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
		// Action handling is delegated to the dashboard via the message bus;
		// the panel itself just records the chat history.
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
	// History pane.
	b.WriteString(m.viewport.View())
	b.WriteByte('\n')
	// Input row: spinner (when thinking) replaces the prompt prefix.
	if m.thinking {
		b.WriteString(s.Chat.ThinkingPrefix.Render(m.spinner.View()))
		b.WriteString(s.Chat.ThinkingText.Render(" thinking…"))
	} else {
		b.WriteString(m.input.View())
	}

	return tea.NewView(border.Render(b.String()))
}

// appendMessage adds msg to history and refreshes the viewport, scrolling to
// the bottom so the latest message is always visible.
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

// submit reads the current input value, appends a user message, and fires the
// async Chat command.
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
	return m, tea.Batch(m.spinner.Tick, m.chatCmd(text))
}

// chatCmd builds the async tea.Cmd that calls OverseerService.Chat.
func (m Model) chatCmd(userMsg string) tea.Cmd {
	svc := m.svc
	snaps := m.snapshots()
	sessions := make([]domain.OverseerSessionContext, 0, len(snaps))
	for _, snap := range snaps {
		sessions = append(sessions, domain.OverseerSessionContext{
			SessionID:   snap.SessionID,
			SessionName: snap.SessionName,
			ProjectName: snap.ProjectName,
			Branch:      snap.Branch,
			AgentType:   snap.AgentType,
			Status:      snap.Status,
			PaneOutput:  snap.PaneOutput,
		})
	}
	return shared.RequestWithTimeout(
		60*time.Second,
		func(ctx context.Context) (service.OverseerChatResponse, error) {
			return svc.Chat(ctx, service.OverseerChatRequest{
				UserMessage: userMsg,
				Sessions:    sessions,
			})
		},
		func(resp service.OverseerChatResponse, err error) tea.Msg {
			if err != nil {
				return shared.OverseerChatResponseMsg{Err: err}
			}
			return shared.OverseerChatResponseMsg{
				Text:   resp.Text,
				Action: resp.Action,
			}
		},
	)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// borderWidth returns the horizontal frame size of the focused border style.
func borderWidth(s *styles.Styles) int {
	l, r := s.Border.Focused.GetFrameSize()
	_ = l
	return r
}

// panelBorder returns a copy of the focused border style sized to the given
// outer dimensions. Exported for tests.
func panelBorder(s *styles.Styles, width, height int) lipgloss.Style {
	return s.Border.Focused.Width(width - borderFrameH).Height(height - borderFrameH)
}
