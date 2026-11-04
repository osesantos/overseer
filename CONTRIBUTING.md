# Contributing to Skipper

Skipper is a personal project, but issues and pull requests are welcome.

## Setup

You need:

- **Go 1.24+**
- **tmux**, for the integration tests and for running the app
- **golangci-lint**, for `make lint`

## Make targets

| Command | What it does |
|---|---|
| `make build` | Build `bin/skipper` and copy it to `~/go/bin` |
| `make install` | `make build`, then copy it to `~/.local/bin` too (`BIN=` changes this) |
| `make run` | Build and start the app |
| `make test` | Unit tests with the race detector |
| `make test-integration` | Integration tests (needs tmux) |
| `make update-golden` | Regenerate golden test files |
| `make mocks` | Regenerate the mocks from `.mockery.yml` |
| `make lint` | Run golangci-lint |
| `make tidy` | `go mod tidy` |
| `make clean` | Remove build output and the test cache |

Before you push, all three of these must be clean:

```bash
go build ./...
go vet ./...
go test ./... -race
```

## Known test failures

Four tests in `internal/adapters/primary/tui/session` fail on `main` and are not yours to fix:

- `TestCreateForm_DefaultsToCreateWorktreeOn`
- `TestCreateForm_TabCyclesThroughWorktreeFields`
- `TestCreateForm_SpaceOnToggleSwitchesMode`
- `TestCreateForm_SubmitWorktreeMode_PassesPickedBaseBranch`

To check that a failure is old and not yours, run the suite on a clean worktree:

```bash
TMPWT=$(mktemp -d)/baseline
git worktree add -q --detach "$TMPWT" HEAD
(cd "$TMPWT" && go test ./...)
git worktree remove --force "$TMPWT"
```

## Layout

Skipper uses a hexagonal (ports and adapters) architecture:

```
.
├── cmd/skipper/              # main.go: wires everything and starts the TUI
├── internal/
│   ├── core/
│   │   ├── domain/           # Types, rules, port interfaces. No I/O
│   │   └── service/          # Use cases, one service per aggregate
│   ├── adapters/
│   │   ├── primary/          # Driving adapters
│   │   │   ├── boardhttp/    # HTTP board API for swarm agents
│   │   │   └── tui/          # Bubble Tea UI
│   │   │       ├── dashboard/    # Root model, key routing, commands
│   │   │       ├── mate/         # First Mate chat panel
│   │   │       ├── pullrequest/  # Full-screen PR page
│   │   │       ├── prs/          # Review inbox
│   │   │       ├── triage/       # Triage inbox
│   │   │       └── ...
│   │   └── secondary/        # Driven adapters: tmux, git, github, jira,
│   │                         # claude, storage, swarmboard, agentstatus
│   ├── shared/               # config, logger, paths
│   └── testutil/             # Golden files, mocks, fixtures
└── .claude/rules/            # The project's architecture rules
```

Dependencies point inward: adapters → services → domain. The domain knows nothing about Bubble Tea, tmux or GitHub.

`boardhttp` is a primary adapter: swarm agents run as separate processes and call into Skipper, so their requests go through the services like the TUI's do.

The rules in `.claude/rules/` (ARCH, SVC, SEC, TUI) are the source of truth. [AGENTS.md](AGENTS.md) summarises them and explains the parts of the code that need care.

## Where to make changes

| What you're changing | Where |
|---|---|
| UI, layout, keys | `internal/adapters/primary/tui/` |
| Rules and validation | `internal/core/domain/` |
| Use cases | `internal/core/service/` |
| External tools | `internal/adapters/secondary/` |
| Config | `internal/shared/config/` |
| Colours and styles | `internal/adapters/primary/tui/styles/` only |

## Tests

- **Unit tests** cover domain rules and services, with the secondary adapters mocked. Table-driven where it fits.
- **Golden tests** snapshot rendered output. After an intended UI change, run `make update-golden` and read the diff.
- **Integration tests** run against real tools and are tagged `//go:build integration`.
- **Mocks** live in `internal/testutil/mocks/` and are generated, never hand-written. Add the interface to `.mockery.yml`, then run `go run github.com/vektra/mockery/v3@v3.5.1`. It rewrites every mock, so keep only the hunks your change explains.

## Formatting

Format only the files you changed. Never run `gofmt -w` on a whole directory: several committed files carry old formatting drift, and a sweep buries your real change in it.

## Commits

Short, lower-case, imperative subjects that say what the change does for the user, for example `reload the PR and triage inboxes on demand with R`.

Releases come from semantic-release on `main`. It only cuts one for Angular-style subjects (`feat: …`, `fix: …`), so use one of those when a change should ship as a release.
