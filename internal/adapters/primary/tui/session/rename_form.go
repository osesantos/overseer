package session

import (
	"context"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/components"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/core/service"
)

type renameKind int

const (
	renameKindSession renameKind = iota
	renameKindProject
)

type RenameFormModel struct {
	kind            renameKind
	targetID        uuid.UUID
	originalName    string
	input           textinput.Model
	errMsg          string
	sessionsService service.SessionService
	projectsService service.ProjectService
	styles          *styles.Styles
	contentWidth    int
}

func NewRenameSessionForm(s *styles.Styles, sessionsService service.SessionService, sess domain.Session, terminalWidth int) RenameFormModel {
	return newRenameForm(s, renameKindSession, sess.ID, sess.Name, sessionsService, service.ProjectService{}, terminalWidth)
}

func NewRenameProjectForm(s *styles.Styles, projectsService service.ProjectService, projectID uuid.UUID, currentName string, terminalWidth int) RenameFormModel {
	return newRenameForm(s, renameKindProject, projectID, currentName, service.SessionService{}, projectsService, terminalWidth)
}

func newRenameForm(
	s *styles.Styles,
	kind renameKind,
	targetID uuid.UUID,
	currentName string,
	sessionsService service.SessionService,
	projectsService service.ProjectService,
	terminalWidth int,
) RenameFormModel {
	contentWidth := formContentWidth(terminalWidth)
	inputWidth := formInputWidth(contentWidth)

	input := textinput.New()
	input.CharLimit = 100
	input.SetWidth(inputWidth)
	input.SetStyles(s.Form.Input)
	input.SetValue(currentName)
	input.Focus()

	return RenameFormModel{
		kind:            kind,
		targetID:        targetID,
		originalName:    currentName,
		input:           input,
		sessionsService: sessionsService,
		projectsService: projectsService,
		styles:          s,
		contentWidth:    contentWidth,
	}
}

func (m RenameFormModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m RenameFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if key.Matches(msg, popupCloseKeyBinding) {
			return m, shared.Emit(shared.RenamePopupCloseMsg{})
		}
		if key.Matches(msg, popupSubmitFormKeyBinding) {
			return m.submit()
		}
	case shared.SessionRenameErrMsg:
		m.errMsg = msg.Err.Error()
		return m, nil
	case shared.ProjectRenameErrMsg:
		m.errMsg = msg.Err.Error()
		return m, nil
	case shared.SessionRenamedMsg, shared.ProjectRenamedMsg:
		return m, shared.Emit(shared.RenamePopupCloseMsg{})
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m RenameFormModel) submit() (tea.Model, tea.Cmd) {
	newName := strings.TrimSpace(m.input.Value())
	if newName == "" {
		m.errMsg = "name is required"
		return m, nil
	}
	if newName == m.originalName {
		return m, shared.Emit(shared.RenamePopupCloseMsg{})
	}
	m.errMsg = ""

	switch m.kind {
	case renameKindSession:
		svc := m.sessionsService
		id := m.targetID
		return m, func() tea.Msg {
			resp, err := svc.Rename(context.Background(), service.RenameSessionRequest{ID: id, NewName: newName})
			if err != nil {
				return shared.SessionRenameErrMsg{Err: err}
			}
			return shared.SessionRenamedMsg{Session: resp.Session}
		}
	case renameKindProject:
		svc := m.projectsService
		id := m.targetID
		return m, func() tea.Msg {
			resp, err := svc.Rename(context.Background(), service.RenameProjectRequest{ID: id, NewName: newName})
			if err != nil {
				return shared.ProjectRenameErrMsg{Err: err}
			}
			return shared.ProjectRenamedMsg{Project: resp.Project}
		}
	}
	return m, nil
}

func (m RenameFormModel) View() tea.View {
	field := m.styles.Form.Field

	parts := []string{
		m.styles.Form.Title.Render(m.title()),
		"",
		field.Label.Render(m.entityLabel()+": ") + field.LabelFocused.Render(m.originalName),
		"",
		field.LabelFocused.Render("New name"),
		m.input.View(),
	}

	if m.errMsg != "" {
		parts = append(parts, "", field.Error.Render(m.errMsg))
	}
	parts = append(parts, "", m.styles.Help.Description.Render("Enter: confirm  Esc: cancel"))

	body := padBodyLines(m.styles, strings.Join(parts, "\n"), m.contentWidth)
	return tea.NewView(components.Modal(m.styles, body, m.contentWidth, 0))
}

func (m RenameFormModel) title() string {
	if m.kind == renameKindProject {
		return "Rename Group"
	}
	return "Rename Session"
}

func (m RenameFormModel) entityLabel() string {
	if m.kind == renameKindProject {
		return "Group"
	}
	return "Session"
}

func (m RenameFormModel) KeyBindings() []key.Binding {
	return []key.Binding{
		popupSubmitFormKeyBinding,
		popupCloseKeyBinding,
	}
}
