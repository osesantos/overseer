package pullrequest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
)

func TestDiffLineKind(t *testing.T) {
	tests := []struct {
		line string
		want diffLineKind
	}{
		{"diff --git a/x b/x", diffFile},
		{"index 1111..2222 100644", diffFile},
		{"--- a/x", diffFile},
		{"+++ b/x", diffFile},
		{"--- /dev/null", diffFile},
		{"+++ /dev/null", diffFile},
		{"new file mode 100644", diffFile},
		{"deleted file mode 100644", diffFile},
		{"Binary files a/x and b/x differ", diffFile},
		{"@@ -1,2 +1,3 @@ func x", diffHunk},
		{"+added", diffAdd},
		{"-removed", diffRemove},
		{"--- not a header when inside a hunk", diffRemove},
		{"+++ not a header either", diffAdd},
		{" context", diffContext},
		{"", diffContext},
	}
	for _, tt := range tests {
		if got := classifyDiffLine(tt.line); got != tt.want {
			t.Errorf("classifyDiffLine(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestRenderDiff_KeepsEveryLineAndExpandsTabs(t *testing.T) {
	s := styles.New()
	rendered, _ := renderDiff(s, "diff --git a/x b/x\n@@ -1 +1 @@\n-\told\n+\tnew\n", 80, false, "")
	out := ansi.Strip(rendered)
	if got := strings.Count(out, "\n"); got != 3 {
		t.Fatalf("rendered %d newlines, want 3:\n%s", got, out)
	}
	if strings.Contains(out, "\t") || !strings.Contains(out, "+    new") {
		t.Fatalf("tabs were not expanded:\n%q", out)
	}
}

func TestRenderDiff_Search_PaintsHitsAndReportsTheirRows(t *testing.T) {
	s := styles.New()
	plain, _ := renderDiff(s, twoFileDiff, 80, false, "")
	out, hits := renderDiff(s, twoFileDiff, 80, false, "patch")

	if ansi.Strip(out) != ansi.Strip(plain) {
		t.Fatalf("search changed the visible text:\n%s\n--- want\n%s", ansi.Strip(out), ansi.Strip(plain))
	}
	// Rows: 0 file rule, 1 hunk header, 2 context, 3 -Patch, 4 -Log, 5 +if, 6 +Patch.
	if want := "[3 6]"; fmt.Sprint(hits) != want {
		t.Fatalf("hits = %v, want %s", hits, want)
	}
	lines := strings.Split(out, "\n")
	for _, i := range hits {
		if !strings.Contains(lines[i], s.PullRequest.Diff.Match.Render("Patch")) {
			t.Errorf("row %d is not painted with the match style:\n%q", i, lines[i])
		}
	}
	if strings.Contains(lines[2], s.PullRequest.Diff.Match.Render("Patch")) {
		t.Errorf("row 2 has no hit but is painted:\n%q", lines[2])
	}
}

func TestRenderDiff_Search_UpperCaseQueryIsCaseSensitive(t *testing.T) {
	_, hits := renderDiff(styles.New(), twoFileDiff, 80, false, "PATCH")
	if len(hits) != 0 {
		t.Fatalf("hits = %v, want none for an upper-case query that matches nothing exactly", hits)
	}
}

func TestRenderDiff_Search_IgnoresLineNumbersAndMatchesFileNamesInSplitView(t *testing.T) {
	_, hits := renderDiff(styles.New(), twoFileDiff, 120, true, "Worker")
	// Rows: 0-6 first file (three paired rows), 7 blank, 8 "src/Worker.cs" rule, 9 hunk header, 10 "+class Worker {}".
	if want := "[8 10]"; fmt.Sprint(hits) != want {
		t.Fatalf("hits = %v, want %s", hits, want)
	}
	if _, hits := renderDiff(styles.New(), twoFileDiff, 120, true, "11"); len(hits) != 0 {
		t.Fatalf("a query matching only the line-number gutter reported hits %v", hits)
	}
}
