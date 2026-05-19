---
paths:
  - "internal/adapters/primary/tui/**/*.go"
---
# TUI-13: Keybindings Declaration

## Rule
Models must declare their keybindings as `key.Binding` in a local `bindings.go` file.

## Why
Using `key.Binding` provides a clear, consistent way to define keybindings with metadata (help text, etc.) and allows for better maintainability and readability. It also enables features like dynamic keybinding updates and integration with help menus.

## Example
✅ Good:
```go
// internal/adapters/primary/tui/session/bindings.go
popupNextFieldKeyBinding  = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field"))
popupPrevFieldKeyBinding  = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "previous field"))
popupSubmitFormKeyBinding = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "create session"))
popupCloseKeyBinding      = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel"))
```

❌ Bad:
```go
// internal/adapters/primary/tui/shared/bindings.go
popupNextFieldKeyBinding  = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field"))
popupPrevFieldKeyBinding  = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "previous field"))
popupSubmitFormKeyBinding = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "create session"))
popupCloseKeyBinding      = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel"))
```
