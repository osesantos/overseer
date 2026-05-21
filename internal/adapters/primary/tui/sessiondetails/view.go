package sessiondetails

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
)

const (
	glyphFeatureBranch = "⎇"
	glyphBaseBranch    = "↳"
	glyphRepo          = "⊞"
	glyphPRLink        = "⎘"
	glyphPRStateDot    = "●"
	glyphAdded         = "⊕"
	glyphRemoved       = "⊖"
	glyphFiles         = "▤"
	glyphCheckPass     = "✓"
	glyphCheckFail     = "✗"
	glyphCheckPending  = "◷"

	titlePullRequest = "Pull Request"

	labelRepository    = "Repository"
	labelFeatureBranch = "Feature Branch"
	labelBaseBranch    = "Base Branch"
	labelStatus        = "Status"
	labelLink          = "Link"
	labelChanges       = "Changes"
	labelChecks        = "Checks"

	// labelColumnWidth fixes the width of the left ("label") column so
	// every row in both sections lines up at the same value start column.
	// 14 = lipgloss.Width("Feature Branch"), the widest label in the panel.
	labelColumnWidth = 14
	columnGap        = 2
)

func (m Model) renderContent(width int) string {
	if m.session == nil {
		return m.styles.SessionDetails.Hint.Render("Select a session")
	}
	if !m.session.HasWorktree() {
		return ""
	}

	sections := [][]string{m.renderRepositorySection(width)}
	if pr := m.renderPRSection(width); pr != nil {
		sections = append(sections, pr)
	}
	return "\n" + joinSections(sections)
}

// joinSections concatenates the section blocks with ONE blank line between
// them. Each block already ends with its own trailing blank (from the last
// field group), so the gap between two sections ends up as two blank lines
// — slightly more breathing room between sections than within them.
func joinSections(sections [][]string) string {
	parts := make([]string, 0, len(sections)*2)
	for i, sec := range sections {
		if len(sec) == 0 {
			continue
		}
		if i > 0 {
			parts = append(parts, "")
		}
		parts = append(parts, sec...)
	}
	return strings.Join(parts, "\n")
}

func sectionHeader(s *styles.SessionDetailsStyles, title string, width int) []string {
	if width <= 0 {
		return nil
	}
	header := s.SectionTitle.Render(truncate(title, width))
	divider := s.SectionDivider.Render(strings.Repeat("─", width))
	return []string{header, divider, ""}
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
	valueW := twoColumnValueWidth(width)
	rows := []string{}

	if repo := repoSlugFromPR(m.prCache[m.session.ID].PR.URL); repo != "" {
		rows = append(rows, twoColumnRow(s, labelRepository, glyphLine(s, glyphRepo, repo, valueW)))
	}

	if m.session.IsCheckout() {
		rows = append(rows, twoColumnRow(s, labelBaseBranch, glyphLine(s, glyphBaseBranch, m.session.BaseBranch+"  (tracking)", valueW)))
		return append(rows, "")
	}
	rows = append(rows, twoColumnRow(s, labelFeatureBranch, glyphLine(s, glyphFeatureBranch, m.session.FeatureBranch, valueW)))
	rows = append(rows, twoColumnRow(s, labelBaseBranch, glyphLine(s, glyphBaseBranch, m.session.BaseBranch, valueW)))
	return append(rows, "")
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
	valueW := twoColumnValueWidth(width)
	rows := sectionHeader(s, titlePullRequest, width)

	statusValue := prStateStyle(s, pr.PR.State).Render(glyphPRStateDot+" "+formatPRState(pr.PR.State)) +
		"  " + s.Glyph.Render(fmt.Sprintf("#%d", pr.PR.Number))
	rows = append(rows, twoColumnRow(s, labelStatus, statusValue))

	if pr.PR.URL != "" {
		rows = append(rows, twoColumnRow(s, labelLink, pathLine(s, glyphPRLink, pr.PR.URL, valueW)))
	}

	changesValue := s.Good.Render(fmt.Sprintf("%s +%d", glyphAdded, pr.PR.Stats.Additions)) +
		"   " + s.Bad.Render(fmt.Sprintf("%s -%d", glyphRemoved, pr.PR.Stats.Deletions)) +
		"   " + s.Glyph.Render(fmt.Sprintf("%s %d files", glyphFiles, pr.PR.Stats.ChangedFiles))
	rows = append(rows, twoColumnRow(s, labelChanges, changesValue))

	if checksValue := renderChecksLine(s, pr.PR.Checks); checksValue != "" {
		rows = append(rows, twoColumnRow(s, labelChecks, checksValue))
	}
	return append(rows, "")
}

func renderChecksLine(s *styles.SessionDetailsStyles, c domain.PRChecks) string {
	if c.Total == 0 {
		return ""
	}
	var parts []string
	if c.Passing > 0 {
		parts = append(parts, s.Good.Render(fmt.Sprintf("%s %d", glyphCheckPass, c.Passing)))
	}
	if c.Failing > 0 {
		parts = append(parts, s.Bad.Render(fmt.Sprintf("%s %d", glyphCheckFail, c.Failing)))
	}
	if c.Pending > 0 {
		parts = append(parts, s.Warn.Render(fmt.Sprintf("%s %d", glyphCheckPending, c.Pending)))
	}
	return strings.Join(parts, "   ")
}

func prStateStyle(s *styles.SessionDetailsStyles, state domain.PRState) lipgloss.Style {
	switch state {
	case domain.PRStateOpen:
		return s.Good
	case domain.PRStateMerged:
		return s.Special
	case domain.PRStateClosed:
		return s.Bad
	case domain.PRStateDraft:
		return s.Warn
	}
	return s.Glyph
}

// formatPRState turns the uppercase domain enum value ("OPEN", "DRAFT", …)
// into a title-cased display label ("Open", "Draft", …). Keeps the
// underlying enum stable for storage / comparison while making the UI
// read naturally.
func formatPRState(state domain.PRState) string {
	str := string(state)
	if str == "" {
		return ""
	}
	return strings.ToUpper(str[:1]) + strings.ToLower(str[1:])
}

// repoSlugFromPR extracts the "owner/repo" slug from a GitHub-style PR URL
// like "https://github.com/owner/repo/pull/123". Returns "" if the URL is
// empty or does not contain "/pull/" — the local domain has no other
// source for the remote URL, so callers omit the line in that case.
func repoSlugFromPR(prURL string) string {
	if prURL == "" {
		return ""
	}
	idx := strings.Index(prURL, "/pull/")
	if idx <= 0 {
		return ""
	}
	parts := strings.Split(prURL[:idx], "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-2] + "/" + parts[len(parts)-1]
}

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
