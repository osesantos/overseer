package domain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewSession_CreatesSession(t *testing.T) {
	before := time.Now()
	projectID := uuid.New()

	s, err := NewSession("alpha", projectID)

	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if s.ID == uuid.Nil {
		t.Fatal("NewSession() ID is nil")
	}
	if s.Name != "alpha" {
		t.Fatalf("NewSession() Name = %q, want %q", s.Name, "alpha")
	}
	if s.ProjectID != projectID {
		t.Fatalf("NewSession() ProjectID = %v, want %v", s.ProjectID, projectID)
	}
	if s.Order != 0 {
		t.Fatalf("NewSession() Order = %d, want 0", s.Order)
	}
	if s.HasWorktree() {
		t.Fatalf("NewSession() HasWorktree() = true, want false (no worktree assigned)")
	}
	if s.CreatedAt.Before(before) {
		t.Fatalf("NewSession() CreatedAt = %v, before creation start %v", s.CreatedAt, before)
	}
	if s.UpdatedAt.Before(before) {
		t.Fatalf("NewSession() UpdatedAt = %v, before creation start %v", s.UpdatedAt, before)
	}
	if !s.CreatedAt.Equal(s.UpdatedAt) {
		t.Fatalf("NewSession() CreatedAt = %v, UpdatedAt = %v, want equal", s.CreatedAt, s.UpdatedAt)
	}
}

func TestNewSession_TrimsName(t *testing.T) {
	s, err := NewSession("  alpha  ", uuid.New())

	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if s.Name != "alpha" {
		t.Fatalf("NewSession() Name = %q, want %q", s.Name, "alpha")
	}
}

func TestNewSession_RejectsZeroProjectID(t *testing.T) {
	_, err := NewSession("orphan", uuid.Nil)
	if !errors.Is(err, ErrSessionEmptyProjectID) {
		t.Fatalf("NewSession() error = %v, want %v", err, ErrSessionEmptyProjectID)
	}
}

func TestNewSession_Validation(t *testing.T) {
	long := strings.Repeat("a", 101)
	tests := []struct {
		name    string
		session string
		wantErr error
	}{
		{name: "empty name", session: "", wantErr: ErrSessionEmptyName},
		{name: "blank name", session: "   ", wantErr: ErrSessionEmptyName},
		{name: "name too long", session: long, wantErr: ErrSessionNameTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSession(tt.session, uuid.New())
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewSession() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewSession_AcceptsExactlyOneHundredCharacterName(t *testing.T) {
	exactly100 := strings.Repeat("a", 100)
	s, err := NewSession(exactly100, uuid.New())
	if err != nil {
		t.Fatalf("NewSession() error = %v, want nil for 100-char name", err)
	}
	if s.Name != exactly100 {
		t.Fatalf("NewSession() Name length = %d, want 100", len(s.Name))
	}
}

func TestSession_HasWorktree_FalseUntilAssigned(t *testing.T) {
	s, _ := NewSession("alpha", uuid.New())
	if s.HasWorktree() {
		t.Fatalf("HasWorktree() = true on fresh session, want false")
	}
	if err := s.AssignWorktree("/abs/wt", "main"); err != nil {
		t.Fatalf("AssignWorktree() error = %v", err)
	}
	if !s.HasWorktree() {
		t.Fatalf("HasWorktree() = false after AssignWorktree, want true")
	}
}

func TestAssignAgentCommand_StoresAndUpdatesTimestamp(t *testing.T) {
	s, _ := NewSession("alpha", uuid.New())
	originalUpdated := s.UpdatedAt
	time.Sleep(time.Millisecond)

	if err := s.AssignAgentCommand("opencode"); err != nil {
		t.Fatalf("AssignAgentCommand() error = %v", err)
	}
	if s.AgentCommand != "opencode" {
		t.Fatalf("AgentCommand = %q, want %q", s.AgentCommand, "opencode")
	}
	if !s.UpdatedAt.After(originalUpdated) {
		t.Fatalf("UpdatedAt = %v, want after %v", s.UpdatedAt, originalUpdated)
	}
}

func TestAssignAgentCommand_TrimsCommand(t *testing.T) {
	s, _ := NewSession("alpha", uuid.New())
	if err := s.AssignAgentCommand("  opencode --config foo  "); err != nil {
		t.Fatalf("AssignAgentCommand() error = %v", err)
	}
	if s.AgentCommand != "opencode --config foo" {
		t.Fatalf("AgentCommand = %q, want %q", s.AgentCommand, "opencode --config foo")
	}
}

func TestAssignAgentCommand_RejectsEmpty(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
	}{
		{name: "empty", cmd: ""},
		{name: "blank", cmd: "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := NewSession("alpha", uuid.New())
			err := s.AssignAgentCommand(tt.cmd)
			if !errors.Is(err, ErrSessionEmptyAgentCommand) {
				t.Fatalf("AssignAgentCommand(%q) error = %v, want %v", tt.cmd, err, ErrSessionEmptyAgentCommand)
			}
			if s.AgentCommand != "" {
				t.Fatalf("AgentCommand = %q, want empty after rejected assignment", s.AgentCommand)
			}
		})
	}
}

func TestAssignWorktree_PopulatesPathAndBranch(t *testing.T) {
	s, _ := NewSession("alpha", uuid.New())
	originalUpdated := s.UpdatedAt
	time.Sleep(time.Millisecond)

	if err := s.AssignWorktree("/abs/worktree", "overseer/alpha"); err != nil {
		t.Fatalf("AssignWorktree() error = %v", err)
	}
	if s.WorktreePath != "/abs/worktree" {
		t.Fatalf("WorktreePath = %q, want %q", s.WorktreePath, "/abs/worktree")
	}
	if s.Branch != "overseer/alpha" {
		t.Fatalf("Branch = %q, want %q", s.Branch, "overseer/alpha")
	}
	if !s.HasWorktree() {
		t.Fatalf("HasWorktree() = false, want true")
	}
	if !s.UpdatedAt.After(originalUpdated) {
		t.Fatalf("UpdatedAt = %v, want after %v", s.UpdatedAt, originalUpdated)
	}
}

func TestAssignWorktree_TrimsFields(t *testing.T) {
	s, _ := NewSession("alpha", uuid.New())
	if err := s.AssignWorktree("  /abs/worktree  ", "  overseer/alpha  "); err != nil {
		t.Fatalf("AssignWorktree() error = %v", err)
	}
	if s.WorktreePath != "/abs/worktree" || s.Branch != "overseer/alpha" {
		t.Fatalf("AssignWorktree did not trim fields: %+v", s)
	}
}

func TestAssignWorktree_Validation(t *testing.T) {
	tests := []struct {
		name         string
		worktreePath string
		branch       string
		wantErr      error
	}{
		{name: "both empty", worktreePath: "", branch: "", wantErr: ErrSessionWorktreeFieldsMismatch},
		{name: "path only", worktreePath: "/abs/worktree", branch: "", wantErr: ErrSessionWorktreeFieldsMismatch},
		{name: "branch only", worktreePath: "", branch: "overseer/alpha", wantErr: ErrSessionWorktreeFieldsMismatch},
		{name: "relative path", worktreePath: "relative/worktree", branch: "overseer/alpha", wantErr: ErrSessionWorktreePathNotAbsolute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := NewSession("alpha", uuid.New())
			err := s.AssignWorktree(tt.worktreePath, tt.branch)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("AssignWorktree() error = %v, want %v", err, tt.wantErr)
			}
			if s.HasWorktree() {
				t.Fatalf("AssignWorktree failed but session still HasWorktree(): %+v", s)
			}
		})
	}
}

func TestWorktreeIsInsideRoot(t *testing.T) {
	tests := []struct {
		name         string
		worktreePath string
		root         string
		want         bool
	}{
		{name: "no worktree is trivially inside", worktreePath: "", root: "/data/worktrees", want: true},
		{name: "direct child of root", worktreePath: "/data/worktrees/abc-123", root: "/data/worktrees", want: true},
		{name: "deeper descendant of root", worktreePath: "/data/worktrees/abc/nested/file", root: "/data/worktrees", want: true},
		{name: "root with trailing slash still matches child", worktreePath: "/data/worktrees/abc", root: "/data/worktrees/", want: true},
		{name: "exact root path is rejected", worktreePath: "/data/worktrees", root: "/data/worktrees", want: false},
		{name: "sibling sharing textual prefix is rejected", worktreePath: "/data/worktrees-evil/abc", root: "/data/worktrees", want: false},
		{name: "parent of root is rejected", worktreePath: "/data", root: "/data/worktrees", want: false},
		{name: "unrelated absolute path is rejected", worktreePath: "/etc/passwd", root: "/data/worktrees", want: false},
		{name: "home directory is rejected", worktreePath: "/home/user", root: "/data/worktrees", want: false},
		{name: "empty root rejects any non-empty path", worktreePath: "/data/worktrees/abc", root: "", want: false},
		{name: "relative root rejects any non-empty path", worktreePath: "/data/worktrees/abc", root: "data/worktrees", want: false},
		{name: "root with surrounding whitespace still validates", worktreePath: "/data/worktrees/abc", root: "  /data/worktrees  ", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := NewSession("alpha", uuid.New())
			if tt.worktreePath != "" {
				s.WorktreePath = tt.worktreePath
			}
			if got := s.WorktreeIsInsideRoot(tt.root); got != tt.want {
				t.Fatalf("WorktreeIsInsideRoot(%q) with WorktreePath=%q = %v, want %v",
					tt.root, tt.worktreePath, got, tt.want)
			}
		})
	}
}

func TestRename_UpdatesNameAndUpdatedAt(t *testing.T) {
	s, _ := NewSession("alpha", uuid.New())
	originalUpdated := s.UpdatedAt
	time.Sleep(time.Millisecond)

	if err := s.Rename("beta"); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	if s.Name != "beta" {
		t.Fatalf("Rename() Name = %q, want %q", s.Name, "beta")
	}
	if !s.UpdatedAt.After(originalUpdated) {
		t.Fatalf("Rename() UpdatedAt = %v, want after %v", s.UpdatedAt, originalUpdated)
	}
}

func TestRename_TrimsAndValidates(t *testing.T) {
	long := strings.Repeat("a", 101)
	tests := []struct {
		name    string
		newName string
		wantErr error
	}{
		{name: "empty", newName: "", wantErr: ErrSessionEmptyName},
		{name: "blank", newName: "   ", wantErr: ErrSessionEmptyName},
		{name: "too long", newName: long, wantErr: ErrSessionNameTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := NewSession("alpha", uuid.New())
			err := s.Rename(tt.newName)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Rename(%q) error = %v, want %v", tt.newName, err, tt.wantErr)
			}
		})
	}

	t.Run("trims valid name", func(t *testing.T) {
		s, _ := NewSession("alpha", uuid.New())
		if err := s.Rename("  beta  "); err != nil {
			t.Fatalf("Rename() error = %v", err)
		}
		if s.Name != "beta" {
			t.Fatalf("Rename() Name = %q, want trimmed %q", s.Name, "beta")
		}
	})
}

func TestAssignLabel_StoresAndUpdatesTimestamp(t *testing.T) {
	s, _ := NewSession("alpha", uuid.New())
	originalUpdated := s.UpdatedAt
	time.Sleep(time.Millisecond)

	if err := s.AssignLabel("WIP"); err != nil {
		t.Fatalf("AssignLabel() error = %v", err)
	}
	if s.Label != "WIP" {
		t.Fatalf("Label = %q, want %q", s.Label, "WIP")
	}
	if !s.UpdatedAt.After(originalUpdated) {
		t.Fatalf("UpdatedAt = %v, want after %v", s.UpdatedAt, originalUpdated)
	}
}

func TestAssignLabel_TrimsCode(t *testing.T) {
	s, _ := NewSession("alpha", uuid.New())

	if err := s.AssignLabel("  testing  "); err != nil {
		t.Fatalf("AssignLabel() error = %v", err)
	}
	if s.Label != "testing" {
		t.Fatalf("Label = %q, want trimmed %q", s.Label, "testing")
	}
}

func TestAssignLabel_EmptyCodeClears(t *testing.T) {
	s, _ := NewSession("alpha", uuid.New())
	_ = s.AssignLabel("done")
	if s.Label != "done" {
		t.Fatalf("precondition: Label = %q, want %q", s.Label, "done")
	}

	if err := s.AssignLabel(""); err != nil {
		t.Fatalf("AssignLabel(\"\") error = %v, want nil (clear)", err)
	}
	if s.Label != "" {
		t.Fatalf("Label = %q, want empty after clear", s.Label)
	}
}

func TestAssignLabel_BlankCodeClears(t *testing.T) {
	s, _ := NewSession("alpha", uuid.New())
	_ = s.AssignLabel("done")

	if err := s.AssignLabel("   "); err != nil {
		t.Fatalf("AssignLabel(\"   \") error = %v, want nil (clear)", err)
	}
	if s.Label != "" {
		t.Fatalf("Label = %q, want empty (blank trims to empty)", s.Label)
	}
}

func TestAssignLabel_RejectsTooLong(t *testing.T) {
	s, _ := NewSession("alpha", uuid.New())
	tooLong := strings.Repeat("a", 51)

	err := s.AssignLabel(tooLong)

	if !errors.Is(err, ErrSessionLabelTooLong) {
		t.Fatalf("AssignLabel(too-long) error = %v, want %v", err, ErrSessionLabelTooLong)
	}
}

func TestAssignLabel_DoesNotBumpTimestampOnError(t *testing.T) {
	s, _ := NewSession("alpha", uuid.New())
	originalUpdated := s.UpdatedAt
	time.Sleep(time.Millisecond)

	_ = s.AssignLabel(strings.Repeat("a", 51))

	if !s.UpdatedAt.Equal(originalUpdated) {
		t.Fatalf("UpdatedAt = %v, want unchanged %v after rejected assignment", s.UpdatedAt, originalUpdated)
	}
}

func TestSession_AssignAgentType_Validates(t *testing.T) {
	t.Run("rejects empty agent type", func(t *testing.T) {
		s, _ := NewSession("alpha", uuid.New())
		err := s.AssignAgentType("")
		if !errors.Is(err, ErrAgentTypeRequired) {
			t.Fatalf("AssignAgentType(\"\") error = %v, want %v", err, ErrAgentTypeRequired)
		}
		if s.AgentType != "" {
			t.Fatalf("AgentType = %q, want empty after rejected assignment", s.AgentType)
		}
	})

	t.Run("valid type stores and bumps UpdatedAt", func(t *testing.T) {
		s, _ := NewSession("alpha", uuid.New())
		originalUpdated := s.UpdatedAt
		time.Sleep(time.Millisecond)

		if err := s.AssignAgentType(AgentTypeClaudeCode); err != nil {
			t.Fatalf("AssignAgentType() error = %v", err)
		}
		if s.AgentType != AgentTypeClaudeCode {
			t.Fatalf("AgentType = %q, want %q", s.AgentType, AgentTypeClaudeCode)
		}
		if !s.UpdatedAt.After(originalUpdated) {
			t.Fatalf("UpdatedAt = %v, want after %v", s.UpdatedAt, originalUpdated)
		}
	})
}

func TestSession_IsSwarm_TrueOnlyFromTwoAgentsUp(t *testing.T) {
	tests := []struct {
		name      string
		swarmSize int
		want      bool
	}{
		{name: "zero is a legacy single-agent session", swarmSize: 0, want: false},
		{name: "one is a single-agent session", swarmSize: 1, want: false},
		{name: "two is the smallest swarm", swarmSize: 2, want: true},
		{name: "eight is the largest swarm", swarmSize: 8, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := NewSession("alpha", uuid.New())
			s.SwarmSize = tt.swarmSize
			if got := s.IsSwarm(); got != tt.want {
				t.Fatalf("IsSwarm() = %v, want %v for SwarmSize %d", got, tt.want, tt.swarmSize)
			}
		})
	}
}

func TestNewSession_IsNotASwarm(t *testing.T) {
	s, err := NewSession("alpha", uuid.New())
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if s.IsSwarm() {
		t.Fatal("NewSession() IsSwarm() = true, want false (swarm is opt-in)")
	}
	if s.AgentCount() != 1 {
		t.Fatalf("NewSession() AgentCount() = %d, want 1", s.AgentCount())
	}
}

func TestSession_AssignSwarmSize_Validation(t *testing.T) {
	tests := []struct {
		name    string
		size    int
		wantErr error
	}{
		{name: "negative", size: -1, wantErr: ErrSessionSwarmSizeOutOfRange},
		{name: "zero", size: 0, wantErr: ErrSessionSwarmSizeOutOfRange},
		{name: "one is below the minimum", size: 1, wantErr: ErrSessionSwarmSizeOutOfRange},
		{name: "above the maximum", size: SwarmMaxAgents + 1, wantErr: ErrSessionSwarmSizeOutOfRange},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := NewSession("alpha", uuid.New())
			err := s.AssignSwarmSize(tt.size)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("AssignSwarmSize(%d) error = %v, want %v", tt.size, err, tt.wantErr)
			}
			if s.SwarmSize != 0 {
				t.Fatalf("SwarmSize = %d, want 0 after rejected assignment", s.SwarmSize)
			}
		})
	}
}

func TestSession_AssignSwarmSize_StoresAndBumpsUpdatedAt(t *testing.T) {
	s, _ := NewSession("alpha", uuid.New())
	originalUpdated := s.UpdatedAt
	time.Sleep(time.Millisecond)

	if err := s.AssignSwarmSize(4); err != nil {
		t.Fatalf("AssignSwarmSize(4) error = %v", err)
	}
	if s.SwarmSize != 4 {
		t.Fatalf("SwarmSize = %d, want 4", s.SwarmSize)
	}
	if !s.IsSwarm() {
		t.Fatal("IsSwarm() = false, want true after AssignSwarmSize(4)")
	}
	if s.AgentCount() != 4 {
		t.Fatalf("AgentCount() = %d, want 4", s.AgentCount())
	}
	if !s.UpdatedAt.After(originalUpdated) {
		t.Fatalf("UpdatedAt = %v, want after %v", s.UpdatedAt, originalUpdated)
	}
}

func TestSession_AssignSwarmSize_AcceptsBoundaries(t *testing.T) {
	for _, size := range []int{SwarmMinAgents, SwarmMaxAgents} {
		s, _ := NewSession("alpha", uuid.New())
		if err := s.AssignSwarmSize(size); err != nil {
			t.Fatalf("AssignSwarmSize(%d) error = %v, want nil", size, err)
		}
	}
}

func TestSession_AgentTmuxID_SingleAgentIgnoresIndex(t *testing.T) {
	s, _ := NewSession("alpha", uuid.New())
	want := s.ID.String() + "-agent"

	for _, index := range []int{0, 1, 2, 99} {
		if got := s.AgentTmuxID(index); got != want {
			t.Fatalf("AgentTmuxID(%d) = %q, want %q for a single-agent session", index, got, want)
		}
	}
}

func TestSession_AgentTmuxID_SwarmIsOneBasedAndSuffixed(t *testing.T) {
	s, _ := NewSession("alpha", uuid.New())
	if err := s.AssignSwarmSize(3); err != nil {
		t.Fatalf("AssignSwarmSize(3) error = %v", err)
	}

	for index := 1; index <= 3; index++ {
		want := fmt.Sprintf("%s-agent-%d", s.ID, index)
		if got := s.AgentTmuxID(index); got != want {
			t.Fatalf("AgentTmuxID(%d) = %q, want %q", index, got, want)
		}
	}
}

func TestSession_AgentTmuxID_SwarmClampsOutOfRangeIndex(t *testing.T) {
	s, _ := NewSession("alpha", uuid.New())
	if err := s.AssignSwarmSize(3); err != nil {
		t.Fatalf("AssignSwarmSize(3) error = %v", err)
	}

	tests := []struct {
		index int
		want  int
	}{
		{index: -5, want: 1},
		{index: 0, want: 1},
		{index: 4, want: 3},
		{index: 99, want: 3},
	}

	for _, tt := range tests {
		want := fmt.Sprintf("%s-agent-%d", s.ID, tt.want)
		if got := s.AgentTmuxID(tt.index); got != want {
			t.Fatalf("AgentTmuxID(%d) = %q, want clamped %q", tt.index, got, want)
		}
	}
}

func TestSession_AgentTmuxIDs_CoversEveryPane(t *testing.T) {
	t.Run("single agent yields one id", func(t *testing.T) {
		s, _ := NewSession("alpha", uuid.New())
		ids := s.AgentTmuxIDs()
		if len(ids) != 1 {
			t.Fatalf("AgentTmuxIDs() length = %d, want 1", len(ids))
		}
		if ids[0] != s.ID.String()+"-agent" {
			t.Fatalf("AgentTmuxIDs()[0] = %q, want %q", ids[0], s.ID.String()+"-agent")
		}
	})

	t.Run("swarm yields one id per agent", func(t *testing.T) {
		s, _ := NewSession("alpha", uuid.New())
		if err := s.AssignSwarmSize(4); err != nil {
			t.Fatalf("AssignSwarmSize(4) error = %v", err)
		}

		ids := s.AgentTmuxIDs()
		if len(ids) != 4 {
			t.Fatalf("AgentTmuxIDs() length = %d, want 4", len(ids))
		}
		for i, id := range ids {
			want := fmt.Sprintf("%s-agent-%d", s.ID, i+1)
			if id != want {
				t.Fatalf("AgentTmuxIDs()[%d] = %q, want %q", i, id, want)
			}
		}
		if slices.Contains(ids, s.ID.String()+"-agent") {
			t.Fatal("AgentTmuxIDs() contains the bare -agent id, want only indexed swarm panes")
		}
	})
}
