package shared

import tea "charm.land/bubbletea/v2"

// UpdateModel forwards msg to m.Update and returns the result with m's concrete type preserved.
func UpdateModel[T any](m T, msg tea.Msg) (T, tea.Cmd) {
	updated, cmd := any(m).(interface {
		Update(tea.Msg) (tea.Model, tea.Cmd)
	}).Update(msg)

	return updated.(T), cmd
}

// Forward adapts a typed child pointer into the untyped forwarder signature Broadcast expects.
func Forward[T any](m *T) func(tea.Msg) tea.Cmd {
	return func(msg tea.Msg) tea.Cmd {
		var cmd tea.Cmd
		*m, cmd = UpdateModel(*m, msg)
		return cmd
	}
}

// Broadcast dispatches msg to each forwarder and returns the batched Cmds.
//
//	return m, shared.Broadcast(msg,
//	    shared.Forward(&m.sessionsModel),
//	    shared.Forward(&m.detailsModel),
//	)
func Broadcast(msg tea.Msg, forwarders ...func(tea.Msg) tea.Cmd) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(forwarders))
	for _, f := range forwarders {
		if cmd := f(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}
