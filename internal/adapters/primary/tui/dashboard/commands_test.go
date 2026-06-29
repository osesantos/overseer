package dashboard

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/core/domain"
)

// --- matchSession ---

func TestMatchSession_ExactSingleWord(t *testing.T) {
	sessions := []domain.Session{{Name: "alpha"}, {Name: "beta"}}
	sess, rest, ok := matchSession([]string{"alpha"}, sessions)
	if !ok {
		t.Fatal("expected match")
	}
	if sess.Name != "alpha" {
		t.Errorf("Name = %q, want %q", sess.Name, "alpha")
	}
	if rest != "" {
		t.Errorf("rest = %q, want empty", rest)
	}
}

func TestMatchSession_MultiWordWithTrailingArgs(t *testing.T) {
	sessions := []domain.Session{{Name: "overseer improvements"}, {Name: "other"}}
	sess, rest, ok := matchSession([]string{"overseer", "improvements", "run", "tests"}, sessions)
	if !ok {
		t.Fatal("expected match")
	}
	if sess.Name != "overseer improvements" {
		t.Errorf("Name = %q, want %q", sess.Name, "overseer improvements")
	}
	if rest != "run tests" {
		t.Errorf("rest = %q, want %q", rest, "run tests")
	}
}

func TestMatchSession_NoMatch(t *testing.T) {
	sessions := []domain.Session{{Name: "alpha"}, {Name: "beta"}}
	_, _, ok := matchSession([]string{"gamma"}, sessions)
	if ok {
		t.Fatal("expected no match")
	}
}

func TestMatchSession_AllArgsMatch_EmptyRemainder(t *testing.T) {
	sessions := []domain.Session{{Name: "my session"}}
	sess, rest, ok := matchSession([]string{"my", "session"}, sessions)
	if !ok {
		t.Fatal("expected match")
	}
	if sess.Name != "my session" {
		t.Errorf("Name = %q", sess.Name)
	}
	if rest != "" {
		t.Errorf("rest = %q, want empty", rest)
	}
}

func TestMatchSession_CaseInsensitive(t *testing.T) {
	sessions := []domain.Session{{Name: "MySession"}}
	_, _, ok := matchSession([]string{"mysession"}, sessions)
	if !ok {
		t.Fatal("expected case-insensitive match")
	}
}

// --- parseCommand ---

func TestParseCommand(t *testing.T) {
	tests := []struct {
		raw      string
		wantName string
		wantArgs []string
	}{
		{"/send alpha hello world", "send", []string{"alpha", "hello", "world"}},
		{"/HELP", "help", nil},
		{"/loop stop mysess", "loop", []string{"stop", "mysess"}},
		{"  /list  ", "list", nil},
		{"/new", "new", nil},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			name, args := parseCommand(tt.raw)
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if len(args) != len(tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
				return
			}
			for i, a := range args {
				if a != tt.wantArgs[i] {
					t.Errorf("args[%d] = %q, want %q", i, a, tt.wantArgs[i])
				}
			}
		})
	}
}

// --- executeCommand routing ---

func TestExecuteCommand_UnknownCommand_ReturnsErrorResult(t *testing.T) {
	m := Model{loops: make(map[uuid.UUID]*domain.LoopState)}
	_, cmd := m.executeCommand("/unknowncmd")
	if cmd == nil {
		t.Fatal("expected non-nil cmd for unknown command")
	}
	msg, ok := cmd().(shared.OverseerCommandResultMsg)
	if !ok {
		t.Fatalf("expected OverseerCommandResultMsg, got %T", cmd())
	}
	if !msg.IsError {
		t.Error("expected IsError=true for unknown command")
	}
	if !strings.Contains(msg.Text, "unknown command") {
		t.Errorf("text %q should mention 'unknown command'", msg.Text)
	}
}

// --- handleLoopTaskCompleted ---

func newTestModel(sessID uuid.UUID, ls *domain.LoopState) Model {
	loops := map[uuid.UUID]*domain.LoopState{sessID: ls}
	return Model{loops: loops}
}

func TestHandleLoopTaskCompleted_EndSentinel_CompletesLoop(t *testing.T) {
	sessID := uuid.New()
	ls := &domain.LoopState{
		SessionID:     sessID,
		SessionName:   "my-session",
		Status:        domain.LoopStatusRunning,
		Iterations:    2,
		MaxIterations: 10,
		StartedAt:     time.Now(),
	}
	m := newTestModel(sessID, ls)
	msg := loopTaskCompletedMsg{
		sessionID: sessID,
		output:    "I finished the task.\nEND\n",
	}
	_, cmd := m.handleLoopTaskCompleted(msg)

	if ls.Status != domain.LoopStatusDone {
		t.Errorf("Status = %s, want done", ls.Status)
	}
	if ls.Iterations != 3 {
		t.Errorf("Iterations = %d, want 3", ls.Iterations)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
}

func TestHandleLoopTaskCompleted_ErrorBranch_SchedulesNextTick(t *testing.T) {
	sessID := uuid.New()
	ls := &domain.LoopState{
		SessionID:         sessID,
		SessionName:       "my-session",
		Status:            domain.LoopStatusRunning,
		Iterations:        0,
		MaxIterations:     10,
		ConsecutiveErrors: 0,
		StartedAt:         time.Now(),
	}
	m := newTestModel(sessID, ls)
	msg := loopTaskCompletedMsg{
		sessionID: sessID,
		err:       errSentinel("task failed"),
	}
	_, cmd := m.handleLoopTaskCompleted(msg)

	if ls.Status == domain.LoopStatusStopped {
		t.Error("single error should not stop the loop")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd to schedule next tick")
	}
}

func TestHandleLoopTaskCompleted_MaxConsecutiveErrors_StopsLoop(t *testing.T) {
	sessID := uuid.New()
	ls := &domain.LoopState{
		SessionID:         sessID,
		SessionName:       "my-session",
		Status:            domain.LoopStatusRunning,
		Iterations:        5,
		MaxIterations:     20,
		ConsecutiveErrors: 2, // one more will hit the max of 3
		StartedAt:         time.Now(),
	}
	m := newTestModel(sessID, ls)
	msg := loopTaskCompletedMsg{
		sessionID: sessID,
		err:       errSentinel("persistent failure"),
	}
	_, cmd := m.handleLoopTaskCompleted(msg)

	if ls.Status != domain.LoopStatusStopped {
		t.Errorf("Status = %s, want stopped after 3 consecutive errors", ls.Status)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
}

func TestHandleLoopTaskCompleted_MaxIterations_StopsLoop(t *testing.T) {
	sessID := uuid.New()
	ls := &domain.LoopState{
		SessionID:     sessID,
		SessionName:   "my-session",
		Status:        domain.LoopStatusRunning,
		Iterations:    9, // will become 10 == MaxIterations
		MaxIterations: 10,
		StartedAt:     time.Now(),
	}
	m := newTestModel(sessID, ls)
	msg := loopTaskCompletedMsg{
		sessionID: sessID,
		output:    "still working...",
	}
	_, cmd := m.handleLoopTaskCompleted(msg)

	if ls.Status != domain.LoopStatusStopped {
		t.Errorf("Status = %s, want stopped", ls.Status)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
}

func TestHandleLoopTaskCompleted_NormalTick_SchedulesNextTick(t *testing.T) {
	sessID := uuid.New()
	ls := &domain.LoopState{
		SessionID:     sessID,
		SessionName:   "my-session",
		Status:        domain.LoopStatusRunning,
		Iterations:    1,
		MaxIterations: 10,
		StartedAt:     time.Now(),
	}
	m := newTestModel(sessID, ls)
	msg := loopTaskCompletedMsg{
		sessionID: sessID,
		output:    "working on it",
	}
	_, cmd := m.handleLoopTaskCompleted(msg)

	if ls.Status != domain.LoopStatusRunning {
		t.Errorf("Status = %s, want still running", ls.Status)
	}
	if ls.Iterations != 2 {
		t.Errorf("Iterations = %d, want 2", ls.Iterations)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd for next tick")
	}
}

func TestHandleLoopTaskCompleted_StoppedLoop_Ignored(t *testing.T) {
	sessID := uuid.New()
	ls := &domain.LoopState{
		SessionID: sessID,
		Status:    domain.LoopStatusStopped,
	}
	m := newTestModel(sessID, ls)
	msg := loopTaskCompletedMsg{
		sessionID: sessID,
		output:    "END",
	}
	_, cmd := m.handleLoopTaskCompleted(msg)

	if cmd != nil {
		t.Fatal("expected nil cmd for already-stopped loop")
	}
}

// errSentinel is a minimal error type for tests.
type errSentinel string

func (e errSentinel) Error() string { return string(e) }
