package shared

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
)

type HelpBarModel struct {
	bindings      []key.Binding
	styles        *styles.Styles
	width, height int
}

func NewHelpBarModel(styles *styles.Styles, bindings []key.Binding) HelpBarModel {
	return HelpBarModel{bindings: bindings, styles: styles, width: 1, height: 1}
}

func (m HelpBarModel) Init() tea.Cmd {
	return nil
}

func (m HelpBarModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m HelpBarModel) View() tea.View {
	return tea.NewView(m.renderHelpBar(m.styles, m.bindings))
}

func (m *HelpBarModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// renderHelpBar lays out bindings as a single styled line where every cell —
// including the gaps between key chips, descriptions, and separators —
// carries the bar background, so the bar reads as one continuous strip.
func (m HelpBarModel) renderHelpBar(s *styles.Styles, bindings []key.Binding) string {
	bar := s.Help.Bar.Height(m.height).Width(m.width)
	available := m.width - bar.GetHorizontalFrameSize()
	sep := s.Help.Separator.Render(" • ")
	sepW := lipgloss.Width(sep)

	var b strings.Builder
	var used int
	first := true
	for _, kb := range bindings {
		if !kb.Enabled() {
			continue
		}
		seg := s.Help.Key.Render(kb.Help().Key) + s.Help.Description.Render(" "+kb.Help().Desc)
		segW := lipgloss.Width(seg)

		addW := segW
		if !first {
			addW += sepW
		}
		if m.width > 0 && used+addW > available {
			break
		}
		if !first {
			b.WriteString(sep)
		}
		b.WriteString(seg)
		used += addW
		first = false
	}

	return bar.Render(b.String())
}
