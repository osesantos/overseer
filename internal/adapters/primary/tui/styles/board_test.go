package styles_test

import (
	"testing"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
)

// themeNames lists every registered theme so board styling is checked against
// all of them, not just the default.
var themeNames = []string{
	"dark", "dracula", "github-dark", "tokyo-night", "monokai", "one-dark",
	"solarized-dark", "nord", "catppuccin-mocha", "porcelain", "deep-sea", "sunset",
}

func TestBoardStyles_AuthorColors_CoverEveryAgentInEveryTheme(t *testing.T) {
	for _, name := range themeNames {
		t.Run(name, func(t *testing.T) {
			s := styles.NewWithTheme(name, true)

			if len(s.Board.AuthorColors) < domain.SwarmMaxAgents {
				t.Fatalf("Board.AuthorColors has %d entries, want at least %d so the largest swarm still gets distinct colours",
					len(s.Board.AuthorColors), domain.SwarmMaxAgents)
			}
			for i, c := range s.Board.AuthorColors {
				if c == nil {
					t.Fatalf("Board.AuthorColors[%d] is nil", i)
				}
			}
		})
	}
}

func TestBoardStyles_AuthorColors_AreMostlyDistinct(t *testing.T) {
	// Exact uniqueness is not guaranteed across every palette, but a swarm whose
	// authors are visually indistinguishable defeats the point of colouring them,
	// so require most of the slots to differ.
	for _, name := range themeNames {
		t.Run(name, func(t *testing.T) {
			s := styles.NewWithTheme(name, true)

			seen := make(map[string]struct{}, len(s.Board.AuthorColors))
			for _, c := range s.Board.AuthorColors[:domain.SwarmMaxAgents] {
				r, g, b, a := c.RGBA()
				seen[string(rune(r))+string(rune(g))+string(rune(b))+string(rune(a))] = struct{}{}
			}

			const minDistinct = domain.SwarmMaxAgents - 2
			if len(seen) < minDistinct {
				t.Fatalf("Board.AuthorColors has only %d distinct colours out of %d, want at least %d",
					len(seen), domain.SwarmMaxAgents, minDistinct)
			}
		})
	}
}

func TestBoardStyles_TextStylesAreConfigured(t *testing.T) {
	s := styles.NewWithTheme("dark", true)

	// A zero lipgloss.Style renders unstyled text, which would make the board
	// indistinguishable from raw output — assert each slot was populated.
	if s.Board.Text.GetForeground() == nil {
		t.Error("Board.Text has no foreground")
	}
	if s.Board.Timestamp.GetForeground() == nil {
		t.Error("Board.Timestamp has no foreground")
	}
	if s.Board.AuthorHuman.GetForeground() == nil {
		t.Error("Board.AuthorHuman has no foreground")
	}
	if s.Board.SystemText.GetForeground() == nil {
		t.Error("Board.SystemText has no foreground")
	}
	if !s.Board.Author.GetBold() {
		t.Error("Board.Author should be bold so authors stand out from message bodies")
	}
}

func TestBoardStyles_AuthorColorFor_WrapsAndIsStableAcrossCalls(t *testing.T) {
	s := styles.NewWithTheme("dark", true)

	first := s.Board.AuthorColorFor(1)
	if first == nil {
		t.Fatal("AuthorColorFor(1) = nil")
	}
	if again := s.Board.AuthorColorFor(1); again != first {
		t.Fatal("AuthorColorFor is not stable across calls")
	}

	// Indices beyond the palette wrap rather than panicking, so an out-of-range
	// author name can never crash a render.
	if s.Board.AuthorColorFor(domain.SwarmMaxAgents+1) == nil {
		t.Fatal("AuthorColorFor wrapped index = nil")
	}
	if s.Board.AuthorColorFor(0) == nil {
		t.Fatal("AuthorColorFor(0) = nil")
	}
	if s.Board.AuthorColorFor(-5) == nil {
		t.Fatal("AuthorColorFor(-5) = nil")
	}
}

func TestGlyphs_SwarmStatusIsDistinctFromUnknown(t *testing.T) {
	// A swarm rendered as Unknown shows a question mark, which reads as a fault
	// rather than "not tracked per-session".
	for _, disableEmoji := range []bool{false, true} {
		g := styles.NewGlyphs(disableEmoji)

		swarm := g.AgentStatus(domain.AgentStatusSwarm)
		if swarm == "" {
			t.Fatalf("disableEmoji=%v: swarm glyph is empty", disableEmoji)
		}
		if swarm == g.AgentStatus(domain.AgentStatusUnknown) {
			t.Fatalf("disableEmoji=%v: swarm glyph %q is the same as unknown", disableEmoji, swarm)
		}
		for _, kind := range []domain.AgentStatusKind{
			domain.AgentStatusRunning,
			domain.AgentStatusWaiting,
			domain.AgentStatusIdle,
			domain.AgentStatusDead,
		} {
			if swarm == g.AgentStatus(kind) {
				t.Fatalf("disableEmoji=%v: swarm glyph %q collides with %q", disableEmoji, swarm, kind)
			}
		}
	}
}
