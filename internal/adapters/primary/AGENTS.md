# AGENTS.md — Primary Adapters (TUI)

## MUST

- MUST: TUI is the only primary adapter for now
- MUST: Each pane / form is a separate BubbleTea sub-model with its own `Init/Update/View`
- MUST: Keyboard messages are routed only to the focused pane (focus enum)
- MUST: Every feature registers its keybindings in the help registry (`bubbles/help`)
- MUST: All styles come from `internal/adapters/primary/tui/styles.New()`

## MAY

- MAY: Define a `tui/components/` package containing pure renderer functions that consume `*styles.Styles` and return rendered strings. Components MUST NOT call `lipgloss.NewStyle()` or `lipgloss.Color()` directly — they are style-consumers only.

## MUST NOT

- MUST NOT: Call adapters/secondary directly — go through service layer
- MUST NOT: Define new styles outside the styles registry
- MUST NOT: Use `fmt.Print*` or write to stdout (use the logger)
