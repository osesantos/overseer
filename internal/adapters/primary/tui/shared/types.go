package shared

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

type Resizable interface {
	SetSize(width, height int)
}

type Focusable interface {
	Focus()
	Blur()
}

type KeyBindable interface {
	KeyBindings() []key.Binding
}

type Component interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (tea.Model, tea.Cmd)
	View() tea.View
	Focusable
	KeyBindable
	Focusable
}
