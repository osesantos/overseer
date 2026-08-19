package inspector

import (
	tea "charm.land/bubbletea/v2"

	"github.com/dnlopes/overseer/internal/core/domain"
)

// View is one tab inside the inspector. Each view owns its own polling
// loop; the inspector merely routes messages and key events to the active
// view, and re-inits a view's loop when it becomes active or when the
// selected session changes.
//
// SetSession takes the whole Session rather than just its ID because views need
// to know whether it is a swarm: that decides which tabs exist at all, and how
// many agent panes the Agent tab can page through.
type View interface {
	Label() string
	Init() tea.Cmd
	Update(msg tea.Msg) (View, tea.Cmd)
	Body() string
	SetSize(width, height int)
	SetSession(sess domain.Session)
}
