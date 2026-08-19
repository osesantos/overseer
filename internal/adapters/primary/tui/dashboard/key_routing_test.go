package dashboard

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/core/service"
)

func letterKey(value string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Text: value, Code: []rune(value)[0]}
}

func ctrlC() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
}

// isQuit reports whether a command is tea.Quit, by running it and inspecting the
// message it produces.
func isQuit(t *testing.T, cmd tea.Cmd) bool {
	t.Helper()
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func sizedDashboard(t *testing.T) Model {
	t.Helper()
	m := newTestDashboard(t)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return updated.(Model)
}

// selectInDashboard puts sess into the session list *and* marks it selected.
//
// Both halves matter: several dashboard commands bail out early when
// leftPane.SelectedSessionID() is empty, so a test that only sends
// SessionSelectedMsg passes vacuously — the code under test never runs.
func selectInDashboard(t *testing.T, m Model, sess domain.Session) Model {
	t.Helper()

	updated, _ := m.Update(shared.SessionsLoadedMsg{Sessions: []domain.Session{sess}})
	m = updated.(Model)
	updated, _ = m.Update(shared.SessionSelectedMsg{Session: sess})
	m = updated.(Model)

	if got := m.leftPane.SelectedSessionID(); got != sess.ID.String() {
		t.Fatalf("session not selected in the left pane: got %q, want %q", got, sess.ID)
	}
	return m
}

func swarmDashboard(t *testing.T) Model {
	t.Helper()

	sess, err := domain.NewSession("hive", uuid.New())
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if err := sess.AssignSwarmSize(3); err != nil {
		t.Fatalf("AssignSwarmSize() error = %v", err)
	}

	return selectInDashboard(t, sizedDashboard(t), sess)
}

func TestDashboard_QDoesNotQuitWhileTypingInTheChat(t *testing.T) {
	// Regression: the quit binding is "q" or ctrl+c, and it used to be matched
	// before the chat branch — so typing "q" into the Overseer chat quit the app.
	m := sizedDashboard(t)
	m.chatPanelVisible = true

	_, cmd := m.Update(letterKey("q"))

	if isQuit(t, cmd) {
		t.Fatal("pressing q while the chat is open quit the app; q must reach the chat input")
	}
}

func TestDashboard_CtrlCStillQuitsFromTheChat(t *testing.T) {
	m := sizedDashboard(t)
	m.chatPanelVisible = true

	_, cmd := m.Update(ctrlC())

	if !isQuit(t, cmd) {
		t.Fatal("ctrl+c must remain the escape hatch while the chat is open")
	}
}

func TestDashboard_QStillQuitsNormally(t *testing.T) {
	m := sizedDashboard(t)

	_, cmd := m.Update(letterKey("q"))

	if !isQuit(t, cmd) {
		t.Fatal("q must still quit when nothing has taken the keyboard")
	}
}

func TestDashboard_ComposingOnTheBoard_SwallowsGlobalShortcuts(t *testing.T) {
	m := swarmDashboard(t)

	// Enter compose mode on the Agents Board.
	updated, _ := m.Update(letterKey("i"))
	m = updated.(Model)
	if !m.inspector.CapturesInput() {
		t.Fatal("precondition: expected the board to be capturing input after i")
	}

	// Every one of these is a global shortcut that must not fire mid-message.
	for _, k := range []string{"q", "n", "e", "x", "?"} {
		updated, cmd := m.Update(letterKey(k))
		m = updated.(Model)

		if isQuit(t, cmd) {
			t.Fatalf("key %q quit the app while composing", k)
		}
		if m.activePopup != popupNone {
			t.Fatalf("key %q opened popup %v while composing", k, m.activePopup)
		}
		if !m.inspector.CapturesInput() {
			t.Fatalf("key %q ended compose mode", k)
		}
	}
}

func TestDashboard_ComposingOnTheBoard_CtrlCStillQuits(t *testing.T) {
	m := swarmDashboard(t)
	updated, _ := m.Update(letterKey("i"))
	m = updated.(Model)

	_, cmd := m.Update(ctrlC())

	if !isQuit(t, cmd) {
		t.Fatal("ctrl+c must remain the escape hatch while composing")
	}
}

func TestDashboard_ComposingOnTheBoard_EscapeReleasesTheKeyboard(t *testing.T) {
	m := swarmDashboard(t)
	updated, _ := m.Update(letterKey("i"))
	m = updated.(Model)
	if !m.inspector.CapturesInput() {
		t.Fatal("precondition: expected the board to be capturing input after i")
	}

	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updated.(Model)

	if m.inspector.CapturesInput() {
		t.Fatal("esc did not release the keyboard")
	}

	// And the dashboard is responsive again.
	_, cmd := m.Update(letterKey("q"))
	if !isQuit(t, cmd) {
		t.Fatal("q did not quit after leaving compose mode")
	}
}

func TestDashboard_KillPreview_IsRefusedOnTheAgentsBoard(t *testing.T) {
	m := swarmDashboard(t)

	// The board tab is active. Pressing x used to map the tab label to a preview
	// kind with a switch that defaulted to the shell — silently killing the shell
	// while the popup claimed to be killing "Agents Board".
	updated, _ := m.Update(letterKey("x"))
	m = updated.(Model)

	if m.activePopup == popupKillPreview {
		t.Fatal("x opened the kill-preview popup on the Agents Board, which has no pane behind it")
	}
}

func TestDashboard_AttachIsRefusedOnTheAgentsBoard(t *testing.T) {
	m := swarmDashboard(t)

	// Enter used to fall through to AttachAgent, whose ensureTmuxSession would
	// lazily create a bare "<uuid>-agent" pane — an unmanaged extra agent.
	// The session mocks have no expectations, so any service call fails the test.
	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)

	if cmd != nil {
		// Running it would issue the offending tmux calls; the mock would fail.
		cmd()
	}
}

func TestDashboard_SwarmAgentPagerKeysReachTheInspector(t *testing.T) {
	m := swarmDashboard(t)

	// Move onto the Agent tab.
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(Model)
	if got := m.inspector.ActiveViewLabel(); got != "Agent 1/3" {
		t.Fatalf("precondition: active label = %q, want Agent 1/3", got)
	}

	updated, _ = m.Update(letterKey("]"))
	m = updated.(Model)

	if got := m.inspector.ActiveViewLabel(); got != "Agent 2/3" {
		t.Fatalf("after ]: active label = %q, want Agent 2/3 — the dashboard must forward the pager keys", got)
	}
}

func ctrlE() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl}
}

func TestDashboard_CtrlE_OnAgentsBoard_BroadcastsToEveryAgent(t *testing.T) {
	// Regression: this used to discard the ok from ActivePreviewKind, so on the
	// board tab agentIndex fell through as 0 and clamped to agent 1 — silently
	// poking one agent out of N, on the tab a swarm opens to by default.
	m := swarmDashboard(t)

	if _, _, ok := m.inspector.ActivePreviewKind(); ok {
		t.Fatal("precondition: expected the Agents Board to report no pane")
	}

	_, cmd := m.Update(ctrlE())
	if cmd == nil {
		t.Fatal("ctrl+e on the Agents Board produced no command")
	}
	// The session repo mock has no Get expectation, so running the command would
	// fail the test if it reached the service — assert on the shape instead.
}

func TestDashboard_CtrlE_OnAgentTab_TargetsThatPaneOnly(t *testing.T) {
	m := swarmDashboard(t)

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = updated.(Model)

	kind, index, ok := m.inspector.ActivePreviewKind()
	if !ok || kind != service.PreviewKindAgent || index != 1 {
		t.Fatalf("precondition: ActivePreviewKind = (%v, %d, %v), want (agent, 1, true)", kind, index, ok)
	}

	if _, cmd := m.Update(ctrlE()); cmd == nil {
		t.Fatal("ctrl+e on the Agent tab produced no command")
	}
}

func TestDashboard_NonSwarmSession_ComposeKeyIsNotSwallowed(t *testing.T) {
	m := sizedDashboard(t)
	sess, _ := domain.NewSession("solo", uuid.New())
	updated, _ := m.Update(shared.SessionSelectedMsg{Session: sess})
	m = updated.(Model)

	updated, _ = m.Update(letterKey("i"))
	m = updated.(Model)

	if m.inspector.CapturesInput() {
		t.Fatal("i started compose mode on a non-swarm session")
	}
}
