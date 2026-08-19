package inspector

import "charm.land/bubbles/v2/key"

var (
	// BoardComposeKeyBinding opens the board's compose line. Compose is modal on
	// purpose: while it is open the dashboard hands over every key, so leaving it
	// always-on would swallow session navigation.
	//
	// Exported because the dashboard must forward it explicitly, and only when
	// CanCompose reports the board is ready — otherwise "i" would be swallowed
	// everywhere else in the UI.
	BoardComposeKeyBinding = key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "message the swarm"),
	)
	boardComposeSubmitKeyBinding = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "post to board"),
	)
	boardComposeCancelKeyBinding = key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel message"),
	)

	// Board history scrolls with pgup/pgdown rather than the arrow keys, because
	// the dashboard claims up/down for session navigation before the inspector
	// ever sees them.
	boardScrollUpKeyBinding = key.NewBinding(
		key.WithKeys("pgup"),
		key.WithHelp("pgup", "scroll board up"),
	)
	boardScrollDownKeyBinding = key.NewBinding(
		key.WithKeys("pgdown"),
		key.WithHelp("pgdown", "scroll board down"),
	)

	// SwarmAgentNextKeyBinding and SwarmAgentPrevKeyBinding page the Agent tab
	// across a swarm's panes. They are exported because the dashboard has to
	// forward them explicitly — the inspector otherwise only ever receives tab.
	SwarmAgentNextKeyBinding = key.NewBinding(
		key.WithKeys("]"),
		key.WithHelp("]", "next agent"),
	)
	SwarmAgentPrevKeyBinding = key.NewBinding(
		key.WithKeys("["),
		key.WithHelp("[", "previous agent"),
	)
)
