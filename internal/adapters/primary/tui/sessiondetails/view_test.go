package sessiondetails

import (
	"strings"
	"testing"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/testutil"
)

func TestRenderCommentsLine(t *testing.T) {
	st := styles.New()
	if got := renderCommentsLine(&st.SessionDetails, st.Glyphs, domain.PRComments{}); got != "" {
		t.Errorf("renderCommentsLine(zero) = %q, want empty", got)
	}
	got := testutil.StripANSI(renderCommentsLine(&st.SessionDetails, st.Glyphs, domain.PRComments{Resolved: 5, Unresolved: 2}))
	if !strings.Contains(got, "2 awaiting you") || !strings.Contains(got, "5 resolved") {
		t.Errorf("renderCommentsLine() = %q, want open and resolved counts", got)
	}
}
