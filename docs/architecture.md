# Architecture

## Overview

Overseer is a terminal-based TUI for managing AI agent sessions. It is built on hexagonal architecture — domain types and ports live in the core with zero external dependencies, use cases in the service layer orchestrate the domain via injected ports, and adapters (primary TUI, secondary storage/config/logger/stubs) sit at the edges. The entire dependency graph is wired at startup in `cmd/overseer/main.go` with plain constructor injection. During bootstrap, all infrastructure adapters (tmux, git, agent launcher) are stubs that return canned data, enabling the TUI to be developed and tested without real system integrations.

## Directory Map

```
cmd/
  overseer/               — composition root; wires all deps, starts BubbleTea program
internal/
  core/
    domain/
      session/            — Session entity, ports (Repository/TmuxAdapter/GitAdapter/AgentLauncher), domain errors
    service/
      session/            — Create / Rename / List / Reorder use cases (one struct each)
  adapters/
    primary/
      tui/
        dashboard/        — top-level BubbleTea model; composes all sub-models
        session/          — session list, create form, rename form sub-models
        preview/          — preview pane sub-model
        status/           — status bar sub-model
        help/             — help registry + help-bar sub-model
        styles/           — centralised Lipgloss style registry
    secondary/
      storage/json/       — JSON-backed session repository (atomic writes, corruption recovery)
      config/yaml/        — YAML config loader with defaults
      logger/slog/        — slog JSON logger wired to XDG log file
      tmux/stub/          — stub TmuxAdapter (canned responses)
      git/stub/           — stub GitAdapter (canned responses)
      agent/stub/         — stub AgentLauncher (canned responses)
  shared/
    paths/                — XDG path helpers + AtomicWrite
    errs/                 — sentinel errors (ErrNotFound, ErrNoOp, …) + Wrap/Is
  testutil/
    fixtures/             — shared test session builders
    golden/               — ANSI-stripping golden file helpers
    mocks/                — handwritten port mocks with call counters
    teatest/              — BubbleTea test harness wrapper (fixed terminal sizing)
```

## Layer Responsibilities

### Domain (`internal/core/domain/`)

Pure Go; only stdlib and `github.com/google/uuid` allowed. Defines entities (`Session`), ports (interfaces for Repository, TmuxAdapter, GitAdapter, AgentLauncher), and sentinel domain errors. Zero I/O. → [Domain AGENTS.md](../internal/core/domain/AGENTS.md)

### Service (`internal/core/service/`)

One struct per use case. Constructor-injected ports. Each exposes `Execute(ctx, req) (resp, error)`. Validates the request, drives the domain, coordinates ports in order — domain first, side effects last. No BubbleTea, no adapter imports. → [Service AGENTS.md](../internal/core/service/AGENTS.md)

### Primary Adapters (`internal/adapters/primary/`)

BubbleTea TUI only. Each screen element is an independent sub-model with `Init/Update/View`. The dashboard model composes sub-models and routes keyboard events to the focused pane via a focus enum. Styles come exclusively from the central style registry. → [Primary AGENTS.md](../internal/adapters/primary/AGENTS.md)

### Secondary Adapters (`internal/adapters/secondary/`)

Implement ports defined in the domain. Allowed to import third-party libs; must translate library errors to domain errors at the boundary. Must never leak library types to callers. Stub adapters provide full interface satisfaction with canned responses. → [Secondary AGENTS.md](../internal/adapters/secondary/AGENTS.md)

## Dependency Direction

```
cmd/overseer (composition root)
        │
        ├──▶ adapters/primary/tui ──calls──▶ service/session
        │
        └──▶ adapters/secondary/* ◀──injects── service/session
                                                      │
                                                      ▼
                                           core/domain/session
                                           (ports + entities)
```

No layer imports anything above it. `cmd/overseer/main.go` is the only place that knows about all layers simultaneously.

## Adding a New Feature

New features follow a vertical-slice path: domain entity/port → service use case → primary TUI adapter(s), optionally a new secondary adapter. The complete step-by-step procedure — including file names, test patterns, AGENTS.md update requirements, and help-registry wiring — is documented in the `overseer-feature` skill at [`.claude/skills/overseer-feature/`](../.claude/skills/overseer-feature/).

## Persistence Model

Session state is stored as a single JSON file at `$XDG_DATA_HOME/overseer/data.json` (default: `~/.local/share/overseer/data.json`). All writes are atomic: data is written to a temp file in the same directory, then renamed over the target. On load, a corrupted file is renamed to `data.json.corrupted.<unix>.json` and the application starts fresh with an empty state. The model is last-writer-wins with no background sync — the TUI holds full in-memory state and flushes on every mutation. XDG path resolution is centralised in `internal/shared/paths`.

## Stub Mode

Three infrastructure adapters are intentionally stubbed for the bootstrap phase:

- **`tmux/stub`** (`TmuxAdapter`): returns a fixed session ID without launching a real tmux session.
- **`git/stub`** (`GitAdapter`): returns a fixed branch name without running git commands.
- **`agent/stub`** (`AgentLauncher`): acknowledges launch without starting a real agent process.

Stubs exist so the TUI, domain, and service layers can be fully built and tested without system integration. Real adapters will replace stubs in a post-bootstrap phase. See [ADR 0002](adr/0002-stub-mode-for-bootstrap.md).

## ADRs

| # | Title |
|---|-------|
| [ADR 0001](adr/0001-hexagonal-architecture-with-primary-secondary-naming.md) | Hexagonal Architecture with Primary/Secondary Naming |
| [ADR 0002](adr/0002-stub-mode-for-bootstrap.md) | Stub Mode for Bootstrap |
| [ADR 0003](adr/0003-json-file-with-atomic-write-through.md) | JSON File with Atomic Write-Through |
| [ADR 0004](adr/0004-tdd-with-teatest-for-tui-layer.md) | TDD with Teatest for TUI Layer |
