package domain

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewSwarmMessage_CreatesMessage(t *testing.T) {
	before := time.Now()
	sessionID := uuid.New()

	msg, err := NewSwarmMessage(sessionID, "agent-2", SwarmRoleAgent, "found the leak in store.go")

	if err != nil {
		t.Fatalf("NewSwarmMessage() error = %v", err)
	}
	if msg.SessionID != sessionID {
		t.Fatalf("NewSwarmMessage() SessionID = %v, want %v", msg.SessionID, sessionID)
	}
	if msg.Author != "agent-2" {
		t.Fatalf("NewSwarmMessage() Author = %q, want %q", msg.Author, "agent-2")
	}
	if msg.Role != SwarmRoleAgent {
		t.Fatalf("NewSwarmMessage() Role = %q, want %q", msg.Role, SwarmRoleAgent)
	}
	if msg.Content != "found the leak in store.go" {
		t.Fatalf("NewSwarmMessage() Content = %q, want %q", msg.Content, "found the leak in store.go")
	}
	if msg.Seq != 0 {
		t.Fatalf("NewSwarmMessage() Seq = %d, want 0 (assigned by the repository on append)", msg.Seq)
	}
	if msg.CreatedAt.Before(before) {
		t.Fatalf("NewSwarmMessage() CreatedAt = %v, before creation start %v", msg.CreatedAt, before)
	}
}

func TestNewSwarmMessage_TrimsAuthorAndContent(t *testing.T) {
	msg, err := NewSwarmMessage(uuid.New(), "  agent-1  ", SwarmRoleAgent, "  hello  ")

	if err != nil {
		t.Fatalf("NewSwarmMessage() error = %v", err)
	}
	if msg.Author != "agent-1" {
		t.Fatalf("NewSwarmMessage() Author = %q, want %q", msg.Author, "agent-1")
	}
	if msg.Content != "hello" {
		t.Fatalf("NewSwarmMessage() Content = %q, want %q", msg.Content, "hello")
	}
}

func TestNewSwarmMessage_Validation(t *testing.T) {
	longAuthor := strings.Repeat("a", swarmAuthorMaxLen+1)
	longContent := strings.Repeat("c", swarmContentMaxLen+1)

	tests := []struct {
		name      string
		sessionID uuid.UUID
		author    string
		role      SwarmRole
		content   string
		wantErr   error
	}{
		{
			name:      "nil session id",
			sessionID: uuid.Nil,
			author:    "agent-1",
			role:      SwarmRoleAgent,
			content:   "hi",
			wantErr:   ErrSwarmMessageEmptySessionID,
		},
		{
			name:      "empty author",
			sessionID: uuid.New(),
			author:    "",
			role:      SwarmRoleAgent,
			content:   "hi",
			wantErr:   ErrSwarmMessageEmptyAuthor,
		},
		{
			name:      "blank author",
			sessionID: uuid.New(),
			author:    "   ",
			role:      SwarmRoleAgent,
			content:   "hi",
			wantErr:   ErrSwarmMessageEmptyAuthor,
		},
		{
			name:      "author too long",
			sessionID: uuid.New(),
			author:    longAuthor,
			role:      SwarmRoleAgent,
			content:   "hi",
			wantErr:   ErrSwarmMessageAuthorTooLong,
		},
		{
			name:      "empty content",
			sessionID: uuid.New(),
			author:    "agent-1",
			role:      SwarmRoleAgent,
			content:   "",
			wantErr:   ErrSwarmMessageEmptyContent,
		},
		{
			name:      "blank content",
			sessionID: uuid.New(),
			author:    "agent-1",
			role:      SwarmRoleAgent,
			content:   "  \n  ",
			wantErr:   ErrSwarmMessageEmptyContent,
		},
		{
			name:      "content too long",
			sessionID: uuid.New(),
			author:    "agent-1",
			role:      SwarmRoleAgent,
			content:   longContent,
			wantErr:   ErrSwarmMessageContentTooLong,
		},
		{
			name:      "unknown role",
			sessionID: uuid.New(),
			author:    "agent-1",
			role:      SwarmRole("robot"),
			content:   "hi",
			wantErr:   ErrSwarmMessageUnknownRole,
		},
		{
			name:      "empty role",
			sessionID: uuid.New(),
			author:    "agent-1",
			role:      SwarmRole(""),
			content:   "hi",
			wantErr:   ErrSwarmMessageUnknownRole,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSwarmMessage(tt.sessionID, tt.author, tt.role, tt.content)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewSwarmMessage() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewSwarmMessage_AcceptsEveryKnownRole(t *testing.T) {
	for _, role := range []SwarmRole{SwarmRoleAgent, SwarmRoleHuman, SwarmRoleSystem} {
		t.Run(string(role), func(t *testing.T) {
			msg, err := NewSwarmMessage(uuid.New(), "someone", role, "content")
			if err != nil {
				t.Fatalf("NewSwarmMessage(%q) error = %v", role, err)
			}
			if msg.Role != role {
				t.Fatalf("NewSwarmMessage(%q) Role = %q", role, msg.Role)
			}
		})
	}
}

func TestNewSwarmMessage_AcceptsBoundaryLengths(t *testing.T) {
	msg, err := NewSwarmMessage(
		uuid.New(),
		strings.Repeat("a", swarmAuthorMaxLen),
		SwarmRoleAgent,
		strings.Repeat("c", swarmContentMaxLen),
	)
	if err != nil {
		t.Fatalf("NewSwarmMessage() error = %v, want nil at exact max lengths", err)
	}
	if len(msg.Author) != swarmAuthorMaxLen {
		t.Fatalf("Author length = %d, want %d", len(msg.Author), swarmAuthorMaxLen)
	}
	if len(msg.Content) != swarmContentMaxLen {
		t.Fatalf("Content length = %d, want %d", len(msg.Content), swarmContentMaxLen)
	}
}

func TestNewSwarmDescriptor_CreatesDescriptor(t *testing.T) {
	sessionID := uuid.New()

	desc, err := NewSwarmDescriptor(sessionID, "http://127.0.0.1:54321", 3, "summarise the storage layer")

	if err != nil {
		t.Fatalf("NewSwarmDescriptor() error = %v", err)
	}
	if desc.SessionID != sessionID {
		t.Fatalf("NewSwarmDescriptor() SessionID = %v, want %v", desc.SessionID, sessionID)
	}
	if desc.BoardURL != "http://127.0.0.1:54321" {
		t.Fatalf("NewSwarmDescriptor() BoardURL = %q", desc.BoardURL)
	}
	if desc.AgentCount != 3 {
		t.Fatalf("NewSwarmDescriptor() AgentCount = %d, want 3", desc.AgentCount)
	}
	if desc.Goal != "summarise the storage layer" {
		t.Fatalf("NewSwarmDescriptor() Goal = %q", desc.Goal)
	}
}

func TestNewSwarmDescriptor_DerivesRosterFromAgentCount(t *testing.T) {
	desc, err := NewSwarmDescriptor(uuid.New(), "http://127.0.0.1:1", 4, "goal")
	if err != nil {
		t.Fatalf("NewSwarmDescriptor() error = %v", err)
	}

	want := []string{"agent-1", "agent-2", "agent-3", "agent-4"}
	if !slices.Equal(desc.Roster, want) {
		t.Fatalf("NewSwarmDescriptor() Roster = %v, want %v", desc.Roster, want)
	}
}

func TestNewSwarmDescriptor_TrimsBoardURLAndGoal(t *testing.T) {
	desc, err := NewSwarmDescriptor(uuid.New(), "  http://127.0.0.1:1  ", 2, "  goal  ")
	if err != nil {
		t.Fatalf("NewSwarmDescriptor() error = %v", err)
	}
	if desc.BoardURL != "http://127.0.0.1:1" {
		t.Fatalf("BoardURL = %q, want trimmed", desc.BoardURL)
	}
	if desc.Goal != "goal" {
		t.Fatalf("Goal = %q, want trimmed", desc.Goal)
	}
}

func TestNewSwarmDescriptor_Validation(t *testing.T) {
	tests := []struct {
		name       string
		sessionID  uuid.UUID
		boardURL   string
		agentCount int
		goal       string
		wantErr    error
	}{
		{
			name:       "nil session id",
			sessionID:  uuid.Nil,
			boardURL:   "http://127.0.0.1:1",
			agentCount: 2,
			goal:       "goal",
			wantErr:    ErrSwarmMessageEmptySessionID,
		},
		{
			name:       "empty board url",
			sessionID:  uuid.New(),
			boardURL:   "   ",
			agentCount: 2,
			goal:       "goal",
			wantErr:    ErrSwarmDescriptorEmptyBoardURL,
		},
		{
			name:       "agent count below minimum",
			sessionID:  uuid.New(),
			boardURL:   "http://127.0.0.1:1",
			agentCount: 1,
			goal:       "goal",
			wantErr:    ErrSessionSwarmSizeOutOfRange,
		},
		{
			name:       "agent count above maximum",
			sessionID:  uuid.New(),
			boardURL:   "http://127.0.0.1:1",
			agentCount: SwarmMaxAgents + 1,
			goal:       "goal",
			wantErr:    ErrSessionSwarmSizeOutOfRange,
		},
		{
			name:       "empty goal",
			sessionID:  uuid.New(),
			boardURL:   "http://127.0.0.1:1",
			agentCount: 2,
			goal:       "  ",
			wantErr:    ErrSwarmDescriptorEmptyGoal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSwarmDescriptor(tt.sessionID, tt.boardURL, tt.agentCount, tt.goal)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewSwarmDescriptor() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestSwarmAgentAuthor(t *testing.T) {
	tests := []struct {
		index int
		want  string
	}{
		{index: 1, want: "agent-1"},
		{index: 3, want: "agent-3"},
		{index: 8, want: "agent-8"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := SwarmAgentAuthor(tt.index); got != tt.want {
				t.Fatalf("SwarmAgentAuthor(%d) = %q, want %q", tt.index, got, tt.want)
			}
		})
	}
}
