package components

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// ---------- Public types ----------

// Node is what callers supply: a generic tree of items. The component
// doesn't care what T is — it only navigates, expands, and renders.
type Node[T any] struct {
	ID       string
	Item     T
	Children []Node[T]
}

// RenderFunc lets callers control how each item is displayed.
// Receives the item, indentation depth, whether it has children,
// whether it's currently expanded, and whether the cursor is on it.
type RenderFunc[T any] func(item T, depth int, hasKids, expanded, focused bool) string

// SelectMsg is emitted whenever the cursor lands on a different node,
// either by navigation or after a refresh.
type SelectMsg[T any] struct {
	ID   string
	Item T
}

// ---------- Configuration ----------

type KeyMap struct {
	Up, Down, Toggle, ExpandAll, CollapseAll key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:          key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:        key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		Toggle:      key.NewBinding(key.WithKeys("enter", " "), key.WithHelp("⏎/space", "toggle")),
		ExpandAll:   key.NewBinding(key.WithKeys("E"), key.WithHelp("E", "expand all")),
		CollapseAll: key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "collapse all")),
	}
}

// ShortHelp/FullHelp satisfy bubbles/help.KeyMap.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Toggle}
}
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Toggle, k.ExpandAll, k.CollapseAll},
	}
}

// ---------- Internal row representation ----------

// row is a flattened view of one visible item.
type row[T any] struct {
	id       string
	item     T
	depth    int
	hasKids  bool
	expanded bool
}

// ---------- Component ----------

type Model[T any] struct {
	nodes    []Node[T]       // source of truth (caller-supplied)
	rows     []row[T]        // flattened, filtered view
	expanded map[string]bool // node ID → expanded?
	cursor   int             // index into rows[]
	focused  bool

	w, h   int
	keys   KeyMap
	render RenderFunc[T]
}

func New[T any](render RenderFunc[T]) Model[T] {
	return Model[T]{
		expanded: make(map[string]bool),
		keys:     DefaultKeyMap(),
		render:   render,
	}
}

// ---------- Configuration ----------

func (m Model[T]) WithKeyMap(k KeyMap) Model[T] {
	m.keys = k
	return m
}

// ---------- Data ----------

// SetNodes replaces the tree's source data and re-flattens. The cursor
// is preserved by ID when possible; if the previously-focused node is
// gone, the cursor stays at the same index (clamped to bounds).
func (m Model[T]) SetNodes(nodes []Node[T]) Model[T] {
	var prevID string
	if len(m.rows) > 0 && m.cursor < len(m.rows) {
		prevID = m.rows[m.cursor].id
	}

	m.nodes = nodes
	m.rows = flatten(nodes, m.expanded)

	// Try to restore cursor by ID
	if prevID != "" {
		for i, r := range m.rows {
			if r.id == prevID {
				m.cursor = i
				return m
			}
		}
	}
	m.cursor = clamp(m.cursor, 0, len(m.rows)-1)
	return m
}

// ExpandAll/CollapseAll toggle expansion state across every group.
func (m Model[T]) ExpandAll() Model[T] {
	walk(m.nodes, func(n Node[T]) {
		if len(n.Children) > 0 {
			m.expanded[n.ID] = true
		}
	})
	return m.reflatten()
}

func (m Model[T]) CollapseAll() Model[T] {
	m.expanded = make(map[string]bool)
	return m.reflatten()
}

// ---------- Selection helpers ----------

// Selected returns the currently-focused item, or zero T and false if empty.
func (m Model[T]) Selected() (T, bool) {
	var zero T
	if len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return zero, false
	}
	return m.rows[m.cursor].item, true
}

// SelectedID returns the ID of the focused node, or "" if empty.
func (m Model[T]) SelectedID() string {
	if len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return ""
	}
	return m.rows[m.cursor].id
}

// ---------- Bubble Tea lifecycle ----------

func (m Model[T]) Init() tea.Cmd { return nil }

func (m Model[T]) Update(msg tea.Msg) (Model[T], tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch {
	case key.Matches(keyMsg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
			return m, m.emitSelection()
		}
	case key.Matches(keyMsg, m.keys.Down):
		if m.cursor < len(m.rows)-1 {
			m.cursor++
			return m, m.emitSelection()
		}
	case key.Matches(keyMsg, m.keys.Toggle):
		return m.toggleCurrent(), nil
	case key.Matches(keyMsg, m.keys.ExpandAll):
		return m.ExpandAll(), nil
	case key.Matches(keyMsg, m.keys.CollapseAll):
		return m.CollapseAll(), nil
	}
	return m, nil
}

func (m Model[T]) View() string {
	if len(m.rows) == 0 {
		return ""
	}
	var b strings.Builder
	visible := m.visibleWindow()
	for i := visible.top; i <= visible.bottom; i++ {
		r := m.rows[i]
		b.WriteString(m.render(r.item, r.depth, r.hasKids, r.expanded, i == m.cursor))
		if i < visible.bottom {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m Model[T]) SetSize(w, h int) Model[T] {
	m.w, m.h = w, h
	return m
}

func (m Model[T]) Focus() Model[T] {
	m.focused = true
	return m
}

func (m Model[T]) Blur() Model[T] {
	m.focused = false
	return m
}

func (m Model[T]) KeyMap() KeyMap { return m.keys }

// ---------- Internal helpers ----------

func (m Model[T]) toggleCurrent() Model[T] {
	if len(m.rows) == 0 {
		return m
	}
	cur := m.rows[m.cursor]
	if !cur.hasKids {
		return m
	}
	m.expanded[cur.id] = !m.expanded[cur.id]
	return m.reflatten()
}

func (m Model[T]) reflatten() Model[T] {
	prevID := m.SelectedID()
	m.rows = flatten(m.nodes, m.expanded)
	if prevID != "" {
		for i, r := range m.rows {
			if r.id == prevID {
				m.cursor = i
				return m
			}
		}
	}
	m.cursor = clamp(m.cursor, 0, len(m.rows)-1)
	return m
}

func (m Model[T]) emitSelection() tea.Cmd {
	if len(m.rows) == 0 {
		return nil
	}
	cur := m.rows[m.cursor]
	return func() tea.Msg { return SelectMsg[T]{ID: cur.id, Item: cur.item} }
}

// visibleWindow computes the slice of rows currently in view, keeping
// the cursor visible. Simple scroll behavior — the cursor sits roughly
// in the middle when scrolling, at the edges when near the top/bottom.
type window struct{ top, bottom int }

func (m Model[T]) visibleWindow() window {
	if m.h <= 0 || len(m.rows) <= m.h {
		return window{top: 0, bottom: len(m.rows) - 1}
	}
	// Keep cursor in view, scrolling as needed
	half := m.h / 2
	top := m.cursor - half
	if top < 0 {
		top = 0
	}
	bottom := top + m.h - 1
	if bottom >= len(m.rows) {
		bottom = len(m.rows) - 1
		top = bottom - m.h + 1
	}
	return window{top: top, bottom: bottom}
}

// ---------- Pure helpers ----------

func flatten[T any](nodes []Node[T], expanded map[string]bool) []row[T] {
	var rows []row[T]
	var walk func(n Node[T], depth int)
	walk = func(n Node[T], depth int) {
		r := row[T]{
			id:       n.ID,
			item:     n.Item,
			depth:    depth,
			hasKids:  len(n.Children) > 0,
			expanded: expanded[n.ID],
		}
		rows = append(rows, r)
		if r.hasKids && r.expanded {
			for _, c := range n.Children {
				walk(c, depth+1)
			}
		}
	}
	for _, root := range nodes {
		walk(root, 0)
	}
	return rows
}

func walk[T any](nodes []Node[T], fn func(Node[T])) {
	for _, n := range nodes {
		fn(n)
		walk(n.Children, fn)
	}
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
