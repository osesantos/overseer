package pullrequest

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
)

type diffLineKind int

const (
	diffContext diffLineKind = iota
	diffFile
	diffHunk
	diffAdd
	diffRemove
)

// diffFilePrefixes are the per-file header lines git writes between hunks. The
// "---"/"+++" entries carry their path prefix so a removed line that happens to
// start with "--" is still a removal.
var diffFilePrefixes = []string{
	"diff --git ", "index ", "--- a/", "+++ b/", "--- /dev/null", "+++ /dev/null",
	"new file mode", "deleted file mode", "old mode", "new mode",
	"similarity index", "rename from", "rename to", "Binary files",
}

func classifyDiffLine(line string) diffLineKind {
	for _, prefix := range diffFilePrefixes {
		if strings.HasPrefix(line, prefix) {
			return diffFile
		}
	}
	switch {
	case strings.HasPrefix(line, "@@"):
		return diffHunk
	case strings.HasPrefix(line, "+"):
		return diffAdd
	case strings.HasPrefix(line, "-"):
		return diffRemove
	}
	return diffContext
}

func diffLineStyle(s styles.DiffStyles, kind diffLineKind) lipgloss.Style {
	switch kind {
	case diffFile:
		return s.File
	case diffHunk:
		return s.Hunk
	case diffAdd:
		return s.Add
	case diffRemove:
		return s.Remove
	}
	return s.Context
}

// renderDiff styles a unified diff line by line. Tabs become four spaces so
// indentation survives the viewport, which does not expand them.
func renderDiff(s styles.DiffStyles, diff string) string {
	lines := strings.Split(strings.TrimRight(diff, "\n"), "\n")
	for i, line := range lines {
		line = strings.ReplaceAll(line, "\t", "    ")
		lines[i] = diffLineStyle(s, classifyDiffLine(line)).Render(line)
	}
	return strings.Join(lines, "\n")
}
