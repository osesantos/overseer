package session

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/testutil"
)

// newSwarmCreateForm builds a create form with swarm mode available, which is
// what the dashboard does when the board server is running.
func newSwarmCreateForm(t *testing.T, projects []domain.Project, initialProjectID uuid.UUID) CreateFormModel {
	t.Helper()
	svc := newCreateFormSessionService(t)
	projectsSvc, _ := newProjectsServiceWithMocks(t)
	return NewCreateForm(
		styles.New(), svc, projectsSvc, projects, initialProjectID,
		nil, nil, testLaunchers(t), 100, true, domain.SwarmMaxAgents,
	)
}

// hasField reports whether a field is in the focus order. Note focusIdxOf falls
// back to 0 for a missing field, so it cannot be used to test for absence.
func hasField(order []formField, target formField) bool {
	return slices.Contains(order, target)
}

// focusField moves the form's focus onto the named field.
func focusField(t *testing.T, form CreateFormModel, field formField) CreateFormModel {
	t.Helper()
	if !hasField(form.focusOrder, field) {
		t.Fatalf("field %v is not in the focus order %v", field, form.focusOrder)
	}
	form.focusIdx = focusIdxOf(form.focusOrder, field)
	form.updateFocusAndBlurs()
	return form
}

func TestCreateForm_SwarmUnavailable_HidesSwarmFields(t *testing.T) {
	form := newCreateFormForTest(t, nil)

	if hasField(form.focusOrder, fieldSwarmToggle) {
		t.Fatal("swarm toggle is focusable while swarm mode is unavailable")
	}
	if strings.Contains(form.View().Content, "Swarm?") {
		t.Fatal("View() shows the Swarm field while swarm mode is unavailable")
	}
}

func TestCreateForm_SwarmAvailable_ShowsToggleButNotCountUntilEnabled(t *testing.T) {
	form := newSwarmCreateForm(t, nil, uuid.Nil)

	view := form.View().Content
	if !strings.Contains(view, "Swarm?") {
		t.Fatalf("View() missing the Swarm toggle: %q", view)
	}
	if strings.Contains(view, "Agents") {
		t.Fatal("View() shows the Agents count before swarm is enabled")
	}
	if hasField(form.focusOrder, fieldSwarmSize) {
		t.Fatal("agents count is focusable before swarm is enabled")
	}
}

func TestCreateForm_EnablingSwarm_RevealsCountAndGoal(t *testing.T) {
	form := focusField(t, newSwarmCreateForm(t, nil, uuid.Nil), fieldSwarmToggle)

	updated, _ := tea.Model(form).Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	form = updated.(CreateFormModel)

	if !form.swarmEnabled {
		t.Fatal("space on the swarm toggle did not enable swarm mode")
	}
	for _, field := range []formField{fieldSwarmSize, fieldSwarmGoal} {
		if !hasField(form.focusOrder, field) {
			t.Fatalf("field %v is not focusable after enabling swarm", field)
		}
	}

	view := form.View().Content
	for _, want := range []string{"Swarm?", "Agents", "Goal"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing %q after enabling swarm: %q", want, view)
		}
	}
}

func TestCreateForm_SwarmSize_DefaultsToTheMinimum(t *testing.T) {
	form := newSwarmCreateForm(t, nil, uuid.Nil)

	if form.swarmSize != domain.SwarmMinAgents {
		t.Fatalf("swarmSize default = %d, want %d", form.swarmSize, domain.SwarmMinAgents)
	}
}

func TestCreateForm_SwarmSize_CyclesWithinBoundsAndWraps(t *testing.T) {
	form := newSwarmCreateForm(t, nil, uuid.Nil)
	form.swarmEnabled = true
	form.rebuildFocusOrder()
	form = focusField(t, form, fieldSwarmSize)

	// Forwards from the minimum up to the maximum, then wrap to the minimum.
	for want := domain.SwarmMinAgents + 1; want <= domain.SwarmMaxAgents; want++ {
		updated, _ := tea.Model(form).Update(formKeyPress("right"))
		form = updated.(CreateFormModel)
		if form.swarmSize != want {
			t.Fatalf("after right: swarmSize = %d, want %d", form.swarmSize, want)
		}
	}
	updated, _ := tea.Model(form).Update(formKeyPress("right"))
	form = updated.(CreateFormModel)
	if form.swarmSize != domain.SwarmMinAgents {
		t.Fatalf("swarmSize after wrapping = %d, want %d", form.swarmSize, domain.SwarmMinAgents)
	}

	// And backwards past the minimum wraps to the maximum.
	updated, _ = tea.Model(form).Update(formKeyPress("left"))
	form = updated.(CreateFormModel)
	if form.swarmSize != domain.SwarmMaxAgents {
		t.Fatalf("swarmSize after left from minimum = %d, want %d", form.swarmSize, domain.SwarmMaxAgents)
	}
}

func TestCreateForm_SwarmSize_RespectsAConfiguredCeiling(t *testing.T) {
	svc := newCreateFormSessionService(t)
	projectsSvc, _ := newProjectsServiceWithMocks(t)
	form := NewCreateForm(
		styles.New(), svc, projectsSvc, nil, uuid.Nil,
		nil, nil, testLaunchers(t), 100, true, 3,
	)
	form.swarmEnabled = true
	form.rebuildFocusOrder()
	form = focusField(t, form, fieldSwarmSize)

	seen := map[int]bool{}
	for range 6 {
		seen[form.swarmSize] = true
		updated, _ := tea.Model(form).Update(formKeyPress("right"))
		form = updated.(CreateFormModel)
	}

	for size := range seen {
		if size > 3 {
			t.Fatalf("swarmSize reached %d, above the configured ceiling of 3", size)
		}
	}
	if !seen[3] {
		t.Fatal("swarmSize never reached the configured ceiling of 3")
	}
}

func TestCreateForm_SubmitSwarmWithoutGoal_ShowsErrorAndDoesNotCreate(t *testing.T) {
	overseer := testutil.MakeProject("/repo/overseer", "Overseer")
	form := newSwarmCreateForm(t, []domain.Project{overseer}, overseer.ID)

	updated, _ := tea.Model(form).Update(formKeyPress("hive"))
	form = updated.(CreateFormModel)
	form.swarmEnabled = true
	form.rebuildFocusOrder()

	// No service mocks are primed, so a create call would fail the test outright.
	_, cmd := tea.Model(form).Update(formKeyPress("enter"))

	if cmd != nil {
		t.Fatal("submit without a goal returned a command, want the form to refuse")
	}
}

func TestCreateForm_SubmitSwarm_PassesSizeAndCarriesTheGoal(t *testing.T) {
	overseer := testutil.MakeProject("/repo/overseer", "Overseer")
	svc, repo, projects, tmux, _ := newCreateFormSessionServiceWithMocks(t)

	projects.EXPECT().Get(mock.Anything, overseer.ID).Return(overseer, nil).Once()
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.UUIDString(), overseer.Path, "").
		Return("tmux-shell", nil).Once()
	for index := 1; index <= 3; index++ {
		tmux.EXPECT().CreateSession(mock.Anything, testutil.SwarmAgentTmuxIDString(index), overseer.Path, "opencode").
			Return("tmux-agent", nil).Once()
	}
	repo.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()
	projects.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()

	projectsSvc, _ := newProjectsServiceWithMocks(t)
	form := NewCreateForm(
		styles.New(), svc, projectsSvc, []domain.Project{overseer}, overseer.ID,
		nil, nil, testLaunchers(t), 100, true, domain.SwarmMaxAgents,
	)

	updated, _ := tea.Model(form).Update(formKeyPress("hive"))
	form = updated.(CreateFormModel)
	form.createWorktree = false
	form.swarmEnabled = true
	form.swarmSize = 3
	form.goalInput.SetValue("summarise the storage layer")
	form.rebuildFocusOrder()

	_, cmd := tea.Model(form).Update(formKeyPress("enter"))
	if cmd == nil {
		t.Fatal("submit cmd = nil")
	}

	msg, ok := cmd().(shared.SessionCreatedMsg)
	if !ok {
		t.Fatalf("submit msg type = %T, want shared.SessionCreatedMsg", cmd())
	}
	if !msg.Session.IsSwarm() {
		t.Fatal("created session IsSwarm() = false, want true")
	}
	if msg.Session.SwarmSize != 3 {
		t.Fatalf("created session SwarmSize = %d, want 3", msg.Session.SwarmSize)
	}
	if msg.SwarmGoal != "summarise the storage layer" {
		t.Fatalf("SwarmGoal = %q, want the goal to reach bootstrapping", msg.SwarmGoal)
	}
}

func TestCreateForm_SubmitNonSwarm_CarriesNoGoal(t *testing.T) {
	overseer := testutil.MakeProject("/repo/overseer", "Overseer")
	svc, repo, projects, tmux, _ := newCreateFormSessionServiceWithMocks(t)

	projects.EXPECT().Get(mock.Anything, overseer.ID).Return(overseer, nil).Once()
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.UUIDString(), overseer.Path, "").
		Return("tmux-shell", nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.AgentTmuxIDString(), overseer.Path, "opencode").
		Return("tmux-agent", nil).Once()
	repo.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()
	projects.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()

	projectsSvc, _ := newProjectsServiceWithMocks(t)
	form := NewCreateForm(
		styles.New(), svc, projectsSvc, []domain.Project{overseer}, overseer.ID,
		nil, nil, testLaunchers(t), 100, true, domain.SwarmMaxAgents,
	)

	updated, _ := tea.Model(form).Update(formKeyPress("solo"))
	form = updated.(CreateFormModel)
	form.createWorktree = false
	// A goal typed and then swarm turned off must not leak through.
	form.goalInput.SetValue("stale goal")
	form.rebuildFocusOrder()

	_, cmd := tea.Model(form).Update(formKeyPress("enter"))
	if cmd == nil {
		t.Fatal("submit cmd = nil")
	}

	msg, ok := cmd().(shared.SessionCreatedMsg)
	if !ok {
		t.Fatalf("submit msg type = %T, want shared.SessionCreatedMsg", cmd())
	}
	if msg.Session.IsSwarm() {
		t.Fatal("created session IsSwarm() = true, want false")
	}
	if msg.SwarmGoal != "" {
		t.Fatalf("SwarmGoal = %q, want empty for a non-swarm session", msg.SwarmGoal)
	}
}
