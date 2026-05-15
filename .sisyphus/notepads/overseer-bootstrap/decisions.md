# Decisions — overseer-bootstrap

## [2026-05-15] Key Architectural Decisions
- Hexagonal architecture with `primary/secondary` naming (not `driving/driven`)
- Ports defined in domain package
- Constructor injection everywhere; wire in `cmd/overseer/main.go`
- Vertical slices by feature
- Stub adapters: real interface impls with canned responses (NOT TODO placeholders)
- Atomic JSON writes: `tmp + rename` pattern
- XDG-compliant paths via `internal/shared/paths`
- TDD: RED → GREEN → REFACTOR per task
- teatest v2 for TUI testing
- Golden files in `testdata/`; ANSI stripped via `termenv.Ascii`
- `bubbles/help` integration from day 1
- `errs.ErrNoOp` sentinel for boundary reorder (silent no-op)
- Session has `ProjectName string` field (no separate Project entity)
- Groups always expanded (no expand/collapse)
