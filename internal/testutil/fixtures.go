package testutil

import (
	"github.com/google/uuid"

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
