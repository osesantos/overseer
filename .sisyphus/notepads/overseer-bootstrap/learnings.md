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

- T9 complete: `internal/adapters/secondary/storage/json/` — JSON-backed session.Repository with atomic tmp+rename writes, corruption recovery (rename to `.corrupted.<unix>.json` + warn via slog), full in-memory map cache. Package named `json` requires `encodingjson "encoding/json"` alias in store.go. `session.Session` has no JSON tags — marshals with PascalCase keys; `uuid.UUID` and `time.Time` both implement json.Marshaler so round-trip works. `io.Discard` (not `os.Discard`) for slog test handler. 10 unit tests + 3 integration tests (corruption recovery, 100 concurrent saves, missing parent dirs). Evidence: task-9-persistence.txt, task-9-corruption.txt, task-9-atomic.txt.

- T10 complete: `internal/adapters/secondary/config/yaml/` — YAML config loader with `Config`/`Default()`/`Load()`/`Validate()`. Package named `yaml` requires `yamlv3 "gopkg.in/yaml.v3"` alias in loader.go. Merge-defaults pattern: start with `Default()`, then `yamlv3.Unmarshal(data, &cfg)` to overlay only fields present in YAML. `gopkg.in/yaml.v3` error messages naturally contain "line X" info for parse errors. `Validate()` exported as method on `Config` for reuse. 8 unit tests covering: defaults, missing file, invalid YAML (with line info), partial YAML (defaults filled), invalid focusOnStart, valid full config, invalid minWidth, all valid focus values. Evidence: task-10-defaults.txt, task-10-invalid.txt.

- T11 complete: `internal/adapters/secondary/logger/slog/` — slog JSON logger wired to XDG log file. Package named `slog` requires `stdslog "log/slog"` alias (same trick as T9's `encodingjson` and T10's `yamlv3`). `OVERSEER_LOG_LEVEL` env var takes precedence over `level` param; `lvl.UnmarshalText([]byte(level))` parses slog level strings ("debug", "info", "warn", "error") case-insensitively. Tests override `XDG_STATE_HOME` via `t.Setenv` to redirect `paths.LogFile()` to temp dir; `t.Setenv("OVERSEER_LOG_LEVEL", "")` clears any ambient env in T1. Evidence: task-11-json-log.txt, task-11-level-env.txt.

- T12 complete: `internal/adapters/secondary/{tmux,git,agent}/stub/` — three stub adapters implementing `session.TmuxAdapter`, `session.GitAdapter`, `session.AgentLauncher`. Tmux stub uses `uuid.New().String()[:8]` for deterministic-enough canned IDs. All three packages use `package stub` matching directory name. Each has a `_test.go` in `package stub_test` with compile-time `var _ Interface = (*stub.Stub)(nil)` checks and call-counter assertions. Package doc comment carries the task-required stub disclaimer ("Replace with real implementation when integrating real X"). Evidence: task-12-compile.txt, task-12-recording.txt, task-12-no-todo.txt.

- T13 complete: `internal/core/service/session/CreateUseCase` follows service package aliasing with `domainsession` to avoid name collision. Create validates via domain factory before mocks/ports, computes per-project max order + 1, checks duplicate name+project before side effects, then calls tmux, git, and repo in order. Handwritten port mocks live in `internal/testutil/mocks` with call counters and canned errors/results. Evidence: task-13-happy.txt, task-13-duplicate.txt, task-13-order.txt.

- T14 complete: `internal/core/service/session/RenameUseCase` — follows same `domainsession` alias pattern as T13. Logic: Get → List (duplicate check excluding self by ID) → domain `s.Rename()` → Save. Duplicate check excludes current session via `candidate.ID == s.ID` guard. Empty name is caught by `s.Rename()` (domain validates), not pre-checked in service. `TestRenameUseCase_UpdatedAtChanges` pins `original.UpdatedAt` to `time.Now().Add(-time.Minute)` before calling Execute, then asserts `SavedSession.UpdatedAt.After(beforeRename)`. Evidence: task-14-happy.txt, task-14-empty.txt.

- T16 complete: `internal/core/service/session/list.go` — ListUseCase with no logger (read-only, no side effects). Service package `session` collides with domain `session` — use `domainsession` alias. Group-by using `map[string]*SessionGroup` then flatten + sort. `sort.Slice` on groups by ProjectName ASC; on sessions within each group by Order ASC. `ListRequest{}` is empty struct (no params for bootstrap). 4 tests: empty repo, single project sorted, multi-project sorted by name, non-sequential orders. Evidence: task-16-grouping.txt.

- T15 complete: `internal/core/service/session/ReorderUseCase` — `ErrNoOp` added to `internal/shared/errs/errs.go`. ReorderUseCase fetches target via `repo.Get`, filters+sorts project sessions by Order ASC, checks single-session and boundary conditions (both return `errs.ErrNoOp`), swaps `.Order` fields in-place, saves both changed sessions via `repo.Save`, re-sorts, and returns `ReorderResponse{Sessions: projectSessions}`. Service package uses `domainsession` alias (same pattern as T13). 6 tests covering MoveDown, MoveUp, BoundaryFirst_Up, BoundaryLast_Down, SingleSession, NotFound. Evidence: task-15-move-down.txt, task-15-boundary.txt.

- T19 complete: `internal/adapters/primary/tui/preview/` — PreviewPane BubbleTea sub-model with viewport + focused/blurred border. Added `github.com/charmbracelet/bubbles@v0.21.1-0.20250623103423-23b8fd6302d7` as direct dep. Golden file generated with `go test -update` flag (xgolden stores at `testdata/{TestName}.golden`). `Focused()` getter added alongside `SetFocus()` pointer receiver. `viewport.HighPerformanceRendering=false` required for test-friendly rendering (no alternate screen). 3 tests: Default (golden), FocusedBorder (diff check), SetFocus (state check). Evidence: task-19-tests.txt.

- T18 complete: `internal/adapters/primary/tui/status/bar.go` — BubbleTea sub-model for top-right status bar. Uses `github.com/charmbracelet/bubbletea` (v1) NOT `charm.land/bubbletea/v2`; v2 is only used by the teatest harness. Tests use `package status` (internal) to set unexported `workdir`/`width` fields for deterministic golden output. Golden file at `testdata/TestStatus_Default.golden` contains `/home/user/projects | stubbed | — | idle`. Truncation uses rune-based loop with `lipgloss.Width()` to measure display columns. `Separator.Render()` (zero args) uses pre-set `SetString(" | ")` value. Evidence: task-18-tests.txt, task-18-commit.txt.

- T21 complete: `internal/adapters/primary/tui/session/` package created with `messages.go`, `rename_form.go`, and `rename_form_test.go`. Used `github.com/charmbracelet/bubbles@v1.0.0` (exact match for `bubbletea v1.3.10` — both packages align on the same direct dep). bubbletea v1.3.10 keeps the v0.x-compatible KeyMsg API: `tea.KeyMsg{Type: tea.KeyEnter}`, `tea.KeyMsg{Type: tea.KeyEsc}`, `tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}`. Tests call `Update()` directly (no teatest harness); cmd is verified by calling `cmd()` to extract the emitted Msg. `textinput.Model.SetValue()` uses pointer receiver but Go auto-takes address of struct field — safe to call on value-typed local var before passing copy to Update. Evidence: task-21-rename-form-tests.txt.
