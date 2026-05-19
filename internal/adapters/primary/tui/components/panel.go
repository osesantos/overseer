package components

import (
	tea "charm.land/bubbletea/v2"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
)

// Panel wraps content in a rounded border using styles.Border (Focused or Blurred).
// PURE function: consumes *styles.Styles, returns rendered output.
// MUST NOT create new lipgloss styles here — only consume *styles.Styles (C7).
func Panel(s *styles.Styles, content string, focused bool) string {
	if focused {
		return s.Border.Focused.Render(content)
	}
	return s.Border.Blurred.Render(content)
}

func PanelWithSize(s *styles.Styles, content string, focused bool, width, height int) tea.View {
	content = s.Pane.Container.Width(width).Height(height).Render(content)

	var rawContent string
	if focused {
		rawContent = s.Border.Focused.Width(width).Height(height).Render(content)
	} else {
		rawContent = s.Border.Blurred.Width(width).Height(height).Render(content)
	}
	return tea.NewView(rawContent)
}
