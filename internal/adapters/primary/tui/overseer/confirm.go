package overseer

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/components"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
)

const confirmPopupWidth = 60

var (
	confirmKeyBinding = key.NewBinding(key.WithKeys("y", "enter"), key.WithHelp("y/enter", "confirm"))
	cancelKeyBinding  = key.NewBinding(key.WithKeys("n", "esc"), key.WithHelp("n/esc", "cancel"))
)

// ConfirmModel is the confirmation popup shown before the Overseer Agent
// executes any action. It follows the same pattern as DeleteFormModel: it
// holds no logic beyond presenting the action details and emitting the
// appropriate message on key press.
type ConfirmModel struct {
	action domain.OverseerAction
	styles *styles.Styles
}

// NewConfirmModel constructs a ConfirmModel for the given action.
func NewConfirmModel(s *styles.Styles, action domain.OverseerAction) ConfirmModel {
	return ConfirmModel{action: action, styles: s}
}

func (m ConfirmModel) Init() tea.Cmd { return nil }

func (m ConfirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if key.Matches(keyMsg, confirmKeyBinding) {
		return m, shared.Emit(shared.OverseerConfirmActionMsg{Action: m.action})
	}
	if key.Matches(keyMsg, cancelKeyBinding) {
		return m, shared.Emit(shared.OverseerCancelActionMsg{})
	}
	return m, nil
}

func (m ConfirmModel) View() tea.View {
	s := m.styles
	field := s.Form.Field

	var b strings.Builder
	b.WriteString(s.Form.Title.Render("Confirm Action"))
	b.WriteString("\n\n")

	b.WriteString(field.Label.Render("Action:  "))
	b.WriteString(field.Input.Render(actionTypeLabel(m.action.Type)))
	b.WriteByte('\n')

	b.WriteString(field.Label.Render("Project: "))
	b.WriteString(field.LabelFocused.Render(m.action.Project))
	b.WriteByte('\n')

	b.WriteString(field.Label.Render("Session: "))
	b.WriteString(field.LabelFocused.Render(m.action.SessionName))
	b.WriteString("\n\n")

	b.WriteString(field.Label.Render("Prompt:"))
	b.WriteByte('\n')
	b.WriteString(field.Input.Render(wrapPrompt(m.action.Prompt, confirmPopupWidth-4)))
	b.WriteString("\n\n")

	b.WriteString(s.Form.Hint.Render("y/enter: confirm   n/esc: cancel"))

	return tea.NewView(components.Modal(s, b.String(), confirmPopupWidth, 0))
}

func actionTypeLabel(t domain.OverseerActionType) string {
	switch t {
	case domain.OverseerActionSendPrompt:
		return "Send prompt"
	default:
		return string(t)
	}
}

// wrapPrompt hard-wraps prompt text to maxWidth runes so it fits inside the
// modal without overflowing.
func wrapPrompt(text string, maxWidth int) string {
	if maxWidth <= 0 || len(text) <= maxWidth {
		return text
	}
	var b strings.Builder
	for len(text) > 0 {
		end := maxWidth
		if end > len(text) {
			end = len(text)
		}
		// Try to break at a space.
		if end < len(text) {
			for i := end; i > 0; i-- {
				if text[i-1] == ' ' {
					end = i
					break
				}
			}
		}
		b.WriteString(text[:end])
		text = strings.TrimPrefix(text[end:], " ")
		if len(text) > 0 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
