package domain

import (
	"strings"

	"github.com/google/uuid"
)

// AgentConversationID returns the Claude conversation ID of the agent pane at
// index. It is derived, not stored, so the same session always yields the same ID.
func (s Session) AgentConversationID(index int) uuid.UUID {
	return uuid.NewSHA1(s.ID, []byte(s.AgentTmuxID(index)))
}

// AgentLaunchCommand returns the command that first starts the agent pane at
// index: cmd with a fixed --session-id for resumable sessions, cmd otherwise.
func (s Session) AgentLaunchCommand(index int, cmd string) string {
	if !s.ResumableAgents {
		return cmd
	}
	bin, rest, _ := strings.Cut(strings.TrimSpace(cmd), " ")
	return strings.TrimSpace(bin + " --session-id " + s.AgentConversationID(index).String() + " " + rest)
}

// AgentRecreateCommand returns the command that restarts a missing agent pane:
// resume its conversation for resumable sessions, continue the worktree's latest
// one for older Claude sessions, and cmd unchanged for anything else. A failed
// resume falls back to the launcher without its prompt, so a prompt is never re-sent.
func (s Session) AgentRecreateCommand(index int, cmd string) string {
	if InferAgentType(cmd) != AgentTypeClaudeCode {
		return cmd
	}
	launcher := LauncherWithoutPrompt(cmd)
	switch {
	case s.ResumableAgents:
		return launcher + " --resume " + s.AgentConversationID(index).String() + " || " + launcher
	case s.HasWorktree():
		return launcher + " --continue || " + launcher
	}
	return cmd
}

// LauncherWithoutPrompt drops one trailing quoted argument ("…" or '…'), which is
// how every prompt Overseer writes ends.
// ponytail: an unquoted trailing prompt (claude fix the bug) is not detected; split
// AgentCommand into launcher + prompt if one ever appears.
func LauncherWithoutPrompt(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if len(cmd) < 2 {
		return cmd
	}
	quote := cmd[len(cmd)-1]
	if quote != '"' && quote != '\'' {
		return cmd
	}
	open := strings.LastIndexByte(cmd[:len(cmd)-1], quote)
	if open <= 0 || cmd[open-1] != ' ' {
		return cmd
	}
	return strings.TrimSpace(cmd[:open])
}
