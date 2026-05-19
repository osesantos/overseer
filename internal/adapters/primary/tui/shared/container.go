package shared

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Pane is the contract that participates in a Container. Each pane
// is a self-contained sub-model with its own lifecycle and rendering.
type Pane interface {
	Init() tea.Cmd
	Update(tea.Msg) (Pane, tea.Cmd)
	View() string
	SetSize(w, h int) Pane
	Focus() Pane
	Blur() Pane
}

// Layout describes how a Container subdivides its allocated space
// among its children.
type Layout int

const (
	LayoutHorizontal Layout = iota // children side by side, equal width
	LayoutVertical                 // children stacked, equal height
)

// ContainerKeys defines focus-navigation bindings.
type ContainerKeys struct {
	Next, Prev key.Binding
}

// ContainerStyles defines visual focus indication.
type ContainerStyles struct {
	Focused, Blurred lipgloss.Style
}

// Container manages a group of sibling Panes: focus, layout, and
// message routing. It is itself a Pane, so containers nest.
type Container struct {
	panes  []Pane
	focus  int
	layout Layout
	keys   ContainerKeys
	styles ContainerStyles

	w, h int
}

// NewContainer constructs a Container around an initial set of panes.
// The first pane is focused by default.
func NewContainer(panes []Pane, layout Layout, keys ContainerKeys, styles ContainerStyles) Container {
	if len(panes) > 0 {
		panes[0] = panes[0].Focus()
	}
	return Container{
		panes:  panes,
		focus:  0,
		layout: layout,
		keys:   keys,
		styles: styles,
	}
}

func (c Container) Init() tea.Cmd {
	cmds := make([]tea.Cmd, len(c.panes))
	for i, p := range c.panes {
		cmds[i] = p.Init()
	}
	return tea.Batch(cmds...)
}

func (c Container) Update(msg tea.Msg) (Pane, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		// Container-level navigation first
		switch {
		case key.Matches(msg, c.keys.Next):
			return c.shiftFocus(1), nil
		case key.Matches(msg, c.keys.Prev):
			return c.shiftFocus(-1), nil
		}
		// Otherwise route to the focused pane only
		return c.routeToFocused(msg)

	default:
		// Broadcast everything else to all panes
		return c.broadcast(msg)
	}
}

func (c Container) View() string {
	rendered := make([]string, len(c.panes))
	for i, p := range c.panes {
		style := c.styles.Blurred
		if i == c.focus {
			style = c.styles.Focused
		}
		rendered[i] = style.Render(p.View())
	}
	if c.layout == LayoutHorizontal {
		return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	}
	return lipgloss.JoinVertical(lipgloss.Left, rendered...)
}

// SetSize divides space among children according to the Container's layout.
func (c Container) SetSize(w, h int) Pane {
	c.w, c.h = w, h
	n := len(c.panes)
	if n == 0 {
		return c
	}

	const border = 2 // 1 char per side from focus border
	for i := range c.panes {
		var cw, ch int
		if c.layout == LayoutHorizontal {
			cw = (w / n) - border
			ch = h - border
		} else {
			cw = w - border
			ch = (h / n) - border
		}
		c.panes[i] = c.panes[i].SetSize(cw, ch)
	}
	return c
}

func (c Container) Focus() Pane {
	if len(c.panes) > 0 {
		c.panes[c.focus] = c.panes[c.focus].Focus()
	}
	return c
}

func (c Container) Blur() Pane {
	for i := range c.panes {
		c.panes[i] = c.panes[i].Blur()
	}
	return c
}

// shiftFocus moves the focus index by delta (wrapping), and calls
// Blur on the previous focus and Focus on the new one.
func (c Container) shiftFocus(delta int) Container {
	if len(c.panes) <= 1 {
		return c
	}
	c.panes[c.focus] = c.panes[c.focus].Blur()
	c.focus = (c.focus + delta + len(c.panes)) % len(c.panes)
	c.panes[c.focus] = c.panes[c.focus].Focus()
	return c
}

func (c Container) routeToFocused(msg tea.Msg) (Pane, tea.Cmd) {
	var cmd tea.Cmd
	c.panes[c.focus], cmd = c.panes[c.focus].Update(msg)
	return c, cmd
}

func (c Container) broadcast(msg tea.Msg) (Pane, tea.Cmd) {
	cmds := make([]tea.Cmd, len(c.panes))
	for i, p := range c.panes {
		c.panes[i], cmds[i] = p.Update(msg)
	}
	return c, tea.Batch(cmds...)
}
