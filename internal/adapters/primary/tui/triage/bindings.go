package triage

import "charm.land/bubbles/v2/key"

var (
	OpenKeyBinding       = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open triage session"))
	TriageKeyBinding     = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "start triage"))
	DismissKeyBinding    = key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "dismiss ticket"))
	BrowserKeyBinding    = key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open in browser"))
	AutoTriageKeyBinding = key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "toggle auto-triage"))
	HiddenKeyBinding     = key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "show/hide dismissed tickets"))
	TopKeyBinding        = key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top"))
	BottomKeyBinding     = key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom"))
	CloseKeyBinding      = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back to sessions"))

	// KeyBindings is what the help bar shows while the page is open.
	KeyBindings = []key.Binding{OpenKeyBinding, BrowserKeyBinding, TriageKeyBinding, DismissKeyBinding, AutoTriageKeyBinding, HiddenKeyBinding, TopKeyBinding, BottomKeyBinding, CloseKeyBinding}
)
