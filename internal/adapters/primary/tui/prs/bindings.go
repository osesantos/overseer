package prs

import "charm.land/bubbles/v2/key"

var (
	OpenKeyBinding    = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open review session"))
	ReviewKeyBinding  = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "start review"))
	DismissKeyBinding = key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "dismiss PR"))
	TopKeyBinding     = key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top"))
	BottomKeyBinding  = key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom"))
	CloseKeyBinding   = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back to sessions"))

	// KeyBindings is what the help bar shows while the page is open.
	KeyBindings = []key.Binding{OpenKeyBinding, ReviewKeyBinding, DismissKeyBinding, TopKeyBinding, BottomKeyBinding, CloseKeyBinding}
)
