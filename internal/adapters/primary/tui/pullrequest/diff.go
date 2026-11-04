package pullrequest

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/osesantos/skipper/internal/adapters/primary/tui/styles"
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

// diffRow is one rendered line of the diff and, unpainted, the cells where
// the search query hits it. Painting is a separate step so the current hit
// can change colour without rendering the diff again.
type diffRow struct {
	text   string
	ranges []lipgloss.Range
}

// shifted moves the row's hit cells right by the width of a prefix.
func (r diffRow) shifted(by int) []lipgloss.Range {
	out := make([]lipgloss.Range, len(r.ranges))
	for i, rg := range r.ranges {
		out[i] = lipgloss.Range{Start: rg.Start + by, End: rg.End + by}
	}
	return out
}

func (r diffRow) paint(style lipgloss.Style) string {
	if len(r.ranges) == 0 {
		return r.text
	}
	ranges := make([]lipgloss.Range, len(r.ranges))
	for i, rg := range r.ranges {
		ranges[i] = lipgloss.NewRange(rg.Start, rg.End, style)
	}
	return lipgloss.StyleRanges(r.text, ranges...)
}

// diffRows draws the diff at width, split (old left, new right) or unified.
// Every row is cut to the width so tinted rows stay aligned; the viewport does
// not need to scroll sideways. A query marks its hits on each row.
func diffRows(s *styles.Styles, diff string, width int, split bool, query string) []diffRow {
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
	return rows
}

// paintDiff turns rows into viewport lines with every hit in the match style
// and the current hit (an index into the returned hits, -1 for none) in its
// own; the hits are the row indexes that have one.
func paintDiff(ds styles.DiffStyles, rows []diffRow, current int) ([]string, []int) {
	lines := make([]string, len(rows))
	var hits []int
	for i, r := range rows {
		style := ds.Match
		if len(r.ranges) > 0 {
			if len(hits) == current {
				style = ds.MatchCurrent
			}
			hits = append(hits, i)
		}
		lines[i] = r.paint(style)
	}
	return lines, hits
}

func renderUnifiedHunk(s *styles.Styles, h hunkDiff, width, gutter int, query string) []diffRow {
	ds := s.PullRequest.Diff
	textW := max(width-2*gutter-2, 1)
	rows := make([]diffRow, 0, len(h.Lines))
	for _, l := range h.Lines {
		nums := ds.LineNo.Render(lineNo(l.OldNo, gutter)+" "+lineNo(l.NewNo, gutter)) + " "
		c := cell(ds, diffLineStyle(ds, l.Kind), marker(l.Kind)+l.Text, textW, query)
		rows = append(rows, diffRow{text: nums + c.text, ranges: c.shifted(lipgloss.Width(nums))})
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
		num := ds.LineNo.Render(lineNo(no(*l), gutter)) + " "
		c := cell(ds, diffLineStyle(ds, l.Kind), l.Text, textW, query)
		return diffRow{text: num + c.text, ranges: c.shifted(lipgloss.Width(num))}
	}
	rows := splitRows(h.Lines)
	out := make([]diffRow, 0, len(rows))
	for _, r := range rows {
		left := side(r.left, func(l diffLine) int { return l.OldNo })
		right := side(r.right, func(l diffLine) int { return l.NewNo })
		text := left.text + sep + right.text
		out = append(out, diffRow{text: text, ranges: append(left.ranges, right.shifted(lipgloss.Width(left.text+sep))...)})
	}
	return out
}

// cell renders text cut and padded to width in style and marks every hit of query.
func cell(ds styles.DiffStyles, style lipgloss.Style, text string, width int, query string) diffRow {
	plain := ansi.Truncate(text, width, "…")
	return diffRow{text: style.Width(width).Render(plain), ranges: matchRanges(plain, query)}
}

// matchRanges locates query in plain as cell ranges for lipgloss.StyleRanges.
// Like vim's smartcase, an all-lower-case query ignores case.
func matchRanges(plain, query string) []lipgloss.Range {
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
		out = append(out, lipgloss.Range{Start: ansi.StringWidth(hay[:start]), End: ansi.StringWidth(hay[:end])})
		from = end
	}
}

func fileRule(s *styles.Styles, path string, width int, query string) diffRow {
	ds := s.PullRequest.Diff
	lead := ds.FileRule.Render("── ")
	name := ds.FileName.Render(path)
	rule := lead + name + " " + ds.FileRule.Render(strings.Repeat("─", max(width-lipgloss.Width(name)-4, 0)))
	return diffRow{text: rule, ranges: diffRow{ranges: matchRanges(path, query)}.shifted(lipgloss.Width(lead))}
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
