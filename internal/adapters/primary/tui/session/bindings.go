package session

import "charm.land/bubbles/v2/key"

var (
	moveDownKeyBinding    = key.NewBinding(key.WithKeys("j"), key.WithHelp("j", "move down"))
	moveUpKeyBinding      = key.NewBinding(key.WithKeys("k"), key.WithHelp("k", "move up"))
	reorderDownKeyBinding = key.NewBinding(key.WithKeys("J"), key.WithHelp("J", "reorder down"))
	reorderUpKeyBinding   = key.NewBinding(key.WithKeys("K"), key.WithHelp("K", "reorder up"))
)
