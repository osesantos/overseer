# Overseer Bootstrap Plan

## TL;DR

> **Quick Summary**: Bootstrap a Go TUI app called Overseer following strict hexagonal architecture (primary/secondary adapters), with a 3-pane dashboard, three end-to-end features (Create/Rename/Reorder session) backed by stubbed tmux/git/agent adapters, a comprehensive TDD testing framework using teatest, and a formalized feature-extension process via `AGENTS.md` hierarchy plus a custom `overseer-feature` Claude skill.
>
> **Deliverables**:
> - Go project skeleton (`go.mod`, `Makefile`, `.golangci.yml`, XDG paths)
> - Dashboard view: sessions list (left) + status bar (top-right) + preview pane (bottom-right)
> - 3 features end-to-end: Create, Rename, Reorder session
> - Stub adapters for tmux/git/agent (real interfaces, canned responses)
> - Testing framework: TDD across 4 layers (domain/service/adapter/TUI) with teatest + golden files
> - Standardization artifacts: root `AGENTS.md`, 4 per-layer `AGENTS.md` files, `overseer-feature` skill (with Feature Shape Catalog + worked Delete example), 4 ADRs
>
> **Estimated Effort**: Large (~30 tasks across 7 waves + 4 final reviews)
> **Parallel Execution**: YES — 7 waves, max 7 concurrent tasks per wave
> **Critical Path**: Wave 1 foundation → Wave 2 domain/storage → Wave 3 use cases → Wave 4 TUI components → Wave 5 integration → Wave 6 skill → Wave 7 docs → Final reviews

---

## Context

### Original Request

David wants to start a TUI application "Overseer" for managing AI agent sessions. The full vision is large (real tmux/git integration, remote mode, diff view, etc.) but the bootstrap delivers 5 things:

1. Go project setup from scratch
2. Main dashboard view
3. 2-3 end-to-end features
4. Formalized extension process via Claude rules/skills
5. Testing framework

David explicitly stated: **standardization > YAGNI**. Code is cheap; the process to extend Overseer must be a walk in the park. Architect for extensibility.

### Interview Summary

**David's Decisions (confirmed)**:

| Question | Decision |
|---|---|
| Go module path | `github.com/dnlopes/overseer` |
| MVP session depth | **Stub mode**: JSON records only, no real tmux/git/worktree yet |
| Bootstrap features | **Create + Rename + Reorder** session |
| Config format | **YAML** |
| Test strategy | **TDD with teatest** (RED → GREEN → REFACTOR) |

### Research Findings (3 background librarian agents)

- **Hexagonal Go consensus** (from iruldev/golang-api-hexagonal 1.2k★, AngusGMorrison/realworld-go 2.5k★, bxcodec/golang-ddd-modular-monolith 1.8k★): `primary/secondary` naming (not `driving/driven`), ports in domain package, constructor injection (no DI framework for TUI scale), vertical slices by feature.
- **BubbleTea testing**: `github.com/charmbracelet/x/exp/teatest/v2` is canonical. Golden files via `teatest.RequireEqualOutput`. Production apps (glow, gum) test core logic exhaustively, integration tests minimally. Coverage targets: domain 90%+, Update 80-90%, View 40-60%, overall realistic 60-70%.
- **Dashboard layout (charmbracelet/soft-serve)**: 3-pane via nested `lipgloss.JoinVertical/JoinHorizontal`, focus management via enum (`type pane int`), keyboard routed only to active pane, focused-border vs hidden-border visual indicator. `viewport.Model` with `HighPerformanceRendering=true` for streaming preview (stubbed in bootstrap).

### Metis Review

**Identified Gaps (all addressed in this plan)**:
- **Project model**: Resolved → `ProjectName string` field on Session, no separate Project entity
- **XDG paths**: Resolved → data/config/logs use XDG dirs (not `~/.overseer/`)
- **Atomic JSON writes**: Required from day 1 (`tmp + rename` pattern)
- **`bubbles/help.Model` integration**: Required from day 1 (each feature registers its keybindings)
- **Empty/error states**: Spec'd for missing JSON, corrupted JSON, missing YAML, invalid YAML, zero sessions, terminal-too-small
- **Status row stub values**: Spec'd (`dir: <cwd>`, `branch: stubbed`, `pr: —`, `agent: idle`)
- **Reorder keys**: J/K (shift+j/k) within-group only, silent no-op at boundaries
- **Group expand/collapse**: Deferred (all groups always expanded — highest-value YAGNI cut)
- **Logging destination**: Required to be a log file (never stderr) — `$XDG_STATE_HOME/overseer/overseer.log`
- **Feature Shape Catalog**: Required in skill — 5 shapes (form-driven, inline-edit, direct-action, async/streaming, read-only), 3 exercised + 2 documented
- **Skill self-test**: Skill must produce a working `Delete` feature on first try (post-bootstrap acid test)
- **ADR scope cap**: 4 ADRs only (Hexagonal, Stub-mode, JSON-write-through, TDD-teatest)

---

## Work Objectives

### Core Objective

Establish the foundation, conventions, and end-to-end vertical slices that make every subsequent Overseer feature follow exactly one well-tested path from domain → use case → adapter → TUI, validated by a Claude skill that can mechanically produce new features.

### Concrete Deliverables

- `go.mod` at module path `github.com/dnlopes/overseer`, Go 1.22 toolchain
- `Makefile` with targets: `build`, `test`, `test-integration`, `update-golden`, `lint`, `run`, `clean`
- `.golangci.yml` with conservative defaults
- `.gitignore`, `.gitattributes` (with `*.golden linguist-generated=true -text`)
- Directory layout: `cmd/overseer/`, `internal/core/{domain,service}/session/`, `internal/adapters/{primary/tui,secondary/{storage,config,logger,tmux,git,agent}}/`, `internal/shared/`, `internal/testutil/{mocks,fixtures}/`, `docs/{architecture.md,adr/}`, `testdata/`
- Working `overseer` binary that launches the dashboard
- Dashboard with 3 panes, focus management, help bar, empty state, terminal-too-small fallback
- 3 end-to-end features (Create / Rename / Reorder) — each exercises full hexagonal path
- 5 stub adapters (storage real; tmux/git/agent stubbed with canned responses)
- Test suite: domain (pure), service (mocked ports), adapter (`//go:build integration`), TUI (teatest + golden files)
- Root `AGENTS.md` (high-level architecture + MUST/MUST NOT rules)
- 4 per-layer `AGENTS.md` files (`internal/core/domain/`, `internal/core/service/`, `internal/adapters/primary/`, `internal/adapters/secondary/`)
- `.claude/skills/overseer-feature/` skill (SKILL.md + Feature Shape Catalog + worked Delete example + README + VERSION + CHANGELOG)
- 4 ADRs in `docs/adr/`

### Definition of Done

- [ ] `make build` produces a working `overseer` binary
- [ ] `make test` passes (zero failures, zero skips)
- [ ] `make test-integration` passes
- [ ] `make lint` passes (zero issues)
- [ ] Running `overseer` launches the dashboard; user can press `n` to create a session, `r` to rename, `J`/`K` to reorder, `Tab` to switch focus, `q` to quit
- [ ] JSON file at `$XDG_DATA_HOME/overseer/data.json` is created/updated correctly
- [ ] Help bar shows all keybindings for the focused pane
- [ ] All 5 teatest scenarios green (Create / Rename / Reorder / Empty / Terminal-too-small)
- [ ] `overseer-feature` skill's worked example (Delete) can be executed by an agent producing green tests + lint

### Must Have

- Hexagonal architecture with `primary/secondary` naming
- Constructor injection (NO DI framework)
- Vertical slices by feature
- Stub adapters as real interface impls with canned responses (NOT TODO placeholders)
- Atomic JSON writes (`tmp + rename`)
- XDG-compliant paths
- `bubbles/help` integration from day 1
- TDD with teatest
- 4 ADRs (Hexagonal+primary/secondary, Stub-mode-for-bootstrap, JSON-write-through, TDD-with-teatest)
- Root + 4 per-layer `AGENTS.md` files, each with explicit MUST / MUST NOT rules
- `overseer-feature` skill with Feature Shape Catalog (5 shapes) and worked Delete example

### Must NOT Have (Guardrails)

- ❌ Real tmux / git / agent integration (all stubbed)
- ❌ A 4th feature (Delete, Detail, Settings, etc.) — Delete is the post-bootstrap acid test, NOT in this plan
- ❌ Ad-hoc groups (only project-based grouping; project is a string field, not entity)
- ❌ Group expand/collapse (all groups always expanded)
- ❌ DI framework (`fx`, `wire`, `dig`)
- ❌ `pkg/` directory
- ❌ TOML or JSON for config (YAML only)
- ❌ Logging to stderr (always to log file)
- ❌ Drag-and-drop / mouse support
- ❌ "Reorder mode" with explicit enter/exit
- ❌ Form base type or generic UI inheritance
- ❌ Mock framework / factory (handwritten mocks per port)
- ❌ Plugin loader / extension-point system
- ❌ Schema migration code (only `SchemaVersion` field present)
- ❌ Hot-reload / config watchers / pub-sub
- ❌ Observability / metrics / traces (slog is the only telemetry)
- ❌ `--version` flag / extra CLI subcommands
- ❌ ADR sprawl (cap at 4)
- ❌ Animation / splash screens / "polish" beyond minimum viable visuals
- ❌ UUIDs shown in TUI (hidden in UI; present in JSON + logs only)
- ❌ Generic prose in `AGENTS.md` without explicit MUST / MUST NOT rules
- ❌ File locking / multi-instance coordination (document last-writer-wins assumption only)
- ❌ Trivial getter/setter/constructor tests
- ❌ ANSI codes in golden files (use `termenv.Ascii` profile in tests)

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** — all verification is agent-executed. No "user manually confirms" criteria.

### Test Decision
- **Infrastructure exists**: NO (greenfield)
- **Automated tests**: YES (TDD)
- **Framework**: stdlib `go test` + `github.com/charmbracelet/x/exp/teatest/v2`
- **TDD discipline**: Each implementation task explicitly follows RED (failing test) → GREEN (minimal impl) → REFACTOR

### QA Policy

Every implementation task MUST include agent-executed QA scenarios using:

- **TUI/Dashboard**: `teatest.NewTestModel` with fixed terminal size; assert via golden files + final-model state inspection
- **CLI/binary**: `interactive_bash` (tmux) — run `make build && ./overseer`, send keystrokes, validate output, then `Ctrl+C` to exit
- **Files (JSON, YAML, golden)**: `Bash` (jq + diff) — assert file presence, parse-and-assert specific fields
- **Library/use case**: `Bash` (`go test -run <Name>`) — run the unit test, assert PASS

Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

Golden files in `testdata/` are validated by `teatest.RequireEqualOutput`; regenerated via `make update-golden`.

### Test Layering (hexagonal-aligned)

| Layer | Test Type | Location | Build Tag |
|---|---|---|---|
| Domain | Pure unit (no mocks) | `internal/core/domain/{feature}/*_test.go` | (none) |
| Use case (service) | Unit with mocked ports | `internal/core/service/{feature}/*_test.go` | (none) |
| Adapter | Integration (real impl) | `internal/adapters/secondary/{kind}/*_test.go` | `//go:build integration` |
| TUI | teatest + golden files | `internal/adapters/primary/tui/**/*_test.go` | (none) |
| End-to-end | teatest full dashboard flow | `internal/adapters/primary/tui/dashboard_e2e_test.go` | (none) |

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 — Foundation (7 tasks, all parallel, no internal deps):
├── T1: Go project scaffolding (go.mod, .gitignore, .gitattributes, .golangci.yml) [quick]
├── T2: Makefile with all targets [quick]
├── T3: internal/shared package (XDG paths, error types) [quick]
├── T4: internal/testutil base (golden helper, termenv config, mock template) [quick]
├── T5: Lipgloss styles registry (internal/adapters/primary/tui/styles) [visual-engineering]
├── T6: Root AGENTS.md + 4 per-layer AGENTS.md files [writing]
└── T7: 4 ADRs in docs/adr/ [writing]

Wave 2 — Domain + Driven Adapters (5 tasks, parallel, depends on Wave 1):
├── T8: Session domain (entity, errors, ports) [deep]
├── T9: JSON storage adapter (atomic writes, corruption recovery, integration tests) [unspecified-high]
├── T10: YAML config loader adapter (validation, missing/invalid handling) [unspecified-high]
├── T11: Logger adapter (slog wired to log file, XDG paths) [quick]
└── T12: Stub adapters: tmux + git + agent (canned responses, all 3 in one task) [quick]

Wave 3 — Use Cases (4 tasks, parallel, depends on Wave 2):
├── T13: CreateSession use case + tests [deep]
├── T14: RenameSession use case + tests [deep]
├── T15: ReorderSession use case + tests [deep]
└── T16: ListSessions use case + tests [quick]

Wave 4 — TUI Components (6 tasks, parallel, depends on Wave 3):
├── T17: SessionsList component (left pane, grouped rendering) [visual-engineering]
├── T18: StatusBar component (top-right, stub values) [visual-engineering]
├── T19: PreviewPane component (bottom-right, stub content) [visual-engineering]
├── T20: CreateSessionForm (modal with textinput) [visual-engineering]
├── T21: RenameSessionForm (modal with textinput) [visual-engineering]
└── T22: Help bar integration (bubbles/help with feature keybindings registry) [visual-engineering]

Wave 5 — Integration (2 tasks, sequential, depends on Wave 4):
├── T23: Dashboard composition (focus enum, keyboard routing, pane assembly, terminal-too-small) [deep]
└── T24: cmd/overseer/main.go (composition root, DI wiring, XDG setup, corruption recovery, startup flow) [deep]

Wave 6 — Standardization Artifacts (3 tasks, parallel, depends on Wave 5):
├── T25: overseer-feature skill structure + SKILL.md + Feature Shape Catalog [writing]
├── T26: Worked example: Delete feature walkthrough in skill [writing]
└── T27: Skill self-test script + README + VERSION + CHANGELOG [writing]

Wave 7 — Final Integration & Docs (3 tasks, parallel, depends on Wave 6):
├── T28: End-to-end teatest scenarios (full dashboard flow: Create → Rename → Reorder → Quit) [unspecified-high]
├── T29: docs/architecture.md (architecture overview with diagrams) [writing]
└── T30: README.md at project root [writing]

Wave FINAL — Review (4 parallel reviews, then user okay):
├── F1: Plan compliance audit (oracle) — every MUST + MUST NOT verified
├── F2: Code quality review (unspecified-high) — build/lint/test/AI-slop scan
├── F3: Real manual QA (unspecified-high) — execute every QA scenario, capture evidence
└── F4: Scope fidelity check (deep) — diff vs spec, no contamination, no creep
→ Present results → Get explicit David okay

Critical Path: T1 → T8 → T13 → T17 → T23 → T24 → T25 → T28 → F1-F4 → user okay
Parallel Speedup: ~75% faster than sequential
Max Concurrent: 7 (Wave 1)
```

### Dependency Matrix

| Task | Depends On | Blocks |
|---|---|---|
| T1-T7 (Wave 1) | — | T8-T12 |
| T8 (Session domain) | T1, T3, T4 | T9, T13-T16 |
| T9 (JSON storage) | T1, T3, T4, T8 | T24 |
| T10 (YAML config) | T1, T3, T4 | T24 |
| T11 (Logger) | T1, T3 | T24 |
| T12 (Stub adapters) | T1, T8 | T13, T15 |
| T13 (CreateSession) | T8, T9, T12 | T20, T23 |
| T14 (RenameSession) | T8, T9 | T21, T23 |
| T15 (ReorderSession) | T8, T9, T12 | T17, T23 |
| T16 (ListSessions) | T8, T9 | T17, T23 |
| T17 (SessionsList) | T5, T16 | T23 |
| T18 (StatusBar) | T5 | T23 |
| T19 (PreviewPane) | T5 | T23 |
| T20 (CreateForm) | T5, T13 | T23 |
| T21 (RenameForm) | T5, T14 | T23 |
| T22 (Help bar) | T5 | T23 |
| T23 (Dashboard) | T17-T22 | T24, T28 |
| T24 (main.go) | T9, T10, T11, T23 | T28, T30 |
| T25-T27 (Skill) | T23 | T28 |
| T28 (E2E tests) | T23, T24, T25-T27 | F-Wave |
| T29-T30 (Docs) | T24, T25-T27 | F-Wave |
| F1-F4 (Reviews) | All tasks | User okay |

### Agent Dispatch Summary

- **Wave 1**: T1-T4 → `quick`, T5 → `visual-engineering`, T6-T7 → `writing`
- **Wave 2**: T8 → `deep`, T9-T10 → `unspecified-high`, T11-T12 → `quick`
- **Wave 3**: T13-T15 → `deep`, T16 → `quick`
- **Wave 4**: T17-T22 → `visual-engineering`
- **Wave 5**: T23-T24 → `deep`
- **Wave 6**: T25-T27 → `writing`
- **Wave 7**: T28 → `unspecified-high`, T29-T30 → `writing`
- **FINAL**: F1 → `oracle`, F2-F3 → `unspecified-high`, F4 → `deep`

---

## TODOs

- [x] 1. **Go Project Scaffolding**

  **What to do**:
  - `go mod init github.com/dnlopes/overseer`; set `go 1.22` and `toolchain go1.22.0` directives
  - Create directory tree: `cmd/overseer/`, `internal/core/{domain,service}/`, `internal/adapters/{primary/tui,secondary/{storage,config,logger,tmux,git,agent}}/`, `internal/shared/`, `internal/testutil/{mocks,fixtures}/`, `docs/adr/`, `bin/` (gitignored), `testdata/`
  - Add `.gitignore`: `bin/`, `*.test`, `*.out`, `.DS_Store`, `coverage.*`
  - Add `.gitattributes`: `*.golden linguist-generated=true -text` and `*.go text eol=lf`
  - Add `.golangci.yml`: enable `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`, `gosimple`, `goimports`, `gocyclo` (threshold 15), `revive`; exclude `testdata/`, `internal/testutil/mocks/`
  - Add `.editorconfig`: indent_style=tab for Go, indent_size=2 for yaml/md
  - Add placeholder `main.go` in `cmd/overseer/` that prints `overseer (bootstrap)` and exits 0 (will be rewritten in T24)
  - Verify with `go build ./...` (must succeed)

  **Must NOT do**:
  - ❌ Do NOT create `pkg/` directory
  - ❌ Do NOT install any deps yet beyond what's needed (defer to subsequent tasks)
  - ❌ Do NOT add LICENSE / CHANGELOG / README at root yet (those come in Wave 7)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Mechanical scaffolding, no design decisions
  - **Skills**: none
    - Reason: Pure file/directory creation; no domain expertise needed

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T2-T7)
  - **Blocks**: T8, T9, T10, T11, T12 (all Wave 2 — they need the directory structure)
  - **Blocked By**: None

  **References**:
  - Hexagonal layout consensus: `.sisyphus/drafts/overseer-bootstrap.md` § "Canonical Layout"
  - Go project layout: https://github.com/golang-standards/project-layout (note: do NOT use `pkg/`)
  - `.golangci.yml` defaults: https://golangci-lint.run/usage/configuration/
  - `.gitattributes` for golden files: https://github.com/charmbracelet/x/blob/fe5d686/.gitattributes

  **Acceptance Criteria**:
  - [ ] `go build ./...` → exit 0, no output
  - [ ] `go vet ./...` → exit 0, no output
  - [ ] `find . -name 'AGENTS.md' -not -path './.git/*' | wc -l` → `0` (will be 5 after T6)
  - [ ] `test ! -d pkg/` → exit 0
  - [ ] `head -3 go.mod` contains `module github.com/dnlopes/overseer` and `go 1.22`

  **QA Scenarios**:

  ```
  Scenario: Fresh scaffolding builds
    Tool: Bash
    Preconditions: cwd at repo root, no Go files exist beyond cmd/overseer/main.go
    Steps:
      1. Run: go build ./...
      2. Run: ./cmd/overseer/main.go is built; binary placed at bin/overseer
    Expected Result: Exit 0; bin/overseer file exists; running it prints "overseer (bootstrap)" then exits 0
    Evidence: .sisyphus/evidence/task-1-build-succeeds.txt

  Scenario: Linter clean on scaffold
    Tool: Bash
    Preconditions: golangci-lint installed (or via `go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest`)
    Steps:
      1. Run: golangci-lint run ./...
    Expected Result: Exit 0; "0 issues"
    Evidence: .sisyphus/evidence/task-1-lint-clean.txt

  Scenario: Forbidden directories absent
    Tool: Bash
    Preconditions: scaffolding complete
    Steps:
      1. Run: test ! -d pkg/ && echo OK
    Expected Result: "OK" printed; exit 0
    Failure Indicators: pkg/ directory exists → fail
    Evidence: .sisyphus/evidence/task-1-no-pkg.txt
  ```

  **Evidence to Capture**:
  - [ ] task-1-build-succeeds.txt (terminal output)
  - [ ] task-1-lint-clean.txt
  - [ ] task-1-no-pkg.txt
  - [ ] task-1-directory-tree.txt (output of `tree -L 3 -I '.git'`)

  **Commit**: YES
  - Message: `chore(scaffold): initialize Go project with .gitignore .gitattributes .golangci.yml`
  - Files: `go.mod`, `.gitignore`, `.gitattributes`, `.golangci.yml`, `.editorconfig`, `cmd/overseer/main.go`, directory stubs
  - Pre-commit: `go build ./... && golangci-lint run ./...`

- [x] 2. **Makefile with All Targets**

  **What to do**:
  - Create `Makefile` at repo root with these PHONY targets:
    - `build`: `go build -o bin/overseer ./cmd/overseer/`
    - `test`: `go test -race -cover ./...` (excludes integration via build tag absence)
    - `test-integration`: `go test -race -tags=integration ./...`
    - `update-golden`: `go test -update ./...` (regenerates golden files)
    - `lint`: `golangci-lint run ./...`
    - `run`: `make build && ./bin/overseer`
    - `clean`: `rm -rf bin/ coverage.* && go clean -testcache`
    - `tidy`: `go mod tidy`
    - `help`: print all targets with descriptions
  - First line: `.DEFAULT_GOAL := help`
  - Each target has a comment after `:` like `## Build the overseer binary` so `help` can auto-extract
  - Add `coverage.out` to `.gitignore` if not already there

  **Must NOT do**:
  - ❌ Do NOT add release / package targets (out of scope)
  - ❌ Do NOT add Docker / docker-compose targets (out of scope)
  - ❌ Do NOT add a `pre-commit` or `install-hooks` target (out of scope)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Single-file Makefile with standard targets
  - **Skills**: none

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T3-T7)
  - **Blocks**: None directly; T28 uses targets
  - **Blocked By**: T1 (needs `cmd/overseer/main.go` to exist for `make build`)

  **References**:
  - Standard Go Makefile patterns: https://github.com/charmbracelet/glow/blob/main/Makefile (reference for structure)
  - Auto-help pattern: https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
  - teatest `-update` flag: https://github.com/charmbracelet/x/blob/fe5d686/exp/teatest/

  **Acceptance Criteria**:
  - [ ] `make help` lists all 9 targets with descriptions
  - [ ] `make build` produces `bin/overseer` (exit 0)
  - [ ] `make test` exit 0 (zero tests yet, that's fine)
  - [ ] `make lint` exit 0
  - [ ] `make clean` removes `bin/` and `coverage.*`
  - [ ] `make` (no args) prints help

  **QA Scenarios**:

  ```
  Scenario: All make targets enumerated and runnable
    Tool: Bash
    Preconditions: T1 complete
    Steps:
      1. Run: make help
      2. Verify output contains: build, test, test-integration, update-golden, lint, run, clean, tidy, help
      3. Run: make build
      4. Verify bin/overseer exists
      5. Run: make clean
      6. Verify bin/ removed
    Expected Result: All steps exit 0; all 9 targets listed in help; clean removes artifacts
    Evidence: .sisyphus/evidence/task-2-make-help.txt and task-2-make-cycle.txt

  Scenario: Build via make produces working binary
    Tool: Bash
    Preconditions: T1 complete
    Steps:
      1. Run: make build
      2. Run: ./bin/overseer
    Expected Result: Binary prints "overseer (bootstrap)" and exits 0
    Evidence: .sisyphus/evidence/task-2-binary-runs.txt
  ```

  **Evidence to Capture**:
  - [ ] task-2-make-help.txt
  - [ ] task-2-make-cycle.txt (build → run → clean)

  **Commit**: YES
  - Message: `chore(build): add Makefile with build/test/lint/run targets`
  - Files: `Makefile`
  - Pre-commit: `make build && make lint`

- [x] 3. **Shared Package: XDG Paths + Error Types**

  **What to do**:
  - Create `internal/shared/paths/paths.go`:
    - `DataDir() string` → `$XDG_DATA_HOME/overseer` or `$HOME/.local/share/overseer`
    - `ConfigDir() string` → `$XDG_CONFIG_HOME/overseer` or `$HOME/.config/overseer`
    - `StateDir() string` → `$XDG_STATE_HOME/overseer` or `$HOME/.local/state/overseer`
    - `DataFile() string` → `DataDir()/data.json`
    - `ConfigFile() string` → `ConfigDir()/config.yaml`
    - `LogFile() string` → `StateDir()/overseer.log`
    - `EnsureDir(dir string) error` → mkdir -p, 0o755
    - `AtomicWrite(path string, data []byte) error` → write to `path + ".tmp"` then `os.Rename`
  - Create `internal/shared/errs/errs.go`:
    - `ErrNotFound`, `ErrAlreadyExists`, `ErrInvalidInput`, `ErrCorruptedData` sentinel errors
    - `Wrap(err error, msg string) error` using `fmt.Errorf("%s: %w", ...)`
    - `Is(err, target error) bool` thin wrapper around `errors.Is`
  - Add unit tests `paths_test.go`, `errs_test.go`:
    - paths: verify XDG env override + fallback paths; verify `EnsureDir` creates nested dirs; verify `AtomicWrite` writes-then-renames (use t.TempDir())
    - errs: verify wrap-and-unwrap; verify Is finds sentinel through chains
  - **TDD order**: write tests first (RED), then impl (GREEN), then refactor

  **Must NOT do**:
  - ❌ Do NOT use `~/.overseer/` (XDG only)
  - ❌ Do NOT create directories at package-init time — only inside `EnsureDir`
  - ❌ Do NOT panic on missing HOME — return error
  - ❌ Do NOT cache paths in package globals

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Small, well-bounded utility package
  - **Skills**: [`programming-skills:golang-dev-guidelines`]
    - Reason: Enforce idiomatic Go conventions and project Go style

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1, T2, T4-T7)
  - **Blocks**: T8, T9, T10, T11 (they all need paths)
  - **Blocked By**: T1 (needs directory tree)

  **References**:
  - XDG spec: https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html
  - Atomic write pattern: https://github.com/google/renameio (note: don't use the library, just copy the pattern — 5 lines)
  - Error sentinels: https://pkg.go.dev/errors

  **Acceptance Criteria**:
  - [ ] `go test ./internal/shared/...` exit 0 with ≥ 6 tests
  - [ ] `paths.DataDir()` returns correct path under `$XDG_DATA_HOME=/tmp/xdg` env
  - [ ] `paths.AtomicWrite(...)` produces final file even after simulated mid-write failure (best-effort assertion: tmp file removed)
  - [ ] `errors.Is(errs.Wrap(errs.ErrNotFound, "x"), errs.ErrNotFound)` returns `true`

  **QA Scenarios**:

  ```
  Scenario: XDG override works
    Tool: Bash
    Preconditions: paths package implemented
    Steps:
      1. Run: XDG_DATA_HOME=/tmp/xdg-test go test -run TestDataDir_XDGOverride ./internal/shared/paths -v
    Expected Result: Test PASS; output contains "/tmp/xdg-test/overseer"
    Evidence: .sisyphus/evidence/task-3-xdg-override.txt

  Scenario: AtomicWrite uses tmp + rename
    Tool: Bash
    Preconditions: paths package implemented
    Steps:
      1. Run: go test -run TestAtomicWrite -v ./internal/shared/paths
    Expected Result: Test PASS; verifies final file exists with correct content, no tmp leftover
    Evidence: .sisyphus/evidence/task-3-atomic-write.txt

  Scenario: errors.Is unwraps correctly
    Tool: Bash
    Preconditions: errs package implemented
    Steps:
      1. Run: go test -run TestWrap_Is -v ./internal/shared/errs
    Expected Result: PASS
    Evidence: .sisyphus/evidence/task-3-errs-wrap.txt
  ```

  **Evidence to Capture**:
  - [ ] task-3-xdg-override.txt
  - [ ] task-3-atomic-write.txt
  - [ ] task-3-errs-wrap.txt
  - [ ] task-3-coverage.txt (output of `go test -cover ./internal/shared/...`)

  **Commit**: YES
  - Message: `feat(shared): add XDG path helpers and error types`
  - Files: `internal/shared/paths/`, `internal/shared/errs/`
  - Pre-commit: `make test && make lint`

- [x] 4. **Test Infrastructure: teatest Helpers + Mock Template**

  **What to do**:
  - Create `internal/testutil/golden/golden.go`:
    - `Setup(t *testing.T)` → sets `lipgloss.SetColorProfile(termenv.Ascii)` to strip ANSI from goldens (deterministic)
    - `ReadBts(tb testing.TB, r io.Reader) []byte` helper for golden file reading
  - Create `internal/testutil/teatest/harness.go`:
    - `NewHarness(t *testing.T, model tea.Model, width, height int) *teatest.TestModel` wrapper that calls golden.Setup + `teatest.NewTestModel` with fixed term size
  - Create `internal/testutil/mocks/template.go` — a documented template comment showing the handwritten mock convention (no logic, just docs):
    ```go
    // Mocks in this package follow this convention:
    //   - File name: <portname>_mock.go (e.g., session_repository_mock.go)
    //   - Type name: Mock<PortName> (e.g., MockSessionRepository)
    //   - Each method records call count + last args in struct fields
    //   - Each method can return canned error via <Method>Err field
    //   - NO test framework dependency (these mocks are usable from any test)
    ```
  - Create `internal/testutil/fixtures/sessions.go`:
    - `MakeSession(name, project string) domain.Session` — only adds the helper, can't reference Session yet so put it as `// Placeholder, will be filled when Session domain exists in T8` (or stub-out with a TODO comment that T8 will resolve — explicitly acceptable here since the type doesn't exist)
  - Add `.gitattributes` entry verification: confirm `*.golden linguist-generated=true -text` line is present (from T1)
  - Add a sample golden test file under `testdata/` directory structure (just create the dir, no files yet)

  **Must NOT do**:
  - ❌ Do NOT introduce a mocking library (mockery, gomock, testify mocks). Handwritten only.
  - ❌ Do NOT add testify or other assertion libs to top-level deps yet (stdlib `t.Errorf` is fine for now; if needed, add later in T8 with explicit justification)
  - ❌ Do NOT cache test models / fixtures in package globals — every test gets a fresh one

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Test infrastructure scaffolding
  - **Skills**: [`programming-skills:golang-dev-guidelines`, `quality:test-on-change`]
    - Reason: Idiomatic Go test setup + project test quality expectations

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1-T3, T5-T7)
  - **Blocks**: T8 onwards (every test uses these helpers)
  - **Blocked By**: T1 (needs directory tree)

  **References**:
  - teatest v2: https://github.com/charmbracelet/x/blob/fe5d686e0c99f12a8f289500e56b75b1a15c6f13/exp/teatest/v2/app_test.go (lines 15-47, 49-78)
  - termenv ANSI stripping: https://github.com/muesli/termenv (Ascii profile)
  - lipgloss SetColorProfile: https://pkg.go.dev/github.com/charmbracelet/lipgloss#SetColorProfile

  **Acceptance Criteria**:
  - [ ] `internal/testutil/golden/`, `internal/testutil/teatest/`, `internal/testutil/mocks/`, `internal/testutil/fixtures/` directories exist
  - [ ] `golden.Setup(t)` callable and sets ASCII profile (verified via unit test)
  - [ ] `teatest.NewHarness(...)` produces a `*teatest.TestModel`
  - [ ] `go vet ./internal/testutil/...` exit 0
  - [ ] `make test ./internal/testutil/...` passes (the meta-test that helpers work)

  **QA Scenarios**:

  ```
  Scenario: golden.Setup strips ANSI
    Tool: Bash
    Preconditions: package implemented
    Steps:
      1. Run: go test -run TestSetup_StripsANSI -v ./internal/testutil/golden
    Expected Result: PASS; renders a styled lipgloss string and asserts no `\x1b` ANSI escapes present
    Evidence: .sisyphus/evidence/task-4-ansi-strip.txt

  Scenario: teatest harness creates TestModel
    Tool: Bash
    Preconditions: package implemented; simple dummy tea.Model in test file
    Steps:
      1. Run: go test -run TestNewHarness -v ./internal/testutil/teatest
    Expected Result: PASS; harness initialized with 80x24 default term size
    Evidence: .sisyphus/evidence/task-4-harness.txt
  ```

  **Evidence to Capture**:
  - [ ] task-4-ansi-strip.txt
  - [ ] task-4-harness.txt
  - [ ] task-4-tree.txt (output of `tree internal/testutil`)

  **Commit**: YES
  - Message: `test(testutil): scaffold teatest helpers, golden file setup, mock template`
  - Files: `internal/testutil/**`
  - Pre-commit: `make test && make lint`

- [x] 5. **Lipgloss Styles Registry**

  **What to do**:
  - Create `internal/adapters/primary/tui/styles/styles.go` with a `Styles` struct exposing named styles:
    - `Border.Focused` — `lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(<focus-color>)`
    - `Border.Blurred` — `lipgloss.NewStyle().Border(lipgloss.HiddenBorder())`
    - `Pane.Sessions`, `Pane.Status`, `Pane.Preview` — pane-specific styles (padding, fg/bg)
    - `Group.Header` — bold + accent color for project name headers
    - `Session.Item.Normal`, `Session.Item.Selected` — list item styles
    - `Status.Label`, `Status.Value`, `Status.Separator` — status row segment styles
    - `Form.Field.Label`, `Form.Field.Input`, `Form.Field.Error` — form modal styles
    - `Help.Key`, `Help.Description`, `Help.Separator` — help bar styles
    - `EmptyState.Title`, `EmptyState.Hint` — empty state styles
    - `TooSmall.Message` — terminal-too-small fallback style
  - `New() *Styles` constructor returning all styles
  - Colors use Lipgloss adaptive colors (light/dark mode aware) where applicable
  - Add unit test `styles_test.go`:
    - assert `New()` returns non-nil
    - assert focused border style differs from blurred
    - assert all named styles produce non-empty render with sample input

  **Must NOT do**:
  - ❌ Do NOT use package-global style variables — return from `New()`
  - ❌ Do NOT define styles inline in component files (must come from registry)
  - ❌ Do NOT add animation / fancy effects
  - ❌ Do NOT use hardcoded width / height in styles (those are set at render time)

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: Visual / UI styling concerns
  - **Skills**: [`programming-skills:golang-dev-guidelines`]
    - Reason: Idiomatic Go style for the registry struct

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with T1-T4, T6, T7)
  - **Blocks**: T17, T18, T19, T20, T21, T22 (all UI components consume styles)
  - **Blocked By**: T1 (directory tree)

  **References**:
  - Lipgloss adaptive colors: https://github.com/charmbracelet/lipgloss#adaptive-colors
  - soft-serve styles inspiration: https://github.com/charmbracelet/soft-serve/blob/16c8e08b7f1bb83ac267be438251fe523aa37dc6/pkg/ui/styles/styles.go (general structure)
  - split-editors border switching: https://github.com/charmbracelet/bubbletea/blob/c60f0c53042238305ec13b486326588f12aea0ec/examples/split-editors/main.go (lines 34-39: focused vs blurred border)

  **Acceptance Criteria**:
  - [ ] `styles.New()` returns `*Styles` with ≥ 15 named styles
  - [ ] `New().Border.Focused.Render("x")` and `New().Border.Blurred.Render("x")` produce visually distinct output
  - [ ] Tests pass under `lipgloss.SetColorProfile(termenv.Ascii)` (used by golden.Setup)
  - [ ] No `var XxxStyle = lipgloss.NewStyle()` at package level (grep check)

  **QA Scenarios**:

  ```
  Scenario: Styles registry returns distinct focused vs blurred
    Tool: Bash
    Preconditions: package implemented
    Steps:
      1. Run: go test -run TestBorder_FocusedVsBlurred -v ./internal/adapters/primary/tui/styles
    Expected Result: PASS; asserts rendered outputs differ
    Evidence: .sisyphus/evidence/task-5-borders-differ.txt

  Scenario: No package-level style vars
    Tool: Bash
    Preconditions: package implemented
    Steps:
      1. Run: grep -nE '^var [A-Za-z]+Style' internal/adapters/primary/tui/styles/*.go
    Expected Result: No output (exit 1 from grep means no matches, which is what we want)
    Failure Indicators: Any match means a package-level style var exists → fail
    Evidence: .sisyphus/evidence/task-5-no-globals.txt
  ```

  **Evidence to Capture**:
  - [ ] task-5-borders-differ.txt
  - [ ] task-5-no-globals.txt
  - [ ] task-5-render-sample.txt (output of a small Go program rendering each named style)

  **Commit**: YES
  - Message: `feat(tui): introduce Lipgloss styles registry`
  - Files: `internal/adapters/primary/tui/styles/`
  - Pre-commit: `make test && make lint`

- [x] 6. **AGENTS.md Hierarchy (Root + 4 Per-Layer)**

  **What to do**: Create 5 `AGENTS.md` files. Each MUST contain explicit MUST / MUST NOT rules — no descriptive prose.

  **Root `AGENTS.md`** (10 rules):
  - MUST: Hexagonal architecture; domain has zero external deps; ports defined in domain
  - MUST: Constructor injection; wire everything in `cmd/overseer/main.go`
  - MUST: Vertical slices — each feature spans `domain/{feat}/`, `service/{feat}/`, `adapters/primary/tui/{feat}/`, optionally `adapters/secondary/.../`
  - MUST: TDD discipline — failing test, then implementation, then refactor; commit per task
  - MUST: All file writes are atomic (`tmp + rename`)
  - MUST: All persistent paths are XDG-compliant via `internal/shared/paths`
  - MUST: All log writes go to the log file (never stderr/stdout in TUI mode)
  - MUST NOT: Add a DI framework (`fx`, `wire`, `dig`)
  - MUST NOT: Use `pkg/` directory
  - MUST NOT: Add a feature without updating its layer's AGENTS.md and the help registry

  **`internal/core/domain/AGENTS.md`** (8 rules):
  - MUST: Pure Go only — only stdlib imports allowed (plus `github.com/google/uuid` for IDs)
  - MUST: Define ports as interfaces in this package
  - MUST: Define domain errors as exported sentinel errors using `errors.New`
  - MUST: Validate inputs in constructors / factory functions (return error, don't panic)
  - MUST NOT: Import any other internal package (`adapters/`, `service/`, etc.)
  - MUST NOT: Import any framework (BubbleTea, Lipgloss, viper, etc.)
  - MUST NOT: Perform I/O — no `os`, no `net/http`, no file ops
  - MUST NOT: Define mock types in this package (mocks live in `internal/testutil/mocks`)

  **`internal/core/service/AGENTS.md`** (7 rules):
  - MUST: Each use case is a struct with constructor-injected ports
  - MUST: Each use case exposes a single `Execute(ctx, req) (resp, error)` method
  - MUST: Validate the request DTO before doing work; return domain errors
  - MUST: Tests use mocked ports from `internal/testutil/mocks`
  - MUST NOT: Import `adapters/` (only `domain/` + stdlib)
  - MUST NOT: Mutate input request structs
  - MUST NOT: Combine multiple use cases into one struct (one struct per use case)

  **`internal/adapters/primary/AGENTS.md`** (8 rules):
  - MUST: TUI is the only primary adapter for now
  - MUST: Each pane / form is a separate BubbleTea sub-model with its own `Init/Update/View`
  - MUST: Keyboard messages are routed only to the focused pane (focus enum)
  - MUST: Every feature registers its keybindings in the help registry (`bubbles/help`)
  - MUST: All styles come from `internal/adapters/primary/tui/styles.New()`
  - MUST NOT: Call adapters/secondary directly — go through service layer
  - MUST NOT: Define new styles outside the styles registry
  - MUST NOT: Use `fmt.Print*` or write to stdout (use the logger)

  **`internal/adapters/secondary/AGENTS.md`** (7 rules):
  - MUST: Each adapter implements a port defined in `internal/core/domain/`
  - MUST: Adapters MAY import 3rd-party libs (yaml, slog, etc.); MUST translate library errors to domain errors at the boundary
  - MUST: Stub adapters provide canned responses, not `// TODO panic`; they MUST satisfy the full interface
  - MUST: Integration tests use the `//go:build integration` tag
  - MUST NOT: Import `service/` or `adapters/primary/`
  - MUST NOT: Leak library types out of the package (return domain types only)
  - MUST NOT: Cache state across process lifetime — single-instance only

  **Must NOT do**:
  - ❌ No descriptive prose without an attached MUST / MUST NOT rule
  - ❌ No more than 12 rules per file (keep them scannable)

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Pure rule-writing, no code
  - **Skills**: [`writing-effective-rules`]
    - Reason: Direct overlap — rule files are the deliverable

  **Parallelization**: Wave 1; parallel with T1-T5, T7. Blocked by: none structural (just T1 for directory tree). Blocks: all subsequent tasks rely on the rules being declarative truth.

  **References**:
  - `writing-effective-rules` skill: `/Users/dnl/.claude/skills/writing-effective-rules/SKILL.md`
  - Metis directives (this plan, "Standardization Artifacts" section)
  - Hexagonal layer responsibilities (Metis "Directives for Prometheus")

  **Acceptance Criteria**:
  - [ ] Exactly 5 `AGENTS.md` files exist: root, `internal/core/domain/`, `internal/core/service/`, `internal/adapters/primary/`, `internal/adapters/secondary/`
  - [ ] Each file contains a "MUST" section and a "MUST NOT" section (verified by grep)
  - [ ] Each file's rules are bulleted, no multi-paragraph prose
  - [ ] Rules sum: root 10, domain 8, service 7, primary 8, secondary 7

  **QA Scenarios**:

  ```
  Scenario: All 5 AGENTS.md files present
    Tool: Bash
    Preconditions: directories from T1 exist
    Steps:
      1. Run: find . -name 'AGENTS.md' -not -path './.git/*' -not -path './.claude/*' -not -path './.sisyphus/*' | sort
    Expected Result: 5 lines exactly:
      ./AGENTS.md
      ./internal/adapters/primary/AGENTS.md
      ./internal/adapters/secondary/AGENTS.md
      ./internal/core/domain/AGENTS.md
      ./internal/core/service/AGENTS.md
    Evidence: .sisyphus/evidence/task-6-agents-files.txt

  Scenario: Each AGENTS.md has MUST and MUST NOT sections
    Tool: Bash
    Preconditions: files exist
    Steps:
      1. For each file, run: grep -cE '^\\*\\*MUST(\\s+NOT)?\\*\\*|^- MUST|^- MUST NOT' <file>
    Expected Result: Each file has ≥ 2 occurrences (one MUST and one MUST NOT minimum)
    Evidence: .sisyphus/evidence/task-6-must-grep.txt

  Scenario: No bare paragraphs without rule prefix
    Tool: Bash
    Preconditions: files exist
    Steps:
      1. For each AGENTS.md, check that every bullet starts with MUST or MUST NOT
    Expected Result: All bullets are rule statements
    Evidence: .sisyphus/evidence/task-6-rule-format.txt
  ```

  **Evidence to Capture**:
  - [ ] task-6-agents-files.txt
  - [ ] task-6-must-grep.txt
  - [ ] task-6-rule-format.txt
  - [ ] Copy of each AGENTS.md committed to the evidence dir

  **Commit**: YES
  - Message: `docs(rules): add root and per-layer AGENTS.md hierarchy`
  - Files: `AGENTS.md`, `internal/core/{domain,service}/AGENTS.md`, `internal/adapters/{primary,secondary}/AGENTS.md`
  - Pre-commit: `make lint` (no code changes, just files)

- [x] 7. **Four ADRs in `docs/adr/`**

  **What to do**: Write exactly 4 ADRs using the Michael Nygard format (`Status / Context / Decision / Consequences`). File names use `NNNN-kebab-title.md` numbering.

  - `docs/adr/0001-hexagonal-architecture-with-primary-secondary-naming.md`
    - Decision: hexagonal layout + `primary/secondary` naming
    - Context: Standardization > YAGNI; need clear extension story
    - Consequences: Vertical slices per feature; ports in domain; constructor DI in main.go

  - `docs/adr/0002-stub-mode-for-bootstrap.md`
    - Decision: tmux/git/agent adapters are stubbed; sessions are JSON records
    - Context: David's "stub mode" decision — focus bootstrap on framework, not integrations
    - Consequences: Stubs are real interface impls with canned data; real integrations are post-bootstrap

  - `docs/adr/0003-json-file-with-atomic-write-through.md`
    - Decision: Single JSON file, atomic writes, write-through on every mutation
    - Context: Simplest persistence; future-replace via swapping adapter
    - Consequences: Last-writer-wins on concurrent processes (documented, not enforced); SchemaVersion field present for future migration

  - `docs/adr/0004-tdd-with-teatest-for-tui-layer.md`
    - Decision: TDD discipline across domain/service/adapter/TUI; teatest v2 for TUI
    - Context: Standardization process must be testable end-to-end
    - Consequences: Golden files in `testdata/`; `make update-golden` regen flow; ANSI stripping via termenv.Ascii

  Each ADR is concise (≤ 60 lines). Status starts as `Accepted` (we're committing to these from day 1).

  **Must NOT do**:
  - ❌ Do NOT write more than 4 ADRs
  - ❌ Do NOT write ADRs for obvious choices (UUID, slog, YAML lib, Go version)
  - ❌ Do NOT use generic templates with empty placeholder sections — write real Context/Consequences
  - ❌ Do NOT mark any ADR as `Proposed` — these are decisions, not proposals

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: none (writing-effective-rules doesn't apply to ADRs; standard ADR format is well-known)

  **Parallelization**: Wave 1; parallel with T1-T6.

  **References**:
  - ADR format: https://github.com/joelparkerhenderson/architecture-decision-record/blob/main/locations/nygard/index.md (Michael Nygard's original)
  - Decision summaries: this plan's "Technical Decisions" section + Metis review

  **Acceptance Criteria**:
  - [ ] Exactly 4 files in `docs/adr/` matching pattern `[0-9]{4}-*.md`
  - [ ] Each ADR has `## Status`, `## Context`, `## Decision`, `## Consequences` sections
  - [ ] No ADR exceeds 80 lines
  - [ ] All ADRs are `Status: Accepted`

  **QA Scenarios**:

  ```
  Scenario: Exactly 4 ADRs with correct format
    Tool: Bash
    Preconditions: docs/adr/ contains files
    Steps:
      1. Run: ls docs/adr/[0-9]*.md | wc -l
      2. Run: for f in docs/adr/[0-9]*.md; do grep -c '^## Status\|^## Context\|^## Decision\|^## Consequences' "$f"; done
    Expected Result: Step 1 prints "4"; Step 2 prints "4" four times
    Evidence: .sisyphus/evidence/task-7-adr-count.txt and task-7-adr-sections.txt

  Scenario: All ADRs are Accepted
    Tool: Bash
    Steps:
      1. Run: grep -l '^Status: Accepted' docs/adr/[0-9]*.md | wc -l
    Expected Result: "4"
    Evidence: .sisyphus/evidence/task-7-adr-accepted.txt
  ```

  **Evidence to Capture**:
  - [ ] task-7-adr-count.txt
  - [ ] task-7-adr-sections.txt
  - [ ] task-7-adr-accepted.txt
  - [ ] Copy of all 4 ADRs to evidence/

  **Commit**: YES
  - Message: `docs(adr): record 4 architectural decisions`
  - Files: `docs/adr/0001-*.md` through `0004-*.md`
  - Pre-commit: `make lint`

- [x] 8. **Session Domain (Entity, Errors, Ports)**

  **What to do**:
  - Create `internal/core/domain/session/session.go`:
    - `Session` struct with fields: `ID uuid.UUID`, `Name string`, `ProjectName string`, `Order int`, `CreatedAt time.Time`, `UpdatedAt time.Time`
    - `New(name, project string) (Session, error)` factory: validates name non-empty (after trim), ≤ 100 chars; project non-empty (after trim), ≤ 100 chars; sets ID, timestamps, Order=0 (will be set by use case)
    - `(s *Session) Rename(newName string) error`: validates, updates `UpdatedAt`
    - `(s *Session) String() string`: pretty form (omits UUID)
  - Create `internal/core/domain/session/errors.go`:
    - `ErrEmptyName = errors.New("session name cannot be empty")`
    - `ErrNameTooLong = errors.New("session name exceeds 100 characters")`
    - `ErrEmptyProject = errors.New("project name cannot be empty")`
    - `ErrProjectTooLong = errors.New("project name exceeds 100 characters")`
    - `ErrNotFound = errors.New("session not found")`
    - `ErrAlreadyExists = errors.New("session already exists")`
  - Create `internal/core/domain/session/ports.go`:
    - `Repository` interface with: `Save(ctx, s Session) error`, `Get(ctx, id uuid.UUID) (Session, error)`, `List(ctx) ([]Session, error)`, `Delete(ctx, id uuid.UUID) error`
    - `TmuxAdapter` interface: `CreateSession(ctx, name string) (tmuxID string, err error)`, `KillSession(ctx, tmuxID string) error`
    - `GitAdapter` interface: `CreateWorktree(ctx, baseBranch, path string) error`, `RemoveWorktree(ctx, path string) error`
    - `AgentLauncher` interface: `Launch(ctx, harness, workdir string) (pid int, err error)`
  - Update `internal/testutil/fixtures/sessions.go` from T4 to use real Session type
  - Add tests `session_test.go`, `errors_test.go`:
    - factory rejects empty name, too-long name, etc.
    - Rename produces new `UpdatedAt` later than original
    - error sentinels match via `errors.Is`
    - all tests follow TDD (write tests first, then impl)

  **Must NOT do**:
  - ❌ Do NOT add a `Project` entity (only `ProjectName string` field)
  - ❌ Do NOT add a status field beyond what's needed (Branch/PR/Agent status are TUI-time concerns)
  - ❌ Do NOT import any package outside stdlib + `github.com/google/uuid`
  - ❌ Do NOT panic — return errors

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Domain modeling decisions matter long-term; need careful types and invariants
  - **Skills**: [`programming-skills:golang-dev-guidelines`, `quality:test-on-change`]
    - Reason: Idiomatic Go + project's testing discipline (TDD)

  **Parallelization**: Wave 2; parallel with T9-T12. Blocked by: T1, T3, T4, T6.

  **References**:
  - Entity pattern: golang-api-hexagonal `internal/domain/user.go` (lines 35-50 for ports definition)
  - UUID lib: https://pkg.go.dev/github.com/google/uuid
  - Sentinel errors: https://pkg.go.dev/errors#New

  **Acceptance Criteria**:
  - [ ] `go test ./internal/core/domain/session/...` ≥ 15 tests, all PASS
  - [ ] `go vet` clean
  - [ ] `grep -r 'import' internal/core/domain/session/` shows only stdlib + `github.com/google/uuid`
  - [ ] `Session.New("", "x")` returns `ErrEmptyName`
  - [ ] `Session.New(strings.Repeat("a", 101), "x")` returns `ErrNameTooLong`
  - [ ] All 4 ports defined as exported interfaces

  **QA Scenarios**:

  ```
  Scenario: Session factory validates inputs (table-driven)
    Tool: Bash
    Preconditions: package implemented
    Steps:
      1. Run: go test -run TestSession_New -v ./internal/core/domain/session
    Expected Result: PASS; ≥ 6 sub-tests cover: empty name, whitespace name, name too long, empty project, project too long, happy path
    Evidence: .sisyphus/evidence/task-8-new-validation.txt

  Scenario: Rename updates timestamp
    Tool: Bash
    Steps:
      1. Run: go test -run TestSession_Rename -v ./internal/core/domain/session
    Expected Result: PASS; asserts UpdatedAt > original UpdatedAt
    Evidence: .sisyphus/evidence/task-8-rename.txt

  Scenario: Domain has no forbidden imports
    Tool: Bash
    Steps:
      1. Run: go list -f '{{.Imports}}' ./internal/core/domain/session
    Expected Result: Output contains only stdlib packages + "github.com/google/uuid"
    Failure Indicators: Any "charmbracelet" or "yaml" or other adapter-layer import
    Evidence: .sisyphus/evidence/task-8-imports-clean.txt
  ```

  **Evidence to Capture**:
  - [ ] task-8-new-validation.txt
  - [ ] task-8-rename.txt
  - [ ] task-8-imports-clean.txt
  - [ ] task-8-coverage.txt

  **Commit**: YES
  - Message: `feat(domain): introduce Session entity with ports`
  - Files: `internal/core/domain/session/`, `internal/testutil/fixtures/sessions.go` (updated)
  - Pre-commit: `make test && make lint`

- [x] 9. **JSON Storage Adapter**

  **What to do**:
  - Create `internal/adapters/secondary/storage/json/store.go`:
    - `Store` struct: holds `path string`, `mu sync.Mutex`, `sessions map[uuid.UUID]session.Session`, `schemaVersion int`
    - `New(path string) (*Store, error)`: ensures parent dir; loads file if exists, creates empty if not; on corruption → rename to `path.corrupted.<unix-ts>.json`, log WARN via injected logger, start fresh
    - Implements `session.Repository`: `Save`, `Get`, `List`, `Delete`
    - Every mutation: update in-memory map → `persist()` (atomic write via `shared/paths.AtomicWrite`)
    - JSON schema: `{"schemaVersion": 1, "sessions": [...]}` where each session has all domain fields
  - Add `store_test.go` (unit, with t.TempDir())
  - Add `store_integration_test.go` (`//go:build integration`):
    - corruption recovery: write invalid JSON, open → asserts corrupted file renamed + new file empty
    - atomic write: simulate concurrent Save calls (e.g., 100 goroutines) → asserts final state coherent
    - missing parent dir: opens new path under TempDir/foo/bar → dir created
    - concurrent process simulation (last-writer-wins): documented in test comment

  **Must NOT do**:
  - ❌ Do NOT use `encoding/json` MarshalIndent for size beyond 2-space indent (keep it small)
  - ❌ Do NOT add file locking or any multi-instance coordination
  - ❌ Do NOT cache parsed JSON across mutations (single source of truth = file, in-memory map mirrors it)
  - ❌ Do NOT log to stderr from this package; use the injected logger

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Storage adapter requires careful concurrency, error handling, atomic-write reasoning
  - **Skills**: [`programming-skills:golang-dev-guidelines`, `quality:test-on-change`]

  **Parallelization**: Wave 2; parallel with T8 (will use Session type once T8 lands), T10-T12.

  **References**:
  - Atomic write pattern: `internal/shared/paths/paths.go` (from T3)
  - Domain Session type: `internal/core/domain/session/` (from T8)
  - Build tag pattern: https://pkg.go.dev/cmd/go#hdr-Build_constraints

  **Acceptance Criteria**:
  - [ ] `go test ./internal/adapters/secondary/storage/json/` ≥ 8 tests, PASS
  - [ ] `go test -tags=integration ./internal/adapters/secondary/storage/json/` ≥ 3 integration tests, PASS
  - [ ] Corrupted file recovery test: invalid JSON in → store opens → corrupted-file renamed → new file with empty sessions array
  - [ ] After Save then process restart, sessions are recovered identically
  - [ ] Implementation satisfies `session.Repository` interface (compile-time check via `var _ session.Repository = (*Store)(nil)`)

  **QA Scenarios**:

  ```
  Scenario: Save → reload preserves state
    Tool: Bash
    Steps:
      1. Run: go test -run TestStore_Persistence -v ./internal/adapters/secondary/storage/json
    Expected Result: PASS; creates store with TempDir, saves 3 sessions, closes, reopens, asserts all 3 present with identical fields
    Evidence: .sisyphus/evidence/task-9-persistence.txt

  Scenario: Corrupted file is recovered
    Tool: Bash
    Steps:
      1. Run: go test -tags=integration -run TestStore_CorruptionRecovery -v ./internal/adapters/secondary/storage/json
    Expected Result: PASS; writes "not json{{{" to file, opens store, asserts: original file renamed to data.corrupted.<ts>.json, new data.json created with {"schemaVersion":1,"sessions":[]}
    Evidence: .sisyphus/evidence/task-9-corruption.txt

  Scenario: Atomic write doesn't leave tmp files
    Tool: Bash
    Steps:
      1. Run: go test -tags=integration -run TestStore_AtomicWrite -v ./internal/adapters/secondary/storage/json
    Expected Result: PASS; after 100 concurrent saves, no .tmp files in dir
    Evidence: .sisyphus/evidence/task-9-atomic.txt
  ```

  **Evidence to Capture**:
  - [ ] task-9-persistence.txt
  - [ ] task-9-corruption.txt
  - [ ] task-9-atomic.txt
  - [ ] task-9-coverage.txt

  **Commit**: YES
  - Message: `feat(storage): implement JSON storage with atomic writes and corruption recovery`
  - Files: `internal/adapters/secondary/storage/json/`
  - Pre-commit: `make test && make test-integration && make lint`

- [x] 10. **YAML Config Loader**

  **What to do**:
  - Create `internal/adapters/secondary/config/yaml/loader.go`:
    - `Config` struct with sensible defaults: `Dashboard.MinWidth int (default 60)`, `Dashboard.MinHeight int (default 15)`, `Dashboard.FocusOnStart string (default "sessions")`, `Logging.Level string (default "info")`, `Storage.Path string (default "" → use paths.DataFile())`
    - `Default() Config` returns the defaults
    - `Load(path string) (Config, error)`:
      - If file doesn't exist → return `Default()` (no error)
      - If invalid YAML → return error wrapping `errs.ErrInvalidInput` with file:line info from yaml.v3
      - Decode into Config with defaults applied for missing fields (use yaml `default` tags or merge logic)
      - Validate: focus value must be one of "sessions", "status", "preview"; min size > 0
    - `(c Config) Validate() error`
  - Add `loader_test.go` (unit):
    - missing file → default values
    - invalid YAML → error with file:line
    - partial YAML → defaults filled in for missing fields
    - invalid value (e.g., FocusOnStart="bogus") → validation error

  **Must NOT do**:
  - ❌ Do NOT use viper or other config frameworks
  - ❌ Do NOT support TOML / JSON / env-var-based config
  - ❌ Do NOT hot-reload / watch the config file
  - ❌ Do NOT cache config in package-level globals

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Config validation edge cases require care
  - **Skills**: [`programming-skills:golang-dev-guidelines`]

  **Parallelization**: Wave 2; parallel with T8, T9, T11, T12.

  **References**:
  - `gopkg.in/yaml.v3`: https://pkg.go.dev/gopkg.in/yaml.v3
  - `internal/shared/paths` for default storage path
  - `internal/shared/errs` for `ErrInvalidInput`

  **Acceptance Criteria**:
  - [ ] `go test ./internal/adapters/secondary/config/yaml/` ≥ 6 tests, PASS
  - [ ] `Default()` returns sensible defaults
  - [ ] `Load("/nonexistent")` returns `Default(), nil`
  - [ ] `Load("/tmp/bad.yaml")` with `not: valid: yaml::` returns error with file:line in message

  **QA Scenarios**:

  ```
  Scenario: Missing config returns defaults
    Tool: Bash
    Steps:
      1. Run: go test -run TestLoad_Missing -v ./internal/adapters/secondary/config/yaml
    Expected Result: PASS; cfg.Dashboard.MinWidth == 60, cfg.Logging.Level == "info"
    Evidence: .sisyphus/evidence/task-10-defaults.txt

  Scenario: Invalid YAML returns error with file:line
    Tool: Bash
    Steps:
      1. Run: go test -run TestLoad_Invalid -v ./internal/adapters/secondary/config/yaml
    Expected Result: PASS; error message contains "line " and file path
    Evidence: .sisyphus/evidence/task-10-invalid.txt
  ```

  **Evidence to Capture**:
  - [ ] task-10-defaults.txt
  - [ ] task-10-invalid.txt
  - [ ] task-10-partial.txt (partial YAML → defaults applied)

  **Commit**: YES
  - Message: `feat(config): implement YAML config loader with defaults`
  - Files: `internal/adapters/secondary/config/yaml/`
  - Pre-commit: `make test && make lint`

- [x] 11. **Logger Adapter (slog → log file)**

  **What to do**:
  - Create `internal/adapters/secondary/logger/slog/logger.go`:
    - `New(level string) (*slog.Logger, io.Closer, error)`:
      - Opens log file at `paths.LogFile()` with `os.O_APPEND|os.O_CREATE|os.O_WRONLY`, perm 0o644
      - Ensures parent dir exists via `paths.EnsureDir`
      - Returns `slog.New(slog.NewJSONHandler(file, &slog.HandlerOptions{Level: parsedLevel}))`
      - Returns the file as `io.Closer` (caller must defer Close)
      - Honors `OVERSEER_LOG_LEVEL` env var override
  - Add `logger_test.go`: writes a log line, asserts file contains expected JSON fields (timestamp, level, msg)
  - Add `logger_integration_test.go` (`//go:build integration`): uses real XDG path

  **Must NOT do**:
  - ❌ Do NOT write to stderr or stdout in any production code path
  - ❌ Do NOT use logrus, zap, or other 3rd-party log libs — stdlib `log/slog` only
  - ❌ Do NOT rotate log file (defer to OS / future feature)

  **Recommended Agent Profile**: `quick`. Skills: [`programming-skills:golang-dev-guidelines`].

  **Parallelization**: Wave 2; parallel with T8-T10, T12.

  **References**:
  - `log/slog`: https://pkg.go.dev/log/slog
  - `paths.LogFile()` from T3

  **Acceptance Criteria**:
  - [ ] `go test ./internal/adapters/secondary/logger/slog/` PASS
  - [ ] `OVERSEER_LOG_LEVEL=debug` overrides level
  - [ ] After `logger.Info("hello")`, the log file contains JSON line with `"msg":"hello"`

  **QA Scenarios**:

  ```
  Scenario: Logger writes JSON to log file
    Tool: Bash
    Steps:
      1. Run: go test -run TestLogger_WritesJSON -v ./internal/adapters/secondary/logger/slog
    Expected Result: PASS; reads back the file and asserts JSON structure
    Evidence: .sisyphus/evidence/task-11-json-log.txt

  Scenario: Log level env override
    Tool: Bash
    Steps:
      1. Run: OVERSEER_LOG_LEVEL=debug go test -run TestLogger_LevelEnv -v ./internal/adapters/secondary/logger/slog
    Expected Result: PASS; debug-level message appears in log
    Evidence: .sisyphus/evidence/task-11-level-env.txt
  ```

  **Evidence**: task-11-json-log.txt, task-11-level-env.txt

  **Commit**: `feat(logger): wire slog to XDG log file`

- [x] 12. **Stub Adapters: tmux + git + agent**

  **What to do**:
  - Create `internal/adapters/secondary/tmux/stub/stub.go`:
    - `Stub` struct, no internal state beyond calls counter for test assertions
    - `CreateSession(ctx, name string) (string, error)` returns `"tmux-stub-" + name + "-" + first-8-chars-of-new-uuid`, no error
    - `KillSession(ctx, id string) error` returns nil (records call)
    - Implements `session.TmuxAdapter`
  - Create `internal/adapters/secondary/git/stub/stub.go`:
    - `CreateWorktree(ctx, base, path string) error` returns nil (records call)
    - `RemoveWorktree(ctx, path string) error` returns nil (records call)
    - Implements `session.GitAdapter`
  - Create `internal/adapters/secondary/agent/stub/stub.go`:
    - `Launch(ctx, harness, workdir string) (int, error)` returns `12345, nil` (records call)
    - Implements `session.AgentLauncher`
  - Add a unit test in each package asserting:
    - Compile-time interface satisfaction (`var _ session.TmuxAdapter = (*Stub)(nil)`)
    - Call recording works (calls counter increments)
  - Add `stub_doc.go` header in each: "This is a stub adapter. It satisfies the port interface with canned responses. Replace with real implementation when integrating real tmux/git/agent."

  **Must NOT do**:
  - ❌ Do NOT actually shell out to `tmux`, `git`, or any binary
  - ❌ Do NOT use `// TODO` placeholders — the methods MUST return real (canned) values
  - ❌ Do NOT add a "verbose stub mode" or auto-side-effects (the stub is inert; no fake streaming, no fake events)

  **Recommended Agent Profile**: `quick`. Skills: [`programming-skills:golang-dev-guidelines`].

  **Parallelization**: Wave 2; parallel with T8-T11. Blocked by: T8 (needs port interfaces).

  **References**: `internal/core/domain/session/ports.go` (from T8)

  **Acceptance Criteria**:
  - [ ] All 3 stub packages compile and pass tests
  - [ ] Compile-time interface satisfaction asserted via `var _ = ...` declarations in each package
  - [ ] No `// TODO` strings in stub files (grep check)
  - [ ] No `exec.Command` calls in stub files (grep check)

  **QA Scenarios**:

  ```
  Scenario: Stubs satisfy port interfaces at compile time
    Tool: Bash
    Steps:
      1. Run: go build ./internal/adapters/secondary/{tmux,git,agent}/stub
    Expected Result: Exit 0; no compile errors
    Evidence: .sisyphus/evidence/task-12-compile.txt

  Scenario: Stubs have no TODO and no exec.Command
    Tool: Bash
    Steps:
      1. Run: grep -rE 'TODO|exec\\.Command' internal/adapters/secondary/{tmux,git,agent}/stub/
    Expected Result: No output, exit 1 (grep finds nothing)
    Evidence: .sisyphus/evidence/task-12-no-todo.txt

  Scenario: Call recording works
    Tool: Bash
    Steps:
      1. Run: go test -v ./internal/adapters/secondary/tmux/stub
    Expected Result: PASS; test calls CreateSession, asserts counter == 1
    Evidence: .sisyphus/evidence/task-12-recording.txt
  ```

  **Evidence**: task-12-compile.txt, task-12-no-todo.txt, task-12-recording.txt

  **Commit**: `feat(stubs): add stub tmux/git/agent adapters with canned responses`

- [x] 13. **CreateSession Use Case**

  **What to do**:
  - Create `internal/core/service/session/create.go`:
    - `CreateUseCase` struct: `repo session.Repository`, `tmux session.TmuxAdapter`, `git session.GitAdapter`, `logger *slog.Logger`
    - `NewCreateUseCase(repo, tmux, git, logger)` constructor
    - `CreateRequest{Name, ProjectName string}` DTO
    - `CreateResponse{Session session.Session}` DTO
    - `Execute(ctx, CreateRequest) (CreateResponse, error)`:
      - Build domain Session via `session.New(name, project)`
      - Compute next Order = max(existing in same project) + 1
      - Call `tmux.CreateSession(ctx, name)` (stub returns canned tmuxID)
      - Call `git.CreateWorktree(ctx, "main", "<workdir>/"+name)` (stub returns nil)
      - On any error, log and return wrapped error
      - `repo.Save(ctx, session)`
      - Return response
  - Mock for `session.Repository` at `internal/testutil/mocks/session_repository_mock.go` (handwritten, follows T4 template)
  - Mock for `session.TmuxAdapter` and `session.GitAdapter` (handwritten)
  - Add `create_test.go`:
    - happy path (with all mocks)
    - rejects empty name (mocks NOT called)
    - rejects duplicate name in same project (existing check via repo.List)
    - propagates tmux error
    - sets Order = max(project) + 1
  - **TDD order**: write tests first

  **Must NOT do**:
  - ❌ Do NOT call adapters directly without going through ports
  - ❌ Do NOT mutate the request DTO
  - ❌ Do NOT import `adapters/` packages — only `domain/` + stdlib

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Use case orchestration with multiple ports, error propagation, ordering logic
  - **Skills**: [`programming-skills:golang-dev-guidelines`, `quality:test-on-change`]

  **Parallelization**: Wave 3; parallel with T14, T15, T16.

  **References**:
  - Use case pattern: realworld-go `internal/domain/user.NewService` (constructor-injected ports)
  - Port interfaces from T8

  **Acceptance Criteria**:
  - [ ] `go test ./internal/core/service/session/...` (Create tests) ≥ 6 sub-tests, all PASS
  - [ ] No import of `internal/adapters/...` from this file
  - [ ] Duplicate-name-in-same-project returns `session.ErrAlreadyExists`
  - [ ] `Order` field is set to 1 + max(existing in project)

  **QA Scenarios**:

  ```
  Scenario: CreateSession happy path
    Tool: Bash
    Steps:
      1. Run: go test -run TestCreateUseCase_HappyPath -v ./internal/core/service/session
    Expected Result: PASS; mock repo Save called with Session{Name:"my-session", Project:"overseer", Order:1}
    Evidence: .sisyphus/evidence/task-13-happy.txt

  Scenario: Duplicate name rejected
    Tool: Bash
    Steps:
      1. Run: go test -run TestCreateUseCase_Duplicate -v ./internal/core/service/session
    Expected Result: PASS; returns ErrAlreadyExists; repo.Save not called
    Evidence: .sisyphus/evidence/task-13-duplicate.txt

  Scenario: Order auto-incremented per project
    Tool: Bash
    Steps:
      1. Run: go test -run TestCreateUseCase_OrderIncrement -v ./internal/core/service/session
    Expected Result: PASS; creating 3 sessions in same project yields Orders 1, 2, 3
    Evidence: .sisyphus/evidence/task-13-order.txt
  ```

  **Evidence**: task-13-happy.txt, task-13-duplicate.txt, task-13-order.txt, task-13-coverage.txt

  **Commit**: `feat(session): implement CreateSession use case`

- [x] 14. **RenameSession Use Case**

  **What to do**:
  - `internal/core/service/session/rename.go`:
    - `RenameUseCase` struct: `repo session.Repository`, `logger *slog.Logger`
    - `RenameRequest{ID uuid.UUID, NewName string}`, `RenameResponse{Session session.Session}`
    - `Execute(ctx, RenameRequest)`: fetch session, call `s.Rename(NewName)`, persist
  - `rename_test.go`:
    - happy path
    - empty name rejected
    - non-existent ID → `ErrNotFound`
    - duplicate name in same project rejected
  - **TDD order**

  **Must NOT do**: Same as T13 (no adapter imports, no DTO mutation).

  **Recommended Agent Profile**: `deep`. Skills: [`programming-skills:golang-dev-guidelines`, `quality:test-on-change`].

  **Parallelization**: Wave 3; parallel with T13, T15, T16.

  **References**: Use case pattern (T13), Session.Rename method (T8).

  **Acceptance Criteria**:
  - [ ] ≥ 5 sub-tests PASS
  - [ ] Non-existent ID → `ErrNotFound`
  - [ ] Duplicate-in-project → `ErrAlreadyExists`

  **QA Scenarios**:

  ```
  Scenario: RenameSession happy path
    Tool: Bash
    Steps:
      1. Run: go test -run TestRenameUseCase_HappyPath -v ./internal/core/service/session
    Expected Result: PASS; mock repo.Save called with updated session
    Evidence: .sisyphus/evidence/task-14-happy.txt

  Scenario: Rename to empty rejected
    Tool: Bash
    Steps:
      1. Run: go test -run TestRenameUseCase_EmptyName -v ./internal/core/service/session
    Expected Result: PASS; returns session.ErrEmptyName; Save not called
    Evidence: .sisyphus/evidence/task-14-empty.txt
  ```

  **Evidence**: task-14-happy.txt, task-14-empty.txt

  **Commit**: `feat(session): implement RenameSession use case`

- [x] 15. **ReorderSession Use Case**

  **What to do**:
  - `internal/core/service/session/reorder.go`:
    - `ReorderUseCase` struct: `repo session.Repository`, `logger *slog.Logger`
    - `ReorderRequest{ID uuid.UUID, Direction int}` (+1 = down, -1 = up)
    - `ReorderResponse{Sessions []session.Session}` (full list in new order, per project)
    - `Execute`:
      - Fetch target session
      - List sessions in same project (sort by Order)
      - Find target index
      - If at boundary (idx==0 with -1 OR idx==len-1 with +1) → return `errs.ErrNoOp` (silent no-op signal)
      - Swap Order with neighbor; persist both
      - Return updated project list
  - `reorder_test.go`:
    - move-down within group
    - move-up within group
    - boundary first + up → `ErrNoOp`
    - boundary last + down → `ErrNoOp`
    - single-session group → `ErrNoOp` either direction
    - across-project: ensured impossible by Direction semantics (no test needed — type-system enforced)

  **Must NOT do**:
  - ❌ Do NOT allow cross-project reorder
  - ❌ Do NOT return an error on boundary — use `errs.ErrNoOp` sentinel (UI translates to silent skip)
  - ❌ Do NOT animate / sleep

  **Recommended Agent Profile**: `deep`. Skills: [`programming-skills:golang-dev-guidelines`, `quality:test-on-change`].

  **Parallelization**: Wave 3; parallel with T13, T14, T16.

  **References**: Order field semantics (T8), `errs.ErrNoOp` to be added to `internal/shared/errs/errs.go` as part of this task.

  **Acceptance Criteria**:
  - [ ] ≥ 6 sub-tests PASS
  - [ ] Boundary case returns `errs.ErrNoOp` (NOT a generic error)
  - [ ] Move-down swaps Order with next session in project list (verified via mock recording)
  - [ ] `errs.ErrNoOp` sentinel added to `internal/shared/errs/errs.go`

  **QA Scenarios**:

  ```
  Scenario: Reorder moves within group
    Tool: Bash
    Steps:
      1. Run: go test -run TestReorderUseCase_MoveDown -v ./internal/core/service/session
    Expected Result: PASS; given [A, B, C] in same project, move B down → final order [A, C, B]
    Evidence: .sisyphus/evidence/task-15-move-down.txt

  Scenario: Boundary is silent no-op
    Tool: Bash
    Steps:
      1. Run: go test -run TestReorderUseCase_BoundaryNoOp -v ./internal/core/service/session
    Expected Result: PASS; first item + up → ErrNoOp; repo.Save NOT called
    Evidence: .sisyphus/evidence/task-15-boundary.txt
  ```

  **Evidence**: task-15-move-down.txt, task-15-boundary.txt

  **Commit**: `feat(session): implement ReorderSession use case`

- [x] 16. **ListSessions Use Case**

  **What to do**:
  - `internal/core/service/session/list.go`:
    - `ListUseCase` struct: `repo session.Repository`
    - `ListRequest{}` (no params for bootstrap)
    - `ListResponse{Groups []SessionGroup}` where `SessionGroup{ProjectName string, Sessions []session.Session}`
    - Sort: groups by ProjectName ASC; within group by Order ASC
  - `list_test.go`:
    - empty repo → empty Groups
    - single project, multiple sessions → 1 group, sorted by Order
    - multiple projects → multiple groups, sorted by ProjectName

  **Must NOT do**: standard (no adapter imports).

  **Recommended Agent Profile**: `quick` (simple use case). Skills: [`programming-skills:golang-dev-guidelines`].

  **Parallelization**: Wave 3; parallel with T13-T15.

  **References**: Session entity (T8).

  **Acceptance Criteria**:
  - [ ] ≥ 4 sub-tests PASS
  - [ ] Empty repo → response has `Groups == nil` or `len(Groups) == 0`
  - [ ] Sessions sorted by Order ASC within each group
  - [ ] Groups sorted by ProjectName ASC

  **QA Scenarios**:

  ```
  Scenario: ListSessions groups and sorts correctly
    Tool: Bash
    Steps:
      1. Run: go test -run TestListUseCase_Grouping -v ./internal/core/service/session
    Expected Result: PASS; given mixed input, asserts groups returned in alphabetical order with sessions ordered
    Evidence: .sisyphus/evidence/task-16-grouping.txt
  ```

  **Evidence**: task-16-grouping.txt

  **Commit**: `feat(session): implement ListSessions use case`

- [ ] 17. **SessionsList Pane Component**

  **What to do**:
  - Create `internal/adapters/primary/tui/session/list.go`:
    - `Model` struct: `groups []service.SessionGroup`, `cursor int` (flat index across all sessions, skipping headers), `styles *styles.Styles`, `focused bool`, `width int`, `height int`
    - `New(styles, listUC *service.ListUseCase) Model` constructor
    - Implements `tea.Model` (Init / Update / View)
    - `Init() tea.Cmd`: returns a `cmd` that calls `listUC.Execute(ctx)` and produces a `groupsLoadedMsg` with the result
    - `Update`: handles `tea.KeyMsg` (j/k cursor, J/K reorder which sends a `reorderRequestMsg`), `tea.WindowSizeMsg`, `groupsLoadedMsg`, `sessionsChangedMsg` (refresh from listUC)
    - `View`: renders project group headers + indented session items; selected item highlighted; outer border uses focused/blurred style based on `focused`
    - Exports `Keybindings() []key.Binding` for help registry (T22)
    - Exports `SetFocus(bool)`, `SelectedSession() (session.Session, ok bool)`
  - Tests `list_test.go` using teatest + golden files:
    - empty list → empty-state hint visible
    - 2 groups with 3 sessions each → snapshot matches golden file
    - j cursor moves down; k cursor moves up; bounds respected
    - Focused border vs blurred border visually distinct

  **Must NOT do**:
  - ❌ Do NOT call `listUC` directly in View (only via Update with cmds)
  - ❌ Do NOT implement expand/collapse on group headers (all always expanded)
  - ❌ Do NOT show UUIDs in the rendered output (hidden in TUI)

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
  - **Skills**: [`bubbletea-designer`, `bubbletea-maintenance`]
    - Reason: This is the most visually-complex pane; designer guides component selection, maintenance ensures BubbleTea best practices

  **Parallelization**: Wave 4; parallel with T18-T22. Blocked by: T5, T16.

  **References**:
  - bubbles/list custom delegate: https://github.com/charmbracelet/bubbles/tree/master/list (note: we manually render groups, not using list.Model directly because groups are non-trivial)
  - soft-serve selection.go (reference for keyboard routing pattern): https://github.com/charmbracelet/soft-serve/blob/16c8e08b/pkg/ui/pages/selection/selection.go#L23-L43
  - Styles registry from T5; ListUseCase from T16

  **Acceptance Criteria**:
  - [ ] `go test ./internal/adapters/primary/tui/session/` (list tests) ≥ 5, PASS
  - [ ] Golden file `testdata/TestList_TwoGroups.golden` committed
  - [ ] Empty state hint reads `"No sessions yet."` + `"Press n to create your first session"`
  - [ ] No UUID appears in any golden file (grep check)
  - [ ] Exports `Keybindings()` returning ≥ 4 bindings (j, k, J, K)

  **QA Scenarios**:

  ```
  Scenario: SessionsList renders groups correctly
    Tool: Bash (go test)
    Preconditions: T5, T16 complete
    Steps:
      1. Run: go test -run TestList_TwoGroups -v ./internal/adapters/primary/tui/session
    Expected Result: PASS; output matches testdata/TestList_TwoGroups.golden
    Evidence: .sisyphus/evidence/task-17-two-groups.txt + the golden file copy

  Scenario: Empty state hint when no sessions
    Tool: Bash
    Steps:
      1. Run: go test -run TestList_EmptyState -v ./internal/adapters/primary/tui/session
    Expected Result: PASS; rendered output contains "No sessions yet." and "Press n to create your first session"
    Evidence: .sisyphus/evidence/task-17-empty-state.txt

  Scenario: No UUIDs leaked to view
    Tool: Bash
    Steps:
      1. Run: grep -E '[0-9a-f]{8}-[0-9a-f]{4}-' internal/adapters/primary/tui/session/testdata/*.golden || echo "OK"
    Expected Result: "OK" printed (no UUID matches)
    Evidence: .sisyphus/evidence/task-17-no-uuid.txt
  ```

  **Evidence**: task-17-two-groups.txt, task-17-empty-state.txt, task-17-no-uuid.txt, golden files

  **Commit**: `feat(tui): add SessionsList pane with project grouping`

- [ ] 18. **StatusBar Pane Component**

  **What to do**:
  - Create `internal/adapters/primary/tui/status/bar.go`:
    - `Model` struct: `workdir, branch, prStatus, agentStatus string`, `styles *styles.Styles`, `width int`
    - `New(styles)` returns a Model with stub values (`branch: "stubbed"`, `pr: "—"`, `agent: "idle"`; workdir is `os.Getwd()` at construction)
    - Implements `tea.Model` Init/Update/View
    - `Update`: handles `tea.WindowSizeMsg`; processes optional `SetWorkdirMsg`, `SetBranchMsg`, etc. (future-proof, but stub mode never sends them)
    - `View`: renders fields as `[workdir] [branch] [pr] [agent]` separated by visible separators using `styles.Status.*`
  - Tests `bar_test.go`:
    - Stub values present in render
    - Width respected (truncates with ellipsis if needed)
    - Golden file for canonical view

  **Must NOT do**:
  - ❌ Do NOT call git / shell — `branch`, `pr` are stub strings
  - ❌ Do NOT add color-coded "alert" states yet

  **Recommended Agent Profile**: `visual-engineering`. Skills: [`bubbletea-designer`].

  **Parallelization**: Wave 4; parallel with T17, T19-T22. Blocked by: T5.

  **References**: soft-serve statusbar.go (https://github.com/charmbracelet/soft-serve/blob/16c8e08b/pkg/ui/components/statusbar/statusbar.go); Styles registry (T5).

  **Acceptance Criteria**:
  - [ ] ≥ 3 tests PASS
  - [ ] Golden file `testdata/TestStatus_Default.golden` committed
  - [ ] Render contains the four stub values verbatim

  **QA Scenarios**:

  ```
  Scenario: StatusBar default render
    Tool: Bash
    Steps:
      1. Run: go test -run TestStatus_Default -v ./internal/adapters/primary/tui/status
    Expected Result: PASS; golden file shows "stubbed", "—", "idle"
    Evidence: .sisyphus/evidence/task-18-status-default.txt + golden file

  Scenario: StatusBar truncates on narrow width
    Tool: Bash
    Steps:
      1. Run: go test -run TestStatus_Truncate -v ./internal/adapters/primary/tui/status
    Expected Result: PASS; at width 40, output shows ellipsis on workdir
    Evidence: .sisyphus/evidence/task-18-truncate.txt
  ```

  **Evidence**: task-18-status-default.txt, task-18-truncate.txt

  **Commit**: `feat(tui): add StatusBar with stub values`

- [ ] 19. **PreviewPane Component**

  **What to do**:
  - Create `internal/adapters/primary/tui/preview/pane.go`:
    - `Model` struct: `viewport viewport.Model`, `styles *styles.Styles`, `focused bool`, `width, height int`
    - `New(styles)` constructs `viewport.Model` with `HighPerformanceRendering = true`, sets static placeholder content: `"Stub mode: preview not available.\n\nThis pane will stream the selected session's tmux output when integration lands."`
    - Implements `tea.Model` Init/Update/View
    - `Update`: handles `tea.WindowSizeMsg`; routes scroll keys (j/k or PgUp/PgDn) only when focused
    - `View`: viewport with focused/blurred border
    - Exports `SetFocus(bool)`, `Keybindings() []key.Binding`
  - Tests using teatest + golden files:
    - Default render shows stub message
    - Focused border differs from blurred (via golden file diff)
    - Scroll keys move viewport when focused

  **Must NOT do**:
  - ❌ Do NOT implement any real streaming (no tmux integration, no fake periodic ticks)
  - ❌ Do NOT show the selected session's name in the placeholder (purely static)

  **Recommended Agent Profile**: `visual-engineering`. Skills: [`bubbletea-designer`].

  **Parallelization**: Wave 4; parallel with T17, T18, T20-T22. Blocked by: T5.

  **References**: glow pager.go viewport usage (https://github.com/charmbracelet/glow/blob/53788271/ui/pager.go#L108-L145); bubbles viewport docs.

  **Acceptance Criteria**:
  - [ ] ≥ 3 tests PASS
  - [ ] Golden file shows the static stub message
  - [ ] Implements `tea.Model` interface

  **QA Scenarios**:

  ```
  Scenario: PreviewPane shows stub message
    Tool: Bash
    Steps:
      1. Run: go test -run TestPreview_Default -v ./internal/adapters/primary/tui/preview
    Expected Result: PASS; golden file contains "Stub mode: preview not available."
    Evidence: .sisyphus/evidence/task-19-preview-default.txt
  ```

  **Evidence**: task-19-preview-default.txt + golden file

  **Commit**: `feat(tui): add PreviewPane with stub content`

- [ ] 20. **CreateSessionForm Modal**

  **What to do**:
  - Create `internal/adapters/primary/tui/session/create_form.go`:
    - `Model` struct: two `textinput.Model` instances (Name, ProjectName), focus index, error message, `createUC *service.CreateUseCase`, `styles *styles.Styles`
    - `New(styles, createUC)` constructor
    - Implements `tea.Model`
    - `Update`:
      - `Tab` cycles focus between fields
      - `Esc` sends `cancelFormMsg`
      - `Enter` validates (both fields non-empty), calls `createUC.Execute`, on success sends `sessionCreatedMsg`, on error displays message and stays in form
    - `View`: modal-style overlay with two labeled inputs + error line + help hint "Tab: next field, Enter: submit, Esc: cancel"
  - Tests with teatest:
    - happy path: send "my-session"+Tab+"overseer"+Enter → asserts `sessionCreatedMsg` emitted with correct fields
    - empty name → error displayed, no message emitted
    - Esc → cancel message emitted

  **Must NOT do**:
  - ❌ Do NOT call `createUC` synchronously in View
  - ❌ Do NOT auto-fill ProjectName from current dir (manual entry for now)
  - ❌ Do NOT add a delete/cancel button (Esc is the cancel)

  **Recommended Agent Profile**: `visual-engineering`. Skills: [`bubbletea-designer`, `bubbletea-maintenance`].

  **Parallelization**: Wave 4. Blocked by: T5, T13.

  **References**: bubbles textinput examples; soft-serve form patterns.

  **Acceptance Criteria**:
  - [ ] ≥ 4 tests PASS
  - [ ] Golden file for empty form, filled form, error state
  - [ ] Form emits `sessionCreatedMsg` on success, `cancelFormMsg` on Esc

  **QA Scenarios**:

  ```
  Scenario: Create form happy path
    Tool: Bash (teatest)
    Steps:
      1. Run: go test -run TestCreateForm_HappyPath -v ./internal/adapters/primary/tui/session
    Expected Result: PASS; teatest sends "my-session-1", Tab, "overseer", Enter; asserts sessionCreatedMsg captured with Name=my-session-1, Project=overseer
    Evidence: .sisyphus/evidence/task-20-create-happy.txt

  Scenario: Create form rejects empty name
    Tool: Bash
    Steps:
      1. Run: go test -run TestCreateForm_EmptyName -v ./internal/adapters/primary/tui/session
    Expected Result: PASS; submit with empty name → error shown, no message emitted
    Evidence: .sisyphus/evidence/task-20-create-empty.txt
  ```

  **Evidence**: task-20-create-happy.txt, task-20-create-empty.txt, golden files

  **Commit**: `feat(tui): add CreateSessionForm modal`

- [ ] 21. **RenameSessionForm Modal**

  **What to do**:
  - Create `internal/adapters/primary/tui/session/rename_form.go`:
    - Same shape as Create form but single `textinput.Model` (Name only), pre-filled with current name
    - `New(styles, renameUC, currentSession)` constructor
    - `Update`: Enter → call renameUC, emit `sessionRenamedMsg`; Esc → cancel
  - Tests with teatest: happy path, empty rejected, Esc cancels.

  **Must NOT do**: Same modal constraints as T20.

  **Recommended Agent Profile**: `visual-engineering`. Skills: [`bubbletea-designer`].

  **Parallelization**: Wave 4. Blocked by: T5, T14.

  **References**: T20 pattern.

  **Acceptance Criteria**:
  - [ ] ≥ 3 tests PASS
  - [ ] Pre-fills input with current name on init
  - [ ] Golden file for default + error state

  **QA Scenarios**:

  ```
  Scenario: Rename form pre-fills + submits
    Tool: Bash
    Steps:
      1. Run: go test -run TestRenameForm_HappyPath -v ./internal/adapters/primary/tui/session
    Expected Result: PASS; input pre-filled with "old-name"; user clears + types "new-name" + Enter; asserts sessionRenamedMsg with NewName=new-name
    Evidence: .sisyphus/evidence/task-21-rename-happy.txt
  ```

  **Evidence**: task-21-rename-happy.txt + golden files

  **Commit**: `feat(tui): add RenameSessionForm modal`

- [ ] 22. **Help Bar Integration (bubbles/help)**

  **What to do**:
  - Create `internal/adapters/primary/tui/help/registry.go`:
    - `Registry` struct keeps a slice of `key.Binding` per pane
    - `RegisterPane(name string, bindings []key.Binding)` adds bindings; replaces if name already registered
    - `BindingsFor(name string) []key.Binding` returns bindings for active pane + global bindings (q to quit, Tab to switch focus, 1/2/3 to jump panes)
  - Create `internal/adapters/primary/tui/help/bar.go`:
    - `Model` struct: `help help.Model` (from bubbles), `registry *Registry`, `activePane string`
    - `View()` renders the help bar showing the active pane's keybindings + globals
    - Toggleable via `?` to switch between short and full help
  - Tests:
    - Registry registers and retrieves bindings correctly
    - Help bar shows global bindings + active pane bindings
    - Toggle short/full help works

  **Must NOT do**:
  - ❌ Do NOT have features hardcode their own help text — they MUST register via the registry
  - ❌ Do NOT show ALL pane bindings at once — only active + globals

  **Recommended Agent Profile**: `visual-engineering`. Skills: [`bubbletea-designer`].

  **Parallelization**: Wave 4. Blocked by: T5.

  **References**: bubbles/help docs (https://github.com/charmbracelet/bubbles/tree/master/help).

  **Acceptance Criteria**:
  - [ ] ≥ 4 tests PASS
  - [ ] Globals always shown: `q: quit`, `Tab: next pane`, `?: toggle help`, `1/2/3: jump to pane`
  - [ ] Active pane's bindings appear; inactive panes' bindings hidden

  **QA Scenarios**:

  ```
  Scenario: Help shows active pane + globals only
    Tool: Bash
    Steps:
      1. Run: go test -run TestHelp_ActivePaneOnly -v ./internal/adapters/primary/tui/help
    Expected Result: PASS; given Sessions pane active, output contains "j: down", "k: up" but NOT preview-specific bindings
    Evidence: .sisyphus/evidence/task-22-active-only.txt

  Scenario: Toggle short/full help
    Tool: Bash
    Steps:
      1. Run: go test -run TestHelp_Toggle -v ./internal/adapters/primary/tui/help
    Expected Result: PASS; before ? : short form; after ? : multi-line full help
    Evidence: .sisyphus/evidence/task-22-toggle.txt
  ```

  **Evidence**: task-22-active-only.txt, task-22-toggle.txt + golden files

  **Commit**: `feat(tui): integrate bubbles/help with keybinding registry`

- [ ] 23. **Dashboard Composition (Focus, Routing, Pane Assembly)**

  **What to do**:
  - Create `internal/adapters/primary/tui/dashboard/model.go`:
    - `Pane` enum: `PaneSessions`, `PaneStatus`, `PaneStreams` (actually status row is non-focusable; focusable panes are Sessions + Preview only — treat status row as decoration)
    - `Model` struct holds: sub-models from T17-T22 (`sessionsList`, `statusBar`, `previewPane`, `helpBar`); `activePane Pane`; `createForm`, `renameForm` *optional* models (nil unless in modal mode); `width`, `height`; `styles *styles.Styles`; service use cases
    - `New(...)` constructor with all dependencies injected
    - `Init() tea.Cmd`: `tea.Batch(sessionsList.Init(), statusBar.Init(), previewPane.Init(), helpBar.Init())`
    - `Update(msg)`:
      - `tea.WindowSizeMsg`: store width/height; if width < cfg.MinWidth (60) OR height < cfg.MinHeight (15) → set `tooSmall` flag; route resize to all sub-models with new pane dimensions
      - `tea.KeyMsg`:
        - Global keys handled first: `q`/`ctrl+c` → `tea.Quit`; `Tab`/`Shift+Tab` → cycle focus between Sessions/Preview; `1` → focus Sessions; `2` → focus Preview; `?` → toggle help
        - If a modal form is active: route msg ONLY to that form; on `cancelFormMsg`/`sessionCreatedMsg`/`sessionRenamedMsg` from form → clear form + refresh sessions list
        - Else: feature hotkeys: `n` → open Create form; `r` → open Rename form (with selected session); `J`/`K` → route to sessionsList as reorder
        - Then route msg to active pane only
      - `sessionsChangedMsg`, `groupsLoadedMsg`: forward to sessionsList
    - `View()`:
      - If `tooSmall` → render `TooSmall.Message` centered: `"Terminal too small. Minimum size: 60x15."` + current size
      - Else: compose 3 panes via lipgloss.JoinHorizontal/JoinVertical with appropriate widths/heights and focus borders
      - If a modal is active, render overlay (centered) on top of dashboard (using lipgloss.Place)
  - Register keybindings to help.Registry on Init: each pane's bindings + global bindings + form-context bindings
  - Tests with teatest:
    - Default render with 80x24, golden file
    - Tab cycles focus (golden files showing focus indicator move)
    - `n` opens Create form (golden file of modal overlay)
    - `r` with no selected session → no-op (Create form does NOT open)
    - `J` boundary at first session → silent (no Save call, no error visible)
    - Terminal-too-small at 40x10 → fallback rendered
    - `q` quits

  **Must NOT do**:
  - ❌ Do NOT route messages to inactive panes
  - ❌ Do NOT call use cases directly in View
  - ❌ Do NOT introduce a new abstraction "FormManager" or "FocusManager" — keep it inline in the dashboard Update
  - ❌ Do NOT animate focus transitions

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Complex orchestration: focus state machine + modal overlay + global vs pane-local keys + terminal-too-small + message routing
  - **Skills**: [`bubbletea-designer`, `bubbletea-maintenance`, `programming-skills:golang-dev-guidelines`]

  **Parallelization**: Wave 5; sequential after Wave 4. Blocked by: T17-T22. Blocks: T24, T28.

  **References**:
  - soft-serve focus enum + routing: https://github.com/charmbracelet/soft-serve/blob/16c8e08b/pkg/ui/pages/selection/selection.go#L269-L282
  - Lipgloss layout composition pattern (from research findings)
  - All sub-models from T17-T22

  **Acceptance Criteria**:
  - [ ] ≥ 8 teatest scenarios PASS
  - [ ] Golden files for: default 80x24, Sessions focused, Preview focused, Create modal open, Rename modal open, Empty state, Too-small fallback (40x10)
  - [ ] `q` and `ctrl+c` both quit
  - [ ] `Tab` cycles focus; visual border indicator updates
  - [ ] `1`, `2` jump to specific panes
  - [ ] No keystroke reaches an inactive pane (verified via mock pane recording)

  **QA Scenarios**:

  ```
  Scenario: Dashboard default render at 80x24
    Tool: Bash (teatest)
    Steps:
      1. Run: go test -run TestDashboard_Default80x24 -v ./internal/adapters/primary/tui/dashboard
    Expected Result: PASS; golden file shows 3 panes with Sessions focused
    Evidence: .sisyphus/evidence/task-23-default.txt + golden

  Scenario: Tab cycles focus
    Tool: Bash
    Steps:
      1. Run: go test -run TestDashboard_TabCyclesFocus -v ./internal/adapters/primary/tui/dashboard
    Expected Result: PASS; sends Tab; asserts activePane changed from Sessions to Preview; golden file diff shows border swap
    Evidence: .sisyphus/evidence/task-23-tab.txt

  Scenario: n opens Create modal
    Tool: Bash
    Steps:
      1. Run: go test -run TestDashboard_OpenCreate -v ./internal/adapters/primary/tui/dashboard
    Expected Result: PASS; after `n`, modal overlay rendered with both empty fields; golden file matches
    Evidence: .sisyphus/evidence/task-23-create-modal.txt

  Scenario: Too-small at 40x10
    Tool: Bash
    Steps:
      1. Run: go test -run TestDashboard_TooSmall -v ./internal/adapters/primary/tui/dashboard
    Expected Result: PASS; output contains "Terminal too small" message; panes NOT rendered
    Evidence: .sisyphus/evidence/task-23-too-small.txt

  Scenario: q quits
    Tool: Bash
    Steps:
      1. Run: go test -run TestDashboard_Quit -v ./internal/adapters/primary/tui/dashboard
    Expected Result: PASS; tea.Quit cmd emitted
    Evidence: .sisyphus/evidence/task-23-quit.txt
  ```

  **Evidence**: task-23-default.txt, task-23-tab.txt, task-23-create-modal.txt, task-23-too-small.txt, task-23-quit.txt + ALL golden files

  **Commit**: `feat(tui): compose dashboard with focus management and keyboard routing`

- [ ] 24. **Composition Root: `cmd/overseer/main.go`**

  **What to do**:
  - Rewrite `cmd/overseer/main.go` (overrides T1 stub) with the full startup flow:
    1. Read config: `cfg, err := yaml.Load(paths.ConfigFile())` — on error, print to stderr and `os.Exit(1)` BEFORE touching the terminal
    2. Initialize logger: `logger, logCloser, err := slog.New(cfg.Logging.Level)` — defer close
    3. Initialize storage (Store calls T9's corruption recovery internally and logs WARN to logger if recovered)
    4. Initialize stub adapters: tmux, git, agent
    5. Construct use cases: Create, Rename, Reorder, List — each gets repo + needed adapters + logger
    6. Construct styles via `styles.New()`
    7. Construct dashboard via `dashboard.New(...)` with all dependencies injected
    8. Construct `tea.Program` with `tea.WithAltScreen()` and optionally `tea.WithMouseCellMotion()` (no mouse interaction but enable for resize events)
    9. Defer panic recovery that calls `tea.DisableMouseAllMotion()` + restores cursor + prints stack to log + non-zero exit
    10. Run `p.Run()` — on error, log and exit 1
  - Add `main_test.go`: minimal test that calls a public helper (e.g., `Bootstrap()` extracted for testability) and verifies it constructs the dashboard without panic given a TempDir for paths
  - Wire `OVERSEER_HOME` env var support (overrides XDG paths for testing)

  **Must NOT do**:
  - ❌ Do NOT call `log.Fatal` or `fmt.Println` in the TUI path (only before TUI starts up, if config parse fails)
  - ❌ Do NOT use a DI framework
  - ❌ Do NOT initialize anything that touches network / external process
  - ❌ Do NOT swallow errors silently

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Composition root is the single point where wiring must be exactly right; subtle errors here cascade everywhere
  - **Skills**: [`bubbletea-maintenance`, `programming-skills:golang-dev-guidelines`]

  **Parallelization**: Wave 5; sequential after T23. Blocked by: T9, T10, T11, T23. Blocks: T28, T30.

  **References**:
  - BubbleTea panic recovery: bubbletea-maintenance/SKILL.md "Issue: Terminal Gets Messed Up" section
  - Composition root pattern: realworld-go cmd/server/main.go

  **Acceptance Criteria**:
  - [ ] `make build` produces a working binary that launches the dashboard
  - [ ] Launching with `OVERSEER_HOME=$TMPDIR` writes data.json under that path
  - [ ] Launching with invalid config (write bad YAML first) prints error to stderr and exits 1 BEFORE entering TUI (terminal not corrupted)
  - [ ] Panic in TUI restores terminal (cursor visible, mouse motion disabled)

  **QA Scenarios**:

  ```
  Scenario: Binary launches and exits cleanly
    Tool: interactive_bash (tmux)
    Steps:
      1. Run: tmux new-session -d -s overseer-test 'OVERSEER_HOME=/tmp/overseer-qa ./bin/overseer'
      2. Wait 2 seconds (TUI starts up)
      3. Send-keys: 'q'
      4. Run: tmux capture-pane -t overseer-test -p > .sisyphus/evidence/task-24-launch.txt
      5. Run: tmux wait-for -t overseer-test (or just sleep)
    Expected Result: TUI rendered the dashboard; q quit cleanly; capture shows dashboard content with focus border
    Evidence: .sisyphus/evidence/task-24-launch.txt

  Scenario: Invalid config exits before TUI
    Tool: Bash
    Steps:
      1. Run: mkdir -p /tmp/overseer-qa-bad/.config/overseer/ && echo 'not: valid: ::' > /tmp/overseer-qa-bad/.config/overseer/config.yaml
      2. Run: XDG_CONFIG_HOME=/tmp/overseer-qa-bad/.config ./bin/overseer
    Expected Result: Exit code 1; stderr contains "config" + file path + "line"; terminal NOT corrupted (cursor visible, no mouse mode lingering)
    Evidence: .sisyphus/evidence/task-24-bad-config.txt

  Scenario: Corrupted data file recovered
    Tool: Bash
    Steps:
      1. Run: mkdir -p /tmp/overseer-qa-corrupt/.local/share/overseer && echo 'invalid{json' > /tmp/overseer-qa-corrupt/.local/share/overseer/data.json
      2. Run: XDG_DATA_HOME=/tmp/overseer-qa-corrupt/.local/share timeout 2 ./bin/overseer || true
      3. Run: ls /tmp/overseer-qa-corrupt/.local/share/overseer/
    Expected Result: Listing shows: original `data.json` (now empty), AND `data.corrupted.<ts>.json`; log file shows WARN about corruption
    Evidence: .sisyphus/evidence/task-24-corruption.txt
  ```

  **Evidence**: task-24-launch.txt, task-24-bad-config.txt, task-24-corruption.txt

  **Commit**: `feat(cmd): wire composition root with XDG setup and startup flow`

- [ ] 25. **overseer-feature Skill — Structure + SKILL.md + Feature Shape Catalog**

  **What to do**:
  - Create `.claude/skills/overseer-feature/` with the structure matching existing skills (bubbletea-designer convention):
    - `SKILL.md` — the main skill instructions
    - `README.md` — quick-start + installation
    - `VERSION` — `1.0.0`
    - `CHANGELOG.md` — entry for 1.0.0 release
    - `DECISIONS.md` — links to ADRs
    - `references/` — supporting docs
    - `assets/` — templates (empty for now, optional)
    - `tests/` — self-test script (added in T27)
    - `.claude-plugin/plugin.json` — plugin metadata
  - **`SKILL.md` content** (required sections):
    1. **Metadata frontmatter** (name, description matching existing skill pattern)
    2. **When to Use This Skill** — list trigger phrases ("add a feature to overseer", "create a new use case", etc.)
    3. **Step-by-Step Procedure** — the canonical feature-creation workflow:
       - Step 1: Define domain (entity/error/port)
       - Step 2: Implement use case (in `internal/core/service/{feature}/`) with mocked port tests
       - Step 3: Implement secondary adapter (storage extension if needed)
       - Step 4: Implement primary adapter (TUI screen/form) with teatest
       - Step 5: Register keybindings in help registry
       - Step 6: Wire in `cmd/overseer/main.go`
       - Step 7: Update per-layer AGENTS.md if new patterns introduced
       - Step 8: Add E2E teatest scenario
       - Step 9: Run `make test && make test-integration && make lint`
    4. **Feature Shape Catalog** — 5 shapes:
       - **Shape A: Form-Driven** (Create/Rename pattern) — modal with inputs, validation, submit/cancel
       - **Shape B: Inline-Edit** — edit-in-place without modal (variant of A)
       - **Shape C: Direct Action** (Reorder pattern) — keypress triggers immediate state change, no input
       - **Shape D: Async/Streaming** — long-running operation, tea.Cmd background, progress feedback (DOCUMENTED, not exercised in bootstrap)
       - **Shape E: Read-Only Detail View** — non-interactive display of session details (DOCUMENTED, not exercised)
       - For each shape: when to use, which bootstrap features exercise it (file refs), template snippet
    5. **Layer-Specific Templates** — small code snippets for each layer (domain entity stub, use case Execute stub, etc.)
    6. **MUST / MUST NOT** — explicit rules tied to root AGENTS.md
    7. **Common Pitfalls** — referencing bubbletea-maintenance issues
  - `README.md` — copy structure from existing `bubbletea-designer/README.md` (Installation, Quick Start, Features, Usage Examples, Files Structure)
  - `.claude-plugin/plugin.json` — match existing format

  **Must NOT do**:
  - ❌ Do NOT include Python scripts (this is a procedural skill, not script-based like bubbletea-designer)
  - ❌ Do NOT make this skill domain-agnostic — it MUST reference Overseer-specific paths and patterns
  - ❌ Do NOT include all 5 shapes' implementations (only catalog + templates)

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: [`customize-opencode`, `writing-effective-rules`]
    - Reason: Skill-writing skill applies directly; writing-effective-rules ensures the rules sections in SKILL.md are sharp

  **Parallelization**: Wave 6; parallel with T26, T27. Blocked by: T23 (so we know the patterns to document).

  **References**:
  - Existing skill convention: `.claude/skills/bubbletea-designer/` (full structure)
  - `customize-opencode` skill docs
  - All AGENTS.md files from T6 (skill must align with them)

  **Acceptance Criteria**:
  - [ ] Directory `.claude/skills/overseer-feature/` exists with: SKILL.md, README.md, VERSION, CHANGELOG.md, DECISIONS.md, references/, tests/, .claude-plugin/plugin.json
  - [ ] `SKILL.md` contains all 7 required sections (frontmatter + 6 numbered above)
  - [ ] Feature Shape Catalog enumerates exactly 5 shapes
  - [ ] At least 3 of the 5 shapes reference bootstrap features by exact file path
  - [ ] VERSION file contains `1.0.0`

  **QA Scenarios**:

  ```
  Scenario: Skill structure complete
    Tool: Bash
    Steps:
      1. Run: ls -la .claude/skills/overseer-feature/
      2. Run: cat .claude/skills/overseer-feature/VERSION
      3. Run: grep -c '^## ' .claude/skills/overseer-feature/SKILL.md
    Expected Result: All expected files present; VERSION = "1.0.0"; SKILL.md has ≥ 7 ## sections
    Evidence: .sisyphus/evidence/task-25-structure.txt

  Scenario: Feature Shape Catalog has exactly 5 shapes
    Tool: Bash
    Steps:
      1. Run: grep -cE '^### Shape [A-E]' .claude/skills/overseer-feature/SKILL.md
    Expected Result: "5"
    Evidence: .sisyphus/evidence/task-25-shapes.txt
  ```

  **Evidence**: task-25-structure.txt, task-25-shapes.txt, full SKILL.md copy to evidence

  **Commit**: `docs(skill): add overseer-feature skill with Feature Shape Catalog`

- [ ] 26. **Worked Example: Delete Feature Walkthrough in Skill**

  **What to do**:
  - Add `references/worked-example-delete.md` to the overseer-feature skill containing a full, step-by-step walkthrough of how to add a `DeleteSession` feature following the standard process. The walkthrough must:
    - Step 0: Show the failing teatest scenario first (TDD)
    - Step 1: Add `DeleteUseCase` in `internal/core/service/session/delete.go` (with full code snippet)
    - Step 2: Show the failing unit test for the use case (TDD)
    - Step 3: Show the use case implementation passing the test
    - Step 4: Show registering the `d` keybinding via the help registry
    - Step 5: Show wiring `d` in the dashboard's Update function (with diff vs current)
    - Step 6: Show the confirmation modal (or "no confirmation" — be explicit about the choice)
    - Step 7: Show updating `cmd/overseer/main.go` to inject the new use case
    - Step 8: Show updating any AGENTS.md files (or note "no updates needed because the pattern is already covered")
    - Step 9: Run `make test && make lint`
  - Each step has: motivation, file path(s) touched, code diff or full file content, test output
  - Reference the walkthrough from `SKILL.md` ("For a complete example, see `references/worked-example-delete.md`")
  - Note prominently: **This worked example MUST be executable by an agent without manual edits.** Reading the doc and following the steps should produce a green-tests-and-lint result.

  **Must NOT do**:
  - ❌ Do NOT actually implement Delete in the codebase (the worked example is documentation, not code)
  - ❌ Do NOT use abbreviated code snippets that leave the reader to "figure out the rest"

  **Recommended Agent Profile**: `writing`. Skills: [`customize-opencode`].

  **Parallelization**: Wave 6; parallel with T25, T27. Blocked by: T23.

  **References**: All implementation tasks (T8-T24) as the source pattern. T25's SKILL.md.

  **Acceptance Criteria**:
  - [ ] File `references/worked-example-delete.md` exists with ≥ 8 numbered steps (0-9, allowing for a final verification step)
  - [ ] Each step has motivation + file paths + code snippet
  - [ ] Document is referenced from SKILL.md
  - [ ] Final step shows passing test output (or describes what passing looks like)

  **QA Scenarios**:

  ```
  Scenario: Worked example has all 9 steps
    Tool: Bash
    Steps:
      1. Run: grep -cE '^## Step [0-9]+' .claude/skills/overseer-feature/references/worked-example-delete.md
    Expected Result: ≥ 9
    Evidence: .sisyphus/evidence/task-26-steps.txt

  Scenario: Referenced from SKILL.md
    Tool: Bash
    Steps:
      1. Run: grep -c 'worked-example-delete' .claude/skills/overseer-feature/SKILL.md
    Expected Result: ≥ 1
    Evidence: .sisyphus/evidence/task-26-reference.txt
  ```

  **Evidence**: task-26-steps.txt, task-26-reference.txt + the worked-example doc

  **Commit**: `docs(skill): document Delete feature worked example`

- [ ] 27. **Skill Self-Test Script + Final Skill Polish**

  **What to do**:
  - Create `.claude/skills/overseer-feature/tests/self_test.sh`:
    - Shell script that an agent can execute to verify the skill's worked example produces a green codebase
    - Strategy: in a clean temp dir, copy the project, follow the worked example mechanically (apply each diff), then run `make test && make test-integration && make lint`
    - Exit 0 on success; nonzero with informative message on failure
    - Document that this is a **smoke test of the skill**, not a unit test of the skill's content
  - Create `CHANGELOG.md` entry:
    - `## [1.0.0] - YYYY-MM-DD` — Initial release. Procedure for adding features to Overseer; Feature Shape Catalog (5 shapes); Delete worked example.
  - Create `DECISIONS.md` with links to relevant ADRs (`docs/adr/0001-*.md` through `0004-*.md`)
  - Ensure `README.md` (from T25) has a "Self-Test" section referencing `tests/self_test.sh`

  **Must NOT do**:
  - ❌ Do NOT make the self-test mutate the actual project repo
  - ❌ Do NOT skip the self-test if `make` not installed — fail with clear message
  - ❌ Do NOT auto-run the self-test in CI (out of scope; document how to run manually)

  **Recommended Agent Profile**: `writing`. Skills: none.

  **Parallelization**: Wave 6; parallel with T25, T26.

  **References**: T26's worked example.

  **Acceptance Criteria**:
  - [ ] `.claude/skills/overseer-feature/tests/self_test.sh` exists and is executable (`chmod +x`)
  - [ ] `CHANGELOG.md` has a 1.0.0 entry with date and 3-line description
  - [ ] `DECISIONS.md` links to all 4 ADRs
  - [ ] Running `bash .claude/skills/overseer-feature/tests/self_test.sh --dry-run` exits 0 (verifies script syntax)

  **QA Scenarios**:

  ```
  Scenario: Self-test script is executable and parses
    Tool: Bash
    Steps:
      1. Run: test -x .claude/skills/overseer-feature/tests/self_test.sh && echo OK
      2. Run: bash -n .claude/skills/overseer-feature/tests/self_test.sh && echo OK
    Expected Result: Both print "OK"
    Evidence: .sisyphus/evidence/task-27-script-ok.txt

  Scenario: CHANGELOG references 1.0.0
    Tool: Bash
    Steps:
      1. Run: grep '^## \\[1.0.0\\]' .claude/skills/overseer-feature/CHANGELOG.md
    Expected Result: 1 match
    Evidence: .sisyphus/evidence/task-27-changelog.txt
  ```

  **Evidence**: task-27-script-ok.txt, task-27-changelog.txt + script + changelog copies

  **Commit**: `docs(skill): add self-test script + README + VERSION + CHANGELOG`

- [ ] 28. **End-to-End teatest Scenarios**

  **What to do**:
  - Create `internal/adapters/primary/tui/dashboard/e2e_test.go`:
    - **Scenario 1**: Full create flow — launch → press `n` → fill form → submit → assert session appears in list
    - **Scenario 2**: Rename flow — pre-seed 1 session → launch → cursor on session → press `r` → clear name → type new → submit → assert renamed
    - **Scenario 3**: Reorder flow — pre-seed 3 sessions in same project → launch → cursor on session 2 → press `J` → assert order changed in list view
    - **Scenario 4**: Focus cycling — Tab cycles Sessions ↔ Preview; visual border switches; sessions list keys (j/k) don't affect anything when preview is focused
    - **Scenario 5**: Quit cleanly with `q` — assert tea.Quit
  - Use teatest's `WithInitialTermSize(80, 24)`; each scenario writes its own golden file
  - Use a pre-seeded mock or in-memory store with known sessions for deterministic results

  **Must NOT do**:
  - ❌ Do NOT mock at the use case level — wire real use cases with mock ports (storage / stub adapters), so we exercise the full path
  - ❌ Do NOT skip assertions on golden file content (full golden compare, not just "no error")

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Tedious orchestration of multi-step teatest with seeded state and golden snapshots
  - **Skills**: [`bubbletea-maintenance`, `quality:test-on-change`]

  **Parallelization**: Wave 7; parallel with T29, T30. Blocked by: T23, T24, T25-T27.

  **References**: teatest patterns from research (T4 helpers); all sub-models from T17-T22.

  **Acceptance Criteria**:
  - [ ] 5 E2E scenarios PASS
  - [ ] 5 golden files committed under `internal/adapters/primary/tui/dashboard/testdata/`
  - [ ] `make test` reports these tests in output

  **QA Scenarios**:

  ```
  Scenario: Full create-rename-reorder-quit E2E
    Tool: Bash
    Steps:
      1. Run: go test -run 'TestE2E_' -v ./internal/adapters/primary/tui/dashboard
    Expected Result: PASS for all 5 sub-tests; golden files match
    Evidence: .sisyphus/evidence/task-28-e2e.txt

  Scenario: E2E exercises real use cases (not deeply mocked)
    Tool: Bash
    Steps:
      1. Run: grep -E 'CreateUseCase|RenameUseCase|ReorderUseCase' internal/adapters/primary/tui/dashboard/e2e_test.go
    Expected Result: All 3 use case types referenced
    Evidence: .sisyphus/evidence/task-28-real-uc.txt
  ```

  **Evidence**: task-28-e2e.txt, task-28-real-uc.txt + 5 golden files

  **Commit**: `test(e2e): add end-to-end dashboard teatest scenarios`

- [ ] 29. **docs/architecture.md (Architecture Overview)**

  **What to do**:
  - Create `docs/architecture.md` with:
    - **Overview**: 1-paragraph summary of Overseer's purpose and hexagonal architecture
    - **Directory Map**: tree of `cmd/`, `internal/`, with one-line description each
    - **Layer Responsibilities**: 4 sections (domain / service / primary / secondary) each with bullet list of responsibilities + links to its AGENTS.md
    - **Dependency Direction**: ASCII diagram showing inward-flowing deps
    - **Adding a New Feature**: 1-paragraph pointer to the `overseer-feature` skill
    - **Persistence Model**: brief note on JSON write-through + XDG paths + atomic writes + last-writer-wins
    - **Stub Mode**: explanation of which adapters are stubbed and why
    - **ADRs**: links to all 4 ADRs

  **Must NOT do**:
  - ❌ Do NOT duplicate ADR content here — link out
  - ❌ Do NOT include code samples (architecture.md is for orientation, not how-to)

  **Recommended Agent Profile**: `writing`. Skills: none.

  **Parallelization**: Wave 7; parallel with T28, T30.

  **References**: All AGENTS.md, all ADRs, this plan's Context section.

  **Acceptance Criteria**:
  - [ ] File exists at `docs/architecture.md`
  - [ ] Has all 8 sections listed above
  - [ ] Links to all 4 ADRs
  - [ ] Links to root + 4 per-layer AGENTS.md
  - [ ] Total length ≤ 200 lines (concise orientation doc, not exhaustive)

  **QA Scenarios**:

  ```
  Scenario: All cross-references resolve
    Tool: Bash
    Steps:
      1. Run: grep -oE '\\(docs/adr/[0-9]+[^)]+\\)' docs/architecture.md | sort -u | wc -l
    Expected Result: "4"
    Evidence: .sisyphus/evidence/task-29-adr-links.txt

  Scenario: AGENTS.md links present
    Tool: Bash
    Steps:
      1. Run: grep -c 'AGENTS.md' docs/architecture.md
    Expected Result: ≥ 5
    Evidence: .sisyphus/evidence/task-29-agents-links.txt
  ```

  **Evidence**: task-29-adr-links.txt, task-29-agents-links.txt + architecture.md copy

  **Commit**: `docs(arch): add architecture overview with diagrams`

- [ ] 30. **Project README.md**

  **What to do**:
  - Create `README.md` at repo root with:
    - **Title + tagline**: Overseer — TUI for managing AI agent sessions
    - **Status**: "Bootstrap — stub mode" with a note that real tmux/git/agent integration is post-bootstrap
    - **Quick Start**: clone, `make build`, `./bin/overseer`
    - **Keybindings**: table of all keybindings (from help registry)
    - **Configuration**: link to config example and XDG paths
    - **Architecture**: 1-paragraph + link to `docs/architecture.md`
    - **Contributing / Adding Features**: link to `.claude/skills/overseer-feature/SKILL.md`
    - **License**: TBD note (placeholder, not picked yet)

  **Must NOT do**:
  - ❌ Do NOT include extensive screenshots / asciinema (later)
  - ❌ Do NOT include CI badges (no CI yet)
  - ❌ Do NOT promise features beyond what bootstrap delivers

  **Recommended Agent Profile**: `writing`. Skills: none.

  **Parallelization**: Wave 7; parallel with T28, T29.

  **References**: This plan, T24's keybindings, docs/architecture.md (T29).

  **Acceptance Criteria**:
  - [ ] `README.md` exists at root
  - [ ] Status section clearly labels "Bootstrap — stub mode"
  - [ ] Keybindings table has ≥ 8 rows (matches help registry)
  - [ ] Links to architecture.md and overseer-feature skill

  **QA Scenarios**:

  ```
  Scenario: README has required sections
    Tool: Bash
    Steps:
      1. Run: grep -c '^## ' README.md
    Expected Result: ≥ 7
    Evidence: .sisyphus/evidence/task-30-readme-sections.txt
  ```

  **Evidence**: task-30-readme-sections.txt + README.md copy

  **Commit**: `docs(readme): add project README`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to David and get explicit "okay" before completing.
>
> **Do NOT auto-proceed after verification. Wait for David's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting David's okay.** Rejection or feedback → fix → re-run → present again → wait for okay.

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read this plan end-to-end. For each "Must Have": verify implementation exists (read file, run command, parse JSON output). For each "Must NOT Have": grep codebase for forbidden patterns (e.g., `import.*fx`, `pkg/`, `TODO`, `os.Stderr` in TUI mode) — reject with file:line if found. Check evidence files exist in `.sisyphus/evidence/`. Verify Definition of Done items one by one.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `go build ./...`, `make lint`, `make test`, `make test-integration`. Review all `.go` files for AI slop: `as any` equivalents (`interface{}` overuse, `_ = x` to silence errors), empty error returns, `fmt.Println` debug leaks, commented-out code, unused imports, generic names (`data`, `result`, `item`, `temp`, `helper`). Check `internal/` packages have no `import "fmt"` to stderr in TUI path. Confirm no DI frameworks imported.
  Output: `Build [PASS/FAIL] | Lint [N issues] | Tests [N pass/N fail] | Integration [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Real Manual QA** — `unspecified-high` (no playwright; TUI uses teatest + tmux)
  Build the binary (`make build`). Start fresh: rename existing `$XDG_DATA_HOME/overseer/` if any. Launch in tmux session, capture every QA scenario from every task: Create flow with a real session, Rename it, Reorder J/K, switch panes with Tab, quit with `q`. Then test edge cases: launch with corrupted JSON (manually write invalid JSON first), launch with no JSON file, launch with invalid YAML config, launch in 40×10 terminal. Save evidence (terminal captures via tmux) to `.sisyphus/evidence/final-qa/`. Run the skill's self-test script and confirm a Delete feature is generated with green tests + lint.
  Output: `Scenarios [N/N pass] | Edge Cases [N tested] | Skill Self-Test [PASS/FAIL] | VERDICT`

- [ ] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do" + "Must NOT do", read actual diff (`git log --oneline` + `git diff main...HEAD`). Verify 1:1 — everything in spec was built (no missing items), nothing beyond spec was built (no creep, no Delete feature snuck in, no extension points pre-built). Specifically check: no 4th feature, no expand/collapse, no real tmux/git/agent code, no `pkg/`, no DI framework, no observability, only 4 ADRs (not more), `bubbles/help` integrated, atomic writes present. Flag any unaccounted changes.
  Output: `Tasks [N/N compliant] | Forbidden patterns [CLEAN/N issues] | Unaccounted [CLEAN/N files] | ADR count [N (must be 4)] | VERDICT`

---

## Commit Strategy

> All commits use Conventional Commits. Commit per task (not per file). Pre-commit verification: `make test && make lint` must pass.

**Wave 1**:
- T1: `chore(scaffold): initialize Go project with .gitignore .gitattributes .golangci.yml`
- T2: `chore(build): add Makefile with build/test/lint/run targets`
- T3: `feat(shared): add XDG path helpers and error types`
- T4: `test(testutil): scaffold teatest helpers, golden file setup, mock template`
- T5: `feat(tui): introduce Lipgloss styles registry`
- T6: `docs(rules): add root and per-layer AGENTS.md hierarchy`
- T7: `docs(adr): record 4 architectural decisions`

**Wave 2**:
- T8: `feat(domain): introduce Session entity with ports`
- T9: `feat(storage): implement JSON storage with atomic writes and corruption recovery`
- T10: `feat(config): implement YAML config loader with defaults`
- T11: `feat(logger): wire slog to XDG log file`
- T12: `feat(stubs): add stub tmux/git/agent adapters with canned responses`

**Wave 3**:
- T13: `feat(session): implement CreateSession use case`
- T14: `feat(session): implement RenameSession use case`
- T15: `feat(session): implement ReorderSession use case`
- T16: `feat(session): implement ListSessions use case`

**Wave 4**:
- T17: `feat(tui): add SessionsList pane with project grouping`
- T18: `feat(tui): add StatusBar with stub values`
- T19: `feat(tui): add PreviewPane with stub content`
- T20: `feat(tui): add CreateSessionForm modal`
- T21: `feat(tui): add RenameSessionForm modal`
- T22: `feat(tui): integrate bubbles/help with keybinding registry`

**Wave 5**:
- T23: `feat(tui): compose dashboard with focus management and keyboard routing`
- T24: `feat(cmd): wire composition root with XDG setup and startup flow`

**Wave 6**:
- T25: `docs(skill): add overseer-feature skill with Feature Shape Catalog`
- T26: `docs(skill): document Delete feature worked example`
- T27: `docs(skill): add self-test script + README + VERSION + CHANGELOG`

**Wave 7**:
- T28: `test(e2e): add end-to-end dashboard teatest scenarios`
- T29: `docs(arch): add architecture overview with diagrams`
- T30: `docs(readme): add project README`

---

## Success Criteria

### Verification Commands

```bash
# Build succeeds
make build  # Expected: ./bin/overseer exists, exit 0

# All tests pass
make test               # Expected: ok everywhere, 0 failed
make test-integration   # Expected: ok everywhere, 0 failed

# Linting clean
make lint  # Expected: 0 issues

# Binary launches
./bin/overseer &
PID=$!
sleep 1
# Send 'q' to quit gracefully
kill -INT $PID
# Expected: clean exit, no terminal corruption

# JSON file created with correct schema
test -f "$XDG_DATA_HOME/overseer/data.json" || test -f "$HOME/.local/share/overseer/data.json"
# Expected: file exists, valid JSON, has fields {schemaVersion, sessions}

# Skill self-test
.claude/skills/overseer-feature/scripts/self_test.sh
# Expected: generates Delete feature, runs make test && make lint, both green

# Grep for forbidden patterns (zero matches expected)
! grep -r "go.uber.org/fx" --include="*.go" .
! grep -r "github.com/google/wire" --include="*.go" .
! test -d pkg/
! grep -rn "os.Stderr" internal/adapters/primary/ --include="*.go"

# Count ADRs (must be exactly 4)
test "$(ls docs/adr/*.md | wc -l)" -eq 4

# Count AGENTS.md (must be exactly 5: root + 4 per-layer)
test "$(find . -name 'AGENTS.md' -not -path './.git/*' | wc -l)" -eq 5
```

### Final Checklist

- [ ] All "Must Have" items implemented and verified
- [ ] All "Must NOT Have" items absent (grep + manual scan)
- [ ] `make build && make test && make test-integration && make lint` all green
- [ ] Binary launches; all keybindings work as spec'd
- [ ] All teatest scenarios green with golden files committed
- [ ] JSON file structure verified at `$XDG_DATA_HOME/overseer/data.json`
- [ ] Corruption recovery verified (rename + start fresh)
- [ ] Terminal-too-small fallback verified at 40×10
- [ ] Empty state verified (no sessions yet hint)
- [ ] `overseer-feature` skill self-test produces working Delete feature
- [ ] 4 ADRs present, no more
- [ ] 5 AGENTS.md files present, each with explicit MUST / MUST NOT
- [ ] Evidence files captured for every QA scenario
- [ ] David's explicit "okay" received after F1-F4 reviews
