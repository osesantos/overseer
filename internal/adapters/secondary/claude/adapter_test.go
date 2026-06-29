package claude

import (
	"testing"

	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/core/domain"
)

func TestParseResponse_PlainText_NoAction(t *testing.T) {
	resp, err := parseResponse("Hello, how can I help?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Action != nil {
		t.Fatalf("expected no action, got %+v", resp.Action)
	}
	if resp.Text != "Hello, how can I help?" {
		t.Fatalf("unexpected text: %q", resp.Text)
	}
}

func TestParseResponse(t *testing.T) {
	validSessionID := uuid.New()

	tests := []struct {
		name       string
		raw        string
		wantText   string
		wantAction bool
		wantType   domain.OverseerActionType
	}{
		{
			name:       "no action block — plain text reply",
			raw:        "Just answering your question.",
			wantText:   "Just answering your question.",
			wantAction: false,
		},
		{
			name: "valid action fence with valid UUID",
			raw: "I'll send the prompt.\n<action>\n" +
				`{"type":"send_prompt","session_id":"` + validSessionID.String() + `","session_name":"mysess","project":"myproj","prompt":"run tests"}` +
				"\n</action>",
			wantText:   "I'll send the prompt.",
			wantAction: true,
			wantType:   domain.OverseerActionSendPrompt,
		},
		{
			name:       "action fence with malformed JSON — fallback to text-only",
			raw:        "Some text\n<action>\n{not json}\n</action>",
			wantText:   "Some text",
			wantAction: false,
		},
		{
			name:       "action fence with unparseable UUID — fallback to text-only",
			raw:        `Text<action>{"type":"send_prompt","session_id":"not-a-uuid","session_name":"s","project":"p","prompt":"x"}</action>`,
			wantText:   "Text",
			wantAction: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := parseResponse(tt.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Text != tt.wantText {
				t.Errorf("Text = %q, want %q", resp.Text, tt.wantText)
			}
			if tt.wantAction && resp.Action == nil {
				t.Fatalf("expected action, got nil")
			}
			if !tt.wantAction && resp.Action != nil {
				t.Fatalf("expected no action, got %+v", resp.Action)
			}
			if tt.wantAction && resp.Action.Type != tt.wantType {
				t.Errorf("Action.Type = %q, want %q", resp.Action.Type, tt.wantType)
			}
		})
	}
}

