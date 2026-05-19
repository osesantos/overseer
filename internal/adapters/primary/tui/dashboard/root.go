package dashboard

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	sessionui "github.com/dnlopes/overseer/internal/adapters/primary/tui/session"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/service"
)

type Pane int

const (
	PaneSessions Pane = iota
	PanePreview
	SessionsListWidthPercent = 30
	TitleBarHeight           = 1
	HelpBarHeight            = 1
)

type Model struct {
	titlebar     TitlebarModel
	sessionsList sessionui.Model
	previewPanel PreviewModel
	helpBar      HelpModel
	createForm   *sessionui.CreateFormModel

	// this model state
	activePane      Pane
	width           int
	height          int
	tooSmall        bool
	styles          *styles.Styles
	sessionsService service.SessionService
}

func New(styles *styles.Styles, sessionsService service.SessionService, registry HelpRegistry) Model {
	sessions := sessionui.New(styles, sessionsService)
	helpBar := newHelpBar(registry, styles)
	previewPanel := newPreview(*styles)
	registry.RegisterPane("sessions", sessions.KeyBindings())
	registry.RegisterPane("preview", previewPanel.KeyBindings())
	sessions.Focus()
	previewPanel.SetFocus(false)
	helpBar.SetActivePane("sessions")

	return Model{
		styles:          styles,
		titlebar:        newTitlebar(styles, "Overseer"),
		sessionsList:    sessions,
		helpBar:         helpBar,
		activePane:      PaneSessions,
		previewPanel:    previewPanel,
		sessionsService: sessionsService,
		width:           80,
		height:          24,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.titlebar.Init(), m.sessionsList.Init(), m.previewPanel.Init(), m.helpBar.Init())
}

func (m Model) hasFocus() bool {
	return m.createForm == nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.resize(msg)
	case sessionui.SessionCreatedMsg:
		m.createForm = nil
		return m, m.sessionsList.Init()
	case sessionui.CancelFormMsg:
		m.createForm = nil
		return m, nil
	case tea.KeyPressMsg:
		if key.Matches(msg, shared.QuitKey) && m.hasFocus() {
			return m, tea.Quit
		}
		if key.Matches(msg, shared.HelpMenuKey) && m.hasFocus() {
			var cmd tea.Cmd
			m.helpBar, cmd = shared.UpdateModel(m.helpBar, msg)
			return m, cmd
		}
		if key.Matches(msg, shared.NewSessionKey) && m.createForm == nil && m.activePane == PaneSessions {
			cf := sessionui.NewCreateForm(m.styles, m.sessionsService)
			m.createForm = &cf
			return m, cf.Init()
		}
	}

	// If a form is open, route all messages to it first
	if m.createForm != nil {
		var cmd tea.Cmd
		createForm, cmd := shared.UpdateModel(*m.createForm, msg)
		m.createForm = &createForm
		return m, cmd
	}

	return m, nil
}

func (m Model) View() tea.View {
	if m.tooSmall {
		msg := m.styles.TooSmall.Message.Render("Terminal too small. Minimum size: 60x15.")
		return tea.NewView(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg))
	}
	if m.createForm != nil {
		return tea.NewView(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.createForm.View().Content))
	}

	titlebarView := m.titlebar.View().Content
	titlebarHeight := max(lipgloss.Height(titlebarView), 1)
	helpView := m.helpBar.View().Content
	helpHeight := max(lipgloss.Height(helpView), 1)

	bodyHeight := max(m.height-titlebarHeight-helpHeight, 1)
	leftWidth := m.width * SessionsListWidthPercent / 100
	rightWidth := m.width - leftWidth // right pane

	left := fit(m.styles, m.sessionsList.View().Content, leftWidth, bodyHeight)
	right := fit(m.styles, m.previewPanel.View().Content, rightWidth, bodyHeight)
	body := fit(m.styles, lipgloss.JoinHorizontal(lipgloss.Top, left, right), m.width, bodyHeight)
	full := lipgloss.JoinVertical(lipgloss.Left, titlebarView, body, helpView)

	return tea.NewView(full)
}

func (m Model) resize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.tooSmall = m.width < 60 || m.height < 15

	leftWidth := m.width * SessionsListWidthPercent / 100
	rightWidth := m.width - leftWidth // right width
	bodyHeight := max(m.height-TitleBarHeight-HelpBarHeight, 1)

	var cmds []tea.Cmd
	var cmd tea.Cmd

	m.sessionsList.SetSize(leftWidth, bodyHeight)
	m.previewPanel.SetSize(rightWidth, bodyHeight)

	m.titlebar, cmd = shared.UpdateModel(m.titlebar, tea.WindowSizeMsg{Width: m.width, Height: TitleBarHeight})
	cmds = append(cmds, cmd)
	m.sessionsList, cmd = shared.UpdateModel(m.sessionsList, tea.WindowSizeMsg{Width: leftWidth, Height: bodyHeight})
	cmds = append(cmds, cmd)
	m.previewPanel, cmd = shared.UpdateModel(m.previewPanel, tea.WindowSizeMsg{Width: rightWidth, Height: bodyHeight})
	cmds = append(cmds, cmd)
	m.helpBar, cmd = shared.UpdateModel(m.helpBar, tea.WindowSizeMsg{Width: m.width, Height: HelpBarHeight})
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func fit(_ *styles.Styles, s string, width, height int) string {
	return lipgloss.NewStyle().Width(width).Height(height).Render(s)
}
