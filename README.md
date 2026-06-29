# Overseer

[![Go Version](https://img.shields.io/badge/go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Tests](https://github.com/dnlopes/overseer/actions/workflows/build.yaml/badge.svg)](https://github.com/dnlopes/overseer/actions/workflows/build.yaml)

> A terminal-based dashboard for managing AI agent coding sessions with Git worktree isolation, tmux integration, and real-time session monitoring.

Overseer is a TUI application that helps developers organize, launch, and monitor AI agent sessions across multiple Git repositories. It provides isolated worktree-based sessions, project management, pull request tracking, and live session preview — all from your terminal.

---

## Table of Contents

- [Features](#features)
- [Screenshots](#screenshots)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Usage](#usage)
- [Overseer Agent Loop](#overseer-agent-loop)
- [Contributing](#contributing)
- [License](#license)

---

## Features

### Session Management
- **Worktree Sessions**: Create isolated git worktrees on new branches for each AI agent session. Automatically forks from a base branch, sets up tmux sessions, and keeps your main working directory clean.
- **Project Sessions**: Attach sessions directly to a project's working directory without creating worktrees — perfect for quick experiments or when you don't need branch isolation.
- **Session Grouping**: Sessions are automatically grouped by project in a collapsible tree view.
- **Labels**: Color-coded labels to categorize and visually distinguish sessions.
- **Ordering**: Reorder sessions within projects to prioritize your work.

### Terminal Integration
- **Tmux Integration**: Automatically creates and manages tmux sessions for both the AI agent and a shell in the session's working directory.
- **One-Key Attach**: Attach to a session's tmux window directly from the dashboard.
- **Editor Launch**: Open a session's working directory in your configured editor (VSCode, Neovim, etc.).
- **Extended Key Support**: Overseer automatically enables tmux's `extended-keys` so modifier sequences like `Shift+Enter` are preserved inside the agent and shell sessions.

### Real-Time Monitoring
- **Live Session Preview**: Toggle between Agent and Shell stream views to see what your AI agent is doing in real-time.

### Overseer Chat Panel
- **Agent Mode**: Chat with a Claude-backed meta-agent that can read session output and send prompts to your sessions on your behalf.
- **Operator Mode**: Type any message starting with `/` to execute a direct command without involving the LLM (see [Operator Commands](#operator-commands)).
- **Scrollable History**: The chat history is fully scrollable — use `↑`/`↓` to scroll line by line or `PgUp`/`PgDn` to jump a full page. New messages auto-scroll to the bottom unless you have scrolled up to read history.
- **Background Loops**: Start an evaluation loop that periodically checks a session's agent pane against your acceptance criteria and sends follow-up prompts until the goal is met.

### Project Discovery
- **Autodiscovery**: Configure one or more root directories and Overseer will scan their immediate subdirectories at startup, automatically registering any Git repositories it finds. A non-intrusive toast notification shows progress, and a warning popup highlights any configured paths that could not be found on disk.

### Developer Experience
- **Keyboard-Driven**: Vim-inspired keybindings — navigate with `j`/`k`, create with `n`, delete with `d`, and more.
- **Customizable**: YAML configuration for themes, launchers, editors, labels, and dashboard dimensions.
- **Fast & Lightweight**: Built in Go with minimal resource usage.

---

## Screenshots

### Dashboard & Session Details

The main dashboard shows your projects and sessions in a three-pane layout. Select a session to see its details in the middle pane — repository info, branch, and linked pull request status with CI checks.

![Dashboard](docs/screenshots/01-dashboard.png)

### Session Creation

Create a new session with a guided form. Choose between worktree mode (isolated branch) or project mode (direct working directory attachment).

![Create Session](docs/screenshots/02-create-session.png)

### Help Menu

Press `?` at any time to see all available keyboard shortcuts.

![Help Menu](docs/screenshots/04-help-menu.png)

---

## Installation

### Prerequisites

- **tmux** (for session management)
- **gh** (GitHub CLI — optional, for PR tracking)
- A configured **AI agent launcher** (e.g., [Claude Code](https://docs.anthropic.com/en/docs/claude-code), [OpenCode](https://github.com/opencode-ai/opencode), or custom)

### Homebrew (Recommended)

```bash
brew tap dnlopes/overseer
brew install overseer
```

### Install to $GOPATH/bin

```bash
go install github.com/dnlopes/overseer/cmd/overseer@latest
```

### From Source

Requires **Go 1.24+**.

```bash
git clone https://github.com/dnlopes/overseer.git
cd overseer
make build
```

The binary will be available at `bin/overseer`.

---

## Quick Start

### 1. Launch Overseer

```bash
overseer
```

On first run, Overseer creates a default configuration file at:
- macOS: `~/Library/Application Support/overseer/config.yaml`
- Linux: `~/.config/overseer/config.yaml`

### 2. Register a Project

Press `n` to create a new session. If you haven't registered any projects yet, Overseer will prompt you to add a Git repository path.

Alternatively, configure `projectDiscovery` paths (see [Configuration](#configuration)) and Overseer will auto-register repos from those directories at startup — no manual registration needed.

### 3. Create a Session

Fill out the session creation form (navigate fields with `Tab` / `Shift+Tab` or `↑` / `↓`):

| Field | Default | Notes |
|-------|---------|-------|
| **Name** | — | Descriptive name for the session |
| **Repository** | — | Choose from registered projects |
| **Create worktree** | Off | Enable for isolated branch-based sessions |
| **Base branch** | — | Branch to fork from (e.g., `main`); shown when worktree is on |
| **New branch** | — | Branch name for this session; auto-generated if left empty |
| **Launcher** | Claude Code | AI agent to run in the session |
| **Editor** | VSCode | Editor to open the session directory |

### 4. Attach to Session

Select a session and press `Enter` to attach to its tmux window. Press `Tab` to toggle between the Agent and Shell views. Press `e` to open the session directory in your editor.

---

## Configuration

Overseer is configured via a YAML file. Here's a full example:

```yaml
# config.yaml
theme: dark
disableEmoji: false

dashboard:
  minWidth: 60
  minHeight: 15
  previewRefreshInterval: 500ms

logging:
  level: info

storage:
  dataDir: ""  # Uses OS default if empty

# Scan these directories at startup and auto-register any Git repos found.
# Only immediate subdirectories are inspected (no recursive walk).
# ~ is expanded to your home directory.
projectDiscovery:
  paths:
    - ~/code
    - ~/projects

launchers:
  - displayName: "Claude Code"
    command: "claude"
    agentType: "claude-code"
  - displayName: "OpenCode"
    command: "opencode"
    agentType: "opencode"

editors:
  - displayName: "VSCode"
    command: "code"
  - displayName: "Neovim"
    command: "nvim"

labels:
  - code: "urgent"
    color: "#ff6b6b"
    glyph: "!"
  - code: "wip"
    color: "#feca57"
    glyph: "⋯"
  - code: "review"
    color: "#54a0ff"
    glyph: "👁"
  - code: "done"
    color: "#1dd1a1"
    glyph: "✓"
```

### Configuration Options

| Section | Option | Description |
|---------|--------|-------------|
| `theme` | — | UI theme (`dark` or `light`) |
| `disableEmoji` | — | Set to `true` to disable emoji glyphs |
| `dashboard` | `minWidth` | Minimum terminal width required |
| `dashboard` | `minHeight` | Minimum terminal height required |
| `dashboard` | `previewRefreshInterval` | How often to refresh the preview pane |
| `logging` | `level` | Log level (`debug`, `info`, `warn`, `error`) |
| `storage` | `dataDir` | Directory for Overseer's data files (must be absolute) |
| `projectDiscovery` | `paths` | List of directories to scan at startup for Git repos |
| `launchers` | — | List of AI agent launchers (`displayName`, `command`, `agentType`) |
| `editors` | — | List of code editors (`displayName`, `command`) |
| `labels` | — | Custom labels for session categorization |

#### Project Discovery

When `projectDiscovery.paths` is set, Overseer scans each listed directory at startup:

- Only **immediate subdirectories** are inspected (one level deep, no recursion).
- Directories that are not valid Git repositories are silently skipped.
- Already-registered repos are skipped without duplication.
- A **toast notification** appears in the top-right corner of the dashboard while scanning, and briefly shows how many new repos were found.
- If any configured path **does not exist on disk**, a warning popup is shown at startup listing the missing paths. Press `Enter` or `Esc` to dismiss it.

---

## Usage

### Keyboard Shortcuts

#### Sessions List

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `Shift+↓` | Reorder session down |
| `Shift+↑` | Reorder session up |
| `n` | Create new session |
| `d` | Delete selected session |
| `r` | Rename selected session |
| `l` | Cycle through labels |
| `Enter` | Attach to session (agent or shell, based on active inspector tab) |
| `e` | Open session in editor |
| `Ctrl+E` | Send Enter keystroke to the agent pane |
| `x` | Kill the preview tmux session |
| `g` / `G` | Go to next / previous project group |

#### Session Creation Form

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Next / previous field |
| `↓` / `↑` | Next / previous field |
| `←` / `→` | Cycle option (launcher, editor, worktree toggle) |
| `Space` | Toggle worktree on/off |
| `e` | Paste a path into the repository field |
| `Enter` | Create session |
| `Esc` | Cancel |

#### Inspector (Preview Pane)

| Key | Action |
|-----|--------|
| `Tab` | Toggle between Agent and Shell views |

#### Overseer Chat Panel

Open / close the chat panel with `Ctrl+O` from anywhere in the dashboard.

**Input modes** — auto-detected per message:

| Prefix | Mode | Behaviour |
|--------|------|-----------|
| _(none)_ | Agent | Message sent to the Claude meta-agent (LLM) |
| `/` | Operator | Parsed as a slash command; no LLM call |

**Operator commands:**

| Command | Description |
|---------|-------------|
| `/send <session> <prompt…>` | Send a prompt directly to a session's agent pane |
| `/delete <session>` | Open the delete-session confirmation dialog |
| `/new` | Open the new-session creation form |
| `/list` | Print all active sessions inline |
| `/loop <session> <criteria…>` | Start a background evaluation loop |
| `/loop stop <session>` | Stop a running loop |
| `/loop info <session>` | Print loop status, iteration count, and criteria |
| `/help` | List all available commands |

**Scrolling the chat history:**

| Key | Action |
|-----|--------|
| `↑` | Scroll history up one line |
| `↓` | Scroll history down one line |
| `PgUp` | Scroll history up one page |
| `PgDn` | Scroll history down one page |

New messages auto-scroll to the bottom only if you were already at the bottom. Scrolling up to read history freezes the view until you scroll back down.

**While the chat panel is open**, `↑`/`↓` scroll the chat history. To navigate the session list at the same time, use `j`/`k` instead.

#### Global

| Key | Action |
|-----|--------|
| `Ctrl+O` | Toggle Overseer chat panel |
| `?` | Show help menu |
| `q` / `Ctrl+C` | Quit |

### Session Modes

#### Worktree Session

Creates a fully isolated development environment:
- Forks a new branch from the base branch
- Creates a git worktree for the new branch
- Spawns tmux sessions (agent + shell) in the worktree directory
- Safe to run long-running or experimental agent tasks
- Easy cleanup: deleting the session removes the worktree

#### Project Session (default)

Attaches directly to the project's working directory:
- No branch or worktree manipulation
- Uses the project's current HEAD
- Useful for quick experiments or when you don't need isolation

---

## Overseer Agent Loop

The `/loop` command starts a background evaluation loop that runs `claude -p` directly in the session's working directory until the task is complete — or a safety limit is hit.

### Starting a loop

```
/loop overseer-improvements all failing tests are now passing
```

Overseer will:
1. Run `claude -p --dangerously-skip-permissions` in the session's working directory with your criteria as the task.
2. Automatically append the sentinel instruction: "When you have finished this task, write the word END on its own line as your final message."
3. If **`END` is found in the output**: the loop stops and marks the task done.
4. If **not done**: wait 5 seconds, then run again.
5. Repeat up to **40 iterations** (safety cap). Stops early after 3 consecutive errors.

### Loop status

- The session row in the list shows a badge: 🔄 running · ✅ done · ⏹ stopped.
- The **Session Details** pane shows a Loop section with status, iteration count, elapsed time, and criteria.
- The chat panel prints a dimmed system message after each iteration and a final summary when the loop ends.

### Stopping a loop manually

```
/loop stop overseer-improvements
```

### Checking loop status

```
/loop info overseer-improvements
```

---

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on how to get started.

Quick links:
- [Bug Reports](https://github.com/dnlopes/overseer/issues/new?template=bug_report.md)
- [Feature Requests](https://github.com/dnlopes/overseer/issues/new?template=feature_request.md)
- [Pull Requests](https://github.com/dnlopes/overseer/pulls)

---

## License

[MIT](LICENSE) © David Lopes
