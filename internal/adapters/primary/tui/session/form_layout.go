package session

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
)

const (
	formMinBoxWidth = 60
	formMaxBoxWidth = 80
	formBoxRatioNum = 7
	formBoxRatioDen = 10
	formModalChrome = 8
)

func formContentWidth(terminalWidth int) int {
	if terminalWidth <= 0 {
		return formMinBoxWidth - formModalChrome
	}
	boxWidth := min(max(terminalWidth*formBoxRatioNum/formBoxRatioDen, formMinBoxWidth), formMaxBoxWidth)
	return boxWidth - formModalChrome
}

func formInputWidth(contentWidth int) int {
	return max(contentWidth-4, 20)
}

func renderField(labelStyle lipgloss.Style, label, value string) string {
	return labelStyle.Render(label) + "\n" + value
}

func renderFieldHint(s *styles.Styles, hint string) string {
	return s.Form.Hint.Render(hint)
}

// padBodyLines wraps every line of `body` in a styled span sized to `width`
// with the modal's background. Lipgloss v2's outer Width() pads short lines
// with UNSTYLED spaces, so without this each line shows terminal-default bg
// in the gap between content and the modal's right edge — visible as dark
// stripes inside the modal.
func padBodyLines(s *styles.Styles, body string, width int) string {
	bg := s.Modal.Box.GetBackground()
	padder := lipgloss.NewStyle().Background(bg).Width(width)
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		lines[i] = padder.Render(line)
	}
	return strings.Join(lines, "\n")
}
