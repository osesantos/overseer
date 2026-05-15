# AGENTS.md — Service Layer (Use Cases)

## MUST

- MUST: Each use case is a struct with constructor-injected ports
- MUST: Each use case exposes a single `Execute(ctx, req) (resp, error)` method
- MUST: Validate the request DTO before doing work; return domain errors
- MUST: Tests use mocked ports from `internal/testutil/mocks`

## MUST NOT

- MUST NOT: Import `adapters/` packages (only `domain/` + stdlib)
- MUST NOT: Mutate input request structs
- MUST NOT: Combine multiple use cases into one struct (one struct per use case)
