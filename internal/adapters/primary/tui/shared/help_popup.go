package shared

import (
	"image/color"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/components"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
)

type HelpPopupGroup struct {
	Title    string
	Bindings []key.Binding
}

var helpPopupCloseKeys = key.NewBinding(
	key.WithKeys("?", "esc", "q"),
	key.WithHelp("?/esc/q", "close"),
)

const (
	helpPopupMinBoxWidth = 44
	helpPopupMaxBoxWidth = 64
	helpPopupBoxRatioNum = 6
	helpPopupBoxRatioDen = 10
	helpPopupModalChrome = 8
	helpPopupKeyDescGap  = 2
)

type HelpPopupModel struct {
	groups        []HelpPopupGroup
	styles        *styles.Styles
	terminalWidth int
}

func NewHelpPopupModel(s *styles.Styles, groups []HelpPopupGroup, terminalWidth int) HelpPopupModel {
	return HelpPopupModel{
		groups:        groups,
		styles:        s,
		terminalWidth: terminalWidth,
	}
}

func (m HelpPopupModel) Init() tea.Cmd { return nil }

func (m HelpPopupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if key.Matches(keyMsg, helpPopupCloseKeys) {
		return m, Emit(HelpPopupCloseMsg{})
	}
	return m, nil
}

func (m HelpPopupModel) View() tea.View {
	contentWidth := helpPopupContentWidth(m.terminalWidth)
	body := padHelpBody(m.styles, m.renderBody(), contentWidth)
	return tea.NewView(components.Modal(m.styles, body, contentWidth, 0))
}

func (m HelpPopupModel) renderBody() string {
	bg := m.styles.Modal.Box.GetBackground()
	titleStyle := m.styles.Form.Title.Background(bg)
	groupTitleStyle := m.styles.Form.Field.LabelFocused.Background(bg)
	hintStyle := m.styles.Form.Hint.Background(bg)
	keyColWidth := m.widestKey()

	var b strings.Builder
	b.WriteString(titleStyle.Render("Keyboard Shortcuts"))
	b.WriteByte('\n')

	first := true
	for _, g := range m.groups {
		visible := enabledBindings(g.Bindings)
		if len(visible) == 0 {
			continue
		}
		if !first {
			b.WriteByte('\n')
		}
		first = false
		if g.Title != "" {
			b.WriteString(groupTitleStyle.Render(g.Title))
			b.WriteByte('\n')
		}
		for _, kb := range visible {
			b.WriteString(renderHelpPopupRow(m.styles, kb, keyColWidth, bg))
			b.WriteByte('\n')
		}
	}

	b.WriteByte('\n')
	b.WriteString(hintStyle.Render("? / esc / q  close"))
	return b.String()
}

func (m HelpPopupModel) widestKey() int {
	w := 0
	for _, g := range m.groups {
		for _, kb := range g.Bindings {
			if !kb.Enabled() {
				continue
			}
			if width := lipgloss.Width(kb.Help().Key); width > w {
				w = width
			}
		}
	}
	return w
}

func renderHelpPopupRow(s *styles.Styles, kb key.Binding, keyColWidth int, bg color.Color) string {
	keyStyle := s.Form.Field.LabelFocused.Background(bg)
	descStyle := s.Form.Field.Label.Background(bg)
	keyText := kb.Help().Key
	pad := max(keyColWidth-lipgloss.Width(keyText), 0)
	keyCell := keyStyle.Render(keyText + strings.Repeat(" ", pad))
	descCell := descStyle.Render(strings.Repeat(" ", helpPopupKeyDescGap) + kb.Help().Desc)
	return keyCell + descCell
}

func enabledBindings(bs []key.Binding) []key.Binding {
	out := make([]key.Binding, 0, len(bs))
	for _, b := range bs {
		if b.Enabled() {
			out = append(out, b)
		}
	}
	return out
}

func helpPopupContentWidth(terminalWidth int) int {
	if terminalWidth <= 0 {
		return helpPopupMinBoxWidth - helpPopupModalChrome
	}
	boxWidth := min(max(terminalWidth*helpPopupBoxRatioNum/helpPopupBoxRatioDen, helpPopupMinBoxWidth), helpPopupMaxBoxWidth)
	return boxWidth - helpPopupModalChrome
}

func padHelpBody(s *styles.Styles, body string, width int) string {
	padder := s.Modal.LinePad.Width(width)
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		lines[i] = padder.Render(line)
	}
	return strings.Join(lines, "\n")
}
