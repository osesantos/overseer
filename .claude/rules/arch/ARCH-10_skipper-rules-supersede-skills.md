# ARCH-10: Skipper Rules Supersede Generic Skills

## Rule
When `.claude/rules/` content conflicts with `bubbletea-designer` or `bubbletea-maintenance` skill advice, Skipper rules WIN.

## Why
Skills provide generic Bubble Tea guidance; Skipper rules encode project-specific decisions that override generic defaults. Local truth beats generic baseline.

## Example
✅ Good:
```
TUI-03 says "never call lipgloss.NewStyle() inside a component"
→ follow TUI-03 even if a generic skill suggests otherwise
```

❌ Bad:
```
Following bubbletea-designer's generic style advice when TUI-03
explicitly forbids it for this project
```
