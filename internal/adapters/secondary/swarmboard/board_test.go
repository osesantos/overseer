package swarmboard_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/secondary/swarmboard"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/shared/paths"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newBoard(t *testing.T) (*swarmboard.Board, paths.Resolver) {
	t.Helper()
	resolver := paths.NewResolver(t.TempDir())
	return swarmboard.New(resolver, discardLogger()), resolver
}

func mustMessage(t *testing.T, sessionID uuid.UUID, author, content string) domain.SwarmMessage {
	t.Helper()
	msg, err := domain.NewSwarmMessage(sessionID, author, domain.SwarmRoleAgent, content)
	if err != nil {
		t.Fatalf("domain.NewSwarmMessage() error = %v", err)
	}
	return msg
}

func TestBoard_Append_AssignsMonotonicSeqStartingAtOne(t *testing.T) {
	board, _ := newBoard(t)
	sessionID := uuid.New()
	ctx := context.Background()

	for want := 1; want <= 3; want++ {
		stored, err := board.Append(ctx, mustMessage(t, sessionID, "agent-1", "post"))
		if err != nil {
			t.Fatalf("Append() error = %v", err)
		}
		if stored.Seq != want {
			t.Fatalf("Append() Seq = %d, want %d", stored.Seq, want)
		}
	}
}

func TestBoard_Append_PreservesMessageContent(t *testing.T) {
	board, _ := newBoard(t)
	sessionID := uuid.New()

	msg, err := domain.NewSwarmMessage(sessionID, "agent-2", domain.SwarmRoleHuman, "look at store.go")
	if err != nil {
		t.Fatalf("domain.NewSwarmMessage() error = %v", err)
	}

	stored, err := board.Append(context.Background(), msg)
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if stored.Author != "agent-2" {
		t.Fatalf("Append() Author = %q, want %q", stored.Author, "agent-2")
	}
	if stored.Role != domain.SwarmRoleHuman {
		t.Fatalf("Append() Role = %q, want %q", stored.Role, domain.SwarmRoleHuman)
	}
	if stored.Content != "look at store.go" {
		t.Fatalf("Append() Content = %q", stored.Content)
	}
	if stored.SessionID != sessionID {
		t.Fatalf("Append() SessionID = %v, want %v", stored.SessionID, sessionID)
	}
}

func TestBoard_ListSince_ZeroReturnsWholeBoardAscending(t *testing.T) {
	board, _ := newBoard(t)
	sessionID := uuid.New()
	ctx := context.Background()

	for _, content := range []string{"first", "second", "third"} {
		if _, err := board.Append(ctx, mustMessage(t, sessionID, "agent-1", content)); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	got, err := board.ListSince(ctx, sessionID, 0)
	if err != nil {
		t.Fatalf("ListSince() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("ListSince() length = %d, want 3", len(got))
	}
	for i, want := range []string{"first", "second", "third"} {
		if got[i].Content != want {
			t.Fatalf("ListSince()[%d].Content = %q, want %q", i, got[i].Content, want)
		}
		if got[i].Seq != i+1 {
			t.Fatalf("ListSince()[%d].Seq = %d, want %d", i, got[i].Seq, i+1)
		}
	}
}

func TestBoard_ListSince_ReturnsOnlyNewerMessages(t *testing.T) {
	board, _ := newBoard(t)
	sessionID := uuid.New()
	ctx := context.Background()

	for _, content := range []string{"a", "b", "c", "d"} {
		if _, err := board.Append(ctx, mustMessage(t, sessionID, "agent-1", content)); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	got, err := board.ListSince(ctx, sessionID, 2)
	if err != nil {
		t.Fatalf("ListSince() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListSince(since=2) length = %d, want 2", len(got))
	}
	if got[0].Content != "c" || got[1].Content != "d" {
		t.Fatalf("ListSince(since=2) = %q,%q, want c,d", got[0].Content, got[1].Content)
	}
}

func TestBoard_ListSince_AtLatestSeqReturnsEmpty(t *testing.T) {
	board, _ := newBoard(t)
	sessionID := uuid.New()
	ctx := context.Background()

	stored, err := board.Append(ctx, mustMessage(t, sessionID, "agent-1", "only"))
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	got, err := board.ListSince(ctx, sessionID, stored.Seq)
	if err != nil {
		t.Fatalf("ListSince() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ListSince(since=latest) length = %d, want 0", len(got))
	}
}

func TestBoard_ListSince_UnknownSessionReturnsEmptyWithoutError(t *testing.T) {
	board, _ := newBoard(t)

	got, err := board.ListSince(context.Background(), uuid.New(), 0)
	if err != nil {
		t.Fatalf("ListSince() error = %v, want nil for a session with no board yet", err)
	}
	if len(got) != 0 {
		t.Fatalf("ListSince() length = %d, want 0", len(got))
	}
}

func TestBoard_Count_ReflectsAppends(t *testing.T) {
	board, _ := newBoard(t)
	sessionID := uuid.New()
	ctx := context.Background()

	if got, err := board.Count(ctx, sessionID); err != nil || got != 0 {
		t.Fatalf("Count() on empty board = %d, %v, want 0, nil", got, err)
	}

	for range 5 {
		if _, err := board.Append(ctx, mustMessage(t, sessionID, "agent-1", "post")); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	got, err := board.Count(ctx, sessionID)
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if got != 5 {
		t.Fatalf("Count() = %d, want 5", got)
	}
}

func TestBoard_Purge_RemovesBoardAndRestartsSeq(t *testing.T) {
	board, resolver := newBoard(t)
	sessionID := uuid.New()
	ctx := context.Background()

	for range 3 {
		if _, err := board.Append(ctx, mustMessage(t, sessionID, "agent-1", "post")); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	if err := board.Purge(ctx, sessionID); err != nil {
		t.Fatalf("Purge() error = %v", err)
	}

	if _, err := os.Stat(resolver.SwarmDir(sessionID)); !os.IsNotExist(err) {
		t.Fatalf("Purge() left %q behind, want it removed", resolver.SwarmDir(sessionID))
	}

	got, err := board.ListSince(ctx, sessionID, 0)
	if err != nil {
		t.Fatalf("ListSince() after purge error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ListSince() after purge length = %d, want 0", len(got))
	}

	stored, err := board.Append(ctx, mustMessage(t, sessionID, "agent-1", "fresh"))
	if err != nil {
		t.Fatalf("Append() after purge error = %v", err)
	}
	if stored.Seq != 1 {
		t.Fatalf("Append() after purge Seq = %d, want 1 (sequence restarts with the board)", stored.Seq)
	}
}

func TestBoard_Purge_UnknownSessionIsNoOp(t *testing.T) {
	board, _ := newBoard(t)

	if err := board.Purge(context.Background(), uuid.New()); err != nil {
		t.Fatalf("Purge() on unknown session error = %v, want nil", err)
	}
}

func TestBoard_Append_IsolatesSessions(t *testing.T) {
	board, _ := newBoard(t)
	first, second := uuid.New(), uuid.New()
	ctx := context.Background()

	if _, err := board.Append(ctx, mustMessage(t, first, "agent-1", "for first")); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	stored, err := board.Append(ctx, mustMessage(t, second, "agent-1", "for second"))
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if stored.Seq != 1 {
		t.Fatalf("second session first Append() Seq = %d, want 1 (sequences are per session)", stored.Seq)
	}

	got, err := board.ListSince(ctx, second, 0)
	if err != nil {
		t.Fatalf("ListSince() error = %v", err)
	}
	if len(got) != 1 || got[0].Content != "for second" {
		t.Fatalf("ListSince(second) = %+v, want only the second session's message", got)
	}
}

func TestBoard_Append_ResumesSeqAcrossInstances(t *testing.T) {
	resolver := paths.NewResolver(t.TempDir())
	sessionID := uuid.New()
	ctx := context.Background()

	first := swarmboard.New(resolver, discardLogger())
	for range 2 {
		if _, err := first.Append(ctx, mustMessage(t, sessionID, "agent-1", "post")); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	// A fresh Board (as after an Overseer restart) must read the existing board
	// rather than overwrite it, and continue the sequence where it left off.
	second := swarmboard.New(resolver, discardLogger())

	stored, err := second.Append(ctx, mustMessage(t, sessionID, "agent-1", "after restart"))
	if err != nil {
		t.Fatalf("Append() on new instance error = %v", err)
	}
	if stored.Seq != 3 {
		t.Fatalf("Append() on new instance Seq = %d, want 3", stored.Seq)
	}

	got, err := second.ListSince(ctx, sessionID, 0)
	if err != nil {
		t.Fatalf("ListSince() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("ListSince() length = %d, want 3 (history must survive a restart)", len(got))
	}
}

func TestBoard_ListSince_SkipsMalformedLinesAndKeepsTheRest(t *testing.T) {
	board, resolver := newBoard(t)
	sessionID := uuid.New()
	ctx := context.Background()

	if _, err := board.Append(ctx, mustMessage(t, sessionID, "agent-1", "good one")); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	// Simulate a torn append: a partial line appended after a clean record.
	boardFile := resolver.SwarmBoardFile(sessionID)
	f, err := os.OpenFile(boardFile, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open board file: %v", err)
	}
	if _, err := f.WriteString("{\"v\":1,\"seq\":2,\"content\":\n"); err != nil {
		t.Fatalf("write partial line: %v", err)
	}
	f.Close()

	got, err := board.ListSince(ctx, sessionID, 0)
	if err != nil {
		t.Fatalf("ListSince() error = %v, want the malformed line skipped rather than a hard failure", err)
	}
	if len(got) != 1 || got[0].Content != "good one" {
		t.Fatalf("ListSince() = %+v, want just the intact record", got)
	}

	if _, err := os.Stat(boardFile); err != nil {
		t.Fatalf("board file must never be deleted on a parse failure: %v", err)
	}
}

func TestBoard_Append_ConcurrentAppendsGetUniqueSequentialSeqs(t *testing.T) {
	board, _ := newBoard(t)
	sessionID := uuid.New()
	ctx := context.Background()

	const writers = 20
	var wg sync.WaitGroup
	seqs := make([]int, writers)
	errs := make([]error, writers)

	for i := range writers {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			stored, err := board.Append(ctx, mustMessage(t, sessionID, "agent-1", "concurrent"))
			seqs[slot] = stored.Seq
			errs[slot] = err
		}(i)
	}
	wg.Wait()

	seen := make(map[int]bool, writers)
	for i, err := range errs {
		if err != nil {
			t.Fatalf("Append() [%d] error = %v", i, err)
		}
		if seen[seqs[i]] {
			t.Fatalf("Append() produced duplicate Seq %d", seqs[i])
		}
		seen[seqs[i]] = true
	}
	for want := 1; want <= writers; want++ {
		if !seen[want] {
			t.Fatalf("Append() never produced Seq %d, want a gapless 1..%d sequence", want, writers)
		}
	}

	got, err := board.ListSince(ctx, sessionID, 0)
	if err != nil {
		t.Fatalf("ListSince() error = %v", err)
	}
	if len(got) != writers {
		t.Fatalf("ListSince() length = %d, want %d — a concurrent append was lost", len(got), writers)
	}
}

func TestBoard_WriteDescriptor_WritesTheAgentContract(t *testing.T) {
	board, resolver := newBoard(t)
	sessionID := uuid.New()

	desc, err := domain.NewSwarmDescriptor(sessionID, "http://127.0.0.1:54321", 3, "summarise the storage layer")
	if err != nil {
		t.Fatalf("domain.NewSwarmDescriptor() error = %v", err)
	}

	if err := board.Write(context.Background(), desc); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	data, err := os.ReadFile(resolver.SwarmDescriptorFile(sessionID))
	if err != nil {
		t.Fatalf("read descriptor: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("descriptor is not valid JSON: %v", err)
	}

	if got["sessionId"] != sessionID.String() {
		t.Fatalf("descriptor sessionId = %v, want %v", got["sessionId"], sessionID)
	}
	if got["boardUrl"] != "http://127.0.0.1:54321" {
		t.Fatalf("descriptor boardUrl = %v", got["boardUrl"])
	}
	if got["goal"] != "summarise the storage layer" {
		t.Fatalf("descriptor goal = %v", got["goal"])
	}
	roster, ok := got["roster"].([]any)
	if !ok || len(roster) != 3 {
		t.Fatalf("descriptor roster = %v, want 3 entries", got["roster"])
	}
	if roster[0] != "agent-1" {
		t.Fatalf("descriptor roster[0] = %v, want agent-1", roster[0])
	}

	// The descriptor must teach the agent how to reach the board, otherwise the
	// injected prompt has to carry the whole protocol.
	for _, key := range []string{"post", "read"} {
		hint, ok := got[key].(string)
		if !ok || hint == "" {
			t.Fatalf("descriptor %q = %v, want a non-empty wire hint", key, got[key])
		}
		if !strings.Contains(hint, sessionID.String()) {
			t.Fatalf("descriptor %q = %q, want it to reference the session id", key, hint)
		}
	}
}

func TestBoard_WriteDescriptor_CarriesPostingConventions(t *testing.T) {
	// Ungoverned, agents write multi-kilobyte essays per post and the board stops
	// being readable. The conventions live in the descriptor so an agent can
	// re-read them, rather than only in the briefing it saw once.
	board, resolver := newBoard(t)
	sessionID := uuid.New()

	desc, err := domain.NewSwarmDescriptor(sessionID, "http://127.0.0.1:1", 3, "goal")
	if err != nil {
		t.Fatalf("domain.NewSwarmDescriptor() error = %v", err)
	}
	if err := board.Write(context.Background(), desc); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	data, err := os.ReadFile(resolver.SwarmDescriptorFile(sessionID))
	if err != nil {
		t.Fatalf("read descriptor: %v", err)
	}

	var got struct {
		Method      []string `json:"method"`
		Conventions []string `json:"conventions"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("descriptor is not valid JSON: %v", err)
	}
	if len(got.Conventions) == 0 {
		t.Fatal("descriptor carries no posting conventions")
	}

	joined := strings.ToLower(strings.Join(got.Conventions, " "))
	for _, want := range []string{"line", "evidence"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("conventions never mention %q: %v", want, got.Conventions)
		}
	}

	// Method is a separate list: how to work, versus how to write it up.
	if len(got.Method) == 0 {
		t.Fatal("descriptor carries no working method")
	}
	method := strings.ToLower(strings.Join(got.Method, " "))
	for _, want := range []string{
		"break",    // falsify your own claim before posting
		"claim",    // take a distinct axis, earliest claim wins
		"confiden", // state confidence and what would change your mind
		"verdict",  // conclude without being asked
	} {
		if !strings.Contains(method, want) {
			t.Fatalf("method never mentions %q: %v", want, got.Method)
		}
	}
}

func TestBoard_WriteDescriptor_OverwritesPreviousContract(t *testing.T) {
	board, resolver := newBoard(t)
	sessionID := uuid.New()
	ctx := context.Background()

	first, _ := domain.NewSwarmDescriptor(sessionID, "http://127.0.0.1:1", 2, "old goal")
	second, _ := domain.NewSwarmDescriptor(sessionID, "http://127.0.0.1:2", 3, "new goal")

	if err := board.Write(ctx, first); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := board.Write(ctx, second); err != nil {
		t.Fatalf("Write() second error = %v", err)
	}

	data, err := os.ReadFile(resolver.SwarmDescriptorFile(sessionID))
	if err != nil {
		t.Fatalf("read descriptor: %v", err)
	}
	if !strings.Contains(string(data), "new goal") || strings.Contains(string(data), "old goal") {
		t.Fatal("Write() did not replace the previous descriptor")
	}

	// AtomicWrite must not leave its scratch file behind.
	if _, err := os.Stat(resolver.SwarmDescriptorFile(sessionID) + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("Write() left a .tmp file behind")
	}
}

func TestBoard_Append_CreatesSwarmDirectoryOnDemand(t *testing.T) {
	board, resolver := newBoard(t)
	sessionID := uuid.New()

	if _, err := os.Stat(resolver.SwarmDir(sessionID)); !os.IsNotExist(err) {
		t.Fatal("swarm dir should not exist before the first append")
	}

	if _, err := board.Append(context.Background(), mustMessage(t, sessionID, "agent-1", "post")); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	info, err := os.Stat(resolver.SwarmDir(sessionID))
	if err != nil {
		t.Fatalf("Append() did not create the swarm dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("swarm dir is not a directory: %v", info.Mode())
	}
	if filepath.Base(resolver.SwarmBoardFile(sessionID)) != "messages.jsonl" {
		t.Fatalf("board file name = %q, want messages.jsonl", filepath.Base(resolver.SwarmBoardFile(sessionID)))
	}
}
