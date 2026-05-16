package components

import (
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
