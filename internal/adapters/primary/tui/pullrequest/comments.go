package pullrequest

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/dnlopes/overseer/internal/core/domain"
)

// hunkTailLines is how much of a thread's diff hunk is shown: the last lines
// are the ones the comment is anchored to.
const hunkTailLines = 6

// renderCommentsSection lists open review threads in full, resolved or
// outdated ones as a single line each, then the conversation with bot
// comments left out. A rule separates every entry; hidden counts are shown so
// nothing vanishes silently.
func (m Model) renderCommentsSection() string {
	if pending, ok := m.pendingLine(); ok {
		return pending
	}
	d := m.styles.SessionDetails
	ps := m.styles.PullRequest
	rule := m.styles.Chat.Separator.Render(strings.Repeat("─", max(m.width, 1)))

	var open, collapsed []domain.PRReviewThread
	for _, th := range m.details.Threads {
		if th.Resolved || th.Outdated {
			collapsed = append(collapsed, th)
		} else {
			open = append(open, th)
		}
	}
	var human []domain.PRComment
	bots := 0
	for _, c := range m.details.Conversation {
		if c.AuthorIsBot {
			bots++
			continue
		}
		human = append(human, c)
	}
	hidden := ""
	if bots > 0 {
		hidden = fmt.Sprintf(" (%d bot comment%s hidden)", bots, plural(bots))
	}
	if len(m.details.Threads) == 0 && len(human) == 0 {
		return ps.Hint.Render("No comments" + hidden)
	}

	var lines []string
	if len(m.details.Threads) > 0 {
		lines = append(lines, d.SectionTitle.Render(fmt.Sprintf("Review threads · %d open · %d resolved", len(open), len(collapsed))), rule)
		for _, th := range open {
			lines = append(lines, m.renderOpenThread(th)...)
			lines = append(lines, rule)
		}
		for _, th := range collapsed {
			state := "resolved"
			if !th.Resolved {
				state = "outdated"
			}
			lines = append(lines, ps.Thread.Resolved.Render(fmt.Sprintf("%s %s · %s · %d comment%s · %s",
				m.styles.Glyphs.CheckPass, threadLocation(th), strings.Join(th.Authors, ", "), len(th.Comments), plural(len(th.Comments)), state)), rule)
		}
		lines = append(lines, "")
	}
	lines = append(lines, d.SectionTitle.Render(fmt.Sprintf("Conversation · %d%s", len(human), hidden)), rule)
	for _, c := range human {
		lines = append(lines, m.renderComment(c)...)
		lines = append(lines, rule)
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderOpenThread(th domain.PRReviewThread) []string {
	ps := m.styles.PullRequest
	lines := []string{ps.Thread.Header.Render(m.styles.Glyphs.CheckPending + " " + threadLocation(th))}
	if hunk := strings.TrimRight(th.DiffHunk, "\n"); hunk != "" {
		hunkLines := strings.Split(hunk, "\n")
		if len(hunkLines) > hunkTailLines {
			hunkLines = hunkLines[len(hunkLines)-hunkTailLines:]
		}
		for _, l := range hunkLines {
			lines = append(lines, ps.Thread.Hunk.Render(ansi.Truncate("  "+strings.ReplaceAll(l, "\t", "    "), max(m.width, 1), "…")))
		}
	}
	for _, c := range th.Comments {
		if c.AuthorIsBot {
			continue
		}
		lines = append(lines, m.renderComment(c)...)
	}
	return lines
}

// renderComment is a blank line, the author line, then the body as markdown.
func (m Model) renderComment(c domain.PRComment) []string {
	ps := m.styles.PullRequest
	head := ps.Thread.Author.Render(c.Author)
	if c.ReviewState != "" {
		head += ps.Hint.Render(" · reviewed: " + strings.ToLower(c.ReviewState))
	}
	if !c.CreatedAt.IsZero() {
		head += ps.Hint.Render(" · " + c.CreatedAt.Local().Format("2006-01-02 15:04"))
	}
	return []string{"", head, m.markdown.Render(c.Body)}
}

func threadLocation(th domain.PRReviewThread) string {
	if th.Line > 0 {
		return fmt.Sprintf("%s:%d", th.Path, th.Line)
	}
	return th.Path
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
