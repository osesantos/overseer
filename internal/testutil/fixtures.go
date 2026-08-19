package testutil

import (
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/dnlopes/overseer/internal/core/domain"
)

// MakeSession builds a project-less Session (no worktree). Tests that need a
// project-backed Session with worktree fields populated should use
// MakeSessionWithWorktree.
func MakeSession(name string, projectID uuid.UUID) domain.Session {
	s, err := domain.NewSession(name, projectID)
	if err != nil {
		panic(err)
	}
	return s
}

// MakeSessionWithWorktree builds a project-backed Mode 1 Session populated
// with the supplied worktree path and branch.
func MakeSessionWithWorktree(name string, projectID uuid.UUID, worktreePath, branch string) domain.Session {
	s, err := domain.NewSession(name, projectID)
	if err != nil {
		panic(err)
	}
	if err := s.AssignWorktree(worktreePath, branch); err != nil {
		panic(err)
	}
	return s
}

// MakeSwarmSession builds a project-less Session configured as a swarm of the
// given size. Panics if size is outside the domain's allowed range, so a typo in
// a test fixture fails loudly rather than silently producing a single-agent
// session.
func MakeSwarmSession(name string, projectID uuid.UUID, size int) domain.Session {
	s, err := domain.NewSession(name, projectID)
	if err != nil {
		panic(err)
	}
	if err := s.AssignSwarmSize(size); err != nil {
		panic(err)
	}
	return s
}

func MakeProject(path, name string) domain.Project {
	p, err := domain.NewProject(path, name)
	if err != nil {
		panic(err)
	}
	return p
}

// UUIDString matches any string that parses as a UUID — used to assert the service
// passes a Session.ID (rather than a user-typed name) as the tmux session name.
func UUIDString() interface{} {
	return mock.MatchedBy(func(s string) bool {
		_, err := uuid.Parse(s)
		return err == nil
	})
}

// AgentTmuxIDString matches strings of the form "<uuid>-agent", used by the
// service to name the tmux session that hosts the agent process.
func AgentTmuxIDString() interface{} {
	return mock.MatchedBy(func(s string) bool {
		if !strings.HasSuffix(s, "-agent") {
			return false
		}
		_, err := uuid.Parse(strings.TrimSuffix(s, "-agent"))
		return err == nil
	})
}

// SwarmAgentTmuxIDString matches "<uuid>-agent-<index>" for the given 1-based
// index — the tmux session name of one pane of a swarm. Used when the session ID
// is generated inside the call under test and so cannot be spelled out.
func SwarmAgentTmuxIDString(index int) interface{} {
	suffix := "-agent-" + strconv.Itoa(index)
	return mock.MatchedBy(func(s string) bool {
		if !strings.HasSuffix(s, suffix) {
			return false
		}
		_, err := uuid.Parse(strings.TrimSuffix(s, suffix))
		return err == nil
	})
}
