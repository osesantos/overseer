package inspector

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/components"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/core/service"
)

// composeRowHeight is the single row the compose input occupies at the bottom of
// the board when it is open.
const composeRowHeight = 1

// boardView is the swarm's Agents Board: a transcript of everything the agents
// and the operator have said, plus a compose line for joining in.
//
// Unlike the other tabs it does not capture a tmux pane — it polls the board
// through SwarmService, which lives in this same process, so there is no HTTP
// hop. It keeps a Seq watermark and asks only for messages newer than that, so
// a long-running swarm stays cheap to follow.
//
// Compose is an explicit mode rather than an always-live input. The dashboard has
// to hand every keystroke over while it is open, and doing that permanently would
// swallow session navigation and make the tab a trap.
type boardView struct {
	sessionID uuid.UUID
	isSwarm   bool

	messages  []domain.SwarmMessage
	latestSeq int
	// rendered holds each message's body already converted to styled text, index
	// for index. Markdown is too costly to redo for the whole transcript every
	// time an agent posts, so each body is rendered once and only re-rendered
	// when a resize changes the wrap width.
	rendered []string
	markdown styles.Markdown

	viewport      viewport.Model
	input         textinput.Model
	composing     bool
	generation    int
	width, height int
	err           error

	service      *service.SwarmService
	styles       *styles.Styles
	pollInterval time.Duration
}

func newBoardView(svc *service.SwarmService, s *styles.Styles, pollInterval time.Duration) *boardView {
	input := textinput.New()
	input.Placeholder = "Message the swarm…"
	input.Prompt = "» "
	input.CharLimit = 2000
	input.SetStyles(s.Form.Input)

	return &boardView{
		viewport:     viewport.New(),
		input:        input,
		service:      svc,
		styles:       s,
		pollInterval: pollInterval,
	}
}

func (v *boardView) Label() string { return "Agents Board" }

func (v *boardView) Init() tea.Cmd {
	v.generation++
	return v.fetch()
}

// CapturesInput reports whether the board is holding the keyboard. The dashboard
// checks this before claiming any key of its own.
func (v *boardView) CapturesInput() bool { return v.composing }

func (v *boardView) Update(msg tea.Msg) (View, tea.Cmd) {
	switch msg := msg.(type) {
	case swarmBoardLoadedMsg:
		if msg.sessionID != v.sessionID || v.sessionID == uuid.Nil {
			return v, nil
		}
		// Drop results from a superseded chain so a session switch or forced
		// refresh does not leave two pollers running.
		if msg.generation != v.generation {
			return v, nil
		}
		if msg.err != nil {
			v.err = msg.err
			return v, v.scheduleNext()
		}
		v.err = nil
		if len(msg.messages) > 0 {
			for _, posted := range msg.messages {
				v.messages = append(v.messages, posted)
				v.rendered = append(v.rendered, v.renderBody(posted))
			}
			v.latestSeq = msg.latestSeq
			v.refreshViewport()
		} else if msg.latestSeq > v.latestSeq {
			v.latestSeq = msg.latestSeq
		}
		return v, v.scheduleNext()

	case swarmBoardPostedMsg:
		if msg.sessionID != v.sessionID {
			return v, nil
		}
		if msg.err != nil {
			v.err = msg.err
		}
		// Fetch immediately so the operator sees their own line land rather
		// than waiting out the poll interval.
		return v, v.fetch()

	case tea.KeyPressMsg:
		return v.handleKey(msg)
	}

	return v, nil
}

func (v *boardView) handleKey(msg tea.KeyPressMsg) (View, tea.Cmd) {
	if !v.composing {
		if key.Matches(msg, BoardComposeKeyBinding) && v.isSwarm {
			v.composing = true
			v.input.Focus()
			v.applySizes()
			return v, textinput.Blink
		}
		if key.Matches(msg, boardScrollUpKeyBinding, boardScrollDownKeyBinding) {
			var cmd tea.Cmd
			v.viewport, cmd = v.viewport.Update(msg)
			return v, cmd
		}
		return v, nil
	}

	switch {
	case key.Matches(msg, boardComposeCancelKeyBinding):
		v.composing = false
		v.input.Blur()
		v.input.SetValue("")
		v.applySizes()
		return v, nil

	case key.Matches(msg, boardComposeSubmitKeyBinding):
		text := strings.TrimSpace(v.input.Value())
		if text == "" {
			return v, nil
		}
		v.input.SetValue("")
		return v, v.post(text)
	}

	var cmd tea.Cmd
	v.input, cmd = v.input.Update(msg)
	return v, cmd
}

func (v *boardView) Body() string {
	if v.sessionID == uuid.Nil {
		return components.CenteredContent(v.styles,
			v.styles.EmptyState.Title.Render("Select a session to preview"), v.width, v.height)
	}
	if !v.isSwarm {
		return components.CenteredContent(v.styles,
			v.styles.EmptyState.Title.Render("This session is not a swarm"), v.width, v.height)
	}
	if v.err != nil {
		return components.CenteredContent(v.styles,
			v.styles.EmptyState.Title.Render("Board error: "+v.err.Error()), v.width, v.height)
	}
	if len(v.messages) == 0 {
		return components.CenteredContent(v.styles,
			v.styles.EmptyState.Title.Render("No board messages yet — the swarm is starting up"), v.width, v.height)
	}

	body := v.viewport.View()
	if v.composing {
		return body + "\n" + v.input.View()
	}
	return body + "\n" + v.styles.Help.Description.Render(v.footerHint())
}

func (v *boardView) footerHint() string {
	return fmt.Sprintf("%d messages · i: message the swarm", len(v.messages))
}

func (v *boardView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.applySizes()
}

// applySizes recomputes the viewport height, which depends on whether the
// compose row or the footer hint is showing. Both are one row, so the transcript
// height is stable — but keeping this in one place means the two callers
// (resize and mode change) cannot drift.
func (v *boardView) applySizes() {
	vpWidth := max(v.width, 1)
	v.viewport.SetWidth(vpWidth)
	v.viewport.SetHeight(max(v.height-composeRowHeight, 1))
	v.input.SetWidth(max(v.width-len(v.input.Prompt), 1))

	// Markdown wraps at a fixed width, so only a width change invalidates the
	// cached bodies — applySizes also runs on mode changes, which must not pay
	// for a full re-render.
	if v.markdown.Width() != vpWidth {
		v.markdown = v.styles.NewMarkdown(vpWidth)
		v.rendered = make([]string, len(v.messages))
		for i, msg := range v.messages {
			v.rendered[i] = v.renderBody(msg)
		}
	}
	v.refreshViewport()
}

func (v *boardView) SetSession(sess domain.Session) {
	v.sessionID = sess.ID
	v.isSwarm = sess.IsSwarm()
	v.messages = nil
	// Must be cleared alongside messages: the two slices are index-for-index, so
	// leaving stale entries here would pair the next session's posts with the
	// previous session's bodies.
	v.rendered = nil
	v.latestSeq = 0
	v.err = nil
	v.composing = false
	v.input.Blur()
	v.input.SetValue("")
	v.refreshViewport()
}

// refreshViewport re-renders the transcript. It sticks to the bottom only when
// the operator was already there, so scrolling back to read history is not undone
// by the next agent post.
func (v *boardView) refreshViewport() {
	separator := v.styles.Chat.Separator.Render(strings.Repeat("─", max(v.viewport.Width(), 1)))

	var b strings.Builder
	for i, msg := range v.messages {
		body := ""
		if i < len(v.rendered) {
			body = v.rendered[i]
		}
		b.WriteString(v.renderHeader(msg))
		b.WriteByte('\n')
		b.WriteString(body)
		b.WriteByte('\n')
		b.WriteString(separator)
		b.WriteByte('\n')
	}

	atBottom := v.viewport.AtBottom()
	v.viewport.SetContent(b.String())
	if atBottom {
		v.viewport.GotoBottom()
	}
}

// renderHeader is the timestamp-and-author line that precedes a message body.
func (v *boardView) renderHeader(msg domain.SwarmMessage) string {
	s := v.styles

	var author string
	switch msg.Role {
	case domain.SwarmRoleHuman:
		author = s.Board.AuthorHuman.Render(msg.Author)
	case domain.SwarmRoleSystem:
		author = s.Board.SystemText.Render("overseer")
	default:
		author = s.Board.Author.Foreground(s.Board.AuthorColorFor(agentIndexFromAuthor(msg.Author))).Render(msg.Author)
	}

	return fmt.Sprintf("%s %s",
		s.Board.Timestamp.Render(msg.CreatedAt.Format("15:04:05")),
		author,
	)
}

// renderBody converts a message's content to styled text. Agent and human posts
// go through markdown — they are LLM prose and routinely contain code fences and
// lists. Overseer's own system notices stay literal so paths and errors in them
// are not reflowed.
func (v *boardView) renderBody(msg domain.SwarmMessage) string {
	if msg.Role == domain.SwarmRoleSystem {
		return v.styles.Board.SystemText.Render(msg.Content)
	}
	return v.markdown.Render(msg.Content)
}

// agentIndexFromAuthor pulls the number out of an "agent-3" author name so the
// palette can colour it consistently. Unparseable names fall back to slot 1
// rather than failing a render.
func agentIndexFromAuthor(author string) int {
	_, digits, found := strings.Cut(author, "-")
	if !found {
		return 1
	}
	index, err := strconv.Atoi(digits)
	if err != nil || index < 1 {
		return 1
	}
	return index
}

// fetch asks for board messages newer than the current watermark. The session ID
// and generation are captured now so a result arriving after a session switch is
// discarded by Update, terminating the old chain.
func (v *boardView) fetch() tea.Cmd {
	sessID := v.sessionID
	svc := v.service
	since := v.latestSeq
	gen := v.generation
	isSwarm := v.isSwarm

	return func() tea.Msg {
		if sessID == uuid.Nil || !isSwarm || svc == nil {
			return swarmBoardLoadedMsg{sessionID: sessID, generation: gen, latestSeq: since}
		}
		resp, err := svc.ListMessages(context.Background(), service.ListSwarmMessagesRequest{
			SessionID: sessID,
			Since:     since,
		})
		return swarmBoardLoadedMsg{
			sessionID:  sessID,
			generation: gen,
			messages:   resp.Messages,
			latestSeq:  resp.LatestSeq,
			err:        err,
		}
	}
}

func (v *boardView) scheduleNext() tea.Cmd {
	next := v.fetch()
	return tea.Tick(v.pollInterval, func(time.Time) tea.Msg {
		return next()
	})
}

// post sends an operator message to the board. The agents are woken by the
// scheduled nudge flush, not here, so the call returns as soon as the message is
// stored.
func (v *boardView) post(content string) tea.Cmd {
	sessID := v.sessionID
	svc := v.service

	return func() tea.Msg {
		if svc == nil {
			return swarmBoardPostedMsg{sessionID: sessID, err: errSwarmDisabled}
		}
		_, err := svc.Post(context.Background(), service.PostSwarmMessageRequest{
			SessionID: sessID,
			Author:    "human",
			Role:      domain.SwarmRoleHuman,
			Content:   content,
		})
		return swarmBoardPostedMsg{sessionID: sessID, err: err}
	}
}
