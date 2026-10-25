package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/shared/paths"
	"github.com/dnlopes/overseer/internal/testutil"
	"github.com/dnlopes/overseer/internal/testutil/mocks"
)

var triageNow = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

func freshIssue(key string, age time.Duration) domain.Issue {
	return domain.Issue{
		Key:            key,
		Project:        domain.IssueProjectKey(key),
		Summary:        "Deployment Failure: [TEST] [commercial] Step system-tests TIMEOUT for elworker",
		StatusCategory: domain.IssueStatusNew,
		CreatedAt:      triageNow.Add(-age),
	}
}

func TestSessionService_CreateForIssue_StartsAProjectSessionRunningTheTriage(t *testing.T) {
	projectID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	vaultPath := expectProjectLookup(t, projects, projectID, "vault")
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.UUIDString(), vaultPath, "").Return("shell", nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.AgentTmuxIDString(), vaultPath, claudeLaunch(`"/ose-rdodcf-triage RDODCF-7699"`)).Return("agent", nil).Once()
	var saved domain.Session
	repo.EXPECT().Save(mock.Anything, mock.Anything).Run(func(_ context.Context, s domain.Session) { saved = s }).Return(nil).Once()
	projects.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.CreateForIssue(context.Background(), CreateTriageSessionRequest{
		Issue:        freshIssue("RDODCF-7699", time.Hour),
		ProjectID:    projectID,
		AgentCommand: `claude "/ose-rdodcf-triage RDODCF-7699"`,
	})

	if err != nil {
		t.Fatalf("CreateForIssue() error = %v", err)
	}
	if resp.Session.Name != "RDODCF-7699 Deployment Failure: [TEST] [commercial]…" {
		t.Fatalf("Name = %q", resp.Session.Name)
	}
	if resp.Session.TriageIssue != "RDODCF-7699" || resp.Session.AgentType != domain.AgentTypeClaudeCode {
		t.Fatalf("session = %+v", resp.Session)
	}
	if saved.HasWorktree() {
		t.Fatalf("saved worktree = %q, want a project session", saved.WorktreePath)
	}
}

func TestSessionService_AutoTriageIssues_StartsOnlyEligibleIssuesOldestFirstUpToTheCap(t *testing.T) {
	vault := testutil.MakeProject("/repo/vault", "vault")
	repo, projects, tmux, git := newSessionMocks(t)
	taken, _ := domain.NewSession("RDODCF-1 old", vault.ID)
	_ = taken.AssignTriageTarget("RDODCF-1")
	projects.EXPECT().List(mock.Anything).Return([]domain.Project{vault}, nil).Once()
	repo.EXPECT().List(mock.Anything).Return([]domain.Session{taken}, nil).Once()

	assigned := freshIssue("RDODCF-2", time.Hour)
	assigned.Assignee = "Vera Cardoso"
	inbox := domain.TriageInbox{Issues: []domain.Issue{
		freshIssue("RDODCF-1", time.Hour), // taken
		assigned,
		freshIssue("RDODCF-3", 30*24*time.Hour), // too old
		freshIssue("RDINC-4", time.Hour),        // no launcher
		freshIssue("RDODCF-5", time.Hour),
		freshIssue("RDODCF-6", 3*time.Hour),
		freshIssue("RDODCF-7", 2*time.Hour),
	}}

	projects.EXPECT().Get(mock.Anything, vault.ID).Return(vault, nil).Twice()
	repo.EXPECT().List(mock.Anything).Return([]domain.Session{taken}, nil).Twice()
	tmux.EXPECT().CreateSession(mock.Anything, mock.Anything, "/repo/vault", mock.Anything).Return("x", nil).Times(4)
	repo.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Twice()
	projects.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Twice()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.AutoTriageIssues(context.Background(), AutoTriageIssuesRequest{
		Inbox:       inbox,
		Commands:    map[string]string{"RDODCF": `claude "/ose-rdodcf-triage {{key}}"`},
		ProjectName: "vault",
		MaxPerPoll:  2,
		MaxAge:      7 * 24 * time.Hour,
		Now:         triageNow,
	})

	if err != nil {
		t.Fatalf("AutoTriageIssues() error = %v", err)
	}
	if len(resp.Created) != 2 {
		t.Fatalf("Created = %d sessions, want 2 (the cap)", len(resp.Created))
	}
	if resp.Created[0].AgentCommand != `claude "/ose-rdodcf-triage RDODCF-6"` || resp.Created[1].AgentCommand != `claude "/ose-rdodcf-triage RDODCF-7"` {
		t.Fatalf("agent commands = %q, %q, want the two oldest eligible issues", resp.Created[0].AgentCommand, resp.Created[1].AgentCommand)
	}
}

func TestSessionService_AutoTriageIssues_MissingProject_ReturnsProjectNotFound(t *testing.T) {
	repo, projects, tmux, git := newSessionMocks(t)
	projects.EXPECT().List(mock.Anything).Return(nil, nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.AutoTriageIssues(context.Background(), AutoTriageIssuesRequest{
		Inbox:       domain.TriageInbox{Issues: []domain.Issue{freshIssue("RDODCF-1", time.Hour)}},
		Commands:    map[string]string{"RDODCF": "claude {{key}}"},
		ProjectName: "vault",
		MaxPerPoll:  1,
		MaxAge:      time.Hour * 24,
		Now:         triageNow,
	})

	if !errors.Is(err, domain.ErrProjectNotFound) {
		t.Fatalf("AutoTriageIssues() error = %v, want ErrProjectNotFound", err)
	}
}

func TestSessionService_Delete_TriageSession_MarksTheIssueHandled(t *testing.T) {
	projectID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	sess, _ := domain.NewSession("RDODCF-7", projectID)
	_ = sess.AssignTriageTarget("RDODCF-7")
	repo.EXPECT().Get(mock.Anything, sess.ID).Return(sess, nil).Once()
	tmux.EXPECT().GetSession(mock.Anything, mock.Anything).Return(domain.TmuxSession{}, domain.ErrTmuxSessionNotFound).Times(3)
	repo.EXPECT().Delete(mock.Anything, sess.ID).Return(nil).Once()
	triage := mocks.NewMockTriageSettingsRepository(t)
	triage.EXPECT().MarkHandled(mock.Anything, "RDODCF-7").Return(nil).Once()

	launcher, _ := domain.NewLauncher("OpenCode", "opencode", domain.AgentTypeOpenCode)
	svc := NewSessionService(repo, projects, tmux, git, noopBoard{}, noopReviews{}, triage, paths.NewResolver(""), launcher, "nvim", testLogger())
	if _, err := svc.Delete(context.Background(), DeleteSessionRequest{ID: sess.ID}); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}
