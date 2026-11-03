package pullrequest

import (
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
	out := ansi.Strip(renderDiff(s, "diff --git a/x b/x\n@@ -1 +1 @@\n-\told\n+\tnew\n", 80, false))
	if got := strings.Count(out, "\n"); got != 3 {
		t.Fatalf("rendered %d newlines, want 3:\n%s", got, out)
	}
	if strings.Contains(out, "\t") || !strings.Contains(out, "+    new") {
		t.Fatalf("tabs were not expanded:\n%q", out)
	}
}
