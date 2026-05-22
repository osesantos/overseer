# HANDOFF — Unified Session Creation

## USER REQUESTS (AS-IS)

- "I want to create a new feature to add support for creating new sessions from existing branches. The branches can be remote branches or local branches somewhere. Can you discuss the UX and the UI of this new feature?"
- "In the end, if both workflows can be merged, I would be happy. I just don't know how to do it properly with a good UX. Let's discuss."
- "Let's go with option B. You can drop the kind, and you don't need to worry about migrations because this is still a work in progress and no one is using this for real. Just make the right thing, even if it takes more time and it's harder."
- (Locked the two-mode model:) "We have a single form for everything, and then we have a toggle on the form that says whether or not to create a work tree. This is enabled by default."
- "1. Create a new, brand new work tree from a given base, and it's an isolated working environment. 2. Attached to the project directory without anything else, without doing branching, without doing work trees."
- (On the third case "worktree on existing branch":) "I'm still not clear what it would cover" — agreed to DROP it.
- (On cache:) "the cache can be refreshed every hour, for instance. Let's put it at a big time interval to not spam APIs. On the pop-up, if we see that the cache is too old, we can trigger a refresh"
- (On timer constants:) "all of these refresh timers, put them as constants so I know where to toggle them if I need to extend or reduce the frequency of the jobs."

## GOAL

Implement the unified single-form session-creation feature: drop `SessionKind`, drop the dual `n`/`o` keybindings + checkout form, replace with one form that has a "Create worktree?" toggle (default ON). Add a branch picker with a hot cache. No migration. TDD inside-out.

## CONTEXT

Working in **Overseer.TUI** — Bubble Tea TUI managing dev sessions via git worktrees. Hexagonal architecture (`internal/core/domain`, `internal/core/service`, `internal/adapters/primary/tui`, `internal/adapters/secondary/{git,storage,github,tmux}`). Branch: `feat/session-from-existing-branch`. No uncommitted source changes; clean to start.

**MANDATORY skills to load on session start**: `bubbletea-designer`, `bubbletea-maintenance` (per AGENTS.md). Also load `overseer-add-feature` — it defines the inside-out 7-step workflow that MUST be followed.

User addresses as "David". Refer to him as David.

## LOCKED DESIGN DECISIONS (NON-NEGOTIABLE)

1. **Drop `SessionKind` entirely**. No `SessionKindFeature`, no `SessionKindCheckout`, no `Kind` field on `Session`, no `IsCheckout()` method, no `NewCheckoutSession()`. No migration code — David explicitly said "no one is using this for real."

2. **Two modes via one boolean** (`HasWorktree()` derived from `WorktreePath != ""`):
   - **Mode 1 (Worktree, default)**: `git worktree add -b <new-branch> <path> <base>`. Isolated dir.
   - **Mode 2 (Project)**: no worktree. Session points at the project's working directory. Agent/shell/editor launch in `project.Path`. The "branch" is read live from `git -C <project.Path> rev-parse --abbrev-ref HEAD`.

3. **Final `Session` struct shape**:
   ```go
   type Session struct {
       ID, Name      // unchanged
       ProjectID     // unchanged
       Order, Label  // unchanged
       Branch        string  // Mode 1 only; empty in Mode 2
       WorktreePath  string  // Mode 1 only; empty in Mode 2
       AgentCommand, EditorCommand
       CreatedAt, UpdatedAt
   }
   func (s Session) HasWorktree() bool { return s.WorktreePath != "" }
   ```
   `BaseBranch` is a service-layer creation-time parameter ONLY. Not stored. Not in the struct.
   `FeatureBranch` is renamed to `Branch`.
   No `LocalBranch` / no duality.

4. **Single keybinding `n`** for new session. **Remove `o` entirely** (delete `checkoutBranchKeyBinding`). No replacement.

5. **Single form** (`session/create_form.go`). Polymorphic — fields hide/show based on the "Create worktree?" toggle:
   - Toggle ON: Name, Repository, Toggle, Base branch (picker), New branch name (textinput, optional → auto-gen), Launcher, Editor
   - Toggle OFF: Name, Repository, Toggle, Launcher, Editor

6. **Branch picker** is a new component (`session/branch_picker.go`):
   - Inline expanded list with type-to-filter
   - Per-row: scope glyph (`●` local / `↓` remote) + name + age
   - Reads from cache; renders instantly

7. **Branch cache**:
   - Lives on the dashboard model: `cachedBranchesByProject map[uuid.UUID]projectBranchCache`
   - Populated on TUI startup: fan out `ListBranchesCmd` per project
   - Periodic refresh via `tea.Tick`
   - On popup open: if cache > threshold old, fire a refresh; stale-while-revalidate (render stale immediately, update silently when fresh arrives)

8. **Timer constants in a dedicated file** `internal/adapters/primary/tui/dashboard/tuning.go`:
   ```go
   const (
       BranchCacheRefreshInterval = 1 * time.Hour
       BranchCacheStaleThreshold  = 5 * time.Minute
   )
   ```
   ALL refresh/timing values go here. NO magic numbers in handlers.

9. **Mode 2 visual differentiation in dashboard list**: leading `· ` (middle dot) glyph prefix on the session name. Mode 1 sessions have no prefix. Decision lives in `session/model.go` `sessionTreeNode` — easy to swap later for icon/dim-style/suffix when we evaluate post-implementation.

10. **No auto-fetch**, no `ctrl+r` refresh key in v1. `ListBranches` is local-only (`git for-each-ref`). The hourly tick + stale-on-popup-open is the entire refresh policy.

11. **Tracking semantics**: pass NO extra flags to `git worktree add -b NAME <path> <base>`. Git's defaults handle it correctly:
    - Base is local ref → no upstream (correct for new features)
    - Base is remote ref → auto-upstream (correct for Day-2 continue-on-new-laptop)

12. **PR lookup** (`cmd/overseer/main.go:162-165`): use `sess.Branch` for Mode 1, read project's HEAD dynamically for Mode 2.

## FILE INVENTORY

### Delete entirely
- `internal/adapters/primary/tui/session/checkout_branch_form.go` (+ test if present)
- `internal/shared/paths/paths.go` → delete `SessionTrackingBranch` (keep `SessionFeatureBranch` for Mode 1 auto-gen)

### Change (full list)
- `internal/core/domain/session.go` — biggest rework
- `internal/core/domain/session_test.go`
- `internal/core/service/session.go` — drop `CheckoutBranch`, two-mode `Create`, add `ListBranches`
- `internal/core/service/session_test.go`
- `internal/adapters/secondary/git/adapter.go` — drop `CreateTrackingWorktree`, add `ListBranches`
- `internal/adapters/secondary/git/adapter_test.go`
- `internal/adapters/secondary/storage/store.go` — drop `Kind`/`BaseBranch`/`FeatureBranch` persistence + legacy migration, add `Branch`
- `internal/adapters/secondary/storage/store_test.go`
- `internal/adapters/primary/tui/session/create_form.go` — polymorphic toggle-driven form
- `internal/adapters/primary/tui/session/create_form_test.go`
- `internal/adapters/primary/tui/session/bindings.go` — may add picker keys
- `internal/adapters/primary/tui/session/model.go` — drop `checkoutGlyph` branch; add `· ` for Mode 2
- `internal/adapters/primary/tui/sessiondetails/view.go` — uniform rendering + Mode 2 dynamic branch read
- `internal/adapters/primary/tui/dashboard/bindings.go` — drop `o`
- `internal/adapters/primary/tui/dashboard/root.go` — drop checkout popup state, add cache wiring + tick handler
- `internal/adapters/primary/tui/shared/messages.go` — drop `CheckoutBranchPopupCloseMsg`, add `BranchesLoadedMsg{ProjectID, Branches, Err, LoadedAt}` + `BranchCacheTickMsg`
- `cmd/overseer/main.go` — PR lookup
- `internal/testutil/mocks/mock_GitAdapter.go` — regenerate via `make mockery` (or per `.mockery.yml`)

### Add
- `internal/adapters/primary/tui/session/branch_picker.go` (+ test)
- `internal/adapters/primary/tui/dashboard/tuning.go` (constants file)
- `internal/core/domain/session.go` → add `BranchInfo` struct + `GitAdapter.ListBranches` port

## IMPLEMENTATION ORDER (Inside-Out per overseer-add-feature skill)

Follow strict TDD: **RED tests first at each layer**, then GREEN, then refactor.

### 1. Domain (`session.go`, `session_test.go`)
- RED: tests for new `Session` shape, `HasWorktree()`, `AssignWorktree(worktreePath, branch)` new signature, `BranchInfo` struct, `GitAdapter.ListBranches` port shape
- GREEN: implement
- Drop `Kind`/`IsCheckout`/`NewCheckoutSession`/`SessionKind*`
- Drop `BaseBranch`/`FeatureBranch`, add `Branch`
- Drop `CreateTrackingWorktree` from `GitAdapter` port

### 2. Service (`service/session.go`, `service/session_test.go`)
- RED: tests for `Create` with `CreateWorktree=true` and `CreateWorktree=false`; `ListBranches`
- GREEN: implement
- Drop `CheckoutBranch`, `CheckoutBranchRequest`, `CheckoutBranchResponse`
- `CreateSessionRequest` gets `CreateWorktree bool`, `Branch string`, `BaseBranch string` (transient)
- In `Create`: branch on `CreateWorktree`. False path skips git ops; persists Mode 2 session.
- Helper `func sessionWorkingDir(sess Session, project Project) string` for attach sites.
- Add `ListBranches` method (wraps adapter call).

### 3. Git adapter (`secondary/git/adapter.go`, `adapter_test.go`)
- Integration test (`t.TempDir() + git init`) for `ListBranches`
- Implement `ListBranches`: `git for-each-ref --format='%(refname:short)|%(objecttype)|%(committerdate:iso-strict)' refs/heads refs/remotes`; parse; distinguish local vs remote-tracking; drop `HEAD` symbolic refs
- Delete `CreateTrackingWorktree` method

### 4. Storage (`secondary/storage/store.go`, `store_test.go`)
- Delete `legacyOpenBranchKind` constant + migration branch
- Drop persistence of `Kind`/`BaseBranch`/`FeatureBranch`; add `Branch`
- Round-trip test for new struct
- Delete legacy-migration test

### 5. Mocks
Regenerate via `.mockery.yml` (drop `CreateTrackingWorktree`, gain `ListBranches`).

### 6. TUI feature package (`primary/tui/session/`)
- Delete `checkout_branch_form.go` (+ test)
- Build `branch_picker.go` (+ test): inline list, type-to-filter, scope glyph + age per row
- Rework `create_form.go`: toggle field, polymorphic field set, picker integration, auto-fill name
- Update `model.go`: drop `checkoutGlyph` branch; add `· ` prefix for `!HasWorktree()` sessions
- Update `sessiondetails/view.go`: uniform rendering for Mode 1; Mode 2 reads `git -C project.Path rev-parse --abbrev-ref HEAD` (cache the result with short TTL to avoid spawning git on every render)

### 7. Dashboard composition (`primary/tui/dashboard/`)
- Create `tuning.go` with the constants
- `root.go`: drop `popupCheckoutBranch` + the `o` handler; add `cachedBranchesByProject` field; on Init fan out `ListBranchesCmd` per project; handle `BranchesLoadedMsg`/`BranchCacheTickMsg`; on `n` keypress check staleness and fire refresh if needed
- `bindings.go`: drop `checkoutBranchKeyBinding` (+ from `sessionsKeyBindings`)
- `shared/messages.go`: drop `CheckoutBranchPopupCloseMsg`; add `BranchesLoadedMsg`, `BranchCacheTickMsg`

### 8. CLI entry (`cmd/overseer/main.go`)
PR lookup branches on `HasWorktree()` — Mode 1 uses `sess.Branch`, Mode 2 reads project HEAD.

### 9. End-to-end verification
- `go build ./...` exit 0
- `lsp_diagnostics` clean across all changed files
- Full test suite passes
- **MANUAL TUI DRIVE in real tmux** via `interactive_bash`: launch binary, press `n`, create one Mode 1 session, one Mode 2 session, verify both work end-to-end. Verify branch picker shows real branches from cache. Verify Mode 2 attach opens shell in project dir.

## CRITICAL CONSTRAINTS

- **No `lipgloss.NewStyle()` inline**; all styles from `*styles.Styles`.
- **All service calls wrapped in `tea.Cmd` closures**; never call services synchronously from `Update()`.
- **One typed message per async result** in `messages.go`.
- **`var _ domain.Port = (*Impl)(nil)`** compile-time check on every secondary adapter.
- **`paths.AtomicWrite`** for file mutations.
- **Wrap errors with `fmt.Errorf("ctx: %w", err)`**; never use deprecated `errs.Wrap`.
- **Domain sentinels returned unwrapped** for `errors.Is`.
- **TDD: write tests FIRST (RED), then implement (GREEN), then refactor.**

## OPEN QUESTIONS (decide during implementation, don't block)

- Mode 2 PR lookup performance: cache project HEAD reads with short TTL in dashboard tick, or read on every render? (Lean: cache 5s.)
- Auto-gen branch name format: keep current `overseer/session-<uuid>` from `paths.SessionFeatureBranch`, or simpler like `overseer/<sanitised-session-name>`? (Lean: keep current.)
- Branch picker UX details (sorting order, filter shortcut keys, error rendering): decide as you build; ask David if uncertain.

## WHAT TO DO ON FIRST TURN OF NEW SESSION

1. Load mandatory skills: `bubbletea-designer`, `bubbletea-maintenance`, `overseer-add-feature`.
2. Read `AGENTS.md` and skim `internal/core/domain/session.go` + `internal/adapters/primary/tui/session/create_form.go` to ground yourself.
3. Create a todo list mirroring the 9 implementation steps above.
4. Begin at Step 1 (Domain) with RED tests first.

## WHAT TO NOT DO

- Don't write migration code for old `Kind` values. David explicitly waived it.
- Don't introduce `LocalBranch` field or any branch duality.
- Don't add `--no-track` or `--track` flags to `git worktree add` — git's defaults are correct.
- Don't preserve any "checkout" terminology in user-facing strings.
- Don't repurpose the `o` keybinding for anything.
- Don't batch tests after implementation. RED first.
- Don't proactively commit; only commit when David explicitly asks.
