// Package jobs provides a generic recurring-work scheduler that lives inside
// the Bubble Tea event loop.
//
// A [Job] is a closure that knows how to do one unit of work and returns a
// tea.Cmd describing it. The [Model] arms a time-based tick per job and, when
// the tick fires, invokes the job again and re-arms. Jobs fan out to per-entity
// work by emitting a shared.JobsBatchMsg.
package jobs

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
)

type Job struct {
	ID       string
	Interval time.Duration
	Run      func() tea.Cmd
}

type Model struct {
	jobs map[string]Job
}

func New(js ...Job) Model {
	m := Model{jobs: make(map[string]Job, len(js))}
	for _, j := range js {
		m.jobs[j.ID] = j
	}
	return m
}

func (m Model) Init() tea.Cmd {
	if len(m.jobs) == 0 {
		return nil
	}
	cmds := make([]tea.Cmd, 0, 2*len(m.jobs))
	for _, j := range m.jobs {
		cmds = append(cmds, j.Run(), m.armTick(j))
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case shared.JobsTickMsg:
		j, ok := m.jobs[msg.JobID]
		if !ok {
			return m, nil
		}
		return m, tea.Batch(j.Run(), m.armTick(j))
	case shared.JobsBatchMsg:
		if len(msg.Cmds) == 0 {
			return m, nil
		}
		return m, tea.Batch(msg.Cmds...)
	}
	return m, nil
}

func (m Model) armTick(j Job) tea.Cmd {
	id := j.ID
	return tea.Tick(j.Interval, func(time.Time) tea.Msg {
		return shared.JobsTickMsg{JobID: id}
	})
}
