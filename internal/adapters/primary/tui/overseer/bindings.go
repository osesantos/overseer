package overseer

import "charm.land/bubbles/v2/key"

var (
	submitKeyBinding       = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "submit"))
	scrollUpKeyBinding     = key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "scroll up"))
	scrollDownKeyBinding   = key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "scroll down"))
	scrollPageUpBinding    = key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up"))
	scrollPageDownBinding  = key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdown", "page down"))
)
