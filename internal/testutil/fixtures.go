package testutil

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/dnlopes/overseer/internal/core/domain"
)

func MakeSession(name string, projectID uuid.UUID) domain.Session {
	s, err := domain.NewSession(name, projectID)
	if err != nil {
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
