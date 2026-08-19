package inspector

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/core/service"
)

func newTestModel(t *testing.T) Model {
	t.Helper()
	return New(styles.New(), service.SessionService{}, nil, 500*time.Millisecond)
}

func keyPress(value string) tea.KeyPressMsg {
	switch value {
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	}
	return tea.KeyPressMsg{Text: value, Code: []rune(value)[0]}
}

func TestInspector_StartsOnAgentView(t *testing.T) {
	m := newTestModel(t)
	if got := m.views[m.activeIx].Label(); got != "Agent" {
		t.Errorf("active view label = %q, want %q", got, "Agent")
	}
}

func TestInspector_ToggleKey_CyclesForward(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(keyPress("tab"))
	m = updated.(Model)
	if got := m.views[m.activeIx].Label(); got != "Shell" {
		t.Errorf("after tab, active view label = %q, want %q", got, "Shell")
	}
}

func TestInspector_ToggleKey_WrapsAround(t *testing.T) {
	m := newTestModel(t)
	// Only the visible views cycle (Editor is gated). With Agent + Shell that
	// is 2 tabs: Agent → Shell → Agent (wraps on the 2nd press).
	presses := m.visibleCount()
	for range presses {
		updated, _ := m.Update(keyPress("tab"))
		m = updated.(Model)
	}
	if got := m.views[m.activeIx].Label(); got != "Agent" {
		t.Errorf("after %dx tab, active view label = %q, want %q", presses, got, "Agent")
	}
}

func TestInspector_RevealEditorMsg_ShowsAndActivatesEditorTab(t *testing.T) {
	m := newTestModel(t)
	if m.visibleCount() != 2 {
		t.Fatalf("precondition: visibleCount = %d, want 2 (Editor hidden)", m.visibleCount())
	}

	updated, _ := m.Update(RevealEditorMsg{})
	m = updated.(Model)

	if m.visibleCount() != 3 {
		t.Errorf("after reveal: visibleCount = %d, want 3", m.visibleCount())
	}
	if got := m.views[m.activeIx].Label(); got != "Editor" {
		t.Errorf("after reveal: active view label = %q, want %q", got, "Editor")
	}
	if got := m.ActiveViewLabel(); got != "Editor" {
		t.Errorf("ActiveViewLabel() = %q, want %q", got, "Editor")
	}
}

func TestInspector_ToggleKey_CyclesThroughEditorOnceRevealed(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(RevealEditorMsg{})
	m = updated.(Model)
	// From Editor: tab wraps to Agent, then Shell, then back to Editor.
	wantSequence := []string{"Agent", "Shell", "Editor"}
	for i, want := range wantSequence {
		updated, _ := m.Update(keyPress("tab"))
		m = updated.(Model)
		if got := m.views[m.activeIx].Label(); got != want {
			t.Fatalf("tab %d: active view label = %q, want %q", i+1, got, want)
		}
	}
}

func TestInspector_SessionSelectedMsg_RehidesEditorTab(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(RevealEditorMsg{})
	m = updated.(Model)
	if m.visibleCount() != 3 {
		t.Fatalf("precondition: Editor not revealed")
	}

	updated, _ = m.Update(shared.SessionSelectedMsg{Session: domain.Session{ID: uuid.New()}})
	m = updated.(Model)

	if m.visibleCount() != 2 {
		t.Errorf("after session switch: visibleCount = %d, want 2 (Editor re-hidden)", m.visibleCount())
	}
	if got := m.views[m.activeIx].Label(); got != "Agent" {
		t.Errorf("after session switch: active view label = %q, want %q", got, "Agent")
	}
}

func TestInspector_SessionSelectedMsg_PropagatesToAllViews(t *testing.T) {
	m := newTestModel(t)
	id := uuid.New()
	updated, _ := m.Update(shared.SessionSelectedMsg{Session: domain.Session{ID: id}})
	m = updated.(Model)
	// Every view receives the whole Session via SetSession.
	if agent := m.views[ixAgent].(*streamView); agent.sessionID != id {
		t.Errorf("agent view sessionID = %v, want %v", agent.sessionID, id)
	}
	for i, v := range m.views {
		sv, ok := v.(*streamView)
		if !ok {
			continue
		}
		if sv.sessionID != id {
			t.Errorf("view[%d] sessionID = %v, want %v", i, sv.sessionID, id)
		}
	}
}

func TestInspector_SessionSelectedMsg_ResetsToAgentView(t *testing.T) {
	m := newTestModel(t)
	updated, _ := m.Update(keyPress("tab"))
	m = updated.(Model)
	if got := m.views[m.activeIx].Label(); got != "Shell" {
		t.Fatalf("precondition failed: active view label = %q, want %q", got, "Shell")
	}

	id := uuid.New()
	updated, _ = m.Update(shared.SessionSelectedMsg{Session: domain.Session{ID: id}})
	m = updated.(Model)

	if got := m.views[m.activeIx].Label(); got != "Agent" {
		t.Errorf("after SessionSelectedMsg, active view label = %q, want %q", got, "Agent")
	}
}

func TestInspector_PreviewCapturedMsg_OnlyActiveViewProcesses(t *testing.T) {
	m := newTestModel(t)
	id := uuid.New()
	for i := range m.views {
		m.views[i].SetSession(domain.Session{ID: id})
	}
	msg := previewCapturedMsg{
		kind:         viewKindAgent,
		sessionID:    id,
		content:      "agent stream",
		sessionReady: true,
	}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	agent := m.views[ixAgent].(*streamView)
	if agent.content != "agent stream" {
		t.Errorf("agent content = %q, want %q", agent.content, "agent stream")
	}

	// Switch to Shell and send an Agent-kind message; Shell should ignore it.
	updated, _ = m.Update(keyPress("tab"))
	m = updated.(Model)
	staleMsg := previewCapturedMsg{
		kind:         viewKindAgent,
		sessionID:    id,
		content:      "stale agent",
		sessionReady: true,
	}
	updated, _ = m.Update(staleMsg)
	m = updated.(Model)

	shell := m.views[ixShell].(*streamView)
	if shell.content != "" {
		t.Errorf("shell received agent-kind message: content = %q, want empty", shell.content)
	}
}

func TestInspector_View_HeightStaysWithinDeclaredSizeWhenStreaming(t *testing.T) {
	const width, height = 80, 20

	m := newTestModel(t)
	m.SetSize(width, height)

	id := uuid.New()
	updated, _ := m.Update(shared.SessionSelectedMsg{Session: domain.Session{ID: id}})
	m = updated.(Model)

	innerH := height - tabStripHeight - 2
	streamingContent := strings.Repeat("agent-output-row\n", innerH+3)
	updated, _ = m.Update(previewCapturedMsg{
		kind:         viewKindAgent,
		sessionID:    id,
		content:      streamingContent,
		sessionReady: true,
	})
	m = updated.(Model)

	out := m.View().Content
	if got := lipgloss.Height(out); got != height {
		t.Errorf("Inspector.View() height with streaming content = %d, want %d (must not overflow declared height — overflow pushes dashboard help bar off-screen and breaks pane-height symmetry)", got, height)
	}
}

func TestInspector_SessionSelectionClearedMsg_ResetsAllViews(t *testing.T) {
	m := newTestModel(t)
	id := uuid.New()
	updated, _ := m.Update(shared.SessionSelectedMsg{Session: domain.Session{ID: id}})
	m = updated.(Model)
	// Seed stale content in the stream views (Agent, Shell).
	for _, v := range m.views {
		sv, ok := v.(*streamView)
		if !ok {
			continue
		}
		sv.content = "stale-content"
		sv.ready = true
	}

	updated, cmd := m.Update(shared.SessionSelectionClearedMsg{})
	m = updated.(Model)

	if cmd != nil {
		t.Errorf("Update(SessionSelectionClearedMsg) cmd = %#v, want nil (no further polling)", cmd)
	}
	if agent := m.views[ixAgent].(*streamView); agent.sessionID != uuid.Nil {
		t.Errorf("agent view sessionID = %v, want uuid.Nil", agent.sessionID)
	}
	for i, v := range m.views {
		sv, ok := v.(*streamView)
		if !ok {
			continue
		}
		if sv.sessionID != uuid.Nil {
			t.Errorf("view[%d] sessionID = %v, want uuid.Nil", i, sv.sessionID)
		}
		if sv.content != "" {
			t.Errorf("view[%d] content = %q, want empty after clear", i, sv.content)
		}
		if sv.ready {
			t.Errorf("view[%d] ready = true, want false after clear", i)
		}
	}
}
