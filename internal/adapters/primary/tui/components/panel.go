package components

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	border := panelBorder(s, focused)
	innerW, innerH := PanelInnerSize(s, focused, width, height)

	content = s.Pane.Container.Width(innerW).Height(innerH).Render(content)
	return tea.NewView(border.Width(width).Height(height).Render(content))
}

// PanelInnerSize returns the width and height available for content inside a
// PanelWithSize of the given total dimensions. Callers that wrap a sized
// child (e.g. a list) in PanelWithSize should size that child to the inner
// dimensions so it does not overflow the panel's frame.
func PanelInnerSize(s *styles.Styles, focused bool, width, height int) (innerW, innerH int) {
	borderW, borderH := panelBorder(s, focused).GetFrameSize()
	containerW, containerH := s.Pane.Container.GetFrameSize()
	return max(width-borderW-containerW, 0), max(height-borderH-containerH, 0)
}

func panelBorder(s *styles.Styles, focused bool) lipgloss.Style {
	if focused {
		return s.Border.Focused
	}
	return s.Border.Blurred
}
