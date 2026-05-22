package session

import (
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
)

const (
	branchScopeGlyphLocal  = "●"
	branchScopeGlyphRemote = "↓"
	branchPickerListHeight = 6
)

type branchPicker struct {
	branches      []domain.BranchInfo
	filtered      []domain.BranchInfo
	defaultBranch string
	filterInput   textinput.Model
	cursor        int
	focused       bool
	styles        *styles.Styles
}

func newBranchPicker(s *styles.Styles, branches []domain.BranchInfo, defaultBranch string, inputWidth int) branchPicker {
	filter := textinput.New()
	filter.Placeholder = "type to filter…"
	filter.CharLimit = 200
	filter.SetWidth(inputWidth)
	filter.SetStyles(s.Form.Input)

	p := branchPicker{
		branches:      sortedBranchInfos(branches, defaultBranch),
		defaultBranch: defaultBranch,
		filterInput:   filter,
		styles:        s,
	}
	p.filtered = p.branches
	return p
}

func (p *branchPicker) setBranches(branches []domain.BranchInfo, defaultBranch string) {
	p.defaultBranch = defaultBranch
	p.branches = sortedBranchInfos(branches, defaultBranch)
	p.applyFilter()
	if p.cursor >= len(p.filtered) {
		p.cursor = max(0, len(p.filtered)-1)
	}
}

// confirmSelection copies the currently-highlighted branch name into the
// filter input so the chosen value stays visible after the user moves on.
// No-op when no branch is currently selected.
func (p *branchPicker) confirmSelection() {
	sel, ok := p.selected()
	if !ok {
		return
	}
	p.filterInput.SetValue(sel.Name)
	p.applyFilter()
	p.cursor = 0
}

func (p *branchPicker) focus() {
	p.focused = true
	p.filterInput.Focus()
}

func (p *branchPicker) blur() {
	p.focused = false
	p.filterInput.Blur()
}

func (p branchPicker) selected() (domain.BranchInfo, bool) {
	if p.cursor < 0 || p.cursor >= len(p.filtered) {
		return domain.BranchInfo{}, false
	}
	return p.filtered[p.cursor], true
}

func (p branchPicker) hasResults() bool {
	return len(p.filtered) > 0
}

func (p *branchPicker) applyFilter() {
	needle := strings.ToLower(strings.TrimSpace(p.filterInput.Value()))
	if needle == "" {
		p.filtered = p.branches
		return
	}
	out := make([]domain.BranchInfo, 0, len(p.branches))
	for _, b := range p.branches {
		if strings.Contains(strings.ToLower(b.Name), needle) {
			out = append(out, b)
		}
	}
	p.filtered = out
}

func (p branchPicker) update(msg tea.Msg) (branchPicker, tea.Cmd) {
	if !p.focused {
		return p, nil
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		if key.Matches(keyMsg, branchPickerUpKeyBinding) {
			if p.cursor > 0 {
				p.cursor--
			}
			return p, nil
		}
		if key.Matches(keyMsg, branchPickerDownKeyBinding) {
			if p.cursor < len(p.filtered)-1 {
				p.cursor++
			}
			return p, nil
		}
	}

	var cmd tea.Cmd
	p.filterInput, cmd = p.filterInput.Update(msg)
	p.applyFilter()
	if p.cursor >= len(p.filtered) {
		p.cursor = max(0, len(p.filtered)-1)
	}
	return p, cmd
}

func (p branchPicker) view() string {
	parts := []string{p.filterInput.View()}
	if len(p.filtered) == 0 {
		parts = append(parts, modalListRow(p.styles, false).Render("  (no branches match)  "))
		return strings.Join(parts, "\n")
	}

	start := 0
	end := len(p.filtered)
	if end > branchPickerListHeight {
		end = branchPickerListHeight
		if p.cursor >= branchPickerListHeight {
			start = p.cursor - branchPickerListHeight + 1
			end = p.cursor + 1
		}
	}
	for i := start; i < end; i++ {
		row := p.renderBranchRow(p.filtered[i], i == p.cursor)
		parts = append(parts, row)
	}
	if len(p.filtered) > branchPickerListHeight {
		parts = append(parts, p.styles.Form.Hint.Render(
			"  "+formatBranchCount(p.cursor+1, len(p.filtered))+"  ",
		))
	}
	return strings.Join(parts, "\n")
}

func (p branchPicker) renderBranchRow(b domain.BranchInfo, selected bool) string {
	glyph := branchScopeGlyphLocal
	if b.Scope == domain.BranchScopeRemote {
		glyph = branchScopeGlyphRemote
	}
	age := ""
	if !b.CommitterDate.IsZero() {
		age = shared.FormatRelativeDuration(time.Since(b.CommitterDate))
	}
	body := glyph + " " + b.Name
	if selected {
		return modalListRow(p.styles, true).Render("  " + body + "  " + age + "  ")
	}
	bg := p.styles.Modal.Box.GetBackground()
	return modalListRow(p.styles, false).Render("  "+body+"  ") + p.styles.ListRow.Aux.Background(bg).Render(age+"  ")
}

func formatBranchCount(cur, total int) string {
	return "▾ " + itoa(cur) + "/" + itoa(total)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

func sortedBranchInfos(in []domain.BranchInfo, defaultBranch string) []domain.BranchInfo {
	out := append([]domain.BranchInfo(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		iDef := defaultBranch != "" && out[i].Name == defaultBranch
		jDef := defaultBranch != "" && out[j].Name == defaultBranch
		if iDef != jDef {
			return iDef
		}
		if out[i].Scope != out[j].Scope {
			return out[i].Scope < out[j].Scope
		}
		return out[i].CommitterDate.After(out[j].CommitterDate)
	})
	return out
}
