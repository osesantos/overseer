package leftpane

import "charm.land/bubbles/v2/key"

var (
	sessionsTabKeyBinding = key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "sessions tab"))
	projectsTabKeyBinding = key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "projects tab"))
)

func SessionsTabBinding() key.Binding { return sessionsTabKeyBinding }
func ProjectsTabBinding() key.Binding { return projectsTabKeyBinding }
