package sessiondetails

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/osesantos/skipper/internal/adapters/primary/tui/shared"
	"github.com/osesantos/skipper/internal/adapters/primary/tui/styles"
	"github.com/osesantos/skipper/internal/core/domain"
	"github.com/osesantos/skipper/internal/testutil"
)

func TestModel_SessionSelectionClearedMsg_ClearsSelectedSession(t *testing.T) {
	m := New(styles.New())
	m.SetSize(80, 20)
	sess := testutil.MakeSession("alpha", uuid.New())
	updated, _ := m.Update(shared.SessionSelectedMsg{Session: sess})
	m = updated.(Model)
	if m.session == nil {
		t.Fatalf("setup: m.session = nil after SessionSelectedMsg, want non-nil")
	}

	updated, cmd := m.Update(shared.SessionSelectionClearedMsg{})
	m = updated.(Model)

	if cmd != nil {
		t.Errorf("Update(SessionSelectionClearedMsg) cmd = %#v, want nil", cmd)
	}
	if m.session != nil {
		t.Errorf("m.session = %+v, want nil after SessionSelectionClearedMsg", *m.session)
	}
	content := m.View().Content
	if !strings.Contains(content, "Select a session") {
		t.Errorf("View().Content missing 'Select a session' hint: %q", content)
	}
}

func TestModel_PRSection_ShowsMergeVerdictAndViewerReview(t *testing.T) {
	m := New(styles.New())
	m.SetSize(100, 30)
	sess := testutil.MakeSession("alpha", uuid.New())
	updated, _ := m.Update(shared.SessionSelectedMsg{Session: sess})
	pr, _ := domain.NewPullRequest(7, "fix: thing", "fix/thing", domain.PRStateOpen)
	pr.MergeState, pr.ReviewDecision, pr.ViewerReviewRequested = domain.PRMergeStateBlocked, domain.PRReviewDecisionReviewRequired, true
	updated, _ = updated.(Model).Update(shared.PRStatusUpdatedMsg{SessionID: sess.ID, PR: pr})

	content := testutil.StripANSI(updated.(Model).View().Content)

	for _, want := range []string{"Merge", "Blocked · review required", "Your review", "review requested"} {
		if !strings.Contains(content, want) {
			t.Errorf("content is missing %q:\n%s", want, content)
		}
	}
}

func TestModel_PRSection_NoViewerReviewRowWhenUninvolved(t *testing.T) {
	m := New(styles.New())
	m.SetSize(100, 30)
	sess := testutil.MakeSession("alpha", uuid.New())
	updated, _ := m.Update(shared.SessionSelectedMsg{Session: sess})
	pr, _ := domain.NewPullRequest(7, "fix: thing", "fix/thing", domain.PRStateOpen)
	pr.MergeState = domain.PRMergeStateClean
	updated, _ = updated.(Model).Update(shared.PRStatusUpdatedMsg{SessionID: sess.ID, PR: pr})

	content := testutil.StripANSI(updated.(Model).View().Content)

	if !strings.Contains(content, "Ready to merge") || strings.Contains(content, "Your review") {
		t.Fatalf("want the merge row and no review row:\n%s", content)
	}
}

func TestModel_PRSection_LongMergeVerdictIsCutToOneRow(t *testing.T) {
	m := New(styles.New())
	m.SetSize(50, 20)
	sess := testutil.MakeSession("alpha", uuid.New())
	updated, _ := m.Update(shared.SessionSelectedMsg{Session: sess})
	pr, _ := domain.NewPullRequest(7, "fix: thing", "fix/thing", domain.PRStateOpen)
	pr.MergeState, pr.ReviewDecision = domain.PRMergeStateBlocked, domain.PRReviewDecisionReviewRequired
	pr.Checks = domain.PRChecks{Total: 3, Failing: 2, Pending: 1}
	updated, _ = updated.(Model).Update(shared.PRStatusUpdatedMsg{SessionID: sess.ID, PR: pr})

	content := testutil.StripANSI(updated.(Model).View().Content)

	if strings.Contains(content, "1 check pending") || !strings.Contains(content, "…") {
		t.Fatalf("merge row should be cut with an ellipsis, not wrapped:\n%s", content)
	}
}
