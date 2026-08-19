package boardhttp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/boardhttp"
	"github.com/dnlopes/overseer/internal/adapters/secondary/storage"
	"github.com/dnlopes/overseer/internal/adapters/secondary/swarmboard"
	"github.com/dnlopes/overseer/internal/adapters/secondary/tmux"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/core/service"
	"github.com/dnlopes/overseer/internal/shared/paths"
)

// stack wires the real board adapter, the real SwarmService and a real HTTP
// server over a temp data dir — everything except tmux, which is stubbed because
// this exercises the agent-facing contract rather than pane management.
//
// The unit tests above mock the board; this closes the gap between them by
// checking the pieces actually agree once assembled.
type stack struct {
	baseURL   string
	sessionID uuid.UUID
	dataDir   string
	tmuxStub  *tmux.Stub
	swarm     *service.SwarmService
}

func newStack(t *testing.T, swarmSize int) stack {
	t.Helper()

	dataDir := t.TempDir()
	resolver := paths.NewResolver(dataDir)
	logger := discardLogger()

	store, err := storage.New(resolver.DataFile(), logger)
	if err != nil {
		t.Fatalf("storage.New() error = %v", err)
	}

	project, err := domain.NewProject(t.TempDir(), "demo")
	if err != nil {
		t.Fatalf("NewProject() error = %v", err)
	}
	if err := store.Projects().Save(context.Background(), project); err != nil {
		t.Fatalf("save project: %v", err)
	}

	sess, err := domain.NewSession("hive", project.ID)
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if err := sess.AssignSwarmSize(swarmSize); err != nil {
		t.Fatalf("AssignSwarmSize() error = %v", err)
	}
	if err := store.Sessions().Save(context.Background(), sess); err != nil {
		t.Fatalf("save session: %v", err)
	}

	board := swarmboard.New(resolver, logger)
	tmuxStub := &tmux.Stub{}
	swarmSvc := service.NewSwarmService(
		board, board, store.Sessions(), tmuxStub,
		resolver, testBoardCap, logger,
	)

	srv := boardhttp.New(swarmSvc, logger)
	baseURL, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	return stack{
		baseURL:   baseURL,
		sessionID: sess.ID,
		dataDir:   dataDir,
		tmuxStub:  tmuxStub,
		swarm:     swarmSvc,
	}
}

func (s stack) messagesURL() string {
	return fmt.Sprintf("%s/v1/sessions/%s/messages", s.baseURL, s.sessionID)
}

// postAs performs the exact call the bootstrap descriptor tells agents to make.
func (s stack) postAs(t *testing.T, author, content string) int {
	t.Helper()
	body, err := json.Marshal(map[string]string{"author": author, "content": content})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	resp, err := http.Post(s.messagesURL(), "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST error = %v", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

type boardResponse struct {
	Messages []struct {
		Seq     int    `json:"seq"`
		Author  string `json:"author"`
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	LatestSeq int `json:"latestSeq"`
}

func (s stack) read(t *testing.T, since int) boardResponse {
	t.Helper()
	resp, err := http.Get(fmt.Sprintf("%s?since=%d", s.messagesURL(), since))
	if err != nil {
		t.Fatalf("GET error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", resp.StatusCode)
	}

	var out boardResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode board: %v", err)
	}
	return out
}

func TestEndToEnd_AgentsTalkThroughTheBoardAndItPersists(t *testing.T) {
	s := newStack(t, 3)

	// Three agents post, exactly as the descriptor's curl hint describes.
	for i, post := range []struct{ author, content string }{
		{"agent-1", "storage is a JSON file behind SessionStore"},
		{"agent-2", "sessions are keyed by UUID in memory then persisted atomically"},
		{"agent-3", "I will check the migration path"},
	} {
		if code := s.postAs(t, post.author, post.content); code != http.StatusCreated {
			t.Fatalf("post %d status = %d, want 201", i+1, code)
		}
	}

	board := s.read(t, 0)
	if len(board.Messages) != 3 {
		t.Fatalf("board has %d messages, want 3", len(board.Messages))
	}
	if board.LatestSeq != 3 {
		t.Fatalf("latestSeq = %d, want 3", board.LatestSeq)
	}
	for i, msg := range board.Messages {
		if msg.Seq != i+1 {
			t.Fatalf("message %d seq = %d, want %d", i, msg.Seq, i+1)
		}
		if msg.Role != "agent" {
			t.Fatalf("message %d role = %q, want agent", i, msg.Role)
		}
	}

	// Reading from a cursor returns only the delta — this is what keeps a long
	// swarm cheap to follow.
	delta := s.read(t, 2)
	if len(delta.Messages) != 1 || delta.Messages[0].Author != "agent-3" {
		t.Fatalf("delta from cursor 2 = %+v, want just agent-3's post", delta.Messages)
	}

	// And it is really on disk as JSONL, one record per line.
	boardFile := paths.NewResolver(s.dataDir).SwarmBoardFile(s.sessionID)
	data, err := os.ReadFile(boardFile)
	if err != nil {
		t.Fatalf("read board file %q: %v", boardFile, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("board file has %d lines, want 3:\n%s", len(lines), data)
	}
	for i, line := range lines {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("line %d is not JSON: %v", i+1, err)
		}
		if rec["v"] != float64(1) {
			t.Fatalf("line %d missing schema version, got %v", i+1, rec["v"])
		}
	}
}

func TestEndToEnd_BootstrapThenHumanPostWakesEveryAgent(t *testing.T) {
	s := newStack(t, 3)
	ctx := context.Background()

	if err := s.swarm.Bootstrap(ctx, service.BootstrapSwarmRequest{
		SessionID: s.sessionID,
		BoardURL:  s.baseURL,
		Goal:      "summarise how sessions are persisted",
	}); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	// The descriptor the agents read must exist and point back at this server.
	descriptorPath := paths.NewResolver(s.dataDir).SwarmDescriptorFile(s.sessionID)
	descData, err := os.ReadFile(descriptorPath)
	if err != nil {
		t.Fatalf("read descriptor %q: %v", descriptorPath, err)
	}
	var descriptor map[string]any
	if err := json.Unmarshal(descData, &descriptor); err != nil {
		t.Fatalf("descriptor is not JSON: %v", err)
	}
	if descriptor["boardUrl"] != s.baseURL {
		t.Fatalf("descriptor boardUrl = %v, want %v", descriptor["boardUrl"], s.baseURL)
	}
	if filepath.Base(descriptorPath) != "swarm.json" {
		t.Fatalf("descriptor filename = %q, want swarm.json", filepath.Base(descriptorPath))
	}

	// The goal is the board's first message, so it shows up in history for free.
	board := s.read(t, 0)
	if len(board.Messages) != 1 {
		t.Fatalf("board after bootstrap has %d messages, want 1 (the goal)", len(board.Messages))
	}
	if board.Messages[0].Role != "human" || !strings.Contains(board.Messages[0].Content, "summarise") {
		t.Fatalf("first board message = %+v, want the goal posted as human", board.Messages[0])
	}

	// Bootstrap typed the briefing into all three panes but deliberately did not
	// submit it: the agent CLI is still booting and would swallow the Enter.
	if s.tmuxStub.SendKeysCalls != 0 {
		t.Fatalf("bootstrap sent %d Enter keystrokes, want 0 — submitting is deferred to SubmitBriefings",
			s.tmuxStub.SendKeysCalls)
	}

	// The flush job submits it, retrying across a few rounds.
	submit, err := s.swarm.SubmitBriefings(ctx, service.SubmitSwarmBriefingsRequest{})
	if err != nil {
		t.Fatalf("SubmitBriefings() error = %v", err)
	}
	if submit.Submitted != 3 {
		t.Fatalf("SubmitBriefings() Submitted = %d, want 3", submit.Submitted)
	}

	// Now the operator posts, and the flush wakes every agent — a human message
	// excludes nobody.
	briefingKeys := s.tmuxStub.SendKeysCalls
	if _, err := s.swarm.Post(ctx, service.PostSwarmMessageRequest{
		SessionID: s.sessionID,
		Author:    "human",
		Role:      domain.SwarmRoleHuman,
		Content:   "focus on the migration path",
	}); err != nil {
		t.Fatalf("Post() error = %v", err)
	}

	resp, err := s.swarm.FlushNudges(ctx, service.FlushSwarmNudgesRequest{})
	if err != nil {
		t.Fatalf("FlushNudges() error = %v", err)
	}
	if resp.Nudged != 3 {
		t.Fatalf("FlushNudges() Nudged = %d, want 3 — a human post must reach every agent", resp.Nudged)
	}
	if s.tmuxStub.SendKeysCalls != briefingKeys+3 {
		t.Fatalf("nudge sent Enter %d times, want 3", s.tmuxStub.SendKeysCalls-briefingKeys)
	}
}

func TestEndToEnd_AgentPostExcludesItselfFromTheNudge(t *testing.T) {
	s := newStack(t, 4)
	ctx := context.Background()

	if code := s.postAs(t, "agent-2", "taking the storage layer"); code != http.StatusCreated {
		t.Fatalf("post status = %d, want 201", code)
	}

	resp, err := s.swarm.FlushNudges(ctx, service.FlushSwarmNudgesRequest{})
	if err != nil {
		t.Fatalf("FlushNudges() error = %v", err)
	}
	if resp.Nudged != 3 {
		t.Fatalf("FlushNudges() Nudged = %d, want 3 (everyone but agent-2)", resp.Nudged)
	}
}

func TestEndToEnd_BurstOfPostsCostsOneNudgeRound(t *testing.T) {
	s := newStack(t, 4)
	ctx := context.Background()

	// Five posts land between flushes. Without coalescing this would be five
	// rounds of pokes — the runaway-chatter failure mode.
	for i := range 5 {
		if code := s.postAs(t, fmt.Sprintf("agent-%d", (i%4)+1), fmt.Sprintf("post %d", i)); code != http.StatusCreated {
			t.Fatalf("post %d status = %d, want 201", i, code)
		}
	}

	resp, err := s.swarm.FlushNudges(ctx, service.FlushSwarmNudgesRequest{})
	if err != nil {
		t.Fatalf("FlushNudges() error = %v", err)
	}
	if resp.Nudged != 3 {
		t.Fatalf("FlushNudges() Nudged = %d, want 3 (one coalesced round, minus the last author)", resp.Nudged)
	}

	// A second flush with nothing new must be silent, or the swarm would nudge
	// itself forever.
	resp, err = s.swarm.FlushNudges(ctx, service.FlushSwarmNudgesRequest{})
	if err != nil {
		t.Fatalf("second FlushNudges() error = %v", err)
	}
	if resp.Nudged != 0 {
		t.Fatalf("second FlushNudges() Nudged = %d, want 0", resp.Nudged)
	}
}

func TestEndToEnd_BoardSurvivesAProcessRestart(t *testing.T) {
	s := newStack(t, 2)

	if code := s.postAs(t, "agent-1", "before restart"); code != http.StatusCreated {
		t.Fatalf("post status = %d, want 201", code)
	}

	// A fresh board over the same data dir stands in for a restarted Overseer.
	resolver := paths.NewResolver(s.dataDir)
	revived := swarmboard.New(resolver, discardLogger())

	messages, err := revived.ListSince(context.Background(), s.sessionID, 0)
	if err != nil {
		t.Fatalf("ListSince() error = %v", err)
	}
	if len(messages) != 1 || messages[0].Content != "before restart" {
		t.Fatalf("revived board = %+v, want the earlier post", messages)
	}

	stored, err := revived.Append(context.Background(), domain.SwarmMessage{
		SessionID: s.sessionID,
		Author:    "agent-2",
		Role:      domain.SwarmRoleAgent,
		Content:   "after restart",
	})
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if stored.Seq != 2 {
		t.Fatalf("Seq after restart = %d, want 2 — the sequence must continue", stored.Seq)
	}
}
