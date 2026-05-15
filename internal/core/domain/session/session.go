package session

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID          uuid.UUID
	Name        string
	ProjectName string
	Order       int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func New(name, project string) (Session, error) {
	name = strings.TrimSpace(name)
	project = strings.TrimSpace(project)

	if name == "" {
		return Session{}, ErrEmptyName
	}
	if len(name) > 100 {
		return Session{}, ErrNameTooLong
	}
	if project == "" {
		return Session{}, ErrEmptyProject
	}
	if len(project) > 100 {
		return Session{}, ErrProjectTooLong
	}

	now := time.Now()
	return Session{
		ID:          uuid.New(),
		Name:        name,
		ProjectName: project,
		Order:       0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (s *Session) Rename(newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return ErrEmptyName
	}
	if len(newName) > 100 {
		return ErrNameTooLong
	}

	s.Name = newName
	s.UpdatedAt = time.Now()
	return nil
}

func (s Session) String() string {
	return "[" + s.ProjectName + "] " + s.Name
}
