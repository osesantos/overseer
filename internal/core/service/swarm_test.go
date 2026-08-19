package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/shared/paths"
	"github.com/dnlopes/overseer/internal/testutil"
	"github.com/dnlopes/overseer/internal/testutil/mocks"
)

const testMaxBoardMessages = 500

func newSwarmMocks(t *testing.T) (
	*mocks.MockSwarmBoardRepository,
	*mocks.MockSwarmDescriptorWriter,
	*mocks.MockSessionRepository,
	*mocks.MockTmuxAdapter,
) {
	t.Helper()
	return mocks.NewMockSwarmBoardRepository(t),
		mocks.NewMockSwarmDescriptorWriter(t),
		mocks.NewMockSessionRepository(t),
		mocks.NewMockTmuxAdapter(t)
}

func newTestSwarmService(
	board domain.SwarmBoardRepository,
	descriptors domain.SwarmDescriptorWriter,
	sessions domain.SessionRepository,
	tmux domain.TmuxAdapter,
) *SwarmService {
	return NewSwarmService(board, descriptors, sessions, tmux, paths.NewResolver(""), testMaxBoardMessages, testLogger())
}

func TestSwarmService_Post_AppendsToTheBoard(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 3)
	board, descriptors, sessions, tmux := newSwarmMocks(t)

	sessions.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	board.EXPECT().Count(mock.Anything, sess.ID).Return(4, nil).Once()

	var appended domain.SwarmMessage
	board.EXPECT().Append(mock.Anything, mock.Anything).
		Run(func(_ context.Context, msg domain.SwarmMessage) { appended = msg }).
		Return(domain.SwarmMessage{Seq: 5, SessionID: sess.ID, Author: "agent-2", Content: "found it"}, nil).Once()

	svc := newTestSwarmService(board, descriptors, sessions, tmux)
	resp, err := svc.Post(context.Background(), PostSwarmMessageRequest{
		SessionID: sess.ID,
		Author:    "agent-2",
		Role:      domain.SwarmRoleAgent,
		Content:   "found it",
	})

	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	if resp.Message.Seq != 5 {
		t.Fatalf("Post() Message.Seq = %d, want 5", resp.Message.Seq)
	}
	if appended.Author != "agent-2" || appended.Content != "found it" {
		t.Fatalf("Post() appended = %+v, want author/content preserved", appended)
	}
	if appended.Role != domain.SwarmRoleAgent {
		t.Fatalf("Post() appended Role = %q", appended.Role)
	}
}

func TestSwarmService_Post_RejectsUnknownSession(t *testing.T) {
	missingID := uuid.New()
	board, descriptors, sessions, tmux := newSwarmMocks(t)
	sessions.EXPECT().Get(mock.Anything, missingID).
		Return(domain.Session{}, domain.ErrSessionNotFound).Once()

	svc := newTestSwarmService(board, descriptors, sessions, tmux)
	_, err := svc.Post(context.Background(), PostSwarmMessageRequest{
		SessionID: missingID,
		Author:    "agent-1",
		Role:      domain.SwarmRoleAgent,
		Content:   "hi",
	})

	if !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("Post() error = %v, want %v", err, domain.ErrSessionNotFound)
	}
}

func TestSwarmService_Post_RejectsNonSwarmSession(t *testing.T) {
	sess := testutil.MakeSession("solo", uuid.New())
	board, descriptors, sessions, tmux := newSwarmMocks(t)
	sessions.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()

	svc := newTestSwarmService(board, descriptors, sessions, tmux)
	_, err := svc.Post(context.Background(), PostSwarmMessageRequest{
		SessionID: sess.ID,
		Author:    "agent-1",
		Role:      domain.SwarmRoleAgent,
		Content:   "hi",
	})

	if !errors.Is(err, domain.ErrSessionNotASwarm) {
		t.Fatalf("Post() error = %v, want %v", err, domain.ErrSessionNotASwarm)
	}
}

func TestSwarmService_Post_RejectsInvalidContent(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 2)
	board, descriptors, sessions, tmux := newSwarmMocks(t)
	sessions.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()

	svc := newTestSwarmService(board, descriptors, sessions, tmux)
	_, err := svc.Post(context.Background(), PostSwarmMessageRequest{
		SessionID: sess.ID,
		Author:    "agent-1",
		Role:      domain.SwarmRoleAgent,
		Content:   "   ",
	})

	if !errors.Is(err, domain.ErrSwarmMessageEmptyContent) {
		t.Fatalf("Post() error = %v, want %v", err, domain.ErrSwarmMessageEmptyContent)
	}
}

func TestSwarmService_Post_TripsTheCircuitBreakerAtTheCap(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 3)
	board, descriptors, sessions, tmux := newSwarmMocks(t)

	sessions.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	board.EXPECT().Count(mock.Anything, sess.ID).Return(testMaxBoardMessages, nil).Once()

	// No Append expectation: the cap must be checked before writing.
	svc := newTestSwarmService(board, descriptors, sessions, tmux)
	_, err := svc.Post(context.Background(), PostSwarmMessageRequest{
		SessionID: sess.ID,
		Author:    "agent-1",
		Role:      domain.SwarmRoleAgent,
		Content:   "one more",
	})

	if !errors.Is(err, domain.ErrSwarmBoardCapReached) {
		t.Fatalf("Post() error = %v, want %v", err, domain.ErrSwarmBoardCapReached)
	}
}

func TestSwarmService_ListMessages_ReturnsBoardSliceAndCursor(t *testing.T) {
	sessionID := uuid.New()
	board, descriptors, sessions, tmux := newSwarmMocks(t)

	board.EXPECT().ListSince(mock.Anything, sessionID, 2).Return([]domain.SwarmMessage{
		{Seq: 3, Content: "third"},
		{Seq: 4, Content: "fourth"},
	}, nil).Once()

	svc := newTestSwarmService(board, descriptors, sessions, tmux)
	resp, err := svc.ListMessages(context.Background(), ListSwarmMessagesRequest{
		SessionID: sessionID,
		Since:     2,
	})

	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(resp.Messages) != 2 {
		t.Fatalf("ListMessages() length = %d, want 2", len(resp.Messages))
	}
	if resp.LatestSeq != 4 {
		t.Fatalf("ListMessages() LatestSeq = %d, want 4", resp.LatestSeq)
	}
}

func TestSwarmService_ListMessages_EmptyResultKeepsCursorSteady(t *testing.T) {
	sessionID := uuid.New()
	board, descriptors, sessions, tmux := newSwarmMocks(t)

	board.EXPECT().ListSince(mock.Anything, sessionID, 7).Return(nil, nil).Once()

	svc := newTestSwarmService(board, descriptors, sessions, tmux)
	resp, err := svc.ListMessages(context.Background(), ListSwarmMessagesRequest{
		SessionID: sessionID,
		Since:     7,
	})

	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if resp.LatestSeq != 7 {
		t.Fatalf("ListMessages() LatestSeq = %d, want the incoming cursor 7 — it must never regress", resp.LatestSeq)
	}
}

// postOnce drives a Post through the service so a nudge becomes pending.
func postOnce(t *testing.T, svc *SwarmService, sess domain.Session, author string) {
	t.Helper()
	_, err := svc.Post(context.Background(), PostSwarmMessageRequest{
		SessionID: sess.ID,
		Author:    author,
		Role:      domain.SwarmRoleAgent,
		Content:   "post",
	})
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
}

func TestSwarmService_FlushNudges_WakesEveryAgentExceptTheLastAuthor(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 4)
	board, descriptors, sessions, tmux := newSwarmMocks(t)

	sessions.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Twice()
	board.EXPECT().Count(mock.Anything, sess.ID).Return(1, nil).Once()
	board.EXPECT().Append(mock.Anything, mock.Anything).
		Return(domain.SwarmMessage{Seq: 2, SessionID: sess.ID, Author: "agent-2"}, nil).Once()

	// agent-2 just spoke, so only 1, 3 and 4 get woken.
	for _, index := range []int{1, 3, 4} {
		tmuxID := sess.AgentTmuxID(index)
		tmux.EXPECT().SendText(mock.Anything, tmuxID, mock.Anything).Return(nil).Once()
		tmux.EXPECT().SendKeys(mock.Anything, tmuxID, "Enter").Return(nil).Once()
	}

	svc := newTestSwarmService(board, descriptors, sessions, tmux)
	postOnce(t, svc, sess, "agent-2")

	resp, err := svc.FlushNudges(context.Background(), FlushSwarmNudgesRequest{})
	if err != nil {
		t.Fatalf("FlushNudges() error = %v", err)
	}
	if resp.Nudged != 3 {
		t.Fatalf("FlushNudges() Nudged = %d, want 3", resp.Nudged)
	}
}

func TestSwarmService_FlushNudges_HumanPostWakesEveryAgent(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 3)
	board, descriptors, sessions, tmux := newSwarmMocks(t)

	sessions.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Twice()
	board.EXPECT().Count(mock.Anything, sess.ID).Return(0, nil).Once()
	board.EXPECT().Append(mock.Anything, mock.Anything).
		Return(domain.SwarmMessage{Seq: 1, SessionID: sess.ID, Author: "human"}, nil).Once()

	for index := 1; index <= 3; index++ {
		tmuxID := sess.AgentTmuxID(index)
		tmux.EXPECT().SendText(mock.Anything, tmuxID, mock.Anything).Return(nil).Once()
		tmux.EXPECT().SendKeys(mock.Anything, tmuxID, "Enter").Return(nil).Once()
	}

	svc := newTestSwarmService(board, descriptors, sessions, tmux)
	if _, err := svc.Post(context.Background(), PostSwarmMessageRequest{
		SessionID: sess.ID,
		Author:    "human",
		Role:      domain.SwarmRoleHuman,
		Content:   "focus on the storage layer",
	}); err != nil {
		t.Fatalf("Post() error = %v", err)
	}

	resp, err := svc.FlushNudges(context.Background(), FlushSwarmNudgesRequest{})
	if err != nil {
		t.Fatalf("FlushNudges() error = %v", err)
	}
	if resp.Nudged != 3 {
		t.Fatalf("FlushNudges() Nudged = %d, want every agent woken by a human post", resp.Nudged)
	}
}

func TestSwarmService_FlushNudges_CoalescesABurstIntoOneRound(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 3)
	board, descriptors, sessions, tmux := newSwarmMocks(t)

	sessions.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Times(4)
	board.EXPECT().Count(mock.Anything, sess.ID).Return(1, nil).Times(3)
	board.EXPECT().Append(mock.Anything, mock.Anything).
		Return(domain.SwarmMessage{Seq: 2, SessionID: sess.ID}, nil).Times(3)

	// Three posts land before the flush; agent-3 spoke last, so exactly two
	// agents are woken — not six.
	for _, index := range []int{1, 2} {
		tmuxID := sess.AgentTmuxID(index)
		tmux.EXPECT().SendText(mock.Anything, tmuxID, mock.Anything).Return(nil).Once()
		tmux.EXPECT().SendKeys(mock.Anything, tmuxID, "Enter").Return(nil).Once()
	}

	svc := newTestSwarmService(board, descriptors, sessions, tmux)
	postOnce(t, svc, sess, "agent-1")
	postOnce(t, svc, sess, "agent-2")
	postOnce(t, svc, sess, "agent-3")

	resp, err := svc.FlushNudges(context.Background(), FlushSwarmNudgesRequest{})
	if err != nil {
		t.Fatalf("FlushNudges() error = %v", err)
	}
	if resp.Nudged != 2 {
		t.Fatalf("FlushNudges() Nudged = %d, want 2 (one coalesced round)", resp.Nudged)
	}
}

func TestSwarmService_FlushNudges_NothingPendingIsAQuietNoOp(t *testing.T) {
	board, descriptors, sessions, tmux := newSwarmMocks(t)

	// No session lookups, no tmux traffic: an idle board must cost nothing.
	svc := newTestSwarmService(board, descriptors, sessions, tmux)
	resp, err := svc.FlushNudges(context.Background(), FlushSwarmNudgesRequest{})

	if err != nil {
		t.Fatalf("FlushNudges() error = %v", err)
	}
	if resp.Nudged != 0 {
		t.Fatalf("FlushNudges() Nudged = %d, want 0", resp.Nudged)
	}
}

func TestSwarmService_FlushNudges_SecondFlushIsQuietUntilSomethingNewIsPosted(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 2)
	board, descriptors, sessions, tmux := newSwarmMocks(t)

	sessions.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Twice()
	board.EXPECT().Count(mock.Anything, sess.ID).Return(0, nil).Once()
	board.EXPECT().Append(mock.Anything, mock.Anything).
		Return(domain.SwarmMessage{Seq: 1, SessionID: sess.ID}, nil).Once()

	tmuxID := sess.AgentTmuxID(2)
	tmux.EXPECT().SendText(mock.Anything, tmuxID, mock.Anything).Return(nil).Once()
	tmux.EXPECT().SendKeys(mock.Anything, tmuxID, "Enter").Return(nil).Once()

	svc := newTestSwarmService(board, descriptors, sessions, tmux)
	postOnce(t, svc, sess, "agent-1")

	if _, err := svc.FlushNudges(context.Background(), FlushSwarmNudgesRequest{}); err != nil {
		t.Fatalf("first FlushNudges() error = %v", err)
	}

	// The mocks are set Once(), so a second round of nudges would fail the test.
	resp, err := svc.FlushNudges(context.Background(), FlushSwarmNudgesRequest{})
	if err != nil {
		t.Fatalf("second FlushNudges() error = %v", err)
	}
	if resp.Nudged != 0 {
		t.Fatalf("second FlushNudges() Nudged = %d, want 0 — pending state must clear", resp.Nudged)
	}
}

func TestSwarmService_FlushNudges_TmuxFailureDoesNotStopOtherAgents(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 3)
	board, descriptors, sessions, tmux := newSwarmMocks(t)

	sessions.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Twice()
	board.EXPECT().Count(mock.Anything, sess.ID).Return(0, nil).Once()
	board.EXPECT().Append(mock.Anything, mock.Anything).
		Return(domain.SwarmMessage{Seq: 1, SessionID: sess.ID}, nil).Once()

	// agent-1's pane is gone; agent-2 must still be woken.
	tmux.EXPECT().SendText(mock.Anything, sess.AgentTmuxID(1), mock.Anything).
		Return(domain.ErrTmuxSessionNotFound).Once()
	tmux.EXPECT().SendText(mock.Anything, sess.AgentTmuxID(2), mock.Anything).Return(nil).Once()
	tmux.EXPECT().SendKeys(mock.Anything, sess.AgentTmuxID(2), "Enter").Return(nil).Once()

	svc := newTestSwarmService(board, descriptors, sessions, tmux)
	postOnce(t, svc, sess, "agent-3")

	resp, err := svc.FlushNudges(context.Background(), FlushSwarmNudgesRequest{})
	if err != nil {
		t.Fatalf("FlushNudges() error = %v, want a dead pane to be tolerated", err)
	}
	if resp.Nudged != 1 {
		t.Fatalf("FlushNudges() Nudged = %d, want 1 (only the reachable agent)", resp.Nudged)
	}
}

func TestSwarmService_Bootstrap_WritesContractPostsGoalAndPromptsEveryAgent(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 3)
	board, descriptors, sessions, tmux := newSwarmMocks(t)

	sessions.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()

	var written domain.SwarmDescriptor
	descriptors.EXPECT().Write(mock.Anything, mock.Anything).
		Run(func(_ context.Context, desc domain.SwarmDescriptor) { written = desc }).
		Return(nil).Once()

	var goalMsg domain.SwarmMessage
	board.EXPECT().Append(mock.Anything, mock.Anything).
		Run(func(_ context.Context, msg domain.SwarmMessage) { goalMsg = msg }).
		Return(domain.SwarmMessage{Seq: 1}, nil).Once()

	// Only the text is typed here. Submitting is deferred to SubmitBriefings
	// because the agent CLI is still starting up and would swallow an Enter now.
	prompts := make(map[string]string, 3)
	for index := 1; index <= 3; index++ {
		tmuxID := sess.AgentTmuxID(index)
		tmux.EXPECT().SendText(mock.Anything, tmuxID, mock.Anything).
			Run(func(_ context.Context, id, text string) { prompts[id] = text }).
			Return(nil).Once()
	}

	svc := newTestSwarmService(board, descriptors, sessions, tmux)
	err := svc.Bootstrap(context.Background(), BootstrapSwarmRequest{
		SessionID: sess.ID,
		BoardURL:  "http://127.0.0.1:54321",
		Goal:      "summarise the storage layer",
	})

	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	if written.AgentCount != 3 || written.Goal != "summarise the storage layer" {
		t.Fatalf("Bootstrap() descriptor = %+v", written)
	}
	if len(written.Roster) != 3 {
		t.Fatalf("Bootstrap() descriptor roster = %v, want 3 entries", written.Roster)
	}

	if goalMsg.Role != domain.SwarmRoleHuman {
		t.Fatalf("Bootstrap() goal message Role = %q, want %q", goalMsg.Role, domain.SwarmRoleHuman)
	}
	if goalMsg.Content != "summarise the storage layer" {
		t.Fatalf("Bootstrap() goal message Content = %q", goalMsg.Content)
	}

	// Each agent must learn its own identity and where the contract lives.
	for index := 1; index <= 3; index++ {
		prompt := prompts[sess.AgentTmuxID(index)]
		wantIdentity := domain.SwarmAgentAuthor(index)
		if !strings.Contains(prompt, wantIdentity) {
			t.Fatalf("agent %d prompt = %q, want it to contain %q", index, prompt, wantIdentity)
		}
		if !strings.Contains(prompt, "swarm.json") {
			t.Fatalf("agent %d prompt = %q, want it to point at the descriptor", index, prompt)
		}
	}
}

func TestSwarmService_Bootstrap_DefersSubmittingTheBriefing(t *testing.T) {
	// Regression: Bootstrap used to send Enter immediately after the text. The
	// pane exists by then, so tmux accepted it — but the agent CLI was still
	// booting and swallowed it, leaving every agent holding an unsent prompt.
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 3)
	board, descriptors, sessions, tmux := newSwarmMocks(t)

	sessions.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	descriptors.EXPECT().Write(mock.Anything, mock.Anything).Return(nil).Once()
	board.EXPECT().Append(mock.Anything, mock.Anything).Return(domain.SwarmMessage{Seq: 1}, nil).Once()
	for index := 1; index <= 3; index++ {
		tmux.EXPECT().SendText(mock.Anything, sess.AgentTmuxID(index), mock.Anything).Return(nil).Once()
	}

	// No SendKeys expectation: any Enter sent during Bootstrap fails this test.
	svc := newTestSwarmService(board, descriptors, sessions, tmux)
	if err := svc.Bootstrap(context.Background(), BootstrapSwarmRequest{
		SessionID: sess.ID,
		BoardURL:  "http://127.0.0.1:1",
		Goal:      "goal",
	}); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
}

func TestSwarmService_SubmitBriefings_SendsEnterToEveryPane(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 3)
	board, descriptors, sessions, tmux := newSwarmMocks(t)

	sessions.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Twice()
	descriptors.EXPECT().Write(mock.Anything, mock.Anything).Return(nil).Once()
	board.EXPECT().Append(mock.Anything, mock.Anything).Return(domain.SwarmMessage{Seq: 1}, nil).Once()
	for index := 1; index <= 3; index++ {
		tmux.EXPECT().SendText(mock.Anything, sess.AgentTmuxID(index), mock.Anything).Return(nil).Once()
		tmux.EXPECT().SendKeys(mock.Anything, sess.AgentTmuxID(index), "Enter").Return(nil).Once()
	}

	svc := newTestSwarmService(board, descriptors, sessions, tmux)
	if err := svc.Bootstrap(context.Background(), BootstrapSwarmRequest{
		SessionID: sess.ID,
		BoardURL:  "http://127.0.0.1:1",
		Goal:      "goal",
	}); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	resp, err := svc.SubmitBriefings(context.Background(), SubmitSwarmBriefingsRequest{})
	if err != nil {
		t.Fatalf("SubmitBriefings() error = %v", err)
	}
	if resp.Submitted != 3 {
		t.Fatalf("SubmitBriefings() Submitted = %d, want 3", resp.Submitted)
	}
}

func TestSwarmService_SubmitBriefings_RetriesThenStops(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 2)
	board, descriptors, sessions, tmux := newSwarmMocks(t)

	// One lookup for Bootstrap plus one per retry round.
	sessions.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Times(1 + briefingSubmitAttempts)
	descriptors.EXPECT().Write(mock.Anything, mock.Anything).Return(nil).Once()
	board.EXPECT().Append(mock.Anything, mock.Anything).Return(domain.SwarmMessage{Seq: 1}, nil).Once()
	for index := 1; index <= 2; index++ {
		tmux.EXPECT().SendText(mock.Anything, sess.AgentTmuxID(index), mock.Anything).Return(nil).Once()
		tmux.EXPECT().SendKeys(mock.Anything, sess.AgentTmuxID(index), "Enter").
			Return(nil).Times(briefingSubmitAttempts)
	}

	svc := newTestSwarmService(board, descriptors, sessions, tmux)
	if err := svc.Bootstrap(context.Background(), BootstrapSwarmRequest{
		SessionID: sess.ID,
		BoardURL:  "http://127.0.0.1:1",
		Goal:      "goal",
	}); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}

	for round := 1; round <= briefingSubmitAttempts; round++ {
		resp, err := svc.SubmitBriefings(context.Background(), SubmitSwarmBriefingsRequest{})
		if err != nil {
			t.Fatalf("round %d error = %v", round, err)
		}
		if resp.Submitted != 2 {
			t.Fatalf("round %d Submitted = %d, want 2", round, resp.Submitted)
		}
	}

	// Budget exhausted: further rounds must be silent, or every working agent
	// would keep receiving stray keystrokes forever.
	resp, err := svc.SubmitBriefings(context.Background(), SubmitSwarmBriefingsRequest{})
	if err != nil {
		t.Fatalf("round after budget error = %v", err)
	}
	if resp.Submitted != 0 {
		t.Fatalf("Submitted after budget = %d, want 0", resp.Submitted)
	}
}

func TestSwarmService_SubmitBriefings_NothingPendingIsANoOp(t *testing.T) {
	board, descriptors, sessions, tmux := newSwarmMocks(t)

	svc := newTestSwarmService(board, descriptors, sessions, tmux)
	resp, err := svc.SubmitBriefings(context.Background(), SubmitSwarmBriefingsRequest{})

	if err != nil {
		t.Fatalf("SubmitBriefings() error = %v", err)
	}
	if resp.Submitted != 0 {
		t.Fatalf("Submitted = %d, want 0", resp.Submitted)
	}
}

func TestSwarmService_Bootstrap_RejectsNonSwarmSession(t *testing.T) {
	sess := testutil.MakeSession("solo", uuid.New())
	board, descriptors, sessions, tmux := newSwarmMocks(t)
	sessions.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()

	svc := newTestSwarmService(board, descriptors, sessions, tmux)
	err := svc.Bootstrap(context.Background(), BootstrapSwarmRequest{
		SessionID: sess.ID,
		BoardURL:  "http://127.0.0.1:1",
		Goal:      "goal",
	})

	if !errors.Is(err, domain.ErrSessionNotASwarm) {
		t.Fatalf("Bootstrap() error = %v, want %v", err, domain.ErrSessionNotASwarm)
	}
}

func TestSwarmService_Bootstrap_RejectsEmptyGoal(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 2)
	board, descriptors, sessions, tmux := newSwarmMocks(t)
	sessions.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()

	svc := newTestSwarmService(board, descriptors, sessions, tmux)
	err := svc.Bootstrap(context.Background(), BootstrapSwarmRequest{
		SessionID: sess.ID,
		BoardURL:  "http://127.0.0.1:1",
		Goal:      "   ",
	})

	if !errors.Is(err, domain.ErrSwarmDescriptorEmptyGoal) {
		t.Fatalf("Bootstrap() error = %v, want %v", err, domain.ErrSwarmDescriptorEmptyGoal)
	}
}

func TestSwarmService_Bootstrap_DescriptorWriteFailureAbortsBeforeSpamming(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 2)
	board, descriptors, sessions, tmux := newSwarmMocks(t)

	sessions.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	descriptors.EXPECT().Write(mock.Anything, mock.Anything).
		Return(errors.New("disk full")).Once()

	// No board Append and no tmux traffic: agents must not be told to read a
	// contract that was never written.
	svc := newTestSwarmService(board, descriptors, sessions, tmux)
	err := svc.Bootstrap(context.Background(), BootstrapSwarmRequest{
		SessionID: sess.ID,
		BoardURL:  "http://127.0.0.1:1",
		Goal:      "goal",
	})

	if err == nil {
		t.Fatal("Bootstrap() error = nil, want the descriptor write failure to propagate")
	}
}
