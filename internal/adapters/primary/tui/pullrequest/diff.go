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

// diffRow is one rendered line of the diff and whether the search query hits it.
type diffRow struct {
	text string
	hit  bool
}

// renderDiff draws the diff at width, split (old left, new right) or unified.
// Every row is cut to the width so tinted rows stay aligned; the viewport does
// not need to scroll sideways. A query paints its hits and the rows that have
// one come back as indexes, so n/N can jump between them.
func renderDiff(s *styles.Styles, diff string, width int, split bool, query string) (string, []int) {
	files := parseUnifiedDiff(diff)
	width = max(width, 20)
	gutter := gutterWidth(files)
	var rows []diffRow
	for i, f := range files {
		if i > 0 {
			rows = append(rows, diffRow{})
		}
		rows = append(rows, fileRule(s, f.Path, width, query))
		for _, h := range f.Hunks {
			rows = append(rows, cell(s.PullRequest.Diff, s.PullRequest.Diff.Hunk, h.Header, width, query))
			if split {
				rows = append(rows, renderSplitHunk(s, h, width, gutter, query)...)
			} else {
				rows = append(rows, renderUnifiedHunk(s, h, width, gutter, query)...)
			}
		}
	}
	lines := make([]string, len(rows))
	var hits []int
	for i, r := range rows {
		lines[i] = r.text
		if r.hit {
			hits = append(hits, i)
		}
	}
	return strings.Join(lines, "\n"), hits
}

func renderUnifiedHunk(s *styles.Styles, h hunkDiff, width, gutter int, query string) []diffRow {
	ds := s.PullRequest.Diff
	textW := max(width-2*gutter-2, 1)
	rows := make([]diffRow, 0, len(h.Lines))
	for _, l := range h.Lines {
		nums := ds.LineNo.Render(lineNo(l.OldNo, gutter) + " " + lineNo(l.NewNo, gutter))
		c := cell(ds, diffLineStyle(ds, l.Kind), marker(l.Kind)+l.Text, textW, query)
		rows = append(rows, diffRow{text: nums + " " + c.text, hit: c.hit})
	}
	return rows
}

func renderSplitHunk(s *styles.Styles, h hunkDiff, width, gutter int, query string) []diffRow {
	ds := s.PullRequest.Diff
	colW := (width - 1) / 2
	textW := max(colW-gutter-1, 1)
	sep := ds.FileRule.Render("│")
	side := func(l *diffLine, no func(diffLine) int) diffRow {
		if l == nil {
			return diffRow{text: ds.LineNo.Width(colW).Render("")}
		}
		c := cell(ds, diffLineStyle(ds, l.Kind), l.Text, textW, query)
		return diffRow{text: ds.LineNo.Render(lineNo(no(*l), gutter)) + " " + c.text, hit: c.hit}
	}
	rows := splitRows(h.Lines)
	out := make([]diffRow, 0, len(rows))
	for _, r := range rows {
		left := side(r.left, func(l diffLine) int { return l.OldNo })
		right := side(r.right, func(l diffLine) int { return l.NewNo })
		out = append(out, diffRow{text: left.text + sep + right.text, hit: left.hit || right.hit})
	}
	return out
}

// cell renders text cut and padded to width in style, with every hit of query
// painted over it in the match style.
func cell(ds styles.DiffStyles, style lipgloss.Style, text string, width int, query string) diffRow {
	plain := ansi.Truncate(text, width, "…")
	ranges := matchRanges(plain, query, ds.Match)
	return diffRow{text: lipgloss.StyleRanges(style.Width(width).Render(plain), ranges...), hit: len(ranges) > 0}
}

// matchRanges locates query in plain as cell ranges for lipgloss.StyleRanges.
// Like vim's smartcase, an all-lower-case query ignores case.
func matchRanges(plain, query string, style lipgloss.Style) []lipgloss.Range {
	if query == "" {
		return nil
	}
	hay := plain
	if strings.ToLower(query) == query {
		// ponytail: ToLower can change a rune's byte length; widths below are
		// read off the folded text, which is right for every ASCII query.
		hay = strings.ToLower(plain)
	}
	var out []lipgloss.Range
	for from := 0; ; {
		i := strings.Index(hay[from:], query)
		if i < 0 {
			return out
		}
		start, end := from+i, from+i+len(query)
		out = append(out, lipgloss.NewRange(ansi.StringWidth(hay[:start]), ansi.StringWidth(hay[:end]), style))
		from = end
	}
}

func fileRule(s *styles.Styles, path string, width int, query string) diffRow {
	ds := s.PullRequest.Diff
	ranges := matchRanges(path, query, ds.Match)
	name := lipgloss.StyleRanges(ds.FileName.Render(path), ranges...)
	rule := ds.FileRule.Render("── ") + name + " " + ds.FileRule.Render(strings.Repeat("─", max(width-lipgloss.Width(name)-4, 0)))
	return diffRow{text: rule, hit: len(ranges) > 0}
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
