package dashboard

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/components"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
)

var AttachShell = key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "attach shell"))
var AttachAgent = key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "attach agent"))

type PreviewModel struct {
	width   int
	height  int
	styles  styles.Styles
	focused bool
}

func newPreview(s styles.Styles) PreviewModel {
	return PreviewModel{styles: s}
}

func (m PreviewModel) Init() tea.Cmd {
	return nil
}

func (m PreviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m PreviewModel) View() tea.View {
	title := m.styles.EmptyState.Title.Render("Not implemented yet")
	hint := components.KeyBadge(&m.styles, "n", "create session")
	content := title + "\n" + hint

	return components.PanelWithSize(&m.styles, content, m.focused, m.width, m.height)
}

func (m *PreviewModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m PreviewModel) KeyBindings() []key.Binding {
	return []key.Binding{AttachShell, AttachAgent}
}

func (m *PreviewModel) SetFocus(focus bool) {
	m.focused = focus
}
