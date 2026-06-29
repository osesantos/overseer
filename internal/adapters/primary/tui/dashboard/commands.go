package dashboard

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	sessionui "github.com/dnlopes/overseer/internal/adapters/primary/tui/session"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/core/service"
)

const (
	overseerRequestTimeout = 60 * time.Second
)

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
		fmt.Fprintf(&b, "  • %s  [%s]\n", sess.Name, string(status))
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
		"  /help                           — show this help",
		"",
		"Agent mode: type anything without a leading / to chat with the Overseer Agent.",
	}, "\n")
	return m.commandResult(help, false)
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
