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
	AutoReview    bool
	// Loaded is false until the first inbox poll answers; LoadError carries the
	// last poll failure so the page never shows "loading" or "nothing" for an
	// inbox it could not fetch.
	Loaded    bool
	LoadError string
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
	// showHidden lists bot-authored and dismissed PRs too; off by default because
	// they are noise next to reviews a person is waiting for.
	showHidden bool
	width      int
	height     int
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
	case key.Matches(keyMsg, DismissKeyBinding):
		if pr, ok := m.selectedPR(); ok {
			return m, shared.Emit(shared.PullRequestDismissMsg{PR: pr})
		}
	case key.Matches(keyMsg, BrowserKeyBinding):
		if pr, ok := m.selectedPR(); ok {
			return m, shared.Emit(shared.PullRequestOpenInBrowserMsg{PR: pr})
		}
	case key.Matches(keyMsg, AutoReviewKeyBinding):
		return m, shared.Emit(shared.AutoReviewToggleMsg{})
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
	listed, hidden := m.listed()
	title := s.SectionTitle.Render(fmt.Sprintf("Pull requests waiting on %s", m.data.Inbox.Viewer))
	count := fmt.Sprintf("%d open", len(listed))
	switch {
	case hidden > 0 && m.showHidden:
		count += fmt.Sprintf("  (%d bot or dismissed, h hides them)", hidden)
	case hidden > 0:
		count += fmt.Sprintf("  (%d bot or dismissed hidden, h shows them)", hidden)
	}
	autoReview := s.Warn.Render("auto-review off")
	if m.data.AutoReview {
		autoReview = s.Good.Render("auto-review on")
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", s.Hint.Render(count), "  ", autoReview)
	switch {
	case m.data.LoadError != "":
		return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, "", s.Bad.Render("Could not load the inbox: "+m.data.LoadError), s.Hint.Render("Retrying on the next poll.")))
	case !m.data.Loaded:
		return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, "", s.Hint.Render("Loading pull requests…")))
	case len(listed) == 0:
		return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, "", s.Hint.Render("Nothing waiting on you.")))
	}
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, "", m.tree.View()))
}

// listed returns the PRs the page shows and how many of the inbox are bot or
// dismissed PRs, the ones h toggles.
func (m Model) listed() (shown []domain.PullRequest, hidden int) {
	for _, pr := range m.data.Inbox.PullRequests {
		if pr.AuthorIsBot || pr.Dismissed {
			hidden++
			if !m.showHidden {
				continue
			}
		}
		shown = append(shown, pr)
	}
	return shown, hidden
}

func (m Model) nodes() []components.TreeNode[prNode] {
	shown, _ := m.listed()
	byRepo := make(map[string][]domain.PullRequest)
	for _, pr := range shown {
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
	case pr.Dismissed:
		node.session = "dismissed"
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
		// Segments are styled only on an unfocused row: the selected row is one
		// style over plain text, so its colours cover the whole line.
		paint := func(style lipgloss.Style, text string) string {
			if focused {
				return text
			}
			return style.Render(text)
		}
		lead := prefix + paint(prStateStyle(d, pr.State), prStateGlyph(g, pr.State)) + " " + paint(d.Glyph, fmt.Sprintf("#%-5d", pr.Number))
		checks, checksStyle := checksGlyph(d, g, pr.Checks)
		tail := strings.Join([]string{
			paint(d.Value, pr.Author),
			paint(d.Hint, shared.FormatRelativeDuration(time.Since(pr.UpdatedAt))),
			paint(checksStyle, checks),
			sessionColumn(s, item, paint),
		}, strings.Repeat(" ", columnGap))

		titleW := max(width-lipgloss.Width(lead)-lipgloss.Width(tail)-2*columnGap, titleMin)
		title := paint(d.Value, fitTitle(pr.Title, titleW))
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

func sessionColumn(s *styles.Styles, item prNode, paint func(lipgloss.Style, string) string) string {
	if item.hasSess {
		return s.Glyphs.AgentStatus(item.status) + " " + paint(s.SessionDetails.Special, item.session)
	}
	return paint(s.SessionDetails.Hint, fmt.Sprintf("%-13s", item.session))
}

func checksGlyph(d styles.SessionDetailsStyles, g styles.Glyphs, c domain.PRChecks) (string, lipgloss.Style) {
	switch c.OverallConclusion() {
	case domain.CheckConclusionSuccess:
		return g.CheckPass, d.Good
	case domain.CheckConclusionFailure:
		return g.CheckFail, d.Bad
	default:
		return g.CheckPending, d.Warn
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
