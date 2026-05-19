---
paths:
  - "internal/adapters/primary/tui/**/*.go"
---
# TUI-04: Messages Centralized

## Rule
Global messages and per-feature message structs live in `internal/adapters/primary/tui/shared/messages.go`

## Why
Centralizing messages allows models to subscribe to messages from other features without creating circular dependencies. It also makes it easier to find and understand the messages used across the TUI.

## Example
✅ Good:
```go
// internal/adapters/primary/tui/shared/messages.go
type SessionSelectedMsg struct{ ID string }

```

❌ Bad:
```go
// internal/adapters/primary/tui/session/messages.go
type SessionSelectedMsg struct{ ID string }
```
