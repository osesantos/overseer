package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dnlopes/overseer/internal/core/domain"
)

type OverseerService struct {
	agent  domain.OverseerAgentPort
	logger *slog.Logger
}

func NewOverseerService(agent domain.OverseerAgentPort, logger *slog.Logger) *OverseerService {
	return &OverseerService{agent: agent, logger: logger}
}

// OverseerChatRequest carries the user's message and a snapshot of all
// current sessions to be injected into the agent's context.
type OverseerChatRequest struct {
	UserMessage string
	Sessions    []domain.OverseerSessionContext
}

// OverseerChatResponse carries the agent's reply text and, when the agent
// requested an action, the parsed OverseerAction for the TUI to present to
// the user as a confirmation dialog.
type OverseerChatResponse struct {
	Text   string
	Action *domain.OverseerAction
}

// Chat forwards the user's message to the underlying agent port together with
// a live snapshot of all sessions. The returned response is ready to be
// appended to the TUI chat history.
func (s *OverseerService) Chat(ctx context.Context, req OverseerChatRequest) (OverseerChatResponse, error) {
	resp, err := s.agent.Chat(ctx, req.UserMessage, req.Sessions)
	if err != nil {
		return OverseerChatResponse{}, fmt.Errorf("overseer chat: %w", err)
	}
	s.logger.InfoContext(ctx, "overseer agent replied",
		slog.Bool("has_action", resp.Action != nil),
		slog.Int("text_len", len(resp.Text)),
	)
	return OverseerChatResponse{Text: resp.Text, Action: resp.Action}, nil
}

// RunLoopTaskRequest carries the working directory and the task criteria for a
// single loop task execution.
type RunLoopTaskRequest struct {
	WorkDir  string
	Criteria string
}

// RunLoopTaskResponse carries the raw stdout from the `claude -p` invocation.
type RunLoopTaskResponse struct {
	Output string
}

// RunLoopTask runs `claude -p --dangerously-skip-permissions` in WorkDir with
// Criteria as the prompt and returns the raw stdout output. The caller is
// responsible for appending the END-sentinel instruction to Criteria.
func (s *OverseerService) RunLoopTask(ctx context.Context, req RunLoopTaskRequest) (RunLoopTaskResponse, error) {
	output, err := s.agent.RunLoopTask(ctx, req.WorkDir, req.Criteria)
	if err != nil {
		return RunLoopTaskResponse{}, fmt.Errorf("loop task: %w", err)
	}
	s.logger.InfoContext(ctx, "loop task completed", slog.Int("output_len", len(output)))
	return RunLoopTaskResponse{Output: output}, nil
}

