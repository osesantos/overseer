# Overseer

> TUI for managing AI agent sessions.

## Status

**Bootstrap — stub mode.** The core TUI, domain, and service layers are complete. Real tmux session management, git integration, and agent launching are post-bootstrap work; all three are currently backed by stubs that return canned responses.

## Quick Start

```sh
git clone https://github.com/dnlopes/overseer.git
cd overseer
make build
./bin/overseer
```

## Terminal Requirements

| Requirement | Details |
|-------------|---------|
| Color support | truecolor (24-bit) recommended; degrades gracefully to 256-color |
| `NO_COLOR` | Respected — disables all ANSI color output |
| minimum size | 60×15 terminal (width×height); smaller shows a "terminal too small" message |

## Keybindings

| Key | Action |
|-----|--------|
| `n` | Create new session |
| `r` | Rename selected session |
| `J` | Move session down |
| `K` | Move session up |
| `Tab` | Switch focus between panes |
| `1` | Focus sessions pane |
| `2` | Focus preview pane |
| `?` | Toggle help |
| `q` / `Ctrl+C` | Quit |

## Configuration

Overseer follows XDG Base Directory conventions:

| Purpose | Default path |
|---------|-------------|
| Config  | `$XDG_CONFIG_HOME/overseer/config.yaml` (`~/.config/overseer/config.yaml`) |
| Data    | `$XDG_DATA_HOME/overseer/data.json` (`~/.local/share/overseer/data.json`) |
| Log     | `$XDG_STATE_HOME/overseer/overseer.log` (`~/.local/state/overseer/overseer.log`) |

The config file is optional; all settings have defaults. Missing files are created on first run.

## Architecture

Overseer is structured around hexagonal architecture with `primary` (TUI) and `secondary` (storage, config, stubs) adapter naming. The domain layer has zero external dependencies; all wiring happens at startup via constructor injection in `cmd/overseer/main.go`. See [`docs/architecture.md`](docs/architecture.md) for the full layer breakdown, directory map, and dependency diagram.

## Contributing / Adding Features

Each feature follows a defined vertical-slice path through domain → service → adapter layers. The step-by-step guide lives in the `overseer-feature` skill: [`.claude/skills/overseer-feature/`](.claude/skills/overseer-feature/).

## License

TBD
