package boardhttp_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/dnlopes/overseer/internal/adapters/primary/boardhttp"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/core/service"
	"github.com/dnlopes/overseer/internal/shared/paths"
	"github.com/dnlopes/overseer/internal/testutil"
	"github.com/dnlopes/overseer/internal/testutil/mocks"
)

const testBoardCap = 500

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newHandler(t *testing.T) (http.Handler, *mocks.MockSwarmBoardRepository, *mocks.MockSessionRepository) {
	t.Helper()
	board := mocks.NewMockSwarmBoardRepository(t)
	descriptors := mocks.NewMockSwarmDescriptorWriter(t)
	sessions := mocks.NewMockSessionRepository(t)
	tmux := mocks.NewMockTmuxAdapter(t)

	swarmSvc := service.NewSwarmService(
		board, descriptors, sessions, tmux,
		paths.NewResolver(""), testBoardCap, discardLogger(),
	)
	return boardhttp.New(swarmSvc, discardLogger()).Handler(), board, sessions
}

func postMessages(t *testing.T, h http.Handler, sessionID string, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func getMessages(t *testing.T, h http.Handler, sessionID, query string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID+"/messages"+query, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestServer_PostMessage_Returns201WithTheStoredMessage(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 3)
	h, board, sessions := newHandler(t)

	sessions.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	board.EXPECT().Count(mock.Anything, sess.ID).Return(0, nil).Once()
	board.EXPECT().Append(mock.Anything, mock.Anything).Return(domain.SwarmMessage{
		Seq:       1,
		SessionID: sess.ID,
		Author:    "agent-2",
		Role:      domain.SwarmRoleAgent,
		Content:   "found the leak",
	}, nil).Once()

	rec := postMessages(t, h, sess.ID.String(), `{"author":"agent-2","content":"found the leak"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if got["seq"] != float64(1) {
		t.Fatalf("response seq = %v, want 1", got["seq"])
	}
	if got["author"] != "agent-2" {
		t.Fatalf("response author = %v, want agent-2", got["author"])
	}
	if got["content"] != "found the leak" {
		t.Fatalf("response content = %v", got["content"])
	}
}

func TestServer_PostMessage_DefaultsRoleToAgent(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 2)
	h, board, sessions := newHandler(t)

	sessions.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	board.EXPECT().Count(mock.Anything, sess.ID).Return(0, nil).Once()

	var appended domain.SwarmMessage
	board.EXPECT().Append(mock.Anything, mock.Anything).
		Run(func(_ context.Context, msg domain.SwarmMessage) { appended = msg }).
		Return(domain.SwarmMessage{Seq: 1}, nil).Once()

	// Agents post without a role; the board should treat them as agents.
	rec := postMessages(t, h, sess.ID.String(), `{"author":"agent-1","content":"hello"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	if appended.Role != domain.SwarmRoleAgent {
		t.Fatalf("appended Role = %q, want %q", appended.Role, domain.SwarmRoleAgent)
	}
}

func TestServer_PostMessage_RejectsMalformedJSON(t *testing.T) {
	h, _, _ := newHandler(t)

	rec := postMessages(t, h, uuid.New().String(), `{"author":"agent-1",`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestServer_PostMessage_RejectsNonUUIDSessionID(t *testing.T) {
	h, _, _ := newHandler(t)

	rec := postMessages(t, h, "not-a-uuid", `{"author":"agent-1","content":"hi"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestServer_PostMessage_RejectsEmptyContentAsBadRequest(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 2)
	h, _, sessions := newHandler(t)
	sessions.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()

	rec := postMessages(t, h, sess.ID.String(), `{"author":"agent-1","content":"   "}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST status = %d, want %d (body: %s)", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestServer_PostMessage_UnknownSessionReturns404(t *testing.T) {
	missingID := uuid.New()
	h, _, sessions := newHandler(t)
	sessions.EXPECT().Get(mock.Anything, missingID).
		Return(domain.Session{}, domain.ErrSessionNotFound).Once()

	rec := postMessages(t, h, missingID.String(), `{"author":"agent-1","content":"hi"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("POST status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestServer_PostMessage_NonSwarmSessionReturns404(t *testing.T) {
	sess := testutil.MakeSession("solo", uuid.New())
	h, _, sessions := newHandler(t)
	sessions.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()

	rec := postMessages(t, h, sess.ID.String(), `{"author":"agent-1","content":"hi"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("POST status = %d, want %d — a non-swarm session has no board", rec.Code, http.StatusNotFound)
	}
}

func TestServer_PostMessage_CapReachedReturns429(t *testing.T) {
	sess := testutil.MakeSwarmSession("hive", uuid.New(), 3)
	h, board, sessions := newHandler(t)

	sessions.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	board.EXPECT().Count(mock.Anything, sess.ID).Return(testBoardCap, nil).Once()

	rec := postMessages(t, h, sess.ID.String(), `{"author":"agent-1","content":"one more"}`)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("POST status = %d, want %d when the board cap is reached", rec.Code, http.StatusTooManyRequests)
	}
}

func TestServer_GetMessages_ReturnsMessagesAndCursor(t *testing.T) {
	sessionID := uuid.New()
	h, board, _ := newHandler(t)

	board.EXPECT().ListSince(mock.Anything, sessionID, 0).Return([]domain.SwarmMessage{
		{Seq: 1, SessionID: sessionID, Author: "human", Role: domain.SwarmRoleHuman, Content: "the goal"},
		{Seq: 2, SessionID: sessionID, Author: "agent-1", Role: domain.SwarmRoleAgent, Content: "on it"},
	}, nil).Once()

	rec := getMessages(t, h, sessionID.String(), "")

	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var got struct {
		Messages []struct {
			Seq     int    `json:"seq"`
			Author  string `json:"author"`
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		LatestSeq int `json:"latestSeq"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if len(got.Messages) != 2 {
		t.Fatalf("messages length = %d, want 2", len(got.Messages))
	}
	if got.LatestSeq != 2 {
		t.Fatalf("latestSeq = %d, want 2", got.LatestSeq)
	}
	if got.Messages[0].Role != "human" || got.Messages[1].Author != "agent-1" {
		t.Fatalf("messages = %+v, want role/author preserved", got.Messages)
	}
}

func TestServer_GetMessages_HonoursSinceCursor(t *testing.T) {
	sessionID := uuid.New()
	h, board, _ := newHandler(t)

	board.EXPECT().ListSince(mock.Anything, sessionID, 5).Return(nil, nil).Once()

	rec := getMessages(t, h, sessionID.String(), "?since=5")

	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", rec.Code)
	}

	var got struct {
		Messages  []any `json:"messages"`
		LatestSeq int   `json:"latestSeq"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if got.LatestSeq != 5 {
		t.Fatalf("latestSeq = %d, want the incoming cursor 5", got.LatestSeq)
	}
	if got.Messages == nil {
		t.Fatal("messages = null, want an empty array so agents can iterate without a nil check")
	}
}

func TestServer_GetMessages_RejectsNonNumericSince(t *testing.T) {
	h, _, _ := newHandler(t)

	rec := getMessages(t, h, uuid.New().String(), "?since=banana")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestServer_GetMessages_RejectsNegativeSince(t *testing.T) {
	h, _, _ := newHandler(t)

	rec := getMessages(t, h, uuid.New().String(), "?since=-3")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestServer_Healthz_Returns200(t *testing.T) {
	h, _, _ := newHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want 200", rec.Code)
	}
}

func TestServer_UnknownRouteReturns404(t *testing.T) {
	h, _, _ := newHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/nonsense", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /v1/nonsense status = %d, want 404", rec.Code)
	}
}

func TestServer_WrongMethodOnMessagesIsRejected(t *testing.T) {
	h, _, _ := newHandler(t)

	req := httptest.NewRequest(http.MethodDelete, "/v1/sessions/"+uuid.New().String()+"/messages", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestServer_Start_BindsLoopbackEphemeralPortAndServes(t *testing.T) {
	board := mocks.NewMockSwarmBoardRepository(t)
	descriptors := mocks.NewMockSwarmDescriptorWriter(t)
	sessions := mocks.NewMockSessionRepository(t)
	tmux := mocks.NewMockTmuxAdapter(t)
	swarmSvc := service.NewSwarmService(
		board, descriptors, sessions, tmux,
		paths.NewResolver(""), testBoardCap, discardLogger(),
	)

	srv := boardhttp.New(swarmSvc, discardLogger())
	baseURL, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := srv.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})

	if !strings.HasPrefix(baseURL, "http://127.0.0.1:") {
		t.Fatalf("Start() baseURL = %q, want a loopback http URL", baseURL)
	}
	if strings.HasSuffix(baseURL, ":0") {
		t.Fatalf("Start() baseURL = %q, want the actually-bound port, not :0", baseURL)
	}

	resp, err := http.Get(baseURL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want 200", resp.StatusCode)
	}
}

func TestServer_Start_RejectsAnUnusableAddress(t *testing.T) {
	board := mocks.NewMockSwarmBoardRepository(t)
	descriptors := mocks.NewMockSwarmDescriptorWriter(t)
	sessions := mocks.NewMockSessionRepository(t)
	tmux := mocks.NewMockTmuxAdapter(t)
	swarmSvc := service.NewSwarmService(
		board, descriptors, sessions, tmux,
		paths.NewResolver(""), testBoardCap, discardLogger(),
	)

	srv := boardhttp.New(swarmSvc, discardLogger())
	if _, err := srv.Start("definitely not an address"); err == nil {
		t.Fatal("Start() error = nil, want a bind failure to propagate")
	}
}
