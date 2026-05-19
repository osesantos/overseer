package leftpane

import "charm.land/bubbles/v2/key"

var (
	sessionsTabKeyBinding = key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "sessions tab"))
	projectsTabKeyBinding = key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "projects tab"))
)

func SessionsTabBinding() key.Binding { return sessionsTabKeyBinding }
func ProjectsTabBinding() key.Binding { return projectsTabKeyBinding }
