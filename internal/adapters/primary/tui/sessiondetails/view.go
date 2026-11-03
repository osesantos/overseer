package sessiondetails

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/components"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
)

const (
	titlePullRequest = "Pull Request"

	labelRepository = "Repository"
	labelBranch     = "Branch"
	labelStatus     = "Status"
	labelLink       = "Link"
	labelChanges    = "Changes"
	labelChecks     = "Checks"
	labelComments   = "Comments"

	labelColumnWidth = 14
	columnGap        = 2
)

func (m Model) renderContent(width, height int) string {
	if m.session == nil {
		return components.CenteredContent(m.styles, m.styles.SessionDetails.Hint.Render("Select a session"), width, height)
	}

	sections := [][]string{m.renderRepositorySection(width)}
	if pr := m.renderPRSection(width); pr != nil {
		sections = append(sections, pr)
	}
	return strings.Join(slices.Concat(sections...), "\n")
}

func sectionHeader(s *styles.SessionDetailsStyles, title string, width int) []string {
	if width <= 0 {
		return nil
	}
	header := s.SectionTitle.Render(truncate(title, width))
	divider := s.SectionDivider.Render(strings.Repeat("─", width))
	return []string{header, divider}
}

// twoColumnRow renders `label  value` with the label padded to a fixed
// column width so every row in the panel lines up at the same value
// start column. Value is expected to already contain its own glyph and
// styling (from glyphLine / pathLine / compound builders).
func twoColumnRow(s *styles.SessionDetailsStyles, label, value string) string {
	padded := s.FieldLabel.Width(labelColumnWidth).Render(truncate(label, labelColumnWidth))
	return padded + strings.Repeat(" ", columnGap) + value
}

func twoColumnValueWidth(totalW int) int {
	return max(totalW-labelColumnWidth-columnGap, 0)
}

func (m Model) renderRepositorySection(width int) []string {
	s := &m.styles.SessionDetails
	g := m.styles.Glyphs
	valueW := twoColumnValueWidth(width)
	rows := []string{}

	if name := m.projectNames[m.session.ProjectID]; name != "" {
		rows = append(rows, twoColumnRow(s, labelRepository, glyphLine(s, g.Repo, name, valueW)))
	}

	branch, suffix := m.branchValue()
	if branch != "" {
		rows = append(rows, twoColumnRow(s, labelBranch, glyphLine(s, g.Branch, branch+suffix, valueW)))
	}
	return append(rows, "")
}

// branchValue returns the branch name to render and an optional suffix.
// Worktree sessions show their persisted Branch unadorned; project-mode
// sessions show the live HEAD (or "(project mode)" placeholder when the
// adapter has not reported one yet) with a trailing "(live)" hint.
func (m Model) branchValue() (string, string) {
	if m.session.HasWorktree() {
		return m.session.Branch, ""
	}
	if live, ok := m.projectBranches[m.session.ProjectID]; ok && live != "" {
		return live, "  (live)"
	}
	return "(project mode)", ""
}

// renderPRSection returns nil when no PR exists (still fetching or
// confirmed-none), so the caller can omit the entire section — no
// header, no divider, no placeholder. The panel only grows once a PR
// is actually available.
func (m Model) renderPRSection(width int) []string {
	pr, ok := m.prCache[m.session.ID]
	if !ok || pr.PR.Number == 0 {
		return nil
	}

	s := &m.styles.SessionDetails
	g := m.styles.Glyphs
	valueW := twoColumnValueWidth(width)
	rows := sectionHeader(s, titlePullRequest, width)

	stateDot := components.PRStateGlyph(g, pr.PR.State)
	statusValue := components.PRStateStyle(*s, pr.PR.State).Render(stateDot+" "+components.FormatPRState(pr.PR.State)) +
		"  " + s.Glyph.Render(fmt.Sprintf("#%d", pr.PR.Number))
	rows = append(rows, twoColumnRow(s, labelStatus, statusValue))

	if pr.PR.URL != "" {
		rows = append(rows, twoColumnRow(s, labelLink, pathLine(s, g.PRLink, pr.PR.URL, valueW)))
	}

	changesValue := s.Good.Render(fmt.Sprintf("%s +%d", g.Added, pr.PR.Stats.Additions)) +
		"   " + s.Bad.Render(fmt.Sprintf("%s -%d", g.Removed, pr.PR.Stats.Deletions)) +
		"   " + s.Glyph.Render(fmt.Sprintf("%s %d files", g.Files, pr.PR.Stats.ChangedFiles))
	rows = append(rows, twoColumnRow(s, labelChanges, changesValue))

	if checksValue := components.PRChecksLine(*s, g, pr.PR.Checks); checksValue != "" {
		rows = append(rows, twoColumnRow(s, labelChecks, checksValue))
	}

	if commentsValue := components.PRCommentsLine(*s, g, pr.PR.Comments); commentsValue != "" {
		rows = append(rows, twoColumnRow(s, labelComments, commentsValue))
	}
	return append(rows, "")
}

// formatPRState turns the uppercase domain enum value ("OPEN", "DRAFT", …)
// into a title-cased display label ("Open", "Draft", …). Keeps the
// underlying enum stable for storage / comparison while making the UI
// read naturally.
func glyphLine(s *styles.SessionDetailsStyles, glyph, value string, width int) string {
	prefix := s.Glyph.Render(glyph + "  ")
	avail := width - lipgloss.Width(prefix)
	return prefix + s.Value.Render(truncate(value, avail))
}

func pathLine(s *styles.SessionDetailsStyles, glyph, path string, width int) string {
	prefix := s.Glyph.Render(glyph + "  ")
	avail := width - lipgloss.Width(prefix)
	return prefix + s.Value.Render(truncatePath(path, avail))
}

// truncate clips s to maxWidth, replacing the trailing characters with "…"
// when truncation occurs. maxWidth ≤ 0 returns empty; maxWidth < 2 returns "…".
func truncate(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth < 2 {
		return "…"
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > maxWidth {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

// truncatePath clips path from the LEFT (keeping the deepest component
// visible), prefixing with "…" when truncation occurs. Useful for long
// URLs and worktree paths where the trailing segment is the meaningful part.
func truncatePath(path string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(path) <= maxWidth {
		return path
	}
	if maxWidth < 2 {
		return "…"
	}
	runes := []rune(path)
	keep := maxWidth - 1
	if keep >= len(runes) {
		return path
	}
	return "…" + string(runes[len(runes)-keep:])
}
