package mocks

import (
	"context"

	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/core/domain"
)

type MockProjectRepository struct {
	SaveCalls    int
	SaveErr      error
	SavedProject domain.Project

	ListCalls  int
	ListResult []domain.Project
	ListErr    error

	GetCalls  int
	GetResult domain.Project
	GetErr    error

	GetByPathCalls    int
	GetByPathLastArg  string
	GetByPathResult   domain.Project
	GetByPathErr      error

	DeleteCalls int
	DeleteErr   error
}

func (m *MockProjectRepository) Save(ctx context.Context, p domain.Project) error {
	m.SaveCalls++
	m.SavedProject = p
	return m.SaveErr
}

func (m *MockProjectRepository) Get(ctx context.Context, id uuid.UUID) (domain.Project, error) {
	m.GetCalls++
	return m.GetResult, m.GetErr
}

func (m *MockProjectRepository) GetByPath(ctx context.Context, path string) (domain.Project, error) {
	m.GetByPathCalls++
	m.GetByPathLastArg = path
	return m.GetByPathResult, m.GetByPathErr
}

func (m *MockProjectRepository) List(ctx context.Context) ([]domain.Project, error) {
	m.ListCalls++
	return m.ListResult, m.ListErr
}

func (m *MockProjectRepository) Delete(ctx context.Context, id uuid.UUID) error {
	m.DeleteCalls++
	return m.DeleteErr
}
