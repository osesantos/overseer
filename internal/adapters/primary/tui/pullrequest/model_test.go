package pullrequest

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
)

func samplePR() domain.PullRequest {
	pr, _ := domain.NewPullRequest(535, "feat: cron schedules", "feature/cron", domain.PRStateOpen)
	pr.Author = "ana"
	pr.BaseBranch = "main"
	pr.Body = "Adds **cron schedules**.\n"
	pr.Stats = domain.PRStats{Additions: 15, Deletions: 9, ChangedFiles: 2}
	pr.UpdatedAt = time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	return pr
}

func sampleDetails() domain.PullRequestDetails {
	at := time.Date(2026, 10, 29, 18, 30, 0, 0, time.UTC)
	return domain.PullRequestDetails{
		PullRequest: samplePR(),
		Files: []domain.PRFile{
			{Path: "src/Scheduler.cs", Additions: 12, Deletions: 8},
			{Path: "src/Worker.cs", Additions: 3, Deletions: 1},
		},
		Diff: strings.Join([]string{
			"diff --git a/src/Scheduler.cs b/src/Scheduler.cs",
			"index 1111..2222 100644",
			"--- a/src/Scheduler.cs",
			"+++ b/src/Scheduler.cs",
			"@@ -10,4 +10,5 @@ public class Scheduler",
			"     var next = cron.Next();",
			"-    Patch(next);",
			"+    if (next is null) return;",
			"+    Patch(next);",
		}, "\n") + "\n" + strings.Repeat(" context line\n", 60),
		Conversation: []domain.PRComment{
			{Author: "dependabot", AuthorIsBot: true, Body: "## Unit Tests\n|Total|Passed|", CreatedAt: at.Add(-time.Hour)},
			{Author: "bob", Body: "Two small things inline, otherwise **fine**.", CreatedAt: at, ReviewState: "COMMENTED"},
			{Author: "ana", Body: "Rebased on main.", CreatedAt: at.Add(time.Hour)},
		},
		Threads: []domain.PRReviewThread{
			{Resolved: true, Outdated: true, Path: "src/Scheduler.cs", Line: 12, Authors: []string{"ana", "me"},
				Comments: []domain.PRComment{{Author: "ana", Body: "needs a guard", CreatedAt: at}, {Author: "me", Body: "fixed", CreatedAt: at}}},
			{Path: "src/Worker.cs", Line: 40, Authors: []string{"bob"}, DiffHunk: "@@ -38,3 +38,4 @@\n hunk line one\n hunk line two\n+hunk line three",
				Comments: []domain.PRComment{{Author: "bob", Body: "why not reuse the *retry helper*?", CreatedAt: at}, {Author: "dependabot", AuthorIsBot: true, Body: "noise", CreatedAt: at}}},
		},
	}
}

func loadedModel(t *testing.T) Model {
	t.Helper()
	m := New(styles.New())
	m.SetSize(100, 30)
	m.SetPullRequest(samplePR())
	m.SetDetails(sampleDetails())
	return m
}

func press(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(msg)
	return updated.(Model), cmd
}

func letter(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Text: string(r)} }

func view(m Model) string { return ansi.Strip(m.View().Content) }

func TestModel_BeforeDetailsLoad_ShowsTheHeaderAndLoading(t *testing.T) {
	m := New(styles.New())
	m.SetSize(100, 30)
	m.SetPullRequest(samplePR())

	v := view(m)
	for _, want := range []string{"Overview", "Diff", "Comments", "#535", "feat: cron schedules", "ana", "Adds cron schedules.", "Loading"} {
		if !strings.Contains(v, want) {
			t.Errorf("view is missing %q:\n%s", want, v)
		}
	}
}

func TestModel_LoadError_IsShownInPlace(t *testing.T) {
	m := New(styles.New())
	m.SetSize(100, 30)
	m.SetPullRequest(samplePR())

	m.SetError(errors.New("gh exit 1"))

	if v := view(m); !strings.Contains(v, "gh exit 1") || !strings.Contains(v, "#535") {
		t.Fatalf("view = %s", v)
	}
}

func TestModel_Overview_ListsFilesWithCounts(t *testing.T) {
	v := view(loadedModel(t))
	for _, want := range []string{"2 files", "src/Scheduler.cs", "+12", "-8", "src/Worker.cs"} {
		if !strings.Contains(v, want) {
			t.Errorf("overview is missing %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "Loading") {
		t.Errorf("overview still says loading:\n%s", v)
	}
}

func TestModel_BracketKeys_CycleSectionsAndWrap(t *testing.T) {
	m := loadedModel(t)
	if m.Section() != sectionOverview {
		t.Fatalf("initial section = %v", m.Section())
	}
	m, _ = press(t, m, letter(']'))
	if m.Section() != sectionDiff || !strings.Contains(view(m), "Patch(next)") {
		t.Fatalf("after ]: section %v\n%s", m.Section(), view(m))
	}
	m, _ = press(t, m, letter(']'))
	if m.Section() != sectionComments {
		t.Fatalf("after ] ]: section %v", m.Section())
	}
	m, _ = press(t, m, letter(']'))
	if m.Section() != sectionOverview {
		t.Fatalf("] did not wrap: section %v", m.Section())
	}
	m, _ = press(t, m, letter('['))
	if m.Section() != sectionComments {
		t.Fatalf("[ did not wrap backwards: section %v", m.Section())
	}
}

func TestModel_GAndShiftG_ScrollOnlyTheActiveSection(t *testing.T) {
	m := loadedModel(t)
	m, _ = press(t, m, letter(']'))

	m, _ = press(t, m, letter('G'))
	if m.viewports[sectionDiff].YOffset() == 0 {
		t.Fatal("G did not scroll the diff to the bottom")
	}
	if m.viewports[sectionOverview].YOffset() != 0 {
		t.Fatal("G moved the overview too")
	}
	m, _ = press(t, m, letter('g'))
	if m.viewports[sectionDiff].YOffset() != 0 {
		t.Fatal("g did not scroll the diff back to the top")
	}
}

func TestModel_JAndK_ScrollOneLine(t *testing.T) {
	m := loadedModel(t)
	m, _ = press(t, m, letter(']'))

	m, _ = press(t, m, letter('j'))
	m, _ = press(t, m, letter('j'))
	if got := m.viewports[sectionDiff].YOffset(); got != 2 {
		t.Fatalf("after j j: YOffset = %d, want 2", got)
	}
	m, _ = press(t, m, letter('k'))
	if got := m.viewports[sectionDiff].YOffset(); got != 1 {
		t.Fatalf("after k: YOffset = %d, want 1", got)
	}
}

func TestModel_Esc_EmitsPageClose(t *testing.T) {
	_, cmd := press(t, loadedModel(t), tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("esc emitted nothing")
	}
	if _, ok := cmd().(shared.PageCloseMsg); !ok {
		t.Fatalf("esc emitted %T, want PageCloseMsg", cmd())
	}
}

func TestModel_R_EmitsRefetch(t *testing.T) {
	_, cmd := press(t, loadedModel(t), letter('r'))
	if cmd == nil {
		t.Fatal("r emitted nothing")
	}
	if _, ok := cmd().(shared.PullRequestRefetchMsg); !ok {
		t.Fatalf("r emitted %T, want PullRequestRefetchMsg", cmd())
	}
}

func TestModel_O_EmitsOpenInBrowserWithThePR(t *testing.T) {
	_, cmd := press(t, loadedModel(t), letter('o'))
	if cmd == nil {
		t.Fatal("o emitted nothing")
	}
	msg, ok := cmd().(shared.PullRequestOpenInBrowserMsg)
	if !ok || msg.PR.Number != 535 {
		t.Fatalf("o emitted %#v, want PullRequestOpenInBrowserMsg for #535", cmd())
	}
}

func TestModel_Comments_HideBotsAndCollapseResolvedThreads(t *testing.T) {
	m := loadedModel(t)
	m, _ = press(t, m, letter('['))

	v := view(m)
	for _, want := range []string{"src/Worker.cs:40", "why not reuse the retry helper?", "hunk line three", "src/Scheduler.cs:12", "ana, me", "2 comments", "bob", "Two small things inline, otherwise fine.", "Rebased on main.", "1 bot comment hidden", "1 open", "1 resolved"} {
		if !strings.Contains(v, want) {
			t.Errorf("comments are missing %q:\n%s", want, v)
		}
	}
	for _, unwanted := range []string{"Unit Tests", "noise", "needs a guard", "*retry"} {
		if strings.Contains(v, unwanted) {
			t.Errorf("comments show %q:\n%s", unwanted, v)
		}
	}
}

func TestModel_Comments_NothingToShow(t *testing.T) {
	m := New(styles.New())
	m.SetSize(100, 30)
	m.SetPullRequest(samplePR())
	d := sampleDetails()
	d.Conversation, d.Threads = nil, nil
	m.SetDetails(d)
	m, _ = press(t, m, letter('['))

	if v := view(m); !strings.Contains(v, "No comments") {
		t.Fatalf("view = %s", v)
	}
}

func TestModel_UpdatedAt_IsTheLoadedPRs(t *testing.T) {
	m := New(styles.New())
	m.SetPullRequest(samplePR())
	if !m.UpdatedAt().IsZero() {
		t.Fatal("UpdatedAt() is set before any details loaded")
	}
	m.SetDetails(sampleDetails())
	if got := m.UpdatedAt(); !got.Equal(samplePR().UpdatedAt) {
		t.Fatalf("UpdatedAt() = %v, want the loaded PR's", got)
	}
}

func TestModel_SetPullRequest_ResetsToOverviewAndLoading(t *testing.T) {
	m := loadedModel(t)
	m, _ = press(t, m, letter(']'))

	m.SetPullRequest(samplePR())

	if m.Section() != sectionOverview || !strings.Contains(view(m), "Loading") {
		t.Fatalf("section %v, view:\n%s", m.Section(), view(m))
	}
}
