# ADR 0002: Stub Mode for Bootstrap

## Status

Accepted

## Context

The full Overseer vision includes real tmux session management, git worktree integration, and agent launcher integration. However, building real integrations during bootstrap would delay establishing the architectural foundation and testing framework. David explicitly decided: focus the bootstrap on framework, not integrations.

## Decision

All external integrations (tmux, git, agent) are stubbed for the bootstrap:

- `internal/adapters/secondary/tmux/stub/` — implements `session.TmuxAdapter` with canned responses
- `internal/adapters/secondary/git/stub/` — implements `session.GitAdapter` with canned responses
- `internal/adapters/secondary/agent/stub/` — implements `session.AgentLauncher` with canned responses
- Sessions are JSON records only; no real tmux sessions are created
- Stub adapters satisfy the full port interface with real (canned) return values — no `// TODO` placeholders

## Consequences

- Bootstrap delivers a working TUI with real architecture but no external process dependencies
- Real integrations are post-bootstrap work: swap stub adapters for real implementations
- The port interfaces defined in the domain layer are the contract; stubs prove the interfaces are usable
- Integration tests for stubs are minimal (compile-time interface satisfaction + call recording)
