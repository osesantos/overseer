package dashboard

import (
	tea "charm.land/bubbletea/v2"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/shared/omarchy"
)

// WithThemeWatch makes the dashboard repaint whenever the Omarchy theme changes; themes is the signal from omarchy.Watch.
func (m Model) WithThemeWatch(themes <-chan struct{}) Model {
	m.themes = themes
	return m
}

// waitTheme blocks until the Omarchy theme changes, then yields the new palette.
func (m Model) waitTheme() tea.Cmd {
	if m.themes == nil {
		return nil
	}
	ch := m.themes
	return func() tea.Msg {
		<-ch
		palette, _ := omarchy.Palette(omarchy.Dir())
		return shared.OmarchyPaletteMsg{Palette: palette}
	}
}
