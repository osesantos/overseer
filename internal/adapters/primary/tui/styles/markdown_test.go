package styles_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
)

func TestMarkdown_RendersFormattingAsStyledText(t *testing.T) {
	md := styles.New().NewMarkdown(60)

	out := md.Render("Found it in **store.go** — see `persist()`.")
	plain := ansi.Strip(out)

	// The markers themselves must be gone; the words must survive.
	if strings.Contains(plain, "**") {
		t.Fatalf("bold markers survived rendering: %q", plain)
	}
	for _, want := range []string{"store.go", "persist()"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("rendered output lost %q: %q", want, plain)
		}
	}
	if out == plain {
		t.Fatal("output carries no ANSI styling — markdown was not rendered")
	}
}

func TestMarkdown_RendersCodeBlocks(t *testing.T) {
	md := styles.New().NewMarkdown(60)

	out := ansi.Strip(md.Render("Here:\n\n```go\nfunc main() {}\n```\n"))

	if strings.Contains(out, "```") {
		t.Fatalf("code fence markers survived rendering: %q", out)
	}
	if !strings.Contains(out, "func main()") {
		t.Fatalf("code block content lost: %q", out)
	}
}

func TestMarkdown_WrapsAtTheGivenWidth(t *testing.T) {
	const width = 30
	md := styles.New().NewMarkdown(width)

	long := strings.Repeat("word ", 60)
	for _, line := range strings.Split(ansi.Strip(md.Render(long)), "\n") {
		if w := ansi.StringWidth(line); w > width {
			t.Fatalf("line width %d exceeds wrap width %d: %q", w, width, line)
		}
	}
}

func TestMarkdown_NeverSwallowsContent(t *testing.T) {
	// A message must survive even when rendering cannot help, otherwise a panel
	// would silently blank out an agent's post.
	tests := []struct {
		name  string
		width int
		input string
	}{
		{name: "zero width disables rendering", width: 0, input: "hello"},
		{name: "negative width disables rendering", width: -5, input: "hello"},
		{name: "plain text", width: 40, input: "just a sentence"},
		{name: "unclosed fence", width: 40, input: "```go\nfunc main() {"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := styles.New().NewMarkdown(tt.width).Render(tt.input)
			if strings.TrimSpace(ansi.Strip(out)) == "" {
				t.Fatalf("Render(%q) returned nothing", tt.input)
			}
		})
	}
}

func TestMarkdown_EmptyInputStaysEmpty(t *testing.T) {
	md := styles.New().NewMarkdown(40)

	for _, input := range []string{"", "   ", "\n\n"} {
		if got := md.Render(input); got != input {
			t.Fatalf("Render(%q) = %q, want it returned unchanged", input, got)
		}
	}
}

func TestMarkdown_WidthReportsWhatItWasBuiltFor(t *testing.T) {
	// Callers compare this against the current viewport width to decide whether a
	// resize invalidated their cached bodies.
	s := styles.New()

	if got := s.NewMarkdown(80).Width(); got != 80 {
		t.Fatalf("Width() = %d, want 80", got)
	}
	if got := s.NewMarkdown(0).Width(); got != 0 {
		t.Fatalf("Width() = %d, want 0 for a disabled renderer", got)
	}
}

func TestMarkdown_TrimsGlamourPaddingSoMessagesSitFlush(t *testing.T) {
	md := styles.New().NewMarkdown(60)

	out := md.Render("one line")

	if strings.HasPrefix(out, "\n") || strings.HasSuffix(out, "\n") {
		t.Fatalf("output keeps glamour's blank-line padding: %q", out)
	}
	if plain := ansi.Strip(out); strings.HasPrefix(plain, "  ") {
		t.Fatalf("output keeps glamour's left margin: %q", plain)
	}
}
