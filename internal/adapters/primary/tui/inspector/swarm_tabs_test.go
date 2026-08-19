package inspector

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/core/service"
)

func swarmSession(t *testing.T, size int) domain.Session {
	t.Helper()
	sess, err := domain.NewSession("hive", uuid.New())
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if err := sess.AssignSwarmSize(size); err != nil {
		t.Fatalf("AssignSwarmSize(%d) error = %v", size, err)
	}
	return sess
}

// selectSession drives a selection through the model and returns the result.
func selectSession(t *testing.T, m Model, sess domain.Session) Model {
	t.Helper()
	updated, _ := m.Update(shared.SessionSelectedMsg{Session: sess})
	return updated.(Model)
}

// visibleLabels is the tab strip as a list of labels, in order.
func visibleLabels(m Model) []string {
	out := make([]string, 0, m.visibleCount())
	for _, ix := range m.visibleIxs() {
		out = append(out, m.views[ix].Label())
	}
	return out
}

func TestInspector_SwarmSession_ShowsBoardAndIndexedAgentTabs(t *testing.T) {
	m := selectSession(t, newTestModel(t), swarmSession(t, 3))

	got := visibleLabels(m)
	want := []string{"Agents Board", "Agent 1/3", "Shell"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("swarm tabs = %v, want %v", got, want)
	}
}

func TestInspector_SwarmSession_StartsOnTheBoard(t *testing.T) {
	m := selectSession(t, newTestModel(t), swarmSession(t, 2))

	if got := m.views[m.activeIx].Label(); got != "Agents Board" {
		t.Fatalf("active view label = %q, want %q — the board is how you follow a swarm", got, "Agents Board")
	}
}

func TestInspector_NonSwarmSession_KeepsTheOriginalTabs(t *testing.T) {
	sess, err := domain.NewSession("solo", uuid.New())
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	m := selectSession(t, newTestModel(t), sess)

	got := visibleLabels(m)
	want := []string{"Agent", "Shell"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("non-swarm tabs = %v, want %v — swarm support must not change existing sessions", got, want)
	}
}

func TestInspector_SwarmSession_TabCyclingStaysWithinVisibleTabs(t *testing.T) {
	m := selectSession(t, newTestModel(t), swarmSession(t, 4))

	// Board -> Agent -> Shell -> back to Board.
	want := []string{"Agent 1/4", "Shell", "Agents Board"}
	for i, wantLabel := range want {
		updated, _ := m.Update(keyPress("tab"))
		m = updated.(Model)
		if got := m.views[m.activeIx].Label(); got != wantLabel {
			t.Fatalf("tab press %d: active label = %q, want %q", i+1, got, wantLabel)
		}
	}
}

func TestInspector_SwarmSession_RevealEditorAppendsWithoutLosingSwarmTabs(t *testing.T) {
	m := selectSession(t, newTestModel(t), swarmSession(t, 2))

	updated, _ := m.Update(RevealEditorMsg{})
	m = updated.(Model)

	got := visibleLabels(m)
	want := []string{"Agents Board", "Agent 1/2", "Shell", "Editor"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("tabs after RevealEditorMsg = %v, want %v", got, want)
	}
	if label := m.views[m.activeIx].Label(); label != "Editor" {
		t.Fatalf("active label = %q, want Editor", label)
	}
}

func TestInspector_SwarmSession_SelectingAnotherSessionRehidesTheEditor(t *testing.T) {
	m := selectSession(t, newTestModel(t), swarmSession(t, 2))
	updated, _ := m.Update(RevealEditorMsg{})
	m = updated.(Model)

	m = selectSession(t, m, swarmSession(t, 2))

	if m.editorVisible {
		t.Fatal("editorVisible = true after session switch, want false")
	}
	if got := m.visibleCount(); got != 3 {
		t.Fatalf("visibleCount = %d, want 3 (board, agent, shell)", got)
	}
}

func TestInspector_SwarmAgentTab_BracketKeysCycleThroughPanes(t *testing.T) {
	m := selectSession(t, newTestModel(t), swarmSession(t, 3))

	// Move onto the Agent tab.
	updated, _ := m.Update(keyPress("tab"))
	m = updated.(Model)
	if got := m.views[m.activeIx].Label(); got != "Agent 1/3" {
		t.Fatalf("precondition: active label = %q, want Agent 1/3", got)
	}

	for _, want := range []string{"Agent 2/3", "Agent 3/3", "Agent 1/3"} {
		updated, _ = m.Update(keyPress("]"))
		m = updated.(Model)
		if got := m.views[m.activeIx].Label(); got != want {
			t.Fatalf("after ]: active label = %q, want %q", got, want)
		}
	}

	// And backwards, wrapping past the start.
	updated, _ = m.Update(keyPress("["))
	m = updated.(Model)
	if got := m.views[m.activeIx].Label(); got != "Agent 3/3" {
		t.Fatalf("after [: active label = %q, want Agent 3/3 (must wrap)", got)
	}
}

func TestInspector_BracketKeys_DoNothingOnOtherTabs(t *testing.T) {
	m := selectSession(t, newTestModel(t), swarmSession(t, 3))

	// Still on the board.
	updated, cmd := m.Update(keyPress("]"))
	m = updated.(Model)

	if cmd != nil {
		t.Fatal("] on the board returned a cmd, want nil — it must not restart a poll")
	}
	if got := m.views[m.activeIx].Label(); got != "Agents Board" {
		t.Fatalf("active label = %q, want Agents Board", got)
	}
}

func TestInspector_ActivePreviewKind_BoardHasNoPane(t *testing.T) {
	m := selectSession(t, newTestModel(t), swarmSession(t, 3))

	if _, _, ok := m.ActivePreviewKind(); ok {
		t.Fatal("ActivePreviewKind() ok = true on the Agents Board — this is what makes 'x' silently kill the shell")
	}
}

func TestInspector_ActivePreviewKind_SwarmAgentCarriesTheIndex(t *testing.T) {
	m := selectSession(t, newTestModel(t), swarmSession(t, 3))
	updated, _ := m.Update(keyPress("tab"))
	m = updated.(Model)
	updated, _ = m.Update(keyPress("]"))
	m = updated.(Model)

	kind, index, ok := m.ActivePreviewKind()
	if !ok {
		t.Fatal("ActivePreviewKind() ok = false on the Agent tab")
	}
	if kind != service.PreviewKindAgent {
		t.Fatalf("ActivePreviewKind() kind = %v, want agent", kind)
	}
	if index != 2 {
		t.Fatalf("ActivePreviewKind() index = %d, want 2 (the pane currently shown)", index)
	}
}

func TestInspector_ActivePreviewKind_NonSwarmTabsUnchanged(t *testing.T) {
	sess, _ := domain.NewSession("solo", uuid.New())
	m := selectSession(t, newTestModel(t), sess)

	kind, index, ok := m.ActivePreviewKind()
	if !ok || kind != service.PreviewKindAgent || index != 0 {
		t.Fatalf("ActivePreviewKind() = (%v, %d, %v), want (agent, 0, true)", kind, index, ok)
	}

	updated, _ := m.Update(keyPress("tab"))
	m = updated.(Model)
	if kind, _, ok := m.ActivePreviewKind(); !ok || kind != service.PreviewKindShell {
		t.Fatalf("ActivePreviewKind() on Shell = (%v, %v), want (shell, true)", kind, ok)
	}
}

func TestInspector_CapturesInput_OnlyWhileComposing(t *testing.T) {
	m := selectSession(t, newTestModel(t), swarmSession(t, 2))

	if m.CapturesInput() {
		t.Fatal("CapturesInput() = true before composing — the board tab would swallow every key")
	}

	updated, _ := m.Update(keyPress("i"))
	m = updated.(Model)
	if !m.CapturesInput() {
		t.Fatal("CapturesInput() = false after pressing i, want true")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updated.(Model)
	if m.CapturesInput() {
		t.Fatal("CapturesInput() = true after esc, want false")
	}
}

func TestInspector_ComposeMode_SwallowsTabInsteadOfSwitchingTabs(t *testing.T) {
	m := selectSession(t, newTestModel(t), swarmSession(t, 2))
	updated, _ := m.Update(keyPress("i"))
	m = updated.(Model)

	updated, _ = m.Update(keyPress("tab"))
	m = updated.(Model)

	if got := m.views[m.activeIx].Label(); got != "Agents Board" {
		t.Fatalf("active label = %q after tab while composing, want to stay on Agents Board", got)
	}
	if !m.CapturesInput() {
		t.Fatal("CapturesInput() = false after tab while composing, want the compose line to survive")
	}
}

func TestInspector_ComposeMode_NotAvailableOnNonSwarmSessions(t *testing.T) {
	sess, _ := domain.NewSession("solo", uuid.New())
	m := selectSession(t, newTestModel(t), sess)

	updated, _ := m.Update(keyPress("i"))
	m = updated.(Model)

	if m.CapturesInput() {
		t.Fatal("CapturesInput() = true on a non-swarm session, want false")
	}
}

func TestInspector_SwarmSession_BoardBodyShowsAStartupHint(t *testing.T) {
	m := selectSession(t, newTestModel(t), swarmSession(t, 3))
	m.SetSize(80, 20)

	body := m.views[ixBoard].Body()
	if !strings.Contains(body, "No board messages yet") {
		t.Fatalf("board body = %q, want an empty-state hint", body)
	}
}
