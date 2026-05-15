package status

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
)

type Model struct {
	workdir     string
	branch      string
	prStatus    string
	agentStatus string
	styles      *styles.Styles
	width       int
}

func New(s *styles.Styles) Model {
	wd, _ := os.Getwd()
	return Model{
		workdir:     wd,
		branch:      "stubbed",
		prStatus:    "—",
		agentStatus: "idle",
		styles:      s,
		width:       80,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
	}
	return m, nil
}

func (m Model) View() string {
	sep := m.styles.Status.Separator.Render()

	trailing := sep +
		m.styles.Status.Value.Render(m.branch) +
		sep +
		m.styles.Status.Value.Render(m.prStatus) +
		sep +
		m.styles.Status.Value.Render(m.agentStatus)

	available := m.width - lipgloss.Width(trailing)
	if available < 0 {
		available = 0
	}

	wd := truncate(m.workdir, available)
	return m.styles.Status.Value.Render(wd) + trailing
}

func truncate(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth < 3 {
		return "..."
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > maxWidth-3 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "..."
}
