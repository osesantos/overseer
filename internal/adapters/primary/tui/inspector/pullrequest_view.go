package inspector

import (
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/components"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
)

// pullRequestView is the PR tab: the selected session's pull request header and
// description, read-only. It never fetches anything itself — it renders the
// per-minute status the dashboard already receives, so the tab costs no extra
// gh call. Enter on it opens the full-screen page, which is where the diff and
// the comments live.
type pullRequestView struct {
	sessionID     uuid.UUID
	pr            domain.PullRequest
	markdown      styles.Markdown
	rendered      string
	width, height int
	styles        *styles.Styles
}

func newPullRequestView(s *styles.Styles) *pullRequestView {
	return &pullRequestView{styles: s}
}

func (v *pullRequestView) Label() string { return "PR" }

func (v *pullRequestView) Init() tea.Cmd { return nil }

func (v *pullRequestView) hasPR() bool { return v.pr.Number > 0 }

// Update takes the session's PR status. A not-found answer hides the tab; any
// other failure keeps the last known PR, since gh hiccups every so often.
func (v *pullRequestView) Update(msg tea.Msg) (View, tea.Cmd) {
	status, ok := msg.(shared.PRStatusUpdatedMsg)
	if !ok || status.SessionID != v.sessionID || v.sessionID == uuid.Nil {
		return v, nil
	}
	switch {
	case status.Err == nil:
		v.pr = status.PR
		v.render()
	case errors.Is(status.Err, domain.ErrPullRequestNotFound):
		v.pr = domain.PullRequest{}
		v.rendered = ""
	}
	return v, nil
}

func (v *pullRequestView) Body() string {
	if v.sessionID == uuid.Nil {
		return components.CenteredContent(v.styles, v.styles.EmptyState.Title.Render("Select a session to preview"), v.width, v.height)
	}
	if !v.hasPR() {
		return components.CenteredContent(v.styles, v.styles.EmptyState.Title.Render("No pull request for this session"), v.width, v.height)
	}

	lines := components.PullRequestHeader(v.styles, v.pr)
	if status := components.PullRequestStatusLines(v.styles, v.pr); len(status) > 0 {
		lines = append(append(lines, ""), status...)
	}
	lines = append(lines, "")
	description := v.rendered
	if strings.TrimSpace(v.pr.Body) == "" {
		description = v.styles.PullRequest.Hint.Render("No description")
	}
	lines = append(lines, strings.Split(description, "\n")...)
	if limit := max(v.height-1, 1); len(lines) > limit {
		lines = lines[:limit]
	}
	return strings.Join(lines, "\n") + "\n" + v.styles.Help.Description.Render("⏎ expand")
}

func (v *pullRequestView) SetSize(width, height int) {
	v.width = width
	v.height = height
	if v.markdown.Width() != max(width, 1) {
		v.markdown = v.styles.NewMarkdown(max(width, 1))
		v.render()
	}
}

func (v *pullRequestView) SetSession(sess domain.Session) {
	v.sessionID = sess.ID
	v.pr = domain.PullRequest{}
	v.rendered = ""
}

func (v *pullRequestView) render() {
	v.rendered = v.markdown.Render(v.pr.Body)
}
