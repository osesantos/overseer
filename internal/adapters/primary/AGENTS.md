# AGENTS.md — Primary Adapters (TUI)

## MUST

- MUST: TUI is the only primary adapter for now
- MUST: Each pane / form is a separate BubbleTea sub-model with its own `Init/Update/View`
- MUST: Keyboard messages are routed only to the focused pane (focus enum)
- MUST: Every feature registers its keybindings in the help registry (`bubbles/help`)
- MUST: All styles come from `internal/adapters/primary/tui/styles.New()`

## MUST NOT

- MUST NOT: Call adapters/secondary directly — go through service layer
- MUST NOT: Define new styles outside the styles registry
- MUST NOT: Use `fmt.Print*` or write to stdout (use the logger)
