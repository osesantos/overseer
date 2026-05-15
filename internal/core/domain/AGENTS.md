# AGENTS.md — Domain Layer

## MUST

- MUST: Use pure Go only — only stdlib imports allowed (plus `github.com/google/uuid` for IDs)
- MUST: Define ports as interfaces in this package
- MUST: Define domain errors as exported sentinel errors using `errors.New`
- MUST: Validate inputs in constructors / factory functions (return error, don't panic)

## MUST NOT

- MUST NOT: Import any other internal package (`adapters/`, `service/`, etc.)
- MUST NOT: Import any framework (BubbleTea, Lipgloss, viper, etc.)
- MUST NOT: Perform I/O — no `os`, no `net/http`, no file ops
- MUST NOT: Define mock types in this package (mocks live in `internal/testutil/mocks`)
