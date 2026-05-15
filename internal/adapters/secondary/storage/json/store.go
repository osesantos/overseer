package json

import (
	"context"
	encodingjson "encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/core/domain/session"
	"github.com/dnlopes/overseer/internal/shared/paths"
)

var _ session.Repository = (*Store)(nil)

type fileSchema struct {
	SchemaVersion int               `json:"schemaVersion"`
	Sessions      []session.Session `json:"sessions"`
}

type Store struct {
	path          string
	mu            sync.Mutex
	sessions      map[uuid.UUID]session.Session
	schemaVersion int
	logger        *slog.Logger
}

func New(path string, logger *slog.Logger) (*Store, error) {
	if err := paths.EnsureDir(filepath.Dir(path)); err != nil {
		return nil, fmt.Errorf("storage: ensure dir: %w", err)
	}
	s := &Store{
		path:          path,
		sessions:      make(map[uuid.UUID]session.Session),
		schemaVersion: 1,
		logger:        logger,
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("storage: read file: %w", err)
	}

	var schema fileSchema
	if err := encodingjson.Unmarshal(data, &schema); err != nil {
		corruptedPath := s.path + ".corrupted." + strconv.FormatInt(time.Now().Unix(), 10) + ".json"
		if renameErr := os.Rename(s.path, corruptedPath); renameErr != nil {
			s.logger.Warn("storage: failed to rename corrupted file",
				"path", s.path,
				"error", renameErr,
			)
		} else {
			s.logger.Warn("storage: corrupted data file detected, renamed and starting fresh",
				"corrupted_path", corruptedPath,
			)
		}
		return nil
	}

	for _, sess := range schema.Sessions {
		s.sessions[sess.ID] = sess
	}
	return nil
}

// persist must be called with s.mu held.
func (s *Store) persist() error {
	all := make([]session.Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		all = append(all, sess)
	}
	schema := fileSchema{
		SchemaVersion: s.schemaVersion,
		Sessions:      all,
	}
	data, err := encodingjson.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("storage: marshal: %w", err)
	}
	return paths.AtomicWrite(s.path, data)
}

func (s *Store) Save(_ context.Context, sess session.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.ID] = sess
	return s.persist()
}

func (s *Store) Get(_ context.Context, id uuid.UUID) (session.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return session.Session{}, session.ErrNotFound
	}
	return sess, nil
}

func (s *Store) List(_ context.Context) ([]session.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]session.Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		result = append(result, sess)
	}
	return result, nil
}

func (s *Store) Delete(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[id]; !ok {
		return session.ErrNotFound
	}
	delete(s.sessions, id)
	return s.persist()
}
