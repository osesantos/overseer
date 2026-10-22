package dashboard

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/components"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/inspector"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/jobs"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/leftpane"
	overseerui "github.com/dnlopes/overseer/internal/adapters/primary/tui/overseer"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/prs"
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

// pageKind selects what the body shows: the sessions dashboard or a full-width
// page opened from the command line (`:prs`).
type pageKind int

const (
	pageSessions pageKind = iota
	pagePRs
)

// ReviewOptions is the pullRequests config the dashboard acts on: the launcher
// template, whether polls start review sessions, and how many per poll.
type ReviewOptions struct {
	Command    string
	AutoReview bool
	MaxPerPoll int
}

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

// toastDismissMsg is an internal message that clears the gap-line toast
// after its auto-dismiss timer fires.
type toastDismissMsg struct{}

type projectBranchCache struct {
	branches      []domain.BranchInfo
	defaultBranch string
	loadedAt      time.Time
}

type Model struct {
	titlebar        TitleBarModel
	leftPane        leftpane.Model
	inspector       inspector.Model
	helpBar         shared.HelpBarModel
	createForm      sessionui.CreateFormModel
	deleteForm      sessionui.DeleteFormModel
	renameForm      sessionui.RenameFormModel
	killPreviewForm sessionui.KillPreviewFormModel
	helpPopup       shared.HelpPopupModel
	scheduler       jobs.Model
	activePopup     popupKind
	page            pageKind
	prsPage         prs.Model
	prsHelpBar      shared.HelpBarModel
	// pendingFocusID is a session to select once the next SessionsLoadedMsg
	// lands, used after a review session is created from the PRs page.
	pendingFocusID uuid.UUID
	// cmdline is the k9s-style one-line prompt opened with ":"; it replaces the
	// help bar while open.
	cmdline       textinput.Model
	cmdlineActive bool
	// review holds the auto-review settings; AutoReview can be flipped at
	// runtime from the :prs page.
	review                  ReviewOptions
	cachedProjects          []domain.Project
	cachedSessions          []domain.Session
	cachedBranchesByProject map[uuid.UUID]projectBranchCache
	prStatuses              map[uuid.UUID]shared.PRStatusUpdatedMsg
	agentStatuses           map[uuid.UUID]domain.AgentStatus
	reviewInbox             domain.ReviewInbox

	// discovery
	discoveryPaths        []string // configured scan roots, already expanded
	discovering           bool     // true while the background scan is in flight
	toast                 string   // non-empty = show a notice in the gap line briefly
	discoveryWarningPaths []string // non-empty = show warning popup
	discoveryCount        int      // newly registered repos (shown in warning popup)

	// Overseer chat panel
	chatPanel        overseerui.Model
	overseerConfirm  overseerui.ConfirmModel
	chatPanelVisible bool

	width           int
	height          int
	minWidth        int
	minHeight       int
	tooSmall        bool
	styles          *styles.Styles
	themes          <-chan struct{}
	sessionsService service.SessionService
	projectsService service.ProjectService
	prService       service.PullRequestService
	overseerService *service.OverseerService
	swarmService    *service.SwarmService
	launchers       []domain.Launcher
	labels          []domain.Label

	// swarmBoardURL is the base URL agents call to reach the board. Empty means
	// swarm mode is disabled, which is what gates the create form's swarm field.
	swarmBoardURL string
	// swarmMaxAgents is the configured ceiling offered by the create form.
	swarmMaxAgents int
}

func New(
	styles *styles.Styles,
	sessionsService service.SessionService,
	projectsService service.ProjectService,
	prService service.PullRequestService,
	overseerService *service.OverseerService,
	swarmService *service.SwarmService,
	scheduler jobs.Model,
	launchers []domain.Launcher,
	labels []domain.Label,
	minWidth, minHeight int,
	previewRefreshInterval time.Duration,
	discoveryPaths []string,
	swarmBoardURL string,
	swarmMaxAgents int,
	review ReviewOptions,
) Model {
	sessionsModel := sessionui.New(styles, sessionsService, labels)
	detailsModel := sessiondetails.New(styles)
	left := leftpane.New(styles, sessionsModel, detailsModel)
	left.SetFocus(true)

	cmdline := textinput.New()
	cmdline.Prompt = ":"
	cmdline.Placeholder = "prs · sessions · help"
	cmdline.SetStyles(styles.Form.Input)

	m := Model{
		styles:                  styles,
		prsPage:                 prs.New(styles),
		prsHelpBar:              shared.NewHelpBarModel(styles, prsPageKeyBindings),
		cmdline:                 cmdline,
		review:                  review,
		titlebar:                newTitlebar(styles, "overseer"),
		leftPane:                left,
		inspector:               inspector.New(styles, sessionsService, swarmService, previewRefreshInterval),
		helpBar:                 shared.NewHelpBarModel(styles, slices.Concat(sessionsKeyBindings, inspectorKeyBindings, generalKeyBindings)),
		scheduler:               scheduler,
		sessionsService:         sessionsService,
		projectsService:         projectsService,
		prService:               prService,
		overseerService:         overseerService,
		swarmService:            swarmService,
		swarmBoardURL:           swarmBoardURL,
		swarmMaxAgents:          swarmMaxAgents,
		labels:                  labels,
		launchers:               launchers,
		minWidth:                minWidth,
		minHeight:               minHeight,
		prStatuses:              make(map[uuid.UUID]shared.PRStatusUpdatedMsg),
		agentStatuses:           make(map[uuid.UUID]domain.AgentStatus),
		cachedBranchesByProject: make(map[uuid.UUID]projectBranchCache),
		discoveryPaths:          discoveryPaths,
		discovering:             len(discoveryPaths) > 0,
		chatPanel:               overseerui.New(styles),
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
		m.waitTheme(),
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
			m.syncPRsPage()
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
			cmds = append(cmds, m.showToast(fmt.Sprintf("Found %d new repo(s)", msg.Count)))
		}
		return m, tea.Batch(cmds...)
	case toastDismissMsg:
		m.toast = ""
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
		cmds := []tea.Cmd{cmd}
		// A swarm's panes exist by now but the agents have no idea what they are
		// for, so brief them: write the contract, post the goal, introduce each
		// agent to its identity.
		if bootstrap := m.bootstrapSwarmCmd(msg.Session, msg.SwarmGoal); bootstrap != nil {
			cmds = append(cmds, bootstrap)
		}
		return m, tea.Batch(cmds...)

	case shared.SwarmBootstrappedMsg:
		if msg.Err != nil {
			m.toast = "Swarm bootstrap failed: " + msg.Err.Error()
		}
		return m, nil
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
	case shared.AgentEnterSentMsg:
		return m, nil
	case shared.PreviewSessionKilledMsg:
		m.activePopup = popupNone
		return m, m.inspector.Init()
	case shared.OmarchyPaletteMsg:
		m.styles.Repaint(msg.Palette)
		return m, m.waitTheme()
	case shared.JobsTickMsg, shared.JobsBatchMsg:
		var cmd tea.Cmd
		m.scheduler, cmd = m.scheduler.Update(msg)
		return m, cmd
	case shared.PullRequestsLoadedMsg:
		if msg.Err != nil {
			return m, m.showToast("PR inbox: " + msg.Err.Error())
		}
		m.reviewInbox = msg.Inbox
		m.syncPRsPage()
		return m, m.autoReviewCmd(msg.Inbox)
	case shared.AutoReviewCompletedMsg:
		if msg.Err != nil {
			return m, m.showToast("auto-review: " + msg.Err.Error())
		}
		if len(msg.Created) == 0 {
			return m, nil
		}
		var cmd tea.Cmd
		m.leftPane, cmd = shared.UpdateModel(m.leftPane, msg)
		return m, cmd
	case shared.AutoReviewToggleMsg:
		m.review.AutoReview = !m.review.AutoReview
		m.syncPRsPage()
		on := m.review.AutoReview
		svc := m.prService
		persist := func() tea.Msg {
			_, err := svc.SetAutoReview(context.Background(), service.SetAutoReviewRequest{On: on})
			return shared.AutoReviewSavedMsg{Err: err}
		}
		if on {
			return m, tea.Batch(persist, m.showToast("auto-review on — eligible PRs get a review session on the next poll"))
		}
		return m, tea.Batch(persist, m.showToast("auto-review off — stays off across restarts until you turn it back on"))
	case shared.AutoReviewSavedMsg:
		if msg.Err != nil {
			return m, m.showToast("auto-review toggle not saved: " + msg.Err.Error())
		}
		return m, nil
	case shared.PullRequestOpenInBrowserMsg:
		svc := m.prService
		url := msg.PR.URL
		return m, func() tea.Msg {
			_, err := svc.OpenInBrowser(context.Background(), service.OpenPullRequestInBrowserRequest{URL: url})
			return shared.PullRequestBrowserOpenedMsg{Err: err}
		}
	case shared.PullRequestBrowserOpenedMsg:
		if msg.Err != nil {
			return m, m.showToast("could not open the browser: " + msg.Err.Error())
		}
		return m, nil
	case shared.PageCloseMsg:
		m.page = pageSessions
		m.reapplySize()
		return m, nil
	case shared.PullRequestOpenMsg:
		if sess, ok := m.reviewSessionFor(msg.PR); ok {
			m.page = pageSessions
			m.reapplySize()
			return m, shared.Emit(shared.SessionFocusRequestedMsg{SessionID: sess.ID})
		}
		return m, m.startReviewCmd(msg.PR)
	case shared.PullRequestReviewMsg:
		if sess, ok := m.reviewSessionFor(msg.PR); ok {
			return m, m.showToast(fmt.Sprintf("PR #%d already has a review session: %s", msg.PR.Number, sess.Name))
		}
		return m, m.startReviewCmd(msg.PR)
	case shared.PullRequestDismissMsg:
		svc := m.prService
		ref := msg.PR.Ref()
		return m, func() tea.Msg {
			_, err := svc.Dismiss(context.Background(), service.DismissPullRequestRequest{Ref: ref})
			return shared.PullRequestDismissedMsg{Ref: ref, Err: err}
		}
	case shared.PullRequestDismissedMsg:
		if msg.Err != nil {
			return m, m.showToast("could not dismiss PR: " + msg.Err.Error())
		}
		m.reviewInbox.PullRequests = slices.DeleteFunc(slices.Clone(m.reviewInbox.PullRequests),
			func(pr domain.PullRequest) bool { return pr.Ref() == msg.Ref })
		m.syncPRsPage()
		return m, m.showToast(fmt.Sprintf("PR #%d hidden from the inbox", msg.Ref.Number))
	case shared.ReviewSessionStartedMsg:
		if msg.Err != nil {
			return m, m.showToast("could not start review: " + msg.Err.Error())
		}
		m.page = pageSessions
		m.pendingFocusID = msg.Session.ID
		m.reapplySize()
		return m, shared.Emit(shared.SessionCreatedMsg{Session: msg.Session})
	case shared.PRStatusUpdatedMsg:
		m.prStatuses[msg.SessionID] = msg
		var cmd tea.Cmd
		m.leftPane, cmd = shared.UpdateModel(m.leftPane, msg)
		if msg.Err == nil {
			cmd = tea.Batch(cmd, m.reapFinishedReviewCmd(msg.SessionID, msg.PR))
		}
		return m, cmd
	case shared.ReviewSessionReapedMsg:
		if msg.Err != nil {
			return m, m.showToast(fmt.Sprintf("could not delete finished review %s: %v", msg.Session.Name, msg.Err))
		}
		return m, tea.Batch(
			m.showToast(fmt.Sprintf("PR #%d is merged or closed — review session %q deleted", msg.Session.ReviewPR.Number, msg.Session.Name)),
			shared.Emit(shared.SessionDeletedMsg{}),
		)
	case shared.AgentStatusesUpdatedMsg:
		m.agentStatuses = msg.Statuses
		m.syncPRsPage()
		var cmd tea.Cmd
		m.leftPane, cmd = shared.UpdateModel(m.leftPane, msg)
		return m, cmd
	case shared.SessionsLoadedMsg:
		if msg.Err == nil {
			m.cachedSessions = msg.Sessions
			m.syncPRsPage()
		}
		// Also forward to the left pane so the session tree updates.
		var cmd tea.Cmd
		m.leftPane, cmd = shared.UpdateModel(m.leftPane, msg)
		if m.pendingFocusID != uuid.Nil {
			focus := shared.SessionFocusRequestedMsg{SessionID: m.pendingFocusID}
			m.pendingFocusID = uuid.Nil
			cmd = tea.Batch(cmd, shared.Emit(focus))
		}
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
		// Command feedback renders in the chat panel as a dimmed system message,
		// or in the gap line when the command came from ":" with the chat hidden.
		if !m.chatPanelVisible {
			return m, m.showToast(msg.Text)
		}
		var cmd tea.Cmd
		m.chatPanel, cmd = shared.UpdateModel(m.chatPanel, msg)
		return m, cmd
	}

	if m.activePopup != popupNone {
		return m.routeToPopup(msg)
	}

	// Paste follows the same ownership rules as typing, and has to be routed
	// explicitly: it is its own message type, so it never reaches the key branch
	// below. Sending it to exactly one input matters — the fallthrough broadcast
	// would otherwise deliver it to the chat *and* the board compose line at once.
	if pasteMsg, ok := msg.(tea.PasteMsg); ok {
		if m.cmdlineActive {
			var cmd tea.Cmd
			m.cmdline, cmd = m.cmdline.Update(pasteMsg)
			return m, cmd
		}
		if m.inspector.CapturesInput() {
			var cmd tea.Cmd
			m.inspector, cmd = shared.UpdateModel(m.inspector, pasteMsg)
			return m, cmd
		}
		if m.chatPanelVisible {
			var cmd tea.Cmd
			m.chatPanel, cmd = shared.UpdateModel(m.chatPanel, pasteMsg)
			return m, cmd
		}
		// Nothing owns an input: drop it rather than broadcasting a paste into
		// panes that would only misinterpret it.
		return m, nil
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		// A view that has taken the keyboard — currently the Agents Board's
		// compose line — keeps every key except the hard-kill escape hatch. This
		// gate has to sit ahead of everything else, including ctrl+o and quit,
		// or a half-typed message would be cut short by a global shortcut.
		if m.inspector.CapturesInput() {
			if key.Matches(keyMsg, hardQuitKeyBinding) {
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.inspector, cmd = shared.UpdateModel(m.inspector, keyMsg)
			return m, cmd
		}

		// The command line owns every key while open, like the board compose
		// line above: a half-typed command must not trigger a shortcut.
		if m.cmdlineActive {
			return m.updateCmdline(keyMsg)
		}

		// ctrl+o toggles the chat panel regardless of focus state.
		if key.Matches(keyMsg, overseerPanelKeyBinding) {
			if cmd, handled := m.handleKey(keyMsg); handled {
				return m, cmd
			}
		}

		if m.chatPanelVisible {
			// Only ctrl+c quits while the chat is open — "q" belongs to the
			// message being typed.
			if key.Matches(keyMsg, hardQuitKeyBinding) {
				return m, tea.Quit
			}
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

		// On a full-width page q goes back to the sessions screen; only ctrl+c
		// quits the app from there.
		if m.page == pagePRs && key.Matches(keyMsg, quitKeyBinding) && !key.Matches(keyMsg, hardQuitKeyBinding) {
			m.page = pageSessions
			m.reapplySize()
			return m, nil
		}
		if key.Matches(keyMsg, quitKeyBinding) {
			return m, tea.Quit
		}
		if key.Matches(keyMsg, cmdlineKeyBinding) {
			m.cmdlineActive = true
			m.cmdline.SetValue("")
			return m, m.cmdline.Focus()
		}

		// A full-width page takes the keys the sessions dashboard would get.
		if m.page == pagePRs {
			var cmd tea.Cmd
			m.prsPage, cmd = shared.UpdateModel(m.prsPage, keyMsg)
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
	// The inspector never sees a key it is not handed, so the swarm agent pager
	// has to be forwarded explicitly — same as tab above.
	if key.Matches(msg, inspector.SwarmAgentNextKeyBinding, inspector.SwarmAgentPrevKeyBinding) {
		var cmd tea.Cmd
		m.inspector, cmd = shared.UpdateModel(m.inspector, msg)
		return cmd, true
	}
	// Only claim the compose key when the board can actually use it, so "i" stays
	// available to the rest of the UI.
	if key.Matches(msg, inspector.BoardComposeKeyBinding) && m.inspector.CanCompose() {
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
			m.width,
			m.swarmBoardURL != "",
			m.swarmMaxAgents,
		)
		m.activePopup = popupNewSession
		cmds := []tea.Cmd{m.createForm.Init()}
		if refresh := m.refreshStaleProjectBranchesCmd(initialProjectID); refresh != nil {
			cmds = append(cmds, refresh)
		}
		return tea.Batch(cmds...), true
	}
	if key.Matches(msg, attachKeyBinding) {
		// Dispatch on the typed preview kind rather than the tab label. The
		// Agents Board has no pane behind it and reports ok=false, which is what
		// stops Enter falling through to AttachAgent and lazily spawning an
		// unmanaged extra agent on a swarm.
		kind, agentIndex, ok := m.inspector.ActivePreviewKind()
		if ok {
			switch kind {
			case service.PreviewKindShell:
				if cmd := m.attachSelectedSessionShellCmd(); cmd != nil {
					return cmd, true
				}
			case service.PreviewKindEditor:
				if cmd := m.attachSelectedSessionEditorCmd(); cmd != nil {
					return cmd, true
				}
			case service.PreviewKindAgent:
				if cmd := m.attachSelectedSessionAgentCmd(agentIndex); cmd != nil {
					return cmd, true
				}
			}
		}
	}
	if key.Matches(msg, openEditorKeyBinding) {
		if cmd := m.revealAndAttachEditorCmd(); cmd != nil {
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

// bootstrapSwarmCmd briefs a freshly created swarm. Returns nil for ordinary
// sessions, or when swarm mode is off and there is therefore no board to point
// the agents at.
func (m Model) bootstrapSwarmCmd(sess domain.Session, goal string) tea.Cmd {
	if !sess.IsSwarm() || m.swarmService == nil || m.swarmBoardURL == "" {
		return nil
	}

	svc := m.swarmService
	boardURL := m.swarmBoardURL
	sessID := sess.ID
	return func() tea.Msg {
		err := svc.Bootstrap(context.Background(), service.BootstrapSwarmRequest{
			SessionID: sessID,
			BoardURL:  boardURL,
			Goal:      goal,
		})
		return shared.SwarmBootstrappedMsg{SessionID: sessID, Err: err}
	}
}

// attachSelectedSessionAgentCmd attaches to an agent pane. agentIndex selects
// which pane of a swarm; it is ignored by single-agent sessions.
func (m Model) attachSelectedSessionAgentCmd(agentIndex int) tea.Cmd {
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
		resp, err := svc.AttachAgent(context.Background(), service.AttachAgentRequest{
			ID:         sessID,
			AgentIndex: agentIndex,
		})
		return shared.SessionAttachReadyMsg{Command: resp.Command, Err: err}
	}
}

// revealAndAttachEditorCmd reveals the inspector's gated Editor tab and, in the
// same batch, prepares an attach into the session's nvim. Pressing "e" both
// surfaces the tab and drops the user straight into the editor; the tab then
// persists so a later Enter re-attaches (see attachSelectedSessionEditorCmd).
func (m *Model) revealAndAttachEditorCmd() tea.Cmd {
	attach := m.attachSelectedSessionEditorCmd()
	if attach == nil {
		return nil
	}
	var revealCmd tea.Cmd
	m.inspector, revealCmd = shared.UpdateModel(m.inspector, inspector.RevealEditorMsg{})
	return tea.Batch(revealCmd, attach)
}

// attachSelectedSessionEditorCmd prepares an attach into the selected session's
// editor tmux session (lazily created running nvim). Returns nil when no
// session is selected.
func (m Model) attachSelectedSessionEditorCmd() tea.Cmd {
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
		resp, err := svc.AttachEditor(context.Background(), service.AttachEditorRequest{ID: sessID})
		return shared.SessionAttachReadyMsg{Command: resp.Command, Err: err}
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

	// The keystroke follows what the active tab represents. On a swarm's Agent
	// tab it goes to that one pane; anywhere else on a swarm — including the
	// Agents Board, which is the default tab and stands for the whole swarm — it
	// goes to every agent.
	//
	// Broadcasting is safe: a bare Enter submits a queued prompt and is a
	// harmless newline otherwise. Previously this discarded the `ok` from
	// ActivePreviewKind, so on the board tab agentIndex fell through as 0 and
	// silently poked agent 1 alone.
	kind, agentIndex, ok := m.inspector.ActivePreviewKind()
	if sess, found := m.leftPane.SelectedSession(); found && sess.IsSwarm() {
		if !ok || kind != service.PreviewKindAgent {
			return func() tea.Msg {
				_, err := svc.SendAgentEnterAll(context.Background(), service.SendAgentEnterAllRequest{ID: sessID})
				return shared.AgentEnterSentMsg{Err: err}
			}
		}
	}

	return func() tea.Msg {
		_, err := svc.SendAgentEnter(context.Background(), service.SendAgentEnterRequest{
			ID:         sessID,
			AgentIndex: agentIndex,
		})
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

	// Refuse when the active tab has no pane behind it. Previously this mapped a
	// tab *label* to a kind and silently defaulted to the shell, so pressing x on
	// the Agents Board killed the shell while the popup claimed otherwise.
	kind, agentIndex, ok := m.inspector.ActivePreviewKind()
	if !ok {
		return nil
	}

	m.killPreviewForm = sessionui.NewKillPreviewForm(
		m.styles, m.sessionsService, sessID, sess.Name,
		kind, agentIndex, m.inspector.ActiveViewLabel(),
	)
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

// updateCmdline handles a key while the ":" prompt is open: Enter runs the line
// through the same parser as the chat panel's "/" commands, Esc discards it.
func (m Model) updateCmdline(keyMsg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(keyMsg, hardQuitKeyBinding):
		return m, tea.Quit
	case key.Matches(keyMsg, cmdlineCancelKeyBinding):
		m.cmdlineActive = false
		m.cmdline.Blur()
		return m, nil
	case key.Matches(keyMsg, cmdlineSubmitKeyBinding):
		raw := strings.TrimSpace(m.cmdline.Value())
		m.cmdlineActive = false
		m.cmdline.Blur()
		if raw == "" {
			return m, nil
		}
		return m.executeCommand("/" + strings.TrimPrefix(raw, "/"))
	}
	var cmd tea.Cmd
	m.cmdline, cmd = m.cmdline.Update(keyMsg)
	return m, cmd
}

// showToast puts text in the gap line and arms its dismissal.
func (m *Model) showToast(text string) tea.Cmd {
	m.toast = text
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return toastDismissMsg{} })
}

// syncPRsPage hands the page the slices it renders from; cheap enough to run on
// every change of inbox, sessions, projects or agent statuses.
func (m *Model) syncPRsPage() {
	names := make(map[string]bool, len(m.cachedProjects))
	for _, p := range m.cachedProjects {
		names[p.Name] = true
	}
	m.prsPage.SetData(prs.Data{
		Inbox:         m.reviewInbox,
		Sessions:      m.cachedSessions,
		ProjectNames:  names,
		AgentStatuses: m.agentStatuses,
		AutoReview:    m.review.AutoReview,
	})
}

// autoReviewCmd starts review sessions for one poll's eligible PRs; nil while
// auto-review is off.
func (m Model) autoReviewCmd(inbox domain.ReviewInbox) tea.Cmd {
	if !m.review.AutoReview {
		return nil
	}
	svc := m.sessionsService
	req := service.AutoReviewPullRequestsRequest{
		Inbox:           inbox,
		CommandTemplate: m.review.Command,
		MaxPerPoll:      m.review.MaxPerPoll,
	}
	return shared.RequestWithTimeout(autoReviewTimeout,
		func(ctx context.Context) (service.AutoReviewPullRequestsResponse, error) {
			return svc.AutoReviewPullRequests(ctx, req)
		},
		func(resp service.AutoReviewPullRequestsResponse, err error) tea.Msg {
			return shared.AutoReviewCompletedMsg{Created: resp.Created, Err: err}
		},
	)
}

func (m Model) reviewSessionFor(pr domain.PullRequest) (domain.Session, bool) {
	for _, sess := range m.cachedSessions {
		if sess.IsReview() && sess.ReviewPR == pr.Ref() {
			return sess, true
		}
	}
	return domain.Session{}, false
}

// reapFinishedReviewCmd deletes a review session once its PR is merged or
// closed; nil when there is nothing to do.
func (m Model) reapFinishedReviewCmd(sessionID uuid.UUID, pr domain.PullRequest) tea.Cmd {
	idx := slices.IndexFunc(m.cachedSessions, func(s domain.Session) bool { return s.ID == sessionID })
	if idx < 0 || !domain.ReviewFinished(m.cachedSessions[idx], pr) {
		return nil
	}
	sess := m.cachedSessions[idx]
	svc := m.sessionsService
	return func() tea.Msg {
		_, err := svc.Delete(context.Background(), service.DeleteSessionRequest{ID: sess.ID})
		return shared.ReviewSessionReapedMsg{Session: sess, Err: err}
	}
}

// startReviewCmd creates a review session for pr in its registered project, or
// says why it cannot.
func (m *Model) startReviewCmd(pr domain.PullRequest) tea.Cmd {
	idx := slices.IndexFunc(m.cachedProjects, func(p domain.Project) bool { return p.Name == pr.RepoName() })
	if idx < 0 {
		return m.showToast(fmt.Sprintf("no local repo for %s — clone it under a discovery path first", pr.Repo))
	}
	svc := m.sessionsService
	req := service.CreateReviewSessionRequest{
		PullRequest:  pr,
		ProjectID:    m.cachedProjects[idx].ID,
		AgentCommand: strings.ReplaceAll(m.review.Command, domain.ReviewCommandNumberToken, strconv.Itoa(pr.Number)),
	}
	return shared.RequestWithTimeout(overseerRequestTimeout,
		func(ctx context.Context) (service.CreateReviewSessionResponse, error) {
			return svc.CreateForPullRequest(ctx, req)
		},
		func(resp service.CreateReviewSessionResponse, err error) tea.Msg {
			return shared.ReviewSessionStartedMsg{Session: resp.Session, Err: err}
		},
	)
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
	// The ":" prompt takes the help bar's line at the bottom, k9s style.
	helpView := m.helpBar.View().Content
	switch {
	case m.cmdlineActive:
		helpView = m.cmdline.View()
	case m.page == pagePRs:
		helpView = m.prsHelpBar.View().Content
	}
	helpHeight := max(lipgloss.Height(helpView), 1)

	bodyHeight := max(m.height-titlebarHeight-TitleBarGap-helpHeight, 1)
	leftWidth := m.width * SessionsListWidthPercent / 100
	rightWidth := m.width - leftWidth

	// When the chat panel is visible the body shrinks to make room for it.
	if m.chatPanelVisible {
		chatHeight := max(bodyHeight*ChatPanelHeightPercent/100, 4)
		bodyHeight = max(bodyHeight-chatHeight, 1)
	}

	var body string
	if m.page == pagePRs {
		body = fit(m.styles, m.prsPage.View().Content, m.width, bodyHeight)
	} else {
		left := fit(m.styles, m.leftPane.View().Content, leftWidth, bodyHeight)
		right := fit(m.styles, m.inspector.View().Content, rightWidth, bodyHeight)
		body = fit(m.styles, lipgloss.JoinHorizontal(lipgloss.Top, left, right), m.width, bodyHeight)
	}

	// The gap line between the titlebar and body doubles as a notification
	// slot: it shows a right-aligned toast while repo discovery is running
	// or briefly after it completes. The line already exists in the layout
	// so no height shift occurs.
	var gapLine string
	switch {
	case m.discovering:
		gapLine = m.styles.Toast.GapLine.Width(m.width).
			Render(m.styles.Toast.Indexing.Render(" Indexing repos… "))
	case m.toast != "":
		gapLine = m.styles.Toast.GapLine.Width(m.width).
			Render(m.styles.Toast.Done.Render(" " + m.toast + " "))
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
	m.prsPage.SetSize(m.width, bodyHeight)
	m.cmdline.SetWidth(m.width - 1)
	m.helpBar.SetSize(m.width, HelpBarHeight)
	m.prsHelpBar.SetSize(m.width, HelpBarHeight)
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
