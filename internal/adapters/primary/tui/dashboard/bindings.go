package dashboard

import "charm.land/bubbles/v2/key"

var (
	newSessionKeyBinding = key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new session"))
	helpMenuKeyBinding   = key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help menu"))
	quitKeyBinding       = key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit"), key.WithHelp("ctrl+c", "quit"))
	nextTabKeyBinding    = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch tab"))
)
