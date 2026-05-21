package components

import (
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
)

func Modal(s *styles.Styles, body string, contentWidth, contentHeight int) string {
	style := s.Modal.Box
	frameW, frameH := style.GetFrameSize()
	if contentWidth > 0 {
		style = style.Width(contentWidth + frameW)
	}
	if contentHeight > 0 {
		style = style.Height(contentHeight + frameH)
	}
	return style.Render(body)
}
