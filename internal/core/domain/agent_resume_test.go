package domain

import (
	"testing"

	"github.com/google/uuid"
)

func resumeTestSession(resumable bool, worktree string, swarm int) Session {
	return Session{
		ID:              uuid.MustParse("11111111-2222-3333-4444-555555555555"),
		WorktreePath:    worktree,
		SwarmSize:       swarm,
		ResumableAgents: resumable,
	}
}

func TestLauncherWithoutPrompt(t *testing.T) {
	tests := []struct {
		name, cmd, want string
	}{
		{"plain", "claude", "claude"},
		{"flags only", "claude --permission-mode auto", "claude --permission-mode auto"},
		{"review prompt", `claude --permission-mode auto "/ose-pr-review 92"`, "claude --permission-mode auto"},
		{"triage prompt", `claude "/ose-rdodcf-triage RDODCF-7699"`, "claude"},
		{"single quotes", `claude --debug 'fix it'`, "claude --debug"},
		{"quote inside a flag value is kept", `claude --append-system-prompt="be brief"`, `claude --append-system-prompt="be brief"`},
		{"unquoted prompt is a known ceiling", "claude fix the bug", "claude fix the bug"},
		{"trailing spaces", `claude "hi"  `, "claude"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LauncherWithoutPrompt(tt.cmd); got != tt.want {
				t.Fatalf("LauncherWithoutPrompt(%q) = %q, want %q", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestAgentConversationID_IsStableAndDistinctPerSwarmAgent(t *testing.T) {
	sess := resumeTestSession(true, "", 2)

	if sess.AgentConversationID(1) != sess.AgentConversationID(1) {
		t.Fatal("AgentConversationID(1) is not stable")
	}
	if sess.AgentConversationID(1) == sess.AgentConversationID(2) {
		t.Fatal("swarm agents 1 and 2 share a conversation ID")
	}
}

func TestAgentLaunchCommand(t *testing.T) {
	sess := resumeTestSession(true, "", 0)
	id := sess.AgentConversationID(1).String()

	tests := []struct {
		name string
		sess Session
		cmd  string
		want string
	}{
		{"plain", sess, "claude", "claude --session-id " + id},
		{"flags and prompt", sess, `claude --permission-mode auto "/ose-pr-review 92"`,
			`claude --session-id ` + id + ` --permission-mode auto "/ose-pr-review 92"`},
		{"not resumable", resumeTestSession(false, "", 0), "claude", "claude"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sess.AgentLaunchCommand(1, tt.cmd); got != tt.want {
				t.Fatalf("AgentLaunchCommand(%q) = %q, want %q", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestAgentRecreateCommand(t *testing.T) {
	resumable := resumeTestSession(true, "", 0)
	id := resumable.AgentConversationID(1).String()
	review := `claude --permission-mode auto "/ose-pr-review 92"`

	tests := []struct {
		name string
		sess Session
		cmd  string
		want string
	}{
		{"resumable drops the prompt and keeps flags", resumable, review,
			"claude --permission-mode auto --resume " + id + " || claude --permission-mode auto"},
		{"resumable worktree still resumes", resumeTestSession(true, "/wt", 0), "claude",
			"claude --resume " + id + " || claude"},
		{"old worktree continues", resumeTestSession(false, "/wt", 0), review,
			"claude --permission-mode auto --continue || claude --permission-mode auto"},
		{"old project session is unchanged", resumeTestSession(false, "", 0), review, review},
		{"opencode is unchanged", resumeTestSession(true, "/wt", 0), "opencode", "opencode"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sess.AgentRecreateCommand(1, tt.cmd); got != tt.want {
				t.Fatalf("AgentRecreateCommand(%q) = %q, want %q", tt.cmd, got, tt.want)
			}
		})
	}
}
