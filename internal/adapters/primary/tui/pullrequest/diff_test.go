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
	out := ansi.Strip(renderDiff(s, "diff --git a/x b/x\n@@ -1 +1 @@\n-\told\n+\tnew\n", 80, false, ""))
	if got := strings.Count(out, "\n"); got != 3 {
		t.Fatalf("rendered %d newlines, want 3:\n%s", got, out)
	}
	if strings.Contains(out, "\t") || !strings.Contains(out, "+    new") {
		t.Fatalf("tabs were not expanded:\n%q", out)
	}
}

// renderDiff is the test shorthand for the rows painted with no current hit.
func renderDiff(s *styles.Styles, diff string, width int, split bool, query string) string {
	lines, _ := paintDiff(s.PullRequest.Diff, diffRows(s, diff, width, split, query), -1)
	return strings.Join(lines, "\n")
}

func TestRenderDiff_Search_PaintsHitsAndReportsTheirRows(t *testing.T) {
	s := styles.New()
	plain := renderDiff(s, twoFileDiff, 80, false, "")
	lines, hits := paintDiff(s.PullRequest.Diff, diffRows(s, twoFileDiff, 80, false, "patch"), -1)
	out := strings.Join(lines, "\n")

	if ansi.Strip(out) != ansi.Strip(plain) {
		t.Fatalf("search changed the visible text:\n%s\n--- want\n%s", ansi.Strip(out), ansi.Strip(plain))
	}
	// Rows: 0 file rule, 1 hunk header, 2 context, 3 -Patch, 4 -Log, 5 +if, 6 +Patch.
	if want := "[3 6]"; fmt.Sprint(hits) != want {
		t.Fatalf("hits = %v, want %s", hits, want)
	}
	for _, i := range hits {
		if !strings.Contains(lines[i], s.PullRequest.Diff.Match.Render("Patch")) {
			t.Errorf("row %d is not painted with the match style:\n%q", i, lines[i])
		}
	}
	if strings.Contains(lines[2], s.PullRequest.Diff.Match.Render("Patch")) {
		t.Errorf("row 2 has no hit but is painted:\n%q", lines[2])
	}
}

func TestPaintDiff_CurrentHit_GetsItsOwnStyle(t *testing.T) {
	s := styles.New()
	rows := diffRows(s, twoFileDiff, 80, false, "patch")

	lines, hits := paintDiff(s.PullRequest.Diff, rows, 1)

	if !strings.Contains(lines[hits[1]], s.PullRequest.Diff.MatchCurrent.Render("Patch")) {
		t.Errorf("current hit row is not painted with MatchCurrent:\n%q", lines[hits[1]])
	}
	if !strings.Contains(lines[hits[0]], s.PullRequest.Diff.Match.Render("Patch")) || strings.Contains(lines[hits[0]], s.PullRequest.Diff.MatchCurrent.Render("Patch")) {
		t.Errorf("other hit row is not painted with Match:\n%q", lines[hits[0]])
	}
}

func TestRenderDiff_Search_UpperCaseQueryIsCaseSensitive(t *testing.T) {
	_, hits := paintDiff(styles.New().PullRequest.Diff, diffRows(styles.New(), twoFileDiff, 80, false, "PATCH"), -1)
	if len(hits) != 0 {
		t.Fatalf("hits = %v, want none for an upper-case query that matches nothing exactly", hits)
	}
}

func TestRenderDiff_Search_IgnoresLineNumbersAndMatchesFileNamesInSplitView(t *testing.T) {
	s := styles.New()
	lines, hits := paintDiff(s.PullRequest.Diff, diffRows(s, twoFileDiff, 120, true, "Worker"), -1)
	if plain := renderDiff(s, twoFileDiff, 120, true, ""); ansi.Strip(strings.Join(lines, "\n")) != ansi.Strip(plain) {
		t.Fatalf("search changed the visible text:\n%s", ansi.Strip(strings.Join(lines, "\n")))
	}
	// Row 10 is "+class Worker {}" in the right-hand column: the hit must land there, not shifted by the left column.
	if !strings.Contains(lines[10], s.PullRequest.Diff.Match.Render("Worker")) {
		t.Fatalf("right column hit is not painted on the word:\n%q", lines[10])
	}
	// Rows: 0-6 first file (three paired rows), 7 blank, 8 "src/Worker.cs" rule, 9 hunk header, 10 "+class Worker {}".
	if want := "[8 10]"; fmt.Sprint(hits) != want {
		t.Fatalf("hits = %v, want %s", hits, want)
	}
	if _, hits := paintDiff(s.PullRequest.Diff, diffRows(s, twoFileDiff, 120, true, "11"), -1); len(hits) != 0 {
		t.Fatalf("a query matching only the line-number gutter reported hits %v", hits)
	}
}
