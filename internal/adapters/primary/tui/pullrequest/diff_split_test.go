package pullrequest

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
)

const twoFileDiff = `diff --git a/src/Scheduler.cs b/src/Scheduler.cs
index 1111..2222 100644
--- a/src/Scheduler.cs
+++ b/src/Scheduler.cs
@@ -10,4 +10,5 @@ public class Scheduler
     var next = cron.Next();
-    Patch(next);
-    Log(next);
+    if (next is null) return;
+    Patch(next);
+    Log(next, true);
     return;
diff --git a/src/Worker.cs b/src/Worker.cs
new file mode 100644
--- /dev/null
+++ b/src/Worker.cs
@@ -0,0 +1,2 @@
+class Worker {}
+// end
`

func TestParseUnifiedDiff_NumbersLinesPerSide(t *testing.T) {
	files := parseUnifiedDiff(twoFileDiff)
	if len(files) != 2 {
		t.Fatalf("files = %d, want 2", len(files))
	}
	f := files[0]
	if f.Path != "src/Scheduler.cs" || len(f.Hunks) != 1 {
		t.Fatalf("file[0] = %+v", f)
	}
	h := f.Hunks[0]
	if h.Header != "@@ -10,4 +10,5 @@ public class Scheduler" || len(h.Lines) != 7 {
		t.Fatalf("hunk = %+v", h)
	}
	want := []struct {
		kind       diffLineKind
		old, new   int
		textPrefix string
	}{
		{diffContext, 10, 10, "    var next"},
		{diffRemove, 11, 0, "    Patch(next);"},
		{diffRemove, 12, 0, "    Log(next);"},
		{diffAdd, 0, 11, "    if (next"},
		{diffAdd, 0, 12, "    Patch(next);"},
		{diffAdd, 0, 13, "    Log(next, true);"},
		{diffContext, 13, 14, "    return;"},
	}
	for i, w := range want {
		got := h.Lines[i]
		if got.Kind != w.kind || got.OldNo != w.old || got.NewNo != w.new || !strings.HasPrefix(got.Text, w.textPrefix) {
			t.Errorf("line %d = %+v, want %+v", i, got, w)
		}
	}
	if files[1].Path != "src/Worker.cs" || files[1].Hunks[0].Lines[0].NewNo != 1 || files[1].Hunks[0].Lines[1].NewNo != 2 {
		t.Fatalf("file[1] = %+v", files[1])
	}
}

func TestSplitRows_PairsRemovedAndAddedRuns(t *testing.T) {
	h := parseUnifiedDiff(twoFileDiff)[0].Hunks[0]

	rows := splitRows(h.Lines)

	// context | removed×2 paired with added×3 | context
	if len(rows) != 5 {
		t.Fatalf("rows = %d, want 5:\n%+v", len(rows), rows)
	}
	if rows[1].left == nil || rows[1].right == nil || rows[1].left.OldNo != 11 || rows[1].right.NewNo != 11 {
		t.Fatalf("row 1 = %+v", rows[1])
	}
	if rows[2].left == nil || rows[2].right == nil || rows[2].left.OldNo != 12 || rows[2].right.NewNo != 12 {
		t.Fatalf("row 2 = %+v", rows[2])
	}
	if rows[3].left != nil || rows[3].right == nil || rows[3].right.NewNo != 13 {
		t.Fatalf("row 3 = %+v, want an unpaired addition", rows[3])
	}
	if rows[4].left == nil || rows[4].right == nil || rows[4].left.Kind != diffContext {
		t.Fatalf("row 4 = %+v, want context on both sides", rows[4])
	}
}

func TestRenderDiff_Split_ShowsBothSidesWithRulesBetweenFiles(t *testing.T) {
	s := styles.New()
	out := ansi.Strip(renderDiff(s, twoFileDiff, 140, true, ""))
	lines := strings.Split(out, "\n")

	for _, want := range []string{"src/Scheduler.cs", "src/Worker.cs", "@@ -10,4 +10,5 @@", "Patch(next);", "if (next is null) return;"} {
		if !strings.Contains(out, want) {
			t.Errorf("split diff is missing %q:\n%s", want, out)
		}
	}
	if rules := strings.Count(out, "─────"); rules < 2 {
		t.Errorf("want a rule per file, got %d:\n%s", rules, out)
	}
	paired := false
	for _, l := range lines {
		if strings.Contains(l, "Patch(next);") && strings.Contains(l, "if (next is null) return;") {
			paired = true
		}
	}
	if !paired {
		t.Errorf("removed and added lines are not side by side:\n%s", out)
	}
	for _, l := range lines {
		if w := ansi.StringWidth(l); w > 140 {
			t.Errorf("row wider than the page (%d): %q", w, l)
		}
	}
}

func TestRenderDiff_Unified_ShowsLineNumbersAndRules(t *testing.T) {
	s := styles.New()
	out := ansi.Strip(renderDiff(s, twoFileDiff, 80, false, ""))

	for _, want := range []string{"src/Scheduler.cs", "11", "+    if (next is null) return;", "-    Patch(next);"} {
		if !strings.Contains(out, want) {
			t.Errorf("unified diff is missing %q:\n%s", want, out)
		}
	}
	if rules := strings.Count(out, "─────"); rules < 2 {
		t.Errorf("want a rule per file, got %d:\n%s", rules, out)
	}
}

func TestModel_DiffFallsBackToUnifiedWhenNarrow_AndSToggles(t *testing.T) {
	m := New(styles.New())
	m.SetSize(90, 30)
	m.SetPullRequest(samplePR())
	d := sampleDetails()
	d.Diff = twoFileDiff
	m.SetDetails(d)
	m, _ = press(t, m, letter(']'))
	if m.diffIsSplit() {
		t.Fatal("90 columns should fall back to unified")
	}

	m.SetSize(160, 30)
	if !m.diffIsSplit() {
		t.Fatal("160 columns should be split")
	}
	m, _ = press(t, m, letter('s'))
	if m.diffIsSplit() {
		t.Fatal("s did not switch to unified")
	}
	if v := view(m); !strings.Contains(v, "-    Patch(next);") {
		t.Fatalf("unified view after s:\n%s", v)
	}
	m, _ = press(t, m, letter('s'))
	if !m.diffIsSplit() {
		t.Fatal("s did not switch back to split")
	}
}

func TestModel_Tab_CyclesSections(t *testing.T) {
	m := loadedModel(t)
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	if m.Section() != sectionDiff {
		t.Fatalf("after tab: %v", m.Section())
	}
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	if m.Section() != sectionOverview {
		t.Fatalf("tab did not wrap: %v", m.Section())
	}
}

func TestModel_Comments_SeparateEveryEntry(t *testing.T) {
	m := loadedModel(t)
	m, _ = press(t, m, letter('['))

	v := view(m)
	// one open thread, one collapsed thread, two human comments → at least 4 rules
	if rules := strings.Count(v, "─────"); rules < 4 {
		t.Fatalf("want a separator per entry, got %d:\n%s", rules, v)
	}
}
