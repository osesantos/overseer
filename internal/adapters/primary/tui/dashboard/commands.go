package dashboard

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	sessionui "github.com/dnlopes/overseer/internal/adapters/primary/tui/session"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/core/service"
)

const (
	loopCheckInterval      = 5 * time.Second
	loopMaxIterations      = 40
	loopMaxConsecutiveWaits = 8 // stop if the agent is still working for this many consecutive iterations
	overseerRequestTimeout = 60 * time.Second
)

// overseerLoopEvalResultMsg carries the result of a single EvaluateLoop
// call. It is an unexported dashboard-internal message.
type overseerLoopEvalResultMsg struct {
	state domain.LoopState // copy of state at eval time
	eval  domain.LoopEvaluation
	err   error
}

// overseerLoopNextTickMsg triggers the next evaluation in a loop's polling
// chain. It is unexported and only handled by the dashboard.
type overseerLoopNextTickMsg struct {
	sessionID string // string so tea.Tick closure can capture it cheaply
}

// executeCommand parses the raw slash-command emitted by the chat panel and
// routes it to the appropriate handler. The returned (tea.Model, tea.Cmd)
// follows the standard Bubble Tea convention.
func (m Model) executeCommand(raw string) (tea.Model, tea.Cmd) {
	name, args := parseCommand(raw)

	switch name {
	case "send":
		return m.cmdSend(args)
	case "delete":
		return m.cmdDelete(args)
	case "new":
		return m.cmdNew()
	case "list":
		return m.cmdList()
	case "help":
		return m.cmdHelp()
	case "loop":
		return m.cmdLoop(args)
	default:
		return m.commandResult(fmt.Sprintf("unknown command %q — type /help for a list of commands", name), true)
	}
}

// parseCommand splits a raw "/cmd arg1 arg2…" string into the command name
// (without the leading slash) and a slice of remaining words.
func parseCommand(raw string) (name string, args []string) {
	parts := strings.Fields(strings.TrimSpace(raw))
	if len(parts) == 0 {
		return "", nil
	}
	name = strings.TrimPrefix(strings.ToLower(parts[0]), "/")
	if len(parts) > 1 {
		args = parts[1:]
	}
	return name, args
}

// commandResult is a helper that emits an OverseerCommandResultMsg so the
// chat panel renders a dimmed system line. It also stops the spinner.
func (m Model) commandResult(text string, isError bool) (tea.Model, tea.Cmd) {
	return m, shared.Emit(shared.OverseerCommandResultMsg{Text: text, IsError: isError})
}

// --- /send ---

func (m Model) cmdSend(args []string) (tea.Model, tea.Cmd) {
	if len(args) < 2 {
		return m.commandResult("usage: /send <session-name> <prompt…>", true)
	}
	sess, prompt, ok := matchSession(args, m.cachedSessions)
	if !ok {
		return m.commandResult(fmt.Sprintf("no session found matching %q", strings.Join(args, " ")), true)
	}
	if strings.TrimSpace(prompt) == "" {
		return m.commandResult("usage: /send <session-name> <prompt…>", true)
	}
	svc := m.sessionsService
	sessionName := sess.Name
	return m, shared.Request(
		func(ctx context.Context) (service.SendAgentPromptResponse, error) {
			return svc.SendAgentPrompt(ctx, service.SendAgentPromptRequest{
				ID:     sess.ID,
				Prompt: prompt,
			})
		},
		func(_ service.SendAgentPromptResponse, err error) tea.Msg {
			return shared.OverseerPromptSentMsg{SessionName: sessionName, Err: err}
		},
	)
}

// --- /delete ---

func (m Model) cmdDelete(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		return m.commandResult("usage: /delete <session-name>", true)
	}
	sess, _, ok := matchSession(args, m.cachedSessions)
	if !ok {
		return m.commandResult(fmt.Sprintf("no session found matching %q", strings.Join(args, " ")), true)
	}
	// Re-use the existing delete popup flow.
	m.deleteForm = sessionui.NewDeleteForm(m.styles, m.sessionsService, sess)
	m.activePopup = popupDeleteSession
	m.chatPanelVisible = false
	m.reapplySize()
	return m, m.deleteForm.Init()
}

// --- /new ---

func (m Model) cmdNew() (tea.Model, tea.Cmd) {
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
	m.chatPanelVisible = false
	m.reapplySize()
	cmds := []tea.Cmd{m.createForm.Init()}
	if refresh := m.refreshStaleProjectBranchesCmd(initialProjectID); refresh != nil {
		cmds = append(cmds, refresh)
	}
	return m, tea.Batch(cmds...)
}

// --- /list ---

func (m Model) cmdList() (tea.Model, tea.Cmd) {
	if len(m.cachedSessions) == 0 {
		return m.commandResult("no sessions", false)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d session(s):\n", len(m.cachedSessions))
	for _, sess := range m.cachedSessions {
		status := domain.AgentStatusUnknown
		if st, ok := m.agentStatuses[sess.ID]; ok {
			status = st.Kind
		}
		loopNote := ""
		if ls, ok := m.loops[sess.ID]; ok && ls.Status == domain.LoopStatusRunning {
			loopNote = " ⟳"
		}
		fmt.Fprintf(&b, "  • %s  [%s]%s\n", sess.Name, string(status), loopNote)
	}
	return m.commandResult(strings.TrimRight(b.String(), "\n"), false)
}

// --- /help ---

func (m Model) cmdHelp() (tea.Model, tea.Cmd) {
	help := strings.Join([]string{
		"Operator commands:",
		"  /send <session> <prompt…>       — send a prompt to a session",
		"  /delete <session>               — open the delete-session dialog",
		"  /new                            — open the new-session dialog",
		"  /list                           — list all sessions",
		"  /loop <session> <criteria…>     — start an evaluation loop",
		"  /loop stop <session>            — stop a running loop",
		"  /loop info <session>            — show loop status",
		"  /help                           — show this help",
		"",
		"Agent mode: type anything without a leading / to chat with the Overseer Agent.",
	}, "\n")
	return m.commandResult(help, false)
}

// --- /loop ---

func (m Model) cmdLoop(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		return m.commandResult("usage: /loop <session> <criteria…>  |  /loop stop <session>  |  /loop info <session>", true)
	}

	subCmd := strings.ToLower(args[0])
	switch subCmd {
	case "stop":
		return m.cmdLoopStop(args[1:])
	case "info":
		return m.cmdLoopInfo(args[1:])
	}

	// /loop <session> <criteria…>
	if m.overseerService == nil {
		return m.commandResult("overseer agent is not configured", true)
	}
	sess, criteria, ok := matchSession(args, m.cachedSessions)
	if !ok {
		return m.commandResult(fmt.Sprintf("no session found matching %q", strings.Join(args, " ")), true)
	}
	if strings.TrimSpace(criteria) == "" {
		return m.commandResult("usage: /loop <session> <criteria…>", true)
	}

	if existing, running := m.loops[sess.ID]; running && existing.Status == domain.LoopStatusRunning {
		return m.commandResult(fmt.Sprintf("a loop is already running on %q — use /loop stop %s first", sess.Name, sess.Name), true)
	}

	ls := &domain.LoopState{
		SessionID:     sess.ID,
		SessionName:   sess.Name,
		Criteria:      criteria,
		Status:        domain.LoopStatusRunning,
		Iterations:    0,
		MaxIterations: loopMaxIterations,
		StartedAt:     time.Now(),
	}
	m.loops[sess.ID] = ls

	note := fmt.Sprintf("Loop started on %q — checking every %ds, max %d iterations.\nCriteria: %s",
		sess.Name, int(loopCheckInterval.Seconds()), loopMaxIterations, criteria)
	return m, tea.Batch(
		shared.Emit(shared.OverseerCommandResultMsg{Text: note}),
		m.broadcastLoopState(),
		m.loopEvalCmd(*ls),
	)
}

func (m Model) cmdLoopStop(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		return m.commandResult("usage: /loop stop <session>", true)
	}
	sess, _, ok := matchSession(args, m.cachedSessions)
	if !ok {
		return m.commandResult(fmt.Sprintf("no session found matching %q", strings.Join(args, " ")), true)
	}
	ls, exists := m.loops[sess.ID]
	if !exists || ls.Status != domain.LoopStatusRunning {
		return m.commandResult(fmt.Sprintf("no running loop found for %q", sess.Name), true)
	}
	ls.Status = domain.LoopStatusStopped
	return m, tea.Batch(
		m.broadcastLoopState(),
		shared.Emit(shared.OverseerCommandResultMsg{Text: fmt.Sprintf("Loop stopped on %q after %d iteration(s).", sess.Name, ls.Iterations)}),
	)
}

func (m Model) cmdLoopInfo(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		return m.commandResult("usage: /loop info <session>", true)
	}
	sess, _, ok := matchSession(args, m.cachedSessions)
	if !ok {
		return m.commandResult(fmt.Sprintf("no session found matching %q", strings.Join(args, " ")), true)
	}
	ls, exists := m.loops[sess.ID]
	if !exists {
		return m.commandResult(fmt.Sprintf("no loop found for %q", sess.Name), false)
	}
	elapsed := time.Since(ls.StartedAt).Round(time.Second)
	info := fmt.Sprintf("Loop on %q — status: %s, iterations: %d/%d, running for: %s\nCriteria: %s",
		sess.Name, string(ls.Status), ls.Iterations, ls.MaxIterations, elapsed, ls.Criteria)
	return m.commandResult(info, false)
}

// loopEvalCmd builds the async Cmd that captures the session's pane and
// calls EvaluateLoop. The evaluation result is returned as an
// overseerLoopEvalResultMsg.
func (m *Model) loopEvalCmd(ls domain.LoopState) tea.Cmd {
	svc := m.overseerService
	sessSvc := m.sessionsService
	return shared.RequestWithTimeout(
		overseerRequestTimeout,
		func(ctx context.Context) (domain.LoopEvaluation, error) {
			resp, err := sessSvc.PreviewSession(ctx, service.PreviewSessionRequest{
				ID:   ls.SessionID,
				Kind: service.PreviewKindAgent,
			})
			pane := ""
			if err == nil && resp.SessionReady {
				pane = truncateLines(resp.Content, 80)
			}
			return svc.EvaluateLoop(ctx, ls.Criteria, pane)
		},
		func(eval domain.LoopEvaluation, err error) tea.Msg {
			return overseerLoopEvalResultMsg{state: ls, eval: eval, err: err}
		},
	)
}

// loopNextTickCmd schedules the next evaluation for a loop after the
// configured interval.
func loopNextTickCmd(ls domain.LoopState) tea.Cmd {
	return tea.Tick(loopCheckInterval, func(time.Time) tea.Msg {
		return overseerLoopNextTickMsg{sessionID: ls.SessionID.String()}
	})
}

// broadcastLoopState copies the current loops map into an
// OverseerLoopStateChangedMsg. Callers must call this whenever m.loops
// changes so sessiondetails and the session list stay up to date.
func (m *Model) broadcastLoopState() tea.Cmd {
	cp := make(map[uuid.UUID]*domain.LoopState, len(m.loops))
	for k, v := range m.loops {
		cp[k] = v
	}
	return shared.Emit(shared.OverseerLoopStateChangedMsg{Loops: cp})
}

// matchSession tries to find a session whose name matches the longest prefix
// of args (case-insensitive). It returns the matched session, the remaining
// words joined as a single string, and whether a match was found.
//
// This handles multi-word session names (e.g. "overseer improvements") by
// trying progressively longer prefixes.
func matchSession(args []string, sessions []domain.Session) (domain.Session, string, bool) {
	for end := len(args); end >= 1; end-- {
		candidate := strings.ToLower(strings.Join(args[:end], " "))
		for _, sess := range sessions {
			if strings.ToLower(sess.Name) == candidate {
				rest := strings.Join(args[end:], " ")
				return sess, rest, true
			}
		}
	}
	return domain.Session{}, "", false
}
