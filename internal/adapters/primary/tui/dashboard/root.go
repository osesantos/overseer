package dashboard

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/components"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/inspector"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/jobs"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/leftpane"
	overseerui "github.com/dnlopes/overseer/internal/adapters/primary/tui/overseer"
	sessionui "github.com/dnlopes/overseer/internal/adapters/primary/tui/session"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/sessiondetails"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/core/service"
)

const (
	SessionsListWidthPercent = 30
	TitleBarHeight           = 1
	TitleBarGap              = 1
	HelpBarHeight            = 1
	// ChatPanelHeightPercent is the share of body height reserved for the
	// Overseer chat panel when it is visible.
	ChatPanelHeightPercent = 30
)

type popupKind int

const (
	popupNone popupKind = iota
	popupNewSession
	popupDeleteSession
	popupRename
	popupHelp
	popupKillPreview
	popupDiscoveryWarning
	popupOverseerConfirm
)

// discoveryDismissMsg is an internal message that clears the discovery toast
// after its auto-dismiss timer fires.
type discoveryDismissMsg struct{}

type projectBranchCache struct {
	branches      []domain.BranchInfo
	defaultBranch string
	loadedAt      time.Time
}

type Model struct {
	titlebar                TitleBarModel
	leftPane                leftpane.Model
	inspector               inspector.Model
	helpBar                 shared.HelpBarModel
	createForm              sessionui.CreateFormModel
	deleteForm              sessionui.DeleteFormModel
	renameForm              sessionui.RenameFormModel
	killPreviewForm         sessionui.KillPreviewFormModel
	helpPopup               shared.HelpPopupModel
	scheduler               jobs.Model
	activePopup             popupKind
	cachedProjects          []domain.Project
	cachedSessions          []domain.Session
	cachedBranchesByProject map[uuid.UUID]projectBranchCache
	prStatuses              map[uuid.UUID]shared.PRStatusUpdatedMsg
	agentStatuses           map[uuid.UUID]domain.AgentStatus

	// discovery
	discoveryPaths        []string // configured scan roots, already expanded
	discovering           bool     // true while the background scan is in flight
	discoveryMsg          string   // non-empty = show success toast briefly
	discoveryWarningPaths []string // non-empty = show warning popup
	discoveryCount        int      // newly registered repos (shown in warning popup)

	// Overseer chat panel
	chatPanel        overseerui.Model
	overseerConfirm  overseerui.ConfirmModel
	chatPanelVisible bool
	// loops tracks active and completed evaluation loops keyed by session ID.
	loops map[uuid.UUID]*domain.LoopState

	width           int
	height          int
	minWidth        int
	minHeight       int
	tooSmall        bool
	styles          *styles.Styles
	sessionsService service.SessionService
	projectsService service.ProjectService
	overseerService *service.OverseerService
	launchers       []domain.Launcher
	editors         []domain.Editor
	labels          []domain.Label
}

func New(
	styles *styles.Styles,
	sessionsService service.SessionService,
	projectsService service.ProjectService,
	overseerService *service.OverseerService,
	scheduler jobs.Model,
	launchers []domain.Launcher,
	editors []domain.Editor,
	labels []domain.Label,
	minWidth, minHeight int,
	previewRefreshInterval time.Duration,
	discoveryPaths []string,
) Model {
	sessionsModel := sessionui.New(styles, sessionsService, labels)
	detailsModel := sessiondetails.New(styles)
	left := leftpane.New(styles, sessionsModel, detailsModel)
	left.SetFocus(true)

	m := Model{
		styles:                  styles,
		titlebar:                newTitlebar(styles, "overseer"),
		leftPane:                left,
		inspector:               inspector.New(styles, sessionsService, previewRefreshInterval),
		helpBar:                 shared.NewHelpBarModel(styles, slices.Concat(sessionsKeyBindings, inspectorKeyBindings, generalKeyBindings)),
		scheduler:               scheduler,
		sessionsService:         sessionsService,
		projectsService:         projectsService,
		overseerService:         overseerService,
		labels:                  labels,
		launchers:               launchers,
		editors:                 editors,
		minWidth:                minWidth,
		minHeight:               minHeight,
		prStatuses:              make(map[uuid.UUID]shared.PRStatusUpdatedMsg),
		agentStatuses:           make(map[uuid.UUID]domain.AgentStatus),
		cachedBranchesByProject: make(map[uuid.UUID]projectBranchCache),
		discoveryPaths:          discoveryPaths,
		discovering:             len(discoveryPaths) > 0,
		chatPanel:               overseerui.New(styles),
		loops:                   make(map[uuid.UUID]*domain.LoopState),
	}
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.titlebar.Init(),
		m.leftPane.Init(),
		m.inspector.Init(),
		m.helpBar.Init(),
		m.scheduler.Init(),
		m.loadProjects(),
		m.scheduleBranchTick(),
		m.chatPanel.Init(),
	}
	if len(m.discoveryPaths) > 0 {
		cmds = append(cmds, m.discoverProjectsCmd())
	}
	return tea.Batch(cmds...)
}

func (m Model) loadProjects() tea.Cmd {
	svc := m.projectsService
	return func() tea.Msg {
		resp, err := svc.List(context.Background(), service.ListProjectsRequest{})
		return shared.ProjectsLoadedMsg{Projects: resp.Projects, Err: err}
	}
}

func (m Model) discoverProjectsCmd() tea.Cmd {
	svc := m.projectsService
	paths := m.discoveryPaths
	return func() tea.Msg {
		resp, err := svc.Discover(context.Background(), service.DiscoverProjectsRequest{Paths: paths})
		return shared.ProjectDiscoveryCompletedMsg{
			Count:        resp.Registered,
			MissingPaths: resp.MissingPaths,
			Err:          err,
		}
	}
}

func (m Model) scheduleBranchTick() tea.Cmd {
	return tea.Tick(BranchCacheRefreshInterval, func(time.Time) tea.Msg {
		return shared.BranchCacheTickMsg{}
	})
}

func (m Model) loadBranchesForProjectCmd(projectID uuid.UUID) tea.Cmd {
	svc := m.sessionsService
	return func() tea.Msg {
		resp, err := svc.ListBranches(context.Background(), service.ListBranchesRequest{ProjectID: projectID})
		return shared.BranchesLoadedMsg{
			ProjectID:     projectID,
			Branches:      resp.Branches,
			DefaultBranch: resp.DefaultBranch,
			LoadedAt:      time.Now(),
			Err:           err,
		}
	}
}

func (m Model) loadCurrentBranchCmd(projectID uuid.UUID) tea.Cmd {
	svc := m.sessionsService
	return func() tea.Msg {
		resp, err := svc.ProjectCurrentBranch(context.Background(), service.ProjectCurrentBranchRequest{ProjectID: projectID})
		return shared.ProjectCurrentBranchLoadedMsg{ProjectID: projectID, Branch: resp.Branch, Err: err}
	}
}

func (m Model) fanOutBranchRefresh() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.cachedProjects))
	for _, p := range m.cachedProjects {
		cmds = append(cmds, m.loadBranchesForProjectCmd(p.ID))
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m Model) branchesByProjectFlat() map[uuid.UUID][]domain.BranchInfo {
	out := make(map[uuid.UUID][]domain.BranchInfo, len(m.cachedBranchesByProject))
	for k, v := range m.cachedBranchesByProject {
		out[k] = v.branches
	}
	return out
}

func (m Model) defaultBranchesByProject() map[uuid.UUID]string {
	out := make(map[uuid.UUID]string, len(m.cachedBranchesByProject))
	for k, v := range m.cachedBranchesByProject {
		if v.defaultBranch != "" {
			out[k] = v.defaultBranch
		}
	}
	return out
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.resize(msg)
	case shared.ProjectsLoadedMsg:
		if msg.Err == nil {
			m.cachedProjects = msg.Projects
			m.refreshProjectNameLookup()
			return m, m.fanOutBranchRefresh()
		}
		return m, nil
	case shared.ProjectDiscoveryCompletedMsg:
		m.discovering = false
		m.discoveryCount = msg.Count
		var cmds []tea.Cmd
		if msg.Count > 0 {
			// reload the project list so newly discovered repos appear in the picker
			cmds = append(cmds, m.loadProjects())
		}
		if len(msg.MissingPaths) > 0 {
			m.discoveryWarningPaths = msg.MissingPaths
			m.activePopup = popupDiscoveryWarning
			return m, tea.Batch(cmds...)
		}
		if msg.Count > 0 {
			m.discoveryMsg = fmt.Sprintf("Found %d new repo(s)", msg.Count)
			cmds = append(cmds, tea.Tick(3*time.Second, func(time.Time) tea.Msg {
				return discoveryDismissMsg{}
			}))
		}
		return m, tea.Batch(cmds...)
	case discoveryDismissMsg:
		m.discoveryMsg = ""
		return m, nil
	case shared.BranchesLoadedMsg:
		if msg.Err == nil {
			m.cachedBranchesByProject[msg.ProjectID] = projectBranchCache{
				branches:      msg.Branches,
				defaultBranch: msg.DefaultBranch,
				loadedAt:      msg.LoadedAt,
			}
		}
		if m.activePopup == popupNewSession {
			return m.routeToPopup(msg)
		}
		return m, nil
	case shared.BranchCacheTickMsg:
		return m, tea.Batch(m.fanOutBranchRefresh(), m.scheduleBranchTick())
	case shared.ProjectCurrentBranchLoadedMsg:
		var cmd tea.Cmd
		m.leftPane, cmd = shared.UpdateModel(m.leftPane, msg)
		return m, cmd
	case shared.ProjectRegisteredMsg:
		m.cachedProjects = append(m.cachedProjects, msg.Project)
		m.refreshProjectNameLookup()
		cmds := []tea.Cmd{m.loadBranchesForProjectCmd(msg.Project.ID)}
		if m.activePopup != popupNone {
			updated, popupCmd := m.routeToPopup(msg)
			m = updated.(Model)
			cmds = append(cmds, popupCmd)
		}
		return m, tea.Batch(cmds...)
	case shared.SessionCreatedMsg:
		m.activePopup = popupNone
		var cmd tea.Cmd
		m.leftPane, cmd = shared.UpdateModel(m.leftPane, msg)
		return m, cmd
	case shared.SessionDeleteRequestedMsg:
		m.deleteForm = sessionui.NewDeleteForm(m.styles, m.sessionsService, msg.Session)
		m.activePopup = popupDeleteSession
		return m, m.deleteForm.Init()
	case shared.SessionDeletedMsg:
		m.activePopup = popupNone
		var cmd tea.Cmd
		m.leftPane, cmd = shared.UpdateModel(m.leftPane, msg)
		return m, cmd
	case shared.SessionRenameRequestedMsg:
		m.renameForm = sessionui.NewRenameSessionForm(m.styles, m.sessionsService, msg.Session, m.width)
		m.activePopup = popupRename
		return m, m.renameForm.Init()
	case shared.ProjectRenameRequestedMsg:
		m.renameForm = sessionui.NewRenameProjectForm(m.styles, m.projectsService, msg.ProjectID, msg.CurrentName, m.width)
		m.activePopup = popupRename
		return m, m.renameForm.Init()
	case shared.SessionRenamedMsg:
		m.activePopup = popupNone
		var cmd tea.Cmd
		m.leftPane, cmd = shared.UpdateModel(m.leftPane, msg)
		return m, cmd
	case shared.ProjectRenamedMsg:
		m.activePopup = popupNone
		m.applyRenamedProject(msg.Project)
		return m, nil
	case shared.SessionSelectedMsg:
		var cmd tea.Cmd
		m.leftPane, cmd = shared.UpdateModel(m.leftPane, msg)
		var inspectorCmd tea.Cmd
		m.inspector, inspectorCmd = shared.UpdateModel(m.inspector, msg)
		if !msg.Session.HasWorktree() {
			return m, tea.Batch(cmd, inspectorCmd, m.loadCurrentBranchCmd(msg.Session.ProjectID))
		}
		return m, tea.Batch(cmd, inspectorCmd)
	case shared.NewSessionPopupCloseMsg, shared.NewSessionDeletePopupCloseMsg, shared.RenamePopupCloseMsg, shared.HelpPopupCloseMsg:
		m.activePopup = popupNone
		return m, nil
	case shared.KillPreviewPopupCloseMsg:
		m.activePopup = popupNone
		return m, m.inspector.Init()
	case shared.SessionAttachReadyMsg:
		if msg.Err != nil || msg.Command == nil {
			return m, nil
		}
		return m, tea.ExecProcess(msg.Command, func(err error) tea.Msg {
			return shared.SessionAttachedMsg{Err: err}
		})
	case shared.SessionAttachedMsg:
		return m, nil
	case shared.SessionEditorLaunchedMsg:
		return m, nil
	case shared.AgentEnterSentMsg:
		return m, nil
	case shared.PreviewSessionKilledMsg:
		m.activePopup = popupNone
		return m, m.inspector.Init()
	case shared.JobsTickMsg, shared.JobsBatchMsg:
		var cmd tea.Cmd
		m.scheduler, cmd = m.scheduler.Update(msg)
		return m, cmd
	case shared.PRStatusUpdatedMsg:
		m.prStatuses[msg.SessionID] = msg
		var cmd tea.Cmd
		m.leftPane, cmd = shared.UpdateModel(m.leftPane, msg)
		return m, cmd
	case shared.AgentStatusesUpdatedMsg:
		m.agentStatuses = msg.Statuses
		var cmd tea.Cmd
		m.leftPane, cmd = shared.UpdateModel(m.leftPane, msg)
		return m, cmd
	case shared.SessionsLoadedMsg:
		if msg.Err == nil {
			m.cachedSessions = msg.Sessions
		}
		// Also forward to the left pane so the session tree updates.
		var cmd tea.Cmd
		m.leftPane, cmd = shared.UpdateModel(m.leftPane, msg)
		return m, cmd
	case shared.OverseerTogglePanelMsg:
		m.chatPanelVisible = !m.chatPanelVisible
		m.reapplySize()
		return m, nil
	case shared.OverseerSubmitMsg:
		// The panel has already appended the user message and started the
		// spinner. We build the session context here — reading from the
		// live cachedSessions — and fire the service call.
		return m, m.overseerChatCmd(msg.UserMessage)
	case shared.OverseerCommandMsg:
		// Operator mode: parse and execute the slash command.
		return m.executeCommand(msg.Raw)
	case overseerLoopEvalResultMsg:
		return m.handleLoopEvalResult(msg)
	case shared.OverseerChatResponseMsg:
		var cmd tea.Cmd
		m.chatPanel, cmd = shared.UpdateModel(m.chatPanel, msg)
		if msg.Action != nil {
			m.overseerConfirm = overseerui.NewConfirmModel(m.styles, *msg.Action)
			m.activePopup = popupOverseerConfirm
			return m, tea.Batch(cmd, m.overseerConfirm.Init())
		}
		return m, cmd
	case shared.OverseerConfirmActionMsg:
		m.activePopup = popupNone
		return m, m.sendAgentPromptCmd(msg.Action)
	case shared.OverseerCancelActionMsg:
		m.activePopup = popupNone
		return m, nil
	case shared.OverseerPromptSentMsg:
		var cmd tea.Cmd
		m.chatPanel, cmd = shared.UpdateModel(m.chatPanel, msg)
		// Immediately refresh the preview so the user sees the sent prompt
		// land without waiting for the next scheduled poll tick.
		var refreshCmd tea.Cmd
		m.inspector, refreshCmd = shared.UpdateModel(m.inspector, inspector.ForceRefreshMsg{})
		return m, tea.Batch(cmd, refreshCmd)
	case shared.OverseerCommandResultMsg:
		// Forward command feedback to the chat panel (renders as dimmed system message).
		var cmd tea.Cmd
		m.chatPanel, cmd = shared.UpdateModel(m.chatPanel, msg)
		return m, cmd
	case overseerLoopNextTickMsg:
		ls, ok := m.loops[uuidMustParse(msg.sessionID)]
		if !ok || ls.Status != domain.LoopStatusRunning {
			return m, nil // loop was stopped between tick and now
		}
		return m, m.loopEvalCmd(*ls)
	}

	if m.activePopup != popupNone {
		return m.routeToPopup(msg)
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		// ctrl+o toggles the chat panel regardless of focus state.
		// ctrl+c is the hard-kill escape hatch that always quits.
		if key.Matches(keyMsg, overseerPanelKeyBinding) {
			if cmd, handled := m.handleKey(keyMsg); handled {
				return m, cmd
			}
		}
		if key.Matches(keyMsg, quitKeyBinding) {
			return m, tea.Quit
		}

		if m.chatPanelVisible {
			// Navigation keys pass through to the session list so the user can
			// change the selected session while typing in the chat.
			if key.Matches(keyMsg, chatPassthroughNav) {
				var cmd tea.Cmd
				m.leftPane, cmd = shared.UpdateModel(m.leftPane, keyMsg)
				return m, cmd
			}
			// tab toggles the inspector (Agent/Shell) tab.
			if key.Matches(keyMsg, inspector.ToggleViewKeyBinding) {
				var cmd tea.Cmd
				m.inspector, cmd = shared.UpdateModel(m.inspector, keyMsg)
				return m, cmd
			}
			// Every other key is consumed by the chat input.
			var cmd tea.Cmd
			m.chatPanel, cmd = shared.UpdateModel(m.chatPanel, keyMsg)
			return m, cmd
		}

		// Normal routing when the chat panel is closed.
		if cmd, handled := m.handleKey(keyMsg); handled {
			return m, cmd
		}
		var cmd tea.Cmd
		m.leftPane, cmd = shared.UpdateModel(m.leftPane, msg)
		return m, cmd
	}

	return m, shared.Broadcast(msg,
		shared.Forward(&m.leftPane),
		shared.Forward(&m.inspector),
		shared.Forward(&m.chatPanel),
	)
}

func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	if key.Matches(msg, quitKeyBinding) {
		return tea.Quit, true
	}
	if key.Matches(msg, overseerPanelKeyBinding) {
		return shared.Emit(shared.OverseerTogglePanelMsg{}), true
	}
	if key.Matches(msg, helpMenuKeyBinding) {
		m.helpPopup = shared.NewHelpPopupModel(m.styles, sessionsHelpGroups, m.width)
		m.activePopup = popupHelp
		return m.helpPopup.Init(), true
	}
	if key.Matches(msg, inspector.ToggleViewKeyBinding) {
		var cmd tea.Cmd
		m.inspector, cmd = shared.UpdateModel(m.inspector, msg)
		return cmd, true
	}
	if key.Matches(msg, newSessionKeyBinding) {
		initialProjectID := m.cursorProjectID()
		m.createForm = sessionui.NewCreateForm(
			m.styles,
			m.sessionsService,
			m.projectsService,
			m.cachedProjects,
			initialProjectID,
			m.branchesByProjectFlat(),
			m.defaultBranchesByProject(),
			m.launchers,
			m.editors,
			m.width,
		)
		m.activePopup = popupNewSession
		cmds := []tea.Cmd{m.createForm.Init()}
		if refresh := m.refreshStaleProjectBranchesCmd(initialProjectID); refresh != nil {
			cmds = append(cmds, refresh)
		}
		return tea.Batch(cmds...), true
	}
	if key.Matches(msg, attachKeyBinding) {
		if m.inspector.ActiveViewLabel() == "Shell" {
			if cmd := m.attachSelectedSessionShellCmd(); cmd != nil {
				return cmd, true
			}
		} else {
			if cmd := m.attachSelectedSessionAgentCmd(); cmd != nil {
				return cmd, true
			}
		}
	}
	if key.Matches(msg, openEditorKeyBinding) {
		if cmd := m.openSelectedSessionEditorCmd(); cmd != nil {
			return cmd, true
		}
	}
	if key.Matches(msg, sendAgentEnterKeyBinding) {
		if cmd := m.sendAgentEnterCmd(); cmd != nil {
			return cmd, true
		}
	}
	if key.Matches(msg, killPreviewSessionKeyBinding) {
		if cmd := m.showKillPreviewPopupCmd(); cmd != nil {
			return cmd, true
		}
	}
	return nil, false
}

func (m Model) refreshStaleProjectBranchesCmd(initialProjectID uuid.UUID) tea.Cmd {
	if initialProjectID == uuid.Nil {
		return nil
	}
	cache, ok := m.cachedBranchesByProject[initialProjectID]
	if ok && time.Since(cache.loadedAt) < BranchCacheStaleThreshold {
		return nil
	}
	return m.loadBranchesForProjectCmd(initialProjectID)
}

func (m Model) attachSelectedSessionShellCmd() tea.Cmd {
	idStr := m.leftPane.SelectedSessionID()
	if idStr == "" {
		return nil
	}
	sessID, err := uuid.Parse(idStr)
	if err != nil {
		return nil
	}
	svc := m.sessionsService
	return func() tea.Msg {
		resp, err := svc.AttachShell(context.Background(), service.AttachShellRequest{ID: sessID})
		return shared.SessionAttachReadyMsg{Command: resp.Command, Err: err}
	}
}

func (m Model) attachSelectedSessionAgentCmd() tea.Cmd {
	idStr := m.leftPane.SelectedSessionID()
	if idStr == "" {
		return nil
	}
	sessID, err := uuid.Parse(idStr)
	if err != nil {
		return nil
	}
	svc := m.sessionsService
	return func() tea.Msg {
		resp, err := svc.AttachAgent(context.Background(), service.AttachAgentRequest{ID: sessID})
		return shared.SessionAttachReadyMsg{Command: resp.Command, Err: err}
	}
}

func (m Model) openSelectedSessionEditorCmd() tea.Cmd {
	idStr := m.leftPane.SelectedSessionID()
	if idStr == "" {
		return nil
	}
	sessID, err := uuid.Parse(idStr)
	if err != nil {
		return nil
	}
	svc := m.sessionsService
	return func() tea.Msg {
		_, err := svc.OpenEditor(context.Background(), service.OpenEditorRequest{ID: sessID})
		return shared.SessionEditorLaunchedMsg{Err: err}
	}
}

func (m Model) sendAgentEnterCmd() tea.Cmd {
	idStr := m.leftPane.SelectedSessionID()
	if idStr == "" {
		return nil
	}
	sessID, err := uuid.Parse(idStr)
	if err != nil {
		return nil
	}
	svc := m.sessionsService
	return func() tea.Msg {
		_, err := svc.SendAgentEnter(context.Background(), service.SendAgentEnterRequest{ID: sessID})
		return shared.AgentEnterSentMsg{Err: err}
	}
}

func (m *Model) showKillPreviewPopupCmd() tea.Cmd {
	idStr := m.leftPane.SelectedSessionID()
	if idStr == "" {
		return nil
	}
	sessID, err := uuid.Parse(idStr)
	if err != nil {
		return nil
	}
	sess, ok := m.leftPane.SelectedSession()
	if !ok {
		return nil
	}
	kind := m.inspector.ActiveViewLabel()
	m.killPreviewForm = sessionui.NewKillPreviewForm(m.styles, m.sessionsService, sessID, sess.Name, kind)
	m.activePopup = popupKillPreview
	return m.killPreviewForm.Init()
}

func (m Model) routeToPopup(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.activePopup {
	case popupNewSession:
		var cmd tea.Cmd
		m.createForm, cmd = shared.UpdateModel(m.createForm, msg)
		return m, cmd
	case popupDeleteSession:
		var cmd tea.Cmd
		m.deleteForm, cmd = shared.UpdateModel(m.deleteForm, msg)
		return m, cmd
	case popupRename:
		var cmd tea.Cmd
		m.renameForm, cmd = shared.UpdateModel(m.renameForm, msg)
		return m, cmd
	case popupHelp:
		var cmd tea.Cmd
		m.helpPopup, cmd = shared.UpdateModel(m.helpPopup, msg)
		return m, cmd
	case popupKillPreview:
		var cmd tea.Cmd
		m.killPreviewForm, cmd = shared.UpdateModel(m.killPreviewForm, msg)
		return m, cmd
	case popupOverseerConfirm:
		var cmd tea.Cmd
		m.overseerConfirm, cmd = shared.UpdateModel(m.overseerConfirm, msg)
		return m, cmd
	case popupDiscoveryWarning:
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			if key.Matches(keyMsg, discoveryPopupDismissBinding) {
				m.activePopup = popupNone
			}
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) applyRenamedProject(p domain.Project) {
	for i := range m.cachedProjects {
		if m.cachedProjects[i].ID == p.ID {
			m.cachedProjects[i] = p
			break
		}
	}
	m.refreshProjectNameLookup()
}

func (m Model) cursorProjectID() uuid.UUID {
	if sess, ok := m.leftPane.SelectedSession(); ok {
		return sess.ProjectID
	}
	return uuid.Nil
}

func (m *Model) refreshProjectNameLookup() {
	names := make(map[uuid.UUID]string, len(m.cachedProjects))
	for _, p := range m.cachedProjects {
		names[p.ID] = p.Name
	}
	m.leftPane.SetProjectNameLookup(names)
}

func (m Model) View() tea.View {
	if m.tooSmall {
		msg := m.styles.TooSmall.Message.Render(fmt.Sprintf("Terminal too small. Minimum size: %dx%d.", m.minWidth, m.minHeight))
		return tea.NewView(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg))
	}
	if m.activePopup != popupNone {
		return tea.NewView(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.popupView()))
	}

	titlebarView := m.titlebar.View().Content
	titlebarHeight := max(lipgloss.Height(titlebarView), 1)
	helpView := m.helpBar.View().Content
	helpHeight := max(lipgloss.Height(helpView), 1)

	bodyHeight := max(m.height-titlebarHeight-TitleBarGap-helpHeight, 1)
	leftWidth := m.width * SessionsListWidthPercent / 100
	rightWidth := m.width - leftWidth

	// When the chat panel is visible the body shrinks to make room for it.
	if m.chatPanelVisible {
		chatHeight := max(bodyHeight*ChatPanelHeightPercent/100, 4)
		bodyHeight = max(bodyHeight-chatHeight, 1)
	}

	left := fit(m.styles, m.leftPane.View().Content, leftWidth, bodyHeight)
	right := fit(m.styles, m.inspector.View().Content, rightWidth, bodyHeight)
	body := fit(m.styles, lipgloss.JoinHorizontal(lipgloss.Top, left, right), m.width, bodyHeight)

	// The gap line between the titlebar and body doubles as a notification
	// slot: it shows a right-aligned toast while repo discovery is running
	// or briefly after it completes. The line already exists in the layout
	// so no height shift occurs.
	var gapLine string
	switch {
	case m.discovering:
		gapLine = m.styles.Toast.GapLine.Width(m.width).
			Render(m.styles.Toast.Indexing.Render(" Indexing repos… "))
	case m.discoveryMsg != "":
		gapLine = m.styles.Toast.GapLine.Width(m.width).
			Render(m.styles.Toast.Done.Render(" " + m.discoveryMsg + " "))
	}

	full := lipgloss.JoinVertical(lipgloss.Left, titlebarView, gapLine, body, helpView)

	if m.chatPanelVisible {
		chatView := m.chatPanel.View().Content
		full = lipgloss.JoinVertical(lipgloss.Left, titlebarView, gapLine, body, chatView, helpView)
	}

	return tea.NewView(full)
}

func (m Model) popupView() string {
	switch m.activePopup {
	case popupNewSession:
		return m.createForm.View().Content
	case popupDeleteSession:
		return m.deleteForm.View().Content
	case popupRename:
		return m.renameForm.View().Content
	case popupHelp:
		return m.helpPopup.View().Content
	case popupKillPreview:
		return m.killPreviewForm.View().Content
	case popupDiscoveryWarning:
		return m.discoveryWarningView()
	case popupOverseerConfirm:
		return m.overseerConfirm.View().Content
	}
	return ""
}

// discoveryWarningView renders a modal listing the discovery paths that could
// not be found on disk. If any repos were also newly registered during the
// same scan, a success line is shown above the warning list.
func (m Model) discoveryWarningView() string {
	s := m.styles
	var b strings.Builder
	b.WriteString(s.Danger.Title.Render("Discovery paths not found"))
	b.WriteString("\n\n")
	if m.discoveryCount > 0 {
		b.WriteString(s.SessionDetails.Good.Render(fmt.Sprintf("%d new repo(s) registered.", m.discoveryCount)))
		b.WriteString("\n\n")
	}
	for _, p := range m.discoveryWarningPaths {
		b.WriteString(s.Danger.Body.Render("  • " + p))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(s.Form.Hint.Render("These paths do not exist. Check your config."))
	b.WriteString("\n\n")
	b.WriteString(s.Form.Hint.Render("Enter / Esc  dismiss"))
	return components.Modal(s, b.String(), 52, 0)
}

func (m Model) resize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.tooSmall = m.width < m.minWidth || m.height < m.minHeight
	m.reapplySize()
	return m, nil
}

// reapplySize recalculates all child panel dimensions from the current m.width
// and m.height. Called both from resize() and when the chat panel is toggled.
func (m *Model) reapplySize() {
	leftWidth := m.width * SessionsListWidthPercent / 100
	rightWidth := m.width - leftWidth

	fullBodyHeight := max(m.height-TitleBarHeight-TitleBarGap-HelpBarHeight, 1)
	bodyHeight := fullBodyHeight
	if m.chatPanelVisible {
		chatHeight := max(fullBodyHeight*ChatPanelHeightPercent/100, 4)
		bodyHeight = max(fullBodyHeight-chatHeight, 1)
		m.chatPanel.SetSize(m.width, chatHeight)
	}

	m.leftPane.SetSize(leftWidth, bodyHeight)
	m.inspector.SetSize(rightWidth, bodyHeight)
	m.helpBar.SetSize(m.width, HelpBarHeight)
	m.titlebar.SetSize(m.width, TitleBarHeight)
}

// sendAgentPromptCmd fires the async SendAgentPrompt call.
func (m Model) sendAgentPromptCmd(action domain.OverseerAction) tea.Cmd {
	svc := m.sessionsService
	sessionName := action.SessionName
	return shared.Request(
		func(ctx context.Context) (service.SendAgentPromptResponse, error) {
			return svc.SendAgentPrompt(ctx, service.SendAgentPromptRequest{
				ID:     action.SessionID,
				Prompt: action.Prompt,
			})
		},
		func(_ service.SendAgentPromptResponse, err error) tea.Msg {
			return shared.OverseerPromptSentMsg{SessionName: sessionName, Err: err}
		},
	)
}

// sessionSnapshots returns a snapshot of all current sessions for injection
// into the Overseer Agent's context. Called by overseerChatCmd at the moment
// the user submits a message, so it always reads from the live cachedSessions.
func (m *Model) sessionSnapshots() []overseerui.SessionSnapshot {
	snaps := make([]overseerui.SessionSnapshot, 0, len(m.cachedSessions))
	for _, sess := range m.cachedSessions {
		projectName := ""
		for _, p := range m.cachedProjects {
			if p.ID == sess.ProjectID {
				projectName = p.Name
				break
			}
		}
		status := domain.AgentStatusUnknown
		if st, ok := m.agentStatuses[sess.ID]; ok {
			status = st.Kind
		}
		snaps = append(snaps, overseerui.SessionSnapshot{
			SessionID:   sess.ID,
			SessionName: sess.Name,
			ProjectName: projectName,
			Branch:      sess.Branch,
			AgentType:   sess.AgentType,
			Status:      status,
		})
	}
	return snaps
}

// overseerChatCmd builds the async tea.Cmd that calls OverseerService.Chat.
// It is called from the dashboard's Update so that it always reads from the
// live cachedSessions at submit time — never from a stale closure.
//
// Before invoking the LLM the command concurrently captures the agent-pane
// output for every session so the Overseer has real context when deciding
// what to do.
func (m *Model) overseerChatCmd(userMsg string) tea.Cmd {
	if m.overseerService == nil {
		return nil
	}
	svc := m.overseerService
	sessSvc := m.sessionsService
	snaps := m.sessionSnapshots()

	return shared.RequestWithTimeout(
		overseerRequestTimeout,
		func(ctx context.Context) (service.OverseerChatResponse, error) {
			// Concurrently capture each session's agent-pane output so the
			// LLM gets real context without blocking on N sequential calls.
			type paneResult struct {
				idx    int
				output string
			}
			ch := make(chan paneResult, len(snaps))
			for i, snap := range snaps {
				i, snap := i, snap
				go func() {
					resp, err := sessSvc.PreviewSession(ctx, service.PreviewSessionRequest{
						ID:   snap.SessionID,
						Kind: service.PreviewKindAgent,
					})
					out := ""
					if err == nil && resp.SessionReady {
						out = truncateLines(ansi.Strip(resp.Content), 50)
					}
					ch <- paneResult{idx: i, output: out}
				}()
			}
			paneOutputs := make([]string, len(snaps))
			for range snaps {
				r := <-ch
				paneOutputs[r.idx] = r.output
			}

			sessions := make([]domain.OverseerSessionContext, 0, len(snaps))
			for i, snap := range snaps {
				sessions = append(sessions, domain.OverseerSessionContext{
					SessionID:   snap.SessionID,
					SessionName: snap.SessionName,
					ProjectName: snap.ProjectName,
					Branch:      snap.Branch,
					AgentType:   snap.AgentType,
					Status:      snap.Status,
					PaneOutput:  paneOutputs[i],
				})
			}
			return svc.Chat(ctx, service.OverseerChatRequest{
				UserMessage: userMsg,
				Sessions:    sessions,
			})
		},
		func(resp service.OverseerChatResponse, err error) tea.Msg {
			if err != nil {
				return shared.OverseerChatResponseMsg{Err: err}
			}
			return shared.OverseerChatResponseMsg{
				Text:   resp.Text,
				Action: resp.Action,
			}
		},
	)
}

// truncateLines returns the last n lines of s, or s unchanged when it has
// fewer than n lines. Used to cap pane-capture output sent to the LLM so
// prompt size stays bounded regardless of session history length.
func truncateLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

func fit(s *styles.Styles, content string, width, height int) string {
	return s.Layout.Box.Width(width).Height(height).Render(content)
}

// handleLoopEvalResult processes the result of a single EvaluateLoop call.
func (m Model) handleLoopEvalResult(msg overseerLoopEvalResultMsg) (tea.Model, tea.Cmd) {
	ls := m.loops[msg.state.SessionID]
	if ls == nil || ls.Status != domain.LoopStatusRunning {
		return m, nil // loop was stopped while the eval was in flight
	}

	if msg.err != nil {
		ls.Iterations++
		ls.ConsecutiveErrors++
		const maxConsecutiveErrors = 3
		if ls.ConsecutiveErrors >= maxConsecutiveErrors {
			ls.Status = domain.LoopStatusStopped
			note := fmt.Sprintf(
				"Loop on %q stopped after %d consecutive evaluation errors (iteration %d). "+
					"The session may be waiting for user input.\nLast error: %s",
				ls.SessionName, maxConsecutiveErrors, ls.Iterations, msg.err)
			return m, tea.Batch(
				shared.Emit(shared.OverseerCommandResultMsg{Text: note, IsError: true}),
				m.broadcastLoopState(),
			)
		}
		errMsg := fmt.Sprintf("Loop on %q — evaluation error (iteration %d): %s", ls.SessionName, ls.Iterations, msg.err)
		return m, tea.Batch(
			shared.Emit(shared.OverseerCommandResultMsg{Text: errMsg, IsError: true}),
			loopNextTickCmd(*ls),
		)
	}

	ls.ConsecutiveErrors = 0

	if msg.eval.Done {
		ls.Iterations++
		ls.Status = domain.LoopStatusDone
		note := fmt.Sprintf("Loop on %q completed after %d iteration(s).\n%s", ls.SessionName, ls.Iterations, msg.eval.Summary)
		return m, tea.Batch(
			shared.Emit(shared.OverseerCommandResultMsg{Text: note}),
			m.broadcastLoopState(),
			func() tea.Msg { return inspector.ForceRefreshMsg{} },
		)
	}

	if msg.eval.AgentStillWorking {
		// WAIT iterations do not count against MaxIterations.
		ls.ConsecutiveWaits++
		if ls.ConsecutiveWaits >= loopMaxConsecutiveWaits {
			ls.Status = domain.LoopStatusStopped
			note := fmt.Sprintf(
				"Loop on %q stopped: agent was still working for %d consecutive iterations without requesting input.",
				ls.SessionName, ls.ConsecutiveWaits)
			return m, tea.Batch(
				shared.Emit(shared.OverseerCommandResultMsg{Text: note, IsError: true}),
				m.broadcastLoopState(),
			)
		}
		waitNote := fmt.Sprintf("Loop on %q — iteration %d/%d: agent is still working, waiting…",
			ls.SessionName, ls.Iterations, ls.MaxIterations)
		return m, tea.Batch(
			shared.Emit(shared.OverseerCommandResultMsg{Text: waitNote}),
			m.broadcastLoopState(),
			loopNextTickCmd(*ls),
		)
	}

	// Agent is waiting for input — count this as a real iteration.
	ls.Iterations++
	ls.ConsecutiveWaits = 0

	if ls.Iterations >= ls.MaxIterations {
		ls.Status = domain.LoopStatusStopped
		note := fmt.Sprintf("Loop on %q stopped: max iterations (%d) reached without criteria being met.", ls.SessionName, ls.MaxIterations)
		return m, tea.Batch(
			shared.Emit(shared.OverseerCommandResultMsg{Text: note, IsError: true}),
			m.broadcastLoopState(),
		)
	}

	// Not done yet — send the suggested prompt and schedule next check.
	var cmds []tea.Cmd
	if msg.eval.PromptToSend != "" {
		cmds = append(cmds, m.sendAgentPromptCmd(domain.OverseerAction{
			SessionID:   ls.SessionID,
			SessionName: ls.SessionName,
			Prompt:      msg.eval.PromptToSend,
		}))
	}
	progress := fmt.Sprintf("Loop on %q — iteration %d/%d: criteria not yet met.",
		ls.SessionName, ls.Iterations, ls.MaxIterations)
	cmds = append(cmds,
		shared.Emit(shared.OverseerCommandResultMsg{Text: progress}),
		m.broadcastLoopState(),
		loopNextTickCmd(*ls),
	)
	return m, tea.Batch(cmds...)
}

// uuidMustParse parses a UUID string; panics on invalid input. Only called
// with UUIDs we serialised ourselves (loop session IDs).
func uuidMustParse(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		panic("overseer: invalid loop session UUID: " + s)
	}
	return id
}
