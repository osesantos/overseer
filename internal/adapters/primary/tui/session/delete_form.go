package session

import (
	"context"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/components"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/core/service"
)

const (
	deletePopupWidth = 80
)

type DeleteFormModel struct {
	sessionID   uuid.UUID
	sessionName string
	hasWorktree bool
	// isSwarm gates the purge toggle: only a swarm session has a message board to
	// purge, so the option is hidden entirely for ordinary sessions.
	isSwarm bool
	// purgeBoard is opt-in. The board outlives the session by default because it
	// records how the swarm reasoned, which usually outlasts the usefulness of the
	// session row itself.
	purgeBoard      bool
	errMsg          string
	sessionsService service.SessionService
	styles          *styles.Styles
}

func NewDeleteForm(s *styles.Styles, sessionsService service.SessionService, sess domain.Session) DeleteFormModel {
	return DeleteFormModel{
		sessionID:       sess.ID,
		sessionName:     sess.Name,
		hasWorktree:     sess.HasWorktree(),
		isSwarm:         sess.IsSwarm(),
		sessionsService: sessionsService,
		styles:          s,
	}
}

func (m DeleteFormModel) Init() tea.Cmd { return nil }

func (m DeleteFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if key.Matches(msg, popupCloseKeyBinding) || key.Matches(msg, deleteCancelKeyBinding) {
			return m, shared.Emit(shared.NewSessionDeletePopupCloseMsg{})
		}
		// Checked before the confirm binding: "p" is not part of that binding, but
		// keeping the order explicit makes it obvious the toggle cannot be
		// swallowed by a future change to the confirm keys.
		if m.isSwarm && key.Matches(msg, deletePurgeBoardKeyBinding) {
			m.purgeBoard = !m.purgeBoard
			return m, nil
		}
		if key.Matches(msg, deleteConfirmKeyBinding) {
			return m.submit()
		}
	case shared.SessionDeleteErrMsg:
		m.errMsg = msg.Err.Error()
		return m, nil
	}
	return m, nil
}

func (m DeleteFormModel) submit() (tea.Model, tea.Cmd) {
	id := m.sessionID
	purgeBoard := m.purgeBoard
	svc := m.sessionsService
	return m, func() tea.Msg {
		_, err := svc.Delete(context.Background(), service.DeleteSessionRequest{
			ID:         id,
			PurgeBoard: purgeBoard,
		})
		if err != nil {
			return shared.SessionDeleteErrMsg{Err: err}
		}
		return shared.SessionDeletedMsg{}
	}
}

func (m DeleteFormModel) View() tea.View {
	field := m.styles.Form.Field
	danger := m.styles.Danger

	var b strings.Builder
	b.WriteString(danger.Title.Render("Delete session"))
	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(field.Label.Render("Session: "))
	b.WriteString(field.LabelFocused.Render(m.sessionName))
	b.WriteByte('\n')
	b.WriteString(m.consequencesHint())
	b.WriteByte('\n')
	if m.isSwarm {
		b.WriteByte('\n')
		b.WriteString(field.Label.Render("Swarm board: "))
		b.WriteString(m.boardDispositionView())
		b.WriteByte('\n')
	}
	b.WriteString(field.Error.Render(m.errMsg))
	b.WriteByte('\n')
	if m.errMsg != "" {
		b.WriteByte('\n')
	}
	b.WriteString(m.styles.Help.Description.Render(m.helpHint()))
	return tea.NewView(components.Modal(m.styles, b.String(), deletePopupWidth, 0))
}

// boardDispositionView states what will happen to the board in plain words. The
// purge is irreversible, so the operator has to be able to read its state off the
// screen rather than remember whether they pressed the key.
func (m DeleteFormModel) boardDispositionView() string {
	if m.purgeBoard {
		return m.styles.Danger.Body.Render("will be PURGED — the transcript is deleted too")
	}
	return m.styles.Form.Field.LabelFocused.Render("kept on disk")
}

func (m DeleteFormModel) helpHint() string {
	if m.isSwarm {
		return "y/enter: confirm delete  p: toggle board purge  n/esc: cancel"
	}
	return "y/enter: confirm delete  n/esc: cancel"
}

func (m DeleteFormModel) consequencesHint() string {
	if m.hasWorktree {
		return m.styles.Danger.Body.Render("This will kill the tmux session, remove the git worktree (uncommitted changes lost), and delete the session record. This cannot be undone.")
	}
	return m.styles.Danger.Body.Render("This will kill the tmux session and delete the session record. This cannot be undone.")
}
