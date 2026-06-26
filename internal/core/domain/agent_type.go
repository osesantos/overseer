package domain

import "strings"

// InferAgentType maps a legacy session's AgentCommand to an AgentType using
// a simple prefix match against the built-in defaults. The first token of the
// command (e.g. "claude" from "claude --debug") is compared to the known
// agent commands; anything else falls back to AgentTypeUnknown, which keeps
// the session routable through the registry (resolves to no detector →
// Unknown status).
func InferAgentType(agentCommand string) AgentType {
	first := strings.TrimSpace(agentCommand)
	if first == "" {
		return AgentTypeUnknown
	}
	if idx := strings.IndexAny(first, " \t"); idx >= 0 {
		first = first[:idx]
	}
	switch first {
	case "claude":
		return AgentTypeClaudeCode
	case "opencode":
		return AgentTypeOpenCode
	}
	return AgentTypeUnknown
}
