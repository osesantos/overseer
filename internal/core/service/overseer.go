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

// EvaluateLoop asks the underlying agent whether the acceptance criteria have
// been met given the current pane output. It is called on every loop tick.
func (s *OverseerService) EvaluateLoop(ctx context.Context, criteria, paneOutput string) (domain.LoopEvaluation, error) {
	eval, err := s.agent.EvaluateLoop(ctx, criteria, paneOutput)
	if err != nil {
		return domain.LoopEvaluation{}, fmt.Errorf("overseer evaluate loop: %w", err)
	}
	s.logger.InfoContext(ctx, "overseer loop evaluated",
		slog.Bool("done", eval.Done),
		slog.Int("prompt_len", len(eval.PromptToSend)),
	)
	return eval, nil
}
