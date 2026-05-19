package session

import "charm.land/bubbles/v2/key"

var (
	popupNextFieldKeyBinding  = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field"))
	popupPrevFieldKeyBinding  = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "previous field"))
	popupSubmitFormKeyBinding = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "create session"))
	popupCloseKeyBinding      = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel"))
)
