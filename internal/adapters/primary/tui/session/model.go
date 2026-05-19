package session

import (
	"context"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"charm.land/bubbles/v2/list"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/components"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/core/service"
)

// sessionItem wraps a domain.Session so it satisfies list.Item.
type sessionItem struct {
	session domain.Session
}

func (i sessionItem) FilterValue() string { return i.session.Name }
func (i sessionItem) Title() string       { return i.session.Name }
func (i sessionItem) Description() string { return i.session.ProjectName }

type Model struct {
	sessions []domain.Session
	styles   *styles.Styles
	service  service.SessionService
	list     list.Model
	focused  bool
	width    int
	height   int
	err      error
}

func New(s *styles.Styles, service service.SessionService) Model {
	delegate := list.NewDefaultDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.Title = "Sessions"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	return Model{styles: s, service: service, list: l}
}

func (m Model) Init() tea.Cmd {
	return m.loadSessions()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case shared.SessionsLoadedMsg:
		if msg.Err != nil {
			return m, nil
		}
		items := make([]list.Item, len(msg.Sessions))
		for i, s := range msg.Sessions {
			items[i] = sessionItem{session: s}
		}
		m.list.SetItems(items)
	}

	if !m.focused {
		return m, nil
	}
	// Forward all keys to list.Model — it handles j/k, filter, pagination, etc.
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	// After the list processes the key, the cursor might have moved — announce the new selection.
	return m, tea.Batch(cmd, m.emitSelection())

}

// emitSelection announces the currently-focused session to the rest of the app.
func (m Model) emitSelection() tea.Cmd {
	cur, ok := m.list.SelectedItem().(sessionItem)

	if !ok {
		return nil
	}
	return shared.Emit(shared.SessionSelectedMsg{ID: cur.session.ID.String()})
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(width, height)
}

func (m *Model) SetFocus(focus bool) {
	m.focused = focus
}

func (m Model) IsFocused() bool {
	return m.focused
}

func (m Model) View() tea.View {
	return components.PanelWithSize(m.styles, m.list.View(), m.focused, m.width, m.height)

}

func (m Model) KeyBindings() []key.Binding {
	return []key.Binding{moveDownKeyBinding, moveUpKeyBinding, reorderDownKeyBinding, reorderUpKeyBinding}
}

// The Cmd: a function that does the work and returns a Msg
func (m Model) loadSessions() tea.Cmd {
	sessions := []domain.Session{}
	return func() tea.Msg {
		result, err := m.service.List(context.Background(), service.ListSessionsRequest{})
		for _, group := range result.Groups {
			sessions = append(sessions, group.Sessions...)
		}
		return shared.SessionsLoadedMsg{Sessions: sessions, Err: err}
	}
}
