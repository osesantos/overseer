package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/osesantos/skipper/internal/core/domain"
)

type MateService struct {
	agent  domain.MateAgentPort
	logger *slog.Logger
}

func NewMateService(agent domain.MateAgentPort, logger *slog.Logger) *MateService {
	return &MateService{agent: agent, logger: logger}
}

// MateChatRequest carries the user's message and a snapshot of all
// current sessions to be injected into the agent's context.
type MateChatRequest struct {
	UserMessage string
	Sessions    []domain.MateSessionContext
}

// MateChatResponse carries the agent's reply text and, when the agent
// requested an action, the parsed MateAction for the TUI to present to
// the user as a confirmation dialog.
type MateChatResponse struct {
	Text   string
	Action *domain.MateAction
}

// Chat forwards the user's message to the underlying agent port together with
// a live snapshot of all sessions. The returned response is ready to be
// appended to the TUI chat history.
func (s *MateService) Chat(ctx context.Context, req MateChatRequest) (MateChatResponse, error) {
	resp, err := s.agent.Chat(ctx, req.UserMessage, req.Sessions)
	if err != nil {
		return MateChatResponse{}, fmt.Errorf("mate chat: %w", err)
	}
	s.logger.InfoContext(ctx, "first mate replied",
		slog.Bool("has_action", resp.Action != nil),
		slog.Int("text_len", len(resp.Text)),
	)
	return MateChatResponse{Text: resp.Text, Action: resp.Action}, nil
}


