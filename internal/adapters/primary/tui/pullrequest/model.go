// Package pullrequest is the full-screen pull request page: Overview (header,
// description, files), Diff and Comments, one scrollable section at a time.
// It is opened with Enter on the inspector's PR tab or with :pr, and renders
// only what the dashboard hands it — it never calls a service.
package pullrequest

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/components"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
)

// headerHeight is the section strip plus the blank line under it.
const headerHeight = 2

type section int

const (
	sectionOverview section = iota
	sectionDiff
	sectionComments
	sectionCount
)

func (s section) label() string {
	return [...]string{"Overview", "Diff", "Comments"}[s]
}

// Model keeps one viewport per section so each remembers its own scroll
// position; the content is rendered once per load or resize, never per key.
type Model struct {
	styles    *styles.Styles
	pr        domain.PullRequest
	details   domain.PullRequestDetails
	loaded    bool
	err       error
	section   section
	viewports [sectionCount]viewport.Model
	// unifiedDiff is the operator's s toggle; the zero value is the split view.
	unifiedDiff bool
	// search is the / prompt. query is the committed search, hits the diff rows
	// it paints and hit the one n/N last jumped to.
	search    textinput.Model
	searching bool
	query     string
	hits      []int
	hit       int
	markdown  styles.Markdown
	width     int
	height    int
}

func New(s *styles.Styles) Model {
	search := textinput.New()
	search.Prompt = "/"
	search.SetStyles(s.Form.Input)
	m := Model{styles: s, search: search}
	for i := range m.viewports {
		m.viewports[i] = viewport.New()
	}
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Section() section { return m.section }

// diffIsSplit reports whether the Diff section shows old and new side by side:
// the operator has not toggled it off and the page is wide enough.
func (m Model) diffIsSplit() bool { return !m.unifiedDiff && m.width >= splitMinWidth }

// PullRequest is the PR on screen, from the details once they are loaded.
func (m Model) PullRequest() domain.PullRequest { return m.pr }

// CapturesInput reports that the / prompt owns the keyboard, so the dashboard
// must not read q or : as its own shortcuts.
func (m Model) CapturesInput() bool { return m.searching }

// UpdatedAt is the loaded details' timestamp; zero until details arrive. The
// dashboard compares it with the per-minute status to decide on a refetch.
func (m Model) UpdatedAt() time.Time {
	if !m.loaded {
		return time.Time{}
	}
	return m.details.PullRequest.UpdatedAt
}

// SetPullRequest starts the page on a PR whose details are still loading.
func (m *Model) SetPullRequest(pr domain.PullRequest) {
	m.pr = pr
	m.details = domain.PullRequestDetails{}
	m.loaded = false
	m.err = nil
	m.section = sectionOverview
	m.searching = false
	m.query = ""
	m.render()
	for i := range m.viewports {
		m.viewports[i].GotoTop()
	}
}

func (m *Model) SetDetails(details domain.PullRequestDetails) {
	m.details = details
	if details.PullRequest.Number > 0 {
		m.pr = details.PullRequest
	}
	m.loaded = true
	m.err = nil
	m.render()
}

func (m *Model) SetError(err error) {
	m.err = err
	m.render()
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	for i := range m.viewports {
		m.viewports[i].SetWidth(max(width, 1))
		m.viewports[i].SetHeight(max(height-headerHeight, 1))
	}
	m.search.SetWidth(max(width-1, 1))
	if m.markdown.Width() != max(width, 1) {
		m.markdown = m.styles.NewMarkdown(max(width, 1))
		m.render()
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.searching {
		return m.updateSearch(msg)
	}
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(keyMsg, CloseKeyBinding):
		if m.query != "" {
			m.setQuery("")
			return m, nil
		}
		return m, shared.Emit(shared.PageCloseMsg{})
	case key.Matches(keyMsg, SearchKeyBinding) && m.section == sectionDiff:
		m.searching = true
		m.search.SetValue("")
		return m, m.search.Focus()
	case key.Matches(keyMsg, NextHitKeyBinding):
		m.jumpHit(1)
	case key.Matches(keyMsg, PrevHitKeyBinding):
		m.jumpHit(-1)
	case key.Matches(keyMsg, NextSectionKeyBinding):
		m.section = (m.section + 1) % sectionCount
	case key.Matches(keyMsg, PrevSectionKeyBinding):
		m.section = (m.section + sectionCount - 1) % sectionCount
	case key.Matches(keyMsg, TopKeyBinding):
		m.viewports[m.section].GotoTop()
	case key.Matches(keyMsg, BottomKeyBinding):
		m.viewports[m.section].GotoBottom()
	case key.Matches(keyMsg, SplitKeyBinding):
		m.unifiedDiff = !m.unifiedDiff
		m.renderDiffSection()
	case key.Matches(keyMsg, RefetchKeyBinding):
		return m, shared.Emit(shared.PullRequestRefetchMsg{})
	case key.Matches(keyMsg, BrowserKeyBinding):
		return m, shared.Emit(shared.PullRequestOpenInBrowserMsg{PR: m.pr})
	default:
		var cmd tea.Cmd
		m.viewports[m.section], cmd = m.viewports[m.section].Update(keyMsg)
		return m, cmd
	}
	return m, nil
}

// updateSearch owns every message while the / prompt is open: enter commits
// the line, esc abandons it, anything else (paste included) edits it.
func (m Model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(keyMsg, searchCancelKeyBinding):
			m.searching = false
			m.search.Blur()
			return m, nil
		case key.Matches(keyMsg, searchSubmitKeyBinding):
			m.searching = false
			m.search.Blur()
			m.setQuery(m.search.Value())
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	return m, cmd
}

// setQuery repaints the diff and, like vim, jumps to the first hit at or
// below the current scroll position, wrapping to the first one.
func (m *Model) setQuery(query string) {
	m.query = query
	m.renderDiffSection()
	m.hit = 0
	top := m.viewports[sectionDiff].YOffset()
	for i, row := range m.hits {
		if row >= top {
			m.hit = i
			break
		}
	}
	m.jumpHit(0)
}

// jumpHit moves delta hits forward or back, wrapping, and scrolls the diff so
// that hit is the top row.
func (m *Model) jumpHit(delta int) {
	n := len(m.hits)
	if n == 0 {
		return
	}
	m.hit = ((m.hit+delta)%n + n) % n
	m.viewports[sectionDiff].SetYOffset(m.hits[m.hit])
}

// searchStatus is the header's "/query · 2/7" once a query is committed.
func (m Model) searchStatus() string {
	if len(m.hits) == 0 {
		return "/" + m.query + " · no matches"
	}
	return fmt.Sprintf("/%s · %d/%d", m.query, m.hit+1, len(m.hits))
}

func (m Model) View() tea.View {
	ps := m.styles.PullRequest
	labels := make([]string, 0, sectionCount)
	for s := section(0); s < sectionCount; s++ {
		if s == m.section {
			labels = append(labels, ps.Section.Active.Render(s.label()))
		} else {
			labels = append(labels, ps.Section.Inactive.Render(s.label()))
		}
	}
	title := ps.Title.Render(fmt.Sprintf("#%d %s", m.pr.Number, m.pr.Title))
	switch {
	case m.searching:
		title = m.search.View()
	case m.query != "":
		title += "  " + ps.Hint.Render(m.searchStatus())
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top, strings.Join(labels, ""), "  ", title)
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, "", m.viewports[m.section].View()))
}

func (m *Model) render() {
	m.viewports[sectionOverview].SetContent(m.renderOverview())
	m.renderDiffSection()
	m.viewports[sectionComments].SetContent(m.renderCommentsSection())
}

func (m Model) renderOverview() string {
	d := m.styles.SessionDetails
	// The strip already shows the title, so the overview starts at the meta line.
	lines := append(components.PullRequestHeader(m.styles, m.pr)[1:], m.reviewLines()...)
	lines = append(lines, "", m.description(), "")
	if pending, ok := m.pendingLine(); ok {
		return strings.Join(append(lines, pending), "\n")
	}
	lines = append(lines, d.SectionTitle.Render(fmt.Sprintf("%d files", len(m.details.Files))))
	for _, f := range m.details.Files {
		lines = append(lines, fmt.Sprintf("%s %s  %s",
			d.Good.Render(fmt.Sprintf("+%-4d", f.Additions)),
			d.Bad.Render(fmt.Sprintf("-%-4d", f.Deletions)),
			d.Value.Render(f.Path)))
	}
	return strings.Join(lines, "\n")
}

// reviewLines is the merge verdict and the reviewer list, shown once the
// details are in for a PR that can still be merged.
func (m Model) reviewLines() []string {
	if !m.loaded || (m.pr.State != domain.PRStateOpen && m.pr.State != domain.PRStateDraft) {
		return nil
	}
	d := m.styles.SessionDetails
	lines := []string{"", m.readinessLine(), "", d.SectionTitle.Render("Reviewers")}
	width := 0
	for _, r := range m.pr.Reviewers {
		if r.Login != m.pr.Author {
			width = max(width, lipgloss.Width(r.Login))
		}
	}
	if width == 0 {
		return append(lines, m.styles.PullRequest.Hint.Render("No reviewers"))
	}
	for _, r := range m.pr.Reviewers {
		if r.Login == m.pr.Author {
			continue
		}
		glyph, style, label := m.reviewerLook(r.State)
		lines = append(lines, style.Render(glyph)+" "+d.Value.Render(fmt.Sprintf("%-*s", width, r.Login))+"  "+style.Render(label))
	}
	return lines
}

func (m Model) readinessLine() string {
	d, g := m.styles.SessionDetails, m.styles.Glyphs
	r := m.pr.MergeReadiness()
	switch {
	case !r.Known:
		return m.styles.PullRequest.Hint.Render("Merge state not computed yet")
	case r.Ready && len(r.Reasons) == 0:
		return d.Good.Render(g.CheckPass + " Ready to merge")
	case r.Ready:
		return d.Warn.Render(g.Warning + " Mergeable · " + strings.Join(r.Reasons, " · "))
	default:
		return d.Bad.Render(g.CheckFail + " Blocked · " + strings.Join(r.Reasons, " · "))
	}
}

func (m Model) reviewerLook(state domain.PRReviewState) (glyph string, style lipgloss.Style, label string) {
	d, g := m.styles.SessionDetails, m.styles.Glyphs
	switch state {
	case domain.PRReviewStateApproved:
		return g.CheckPass, d.Good, "approved"
	case domain.PRReviewStateChangesRequested:
		return g.CheckFail, d.Bad, "changes requested"
	case domain.PRReviewStateRequested:
		return g.CheckPending, d.Warn, "review requested"
	case domain.PRReviewStatePending:
		return g.CheckPending, d.Warn, "review not submitted"
	case domain.PRReviewStateDismissed:
		return g.ProjectMode, d.Hint, "approval dismissed"
	default:
		return g.ProjectMode, d.Hint, "commented"
	}
}

// renderDiffSection fills the diff viewport and records the search hits; it
// runs on load, resize, the s toggle and every new query, never per key.
func (m *Model) renderDiffSection() {
	content, hits := "", []int(nil)
	switch {
	case m.err != nil || !m.loaded:
		content, _ = m.pendingLine()
	case strings.TrimSpace(m.details.Diff) == "":
		content = m.styles.PullRequest.Hint.Render("No diff")
	default:
		content, hits = renderDiff(m.styles, m.details.Diff, m.width, m.diffIsSplit(), m.query)
	}
	m.viewports[sectionDiff].SetContent(content)
	m.hits = hits
	m.hit = min(m.hit, max(len(hits)-1, 0))
}

func (m Model) description() string {
	if strings.TrimSpace(m.pr.Body) == "" {
		return m.styles.PullRequest.Hint.Render("No description")
	}
	return m.markdown.Render(m.pr.Body)
}

// pendingLine is what every section shows instead of its content while the
// details are loading or failed to load.
func (m Model) pendingLine() (string, bool) {
	switch {
	case m.err != nil:
		return m.styles.SessionDetails.Bad.Render("Could not load the pull request: "+m.err.Error()) +
			"  " + m.styles.PullRequest.Hint.Render("r retries"), true
	case !m.loaded:
		return m.styles.PullRequest.Hint.Render("Loading diff and comments…"), true
	}
	return "", false
}
