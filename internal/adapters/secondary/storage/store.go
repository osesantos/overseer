// Package storage provides the JSON-backed repository for sessions and projects.
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/shared/paths"
)

const currentSchemaVersion = 1

type fileSchema struct {
	SchemaVersion         int                     `json:"schemaVersion"`
	Projects              []domain.Project        `json:"projects"`
	Sessions              []domain.Session        `json:"sessions"`
	DismissedPullRequests []domain.PullRequestRef `json:"dismissedPullRequests,omitempty"`
}

type Store struct {
	path      string
	mu        sync.Mutex
	sessions  map[uuid.UUID]domain.Session
	projects  map[uuid.UUID]domain.Project
	dismissed map[domain.PullRequestRef]struct{}
	logger    *slog.Logger
}

func New(path string, logger *slog.Logger) (*Store, error) {
	if err := paths.EnsureDir(filepath.Dir(path)); err != nil {
		return nil, fmt.Errorf("storage: ensure dir: %w", err)
	}
	s := &Store{
		path:      path,
		sessions:  make(map[uuid.UUID]domain.Session),
		projects:  make(map[uuid.UUID]domain.Project),
		dismissed: make(map[domain.PullRequestRef]struct{}),
		logger:    logger,
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Sessions() *SessionStore     { return &SessionStore{store: s} }
func (s *Store) Projects() *ProjectStore     { return &ProjectStore{store: s} }
func (s *Store) Dismissals() *DismissalStore { return &DismissalStore{store: s} }

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("storage: read file: %w", err)
	}

	var schema fileSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		s.quarantine("invalid JSON")
		return nil
	}

	for _, project := range schema.Projects {
		s.projects[project.ID] = project
	}
	for _, ref := range schema.DismissedPullRequests {
		s.dismissed[ref] = struct{}{}
	}
	for _, sess := range schema.Sessions {
		if sess.AgentType == "" {
			inferred := domain.InferAgentType(sess.AgentCommand)
			s.logger.Info("storage: agent type inferred for legacy session",
				"session_id", sess.ID.String(),
				"session_name", sess.Name,
				"agent_command", sess.AgentCommand,
				"agent_type", string(inferred),
			)
			sess.AgentType = inferred
		}
		s.sessions[sess.ID] = sess
	}
	return nil
}

func (s *Store) quarantine(reason string) {
	corruptedPath := s.path + ".corrupted." + strconv.FormatInt(time.Now().Unix(), 10) + ".json"
	if renameErr := os.Rename(s.path, corruptedPath); renameErr != nil {
		s.logger.Warn("storage: failed to rename corrupted file",
			"path", s.path,
			"reason", reason,
			"error", renameErr,
		)
		return
	}
	s.logger.Warn("storage: data file quarantined, starting fresh",
		"corrupted_path", corruptedPath,
		"reason", reason,
	)
}

func (s *Store) persist() error {
	projects := make([]domain.Project, 0, len(s.projects))
	for _, p := range s.projects {
		projects = append(projects, p)
	}
	sessions := make([]domain.Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}
	dismissed := make([]domain.PullRequestRef, 0, len(s.dismissed))
	for ref := range s.dismissed {
		dismissed = append(dismissed, ref)
	}
	sort.Slice(dismissed, func(i, j int) bool {
		return dismissed[i].Repo < dismissed[j].Repo || (dismissed[i].Repo == dismissed[j].Repo && dismissed[i].Number < dismissed[j].Number)
	})
	schema := fileSchema{SchemaVersion: currentSchemaVersion, Projects: projects, Sessions: sessions, DismissedPullRequests: dismissed}
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("storage: marshal: %w", err)
	}
	return paths.AtomicWrite(s.path, data)
}

var _ domain.SessionRepository = (*SessionStore)(nil)

type SessionStore struct {
	store *Store
}

func (s *SessionStore) Save(_ context.Context, sess domain.Session) error {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	s.store.sessions[sess.ID] = sess
	return s.store.persist()
}

func (s *SessionStore) Get(_ context.Context, id uuid.UUID) (domain.Session, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	sess, ok := s.store.sessions[id]
	if !ok {
		return domain.Session{}, domain.ErrSessionNotFound
	}
	return sess, nil
}

func (s *SessionStore) List(_ context.Context) ([]domain.Session, error) {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	result := make([]domain.Session, 0, len(s.store.sessions))
	for _, sess := range s.store.sessions {
		result = append(result, sess)
	}
	return result, nil
}

func (s *SessionStore) Delete(_ context.Context, id uuid.UUID) error {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	if _, ok := s.store.sessions[id]; !ok {
		return domain.ErrSessionNotFound
	}
	delete(s.store.sessions, id)
	return s.store.persist()
}

var _ domain.PullRequestDismissalRepository = (*DismissalStore)(nil)

type DismissalStore struct {
	store *Store
}

func (d *DismissalStore) Add(_ context.Context, ref domain.PullRequestRef) error {
	d.store.mu.Lock()
	defer d.store.mu.Unlock()
	d.store.dismissed[ref] = struct{}{}
	return d.store.persist()
}

func (d *DismissalStore) List(_ context.Context) ([]domain.PullRequestRef, error) {
	d.store.mu.Lock()
	defer d.store.mu.Unlock()
	result := make([]domain.PullRequestRef, 0, len(d.store.dismissed))
	for ref := range d.store.dismissed {
		result = append(result, ref)
	}
	return result, nil
}

var _ domain.ProjectRepository = (*ProjectStore)(nil)

type ProjectStore struct {
	store *Store
}

func (p *ProjectStore) Save(_ context.Context, project domain.Project) error {
	p.store.mu.Lock()
	defer p.store.mu.Unlock()
	p.store.projects[project.ID] = project
	return p.store.persist()
}

func (p *ProjectStore) Get(_ context.Context, id uuid.UUID) (domain.Project, error) {
	p.store.mu.Lock()
	defer p.store.mu.Unlock()
	project, ok := p.store.projects[id]
	if !ok {
		return domain.Project{}, domain.ErrProjectNotFound
	}
	return project, nil
}

func (p *ProjectStore) GetByPath(_ context.Context, path string) (domain.Project, error) {
	p.store.mu.Lock()
	defer p.store.mu.Unlock()
	for _, project := range p.store.projects {
		if project.Path == path {
			return project, nil
		}
	}
	return domain.Project{}, domain.ErrProjectNotFound
}

func (p *ProjectStore) List(_ context.Context) ([]domain.Project, error) {
	p.store.mu.Lock()
	defer p.store.mu.Unlock()
	result := make([]domain.Project, 0, len(p.store.projects))
	for _, project := range p.store.projects {
		result = append(result, project)
	}
	return result, nil
}

func (p *ProjectStore) Delete(_ context.Context, id uuid.UUID) error {
	p.store.mu.Lock()
	defer p.store.mu.Unlock()
	if _, ok := p.store.projects[id]; !ok {
		return domain.ErrProjectNotFound
	}
	delete(p.store.projects, id)
	return p.store.persist()
}
