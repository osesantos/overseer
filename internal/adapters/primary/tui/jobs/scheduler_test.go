package jobs

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
)

func noopRun() tea.Cmd { return nil }

func TestModel_New_StoresJobsByID(t *testing.T) {
	a := Job{ID: "a", Interval: time.Hour, Run: noopRun}
	b := Job{ID: "b", Interval: time.Hour, Run: noopRun}

	m := New(a, b)

	if got := len(m.jobs); got != 2 {
		t.Fatalf("len(jobs) = %d, want 2", got)
	}
	if _, ok := m.jobs["a"]; !ok {
		t.Fatal("job 'a' missing from scheduler")
	}
	if _, ok := m.jobs["b"]; !ok {
		t.Fatal("job 'b' missing from scheduler")
	}
}

func TestModel_New_DuplicateID_LastWins(t *testing.T) {
	first := Job{ID: "dup", Interval: time.Hour, Run: noopRun}
	second := Job{ID: "dup", Interval: 2 * time.Hour, Run: noopRun}

	m := New(first, second)

	if got := len(m.jobs); got != 1 {
		t.Fatalf("len(jobs) = %d, want 1 (duplicate IDs collapse)", got)
	}
	if got := m.jobs["dup"].Interval; got != 2*time.Hour {
		t.Fatalf("dup.Interval = %v, want 2h (last wins)", got)
	}
}

func TestModel_Init_NoJobs_ReturnsNilCmd(t *testing.T) {
	m := New()
	if cmd := m.Init(); cmd != nil {
		t.Fatalf("Init() with zero jobs returned non-nil cmd")
	}
}

func TestModel_Init_InvokesRunOnceForEachJob(t *testing.T) {
	var aCount, bCount int
	a := Job{ID: "a", Interval: time.Hour, Run: func() tea.Cmd { aCount++; return nil }}
	b := Job{ID: "b", Interval: time.Hour, Run: func() tea.Cmd { bCount++; return nil }}

	m := New(a, b)
	if cmd := m.Init(); cmd == nil {
		t.Fatal("Init() returned nil with jobs configured")
	}

	if aCount != 1 {
		t.Fatalf("Job 'a' Run called %d times, want 1", aCount)
	}
	if bCount != 1 {
		t.Fatalf("Job 'b' Run called %d times, want 1", bCount)
	}
}

func TestModel_Update_JobsTickMsg_KnownJob_FiresRunAndReturnsCmd(t *testing.T) {
	var runs int
	j := Job{ID: "poll", Interval: time.Hour, Run: func() tea.Cmd { runs++; return nil }}
	m := New(j)

	_, cmd := m.Update(shared.JobsTickMsg{JobID: "poll"})
	if cmd == nil {
		t.Fatal("Update(tickMsg) returned nil cmd; tick should re-arm")
	}
	if runs != 1 {
		t.Fatalf("Run invocations = %d, want 1", runs)
	}
}

func TestModel_Update_JobsTickMsg_UnknownJob_IsNoOp(t *testing.T) {
	var runs int
	j := Job{ID: "poll", Interval: time.Hour, Run: func() tea.Cmd { runs++; return nil }}
	m := New(j)

	_, cmd := m.Update(shared.JobsTickMsg{JobID: "ghost"})
	if cmd != nil {
		t.Fatal("Update(unknown tickMsg) returned non-nil cmd")
	}
	if runs != 0 {
		t.Fatalf("Run called for unknown job; got %d invocations", runs)
	}
}

func TestModel_Update_JobsBatchMsg_NonEmpty_ReturnsNonNilCmd(t *testing.T) {
	m := New()
	_, cmd := m.Update(shared.JobsBatchMsg{
		Cmds: []tea.Cmd{func() tea.Msg { return nil }, func() tea.Msg { return nil }},
	})
	if cmd == nil {
		t.Fatal("Update(BatchMsg{cmds:[...]}) returned nil; want batched cmd")
	}
}

func TestModel_Update_JobsBatchMsg_Empty_ReturnsNilCmd(t *testing.T) {
	m := New()
	_, cmd := m.Update(shared.JobsBatchMsg{Cmds: nil})
	if cmd != nil {
		t.Fatal("Update(BatchMsg{cmds:nil}) returned non-nil cmd")
	}
}

func TestModel_Update_UnknownMsg_IsNoOp(t *testing.T) {
	m := New(Job{ID: "x", Interval: time.Hour, Run: noopRun})

	_, cmd := m.Update("not a known message type")
	if cmd != nil {
		t.Fatal("Update(unknown msg) returned non-nil cmd")
	}
}

func TestModel_Update_TickMsg_ReExecutesRunOnEveryTick(t *testing.T) {
	var runs int
	j := Job{ID: "poll", Interval: time.Hour, Run: func() tea.Cmd { runs++; return nil }}
	m := New(j)

	_, _ = m.Update(shared.JobsTickMsg{JobID: "poll"})
	_, _ = m.Update(shared.JobsTickMsg{JobID: "poll"})
	_, _ = m.Update(shared.JobsTickMsg{JobID: "poll"})

	if runs != 3 {
		t.Fatalf("Run invocations = %d, want 3 (one per tick)", runs)
	}
}
