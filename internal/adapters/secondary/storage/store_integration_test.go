//go:build integration

package storage_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/secondary/storage"
	"github.com/dnlopes/overseer/internal/core/domain"
)

func TestIntegration_CorruptionRecovery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")
	ctx := context.Background()

	if err := os.WriteFile(path, []byte(`{not valid json`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store, err := storage.New(path, discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v after corruption", err)
	}

	all, err := store.Sessions().List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) != 0 {
		t.Errorf("List() len = %d after corruption recovery, want 0", len(all))
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	hasCorrupted := false
	for _, e := range entries {
		if strings.Contains(e.Name(), ".corrupted.") {
			hasCorrupted = true
			break
		}
	}
	if !hasCorrupted {
		t.Error("expected corrupted file to be renamed with .corrupted. suffix, none found")
	}

	sess := makeSession(t, "post-recovery", uuid.New())
	if err := store.Sessions().Save(ctx, sess); err != nil {
		t.Fatalf("Save() after corruption recovery error = %v", err)
	}
	got, err := store.Sessions().Get(ctx, sess.ID)
	if err != nil {
		t.Fatalf("Get() after corruption recovery error = %v", err)
	}
	if got.Name != sess.Name {
		t.Errorf("Get() Name = %q, want %q", got.Name, sess.Name)
	}
}

func TestIntegration_ConcurrentSaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	ctx := context.Background()

	store, err := storage.New(path, discardLogger())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	sessionStore := store.Sessions()

	const n = 100
	projectID := uuid.New()
	sessions := make([]domain.Session, n)
	for i := range sessions {
		sess, err := domain.NewSession(fmt.Sprintf("session-%d", i), projectID)
		if err != nil {
			t.Fatalf("domain.NewSession() error = %v", err)
		}
		sessions[i] = sess
	}

	errCh := make(chan error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for _, sess := range sessions {
		go func(s domain.Session) {
			defer wg.Done()
			errCh <- sessionStore.Save(ctx, s)
		}(sess)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Errorf("concurrent Save() error = %v", err)
		}
	}

	all, err := sessionStore.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) != n {
		t.Errorf("List() len = %d after %d concurrent saves, want %d", len(all), n, n)
	}
}
