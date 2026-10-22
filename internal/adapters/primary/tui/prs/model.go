// Package prs is the review inbox page: every open PR with a review requested
// from the operator, grouped by repository, with the state of its review session.
package prs

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
	titleMin     = 16
)

type nodeKind int

const (
	nodeRepo nodeKind = iota
	nodePR
)

// Data is everything the page needs to render; the dashboard owns all of it.
type Data struct {
	Inbox         domain.ReviewInbox
	Sessions      []domain.Session
	ProjectNames  map[string]bool
	AgentStatuses map[uuid.UUID]domain.AgentStatus
}

type prNode struct {
	kind    nodeKind
	label   string
	pr      domain.PullRequest
	session string // review session column, empty when none applies
	status  domain.AgentStatusKind
	hasSess bool
}

type Model struct {
	styles *styles.Styles
	tree   components.TreeModel[prNode]
	data   Data
	width  int
	height int
}

func New(s *styles.Styles) Model {
	return Model{styles: s, tree: components.NewTree(renderNode(s)).Focus()}
}

func (m Model) Init() tea.Cmd { return nil }

// SetData replaces the page's data and rebuilds the tree, keeping the cursor on
// the same PR when it is still listed.
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
		if pr, ok := m.selectedPR(); ok {
			return m, shared.Emit(shared.PullRequestOpenMsg{PR: pr})
		}
	case key.Matches(keyMsg, ReviewKeyBinding):
		if pr, ok := m.selectedPR(); ok {
			return m, shared.Emit(shared.PullRequestReviewMsg{PR: pr})
		}
	}
	var cmd tea.Cmd
	m.tree, cmd = m.tree.Update(keyMsg)
	return m, cmd
}

// selectedPR returns the PR under the cursor; a repo group row returns false.
func (m Model) selectedPR() (domain.PullRequest, bool) {
	item, ok := m.tree.Selected()
	if !ok || item.kind != nodePR {
		return domain.PullRequest{}, false
	}
	return item.pr, true
}

func (m Model) View() tea.View {
	s := m.styles.SessionDetails
	title := s.SectionTitle.Render(fmt.Sprintf("Pull requests waiting on %s", m.data.Inbox.Viewer))
	count := s.Hint.Render(fmt.Sprintf("%d open", len(m.data.Inbox.PullRequests)))
	header := lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", count)
	if len(m.data.Inbox.PullRequests) == 0 {
		return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, "", s.Hint.Render("Nothing waiting on you.")))
	}
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, "", m.tree.View()))
}

func (m Model) nodes() []components.TreeNode[prNode] {
	byRepo := make(map[string][]domain.PullRequest)
	for _, pr := range m.data.Inbox.PullRequests {
		byRepo[pr.Repo] = append(byRepo[pr.Repo], pr)
	}
	repos := make([]string, 0, len(byRepo))
	for repo := range byRepo {
		repos = append(repos, repo)
	}
	sort.Strings(repos)

	sessionByPR := make(map[domain.PullRequestRef]domain.Session, len(m.data.Sessions))
	for _, sess := range m.data.Sessions {
		if sess.IsReview() {
			sessionByPR[sess.ReviewPR] = sess
		}
	}

	nodes := make([]components.TreeNode[prNode], 0, len(repos))
	for _, repo := range repos {
		prs := byRepo[repo]
		sort.Slice(prs, func(i, j int) bool { return prs[i].UpdatedAt.After(prs[j].UpdatedAt) })
		children := make([]components.TreeNode[prNode], 0, len(prs))
		for _, pr := range prs {
			children = append(children, components.TreeNode[prNode]{
				ID:   fmt.Sprintf("pr:%s#%d", pr.Repo, pr.Number),
				Item: m.prNode(pr, sessionByPR),
			})
		}
		nodes = append(nodes, components.TreeNode[prNode]{
			ID:       "repo:" + repo,
			Item:     prNode{kind: nodeRepo, label: fmt.Sprintf("%s (%d)", repo, len(prs))},
			Children: children,
		})
	}
	return nodes
}

func (m Model) prNode(pr domain.PullRequest, sessionByPR map[domain.PullRequestRef]domain.Session) prNode {
	node := prNode{kind: nodePR, pr: pr}
	switch sess, ok := sessionByPR[pr.Ref()]; {
	case ok:
		node.hasSess = true
		node.session = "review session"
		if st, found := m.data.AgentStatuses[sess.ID]; found {
			node.status = st.Kind
		}
	case !m.data.ProjectNames[pr.RepoName()]:
		node.session = "no local repo"
	case pr.ReviewedByViewer:
		node.session = "reviewed"
	case pr.Author == m.data.Inbox.Viewer:
		node.session = "mine"
	case pr.AuthorIsBot:
		node.session = "bot"
	}
	return node
}

func renderNode(s *styles.Styles) components.TreeRenderFunc[prNode] {
	return func(item prNode, _, depth, width int, hasKids, expanded, focused bool) string {
		prefix := components.TreePrefix(depth, hasKids, expanded)
		if item.kind == nodeRepo {
			style := s.Group.Header
			if focused {
				style = s.Group.HeaderSelected
			}
			return style.Render(prefix + item.label)
		}

		g := s.Glyphs
		d := s.SessionDetails
		pr := item.pr
		lead := prefix + prStateStyle(d, pr.State).Render(prStateGlyph(g, pr.State)) + " " + d.Glyph.Render(fmt.Sprintf("#%-5d", pr.Number))
		tail := strings.Join([]string{
			d.Value.Render(pr.Author),
			d.Hint.Render(shared.FormatRelativeDuration(time.Since(pr.UpdatedAt))),
			checksGlyph(d, g, pr.Checks),
			sessionColumn(s, item),
		}, strings.Repeat(" ", columnGap))

		titleW := max(width-lipgloss.Width(lead)-lipgloss.Width(tail)-2*columnGap, titleMin)
		title := d.Value.Render(fitTitle(pr.Title, titleW))
		row := lead + " " + title + strings.Repeat(" ", max(width-lipgloss.Width(lead)-1-lipgloss.Width(title)-lipgloss.Width(tail), columnGap)) + tail
		if focused {
			return s.ListRow.Selected.Width(width).Render(row)
		}
		return row
	}
}

func fitTitle(title string, width int) string {
	runes := []rune(strings.TrimSpace(title))
	if len(runes) <= width {
		return string(runes) + strings.Repeat(" ", width-len(runes))
	}
	return string(runes[:width-1]) + "…"
}

func sessionColumn(s *styles.Styles, item prNode) string {
	if item.hasSess {
		return s.Glyphs.AgentStatus(item.status) + " " + s.SessionDetails.Special.Render(item.session)
	}
	return s.SessionDetails.Hint.Render(fmt.Sprintf("%-13s", item.session))
}

func checksGlyph(d styles.SessionDetailsStyles, g styles.Glyphs, c domain.PRChecks) string {
	switch c.OverallConclusion() {
	case domain.CheckConclusionSuccess:
		return d.Good.Render(g.CheckPass)
	case domain.CheckConclusionFailure:
		return d.Bad.Render(g.CheckFail)
	default:
		return d.Warn.Render(g.CheckPending)
	}
}

func prStateStyle(d styles.SessionDetailsStyles, state domain.PRState) lipgloss.Style {
	switch state {
	case domain.PRStateOpen:
		return d.Good
	case domain.PRStateMerged:
		return d.Special
	case domain.PRStateClosed:
		return d.Bad
	default:
		return d.Hint
	}
}

func prStateGlyph(g styles.Glyphs, state domain.PRState) string {
	switch state {
	case domain.PRStateOpen:
		return g.PRStateOpen
	case domain.PRStateMerged:
		return g.PRStateMerged
	case domain.PRStateClosed:
		return g.PRStateClosed
	default:
		return g.PRStateDraft
	}
}
