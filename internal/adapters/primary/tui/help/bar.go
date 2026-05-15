package help

import (
	bubblehelp "github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// keyMapAdapter wraps a flat []key.Binding to satisfy bubblehelp.KeyMap.
type keyMapAdapter struct {
	bindings []key.Binding
}

func (k keyMapAdapter) ShortHelp() []key.Binding    { return k.bindings }
func (k keyMapAdapter) FullHelp() [][]key.Binding   { return [][]key.Binding{k.bindings} }

// barKeys holds the bindings consumed by the help bar itself.
type barKeys struct {
	toggleHelp key.Binding
}

func defaultBarKeys() barKeys {
	return barKeys{
		toggleHelp: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
	}
}

// Model is the BubbleTea sub-model for the help bar.
type Model struct {
	help       bubblehelp.Model
	registry   *Registry
	activePane string
	showFull   bool
	keys       barKeys
}

// NewHelpBar returns a Model wired to registry.
func NewHelpBar(registry *Registry) Model {
	return Model{
		help:     bubblehelp.New(),
		registry: registry,
		keys:     defaultBarKeys(),
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, m.keys.toggleHelp) {
			m.showFull = !m.showFull
			m.help.ShowAll = m.showFull
		}
	case tea.WindowSizeMsg:
		m.help.Width = msg.Width
	}
	return m, nil
}

func (m Model) View() string {
	km := keyMapAdapter{bindings: m.registry.BindingsFor(m.activePane)}
	return m.help.View(km)
}

// SetActivePane updates which pane's bindings appear in the bar.
func (m *Model) SetActivePane(name string) {
	m.activePane = name
}
