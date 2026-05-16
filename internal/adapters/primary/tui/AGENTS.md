# AGENTS.md — TUI Layer

## Architecture

The TUI layer follows a semantic theme system:

1. **Theme** (`tui/styles/theme.go`, `theme_dark.go`) — 15 semantic color tokens; loaded via `LoadTheme("dark")`
2. **Styles** (`tui/styles/styles.go`) — `New()` builds all styles from the theme; single source of truth
3. **Components** (`tui/components/`) — pure renderer functions that consume `*styles.Styles`; define NO new styles
4. **Titlebar** (`tui/titlebar/`) — stateful sub-model (branding + active pane indicator)

## MUST

- MUST: All styles come from `styles.New()` — never call `lipgloss.NewStyle()` outside the styles package
- MUST: Components in `tui/components/` are pure functions: `func Foo(s *styles.Styles, ...) string`
- MUST: Each pane/form is a separate BubbleTea sub-model with `Init/Update/View`

## MUST NOT

- MUST NOT: Call `lipgloss.NewStyle()` or `lipgloss.Color()` inside `tui/components/` files
- MUST NOT: Add a new theme without updating `LoadTheme()` switch in `theme.go`
- MUST NOT: Add activity-specific color tokens (Waiting/Thinking/etc.) — overseer has no activity domain
