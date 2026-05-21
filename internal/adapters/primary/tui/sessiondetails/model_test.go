package sessiondetails

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/testutil"
)

func TestModel_SessionSelectionClearedMsg_ClearsSelectedSession(t *testing.T) {
	m := New(styles.New())
	m.SetSize(80, 20)
	sess := testutil.MakeSession("alpha", uuid.New())
	updated, _ := m.Update(shared.SessionSelectedMsg{Session: sess})
	m = updated.(Model)
	if m.session == nil {
		t.Fatalf("setup: m.session = nil after SessionSelectedMsg, want non-nil")
	}

	updated, cmd := m.Update(shared.SessionSelectionClearedMsg{})
	m = updated.(Model)

	if cmd != nil {
		t.Errorf("Update(SessionSelectionClearedMsg) cmd = %#v, want nil", cmd)
	}
	if m.session != nil {
		t.Errorf("m.session = %+v, want nil after SessionSelectionClearedMsg", *m.session)
	}
	content := m.View().Content
	if !strings.Contains(content, "Select a session") {
		t.Errorf("View().Content missing 'Select a session' hint: %q", content)
	}
}
