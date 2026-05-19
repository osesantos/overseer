package dashboard

import (
	tea "charm.land/bubbletea/v2"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
)

// TitlebarSetActivePaneMsg updates the active pane label in the title bar.
type TitlebarSetActivePaneMsg struct {
	Label string
}

// TitleBarModel renders the top title bar with app branding + active pane label.
type TitleBarModel struct {
	width   int
	height  int
	appName string
	styles  *styles.Styles
}

func newTitlebar(s *styles.Styles, appName string) TitleBarModel {
	return TitleBarModel{styles: s, appName: appName}
}

func (m TitleBarModel) Init() tea.Cmd { return nil }

func (m *TitleBarModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}
func (m TitleBarModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m TitleBarModel) View() tea.View {
	branding := m.styles.TitleBar.Branding.Width(m.width).Height(m.height).Render(m.appName)
	return tea.NewView(branding)
}
