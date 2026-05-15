# AGENTS.md — Secondary Adapters

## MUST

- MUST: Each adapter implements a port defined in `internal/core/domain/`
- MUST: Adapters MAY import 3rd-party libs (yaml, slog, etc.); MUST translate library errors to domain errors at the boundary
- MUST: Stub adapters provide canned responses, not `// TODO panic`; they MUST satisfy the full interface
- MUST: Integration tests use the `//go:build integration` tag

## MUST NOT

- MUST NOT: Import `service/` or `adapters/primary/`
- MUST NOT: Leak library types out of the package (return domain types only)
- MUST NOT: Cache state across process lifetime — single-instance only
