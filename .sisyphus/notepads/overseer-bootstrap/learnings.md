# Learnings — overseer-bootstrap

## [2026-05-15] Session Start
- Plan: 30 implementation tasks + 4 final review tasks
- Module path: `github.com/dnlopes/overseer`
- Go version: 1.22
- Architecture: Hexagonal with primary/secondary naming
- Config: YAML only
- Test strategy: TDD with teatest
- Persistence: JSON file with atomic writes (tmp + rename)
- XDG paths: data/config/state dirs
- No DI framework (constructor injection only)
- No `pkg/` directory
- Logging: slog to file only (never stderr in TUI mode)
- Stub mode: tmux/git/agent adapters are stubbed with canned responses
- Scaffolded root Go module with Go 1.22/toolchain 1.22.0, bootstrap main, and empty hexagonal directories using .gitkeep files.
- Evidence captured for build, vet, no-pkg check, and directory tree under .sisyphus/evidence/.
- Added root Makefile with help-driven PHONY targets for build/test/lint/run/clean/tidy, and added `coverage.out` to .gitignore.
- Verified `make help`, `make build`, `make test`, `make`, and `make clean`; saved evidence under `.sisyphus/evidence/`.
- Added `internal/shared/paths` with XDG-aware `DataDir`, `ConfigDir`, `StateDir`, file helpers, `EnsureDir`, and atomic temp-file rename writes.
- Added `internal/shared/errs` with stdlib sentinel errors plus thin `Wrap`/`Is` helpers.
- Verified shared package tests, XDG override behavior, and atomic write behavior; saved coverage and targeted test evidence under `.sisyphus/evidence/`.
- Added `internal/testutil/golden` with ANSI-stripping setup and byte reader helper; verified output stripping via test.
- Added `internal/testutil/teatest` harness wrapper around `teatest.NewTestModel` with fixed terminal sizing and golden setup; test uses a minimal dummy model.
- Added handwritten mock template docs and a session fixture placeholder for future T8 Session type.
- `go get` pulled the teatest dependency stack; compatibility required bumping transitive `x/ansi` and `x/cellbuf` via the module graph.
- Verified `go test -v ./internal/testutil/...` and saved harness/tree evidence under `.sisyphus/evidence/task-4-*.txt`.
- T5 complete: `internal/adapters/primary/tui/styles/styles.go` with 20 named styles in 9 nested structs. All styles returned from `New() *Styles` — zero package-level vars. `lipgloss.SetColorProfile(termenv.Ascii)` in TestMain strips escape sequences cleanly; HiddenBorder vs RoundedBorder renders differ even in ASCII mode (space-only vs box-drawing chars). `SetString(" | ")` on Separator pre-sets content for zero-arg `Render()` calls; `Render("x")` still works as expected (overrides). Evidence: task-5-borders-differ.txt, task-5-no-globals.txt.

- T8 complete: added `internal/core/domain/session` with pure domain Session entity, sentinel errors, and domain-owned ports; `New`/`Rename` trim and enforce 100-character name/project limits. Added `github.com/google/uuid` for domain IDs and replaced session fixture placeholder with `MakeSession` helper. Evidence: task-8-new-validation.txt, task-8-imports-clean.txt.
