package github

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/dnlopes/overseer/internal/core/domain"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("readFixture(%q) error = %v", name, err)
	}
	return data
}

type fakeCommander struct {
	stdout []byte
	stderr []byte
	err    error
	calls  []fakeCall
}

type fakeCall struct {
	dir  string
	args []string
}

func (f *fakeCommander) Run(_ context.Context, dir string, args ...string) ([]byte, []byte, error) {
	f.calls = append(f.calls, fakeCall{dir: dir, args: args})
	return f.stdout, f.stderr, f.err
}

func TestAdapter_GetForBranch_OpenPRWithGreenCI_ParsesAllFields(t *testing.T) {
	cmd := &fakeCommander{stdout: readFixture(t, "pr_open_success.json")}
	adapter := NewWithCommander(cmd, testLogger())

	pr, err := adapter.GetForBranch(context.Background(), "/repo", "feature/background-jobs")
	if err != nil {
		t.Fatalf("GetForBranch() error = %v", err)
	}
	if pr.Number != 42 {
		t.Fatalf("Number = %d, want 42", pr.Number)
	}
	if pr.Title != "Add background-jobs framework" {
		t.Fatalf("Title = %q", pr.Title)
	}
	if pr.State != domain.PRStateOpen {
		t.Fatalf("State = %q, want OPEN", pr.State)
	}
	if pr.URL != "https://github.com/dnlopes/overseer/pull/42" {
		t.Fatalf("URL = %q", pr.URL)
	}
	if pr.Branch != "feature/background-jobs" {
		t.Fatalf("Branch = %q", pr.Branch)
	}
	if pr.Author != "david.lopes" {
		t.Fatalf("Author = %q", pr.Author)
	}
	if pr.Stats != (domain.PRStats{Additions: 320, Deletions: 18, ChangedFiles: 12}) {
		t.Fatalf("Stats = %+v", pr.Stats)
	}
	if pr.Checks.Total != 3 || pr.Checks.Passing != 3 {
		t.Fatalf("Checks = %+v, want 3 total / 3 passing", pr.Checks)
	}
	if pr.Checks.OverallConclusion() != domain.CheckConclusionSuccess {
		t.Fatalf("OverallConclusion = %q, want SUCCESS", pr.Checks.OverallConclusion())
	}
}

func TestAdapter_GetForBranch_PassesCorrectArgsToGH(t *testing.T) {
	cmd := &fakeCommander{stdout: readFixture(t, "pr_open_success.json")}
	adapter := NewWithCommander(cmd, testLogger())

	_, err := adapter.GetForBranch(context.Background(), "/repo/path", "branch-x")
	if err != nil {
		t.Fatalf("GetForBranch() error = %v", err)
	}
	if len(cmd.calls) != 1 {
		t.Fatalf("commander called %d times, want 1", len(cmd.calls))
	}
	call := cmd.calls[0]
	if call.dir != "/repo/path" {
		t.Fatalf("dir = %q, want /repo/path", call.dir)
	}
	wantArgs := []string{"pr", "view", "branch-x", "--json", prJSONFields}
	if len(call.args) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", call.args, wantArgs)
	}
	for i := range wantArgs {
		if call.args[i] != wantArgs[i] {
			t.Fatalf("args[%d] = %q, want %q", i, call.args[i], wantArgs[i])
		}
	}
}

func TestAdapter_GetForBranch_FailingAndPendingChecks_SummarisedCorrectly(t *testing.T) {
	cmd := &fakeCommander{stdout: readFixture(t, "pr_open_failing_ci.json")}
	adapter := NewWithCommander(cmd, testLogger())

	pr, err := adapter.GetForBranch(context.Background(), "/repo", "fix/flaky")
	if err != nil {
		t.Fatalf("GetForBranch() error = %v", err)
	}
	if pr.Checks.Total != 3 || pr.Checks.Passing != 1 || pr.Checks.Failing != 1 || pr.Checks.Pending != 1 {
		t.Fatalf("Checks = %+v, want total=3 passing=1 failing=1 pending=1", pr.Checks)
	}
	if pr.Checks.OverallConclusion() != domain.CheckConclusionFailure {
		t.Fatalf("OverallConclusion = %q, want FAILURE", pr.Checks.OverallConclusion())
	}
}

func TestAdapter_GetForBranch_MergedPRNoChecks_StateMappedAndChecksEmpty(t *testing.T) {
	cmd := &fakeCommander{stdout: readFixture(t, "pr_merged_no_checks.json")}
	adapter := NewWithCommander(cmd, testLogger())

	pr, err := adapter.GetForBranch(context.Background(), "/repo", "scaffold")
	if err != nil {
		t.Fatalf("GetForBranch() error = %v", err)
	}
	if pr.State != domain.PRStateMerged {
		t.Fatalf("State = %q, want MERGED", pr.State)
	}
	if pr.Checks.Total != 0 {
		t.Fatalf("Checks.Total = %d, want 0", pr.Checks.Total)
	}
}

func TestAdapter_GetForBranch_GHReportsNoPR_ReturnsSentinelNotFound(t *testing.T) {
	cmd := &fakeCommander{
		stderr: []byte("no pull requests found for branch \"missing\"\n"),
		err:    errors.New("exit status 1"),
	}
	adapter := NewWithCommander(cmd, testLogger())

	_, err := adapter.GetForBranch(context.Background(), "/repo", "missing")
	if !errors.Is(err, domain.ErrPullRequestNotFound) {
		t.Fatalf("error = %v, want ErrPullRequestNotFound", err)
	}
}

func TestAdapter_GetForBranch_GHFails_WrappedError(t *testing.T) {
	cmd := &fakeCommander{
		stderr: []byte("error connecting to github.com"),
		err:    errors.New("exit status 1"),
	}
	adapter := NewWithCommander(cmd, testLogger())

	_, err := adapter.GetForBranch(context.Background(), "/repo", "branch")
	if err == nil {
		t.Fatal("error = nil, want wrapped error")
	}
	if errors.Is(err, domain.ErrPullRequestNotFound) {
		t.Fatal("unexpected ErrPullRequestNotFound for a generic gh failure")
	}
}

func TestAdapter_GetForBranch_MalformedJSON_ReturnsError(t *testing.T) {
	cmd := &fakeCommander{stdout: []byte("not json")}
	adapter := NewWithCommander(cmd, testLogger())

	_, err := adapter.GetForBranch(context.Background(), "/repo", "branch")
	if err == nil {
		t.Fatal("error = nil, want decode error")
	}
}

func TestAdapter_GetForBranch_UnknownState_ReturnsError(t *testing.T) {
	cmd := &fakeCommander{stdout: []byte(`{"number":1,"title":"t","state":"DRAFT","headRefName":"b","statusCheckRollup":[]}`)}
	adapter := NewWithCommander(cmd, testLogger())

	_, err := adapter.GetForBranch(context.Background(), "/repo", "b")
	if err == nil {
		t.Fatal("error = nil, want unknown state error")
	}
}

func TestAdapter_New_ProductionConstructor_UsesRealCommander(t *testing.T) {
	a := New(testLogger())
	if a == nil {
		t.Fatal("New() returned nil")
	}
	if a.cmd == nil {
		t.Fatal("New() left commander nil")
	}
}

var _ domain.PullRequestPort = (*Adapter)(nil)
