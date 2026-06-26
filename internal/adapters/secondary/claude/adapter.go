package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/core/domain"
)

var _ domain.OverseerAgentPort = (*Adapter)(nil)

// actionFence matches a <action>…</action> block in Claude's response. The
// inner content is expected to be a JSON object. We use a non-greedy match so
// a single response cannot embed multiple action blocks accidentally.
var actionFence = regexp.MustCompile(`(?s)<action>\s*(\{.*?\})\s*</action>`)

// rawAction is the JSON shape we expect inside an <action> block.
type rawAction struct {
	Type        string `json:"type"`
	SessionID   string `json:"session_id"`
	SessionName string `json:"session_name"`
	Project     string `json:"project"`
	Prompt      string `json:"prompt"`
}

// Adapter invokes `claude -p` as a non-interactive subprocess and returns a
// structured OverseerResponse. The adapter is stateless — each Chat call is a
// fresh subprocess invocation.
type Adapter struct {
	claudeBin string
	logger    *slog.Logger
}

// New constructs an Adapter. Returns ErrOverseerAgentNotFound when the
// `claude` binary cannot be located on PATH.
func New(logger *slog.Logger) (*Adapter, error) {
	path, err := exec.LookPath("claude")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrOverseerAgentNotFound, err)
	}
	return &Adapter{claudeBin: path, logger: logger}, nil
}

// Chat builds a system-enriched prompt, invokes `claude -p`, parses the
// response text, and returns an OverseerResponse. The <action>…</action>
// fence is stripped from Text before it is returned so the chat history shows
// only the human-readable portion.
func (a *Adapter) Chat(ctx context.Context, userMsg string, sessions []domain.OverseerSessionContext) (domain.OverseerResponse, error) {
	prompt := buildPrompt(userMsg, sessions)

	a.logger.Debug("overseer: invoking claude", slog.Int("prompt_len", len(prompt)))

	cmd := exec.CommandContext(ctx, a.claudeBin, "-p", prompt)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return domain.OverseerResponse{}, fmt.Errorf("%w: exit %d: %s",
				domain.ErrOverseerAgentFailed, exitErr.ExitCode(), strings.TrimSpace(stderr.String()))
		}
		return domain.OverseerResponse{}, fmt.Errorf("%w: %w", domain.ErrOverseerAgentFailed, err)
	}

	raw := stdout.String()
	a.logger.Debug("overseer: claude replied", slog.Int("response_len", len(raw)))

	return parseResponse(raw)
}

// buildPrompt assembles the full text sent to `claude -p`. It includes a role
// description, a snapshot of all session contexts, and the user's message.
func buildPrompt(userMsg string, sessions []domain.OverseerSessionContext) string {
	var b strings.Builder

	b.WriteString("You are the Overseer Agent, a meta-agent managing AI coding sessions.\n")
	b.WriteString("You can read session output and send prompts to sessions.\n\n")

	b.WriteString("=== Current Sessions ===\n")
	if len(sessions) == 0 {
		b.WriteString("(no sessions)\n")
	}
	for _, s := range sessions {
		fmt.Fprintf(&b, "\nSession:  %s\n", s.SessionName)
		fmt.Fprintf(&b, "Project:  %s\n", s.ProjectName)
		fmt.Fprintf(&b, "Branch:   %s\n", s.Branch)
		fmt.Fprintf(&b, "Agent:    %s\n", string(s.AgentType))
		fmt.Fprintf(&b, "Status:   %s\n", string(s.Status))
		if s.PaneOutput != "" {
			b.WriteString("Output:\n")
			b.WriteString(s.PaneOutput)
			b.WriteByte('\n')
		}
		b.WriteString("---\n")
	}

	b.WriteString("\n=== Instructions ===\n")
	b.WriteString("When you want to send a prompt to a session, include exactly one action block:\n\n")
	b.WriteString("<action>\n")
	b.WriteString(`{"type":"send_prompt","session_id":"<uuid>","session_name":"<name>","project":"<project>","prompt":"<text>"}`)
	b.WriteString("\n</action>\n\n")
	b.WriteString("If you are only answering a question, respond normally with no action block.\n")
	b.WriteString("Be concise. The user is a developer reading output in a narrow terminal panel.\n\n")

	b.WriteString("=== User Message ===\n")
	b.WriteString(userMsg)
	b.WriteByte('\n')

	return b.String()
}

// parseResponse extracts the human-readable text and optional action from
// Claude's raw stdout. The <action>…</action> block (if any) is stripped from
// the returned Text.
func parseResponse(raw string) (domain.OverseerResponse, error) {
	match := actionFence.FindStringSubmatchIndex(raw)
	if match == nil {
		// No action block — plain text reply.
		return domain.OverseerResponse{Text: strings.TrimSpace(raw)}, nil
	}

	// Extract JSON between the fences.
	jsonStart, jsonEnd := match[2], match[3]
	jsonBytes := []byte(raw[jsonStart:jsonEnd])

	var ra rawAction
	if err := json.Unmarshal(jsonBytes, &ra); err != nil {
		// Malformed JSON: surface the reply text without an action rather
		// than dropping the entire response.
		textOnly := actionFence.ReplaceAllString(raw, "")
		return domain.OverseerResponse{Text: strings.TrimSpace(textOnly)}, nil
	}

	sessionID, err := uuid.Parse(ra.SessionID)
	if err != nil {
		// Unparseable UUID: same fallback — show text, drop action.
		textOnly := actionFence.ReplaceAllString(raw, "")
		return domain.OverseerResponse{Text: strings.TrimSpace(textOnly)}, nil
	}

	action := &domain.OverseerAction{
		Type:        domain.OverseerActionType(ra.Type),
		SessionID:   sessionID,
		SessionName: ra.SessionName,
		Project:     ra.Project,
		Prompt:      ra.Prompt,
	}

	// Strip the action block from the display text.
	text := strings.TrimSpace(actionFence.ReplaceAllString(raw, ""))

	return domain.OverseerResponse{Text: text, Action: action}, nil
}
