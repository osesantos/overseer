# Draft: Overseer Bootstrap Plan

> Working memory for the Overseer project bootstrap planning session.
> This file is updated continuously during the interview.

---

## Project Vision

**Codename**: Overseer
**Type**: Terminal User Interface (TUI) application
**Purpose**: Main developer tool for managing a fleet of AI agent sessions across projects
**Interaction**: Keyboard-only, no mouse
**Deployment**: Local + remote (via tmux)

---

## Long-Term Feature Vision (NOT in initial scope, kept for context)

1. Launch new AI agent sessions (Claude Code, OpenCode, etc.) — 1 session = 1 git worktree
2. Maintain git context per session (base branch, PR status, diff stats)
3. Launch tmux sessions targeting worktrees; agents run in tmux for persistence
4. Run Overseer in remote server, clients connect via tmux
5. Built-in PR-style diff view
6. Fully configuration-driven (all aspects, sensible defaults)
7. Preview tmux sessions without entering them

**Dashboard layout (the core view)**:
```
┌────────────────────────┬───────────────────────────────────┐
│                        │ Status row: working dir, branch,  │
│ Sessions list:         │ PR status, agent status           │
│  - grouped by project  ├───────────────────────────────────┤
│  - or ad-hoc groups    │                                   │
│                        │ Preview pane: streaming tmux      │
│                        │ session output                    │
└────────────────────────┴───────────────────────────────────┘
```

---

## Tech Stack (confirmed)

- **Language**: Go (1.22+)
- **TUI**: BubbleTea (`github.com/charmbracelet/bubbletea`)
- **Styling**: Lipgloss (`github.com/charmbracelet/lipgloss`)
- **Components**: bubbles (`github.com/charmbracelet/bubbles`)
- **Testing**: teatest v2 (`github.com/charmbracelet/x/exp/teatest/v2`)
- **Persistence**: in-memory JSON file (initial)
- **Config**: YAML
- **Architecture**: Hexagonal (Ports & Adapters)

---

## Ground Rules (David's explicit preferences)

### Rule 1: Standardization > YAGNI
> "Code is cheap; extending Overseer with new features must be a walk in the park."
> Even small features must follow the standard process, no exceptions.

**Implication**: Heavy investment in conventions, templates, scaffolding, and rules/skills upfront.

### Rule 2: Architect for Extensibility
**Implication**: Clear abstractions and separation of concerns from day 1.

---

## David's Decisions (confirmed via interview)

| Question | Decision |
|---|---|
| Go module path | `github.com/dnlopes/overseer` |
| MVP session depth | **Stub mode**: JSON records only. No real tmux/git/worktree yet. |
| Bootstrap features | **Create + Rename + Reorder** |
| Config format | **YAML** |
| Test strategy | **TDD with teatest** (RED → GREEN → REFACTOR) |

---

## Research Synthesis

### Hexagonal Architecture (Go)

**Canonical Layout (consensus across 3 production projects: iruldev/golang-api-hexagonal, AngusGMorrison/realworld-go, bxcodec/golang-ddd-modular)**:

```
overseer/
├── cmd/overseer/main.go               # Bootstrap + DI wiring (Composition Root)
├── internal/
│   ├── core/
│   │   ├── domain/{feature}/          # Entities, value objects, errors, ports
│   │   └── service/{feature}/         # Use cases (CreateSession, RenameSession, etc.)
│   ├── adapters/
│   │   ├── primary/tui/               # BubbleTea TUI (driving adapter)
│   │   └── secondary/                 # JSON storage, config, logger (driven adapters)
│   ├── shared/                        # Cross-cutting (logger interface, error utils)
│   └── testutil/                      # Test fixtures, mocks
├── docs/                              # Architecture docs + ADRs
├── .claude/skills/                    # Overseer-specific skills (feature creator)
└── ...
```

**Key Conventions**:
- Use `primary/secondary` (NOT `driving/driven`) — ecosystem consensus
- **Ports defined in domain package** (not adapters)
- **Constructor injection** (no DI framework like fx for a TUI app)
- **Vertical slices**: co-locate feature code (domain/{feature}, service/{feature}, etc.)
- Domain has ZERO external deps; everything flows inward

**References**:
- iruldev/golang-api-hexagonal (1.2k★): observability, fx framework
- AngusGMorrison/realworld-go (2.5k★): type-driven, constructor injection ✅ closest match
- bxcodec/golang-ddd-modular-monolith (1.8k★): feature module pattern ✅ closest match

### BubbleTea Testing

**Recommended Stack**:
- `github.com/charmbracelet/x/exp/teatest/v2` for TUI integration tests
- Standard `go test` + table-driven tests for domain/service layers
- Golden files in `testdata/` for view snapshots
- `.gitattributes` with `*.golden linguist-generated=true -text`

**Test Layering** (matches hexagonal):
- **Domain tests**: pure unit tests, no mocks
- **Use case tests**: with mocked ports (handwritten mocks in `internal/testutil/mocks/`)
- **Adapter tests**: integration tests with real implementations, `//go:build integration` tag
- **TUI tests**: teatest with fixed terminal size, golden file assertions

**Patterns**:
- `teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))`
- `tm.Send(tea.KeyMsg{...})` for keyboard simulation
- `tm.Type("text")` for typing
- `teatest.WaitFor(...)` for async ops
- `teatest.RequireEqualOutput(t, out)` for golden files
- `go test -update` to refresh golden files

**Coverage targets**:
- Domain logic: 90%+
- Use cases: 80-90%
- Update(): 80-90%
- View(): 40-60% (use golden files, not line assertions)
- Overall realistic: 60-70%

### Dashboard Layout Patterns

**Production reference: charmbracelet/soft-serve**
- 3-pane layout via nested `lipgloss.JoinVertical` + `JoinHorizontal`
- Focus management via enum (`type pane int`)
- Route keyboard messages **only to active pane**
- Visual focus indicator: focused border vs hidden/dim border (Lipgloss `lipgloss.RoundedBorder()` vs `lipgloss.HiddenBorder()`)
- Status bar as separate sub-component with `SetStatus()` method

**Layout composition for Overseer dashboard**:
```go
rightPane := lipgloss.JoinVertical(lipgloss.Left,
    statusRow,         // Top: working dir / branch / PR / agent
    previewPane,       // Bottom: tmux stream (viewport)
)
mainView := lipgloss.JoinHorizontal(lipgloss.Top,
    sessionsListPane,  // Left
    rightPane,         // Right (status + preview stacked)
)
```

**For grouped sessions list (Phase 1 — manual grouping, simpler)**:
- Custom render with group headers (`▼` expanded, `▶` collapsed)
- Indented session items under each group
- Selection moves through expanded items, skipping collapsed group contents

**For preview pane** (stubbed for now, real later):
- `viewport.Model` with `HighPerformanceRendering = true`
- `SetContent(string)` to update
- Track `AtBottom()` to auto-scroll new content
- For now: show placeholder "preview not available" since tmux is stubbed

**References**:
- soft-serve repo.go, selection.go, statusbar.go
- glow pager.go (viewport streaming)
- bubbletea split-editors example (border focus styling)

---

## Technical Decisions Locked In

### Architecture
- **Pattern**: Hexagonal (Ports & Adapters)
- **Naming**: `primary` (TUI) and `secondary` (storage, config)
- **DI**: Constructor injection, wired in `cmd/overseer/main.go`
- **Feature organization**: Vertical slices by domain concept

### Stack & Tools
- **Go version**: 1.22+ (for slices/maps stdlib, log/slog)
- **Logging**: stdlib `log/slog`
- **YAML library**: `gopkg.in/yaml.v3`
- **UUID for session IDs**: `github.com/google/uuid`
- **Linting**: `golangci-lint` with reasonable defaults
- **Pre-commit**: NOT in bootstrap scope (add later)

### Persistence (stub mode)
- Single JSON file at `~/.overseer/data.json` (configurable)
- Write-through model: every mutation saves immediately
- In-memory cache, file is source of truth on startup
- File format: versioned schema for forward compatibility

### Testing
- **Unit tests**: alongside source (`foo_test.go`)
- **Integration tests**: `//go:build integration` tag
- **TUI tests**: teatest with golden files in `testdata/`
- **Mocks**: handwritten in `internal/testutil/mocks/`

### "Process to Extend" — deliverable #4 implementation
**Three artifacts to create**:
1. **Root `AGENTS.md`**: high-level architecture, ground rules
2. **Per-layer `AGENTS.md`**: each major directory (domain, service, adapters/primary, adapters/secondary) has its own rules
3. **`.claude/skills/overseer-feature/SKILL.md`**: step-by-step procedure to add a new feature, with file templates and a checklist

---

## Scope Boundaries

### IN scope (bootstrap)
- Go project skeleton (`go.mod`, directory structure, `Makefile`)
- Hexagonal architecture scaffolding (domain, service, adapters)
- Dashboard view (3 panes: sessions list, status, preview)
- 3 features end-to-end: Create / Rename / Reorder session
- Stub adapters for tmux + git (interface only, no real impl)
- JSON file persistence (write-through)
- YAML config loader
- Testing framework (teatest setup, mock framework, sample tests at each layer)
- Standardization artifacts: AGENTS.md hierarchy + overseer-feature skill
- ADRs for major decisions

### OUT of scope (explicitly NOT in bootstrap)
- Real tmux integration (stubbed)
- Real git worktree creation (stubbed)
- Real agent launching (stubbed)
- PR status / diff view
- Remote server mode
- Ad-hoc groups feature (only project-based grouping)
- Authentication / multi-user
- Database (SQLite, Postgres) — JSON only for now
- pre-commit hooks / git hooks
- Release packaging (goreleaser, etc.)

### Guardrails (MUST NOT do)
- ❌ Don't implement real tmux/git/agent integration — keep stubbed
- ❌ Don't use a DI framework (fx, wire) — constructor injection only
- ❌ Don't use TOML or JSON for config — YAML only
- ❌ Don't scatter feature code across layers — use vertical slices
- ❌ Don't add features beyond the 3 specified
- ❌ Don't skip the standardization artifacts even if the features work without them
- ❌ Don't use `pkg/` for internal code — `internal/` is the right place
- ❌ Don't add a global state singleton — everything via DI

---

## Open Questions Resolved

- Go module path: ✅ `github.com/dnlopes/overseer`
- Specific 2-3 features: ✅ Create + Rename + Reorder
- MVP session depth: ✅ Stub mode (JSON records only)
- Configuration format: ✅ YAML
- Testing approach: ✅ TDD with teatest
- "Process to extend" form: ✅ AGENTS.md hierarchy + Overseer feature skill (defaulted, will confirm in summary)
- JSON persistence model: ✅ Write-through (defaulted)
- Go version: ✅ 1.22+ (defaulted)
- Logging: ✅ stdlib log/slog (defaulted)
