package dashboard

import "charm.land/bubbles/v2/key"

var (
	newSessionKeyBinding  = key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new session"))
	nextTabKeyBinding     = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch tab"))
	helpMenuKeyBinding    = key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help menu"))
	quitKeyBinding        = key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit"), key.WithHelp("ctrl+c", "quit"))
	attachShellKeyBinding = key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "attach shell"))
	attachAgentKeyBinding = key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "attach agent"))

	sessionsListKeyBindings = []key.Binding{newSessionKeyBinding, attachShellKeyBinding, attachAgentKeyBinding, nextTabKeyBinding, helpMenuKeyBinding, quitKeyBinding}
	detailsPanelKeyBindings = []key.Binding{nextTabKeyBinding, helpMenuKeyBinding, quitKeyBinding}
)
