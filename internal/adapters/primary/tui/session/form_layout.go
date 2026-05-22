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

	formLabelColumnWidth = 17
	formLabelGap         = 1
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

// formValueColumnWidth is the usable width of the right (value) column in
// the 2-column field layout, accounting for the label column + gap.
func formValueColumnWidth(contentWidth int) int {
	return max(contentWidth-formLabelColumnWidth-formLabelGap-2, 20)
}

// renderField renders a label/value pair side-by-side: label is padded to a
// fixed column width on the left, value sits on the right. The label and
// gap columns are sized to the value's line count so multi-line values (the
// branch picker) get a bg-painted column underneath the label — without it
// lipgloss.JoinHorizontal pads the shorter column with unstyled spaces and
// the terminal-default background bleeds through as a dark stripe.
func renderField(s *styles.Styles, labelStyle lipgloss.Style, label, value string) string {
	bg := s.Modal.Box.GetBackground()
	h := max(lipgloss.Height(value), 1)
	padded := labelStyle.Background(bg).Width(formLabelColumnWidth).Height(h).Render(label)
	gap := lipgloss.NewStyle().Background(bg).Width(formLabelGap).Height(h).Render("")
	return lipgloss.JoinHorizontal(lipgloss.Top, padded, gap, value)
}

// renderFieldHint renders a hint indented to sit under the value column,
// so it visually belongs to the field it annotates. The indent is styled
// with the modal background to match the rest of the row.
func renderFieldHint(s *styles.Styles, hint string) string {
	bg := s.Modal.Box.GetBackground()
	indent := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", formLabelColumnWidth+formLabelGap))
	return indent + s.Form.Hint.Background(bg).Render(hint)
}

// modalListRow returns the ListRow style suitable for in-modal value
// rendering (selector view, branch picker rows). The shared ListRow.Normal
// has no background and would leak the terminal default across the value
// column inside a modal; this overlay paints the modal background through.
func modalListRow(s *styles.Styles, selected bool) lipgloss.Style {
	if selected {
		return s.ListRow.Selected
	}
	return s.ListRow.Normal.Background(s.Modal.Box.GetBackground())
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
