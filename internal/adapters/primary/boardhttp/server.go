// Package boardhttp exposes a swarm session's message board over HTTP.
//
// It is a primary (driving) adapter: the agent processes live outside Overseer
// and call in, so requests flow inward through SwarmService exactly like the
// TUI's do. The TUI itself does not use this server — it shares the process and
// reads the board through the service directly.
//
// The server binds loopback only and has no authentication. That is deliberate
// for a single-developer machine, and the reason it must not be exposed.
package boardhttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/core/service"
)

// readHeaderTimeout bounds how long a client may dawdle over its request
// headers; without it the server is trivially tied up by a half-open connection.
const readHeaderTimeout = 10 * time.Second

// Server serves the board API for the agent processes of every swarm session.
type Server struct {
	swarm  *service.SwarmService
	logger *slog.Logger

	httpServer *http.Server
	listener   net.Listener
}

func New(swarm *service.SwarmService, logger *slog.Logger) *Server {
	return &Server{swarm: swarm, logger: logger}
}

// Handler builds the route table. Exported so tests can exercise the routes
// without binding a port.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("POST /v1/sessions/{sessionID}/messages", s.handlePostMessage)
	mux.HandleFunc("GET /v1/sessions/{sessionID}/messages", s.handleGetMessages)
	// Catch other verbs on the messages route so a typo yields 405 rather than
	// the 404 an unregistered pattern would produce.
	mux.HandleFunc("/v1/sessions/{sessionID}/messages", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	return mux
}

// Start binds addr and serves in the background, returning the base URL the
// agents should call. Binding happens synchronously so a port clash surfaces
// here rather than silently in a goroutine, and the returned URL carries the
// port that was actually assigned — which is the point of binding to :0.
func (s *Server) Start(addr string) (string, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return "", fmt.Errorf("boardhttp: listen on %q: %w", addr, err)
	}

	s.listener = listener
	s.httpServer = &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	go func() {
		if err := s.httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("boardhttp: serve stopped", "error", err)
		}
	}()

	baseURL := "http://" + listener.Addr().String()
	s.logger.Info("boardhttp: swarm board listening", "base_url", baseURL)
	return baseURL, nil
}

// Shutdown stops serving. It is safe to call on a server that never started.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("boardhttp: shutdown: %w", err)
	}
	return nil
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// postMessageBody is what an agent sends. Role is optional: agents omit it and
// are treated as agents, so the documented curl call stays short.
type postMessageBody struct {
	Author  string `json:"author"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

type messageDTO struct {
	Seq       int       `json:"seq"`
	SessionID string    `json:"sessionId"`
	Author    string    `json:"author"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

type listMessagesDTO struct {
	Messages  []messageDTO `json:"messages"`
	LatestSeq int          `json:"latestSeq"`
}

func toDTO(msg domain.SwarmMessage) messageDTO {
	return messageDTO{
		Seq:       msg.Seq,
		SessionID: msg.SessionID.String(),
		Author:    msg.Author,
		Role:      string(msg.Role),
		Content:   msg.Content,
		CreatedAt: msg.CreatedAt,
	}
}

func (s *Server) handlePostMessage(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := parseSessionID(w, r)
	if !ok {
		return
	}

	var body postMessageBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "malformed JSON body")
		return
	}

	role := domain.SwarmRole(body.Role)
	if body.Role == "" {
		role = domain.SwarmRoleAgent
	}

	resp, err := s.swarm.Post(r.Context(), service.PostSwarmMessageRequest{
		SessionID: sessionID,
		Author:    body.Author,
		Role:      role,
		Content:   body.Content,
	})
	if err != nil {
		s.writePostError(w, r, sessionID, err)
		return
	}

	writeJSON(w, http.StatusCreated, toDTO(resp.Message))
}

// writePostError maps a Post failure onto a status code an agent can act on:
// 404 when the board does not exist, 429 when the swarm has talked itself into
// the cap, 400 for anything the caller got wrong, 500 otherwise.
func (s *Server) writePostError(w http.ResponseWriter, r *http.Request, sessionID uuid.UUID, err error) {
	switch {
	case errors.Is(err, domain.ErrSessionNotFound), errors.Is(err, domain.ErrSessionNotASwarm):
		writeError(w, http.StatusNotFound, "no swarm board for this session")
	case errors.Is(err, domain.ErrSwarmBoardCapReached):
		writeError(w, http.StatusTooManyRequests, "swarm board message cap reached")
	case errors.Is(err, domain.ErrSwarmMessageEmptyAuthor),
		errors.Is(err, domain.ErrSwarmMessageAuthorTooLong),
		errors.Is(err, domain.ErrSwarmMessageEmptyContent),
		errors.Is(err, domain.ErrSwarmMessageContentTooLong),
		errors.Is(err, domain.ErrSwarmMessageUnknownRole):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		s.logger.ErrorContext(r.Context(), "boardhttp: post failed",
			"session_id", sessionID.String(), "error", err)
		writeError(w, http.StatusInternalServerError, "could not post message")
	}
}

func (s *Server) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := parseSessionID(w, r)
	if !ok {
		return
	}

	since := 0
	if raw := r.URL.Query().Get("since"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "since must be a non-negative integer")
			return
		}
		since = parsed
	}

	resp, err := s.swarm.ListMessages(r.Context(), service.ListSwarmMessagesRequest{
		SessionID: sessionID,
		Since:     since,
	})
	if err != nil {
		s.logger.ErrorContext(r.Context(), "boardhttp: list failed",
			"session_id", sessionID.String(), "error", err)
		writeError(w, http.StatusInternalServerError, "could not read board")
		return
	}

	// Always emit an array, never null, so an agent can iterate the result
	// without special-casing an empty board.
	out := listMessagesDTO{Messages: make([]messageDTO, 0, len(resp.Messages)), LatestSeq: resp.LatestSeq}
	for _, msg := range resp.Messages {
		out.Messages = append(out.Messages, toDTO(msg))
	}
	writeJSON(w, http.StatusOK, out)
}

func parseSessionID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	sessionID, err := uuid.Parse(r.PathValue("sessionID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "session id must be a UUID")
		return uuid.Nil, false
	}
	return sessionID, true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
