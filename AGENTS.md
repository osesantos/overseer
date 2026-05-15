# AGENTS.md — Overseer Root

## MUST

- MUST: Follow hexagonal architecture; domain has zero external deps; ports defined in domain package
- MUST: Use constructor injection; wire everything in `cmd/overseer/main.go`
- MUST: Implement vertical slices — each feature spans `domain/{feat}/`, `service/{feat}/`, `adapters/primary/tui/{feat}/`, optionally `adapters/secondary/.../`
- MUST: Follow TDD discipline — write failing test first, then implementation, then refactor; one commit per task
- MUST: All file writes are atomic (`tmp + rename` pattern via `internal/shared/paths.AtomicWrite`)
- MUST: All persistent paths are XDG-compliant via `internal/shared/paths`
- MUST: All log writes go to the log file (never stderr/stdout in TUI mode)

## MUST NOT

- MUST NOT: Add a DI framework (`fx`, `wire`, `dig`)
- MUST NOT: Use `pkg/` directory
- MUST NOT: Add a feature without updating its layer's AGENTS.md and the help registry
