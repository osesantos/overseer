package inspector

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"

	"github.com/osesantos/skipper/internal/adapters/primary/tui/shared"
	"github.com/osesantos/skipper/internal/core/domain"
)

func ordinarySession(t *testing.T) domain.Session {
	t.Helper()
	sess, err := domain.NewSession("api", uuid.New())
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	return sess
}

func samplePR() domain.PullRequest {
	pr, _ := domain.NewPullRequest(535, "feat: cron schedules", "feature/cron", domain.PRStateOpen)
	pr.Author = "ana"
	pr.BaseBranch = "main"
	pr.Labels = []string{"feature", "semver-minor"}
	pr.Body = "## Summary\n\nAdds **cron schedules**.\n\n- parses the expression\n- patches the CR back\n"
	pr.Stats = domain.PRStats{Additions: 15, Deletions: 9, ChangedFiles: 3}
	pr.Checks = domain.PRChecks{Total: 3, Passing: 3}
	pr.Comments = domain.PRComments{Resolved: 5, Unresolved: 2, AwaitingViewer: 1}
	return pr
}

func withPRStatus(t *testing.T, m Model, sess domain.Session, pr domain.PullRequest, err error) Model {
	t.Helper()
	updated, _ := m.Update(shared.PRStatusUpdatedMsg{SessionID: sess.ID, PR: pr, Err: err})
	return updated.(Model)
}

func TestInspector_PullRequestTab_AppearsWhenTheStatusCarriesAPR(t *testing.T) {
	sess := ordinarySession(t)
	m := selectSession(t, newTestModel(t), sess)
	if got := visibleLabels(m); !slices.Equal(got, []string{"Agent", "Shell"}) {
		t.Fatalf("tabs before the status = %v", got)
	}

	m = withPRStatus(t, m, sess, samplePR(), nil)

	if got := visibleLabels(m); !slices.Equal(got, []string{"Agent", "Shell", "PR"}) {
		t.Fatalf("tabs after the status = %v, want the PR tab", got)
	}
	updated, _ := m.Update(RevealEditorMsg{})
	if got := visibleLabels(updated.(Model)); !slices.Equal(got, []string{"Agent", "Shell", "PR", "Editor"}) {
		t.Fatalf("tabs with the editor = %v, want Editor last", got)
	}
}

func TestInspector_PullRequestTab_IgnoresAnotherSessionsStatus(t *testing.T) {
	sess := ordinarySession(t)
	m := selectSession(t, newTestModel(t), sess)

	m = withPRStatus(t, m, ordinarySession(t), samplePR(), nil)

	if m.HasPullRequest() {
		t.Fatalf("tabs = %v, want no PR tab for another session's status", visibleLabels(m))
	}
}

func TestInspector_PullRequestTab_HidesWhenTheSelectionChanges(t *testing.T) {
	sess := ordinarySession(t)
	m := withPRStatus(t, selectSession(t, newTestModel(t), sess), sess, samplePR(), nil)

	m = selectSession(t, m, ordinarySession(t))

	if got := visibleLabels(m); !slices.Equal(got, []string{"Agent", "Shell"}) {
		t.Fatalf("tabs after switching session = %v, want no PR tab", got)
	}
}

func TestInspector_PullRequestTab_NotFoundHidesItAndFallsBackToTheFirstTab(t *testing.T) {
	sess := ordinarySession(t)
	m := withPRStatus(t, selectSession(t, newTestModel(t), sess), sess, samplePR(), nil)
	updated, _ := m.Update(ShowPullRequestMsg{})
	m = updated.(Model)
	if m.ActiveViewLabel() != "PR" {
		t.Fatalf("active tab = %q, want PR", m.ActiveViewLabel())
	}

	m = withPRStatus(t, m, sess, domain.PullRequest{}, domain.ErrPullRequestNotFound)

	if m.HasPullRequest() || m.ActiveViewLabel() != "Agent" {
		t.Fatalf("after not-found: has PR %v, active %q; want hidden and Agent", m.HasPullRequest(), m.ActiveViewLabel())
	}
}

func TestInspector_PullRequestTab_OtherErrorsKeepTheLastKnownPR(t *testing.T) {
	sess := ordinarySession(t)
	m := withPRStatus(t, selectSession(t, newTestModel(t), sess), sess, samplePR(), nil)

	m = withPRStatus(t, m, sess, domain.PullRequest{}, errGHDown)

	if !m.HasPullRequest() {
		t.Fatal("a transient gh failure hid the PR tab")
	}
}

func TestInspector_ActivePreviewKind_PullRequestHasNoPane(t *testing.T) {
	sess := ordinarySession(t)
	m := withPRStatus(t, selectSession(t, newTestModel(t), sess), sess, samplePR(), nil)
	updated, _ := m.Update(ShowPullRequestMsg{})
	m = updated.(Model)

	if _, _, ok := m.ActivePreviewKind(); ok {
		t.Fatal("ActivePreviewKind() reported a pane behind the PR tab")
	}
	if pr, ok := m.ShownPullRequest(); !ok || pr.Number != 535 {
		t.Fatalf("ShownPullRequest() = %+v, %v; want #535", pr, ok)
	}
}

func TestInspector_ShownPullRequest_IsFalseOnOtherTabs(t *testing.T) {
	sess := ordinarySession(t)
	m := withPRStatus(t, selectSession(t, newTestModel(t), sess), sess, samplePR(), nil)

	if _, ok := m.ShownPullRequest(); ok {
		t.Fatal("ShownPullRequest() is true while the Agent tab is active")
	}
}

func TestPullRequestView_BodyShowsHeaderAndRenderedDescription(t *testing.T) {
	sess := ordinarySession(t)
	m := newTestModel(t)
	m.SetSize(80, 24)
	m = withPRStatus(t, selectSession(t, m, sess), sess, samplePR(), nil)
	updated, _ := m.Update(ShowPullRequestMsg{})
	m = updated.(Model)

	body := ansi.Strip(m.views[ixPullRequest].Body())

	for _, want := range []string{"feat: cron schedules", "#535", "Open", "ana", "feature/cron → main", "feature, semver-minor", "+15", "-9", "3 files", "2 open (1 awaiting you)", "5 resolved", "Adds cron schedules.", "parses the expression", "⏎ expand"} {
		if !strings.Contains(body, want) {
			t.Errorf("body is missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "**cron") {
		t.Errorf("markdown was not rendered:\n%s", body)
	}
}

func TestPullRequestView_BodyShowsMergeVerdictAndViewerReview(t *testing.T) {
	tests := []struct {
		name  string
		edit  func(*domain.PullRequest)
		wants []string
	}{
		{name: "ready and approved", edit: func(pr *domain.PullRequest) {
			pr.MergeState, pr.ViewerReview = domain.PRMergeStateClean, domain.PRReviewStateApproved
		}, wants: []string{"Ready to merge", "you", "approved"}},
		{name: "blocked and waiting on the viewer", edit: func(pr *domain.PullRequest) {
			pr.MergeState, pr.ReviewDecision, pr.ViewerReviewRequested = domain.PRMergeStateBlocked, domain.PRReviewDecisionReviewRequired, true
		}, wants: []string{"Blocked · review required", "review requested"}},
		{name: "staged review", edit: func(pr *domain.PullRequest) {
			pr.MergeState, pr.ViewerReviewPending = domain.PRMergeStateBlocked, true
		}, wants: []string{"review not submitted"}},
		{name: "changes requested", edit: func(pr *domain.PullRequest) {
			pr.ViewerReview = domain.PRReviewStateChangesRequested
		}, wants: []string{"Merge state not computed yet", "changes requested"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sess := ordinarySession(t)
			m := newTestModel(t)
			m.SetSize(80, 30)
			pr := samplePR()
			tt.edit(&pr)
			m = withPRStatus(t, selectSession(t, m, sess), sess, pr, nil)

			body := ansi.Strip(m.views[ixPullRequest].Body())

			for _, want := range tt.wants {
				if !strings.Contains(body, want) {
					t.Errorf("body is missing %q:\n%s", want, body)
				}
			}
		})
	}
}

func TestPullRequestView_MergedPRShowsNoStatusLines(t *testing.T) {
	sess := ordinarySession(t)
	m := newTestModel(t)
	m.SetSize(80, 30)
	pr := samplePR()
	pr.State, pr.ViewerReview = domain.PRStateMerged, domain.PRReviewStateApproved
	m = withPRStatus(t, selectSession(t, m, sess), sess, pr, nil)

	if body := ansi.Strip(m.views[ixPullRequest].Body()); strings.Contains(body, "Merge state") || strings.Contains(body, "approved") {
		t.Fatalf("merged PR body should carry no merge or review status:\n%s", body)
	}
}

func TestPullRequestView_EmptyDescriptionShowsAPlaceholder(t *testing.T) {
	sess := ordinarySession(t)
	m := newTestModel(t)
	m.SetSize(80, 24)
	pr := samplePR()
	pr.Body = ""
	m = withPRStatus(t, selectSession(t, m, sess), sess, pr, nil)

	if body := ansi.Strip(m.views[ixPullRequest].Body()); !strings.Contains(body, "No description") {
		t.Fatalf("body = %q, want the placeholder", body)
	}
}

func TestPullRequestView_BodyIsCutToTheTabHeight(t *testing.T) {
	sess := ordinarySession(t)
	m := newTestModel(t)
	m.SetSize(60, 12)
	pr := samplePR()
	pr.Body = strings.Repeat("line of prose\n\n", 40)
	m = withPRStatus(t, selectSession(t, m, sess), sess, pr, nil)

	body := m.views[ixPullRequest].Body()
	if lines := strings.Count(body, "\n") + 1; lines > 12-tabStripHeight {
		t.Fatalf("body has %d lines, want at most the tab height", lines)
	}
	if !strings.Contains(ansi.Strip(body), "⏎ expand") {
		t.Fatalf("a cut body must still end with the expand hint:\n%s", body)
	}
}

func TestPullRequestView_ResizeRerendersTheDescription(t *testing.T) {
	sess := ordinarySession(t)
	m := newTestModel(t)
	m.SetSize(120, 24)
	pr := samplePR()
	pr.Body = strings.Repeat("word ", 40)
	m = withPRStatus(t, selectSession(t, m, sess), sess, pr, nil)
	wide := strings.Count(m.views[ixPullRequest].Body(), "\n")

	m.SetSize(40, 24)

	if narrow := strings.Count(m.views[ixPullRequest].Body(), "\n"); narrow <= wide {
		t.Fatalf("narrow body has %d lines, wide had %d; want the description re-wrapped", narrow, wide)
	}
}

var errGHDown = errors.New("gh down")
