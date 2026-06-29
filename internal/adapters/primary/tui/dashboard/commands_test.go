package dashboard

import (
	"strings"
	"testing"

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
	m := Model{}
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
