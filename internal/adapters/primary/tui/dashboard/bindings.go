package dashboard

import (
	"charm.land/bubbles/v2/key"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/inspector"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/session"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
)

var (
	newSessionKeyBinding  = key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new session"))
	helpMenuKeyBinding    = key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help menu"))
	quitKeyBinding        = key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q/ctrl+c", "quit"))
	attachShellKeyBinding = key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "attach shell"))
	attachAgentKeyBinding = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "attach agent"))
	openEditorKeyBinding  = key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "open editor"))

	sessionsKeyBindings  = []key.Binding{newSessionKeyBinding, attachAgentKeyBinding, attachShellKeyBinding, openEditorKeyBinding, session.ReorderSessionUpKeyBinding, session.ReorderSessionDownKeyBinding, session.GoToNextGroupKeyBinding, session.GoToPrevGroupKeyBinding, session.DeleteSessionKeyBinding, session.RenameKeyBinding, session.CycleLabelKeyBinding}
	inspectorKeyBindings = []key.Binding{inspector.ToggleViewKeyBinding}
	generalKeyBindings   = []key.Binding{helpMenuKeyBinding, quitKeyBinding}

	sessionsHelpGroups = []shared.HelpPopupGroup{
		{
			Title:    "Sessions",
			Bindings: sessionsKeyBindings,
		},
		{
			Title:    "Inspector",
			Bindings: inspectorKeyBindings,
		},
		{
			Title:    "General",
			Bindings: generalKeyBindings,
		},
	}
)
