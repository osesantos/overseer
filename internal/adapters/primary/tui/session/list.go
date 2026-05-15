package session

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	domainsession "github.com/dnlopes/overseer/internal/core/domain/session"
	servicesession "github.com/dnlopes/overseer/internal/core/service/session"
)

type groupsLoadedMsg struct {
	groups []servicesession.SessionGroup
	err    error
}

type reorderRequestMsg struct {
	direction int
}

type SessionGroup = servicesession.SessionGroup

type Model struct {
	groups  []SessionGroup
	cursor  int
	styles  *styles.Styles
	focused bool
	width   int
	height  int
	listUC  *servicesession.ListUseCase
}

func New(s *styles.Styles, listUC *servicesession.ListUseCase) Model {
	return Model{
		styles: s,
		listUC: listUC,
	}
}

func (m Model) Init() tea.Cmd {
	if m.listUC == nil {
		return nil
	}
	return func() tea.Msg {
		resp, err := m.listUC.Execute(context.Background(), servicesession.ListRequest{})
		if err != nil {
			return groupsLoadedMsg{err: err}
		}
		return groupsLoadedMsg{groups: resp.Groups}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case groupsLoadedMsg:
		if msg.err == nil {
			m.groups = msg.groups
			m.cursor = 0
		}

	case tea.KeyPressMsg:
		total := m.totalItems()
		switch msg.String() {
		case "j":
			if total > 0 && m.cursor < total-1 {
				m.cursor++
			}
		case "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "J":
			return m, func() tea.Msg { return reorderRequestMsg{direction: +1} }
		case "K":
			return m, func() tea.Msg { return reorderRequestMsg{direction: -1} }
		}
	}

	return m, nil
}

func (m Model) View() tea.View {
	return tea.NewView(m.render())
}

func (m Model) render() string {
	var content string
	if m.totalItems() == 0 {
		content = m.renderEmpty()
	} else {
		content = m.renderGroups()
	}

	var border lipgloss.Style
	if m.focused {
		border = m.styles.Border.Focused
	} else {
		border = m.styles.Border.Blurred
	}
	return border.Render(content)
}

func (m Model) renderEmpty() string {
	title := m.styles.EmptyState.Title.Render("No sessions yet.")
	hint := m.styles.EmptyState.Hint.Render("Press n to create your first session")
	return title + "\n" + hint
}

func (m Model) renderGroups() string {
	lines := make([]string, 0, len(m.groups)*2)
	flatIdx := 0

	for _, g := range m.groups {
		lines = append(lines, m.styles.Group.Header.Render(g.ProjectName))
		for _, s := range g.Sessions {
			if flatIdx == m.cursor {
				lines = append(lines, m.styles.Session.Item.Selected.Render(s.Name))
			} else {
				lines = append(lines, m.styles.Session.Item.Normal.Render(s.Name))
			}
			flatIdx++
		}
	}

	return strings.Join(lines, "\n")
}

func (m Model) totalItems() int {
	n := 0
	for _, g := range m.groups {
		n += len(g.Sessions)
	}
	return n
}

func (m *Model) SetFocus(focused bool) {
	m.focused = focused
}

func (m Model) SelectedSession() (domainsession.Session, bool) {
	if m.totalItems() == 0 {
		return domainsession.Session{}, false
	}
	flatIdx := 0
	for _, g := range m.groups {
		for _, s := range g.Sessions {
			if flatIdx == m.cursor {
				return s, true
			}
			flatIdx++
		}
	}
	return domainsession.Session{}, false
}

func (m Model) Keybindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("j"), key.WithHelp("j", "move down")),
		key.NewBinding(key.WithKeys("k"), key.WithHelp("k", "move up")),
		key.NewBinding(key.WithKeys("J"), key.WithHelp("J", "reorder down")),
		key.NewBinding(key.WithKeys("K"), key.WithHelp("K", "reorder up")),
	}
}
