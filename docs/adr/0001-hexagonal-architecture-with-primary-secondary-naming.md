# ADR 0001: Hexagonal Architecture with Primary/Secondary Naming

## Status

Accepted

## Context

Overseer needs a clear extension story. David's explicit requirement is "standardization > YAGNI" — the process to add a new feature must be mechanical and well-defined. The codebase must remain navigable as features accumulate. Multiple Go hexagonal architecture references (iruldev/golang-api-hexagonal, AngusGMorrison/realworld-go, bxcodec/golang-ddd-modular-monolith) converge on `primary/secondary` naming over `driving/driven`.

## Decision

Adopt hexagonal architecture with `primary/secondary` adapter naming:

- Domain layer (`internal/core/domain/`) has zero external dependencies; ports are defined here as interfaces
- Service layer (`internal/core/service/`) contains use cases; each use case is a struct with constructor-injected ports
- Primary adapters (`internal/adapters/primary/`) drive the application (TUI)
- Secondary adapters (`internal/adapters/secondary/`) are driven by the application (storage, config, logger, stubs)
- All wiring happens in `cmd/overseer/main.go` via constructor injection (no DI framework)
- Features are implemented as vertical slices spanning all relevant layers

## Consequences

- Every new feature follows exactly one path: domain → use case → adapter(s)
- The `overseer-feature` skill can mechanically guide feature addition
- No DI framework needed at TUI scale; constructor injection is sufficient
- `pkg/` directory is forbidden; all code lives under `internal/` or `cmd/`
- Per-layer `AGENTS.md` files enforce layer boundaries for AI agents
