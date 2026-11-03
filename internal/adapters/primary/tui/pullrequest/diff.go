package pullrequest

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
)

// splitMinWidth is the narrowest page that still fits two readable columns.
const splitMinWidth = 100

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

type fileDiff struct {
	Path  string
	Hunks []hunkDiff
}

type hunkDiff struct {
	Header string
	Lines  []diffLine
}

// diffLine is one hunk line; OldNo/NewNo are zero on the side it does not exist.
type diffLine struct {
	Kind  diffLineKind
	Text  string
	OldNo int
	NewNo int
}

var hunkHeaderRE = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

// parseUnifiedDiff turns `gh pr diff` output into files, hunks and numbered
// lines. Header lines between the file line and the first hunk (index, mode,
// ---/+++) are dropped: the file rule shows the path and the hunks show the rest.
func parseUnifiedDiff(diff string) []fileDiff {
	var files []fileDiff
	var oldNo, newNo int
	for _, raw := range strings.Split(strings.TrimRight(diff, "\n"), "\n") {
		if strings.HasPrefix(raw, "diff --git ") {
			files = append(files, fileDiff{Path: pathFromGitHeader(raw)})
			continue
		}
		if len(files) == 0 {
			continue
		}
		f := &files[len(files)-1]
		if m := hunkHeaderRE.FindStringSubmatch(raw); m != nil {
			oldNo, _ = strconv.Atoi(m[1])
			newNo, _ = strconv.Atoi(m[2])
			f.Hunks = append(f.Hunks, hunkDiff{Header: raw})
			continue
		}
		if len(f.Hunks) == 0 {
			continue
		}
		h := &f.Hunks[len(f.Hunks)-1]
		line := diffLine{Kind: diffContext}
		switch {
		case strings.HasPrefix(raw, "+"):
			line.Kind, line.Text, line.NewNo = diffAdd, raw[1:], newNo
			newNo++
		case strings.HasPrefix(raw, "-"):
			line.Kind, line.Text, line.OldNo = diffRemove, raw[1:], oldNo
			oldNo++
		case strings.HasPrefix(raw, `\`):
			// "\ No newline at end of file": a note, not a numbered line.
			line.Text = raw
		default:
			line.Text, line.OldNo, line.NewNo = strings.TrimPrefix(raw, " "), oldNo, newNo
			oldNo++
			newNo++
		}
		line.Text = strings.ReplaceAll(line.Text, "\t", "    ")
		h.Lines = append(h.Lines, line)
	}
	return files
}

// pathFromGitHeader reads the new path out of "diff --git a/old b/new".
func pathFromGitHeader(line string) string {
	rest := strings.TrimPrefix(line, "diff --git ")
	if i := strings.LastIndex(rest, " b/"); i >= 0 {
		return rest[i+3:]
	}
	return rest
}

// splitRow is one line of the split view; a nil side is blank.
type splitRow struct {
	left, right *diffLine
}

// splitRows pairs each run of removed lines with the run of added lines that
// follows it, the way GitHub's split view does; context sits on both sides.
func splitRows(lines []diffLine) []splitRow {
	rows := make([]splitRow, 0, len(lines))
	for i := 0; i < len(lines); {
		if lines[i].Kind != diffRemove && lines[i].Kind != diffAdd {
			rows = append(rows, splitRow{left: &lines[i], right: &lines[i]})
			i++
			continue
		}
		var removed, added []*diffLine
		for i < len(lines) && lines[i].Kind == diffRemove {
			removed = append(removed, &lines[i])
			i++
		}
		for i < len(lines) && lines[i].Kind == diffAdd {
			added = append(added, &lines[i])
			i++
		}
		for k := range max(len(removed), len(added)) {
			var row splitRow
			if k < len(removed) {
				row.left = removed[k]
			}
			if k < len(added) {
				row.right = added[k]
			}
			rows = append(rows, row)
		}
	}
	return rows
}

// renderDiff draws the diff at width, split (old left, new right) or unified.
// Every row is cut to the width so tinted rows stay aligned; the viewport does
// not need to scroll sideways.
func renderDiff(s *styles.Styles, diff string, width int, split bool) string {
	files := parseUnifiedDiff(diff)
	width = max(width, 20)
	gutter := gutterWidth(files)
	var out []string
	for i, f := range files {
		if i > 0 {
			out = append(out, "")
		}
		out = append(out, fileRule(s, f.Path, width))
		for _, h := range f.Hunks {
			out = append(out, s.PullRequest.Diff.Hunk.Width(width).Render(ansi.Truncate(h.Header, width, "…")))
			if split {
				out = append(out, renderSplitHunk(s, h, width, gutter)...)
			} else {
				out = append(out, renderUnifiedHunk(s, h, width, gutter)...)
			}
		}
	}
	return strings.Join(out, "\n")
}

func renderUnifiedHunk(s *styles.Styles, h hunkDiff, width, gutter int) []string {
	ds := s.PullRequest.Diff
	textW := max(width-2*gutter-2, 1)
	rows := make([]string, 0, len(h.Lines))
	for _, l := range h.Lines {
		nums := ds.LineNo.Render(lineNo(l.OldNo, gutter) + " " + lineNo(l.NewNo, gutter))
		rows = append(rows, nums+" "+diffLineStyle(ds, l.Kind).Width(textW).Render(ansi.Truncate(marker(l.Kind)+l.Text, textW, "…")))
	}
	return rows
}

func renderSplitHunk(s *styles.Styles, h hunkDiff, width, gutter int) []string {
	ds := s.PullRequest.Diff
	colW := (width - 1) / 2
	textW := max(colW-gutter-1, 1)
	sep := ds.FileRule.Render("│")
	side := func(l *diffLine, no func(diffLine) int) string {
		if l == nil {
			return ds.LineNo.Width(colW).Render("")
		}
		return ds.LineNo.Render(lineNo(no(*l), gutter)) + " " + diffLineStyle(ds, l.Kind).Width(textW).Render(ansi.Truncate(l.Text, textW, "…"))
	}
	rows := splitRows(h.Lines)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, side(r.left, func(l diffLine) int { return l.OldNo })+sep+side(r.right, func(l diffLine) int { return l.NewNo }))
	}
	return out
}

func fileRule(s *styles.Styles, path string, width int) string {
	ds := s.PullRequest.Diff
	name := ds.FileName.Render(path)
	return ds.FileRule.Render("── ") + name + " " + ds.FileRule.Render(strings.Repeat("─", max(width-lipgloss.Width(name)-4, 0)))
}

// gutterWidth is the width of the widest line number, at least 3.
func gutterWidth(files []fileDiff) int {
	highest := 0
	for _, f := range files {
		for _, h := range f.Hunks {
			for _, l := range h.Lines {
				highest = max(highest, l.OldNo, l.NewNo)
			}
		}
	}
	return max(len(strconv.Itoa(highest)), 3)
}

func lineNo(n, width int) string {
	if n == 0 {
		return strings.Repeat(" ", width)
	}
	return fmt.Sprintf("%*d", width, n)
}

func marker(kind diffLineKind) string {
	switch kind {
	case diffAdd:
		return "+"
	case diffRemove:
		return "-"
	}
	return " "
}

func diffLineStyle(s styles.DiffStyles, kind diffLineKind) lipgloss.Style {
	switch kind {
	case diffAdd:
		return s.Add
	case diffRemove:
		return s.Remove
	}
	return s.Context
}
