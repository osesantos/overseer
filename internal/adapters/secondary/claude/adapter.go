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

	raw, err := a.run(ctx, prompt)
	if err != nil {
		return domain.OverseerResponse{}, err
	}

	a.logger.Debug("overseer: claude replied", slog.Int("response_len", len(raw)))
	return parseResponse(raw)
}

// EvaluateLoop asks Claude whether the acceptance criteria have been met
// given the current pane output. The response is expected to contain a
// <loop>{"done":bool,...}</loop> block.
func (a *Adapter) EvaluateLoop(ctx context.Context, criteria, paneOutput string) (domain.LoopEvaluation, error) {
	prompt := buildLoopPrompt(criteria, paneOutput)

	a.logger.Debug("overseer: invoking claude for loop eval", slog.Int("prompt_len", len(prompt)))

	raw, err := a.run(ctx, prompt)
	if err != nil {
		return domain.LoopEvaluation{}, err
	}

	a.logger.Debug("overseer: claude loop eval replied", slog.Int("response_len", len(raw)))
	return parseLoopResponse(raw)
}

// run executes `claude -p --dangerously-skip-permissions <prompt>` and returns
// stdout. The skip-permissions flag is required because the subprocess runs
// detached from the controlling TTY (Setsid: true); without it Claude Code
// shows an allow/refuse permission dialog that defaults to refuse when no
// TTY is present, silently producing empty output.
func (a *Adapter) run(ctx context.Context, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, a.claudeBin, "-p", "--dangerously-skip-permissions", prompt)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("%w: exit %d: %s",
				domain.ErrOverseerAgentFailed, exitErr.ExitCode(), strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("%w: %w", domain.ErrOverseerAgentFailed, err)
	}
	return stdout.String(), nil
}

// buildPrompt assembles the full text sent to `claude -p` for Chat calls.
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
		fmt.Fprintf(&b, "ID:       %s\n", s.SessionID.String())
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

// buildLoopPrompt assembles the prompt used for loop evaluation calls.
// The evaluator watches for END in the pane output and otherwise defaults to
// WAIT — only generating a new directive when the agent is clearly idle.
func buildLoopPrompt(criteria, paneOutput string) string {
	var b strings.Builder

	b.WriteString("You are an evaluation agent monitoring a coding session. The session agent has already been given a task and told to write END when finished.\n\n")

	b.WriteString("=== Task ===\n")
	b.WriteString(criteria)
	b.WriteString("\n\n")

	b.WriteString("=== Session Output ===\n")
	if paneOutput == "" {
		b.WriteString("(no output available)\n")
	} else {
		b.WriteString(paneOutput)
		b.WriteByte('\n')
	}

	b.WriteString("\n=== Instructions ===\n")
	b.WriteString("Reply with exactly one of:\n\n")
	b.WriteString("END   — the session output contains the word END on its own line, OR the task is unambiguously complete.\n\n")
	b.WriteString("WAIT  — the agent is actively working (running commands, compiling, writing files, producing output). This is the DEFAULT when the agent is busy or when you are unsure.\n\n")
	b.WriteString("<instruction> — a single short directive, ONLY if the agent is completely idle and clearly needs a nudge to continue. Never send an instruction while the agent is still producing output.\n\n")
	b.WriteString("Reply only with END, WAIT, or a short directive. No explanation.\n")

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

// parseLoopResponse interprets Claude's raw stdout for a loop evaluation.
// "END" (case-insensitive, trimmed) signals the criteria are met.
// "WAIT" signals the session agent is still actively working.
// Anything else is treated as the next prompt to send to the session agent.
func parseLoopResponse(raw string) (domain.LoopEvaluation, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return domain.LoopEvaluation{}, fmt.Errorf("overseer: empty loop evaluation response")
	}
	upper := strings.ToUpper(text)
	if upper == "END" {
		return domain.LoopEvaluation{Done: true, Summary: "Acceptance criteria met."}, nil
	}
	if upper == "WAIT" {
		return domain.LoopEvaluation{AgentStillWorking: true}, nil
	}
	return domain.LoopEvaluation{Done: false, PromptToSend: text}, nil
}
