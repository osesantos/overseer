package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/shared/config"
	"github.com/dnlopes/overseer/internal/shared/errs"
)

func TestDefault_ReturnsCorrectValues(t *testing.T) {
	cfg := config.Default()

	if cfg.Theme != "dark" {
		t.Errorf("Theme: want dark, got %s", cfg.Theme)
	}
	if cfg.Dashboard.MinWidth != 60 {
		t.Errorf("MinWidth: want 60, got %d", cfg.Dashboard.MinWidth)
	}
	if cfg.Dashboard.MinHeight != 15 {
		t.Errorf("MinHeight: want 15, got %d", cfg.Dashboard.MinHeight)
	}
	if cfg.Dashboard.PreviewRefreshInterval != "500ms" {
		t.Errorf("PreviewRefreshInterval: want 500ms, got %s", cfg.Dashboard.PreviewRefreshInterval)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level: want info, got %s", cfg.Logging.Level)
	}
	if cfg.Storage.DataDir != "" {
		t.Errorf("Storage.DataDir: want empty, got %s", cfg.Storage.DataDir)
	}
}

func TestDefault_ShipsOpencodeAndClaudeLaunchers(t *testing.T) {
	cfg := config.Default()

	if len(cfg.Launchers) != 2 {
		t.Fatalf("Launchers: want 2 entries, got %d", len(cfg.Launchers))
	}
	if cfg.Launchers[0].DisplayName != "Claude Code (default)" || cfg.Launchers[0].Command != "claude" {
		t.Errorf("Launchers[0]: want {Claude Code (default), claude}, got %+v", cfg.Launchers[0])
	}
	if cfg.Launchers[1].DisplayName != "OpenCode (default)" || cfg.Launchers[1].Command != "opencode" {
		t.Errorf("Launchers[1]: want {OpenCode (default), opencode}, got %+v", cfg.Launchers[1])
	}
}

func TestLoad_BuiltinDefaultsHaveAgentType(t *testing.T) {
	cfg := config.Default()

	if len(cfg.Launchers) != 2 {
		t.Fatalf("Launchers: want 2 entries, got %d", len(cfg.Launchers))
	}
	if cfg.Launchers[0].AgentType != "claude-code" {
		t.Errorf("Launchers[0].AgentType = %q, want %q", cfg.Launchers[0].AgentType, "claude-code")
	}
	if cfg.Launchers[1].AgentType != "opencode" {
		t.Errorf("Launchers[1].AgentType = %q, want %q", cfg.Launchers[1].AgentType, "opencode")
	}
}

func TestLoad_CustomLauncherWithoutAgentType_Fails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `launchers:
  - displayName: My Custom Agent
    command: my-agent
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for custom launcher without agentType, got nil")
	}
	if !errs.Is(err, errs.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput in error chain, got: %v", err)
	}
	if !strings.Contains(err.Error(), "My Custom Agent") {
		t.Errorf("expected error to include launcher display name %q, got: %v", "My Custom Agent", err)
	}
	if !strings.Contains(err.Error(), "agentType") {
		t.Errorf("expected error to mention %q, got: %v", "agentType", err)
	}
}

func TestLoad_DefaultsApplyWhenAgentStatusSectionMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := "logging:\n  level: debug\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.AgentStatus.Enabled {
		t.Errorf("AgentStatus.Enabled: want true, got false")
	}
	if cfg.AgentStatus.RefreshInterval != 5*time.Second {
		t.Errorf("AgentStatus.RefreshInterval: want 5s, got %v", cfg.AgentStatus.RefreshInterval)
	}
	if !cfg.AgentStatus.Display.SessionList {
		t.Errorf("AgentStatus.Display.SessionList: want true, got false")
	}
	if !cfg.AgentStatus.Display.StatusBar {
		t.Errorf("AgentStatus.Display.StatusBar: want true, got false")
	}
	if cfg.AgentStatus.Display.RowHighlight != "subtle" {
		t.Errorf("AgentStatus.Display.RowHighlight: want %q, got %q", "subtle", cfg.AgentStatus.Display.RowHighlight)
	}
}

func TestDefault_ShipsNvimEditorCommand(t *testing.T) {
	cfg := config.Default()

	if cfg.EditorCommand != "nvim" {
		t.Errorf("EditorCommand: want nvim, got %q", cfg.EditorCommand)
	}
}

func TestDefault_ShipsSwarmDefaults(t *testing.T) {
	cfg := config.Default()

	if !cfg.Swarm.Enabled {
		t.Error("Swarm.Enabled: want true, got false")
	}
	if cfg.Swarm.MaxAgents != domain.SwarmMaxAgents {
		t.Errorf("Swarm.MaxAgents: want %d, got %d", domain.SwarmMaxAgents, cfg.Swarm.MaxAgents)
	}
	if cfg.Swarm.NudgeDebounce != 2*time.Second {
		t.Errorf("Swarm.NudgeDebounce: want 2s, got %v", cfg.Swarm.NudgeDebounce)
	}
	if cfg.Swarm.MaxMessagesPerSession != 500 {
		t.Errorf("Swarm.MaxMessagesPerSession: want 500, got %d", cfg.Swarm.MaxMessagesPerSession)
	}
	if cfg.Swarm.BoardAddr != "127.0.0.1:0" {
		t.Errorf("Swarm.BoardAddr: want 127.0.0.1:0 (loopback, ephemeral port), got %q", cfg.Swarm.BoardAddr)
	}
}

func TestLoad_DefaultsApplyWhenSwarmSectionMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("logging:\n  level: debug\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.Swarm.Enabled {
		t.Error("Swarm.Enabled: want true, got false")
	}
	if cfg.Swarm.MaxMessagesPerSession != 500 {
		t.Errorf("Swarm.MaxMessagesPerSession: want 500, got %d", cfg.Swarm.MaxMessagesPerSession)
	}
}

func TestLoad_SwarmSectionOverridesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "swarm:\n" +
		"  enabled: false\n" +
		"  maxAgents: 4\n" +
		"  nudgeDebounce: 10s\n" +
		"  maxMessagesPerSession: 50\n" +
		"  boardAddr: 127.0.0.1:9999\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Swarm.Enabled {
		t.Error("Swarm.Enabled: want false, got true")
	}
	if cfg.Swarm.MaxAgents != 4 {
		t.Errorf("Swarm.MaxAgents: want 4, got %d", cfg.Swarm.MaxAgents)
	}
	if cfg.Swarm.NudgeDebounce != 10*time.Second {
		t.Errorf("Swarm.NudgeDebounce: want 10s, got %v", cfg.Swarm.NudgeDebounce)
	}
	if cfg.Swarm.MaxMessagesPerSession != 50 {
		t.Errorf("Swarm.MaxMessagesPerSession: want 50, got %d", cfg.Swarm.MaxMessagesPerSession)
	}
	if cfg.Swarm.BoardAddr != "127.0.0.1:9999" {
		t.Errorf("Swarm.BoardAddr: want 127.0.0.1:9999, got %q", cfg.Swarm.BoardAddr)
	}
}

func TestLoad_SwarmValidation(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "maxAgents below the domain minimum",
			content: "swarm:\n  maxAgents: 1\n",
		},
		{
			name:    "maxAgents above the domain ceiling",
			content: "swarm:\n  maxAgents: 99\n",
		},
		{
			name:    "non-positive nudgeDebounce",
			content: "swarm:\n  nudgeDebounce: 0s\n",
		},
		{
			name:    "negative maxMessagesPerSession",
			content: "swarm:\n  maxMessagesPerSession: -1\n",
		},
		{
			name:    "blank boardAddr",
			content: "swarm:\n  boardAddr: \"   \"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}

			_, err := config.Load(path)
			if !errors.Is(err, errs.ErrInvalidInput) {
				t.Fatalf("Load() error = %v, want %v", err, errs.ErrInvalidInput)
			}
		})
	}
}

func TestLoad_SwarmDisabled_SkipsValidation(t *testing.T) {
	// A disabled swarm should not block startup over settings it will never use.
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "swarm:\n  enabled: false\n  maxAgents: 99\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := config.Load(path); err != nil {
		t.Fatalf("Load() error = %v, want nil when swarm is disabled", err)
	}
}

func TestLoad_MissingFile_ReturnsDefaults(t *testing.T) {
	cfg, err := config.Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}

	def := config.Default()
	if cfg.Theme != def.Theme {
		t.Errorf("Theme: want %s, got %s", def.Theme, cfg.Theme)
	}
	if cfg.Dashboard != def.Dashboard {
		t.Errorf("Dashboard: want %+v, got %+v", def.Dashboard, cfg.Dashboard)
	}
	if cfg.Logging != def.Logging {
		t.Errorf("Logging: want %+v, got %+v", def.Logging, cfg.Logging)
	}
	if cfg.Storage != def.Storage {
		t.Errorf("Storage: want %+v, got %+v", def.Storage, cfg.Storage)
	}
	if len(cfg.Launchers) != len(def.Launchers) {
		t.Errorf("Launchers length: want %d, got %d", len(def.Launchers), len(cfg.Launchers))
	}
	if cfg.EditorCommand != def.EditorCommand {
		t.Errorf("EditorCommand: want %q, got %q", def.EditorCommand, cfg.EditorCommand)
	}
}

func TestLoad_InvalidYAML_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(path, []byte("dashboard: [\ninvalid yaml {"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
	if !errs.Is(err, errs.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput in error chain, got: %v", err)
	}
	if !strings.Contains(err.Error(), "line") {
		t.Errorf("expected error to contain 'line', got: %v", err)
	}
}

func TestLoad_ThemeKey_OverridesDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := "theme: tokyo-night\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Theme != "tokyo-night" {
		t.Errorf("Theme: want tokyo-night, got %s", cfg.Theme)
	}
}

func TestLoad_ThemeOmitted_KeepsDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := "logging:\n  level: debug\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Theme != "dark" {
		t.Errorf("Theme: omitting field should preserve dark default, got %s", cfg.Theme)
	}
}

func TestLoad_DisableEmojiKey_OverridesDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := "disableEmoji: true\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.DisableEmoji {
		t.Errorf("DisableEmoji: want true, got %v", cfg.DisableEmoji)
	}
}

func TestLoad_DisableEmojiOmitted_KeepsDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := "logging:\n  level: debug\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DisableEmoji {
		t.Errorf("DisableEmoji: omitting field should preserve false default, got %v", cfg.DisableEmoji)
	}
}

func TestLoad_PartialYAML_FillsDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := "logging:\n  level: debug\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level: want debug, got %s", cfg.Logging.Level)
	}
	if cfg.Dashboard.MinWidth != 60 {
		t.Errorf("MinWidth: want 60, got %d", cfg.Dashboard.MinWidth)
	}
	if len(cfg.Launchers) != 2 {
		t.Errorf("Launchers: want 2 defaults retained, got %d", len(cfg.Launchers))
	}
}

func TestLoad_ValidFullConfig_AllFieldsSet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `theme: dracula
dashboard:
  minWidth: 100
  minHeight: 30
logging:
  level: warn
storage:
  dataDir: /tmp/overseer
launchers:
  - displayName: Custom Agent
    command: my-agent --foo
    agentType: claude-code
  - displayName: Plain Bash
    command: bash
    agentType: opencode
editorCommand: cursor --wait
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Theme != "dracula" {
		t.Errorf("Theme: want dracula, got %s", cfg.Theme)
	}
	if cfg.Dashboard.MinWidth != 100 {
		t.Errorf("MinWidth: want 100, got %d", cfg.Dashboard.MinWidth)
	}
	if cfg.Dashboard.MinHeight != 30 {
		t.Errorf("MinHeight: want 30, got %d", cfg.Dashboard.MinHeight)
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("Logging.Level: want warn, got %s", cfg.Logging.Level)
	}
	if cfg.Storage.DataDir != "/tmp/overseer" {
		t.Errorf("Storage.DataDir: want /tmp/overseer, got %s", cfg.Storage.DataDir)
	}
	if len(cfg.Launchers) != 4 {
		t.Fatalf("Launchers: want 4 entries, got %d", len(cfg.Launchers))
	}
	if cfg.Launchers[0].DisplayName != "Claude Code (default)" || cfg.Launchers[0].Command != "claude" {
		t.Errorf("Launchers[0]: want {Claude Code (default), claude}, got %+v", cfg.Launchers[0])
	}
	if cfg.Launchers[1].DisplayName != "OpenCode (default)" || cfg.Launchers[1].Command != "opencode" {
		t.Errorf("Launchers[1]: want {OpenCode (default), opencode}, got %+v", cfg.Launchers[1])
	}
	if cfg.Launchers[2].DisplayName != "Custom Agent" || cfg.Launchers[2].Command != "my-agent --foo" {
		t.Errorf("Launchers[2]: want {Custom Agent, my-agent --foo}, got %+v", cfg.Launchers[2])
	}
	if cfg.Launchers[3].DisplayName != "Plain Bash" || cfg.Launchers[3].Command != "bash" {
		t.Errorf("Launchers[3]: want {Plain Bash, bash}, got %+v", cfg.Launchers[3])
	}
	if cfg.EditorCommand != "cursor --wait" {
		t.Errorf("EditorCommand: want %q, got %q", "cursor --wait", cfg.EditorCommand)
	}
}

func TestLoad_RelativeDataDir_RejectedWithInvalidInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := "storage:\n  dataDir: ./relative/path\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for relative DataDir, got nil")
	}
	if !errs.Is(err, errs.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput in error chain, got: %v", err)
	}
}

func TestLoad_ExplicitEmptyLaunchers_KeepsDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := "launchers: []\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("explicit empty launchers should load cleanly, got: %v", err)
	}
	if len(cfg.Launchers) != 2 {
		t.Errorf("Launchers: want 2 defaults, got %d", len(cfg.Launchers))
	}
}

func TestLoad_LauncherFieldOmitted_KeepsDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := "logging:\n  level: debug\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Launchers) != 2 {
		t.Errorf("Launchers: omitting field should preserve 2 defaults, got %d", len(cfg.Launchers))
	}
}

func TestLoad_LauncherMissingCommand_RejectedWithInvalidInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `launchers:
  - displayName: NoCommand
    command: ""
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for empty launcher command, got nil")
	}
	if !errs.Is(err, errs.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput in error chain, got: %v", err)
	}
}

func TestLoad_EditorCommandKey_OverridesDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := "editorCommand: hx\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.EditorCommand != "hx" {
		t.Errorf("EditorCommand: want hx, got %q", cfg.EditorCommand)
	}
}

func TestLoad_EditorCommandOmitted_FallsBackToNvim(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := "logging:\n  level: debug\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.EditorCommand != "nvim" {
		t.Errorf("EditorCommand: omitting field should fall back to nvim, got %q", cfg.EditorCommand)
	}
}

func TestLoad_EditorCommandBlank_FallsBackToNvim(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := "editorCommand: \"   \"\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.EditorCommand != "nvim" {
		t.Errorf("EditorCommand: blank should fall back to nvim, got %q", cfg.EditorCommand)
	}
}

func TestValidate_InvalidMinWidth_ReturnsError(t *testing.T) {
	cfg := config.Default()
	cfg.Dashboard.MinWidth = 0

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for MinWidth=0, got nil")
	}
}

func TestValidate_InvalidMinHeight_ReturnsError(t *testing.T) {
	cfg := config.Default()
	cfg.Dashboard.MinHeight = 0

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for MinHeight=0, got nil")
	}
}

func TestDomainLaunchers_ConvertsValidEntries(t *testing.T) {
	cfg := config.Config{
		Launchers: []config.LauncherConfig{
			{DisplayName: "OpenCode", Command: "opencode", AgentType: "opencode"},
			{DisplayName: "Claude", Command: "claude --debug", AgentType: "claude-code"},
		},
	}

	got, err := cfg.DomainLaunchers()
	if err != nil {
		t.Fatalf("DomainLaunchers() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("DomainLaunchers() length = %d, want 2", len(got))
	}
	if got[0].DisplayName != "OpenCode" || got[0].Command != "opencode" {
		t.Errorf("DomainLaunchers()[0] = %+v, want {OpenCode, opencode}", got[0])
	}
	if got[1].DisplayName != "Claude" || got[1].Command != "claude --debug" {
		t.Errorf("DomainLaunchers()[1] = %+v, want {Claude, claude --debug}", got[1])
	}
}

func TestDomainLaunchers_InvalidEntry_ReturnsInvalidInput(t *testing.T) {
	cfg := config.Config{
		Launchers: []config.LauncherConfig{
			{DisplayName: "OK", Command: "ok"},
			{DisplayName: "", Command: "x"},
		},
	}

	_, err := cfg.DomainLaunchers()
	if err == nil {
		t.Fatal("expected error for empty display name, got nil")
	}
	if !errs.Is(err, errs.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput in error chain, got: %v", err)
	}
}

func TestDefault_ShipsFiveBuiltInLabels(t *testing.T) {
	cfg := config.Default()

	wantCodes := []string{"WIP", "draft", "testing", "ready", "done"}
	if len(cfg.Labels) != len(wantCodes) {
		t.Fatalf("Labels: want %d entries, got %d", len(wantCodes), len(cfg.Labels))
	}
	for i, want := range wantCodes {
		if cfg.Labels[i].Code != want {
			t.Errorf("Labels[%d].Code = %q, want %q", i, cfg.Labels[i].Code, want)
		}
		if cfg.Labels[i].Color == "" {
			t.Errorf("Labels[%d].Color is empty", i)
		}
		if cfg.Labels[i].Glyph != "" {
			t.Errorf("Labels[%d].Glyph = %q, want empty (built-in glyph defaults are owned by styles.Glyphs.LabelGlyph)", i, cfg.Labels[i].Glyph)
		}
	}
}

func TestLoad_LabelsKey_ReplacesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `labels:
  - code: in-progress
    color: "#ff00ff"
    glyph: "🔥"
  - code: blocked
    color: red
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Labels) != 2 {
		t.Fatalf("Labels: want 2 entries (full replacement), got %d", len(cfg.Labels))
	}
	if cfg.Labels[0].Code != "in-progress" || cfg.Labels[0].Color != "#ff00ff" || cfg.Labels[0].Glyph != "🔥" {
		t.Errorf("Labels[0] = %+v, want {in-progress, #ff00ff, 🔥}", cfg.Labels[0])
	}
	if cfg.Labels[1].Code != "blocked" || cfg.Labels[1].Color != "red" || cfg.Labels[1].Glyph != "" {
		t.Errorf("Labels[1] = %+v, want {blocked, red, \"\"} (glyph omitted)", cfg.Labels[1])
	}
}

func TestLoad_LabelsOmitted_KeepsDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := "theme: dark\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Labels) != 5 {
		t.Fatalf("Labels: omitting field should preserve 5 defaults, got %d", len(cfg.Labels))
	}
	if cfg.Labels[0].Code != "WIP" {
		t.Errorf("Labels[0].Code = %q, want %q", cfg.Labels[0].Code, "WIP")
	}
}

func TestLoad_LabelsEmptyList_ClearsAllLabels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := "labels: []\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Labels) != 0 {
		t.Fatalf("Labels: explicit empty list should clear defaults, got %d entries", len(cfg.Labels))
	}
}

func TestValidate_LabelInvalidCode_ReturnsInvalidInput(t *testing.T) {
	cfg := config.Default()
	cfg.Labels = []config.LabelConfig{
		{Code: "", Color: "#fff"},
	}

	err := cfg.Validate()

	if err == nil {
		t.Fatal("Validate() error = nil, want error for empty label code")
	}
	if !errs.Is(err, errs.ErrInvalidInput) {
		t.Errorf("Validate() error = %v, want ErrInvalidInput", err)
	}
}

func TestValidate_LabelInvalidColor_ReturnsInvalidInput(t *testing.T) {
	cfg := config.Default()
	cfg.Labels = []config.LabelConfig{
		{Code: "WIP", Color: ""},
	}

	err := cfg.Validate()

	if err == nil {
		t.Fatal("Validate() error = nil, want error for empty label color")
	}
	if !errs.Is(err, errs.ErrInvalidInput) {
		t.Errorf("Validate() error = %v, want ErrInvalidInput", err)
	}
}

func TestValidate_LabelDuplicateCodes_ReturnsInvalidInput(t *testing.T) {
	cfg := config.Default()
	cfg.Labels = []config.LabelConfig{
		{Code: "WIP", Color: "#f00"},
		{Code: "WIP", Color: "#0f0"},
	}

	err := cfg.Validate()

	if err == nil {
		t.Fatal("Validate() error = nil, want error for duplicate label codes")
	}
	if !errs.Is(err, errs.ErrInvalidInput) {
		t.Errorf("Validate() error = %v, want ErrInvalidInput", err)
	}
}

func TestDomainLabels_BuildsValidatedDomainObjects(t *testing.T) {
	cfg := config.Config{
		Labels: []config.LabelConfig{
			{Code: "WIP", Color: "#F59E0B", Glyph: "🚧"},
			{Code: "ready", Color: "#60A5FA"},
		},
	}

	got, err := cfg.DomainLabels()
	if err != nil {
		t.Fatalf("DomainLabels() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("DomainLabels() len = %d, want 2", len(got))
	}
	if got[0].Code != "WIP" || got[0].Color != "#F59E0B" || got[0].Glyph != "🚧" {
		t.Errorf("DomainLabels()[0] = %+v, want {WIP, #F59E0B, 🚧}", got[0])
	}
	if got[1].Code != "ready" || got[1].Color != "#60A5FA" || got[1].Glyph != "" {
		t.Errorf("DomainLabels()[1] = %+v, want {ready, #60A5FA, \"\"}", got[1])
	}
}

func TestDomainLabels_InvalidEntry_ReturnsInvalidInput(t *testing.T) {
	cfg := config.Config{
		Labels: []config.LabelConfig{
			{Code: "OK", Color: "#000"},
			{Code: "", Color: "#fff"},
		},
	}

	_, err := cfg.DomainLabels()
	if err == nil {
		t.Fatal("DomainLabels() error = nil, want error for empty code")
	}
	if !errs.Is(err, errs.ErrInvalidInput) {
		t.Errorf("DomainLabels() error = %v, want ErrInvalidInput", err)
	}
}

func TestLoad_PreviewRefreshIntervalKey_OverridesDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := "dashboard:\n  previewRefreshInterval: 2s\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Dashboard.PreviewRefreshInterval != "2s" {
		t.Errorf("PreviewRefreshInterval: want 2s, got %s", cfg.Dashboard.PreviewRefreshInterval)
	}
	got, err := cfg.PreviewRefreshDuration()
	if err != nil {
		t.Fatalf("PreviewRefreshDuration() error = %v", err)
	}
	if got != 2*time.Second {
		t.Errorf("PreviewRefreshDuration() = %v, want 2s", got)
	}
}

func TestLoad_PreviewRefreshIntervalOmitted_KeepsDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := "dashboard:\n  minWidth: 100\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Dashboard.PreviewRefreshInterval != "500ms" {
		t.Errorf("PreviewRefreshInterval: omitting field should preserve 500ms default, got %s", cfg.Dashboard.PreviewRefreshInterval)
	}
}

func TestLoad_InvalidPreviewRefreshInterval_RejectedWithInvalidInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := "dashboard:\n  previewRefreshInterval: not-a-duration\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for unparseable duration, got nil")
	}
	if !errs.Is(err, errs.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput in error chain, got: %v", err)
	}
}

func TestLoad_ZeroPreviewRefreshInterval_RejectedWithInvalidInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := "dashboard:\n  previewRefreshInterval: 0s\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for zero duration, got nil")
	}
	if !errs.Is(err, errs.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput in error chain, got: %v", err)
	}
}

func TestLoad_NegativePreviewRefreshInterval_RejectedWithInvalidInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := "dashboard:\n  previewRefreshInterval: -1s\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for negative duration, got nil")
	}
	if !errs.Is(err, errs.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput in error chain, got: %v", err)
	}
}

func TestPreviewRefreshDuration_DefaultParsesTo500ms(t *testing.T) {
	cfg := config.Default()
	got, err := cfg.PreviewRefreshDuration()
	if err != nil {
		t.Fatalf("PreviewRefreshDuration() error = %v", err)
	}
	if got != 500*time.Millisecond {
		t.Errorf("PreviewRefreshDuration() = %v, want 500ms", got)
	}
}
