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

// --- handleLoopEvalResult ---

func newTestModel(sessID uuid.UUID, ls *domain.LoopState) Model {
	loops := map[uuid.UUID]*domain.LoopState{sessID: ls}
	return Model{loops: loops}
}

func TestHandleLoopEvalResult_DoneBranch(t *testing.T) {
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
	msg := overseerLoopEvalResultMsg{
		state: *ls,
		eval:  domain.LoopEvaluation{Done: true, Summary: "All tests pass."},
	}
	_, cmd := m.handleLoopEvalResult(msg)

	if ls.Status != domain.LoopStatusDone {
		t.Errorf("Status = %s, want %s", ls.Status, domain.LoopStatusDone)
	}
	if ls.Iterations != 3 {
		t.Errorf("Iterations = %d, want 3", ls.Iterations)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
}

func TestHandleLoopEvalResult_ErrorBranch_SchedulesNextTick(t *testing.T) {
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
	msg := overseerLoopEvalResultMsg{
		state: *ls,
		err:   errSentinel("eval failed"),
	}
	_, cmd := m.handleLoopEvalResult(msg)

	if ls.Status == domain.LoopStatusStopped {
		t.Error("single error should not stop the loop")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd to schedule next tick")
	}
}

func TestHandleLoopEvalResult_ErrorBranch_MaxConsecutiveErrors_StopsLoop(t *testing.T) {
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
	msg := overseerLoopEvalResultMsg{
		state: *ls,
		err:   errSentinel("persistent failure"),
	}
	_, cmd := m.handleLoopEvalResult(msg)

	if ls.Status != domain.LoopStatusStopped {
		t.Errorf("Status = %s, want stopped after 3 consecutive errors", ls.Status)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
}

func TestHandleLoopEvalResult_MaxIterations_StopsLoop(t *testing.T) {
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
	// Use a question-style prompt so the heuristic treats the agent as
	// waiting for input (not still working) — only then does the max-iterations
	// guard trigger.
	msg := overseerLoopEvalResultMsg{
		state: *ls,
		eval:  domain.LoopEvaluation{Done: false, PromptToSend: "should I run the tests again?"},
	}
	_, cmd := m.handleLoopEvalResult(msg)

	if ls.Status != domain.LoopStatusStopped {
		t.Errorf("Status = %s, want stopped", ls.Status)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
}

func TestHandleLoopEvalResult_ProgressBranch_SendsPromptAndSchedulesTick(t *testing.T) {
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
	// Question-style prompt → heuristic says agent is waiting → normal progress branch.
	msg := overseerLoopEvalResultMsg{
		state: *ls,
		eval:  domain.LoopEvaluation{Done: false, PromptToSend: "should I run the failing tests again?"},
	}
	_, cmd := m.handleLoopEvalResult(msg)

	if ls.Status != domain.LoopStatusRunning {
		t.Errorf("Status = %s, want still running", ls.Status)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd for progress branch")
	}
}

func TestHandleLoopEvalResult_StoppedLoop_Ignored(t *testing.T) {
	sessID := uuid.New()
	ls := &domain.LoopState{
		SessionID: sessID,
		Status:    domain.LoopStatusStopped,
	}
	m := newTestModel(sessID, ls)
	msg := overseerLoopEvalResultMsg{
		state: *ls,
		eval:  domain.LoopEvaluation{Done: true},
	}
	_, cmd := m.handleLoopEvalResult(msg)

	if cmd != nil {
		t.Fatal("expected nil cmd for already-stopped loop")
	}
}

func TestHandleLoopEvalResult_AgentStillWorking_WAIT_SkipsIteration(t *testing.T) {
	sessID := uuid.New()
	ls := &domain.LoopState{
		SessionID:     sessID,
		SessionName:   "my-session",
		Status:        domain.LoopStatusRunning,
		Iterations:    9,  // would hit MaxIterations on a normal eval
		MaxIterations: 10,
		StartedAt:     time.Now(),
	}
	m := newTestModel(sessID, ls)
	msg := overseerLoopEvalResultMsg{
		state: *ls,
		eval:  domain.LoopEvaluation{AgentStillWorking: true},
	}
	_, cmd := m.handleLoopEvalResult(msg)

	if ls.Status != domain.LoopStatusRunning {
		t.Errorf("Status = %s, want still running (WAIT skips max-iterations guard)", ls.Status)
	}
	if ls.Iterations != 9 {
		t.Errorf("Iterations = %d, want 9 (WAIT does not increment)", ls.Iterations)
	}
	if ls.ConsecutiveWaits != 1 {
		t.Errorf("ConsecutiveWaits = %d, want 1", ls.ConsecutiveWaits)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd to schedule next tick")
	}
}

func TestHandleLoopEvalResult_AgentStillWorking_DirectivePrompt_SkipsIteration(t *testing.T) {
	sessID := uuid.New()
	ls := &domain.LoopState{
		SessionID:     sessID,
		SessionName:   "my-session",
		Status:        domain.LoopStatusRunning,
		Iterations:    9,
		MaxIterations: 10,
		StartedAt:     time.Now(),
	}
	m := newTestModel(sessID, ls)
	// Directive with no question mark → heuristic says agent still working.
	msg := overseerLoopEvalResultMsg{
		state: *ls,
		eval:  domain.LoopEvaluation{Done: false, PromptToSend: "run the failing tests"},
	}
	_, cmd := m.handleLoopEvalResult(msg)

	if ls.Status != domain.LoopStatusRunning {
		t.Errorf("Status = %s, want still running", ls.Status)
	}
	if ls.Iterations != 9 {
		t.Errorf("Iterations = %d, want 9 (directive prompt does not increment)", ls.Iterations)
	}
	if ls.ConsecutiveWaits != 1 {
		t.Errorf("ConsecutiveWaits = %d, want 1", ls.ConsecutiveWaits)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd to schedule next tick")
	}
}

func TestHandleLoopEvalResult_AgentStillWorking_ExceedsMaxWaits_StopsLoop(t *testing.T) {
	sessID := uuid.New()
	ls := &domain.LoopState{
		SessionID:        sessID,
		SessionName:      "my-session",
		Status:           domain.LoopStatusRunning,
		Iterations:       5,
		MaxIterations:    40,
		ConsecutiveWaits: loopMaxConsecutiveWaits - 1, // one more will hit the cap
		StartedAt:        time.Now(),
	}
	m := newTestModel(sessID, ls)
	msg := overseerLoopEvalResultMsg{
		state: *ls,
		eval:  domain.LoopEvaluation{AgentStillWorking: true},
	}
	_, cmd := m.handleLoopEvalResult(msg)

	if ls.Status != domain.LoopStatusStopped {
		t.Errorf("Status = %s, want stopped after %d consecutive waits", ls.Status, loopMaxConsecutiveWaits)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
}

// errSentinel is a minimal error type for tests.
type errSentinel string

func (e errSentinel) Error() string { return string(e) }
