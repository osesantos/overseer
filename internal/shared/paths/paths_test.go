package paths

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestNewResolver_NoOverride_UsesXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-data")
	t.Setenv("HOME", "/tmp/home")

	r := NewResolver("")
	got := r.DataFile()
	want := filepath.Join("/tmp/xdg-data", "skipper", "data.json")
	if got != want {
		t.Fatalf("Resolver.DataFile() = %q, want %q", got, want)
	}
}

func TestNewResolver_NoOverride_FallsBackToHomeWhenXDGUnset(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "/tmp/home")

	r := NewResolver("")
	got := r.DataFile()
	want := filepath.Join("/tmp/home", ".local", "share", "skipper", "data.json")
	if got != want {
		t.Fatalf("Resolver.DataFile() = %q, want %q", got, want)
	}
}

func TestNewResolver_DataDirOverride_UsedVerbatim(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-data")

	r := NewResolver("/custom/data/dir")
	want := filepath.Join("/custom/data/dir", "data.json")
	if got := r.DataFile(); got != want {
		t.Fatalf("Resolver.DataFile() = %q, want override-derived %q", got, want)
	}
}

func TestResolver_DataFile_JoinedFromDataDir(t *testing.T) {
	r := NewResolver("/custom/data")
	got := r.DataFile()
	want := filepath.Join("/custom/data", "data.json")
	if got != want {
		t.Fatalf("Resolver.DataFile() = %q, want %q", got, want)
	}
}

func TestResolver_WorktreeRoot_JoinedFromDataDir(t *testing.T) {
	r := NewResolver("/custom/data")
	got := r.WorktreeRoot()
	want := filepath.Join("/custom/data", "worktrees")
	if got != want {
		t.Fatalf("Resolver.WorktreeRoot() = %q, want %q", got, want)
	}
}

func TestResolver_SessionWorktreePath_UsesUUIDUnderWorktreeRoot(t *testing.T) {
	r := NewResolver("/custom/data")
	id := uuid.New()
	got := r.SessionWorktreePath(id)
	want := filepath.Join("/custom/data", "worktrees", id.String()[:8])
	if got != want {
		t.Fatalf("Resolver.SessionWorktreePath() = %q, want %q", got, want)
	}
}

func TestResolver_LogFile_JoinedFromStateDir(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/xdg-state")

	r := NewResolver("")
	got := r.LogFile()
	want := filepath.Join("/tmp/xdg-state", "skipper", "skipper.log")
	if got != want {
		t.Fatalf("Resolver.LogFile() = %q, want %q", got, want)
	}
}

func TestResolver_LogFile_NotAffectedByDataDirOverride(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/xdg-state")

	r := NewResolver("/custom/data")
	want := filepath.Join("/tmp/xdg-state", "skipper", "skipper.log")
	if got := r.LogFile(); got != want {
		t.Fatalf("Resolver.LogFile() = %q, want %q (DataDir override must not leak into state paths)", got, want)
	}
}

func TestConfigFile_UsesXDGConfigHome(t *testing.T) {
	t.Setenv("SKIPPER_CONFIG_FILE", "")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config")

	got := ConfigFile()
	want := filepath.Join("/tmp/xdg-config", "skipper", "config.yaml")
	if got != want {
		t.Fatalf("ConfigFile() = %q, want %q", got, want)
	}
}

func TestConfigFile_FallsBackToHome(t *testing.T) {
	t.Setenv("SKIPPER_CONFIG_FILE", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/tmp/home")

	got := ConfigFile()
	want := filepath.Join("/tmp/home", ".config", "skipper", "config.yaml")
	if got != want {
		t.Fatalf("ConfigFile() = %q, want %q", got, want)
	}
}

func TestConfigFile_EnvOverride_TakesPrecedenceOverXDG(t *testing.T) {
	t.Setenv("SKIPPER_CONFIG_FILE", "/etc/skipper/custom.yaml")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config")

	got := ConfigFile()
	if got != "/etc/skipper/custom.yaml" {
		t.Fatalf("ConfigFile() = %q, want env override %q", got, "/etc/skipper/custom.yaml")
	}
}

func TestConfigFile_EmptyEnvTreatedAsUnset(t *testing.T) {
	t.Setenv("SKIPPER_CONFIG_FILE", "")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config")

	got := ConfigFile()
	want := filepath.Join("/tmp/xdg-config", "skipper", "config.yaml")
	if got != want {
		t.Fatalf("empty env: ConfigFile() = %q, want XDG path %q", got, want)
	}
}

func TestConfigFile_EnvOverride_PreservedVerbatim_NotTreatedAsDir(t *testing.T) {
	t.Setenv("SKIPPER_CONFIG_FILE", "/some/dir/myname.yml")

	got := ConfigFile()
	if got != "/some/dir/myname.yml" {
		t.Fatalf("ConfigFile() = %q, want verbatim env value (no config.yaml suffix appended)", got)
	}
}

func TestEnsureDirCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	if err := EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir() error = %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("os.Stat() error = %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("EnsureDir() created non-directory %v", info.Mode())
	}
}

func TestSessionFeatureBranchUsesSkipperPrefix(t *testing.T) {
	sessionID := uuid.New()
	got := SessionFeatureBranch(sessionID)
	want := "skipper/" + sessionID.String()[:8]
	if got != want {
		t.Fatalf("SessionFeatureBranch() = %q, want %q", got, want)
	}
}

func TestResolver_SwarmDir_UsesUUIDUnderDataDir(t *testing.T) {
	r := NewResolver("/custom/data")
	id := uuid.New()
	got := r.SwarmDir(id)
	want := filepath.Join("/custom/data", "swarm", id.String()[:8])
	if got != want {
		t.Fatalf("Resolver.SwarmDir() = %q, want %q", got, want)
	}
}

func TestResolver_SwarmDir_IsolatesSessions(t *testing.T) {
	r := NewResolver("/custom/data")
	first, second := r.SwarmDir(uuid.New()), r.SwarmDir(uuid.New())
	if first == second {
		t.Fatalf("Resolver.SwarmDir() returned %q for two different sessions, want distinct dirs", first)
	}
}

func TestResolver_SwarmDir_StaysOutOfTheWorktreeRoot(t *testing.T) {
	r := NewResolver("/custom/data")
	id := uuid.New()
	if got := r.SwarmDir(id); strings.HasPrefix(got, r.WorktreeRoot()) {
		t.Fatalf("Resolver.SwarmDir() = %q, must not live under WorktreeRoot() %q — board data would land in a git tree", got, r.WorktreeRoot())
	}
}

func TestResolver_SwarmBoardFile_IsJSONLUnderSwarmDir(t *testing.T) {
	r := NewResolver("/custom/data")
	id := uuid.New()
	got := r.SwarmBoardFile(id)
	want := filepath.Join(r.SwarmDir(id), "messages.jsonl")
	if got != want {
		t.Fatalf("Resolver.SwarmBoardFile() = %q, want %q", got, want)
	}
}

func TestResolver_SwarmDescriptorFile_IsSwarmJSONUnderSwarmDir(t *testing.T) {
	r := NewResolver("/custom/data")
	id := uuid.New()
	got := r.SwarmDescriptorFile(id)
	want := filepath.Join(r.SwarmDir(id), "swarm.json")
	if got != want {
		t.Fatalf("Resolver.SwarmDescriptorFile() = %q, want %q", got, want)
	}
}

func TestResolver_SwarmFiles_HonourDataDirOverride(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-data")

	r := NewResolver("/custom/data")
	id := uuid.New()
	for name, got := range map[string]string{
		"SwarmDir":            r.SwarmDir(id),
		"SwarmBoardFile":      r.SwarmBoardFile(id),
		"SwarmDescriptorFile": r.SwarmDescriptorFile(id),
	} {
		if !strings.HasPrefix(got, "/custom/data") {
			t.Fatalf("Resolver.%s() = %q, want it under the /custom/data override", name, got)
		}
	}
}

func TestWithLegacyFallback(t *testing.T) {
	const base = "/base"
	current, legacy := filepath.Join(base, "skipper"), filepath.Join(base, "overseer")
	tests := []struct {
		name     string
		existing []string
		want     string
	}{
		{"neither exists", nil, current},
		{"only current exists", []string{current}, current},
		{"only legacy exists", []string{legacy}, legacy},
		{"both exist", []string{current, legacy}, current},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists := func(p string) bool { return slices.Contains(tt.existing, p) }
			if got := withLegacyFallback(base, exists); got != tt.want {
				t.Fatalf("withLegacyFallback() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWithLegacyFallback_EmptyBaseStaysEmpty(t *testing.T) {
	if got := withLegacyFallback("", func(string) bool { return true }); got != "" {
		t.Fatalf("withLegacyFallback(\"\") = %q, want empty", got)
	}
}

func TestNewResolver_UsesLegacyDataDirWhenOnlyItExists(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)
	if err := EnsureDir(filepath.Join(xdg, "overseer")); err != nil {
		t.Fatal(err)
	}

	r := NewResolver("")
	if got, want := r.WorktreeRoot(), filepath.Join(xdg, "overseer", "worktrees"); got != want {
		t.Fatalf("Resolver.WorktreeRoot() = %q, want legacy %q", got, want)
	}
}

func TestConfigFile_UsesLegacyDirWhenOnlyItExists(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("SKIPPER_CONFIG_FILE", "")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if err := EnsureDir(filepath.Join(xdg, "overseer")); err != nil {
		t.Fatal(err)
	}

	if got, want := ConfigFile(), filepath.Join(xdg, "overseer", "config.yaml"); got != want {
		t.Fatalf("ConfigFile() = %q, want legacy %q", got, want)
	}
}

func TestResolver_LogFile_NeverFallsBackToLegacyStateDir(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_STATE_HOME", xdg)
	if err := EnsureDir(filepath.Join(xdg, "overseer")); err != nil {
		t.Fatal(err)
	}

	if got, want := NewResolver("").LogFile(), filepath.Join(xdg, "skipper", "skipper.log"); got != want {
		t.Fatalf("Resolver.LogFile() = %q, want %q", got, want)
	}
}
