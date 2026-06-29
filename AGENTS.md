# Mandatory rules you must follow all the time

1. at the beggining of the session, you MUST load the `bubbletea-designer` and `bubbletea-maintenance` skills. They provide deep context on how to work with the bubbletea framework.

---

# Overseer — Agent Reference

Everything an agent needs to navigate and extend this codebase without breaking its conventions.

---

## Project Overview

Overseer is a terminal-based dashboard for managing AI agent coding sessions. It is a **Bubble Tea v2** TUI application written in Go, following a strict **hexagonal (ports-and-adapters) architecture**.

Key capabilities:
- Create and manage tmux-backed coding sessions per Git project/worktree
- Real-time preview of agent and shell panes
- Pull-request tracking via GitHub CLI
- Agent-status detection (idle / running / waiting / dead) per session
- Overseer chat panel — an LLM meta-agent (Claude Code) that can control sessions
- Operator slash-commands: `/send`, `/loop`, `/new`, `/delete`, `/list`, `/help`
- Background evaluation loops (`/loop <session> <criteria>`) — runs `claude -p` in the session's working directory; 5s interval between iterations; up to 40 iterations

---

## Architecture Rules

These rules are the single source of truth. **ARCH-10 states: Overseer rules supersede generic skill advice.**

### ARCH rules (cross-layer)

| Rule | Summary |
|------|---------|
| **ARCH-01** | Dependencies flow inward only: secondary → service → domain ← primary. Domain has zero imports of services or adapters. |
| **ARCH-02** | Adapters translate; they don't decide. Validation and invariants live in `domain`, not adapters. |
| **ARCH-03** | Service method signatures use domain types or service-local Request/Response structs — never adapter types. |
| **ARCH-04** | Domain defines port interfaces; secondary adapters implement them. `var _ domain.Port = (*Impl)(nil)` in every adapter. |
| **ARCH-05** | Primary adapters (TUI) call services only — never repos or external systems directly. |
| **ARCH-06** | One `<Aggregate>Service` struct per domain aggregate. Methods are use-cases named with verbs. |
| **ARCH-07** | Error wrapping: `fmt.Errorf("context: %w", err)`. `errs.Wrap` is deprecated — do not use in new code. |
| **ARCH-08** | Compile-time interface conformance: `var _ Iface = (*Impl)(nil)` at package level for every port implementation. |
| **ARCH-09** | `*slog.Logger` injected via constructor. Never `log.Print*` or package-level loggers. |
| **ARCH-10** | Overseer rules WIN when they conflict with generic skill advice. |

### SVC rules (service layer)

| Rule | Summary |
|------|---------|
| **SVC-01** | One service struct per aggregate. |
| **SVC-02** | Request/Response structs for every use-case method (not bare parameter lists). |
| **SVC-04** | Wrap errors with `fmt.Errorf("ctx: %w", err)`; return domain sentinel errors unwrapped. |
| **SVC-05** | `*slog.Logger` injected via constructor; log INFO at use-case boundaries, WARN/ERROR on failures. |
| **SVC-06** | Methods named after business operations (`Create`, `Rename`, `Delete`, `List`, `Reorder`). Never `Save`. |

### SEC rules (secondary adapters)

| Rule | Summary |
|------|---------|
| **SEC-01** | One secondary adapter implements exactly one domain port; no domain logic inside. |
| **SEC-02** | All `os.*`, `net.*`, `exec.*` I/O lives in secondary adapters only. |
| **SEC-03** | One Go package per technology: `storage`, `tmux`, `git`, `claude`, `github`. |
| **SEC-04** | Persist via `paths.AtomicWrite` (write-to-tmp + rename). Never `os.WriteFile` on live user data. |
| **SEC-05** | Every persisted file carries a `schemaVersion` field. |
| **SEC-06** | On parse failure rename the file to `<file>.corrupted.<unix-ts>.json`; never delete user data. |

### TUI rules (primary adapter)

| Rule | Summary |
|------|---------|
| **TUI-01** | `components/` exports pure functions returning `string`. No `Init/Update/View` in components. |
| **TUI-02** | Each feature package: `model.go`, `messages.go` (if needed), `bindings.go`, optional `*_form.go`. |
| **TUI-03** | All lipgloss styles from `*styles.Styles` injected into the model. Never `lipgloss.NewStyle()` inside a component or feature model. |
| **TUI-04** | All cross-feature messages live in `internal/adapters/primary/tui/shared/messages.go`. |
| **TUI-05** | Typed messages, one per async result. No generic `EventMsg` with string discriminator. |
| **TUI-06** | Service calls inside `tea.Cmd` closures. Models never call services synchronously in `Update` or `View`. |
| **TUI-09** | Every top-level model handles `tea.WindowSizeMsg` and renders a "too small" message below configured minimum dimensions. |
| **TUI-10** | Theme structs in `styles/theme.go`, palettes in `styles/theme_<name>.go`. Never hard-code colors outside `styles/`. |
| **TUI-11** | Alt-screen via `altScreenModel` wrapper whose `View()` sets `v.AltScreen = true`. The v1 `tea.WithAltScreen()` option does not exist in Bubble Tea v2. |
| **TUI-12** | Key matching via `key.Matches(msg, binding)`. Never compare raw strings like `msg.String() == "enter"`. |
| **TUI-13** | Declare keybindings as `key.Binding` in a per-feature `bindings.go` file. |

---

## Repository Structure

```
overseer/
├── cmd/overseer/          # main.go — wires dependencies, starts tea.Program
├── internal/
│   ├── core/
│   │   ├── domain/        # Domain types, port interfaces, sentinel errors
│   │   └── service/       # Use-case services (SessionService, ProjectService, OverseerService)
│   ├── adapters/
│   │   ├── primary/
│   │   │   └── tui/       # Bubble Tea TUI (primary adapter)
│   │   │       ├── components/     # Pure rendering functions (TUI-01)
│   │   │       ├── dashboard/      # Root model — wires all panes; root.go, commands.go, bindings.go
│   │   │       ├── inspector/      # Right pane: Agent + Shell preview tabs with polling
│   │   │       ├── jobs/           # Background scheduler (agent-status, PR status, branch cache)
│   │   │       ├── leftpane/       # Left pane: session list + session details
│   │   │       ├── overseer/       # Overseer chat panel (model.go, confirm.go, bindings.go)
│   │   │       ├── session/        # Session list model + create/delete/rename forms
│   │   │       ├── sessiondetails/ # Session details card (repo, PR, loop section)
│   │   │       ├── shared/         # Messages, helpers, emit utilities
│   │   │       └── styles/         # Styles, themes, glyphs
│   │   └── secondary/
│   │       ├── agentstatus/        # Agent-status detectors (claudecode, opencode, registry)
│   │       ├── claude/             # OverseerAgentPort impl — invokes `claude -p`
│   │       ├── git/                # GitAdapter impl — worktree management
│   │       ├── github/             # GitHub CLI adapter — PR status
│   │       ├── storage/            # JSON persistence (atomic writes, schema versioning)
│   │       └── tmux/               # TmuxAdapter impl — session create/kill/capture/send-keys
│   ├── shared/
│   │   ├── config/        # YAML config loader
│   │   ├── errs/          # Error helpers (deprecated wrapper — use fmt.Errorf in new code)
│   │   ├── logger/        # slog setup
│   │   └── paths/         # AtomicWrite, OS data-dir resolution
│   └── testutil/          # Golden files, ANSI strip helper, mock factories
├── .claude/
│   └── rules/             # Project rules (ARCH-*, TUI-*, SVC-*, SEC-*) — authoritative
└── AGENTS.md              # This file
```

---

## Key Packages and Their Roles

### `internal/core/domain`
Pure Go structs and interfaces. No I/O. Defines:
- `Session`, `Project`, `Label` aggregates
- Port interfaces: `SessionRepository`, `TmuxAdapter`, `GitAdapter`, `OverseerAgentPort`
- Overseer types: `LoopState`, `LoopStatus`, `OverseerMessage`, `OverseerAction`, `OverseerSessionContext`
- `ScanForEnd(paneOutput string) bool` — detects the `END` sentinel in loop task output (domain-layer utility)
- `InferAgentType(agentCommand string) AgentType` — maps a legacy session's agent command string to a typed `AgentType` (lives in `domain/agent_type.go`)
- Sentinel errors: `ErrTmuxSessionNotFound`, `ErrOverseerAgentNotFound`, etc.

### `internal/core/service`
Use-case layer. Each file owns one aggregate:
- `session.go` — `SessionService`: Create, Rename, Delete, List, Reorder, AttachAgent, AttachShell, SendAgentPrompt, PreviewSession
- `project.go` — `ProjectService`: Register, Rename, List
- `overseer.go` — `OverseerService`: Chat, EvaluateLoop

### `internal/adapters/primary/tui/dashboard`
Root Bubble Tea model. Owns:
- All pane layout and sizing (`root.go`)
- All global key bindings (`bindings.go`)
- Operator slash-command execution (`commands.go`)
- Background loop management (`commands.go`: `startLoopTaskCmd`, `handleLoopTaskCompleted`)

### `internal/adapters/primary/tui/overseer`
Chat panel model. Handles:
- Auto-detection of Agent mode (`» `) vs Operator mode (`$ `) based on `/` prefix
- Viewport-backed scrollable message history
- Spinner during LLM/command processing
- `OverseerRoleUser` / `OverseerRoleAgent` / `OverseerRoleSystem` message rendering

### `internal/adapters/primary/tui/inspector`
Right-pane preview with generation-counter-based polling (prevents chain doubling on `ForceRefreshMsg`).

### `internal/adapters/secondary/claude`
Implements `OverseerAgentPort`. Invokes `claude -p <prompt>` as a subprocess.
- `Chat`: parses `<action>{...}</action>` fence for structured actions; uses `overseerRequestTimeout = 60s` because LLM calls routinely exceed 30s
- `RunLoopTask`: runs `claude -p --dangerously-skip-permissions <criteria>` in the session's working directory and returns raw stdout; no timeout (subprocess runs until `claude` exits naturally); the dashboard scans output with `domain.ScanForEnd` to detect task completion

---

## Shared Utilities (`internal/adapters/primary/tui/shared`)

| Function | Purpose |
|----------|---------|
| `shared.Emit[T](msg T) tea.Cmd` | Wrap a message as a `tea.Cmd` |
| `shared.Request[T](fn, wrap)` | Async service call with 30s timeout |
| `shared.RequestWithTimeout[T](d, fn, wrap)` | Async service call with custom timeout |
| `shared.UpdateModel[T](m T, msg) (T, tea.Cmd)` | Type-safe model update |
| `shared.Broadcast(msg, forwarders...)` | Fan-out a message to multiple models |
| `shared.Forward[T](m *T) func(tea.Msg) tea.Cmd` | Forwarder factory for Broadcast |

---

## Chat Pane Scroll Behaviour

The Overseer chat panel (`internal/adapters/primary/tui/overseer/model.go`) uses a `charm.land/bubbles/v2/viewport.Model` for scrollable message history.

**Scroll keys** (intercepted before reaching the text input):

| Key | Action |
|-----|--------|
| `↑` | Scroll up one line |
| `↓` | Scroll down one line |
| `PgUp` | Scroll up one page |
| `PgDn` | Scroll down one page |

**Auto-scroll behaviour**: new messages only snap the viewport to the bottom if `viewport.AtBottom()` was true before the message arrived. Scrolling up to read history will not be interrupted by incoming messages.

**Navigation while chat is open**: `↑` / `↓` are also forwarded to the session list via `chatPassthroughNav` so you can change the selected session while the chat panel is visible, but only when the key event is not consumed by the viewport first. (The viewport consumes them; use `j`/`k` in the session list area instead.)

---

## Development Workflow

### Adding a new feature

Use the `overseer-add-feature` skill — it provides the step-by-step hexagonal workflow:

```
1. Define domain types / port method in internal/core/domain/
2. Implement the service use-case in internal/core/service/
3. Implement any new secondary adapter in internal/adapters/secondary/<tech>/
4. Add typed messages to internal/adapters/primary/tui/shared/messages.go
5. Add TUI model/form in the appropriate feature package
6. Wire into dashboard/root.go (handle messages, route commands)
7. go build ./... && go test ./...
```

### Build and test

```bash
go build ./...          # must always be clean
go test ./...           # pre-existing tui/session mock failures are known; all others must pass
```

### Adding a new operator command

1. Add a handler function `cmdFoo(args []string) (tea.Model, tea.Cmd)` in `dashboard/commands.go`
2. Add the case to the `switch name` block in `executeCommand`
3. Update `/help` output in `cmdHelp`
4. Add a key binding to `bindings.go` if the command needs one

### Adding a new slash-command result message

All async results go through `shared.OverseerCommandResultMsg{Text, IsError}` — the chat panel renders them as dimmed system messages with `○ ` prefix. Only create a new message type if the result triggers a state change beyond rendering text in the chat.

---

## Available Skills

Load these at the start of every session (mandatory):

| Skill | When to use |
|-------|-------------|
| `bubbletea-designer` | Designing new TUI components, selecting Charmbracelet components, planning architecture |
| `bubbletea-maintenance` | Debugging existing Bubble Tea code, fixing layout issues, performance problems |
| `overseer-add-feature` | Step-by-step workflow for adding a new feature to Overseer |

---

## Coding Conventions

- **Go version**: 1.24+
- **Module**: `github.com/dnlopes/overseer`
- **Error wrapping**: `fmt.Errorf("context: %w", err)` always; `%v` loses the chain
- **No global state**: no package-level vars except `key.Binding` declarations and compile-time conformance checks
- **Context**: always `context.Context` as the first parameter for I/O operations; use `context.Background()` inside `tea.Cmd` closures
- **UUID**: `github.com/google/uuid` for all IDs
- **Logging**: `slog.InfoContext`, `slog.WarnContext`, `slog.ErrorContext` with structured key-value pairs
- **Tests**: table-driven where possible; use `internal/testutil` helpers; golden files for viewport/render output
- **Mocks**: `internal/testutil/mocks/` — generated with mockery; never hand-write mocks
- **No raw key strings in Update**: `key.Matches(msg, binding)` only (TUI-12)
- **No `lipgloss.NewStyle()` in feature code**: use `*styles.Styles` (TUI-03)
- **All cross-feature messages**: in `shared/messages.go` (TUI-04)

---

## Known Pre-Existing Test Failures

The `internal/adapters/primary/tui/session` package has 4 failing tests related to `CreateWorktree` mock expectations. These are pre-existing and unrelated to Overseer agent work. Do not attempt to fix them unless explicitly asked.
