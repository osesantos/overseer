# ADR 0004: TDD with teatest for TUI Layer

## Status

Accepted

## Context

Overseer's extension process must be mechanical and verifiable. The `overseer-feature` skill must produce features that are testable end-to-end. TUI testing is notoriously difficult; `github.com/charmbracelet/x/exp/teatest/v2` is the canonical solution for BubbleTea applications, used by production apps (glow, gum). Golden files provide regression protection for rendered output.

## Decision

Adopt TDD discipline across all layers with teatest for the TUI layer:

- **TDD order**: write failing test (RED) → minimal implementation (GREEN) → refactor
- **Domain layer**: pure unit tests, no mocks, no external deps
- **Service layer**: unit tests with handwritten mocked ports from `internal/testutil/mocks`
- **Secondary adapters**: integration tests with `//go:build integration` tag
- **TUI layer**: `teatest.NewTestModel` with fixed terminal size; golden files via `teatest.RequireEqualOutput`
- ANSI codes stripped in golden files via `lipgloss.SetColorProfile(termenv.Ascii)` in `internal/testutil/golden.Setup`
- Golden files regenerated via `make update-golden`
- Coverage targets: domain 90%+, service 80%+, TUI 40-60%, overall 60-70%

## Consequences

- Every feature has tests at every layer before the feature is considered done
- Golden files catch visual regressions automatically
- `make test` and `make test-integration` are the verification gates
- Handwritten mocks (no mockery/gomock) keep the dependency graph clean
- The `overseer-feature` skill includes test templates for each layer
