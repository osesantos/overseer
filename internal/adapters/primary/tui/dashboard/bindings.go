package dashboard

import (
	"charm.land/bubbles/v2/key"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/inspector"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/session"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
)

var (
	newSessionKeyBinding         = key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new session"))
	helpMenuKeyBinding           = key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help menu"))
	quitKeyBinding               = key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q/ctrl+c", "quit"))
	attachKeyBinding             = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "attach to preview"))
	openEditorKeyBinding         = key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "open editor"))
	killPreviewSessionKeyBinding = key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "kill preview session"))
	sendAgentEnterKeyBinding     = key.NewBinding(key.WithKeys("ctrl+e"), key.WithHelp("ctrl+e", "send enter to agent"))
	overseerPanelKeyBinding      = key.NewBinding(key.WithKeys("ctrl+o"), key.WithHelp("ctrl+o", "overseer agent"))

	discoveryPopupDismissBinding = key.NewBinding(key.WithKeys("enter", "esc", " "), key.WithHelp("enter/esc", "dismiss"))

	// chatPassthroughNav are the only keys forwarded to the session list while
	// the Overseer chat panel is open. Only the arrow keys pass through so the
	// user can navigate sessions; every other key (letters, enter, esc, …) is
	// consumed by the chat input.
	chatPassthroughNav = key.NewBinding(key.WithKeys("up", "down"))

	sessionsKeyBindings  = []key.Binding{newSessionKeyBinding, attachKeyBinding, sendAgentEnterKeyBinding, openEditorKeyBinding, session.ReorderSessionUpKeyBinding, session.ReorderSessionDownKeyBinding, session.GoToNextGroupKeyBinding, session.GoToPrevGroupKeyBinding, session.DeleteSessionKeyBinding, session.RenameKeyBinding, session.CycleLabelKeyBinding}
	inspectorKeyBindings = []key.Binding{inspector.ToggleViewKeyBinding, killPreviewSessionKeyBinding}
	generalKeyBindings   = []key.Binding{helpMenuKeyBinding, overseerPanelKeyBinding, quitKeyBinding}

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
