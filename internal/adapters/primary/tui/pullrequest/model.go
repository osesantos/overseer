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
	markdown    styles.Markdown
	width       int
	height      int
}

func New(s *styles.Styles) Model {
	m := Model{styles: s}
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
	if m.markdown.Width() != max(width, 1) {
		m.markdown = m.styles.NewMarkdown(max(width, 1))
		m.render()
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(keyMsg, CloseKeyBinding):
		return m, shared.Emit(shared.PageCloseMsg{})
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
		m.viewports[sectionDiff].SetContent(m.renderDiffSection())
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
	header := lipgloss.JoinHorizontal(lipgloss.Top, strings.Join(labels, ""), "  ", title)
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, "", m.viewports[m.section].View()))
}

func (m *Model) render() {
	m.viewports[sectionOverview].SetContent(m.renderOverview())
	m.viewports[sectionDiff].SetContent(m.renderDiffSection())
	m.viewports[sectionComments].SetContent(m.renderCommentsSection())
}

func (m Model) renderOverview() string {
	d := m.styles.SessionDetails
	// The strip already shows the title, so the overview starts at the meta line.
	lines := append(components.PullRequestHeader(m.styles, m.pr)[1:], "", m.description(), "")
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

func (m Model) renderDiffSection() string {
	if pending, ok := m.pendingLine(); ok {
		return pending
	}
	if strings.TrimSpace(m.details.Diff) == "" {
		return m.styles.PullRequest.Hint.Render("No diff")
	}
	return renderDiff(m.styles, m.details.Diff, m.width, m.diffIsSplit())
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
