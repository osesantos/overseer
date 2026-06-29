package inspector

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/components"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
)

// loopViewContentMsg is an inspector-internal message that updates the Loop
// tab's displayed content. It is produced by the inspector itself when it
// receives LoopStartedMsg or LoopOutputUpdatedMsg from the dashboard.
type loopViewContentMsg struct {
	content string
}

// loopView is the "Loop" inspector tab. Unlike streamView it does not poll a
// tmux pane; instead it displays text pushed to it via loopViewContentMsg.
type loopView struct {
	content string
	width   int
	height  int
	styles  *styles.Styles
}

func newLoopView(s *styles.Styles) *loopView {
	return &loopView{styles: s}
}

func (v *loopView) Label() string { return "Loop" }

func (v *loopView) Init() tea.Cmd { return nil }

func (v *loopView) Update(msg tea.Msg) (View, tea.Cmd) {
	if m, ok := msg.(loopViewContentMsg); ok {
		v.content = m.content
	}
	return v, nil
}

func (v *loopView) Body() string {
	if v.content == "" {
		return components.CenteredContent(v.styles,
			v.styles.EmptyState.Title.Render("No loop running — use /loop <session> <criteria…>"),
			v.width, v.height)
	}
	lines := strings.Split(v.content, "\n")
	if v.height > 0 && len(lines) > v.height {
		lines = lines[len(lines)-v.height:]
	}
	return strings.Join(lines, "\n")
}

func (v *loopView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

func (v *loopView) SetSession(_ uuid.UUID) {}
