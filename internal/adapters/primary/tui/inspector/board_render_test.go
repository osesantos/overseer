package inspector

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/core/domain"
)

// postToBoard feeds messages to the board the way the polling chain does.
func postToBoard(t *testing.T, v *boardView, msgs ...domain.SwarmMessage) {
	t.Helper()
	latest := v.latestSeq
	for _, msg := range msgs {
		if msg.Seq > latest {
			latest = msg.Seq
		}
	}
	updated, _ := v.Update(swarmBoardLoadedMsg{
		sessionID:  v.sessionID,
		generation: v.generation,
		messages:   msgs,
		latestSeq:  latest,
	})
	if got, ok := updated.(*boardView); !ok || got != v {
		t.Fatal("Update did not return the same board view")
	}
}

func boardWithSession(t *testing.T, size int) *boardView {
	t.Helper()
	m := selectSession(t, newTestModel(t), swarmSession(t, size))
	m.SetSize(80, 20)

	board, ok := m.views[ixBoard].(*boardView)
	if !ok {
		t.Fatal("board view has the wrong type")
	}
	return board
}

func agentPost(seq int, author, content string) domain.SwarmMessage {
	return domain.SwarmMessage{
		Seq:       seq,
		Author:    author,
		Role:      domain.SwarmRoleAgent,
		Content:   content,
		CreatedAt: time.Date(2026, 8, 19, 9, 30, 0, 0, time.UTC),
	}
}

func TestBoardView_RendersMessageBodiesAsMarkdown(t *testing.T) {
	board := boardWithSession(t, 3)
	postToBoard(t, board, agentPost(1, "agent-1", "Found it in **store.go**"))

	body := ansi.Strip(board.Body())

	if strings.Contains(body, "**") {
		t.Fatalf("bold markers survived into the board: %q", body)
	}
	if !strings.Contains(body, "store.go") {
		t.Fatalf("board lost the message content: %q", body)
	}
}

func TestBoardView_DrawsASeparatorAfterEachMessage(t *testing.T) {
	board := boardWithSession(t, 3)
	postToBoard(t, board,
		agentPost(1, "agent-1", "first"),
		agentPost(2, "agent-2", "second"),
	)

	body := ansi.Strip(board.Body())
	if got := strings.Count(body, "─"); got == 0 {
		t.Fatalf("no separator drawn: %q", body)
	}

	// One rule per message, so the transcript reads as discrete posts.
	rules := 0
	for _, line := range strings.Split(body, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" && strings.Trim(trimmed, "─") == "" {
			rules++
		}
	}
	if rules != 2 {
		t.Fatalf("separator lines = %d, want 2 (one per message): %q", rules, body)
	}
}

func TestBoardView_CachedBodiesStayAlignedWithMessages(t *testing.T) {
	board := boardWithSession(t, 3)
	postToBoard(t, board,
		agentPost(1, "agent-1", "alpha"),
		agentPost(2, "agent-2", "bravo"),
	)
	postToBoard(t, board, agentPost(3, "agent-3", "charlie"))

	if len(board.rendered) != len(board.messages) {
		t.Fatalf("rendered=%d messages=%d — the caches must stay index-for-index",
			len(board.rendered), len(board.messages))
	}

	body := ansi.Strip(board.Body())
	for _, want := range []string{"alpha", "bravo", "charlie"} {
		if !strings.Contains(body, want) {
			t.Fatalf("board is missing %q: %q", want, body)
		}
	}
}

func TestBoardView_SessionSwitchClearsTheRenderCache(t *testing.T) {
	// Regression: SetSession cleared messages but not rendered, so the next
	// session's posts were paired with the previous session's bodies.
	board := boardWithSession(t, 3)
	postToBoard(t, board, agentPost(1, "agent-1", "from the first session"))

	board.SetSession(swarmSession(t, 2))

	if len(board.rendered) != 0 {
		t.Fatalf("rendered cache survived the session switch: %v", board.rendered)
	}
	if len(board.messages) != 0 {
		t.Fatalf("messages survived the session switch: %v", board.messages)
	}

	postToBoard(t, board, agentPost(1, "agent-1", "from the second session"))

	body := ansi.Strip(board.Body())
	if strings.Contains(body, "from the first session") {
		t.Fatalf("board shows the previous session's message: %q", body)
	}
	if !strings.Contains(body, "from the second session") {
		t.Fatalf("board lost the new message: %q", body)
	}
}

func TestBoardView_ResizeRerendersEveryBodyAtTheNewWidth(t *testing.T) {
	board := boardWithSession(t, 3)
	postToBoard(t, board, agentPost(1, "agent-1", strings.Repeat("word ", 40)))

	board.SetSize(40, 20)

	if got := board.markdown.Width(); got != 40 {
		t.Fatalf("markdown width after resize = %d, want 40", got)
	}
	if len(board.rendered) != len(board.messages) {
		t.Fatalf("rendered=%d messages=%d after resize", len(board.rendered), len(board.messages))
	}
	for _, line := range strings.Split(ansi.Strip(board.Body()), "\n") {
		if w := ansi.StringWidth(line); w > 40 {
			t.Fatalf("line width %d exceeds the new width 40: %q", w, line)
		}
	}
}

func TestBoardView_SystemNoticesAreNotMarkdownRendered(t *testing.T) {
	// Overseer's own notices carry paths and errors that markdown would reflow.
	board := boardWithSession(t, 3)
	postToBoard(t, board, domain.SwarmMessage{
		Seq:       1,
		Author:    "overseer",
		Role:      domain.SwarmRoleSystem,
		Content:   "board capped at 500 messages",
		CreatedAt: time.Date(2026, 8, 19, 9, 30, 0, 0, time.UTC),
	})

	body := ansi.Strip(board.Body())
	if !strings.Contains(body, "board capped at 500 messages") {
		t.Fatalf("system notice not rendered verbatim: %q", body)
	}
}

func TestBoardView_HeaderCarriesTimestampAndAuthor(t *testing.T) {
	board := boardWithSession(t, 3)
	postToBoard(t, board, agentPost(1, "agent-2", "hello"))

	body := ansi.Strip(board.Body())
	if !strings.Contains(body, "09:30:00") {
		t.Fatalf("timestamp missing: %q", body)
	}
	if !strings.Contains(body, "agent-2") {
		t.Fatalf("author missing: %q", body)
	}
}

func TestBoardView_UnknownSessionRendersNothingSurprising(t *testing.T) {
	board := newBoardView(nil, newTestModel(t).styles, 0)
	board.SetSession(domain.Session{ID: uuid.Nil})
	board.SetSize(80, 20)

	if body := ansi.Strip(board.Body()); !strings.Contains(body, "Select a session") {
		t.Fatalf("body = %q, want the no-selection placeholder", body)
	}
}
