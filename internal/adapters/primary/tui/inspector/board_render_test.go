package inspector

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

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

func TestBoardView_ConcurrentFetchesDoNotDuplicateAMessage(t *testing.T) {
	// Regression: posting triggers an immediate fetch while the scheduled poll is
	// still in flight. Both ask for the same `since` and both carry the same
	// generation, so both used to append the same delta — the operator saw their
	// own message twice until a session switch rebuilt the list.
	board := boardWithSession(t, 3)

	delta := []domain.SwarmMessage{agentPost(1, "human", "focus on the storage layer")}
	loaded := swarmBoardLoadedMsg{
		sessionID:  board.sessionID,
		generation: board.generation,
		messages:   delta,
		latestSeq:  1,
	}

	// Same payload delivered twice, exactly as two racing chains would.
	board.Update(loaded)
	board.Update(loaded)

	if len(board.messages) != 1 {
		t.Fatalf("messages = %d, want 1 — the delta was appended twice", len(board.messages))
	}
	if len(board.rendered) != len(board.messages) {
		t.Fatalf("rendered=%d messages=%d", len(board.rendered), len(board.messages))
	}
	if got := strings.Count(ansi.Strip(board.Body()), "focus on the storage layer"); got != 1 {
		t.Fatalf("message rendered %d times, want 1", got)
	}
}

func TestBoardView_OutOfOrderDeltaDoesNotRegressTheCursor(t *testing.T) {
	board := boardWithSession(t, 3)

	postToBoard(t, board, agentPost(1, "agent-1", "first"), agentPost(2, "agent-2", "second"))

	// A slow chain returning an already-consumed delta must be ignored wholesale.
	board.Update(swarmBoardLoadedMsg{
		sessionID:  board.sessionID,
		generation: board.generation,
		messages:   []domain.SwarmMessage{agentPost(1, "agent-1", "first")},
		latestSeq:  1,
	})

	if len(board.messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(board.messages))
	}
	if board.latestSeq != 2 {
		t.Fatalf("latestSeq = %d, want 2 — the cursor must never go backwards", board.latestSeq)
	}
}

func TestBoardView_PostRetiresTheOldPollWithoutStoppingPolling(t *testing.T) {
	// The duplication fix works by bumping the generation on post so the in-flight
	// scheduled poll is retired. That is only safe if the replacement chain keeps
	// polling — get it wrong and the board silently freezes after the first post.
	board := boardWithSession(t, 3)
	retired := board.generation

	_, cmd := board.Update(swarmBoardPostedMsg{sessionID: board.sessionID})
	if cmd == nil {
		t.Fatal("post produced no immediate fetch")
	}
	if board.generation == retired {
		t.Fatal("generation did not advance; the old poll would run alongside the new one")
	}

	if _, stale := board.Update(swarmBoardLoadedMsg{
		sessionID: board.sessionID, generation: retired,
	}); stale != nil {
		t.Fatal("the retired chain rescheduled itself — two pollers")
	}

	if _, live := board.Update(swarmBoardLoadedMsg{
		sessionID: board.sessionID, generation: board.generation,
	}); live == nil {
		t.Fatal("the live chain did not reschedule — the board would stop updating")
	}
}

func TestBoardView_PasteReachesTheComposeLine(t *testing.T) {
	board := boardWithSession(t, 3)

	// Not composing: nothing to paste into, so it must be ignored.
	board.Update(tea.PasteMsg{Content: "ignored"})
	if got := board.input.Value(); got != "" {
		t.Fatalf("input = %q, want empty when not composing", got)
	}

	// Composing: paste lands in the input.
	board.Update(keyPress("i"))
	if !board.composing {
		t.Fatal("precondition: i did not open the compose line")
	}

	board.Update(tea.PasteMsg{Content: "pasted into the board"})
	if got := board.input.Value(); !strings.Contains(got, "pasted into the board") {
		t.Fatalf("input = %q, want the pasted text", got)
	}
}

func TestInspector_ForwardsPasteToTheActiveView(t *testing.T) {
	m := selectSession(t, newTestModel(t), swarmSession(t, 3))
	m.SetSize(80, 20)

	updated, _ := m.Update(keyPress("i"))
	m = updated.(Model)
	if !m.CapturesInput() {
		t.Fatal("precondition: compose mode not open")
	}

	updated, _ = m.Update(tea.PasteMsg{Content: "via the inspector"})
	m = updated.(Model)

	board := m.views[ixBoard].(*boardView)
	if got := board.input.Value(); !strings.Contains(got, "via the inspector") {
		t.Fatalf("board input = %q, want the paste routed through the inspector", got)
	}
}

func TestBoardView_HeaderLeadsWithTheSequenceNumber(t *testing.T) {
	// Agents cite each other by sequence number — 86% of posts in one review run —
	// so the operator cannot follow the conversation unless the number is on screen.
	board := boardWithSession(t, 3)
	postToBoard(t, board,
		agentPost(1, "agent-1", "first"),
		agentPost(2, "agent-2", "@agent-1 seq 1 is wrong"),
	)

	body := ansi.Strip(board.Body())
	for _, want := range []string{"#1", "#2"} {
		if !strings.Contains(body, want) {
			t.Fatalf("board does not show %q: %q", want, body)
		}
	}

	// The number must precede the timestamp on its line, not trail it.
	for _, line := range strings.Split(body, "\n") {
		if !strings.Contains(line, "09:30:00") {
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			t.Fatalf("header line does not lead with the sequence number: %q", line)
		}
	}
}

func TestBoardView_SequenceNumbersKeepTimestampsAligned(t *testing.T) {
	// A board runs to hundreds of messages; unpadded numbers would ragged the
	// timestamp column and make it much harder to scan.
	board := boardWithSession(t, 3)
	postToBoard(t, board, agentPost(1, "agent-1", "one"), agentPost(42, "agent-2", "forty-two"))

	var columns []int
	for _, line := range strings.Split(ansi.Strip(board.Body()), "\n") {
		if idx := strings.Index(line, "09:30:00"); idx >= 0 {
			columns = append(columns, idx)
		}
	}
	if len(columns) != 2 {
		t.Fatalf("expected 2 header lines, found %d", len(columns))
	}
	if columns[0] != columns[1] {
		t.Fatalf("timestamp columns differ (%d vs %d) — the seq field is not padded", columns[0], columns[1])
	}
}
