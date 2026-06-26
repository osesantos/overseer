package session

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
)

type repoPickerMode int

const (
	repoPickerModeList repoPickerMode = iota
	repoPickerModePaste
	repoPickerModeSearch
)

const repoPickerSearchListHeight = 6

// repoPicker chooses a repository for a new session. In list mode the user
// cycles through previously-used repos (recent first); a sentinel entry
// "+ New repo by path..." switches the picker to paste mode for a fresh path.
// Pressing "/" enters search mode: a live filter input narrows the list and
// Enter confirms the highlighted project immediately.
type repoPicker struct {
	mode        repoPickerMode
	projects    []domain.Project
	listIdx     int
	pasteInput  textinput.Model
	filterInput textinput.Model
	filtered    []domain.Project
	searchIdx   int
	focused     bool
	styles      *styles.Styles
}

func newRepoPicker(s *styles.Styles, projects []domain.Project, initialProjectID uuid.UUID, inputWidth int) repoPicker {
	sorted := append([]domain.Project(nil), projects...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].UpdatedAt.After(sorted[j].UpdatedAt)
	})

	pasteInput := textinput.New()
	pasteInput.Placeholder = "/absolute/path/to/repo"
	pasteInput.CharLimit = 500
	pasteInput.SetWidth(inputWidth)
	pasteInput.SetStyles(s.Form.Input)

	filterInput := textinput.New()
	filterInput.Placeholder = "search repos…"
	filterInput.CharLimit = 200
	filterInput.SetWidth(inputWidth)
	filterInput.SetStyles(s.Form.Input)

	listIdx := 0
	if initialProjectID != uuid.Nil {
		for i, p := range sorted {
			if p.ID == initialProjectID {
				listIdx = i
				break
			}
		}
	}

	return repoPicker{
		mode:        repoPickerModeList,
		projects:    sorted,
		listIdx:     listIdx,
		pasteInput:  pasteInput,
		filterInput: filterInput,
		filtered:    sorted,
		styles:      s,
	}
}

func (p *repoPicker) focus() {
	p.focused = true
	if p.mode == repoPickerModePaste {
		p.pasteInput.Focus()
	} else if p.mode == repoPickerModeSearch {
		p.filterInput.Focus()
	}
}

func (p *repoPicker) blur() {
	p.focused = false
	p.pasteInput.Blur()
	p.filterInput.Blur()
}

func (p repoPicker) isPasteMode() bool {
	return p.mode == repoPickerModePaste
}

// itemCount returns total cyclable items in list mode: projects + sentinel.
func (p repoPicker) itemCount() int {
	return len(p.projects) + 1
}

// onSentinel reports whether the current list-mode selection is the
// "+ New repo by path..." sentinel entry.
func (p repoPicker) onSentinel() bool {
	return p.listIdx == len(p.projects)
}

// selectedProject returns the currently highlighted existing project, or nil
// if the sentinel is selected or there are no projects.
// In search mode, returns the project at searchIdx within the filtered slice.
func (p repoPicker) selectedProject() *domain.Project {
	if p.mode == repoPickerModeSearch {
		if p.searchIdx < 0 || p.searchIdx >= len(p.filtered) {
			return nil
		}
		return &p.filtered[p.searchIdx]
	}
	if p.onSentinel() {
		return nil
	}
	if p.listIdx < 0 || p.listIdx >= len(p.projects) {
		return nil
	}
	return &p.projects[p.listIdx]
}

// pastedPath returns the trimmed paste-mode input. Empty when not in paste
// mode or the field is blank.
func (p repoPicker) pastedPath() string {
	if p.mode != repoPickerModePaste {
		return ""
	}
	return strings.TrimSpace(p.pasteInput.Value())
}

func (p *repoPicker) cycle(direction int) {
	n := p.itemCount()
	if n <= 0 {
		return
	}
	p.listIdx = ((p.listIdx+direction)%n + n) % n
}

// enterPasteMode switches to paste mode and focuses the text input.
func (p *repoPicker) enterPasteMode() {
	p.mode = repoPickerModePaste
	p.pasteInput.SetValue("")
	p.pasteInput.Focus()
	p.filterInput.Blur()
}

func (p *repoPicker) exitPasteMode() {
	p.mode = repoPickerModeList
	p.pasteInput.Blur()
}

// enterSearchMode switches to search mode. The filter is cleared and all
// projects are shown; the cursor starts at 0.
func (p *repoPicker) enterSearchMode() {
	p.mode = repoPickerModeSearch
	p.filterInput.SetValue("")
	p.applyFilter()
	p.searchIdx = 0
	p.filterInput.Focus()
	p.pasteInput.Blur()
}

// exitSearchMode returns to list mode, syncing listIdx to the project that was
// highlighted in search mode (so the cycler stays consistent).
func (p *repoPicker) exitSearchMode() {
	if proj := p.selectedProject(); proj != nil {
		for i, pr := range p.projects {
			if pr.ID == proj.ID {
				p.listIdx = i
				break
			}
		}
	}
	p.mode = repoPickerModeList
	p.filterInput.Blur()
}

// applyFilter rebuilds p.filtered from p.projects using the current filter
// input value. searchIdx is clamped to stay within the new slice bounds.
func (p *repoPicker) applyFilter() {
	needle := strings.ToLower(strings.TrimSpace(p.filterInput.Value()))
	if needle == "" {
		p.filtered = p.projects
	} else {
		out := make([]domain.Project, 0, len(p.projects))
		for _, pr := range p.projects {
			if strings.Contains(strings.ToLower(pr.Name), needle) {
				out = append(out, pr)
			}
		}
		p.filtered = out
	}
	if p.searchIdx >= len(p.filtered) {
		p.searchIdx = max(0, len(p.filtered)-1)
	}
}

// adoptRegisteredProject appends a newly-registered project to the list and
// selects it, returning the picker to list mode. Called by the create form
// after silently registering a path the user pasted.
func (p *repoPicker) adoptRegisteredProject(project domain.Project) {
	p.projects = append([]domain.Project{project}, p.projects...)
	p.filtered = p.projects
	p.listIdx = 0
	p.exitPasteMode()
}

func (p repoPicker) update(msg tea.Msg) (repoPicker, tea.Cmd) {
	if !p.focused {
		return p, nil
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch p.mode {
		case repoPickerModeList:
			if key.Matches(keyMsg, popupSelectorNextKeyBinding) {
				p.cycle(1)
				return p, nil
			}
			if key.Matches(keyMsg, popupSelectorPrevKeyBinding) {
				p.cycle(-1)
				return p, nil
			}
			if key.Matches(keyMsg, repoPickerEnterPasteKeyBinding) {
				p.enterPasteMode()
				return p, textinput.Blink
			}
			if key.Matches(keyMsg, repoPickerEnterSearchKeyBinding) {
				p.enterSearchMode()
				return p, textinput.Blink
			}
			return p, nil

		case repoPickerModeSearch:
			if key.Matches(keyMsg, branchPickerUpKeyBinding) {
				if p.searchIdx > 0 {
					p.searchIdx--
				}
				return p, nil
			}
			if key.Matches(keyMsg, branchPickerDownKeyBinding) {
				if p.searchIdx < len(p.filtered)-1 {
					p.searchIdx++
				}
				return p, nil
			}
			// Esc: exit search mode; consume the key so the form doesn't close.
			if key.Matches(keyMsg, popupCloseKeyBinding) {
				p.exitSearchMode()
				return p, nil
			}
			// Enter: let it bubble up — the form's submit binding will fire and
			// resolve() will return the currently highlighted filtered project.

		case repoPickerModePaste:
			if key.Matches(keyMsg, repoPickerExitPasteKeyBinding) {
				p.exitPasteMode()
				return p, nil
			}
		}
	}

	switch p.mode {
	case repoPickerModeSearch:
		var cmd tea.Cmd
		p.filterInput, cmd = p.filterInput.Update(msg)
		p.applyFilter()
		return p, cmd
	case repoPickerModePaste:
		var cmd tea.Cmd
		p.pasteInput, cmd = p.pasteInput.Update(msg)
		return p, cmd
	}
	return p, nil
}

func (p repoPicker) view() string {
	switch p.mode {
	case repoPickerModePaste:
		return p.pasteInput.View()

	case repoPickerModeSearch:
		parts := []string{p.filterInput.View()}
		if len(p.filtered) == 0 {
			parts = append(parts, modalListRow(p.styles, false).Render("  (no repos match)  "))
			return strings.Join(parts, "\n")
		}
		start := 0
		end := len(p.filtered)
		if end > repoPickerSearchListHeight {
			end = repoPickerSearchListHeight
			if p.searchIdx >= repoPickerSearchListHeight {
				start = p.searchIdx - repoPickerSearchListHeight + 1
				end = p.searchIdx + 1
			}
		}
		for i := start; i < end; i++ {
			row := modalListRow(p.styles, i == p.searchIdx).Render("  " + p.filtered[i].Name + "  ")
			parts = append(parts, row)
		}
		if len(p.filtered) > repoPickerSearchListHeight {
			parts = append(parts, p.styles.Form.Hint.Render(
				"  "+formatBranchCount(p.searchIdx+1, len(p.filtered))+"  ",
			))
		}
		return strings.Join(parts, "\n")
	}

	// repoPickerModeList (default)
	if p.itemCount() == 0 {
		return modalListRow(p.styles, false).Render("  (no repos yet) — press e to add a path  ")
	}

	label := p.currentLabel()
	if p.focused {
		return modalListRow(p.styles, true).Render("< " + label + " >")
	}
	return modalListRow(p.styles, false).Render("  " + label + "  ")
}

func (p repoPicker) currentLabel() string {
	if proj := p.selectedProject(); proj != nil {
		return proj.Name
	}
	return "+ New repo by path..."
}

// resolvedSelection describes what the picker currently points at — either an
// existing registered project, a freshly-typed path that must be registered
// before use, or nothing usable.
type resolvedSelection struct {
	Project *domain.Project
	NewPath string
}

func (s resolvedSelection) IsZero() bool {
	return s.Project == nil && s.NewPath == ""
}

func (p repoPicker) resolve() resolvedSelection {
	if p.mode == repoPickerModePaste {
		if path := p.pastedPath(); path != "" {
			return resolvedSelection{NewPath: path}
		}
		return resolvedSelection{}
	}
	if proj := p.selectedProject(); proj != nil {
		return resolvedSelection{Project: proj}
	}
	return resolvedSelection{}
}

// projectIDForCreate returns the project ID to use in CreateSessionRequest
// when the resolution is an existing project. Returns uuid.Nil otherwise.
func (s resolvedSelection) projectIDForCreate() uuid.UUID {
	if s.Project == nil {
		return uuid.Nil
	}
	return s.Project.ID
}
