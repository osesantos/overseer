package pullrequest

import "charm.land/bubbles/v2/key"

var (
	NextSectionKeyBinding = key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "next section"))
	PrevSectionKeyBinding = key.NewBinding(key.WithKeys("["), key.WithHelp("[", "previous section"))
	TopKeyBinding         = key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top"))
	BottomKeyBinding      = key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom"))
	RefetchKeyBinding     = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refetch"))
	BrowserKeyBinding     = key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open in browser"))
	CloseKeyBinding       = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back to sessions"))
	// scrollKeyBinding is only for the help bar: the viewport's default key map
	// does the scrolling (j/k, arrows, ctrl+d/ctrl+u, pgup/pgdn).
	scrollKeyBinding = key.NewBinding(key.WithKeys("j", "k", "ctrl+d", "ctrl+u"), key.WithHelp("j/k ctrl+d/u", "scroll"))

	// KeyBindings is what the help bar shows while the page is open.
	KeyBindings = []key.Binding{PrevSectionKeyBinding, NextSectionKeyBinding, scrollKeyBinding, TopKeyBinding, BottomKeyBinding, RefetchKeyBinding, BrowserKeyBinding, CloseKeyBinding}
)
