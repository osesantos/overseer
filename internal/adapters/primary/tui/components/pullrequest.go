package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/osesantos/skipper/internal/adapters/primary/tui/styles"
	"github.com/osesantos/skipper/internal/core/domain"
)

// PullRequestHeader renders the lines every PR view opens with: title, state
// and number, author and branches, labels, change counts with checks, and the
// review-thread counts. Rows with nothing to say are left out.
func PullRequestHeader(s *styles.Styles, pr domain.PullRequest) []string {
	d, g := s.SessionDetails, s.Glyphs

	meta := []string{
		PRStateStyle(d, pr.State).Render(PRStateGlyph(g, pr.State) + " " + FormatPRState(pr.State)),
		d.Glyph.Render(fmt.Sprintf("#%d", pr.Number)),
	}
	if pr.Author != "" {
		meta = append(meta, d.Value.Render(pr.Author))
	}
	if pr.Branch != "" {
		branches := pr.Branch
		if pr.BaseBranch != "" {
			branches += " → " + pr.BaseBranch
		}
		meta = append(meta, d.Glyph.Render(g.Branch+" "+branches))
	}

	lines := []string{s.PullRequest.Title.Render(pr.Title), strings.Join(meta, "  ")}
	if len(pr.Labels) > 0 {
		lines = append(lines, d.FieldLabel.Render("labels ")+d.Value.Render(strings.Join(pr.Labels, ", ")))
	}
	changes := d.Good.Render(fmt.Sprintf("%s +%d", g.Added, pr.Stats.Additions)) +
		"   " + d.Bad.Render(fmt.Sprintf("%s -%d", g.Removed, pr.Stats.Deletions)) +
		"   " + d.Glyph.Render(fmt.Sprintf("%s %d files", g.Files, pr.Stats.ChangedFiles))
	if checks := PRChecksLine(d, g, pr.Checks); checks != "" {
		changes += "   " + checks
	}
	lines = append(lines, changes)
	if comments := PRCommentsLine(d, g, pr.Comments); comments != "" {
		lines = append(lines, comments)
	}
	return lines
}

// PullRequestStatusLines answers "can it merge, and where do I stand?" for an
// open or draft PR: the merge verdict and the viewer's own review.
func PullRequestStatusLines(s *styles.Styles, pr domain.PullRequest) []string {
	if pr.State != domain.PRStateOpen && pr.State != domain.PRStateDraft {
		return nil
	}
	lines := []string{PRReadinessLine(s, pr)}
	if review := PRViewerReviewLine(s, pr); review != "" {
		lines = append(lines, s.SessionDetails.FieldLabel.Render("you ")+review)
	}
	return lines
}

// PRViewerReviewLine is where the viewer stands on pr; empty when they are not involved.
func PRViewerReviewLine(s *styles.Styles, pr domain.PullRequest) string {
	verdict := pr.ViewerVerdict()
	if verdict == "" {
		return ""
	}
	glyph, style, label := PRReviewLook(s, verdict)
	return style.Render(glyph + " " + label)
}

// PRReadinessLine is GitHub's merge verdict with the reasons for a block.
func PRReadinessLine(s *styles.Styles, pr domain.PullRequest) string {
	d, g := s.SessionDetails, s.Glyphs
	r := pr.MergeReadiness()
	switch {
	case !r.Known:
		return s.PullRequest.Hint.Render("Merge state not computed yet")
	case r.Ready && len(r.Reasons) == 0:
		return d.Good.Render(g.CheckPass + " Ready to merge")
	case r.Ready:
		return d.Warn.Render(g.Warning + " Mergeable · " + strings.Join(r.Reasons, " · "))
	default:
		return d.Bad.Render(g.CheckFail + " Blocked · " + strings.Join(r.Reasons, " · "))
	}
}

// PRReviewLook is the glyph, style and label for a review state.
func PRReviewLook(s *styles.Styles, state domain.PRReviewState) (glyph string, style lipgloss.Style, label string) {
	d, g := s.SessionDetails, s.Glyphs
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

// PRCommentsLine summarises review threads: open (and awaiting the viewer)
// next to resolved; empty when the PR has no threads.
func PRCommentsLine(d styles.SessionDetailsStyles, g styles.Glyphs, c domain.PRComments) string {
	if c.Total() == 0 {
		return ""
	}
	open := fmt.Sprintf("%s %d open", g.CheckPending, c.Unresolved)
	if c.AwaitingViewer > 0 {
		open += fmt.Sprintf(" (%d awaiting you)", c.AwaitingViewer)
	}
	return d.Warn.Render(open) + "   " + d.Good.Render(fmt.Sprintf("%s %d resolved", g.CheckPass, c.Resolved))
}

// PRChecksLine lists passing, failing and pending check counts; empty when
// the PR has no checks.
func PRChecksLine(d styles.SessionDetailsStyles, g styles.Glyphs, c domain.PRChecks) string {
	if c.Total == 0 {
		return ""
	}
	var parts []string
	if c.Passing > 0 {
		parts = append(parts, d.Good.Render(fmt.Sprintf("%s %d", g.CheckPass, c.Passing)))
	}
	if c.Failing > 0 {
		parts = append(parts, d.Bad.Render(fmt.Sprintf("%s %d", g.CheckFail, c.Failing)))
	}
	if c.Pending > 0 {
		parts = append(parts, d.Warn.Render(fmt.Sprintf("%s %d", g.CheckPending, c.Pending)))
	}
	return strings.Join(parts, "   ")
}

func PRStateStyle(d styles.SessionDetailsStyles, state domain.PRState) lipgloss.Style {
	switch state {
	case domain.PRStateOpen:
		return d.Good
	case domain.PRStateMerged:
		return d.Special
	case domain.PRStateClosed:
		return d.Bad
	case domain.PRStateDraft:
		return d.Warn
	}
	return d.Glyph
}

func PRStateGlyph(g styles.Glyphs, state domain.PRState) string {
	switch state {
	case domain.PRStateOpen:
		return g.PRStateOpen
	case domain.PRStateMerged:
		return g.PRStateMerged
	case domain.PRStateClosed:
		return g.PRStateClosed
	case domain.PRStateDraft:
		return g.PRStateDraft
	}
	return g.PRStateOpen
}

// FormatPRState turns the uppercase enum ("OPEN") into a display label ("Open").
func FormatPRState(state domain.PRState) string {
	str := string(state)
	if str == "" {
		return ""
	}
	return strings.ToUpper(str[:1]) + strings.ToLower(str[1:])
}
