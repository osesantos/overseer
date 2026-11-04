package sessiondetails

import (
	"strings"
	"testing"

	"github.com/osesantos/skipper/internal/adapters/primary/tui/components"
	"github.com/osesantos/skipper/internal/adapters/primary/tui/styles"
	"github.com/osesantos/skipper/internal/core/domain"
	"github.com/osesantos/skipper/internal/testutil"
)

func TestRenderCommentsLine(t *testing.T) {
	st := styles.New()
	if got := components.PRCommentsLine(st.SessionDetails, st.Glyphs, domain.PRComments{}); got != "" {
		t.Errorf("renderCommentsLine(zero) = %q, want empty", got)
	}
	got := testutil.StripANSI(components.PRCommentsLine(st.SessionDetails, st.Glyphs, domain.PRComments{Resolved: 5, Unresolved: 3, AwaitingViewer: 2}))
	if !strings.Contains(got, "3 open (2 awaiting you)") || !strings.Contains(got, "5 resolved") {
		t.Errorf("renderCommentsLine() = %q, want open and resolved counts", got)
	}
}
