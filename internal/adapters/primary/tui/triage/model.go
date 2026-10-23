// Package triage is the support-ticket inbox page: every open RDODCF / RDINC
// ticket the triage query returns, grouped by Jira project, with the state of
// its triage session.
package triage

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/components"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
)

const (
	headerHeight = 2
	columnGap    = 2
	summaryMin   = 16
)

type nodeKind int

const (
	nodeProject nodeKind = iota
	nodeIssue
)

// Data is everything the page needs to render; the dashboard owns all of it.
type Data struct {
	Inbox    domain.TriageInbox
	Sessions []domain.Session
	// Launchers are the Jira projects a triage can be started for.
	Launchers     map[string]bool
	AgentStatuses map[uuid.UUID]domain.AgentStatus
	AutoTriage    bool
	// Loaded is false until the first poll answers; LoadError carries the last
	// poll failure.
	Loaded    bool
	LoadError string
}

type issueNode struct {
	kind    nodeKind
	label   string
	issue   domain.Issue
	session string // triage session column, empty when none applies
	status  domain.AgentStatusKind
	hasSess bool
}

type Model struct {
	styles *styles.Styles
	tree   components.TreeModel[issueNode]
	data   Data
	// showHidden lists dismissed tickets too.
	showHidden bool
	width      int
	height     int
}

func New(s *styles.Styles) Model {
	return Model{styles: s, tree: components.NewTree(renderNode(s)).Focus()}
}

func (m Model) Init() tea.Cmd { return nil }

// SetData replaces the page's data and rebuilds the tree, keeping the cursor on
// the same issue when it is still listed.
func (m *Model) SetData(data Data) {
	m.data = data
	m.tree = m.tree.SetNodes(m.nodes()).ExpandAll()
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.tree = m.tree.SetSize(width, max(height-headerHeight, 1))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(keyMsg, CloseKeyBinding):
		return m, shared.Emit(shared.PageCloseMsg{})
	case key.Matches(keyMsg, OpenKeyBinding):
		if issue, ok := m.selectedIssue(); ok {
			return m, shared.Emit(shared.IssueOpenMsg{Issue: issue})
		}
	case key.Matches(keyMsg, TriageKeyBinding):
		if issue, ok := m.selectedIssue(); ok {
			return m, shared.Emit(shared.IssueTriageMsg{Issue: issue})
		}
	case key.Matches(keyMsg, DismissKeyBinding):
		if issue, ok := m.selectedIssue(); ok {
			return m, shared.Emit(shared.IssueDismissMsg{Issue: issue})
		}
	case key.Matches(keyMsg, BrowserKeyBinding):
		if issue, ok := m.selectedIssue(); ok {
			return m, shared.Emit(shared.IssueOpenInBrowserMsg{Issue: issue})
		}
	case key.Matches(keyMsg, AutoTriageKeyBinding):
		return m, shared.Emit(shared.AutoTriageToggleMsg{})
	case key.Matches(keyMsg, HiddenKeyBinding):
		m.showHidden = !m.showHidden
		m.tree = m.tree.SetNodes(m.nodes()).ExpandAll()
		return m, nil
	case key.Matches(keyMsg, TopKeyBinding):
		m.tree = m.tree.SelectIndex(0)
		return m, nil
	case key.Matches(keyMsg, BottomKeyBinding):
		m.tree = m.tree.SelectIndex(m.tree.RowCount() - 1)
		return m, nil
	}
	var cmd tea.Cmd
	m.tree, cmd = m.tree.Update(keyMsg)
	return m, cmd
}

// selectedIssue returns the issue under the cursor; a project row returns false.
func (m Model) selectedIssue() (domain.Issue, bool) {
	item, ok := m.tree.Selected()
	if !ok || item.kind != nodeIssue {
		return domain.Issue{}, false
	}
	return item.issue, true
}

func (m Model) View() tea.View {
	s := m.styles.SessionDetails
	listed, hidden := m.listed()
	count := fmt.Sprintf("%d open", len(listed))
	switch {
	case hidden > 0 && m.showHidden:
		count += fmt.Sprintf("  (%d dismissed, h hides them)", hidden)
	case hidden > 0:
		count += fmt.Sprintf("  (%d dismissed hidden, h shows them)", hidden)
	}
	autoTriage := s.Warn.Render("auto-triage off")
	if m.data.AutoTriage {
		autoTriage = s.Good.Render("auto-triage on")
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top, s.SectionTitle.Render("Support tickets"), "  ", s.Hint.Render(count), "  ", autoTriage)
	switch {
	case m.data.LoadError != "":
		return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, "", s.Bad.Render("Could not load the inbox: "+m.data.LoadError), s.Hint.Render("Retrying on the next poll.")))
	case !m.data.Loaded:
		return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, "", s.Hint.Render("Loading tickets…")))
	case len(listed) == 0:
		return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, "", s.Hint.Render("No open tickets.")))
	}
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, "", m.tree.View()))
}

// listed returns the issues the page shows and how many open ones are
// dismissed, the ones h toggles. Done issues are never listed: they are only in
// the inbox until their triage session is reaped.
func (m Model) listed() (shown []domain.Issue, hidden int) {
	for _, issue := range m.data.Inbox.Issues {
		if issue.StatusCategory == domain.IssueStatusDone {
			continue
		}
		if issue.Dismissed {
			hidden++
			if !m.showHidden {
				continue
			}
		}
		shown = append(shown, issue)
	}
	return shown, hidden
}

func (m Model) nodes() []components.TreeNode[issueNode] {
	shown, _ := m.listed()
	byProject := make(map[string][]domain.Issue)
	for _, issue := range shown {
		byProject[issue.Project] = append(byProject[issue.Project], issue)
	}
	projects := make([]string, 0, len(byProject))
	for project := range byProject {
		projects = append(projects, project)
	}
	sort.Strings(projects)

	sessionByKey := make(map[string]domain.Session, len(m.data.Sessions))
	for _, sess := range m.data.Sessions {
		if sess.IsTriage() {
			sessionByKey[sess.TriageIssue] = sess
		}
	}

	nodes := make([]components.TreeNode[issueNode], 0, len(projects))
	for _, project := range projects {
		issues := byProject[project]
		sort.SliceStable(issues, func(i, j int) bool { return issues[i].CreatedAt.After(issues[j].CreatedAt) })
		children := make([]components.TreeNode[issueNode], 0, len(issues))
		for _, issue := range issues {
			children = append(children, components.TreeNode[issueNode]{
				ID:   "issue:" + issue.Key,
				Item: m.issueNode(issue, sessionByKey),
			})
		}
		nodes = append(nodes, components.TreeNode[issueNode]{
			ID:       "project:" + project,
			Item:     issueNode{kind: nodeProject, label: fmt.Sprintf("%s (%d)", project, len(issues))},
			Children: children,
		})
	}
	return nodes
}

func (m Model) issueNode(issue domain.Issue, sessionByKey map[string]domain.Session) issueNode {
	node := issueNode{kind: nodeIssue, issue: issue}
	switch sess, ok := sessionByKey[issue.Key]; {
	case ok:
		node.hasSess = true
		node.session = "triage session"
		if st, found := m.data.AgentStatuses[sess.ID]; found {
			node.status = st.Kind
		}
	case issue.Dismissed:
		node.session = "dismissed"
	case !m.data.Launchers[issue.Project]:
		node.session = "no launcher"
	case issue.Handled:
		node.session = "handled"
	case issue.Assignee != "":
		node.session = "assigned"
	}
	return node
}

func renderNode(s *styles.Styles) components.TreeRenderFunc[issueNode] {
	return func(item issueNode, _, depth, width int, hasKids, expanded, focused bool) string {
		prefix := components.TreePrefix(depth, hasKids, expanded)
		if item.kind == nodeProject {
			style := s.Group.Header
			if focused {
				style = s.Group.HeaderSelected
			}
			return style.Render(prefix + item.label)
		}

		d := s.SessionDetails
		issue := item.issue
		// Segments are styled only on an unfocused row: the selected row is one
		// style over plain text, so its colours cover the whole line.
		paint := func(style lipgloss.Style, text string) string {
			if focused {
				return text
			}
			return style.Render(text)
		}
		assignee := issue.Assignee
		if assignee == "" {
			assignee = "unassigned"
		}
		age := "?"
		if !issue.CreatedAt.IsZero() {
			age = shared.FormatRelativeDuration(time.Since(issue.CreatedAt))
		}
		lead := prefix + paint(d.Glyph, fmt.Sprintf("%-12s", issue.Key))
		tail := strings.Join([]string{
			paint(statusStyle(d, issue.StatusCategory), issue.Status),
			paint(d.Hint, issue.Priority),
			paint(d.Value, assignee),
			paint(d.Hint, age),
			sessionColumn(s, item, paint),
		}, strings.Repeat(" ", columnGap))

		summaryW := max(width-lipgloss.Width(lead)-lipgloss.Width(tail)-2*columnGap, summaryMin)
		summary := paint(d.Value, fitText(issue.Summary, summaryW))
		row := lead + " " + summary + strings.Repeat(" ", max(width-lipgloss.Width(lead)-1-lipgloss.Width(summary)-lipgloss.Width(tail), columnGap)) + tail
		if focused {
			return s.ListRow.Selected.Width(width).Render(row)
		}
		return row
	}
}

func fitText(text string, width int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= width {
		return string(runes) + strings.Repeat(" ", width-len(runes))
	}
	return string(runes[:width-1]) + "…"
}

func sessionColumn(s *styles.Styles, item issueNode, paint func(lipgloss.Style, string) string) string {
	if item.hasSess {
		return s.Glyphs.AgentStatus(item.status) + " " + paint(s.SessionDetails.Special, item.session)
	}
	return paint(s.SessionDetails.Hint, fmt.Sprintf("%-14s", item.session))
}

func statusStyle(d styles.SessionDetailsStyles, category domain.IssueStatusCategory) lipgloss.Style {
	switch category {
	case domain.IssueStatusNew:
		return d.Warn
	case domain.IssueStatusInProgress:
		return d.Special
	default:
		return d.Hint
	}
}
