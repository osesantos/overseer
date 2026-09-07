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
- **Swarm sessions** — 2–8 agents on one goal, coordinating through a shared message board served over loopback HTTP; Overseer wakes idle agents via tmux because agent CLIs do not poll

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
│   │   │   ├── boardhttp/  # HTTP board API for swarm agent processes (inbound adapter)
│   │   │   └── tui/       # Bubble Tea TUI (primary adapter)
│   │   │       ├── components/     # Pure rendering functions (TUI-01)
│   │   │       ├── dashboard/      # Root model — wires all panes; root.go, commands.go, bindings.go
│   │   │       ├── inspector/      # Right pane: preview tabs with polling + the swarm Agents Board
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
│   │       ├── swarmboard/         # JSONL swarm board + bootstrap descriptor writer
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
- `Session`, `Project`, `Label`, `SwarmMessage` aggregates
- Port interfaces: `SessionRepository`, `TmuxAdapter`, `GitAdapter`, `OverseerAgentPort`, `SwarmBoardRepository`, `SwarmDescriptorWriter`
- Overseer types: `LoopState`, `LoopStatus`, `OverseerMessage`, `OverseerAction`, `OverseerSessionContext`
- Swarm types: `SwarmMessage`, `SwarmRole`, `SwarmDescriptor`; bounds `SwarmMinAgents` / `SwarmMaxAgents`
- `ScanForEnd(paneOutput string) bool` — detects the `END` sentinel in loop task output (domain-layer utility)
- `InferAgentType(agentCommand string) AgentType` — maps a legacy session's agent command string to a typed `AgentType` (lives in `domain/agent_type.go`)
- `SwarmAgentAuthor(index int) string` — the canonical board author name for an agent (`agent-3`)
- Sentinel errors: `ErrTmuxSessionNotFound`, `ErrOverseerAgentNotFound`, `ErrSessionNotASwarm`, `ErrSwarmBoardCapReached`, etc.

**Agent tmux naming lives here and nowhere else.** `Session.AgentTmuxID(index)` returns `<uuid>-agent` for a single-agent session and `<uuid>-agent-<index>` for a swarm; `AgentTmuxIDs()` enumerates all of them. Never concatenate `"-agent"` by hand — that convention used to be duplicated across six call sites and is now centralised.

### `internal/core/service`
Use-case layer. Each file owns one aggregate:
- `session.go` — `SessionService`: Create, Rename, Delete, List, Reorder, AttachAgent, AttachShell, SendAgentPrompt, PreviewSession. Agent-targeting methods take an `AgentIndex` (ignored by single-agent sessions).
- `project.go` — `ProjectService`: Register, Rename, List
- `overseer.go` — `OverseerService`: Chat, EvaluateLoop
- `swarm.go` — `SwarmService`: Post, ListMessages, FlushNudges, Bootstrap

#### Why `SwarmService` splits posting from nudging

Agent CLIs do not loop — they finish a turn and idle at their prompt — so nothing would ever make agent 3 read what agent 2 wrote. Overseer has to poke them by typing into their tmux panes.

`Post` only records that a nudge is *due*; `FlushNudges` performs it, driven by the `swarm-nudge-flush` scheduler job at `swarm.nudgeDebounce`. That split keeps timing out of the service (so it stays deterministically testable) and coalesces a burst of posts into one round of pokes — the main defence against a swarm nudging itself into an endless conversation. The most recent author is skipped, since they just spoke.

Do **not** reintroduce timers inside the service; the flush interval is the debounce window.

#### Why briefing and submitting are separate

`Bootstrap` types each agent's briefing but deliberately does **not** send Enter. A pane exists milliseconds after creation, so tmux accepts the keystroke — but the agent CLI is still booting and swallows it, leaving the agent holding a prompt it never submitted. That was a real bug: every swarm looked stalled on creation.

`SubmitBriefings` sends the Enter, called by the same flush job, retrying `briefingSubmitAttempts` rounds. Enter is safe to repeat (submits a queued prompt, harmless newline otherwise), which is what makes a blind retry acceptable.

This is deliberately **retry, not readiness detection**. Modelling readiness would mean either importing the `claudecode` patterns into the service (breaks ARCH-01) or extending the status detectors, which currently hardcode pane index 0 and cannot target agent N. If you add per-pane status detection, this is the first thing worth revisiting.

`SessionService.SendAgentEnterAll` is the operator-facing version of the same primitive, exposed as `ctrl+e` (tab-aware) and `/enter <session>`.

#### Board verbosity is a prompt problem, not a validation problem

Left ungoverned, agents write multi-kilobyte essays per post — a real five-agent run averaged ~3KB across 38 posts, which makes the board unreadable at exactly the moment it matters most.

Two levers, and only one of them is the right one:

- **`boardConventions`** in `secondary/swarmboard/board.go` — the posting rules, shipped in the descriptor so an agent can re-read them. This is where to tune verbosity. Its sibling **`boardMethod`** governs *how agents work* rather than how they write it up; keep the two lists separate so it is obvious which you are changing.
- **`swarmContentMaxLen`** (8000) in `domain/swarm.go` is a *sanity guard*, not a style control. Do not lower it to force brevity: a rejected post is work the agent already paid for, and it will just retry.

The rules deliberately target **prose, not evidence**. The tables and held-out numbers inside those long posts are what caught a bad task premise in the run that motivated this; compressing them away would trade the feature's main benefit for a shorter scroll.

Both prompts (`swarmBootstrapPrompt`, `swarmNudgePrompt`) also carry a one-line brevity clause, because agents drift back to essays over a long session. Keep them short — they go through `tmux send-keys`, where a long string is the brittle part. There are length assertions on both.

#### `boardMethod` is derived from measurements, not prompt folklore

Each rule answers something observed on a real run, and the motivating measurement is in a comment beside it. Two worth knowing before you edit them:

- **Falsify-before-posting.** Across two runs, the one where agents attacked their own claims 11 times had 5 reversals; the one where they never did had 10. Pre-testing a claim roughly halves correction churn — this is the highest-value rule in the list.
- **Post a verdict unprompted.** Neither run did. The second run's review artefact exists only because the operator asked for it two thirds of the way in; left alone, a swarm keeps opening findings rather than concluding.

If you add a rule, tie it to something you measured on a board. The failure mode here is accumulating plausible-sounding instructions that dilute the ones that work.

### `internal/adapters/primary/tui/dashboard`
Root Bubble Tea model. Owns:
- All pane layout and sizing (`root.go`)
- All global key bindings (`bindings.go`)
- Operator slash-command execution (`commands.go`)
- Background loop management (`commands.go`: `startLoopTaskCmd`, `handleLoopTaskCompleted`)
- Swarm bootstrapping after creation (`root.go`: `bootstrapSwarmCmd`)

#### Key routing — read this before adding a key

**The inspector has no focus model and only receives keys the dashboard explicitly hands it.** `inspector.SetFocus(true)` is never called; the only focused pane is the left one. If you add an inspector key, you must forward it from `handleKey`, the way `tab`, `[` / `]` and `i` already are — otherwise `handleKey` or the left pane will consume it first and your key will appear dead.

Routing order in `Update`, which matters:

1. **Popup gate** — an open popup owns every key.
2. **Capture gate** — `m.inspector.CapturesInput()` (board compose) owns every key except `hardQuitKeyBinding`. This sits ahead of `ctrl+o` and quit on purpose: a half-typed message must not be cut short by a global shortcut.
3. `ctrl+o` — toggles the chat panel.
4. **Chat branch** — when the chat is visible, only `hardQuitKeyBinding` quits; arrows pass through to the session list, `tab` to the inspector, everything else to the chat input.
5. `quitKeyBinding` — plain `q` or `ctrl+c`.
6. `handleKey`, then fallthrough to the left pane.

`quitKeyBinding` is `q`/`ctrl+c`; `hardQuitKeyBinding` is `ctrl+c` only. The split exists because `q` is a character the user means to type when an input owns the keyboard — matching the combined binding too early is what used to quit the app mid-sentence in the chat panel.

#### Paste is a separate message type

`tea.PasteMsg` is **not** a `tea.KeyPressMsg`. Any `Update` that only switches on key presses silently drops paste — which is exactly why you could not paste into the chat panel or the board's compose line at all. A model with a `textinput` must forward `tea.PasteMsg` explicitly; `textinput` itself knows how to consume it.

Paste is routed in `root.go` *before* the key branch and follows the same ownership rules as typing: the board's compose line first, then the chat panel, otherwise dropped. It must land in **exactly one** input — the fallthrough broadcast would otherwise deliver it to the chat and the board simultaneously when both are open.

### `internal/adapters/primary/tui/overseer`
Chat panel model. Handles:
- Auto-detection of Agent mode (`» `) vs Operator mode (`$ `) based on `/` prefix
- Viewport-backed scrollable message history
- Spinner during LLM/command processing
- `OverseerRoleUser` / `OverseerRoleAgent` / `OverseerRoleSystem` message rendering

### `internal/adapters/primary/tui/inspector`
Right-pane preview with generation-counter-based polling (prevents chain doubling on `ForceRefreshMsg`).

Holds **five** views at fixed indexes — `ixAgent`, `ixBoard`, `ixSwarmAgent`, `ixShell`, `ixEditor` — and renders a subset chosen by `visibleIxs()`:

| Session type | Visible tabs |
|--------------|--------------|
| Ordinary | `Agent` · `Shell` · `Editor` (once revealed) |
| Swarm | `Agents Board` · `Agent N/M` · `Shell` · `Editor` (once revealed) |

Things to know before editing this package:

- `activeIx` indexes `views`, **not** the visible subset. The visible set is no longer a prefix, so anything iterating tabs must walk `visibleIxs()` — a bare `for i := range n` will render the wrong labels.
- `ixEditor` must stay last so `RevealEditorMsg` and the size loop need no knowledge of the swarm tabs.
- `ActivePreviewKind() (service.PreviewKind, int, bool)` is the **only** way to learn what pane the active tab shows. Never map tab *labels* back to a kind: that is exactly how `x` on the Agents Board used to silently kill the shell.
- `CapturesInput()` reports that the board's compose line owns the keyboard; `CanCompose()` reports that the compose key would do something. The dashboard consults both.
- `swarmBoardLoadedMsg` / `swarmBoardPostedMsg` are routed to `ixBoard` explicitly, not to `activeIx`, so the board absorbs the tail of its own polling chain after the user tabs away.

### `internal/adapters/primary/boardhttp`
Inbound HTTP adapter exposing a swarm session's board to the **agent processes**, which live outside Overseer. Requests flow inward through `SwarmService` like the TUI's do (ARCH-05).

The TUI does **not** use this server — it shares the process and reads the board through the service directly. There is no loopback round-trip and no WebSocket.

Binds loopback on an ephemeral port and has **no authentication**; that is deliberate for a single-developer machine and the reason it must not be exposed.

### `internal/adapters/secondary/swarmboard`
Implements both `SwarmBoardRepository` and `SwarmDescriptorWriter` over a per-session directory under the data dir (never inside the user's repo).

- The board is append-only JSONL, one record per line, each carrying its own `v` schema version — a file header could not be rewritten without losing the crash-safety of a plain append (SEC-05 satisfied per record).
- `Append` holds a mutex across sequence assignment *and* the write, so `Seq` stays gapless under concurrent posts from several agents. Reads take the same lock, so they never observe a half-written line.
- A malformed or partially-written line is **skipped with a warning**, never quarantined or deleted. This is a deliberate reading of SEC-06: a torn append must not cost the operator the rest of the conversation.

### `internal/adapters/secondary/claude`
Implements `OverseerAgentPort`. Invokes `claude -p <prompt>` as a subprocess.
- `Chat`: parses `<action>{...}</action>` fence for structured actions; uses `overseerRequestTimeout = 60s` because LLM calls routinely exceed 30s
- `RunLoopTask`: runs `claude -p --dangerously-skip-permissions <criteria>` in the session's working directory and returns raw stdout; no timeout (subprocess runs until `claude` exits naturally); the dashboard scans output with `domain.ScanForEnd` to detect task completion

---

## Markdown Rendering in Chat Panels

Both the Overseer chat panel and the swarm Agents Board render message bodies as markdown via `styles.Markdown` (`charm.land/glamour/v2`).

Rules if you touch either panel:

- **Glamour lives in `styles/`.** It has its own stylesheet system, so it is wrapped by `Styles.NewMarkdown(width)` rather than configured in feature code — otherwise TUI-03 ("all styles from `*styles.Styles`") is dead letter.
- **The stylesheet is pinned to `"dark"`, not `WithAutoStyle`.** Auto-detection probes the terminal background, which makes output depend on where the process runs and breaks golden tests.
- **Bodies are rendered once and cached.** Each panel keeps a `rendered []string` parallel to its message slice. Re-rendering the whole transcript per incoming message — which is what the plain-text version did — does not scale with glamour.
- **The two slices must stay index-for-index.** Anything that clears messages must clear `rendered` too; a session switch that cleared only one paired the new session's posts with the old session's bodies.
- **Only a width change invalidates the cache.** `applySizes`/`SetSize` also run on mode changes, which must not pay for a full re-render — hence the `markdown.Width() != vpWidth` guard.
- **System notices bypass markdown.** They are Overseer's own one-liners carrying paths and errors that reflowing would mangle.
- **`Render` never returns empty for non-empty input.** A glamour failure degrades to the raw text; a message must never vanish because it could not be prettified.

### The board appends by Seq, not by trust

The board polls deltas, and **the `generation` counter cannot deduplicate them**. Posting fires an immediate fetch while the scheduled poll is still in flight; both carry the *same* generation and the *same* cursor, so both return the same message and the guard sees nothing wrong. That shipped as a visible bug — the operator's own post rendered twice until a session switch rebuilt the list.

Two invariants hold it together, and both have regression tests:

1. **Append is idempotent.** A message with `Seq <= latestSeq` is skipped. Never append a delta on the assumption it is new.
2. **The cursor only moves forward.** A slow chain replaying an old delta must not rewind `latestSeq`.

`Seq` is also user-facing: `renderHeader` leads with `#N` because agents cite each other by it in the large majority of posts, and `boardConventions` tells them to use the same `#12` notation. Keep the field padded — an unpadded number ragged the timestamp column once the board passes 9 messages.

`swarmBoardPostedMsg` also bumps `generation` so the immediate fetch *replaces* the scheduled poll rather than racing it. That is only safe because the replacement reschedules — get it wrong and the board freezes after the first post, which is what `TestBoardView_PostRetiresTheOldPollWithoutStoppingPolling` exists to catch.

### Bordered-panel width

`lipgloss` treats a bordered style's `Width` as the **total** including the border glyphs, so the usable interior is `Width-2`. The chat panel's viewport was sized to the full width and so ran two columns wider than the box it drew into — invisible with plain text, obvious once messages gained a full-width separator. See `borderSideW` in `tui/overseer/model.go`.

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
go vet ./...            # must always be clean
go test ./... -race     # pre-existing tui/session mock failures are known; all others must pass
```

**Regenerating mocks** — `mockery` is not vendored. Add the interface to `.mockery.yml`, then:

```bash
go run github.com/vektra/mockery/v3@v3.5.1
```

This rewrites every mock, so expect cosmetic diffs in unrelated mock files if the pinned version differs from whatever generated the committed ones.

**Do not run `gofmt -w` on a whole directory.** Several committed files carry pre-existing formatting drift (`domain/agent_type.go`, `service/overseer.go`, `tui/session/bindings.go`, and others). A directory-wide format sweeps them into your diff and buries the actual change. Format only the files you edited.

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

## Adding a swarm-aware feature

Swarm sessions break several assumptions that held when every session had exactly one agent pane. If you touch anything agent-related, check all of these:

| Assumption | What to do instead |
|------------|--------------------|
| Pane name is `<uuid>-agent` | Use `Session.AgentTmuxID(index)` / `AgentTmuxIDs()` |
| A session has one agent | Loop `Session.AgentCount()` |
| The active tab implies a pane | Ask `inspector.ActivePreviewKind()`; it returns `false` for the Agents Board |
| One status per session is enough | `AgentStatusService.pollOne` reports `AgentStatusSwarm` — a distinct kind, *not* `Unknown`. Unknown means detection failed; this means it does not apply. Aggregating N panes needs a rule nobody has specified yet |
| Teardown kills three tmux sessions | `SessionService.Delete` kills shell + every agent pane + editor |
| Deleting a session removes all its data | The board **outlives** the session by default. `Delete` only purges it when `DeleteSessionRequest.PurgeBoard` is set, which the delete form arms with `p`. Board dirs under `<dataDir>/swarm/` therefore accumulate; nothing prunes them |

---

## Known Pre-Existing Test Failures

The `internal/adapters/primary/tui/session` package has 4 failing tests related to `CreateWorktree` mock expectations:

- `TestCreateForm_DefaultsToCreateWorktreeOn`
- `TestCreateForm_TabCyclesThroughWorktreeFields`
- `TestCreateForm_SpaceOnToggleSwitchesMode`
- `TestCreateForm_SubmitWorktreeMode_PassesPickedBaseBranch`

These are pre-existing and unrelated to agent work. Do not attempt to fix them unless explicitly asked. To confirm a failure is pre-existing rather than yours, test a clean tree instead of stashing:

```bash
TMPWT=$(mktemp -d)/baseline
git worktree add -q --detach "$TMPWT" HEAD
(cd "$TMPWT" && go test ./...)
git worktree remove --force "$TMPWT"
```

### Known test-helper footguns

Several dashboard commands bail out early when `leftPane.SelectedSessionID()` is empty. A test that only sends `SessionSelectedMsg` does **not** populate the list, so the command returns `nil` and the assertion passes without ever running the code under test. Use `selectInDashboard` (in `key_routing_test.go`), which sends `SessionsLoadedMsg` *and* `SessionSelectedMsg` and asserts the selection took.



`focusIdxOf` in `tui/session/create_form_test.go` returns `0` — not `-1` — for a field that is not in the focus order. Asserting `>= 0` to test for presence therefore always passes, and focusing a missing field silently focuses **Name** instead. Use an explicit `slices.Contains` check (see `hasField` in `create_form_swarm_test.go`).
