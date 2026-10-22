package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/shared/paths"
	"github.com/dnlopes/overseer/internal/testutil"
)

func humanPR(repo string, number int, author string) domain.PullRequest {
	return domain.PullRequest{Repo: repo, Number: number, Title: "fix(RTAFB-1): something", State: domain.PRStateOpen, Author: author}
}

func TestSessionService_CreateForPullRequest_ChecksOutThePRHeadAndLaunchesTheReview(t *testing.T) {
	projectID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	repoPath := expectProjectLookup(t, projects, projectID, "signer")
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	git.EXPECT().CreateDetachedWorktree(mock.Anything, repoPath, "pull/92/head", mock.Anything).Return(nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.UUIDString(), mock.Anything, "").Return("shell", nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, testutil.AgentTmuxIDString(), mock.Anything, `claude "/ose-pr-review 92"`).Return("agent", nil).Once()
	var saved domain.Session
	repo.EXPECT().Save(mock.Anything, mock.Anything).Run(func(_ context.Context, s domain.Session) { saved = s }).Return(nil).Once()
	projects.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.CreateForPullRequest(context.Background(), CreateReviewSessionRequest{
		PullRequest:  humanPR("OutSystems/signer", 92, "alice"),
		ProjectID:    projectID,
		AgentCommand: `claude "/ose-pr-review 92"`,
	})

	if err != nil {
		t.Fatalf("CreateForPullRequest() error = %v", err)
	}
	if resp.Session.Name != "PR #92 fix(RTAFB-1): something" {
		t.Fatalf("Name = %q", resp.Session.Name)
	}
	if resp.Session.ReviewPR != (domain.PullRequestRef{Repo: "OutSystems/signer", Number: 92}) {
		t.Fatalf("ReviewPR = %+v", resp.Session.ReviewPR)
	}
	if resp.Session.AgentType != domain.AgentTypeClaudeCode {
		t.Fatalf("AgentType = %q, want claude-code inferred from the command", resp.Session.AgentType)
	}
	if want := paths.NewResolver("").SessionWorktreePath(resp.Session.ID); saved.WorktreePath != want || saved.Branch != "pull/92/head" {
		t.Fatalf("saved worktree = %q / %q", saved.WorktreePath, saved.Branch)
	}
}

func TestSessionService_CreateForPullRequest_TruncatesLongTitles(t *testing.T) {
	projectID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	expectProjectLookup(t, projects, projectID, "signer")
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	git.EXPECT().CreateDetachedWorktree(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("x", nil).Twice()
	repo.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()
	projects.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()

	pr := humanPR("o/r", 5, "alice")
	pr.Title = "chore(RTAFB-7437): Bump the dependencies group across 1 directory with 10 updates and a very long tail"
	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.CreateForPullRequest(context.Background(), CreateReviewSessionRequest{PullRequest: pr, ProjectID: projectID, AgentCommand: "claude"})

	if err != nil {
		t.Fatalf("CreateForPullRequest() error = %v", err)
	}
	if resp.Session.Name != "PR #5 chore(RTAFB-7437): Bump the dependencie…" {
		t.Fatalf("Name = %q", resp.Session.Name)
	}
}

func TestSessionService_CreateForPullRequest_WorktreeFailure_CreatesNothing(t *testing.T) {
	projectID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	expectProjectLookup(t, projects, projectID, "signer")
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	git.EXPECT().CreateDetachedWorktree(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("fetch failed")).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.CreateForPullRequest(context.Background(), CreateReviewSessionRequest{PullRequest: humanPR("o/r", 5, "alice"), ProjectID: projectID, AgentCommand: "claude"})

	if err == nil {
		t.Fatal("CreateForPullRequest() error = nil, want worktree failure")
	}
}

func TestSessionService_CreateForPullRequest_TmuxFailure_RemovesTheWorktree(t *testing.T) {
	projectID := uuid.New()
	repo, projects, tmux, git := newSessionMocks(t)
	repoPath := expectProjectLookup(t, projects, projectID, "signer")
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Once()
	git.EXPECT().CreateDetachedWorktree(mock.Anything, repoPath, "pull/5/head", mock.Anything).Return(nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, mock.Anything, mock.Anything, "").Return("", errors.New("tmux down")).Once()
	git.EXPECT().RemoveWorktree(mock.Anything, repoPath, mock.Anything).Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	_, err := svc.CreateForPullRequest(context.Background(), CreateReviewSessionRequest{PullRequest: humanPR("o/r", 5, "alice"), ProjectID: projectID, AgentCommand: "claude"})

	if err == nil {
		t.Fatal("CreateForPullRequest() error = nil, want tmux failure")
	}
}

func TestSessionService_AutoReviewPullRequests_StartsOnlyEligiblePRsUpToTheCap(t *testing.T) {
	projectID := uuid.New()
	project := testutil.MakeProject("/repo/signer", "signer")
	project.ID = projectID
	repo, projects, tmux, git := newSessionMocks(t)

	taken, _ := domain.NewSession("PR #1 old", projectID)
	_ = taken.AssignReviewTarget(domain.PullRequestRef{Repo: "o/signer", Number: 1})
	projects.EXPECT().List(mock.Anything).Return([]domain.Project{project}, nil).Once()
	repo.EXPECT().List(mock.Anything).Return([]domain.Session{taken}, nil).Once()

	inbox := domain.ReviewInbox{Viewer: "me", PullRequests: []domain.PullRequest{
		humanPR("o/signer", 1, "alice"), // taken
		{Repo: "o/signer", Number: 2, State: domain.PRStateOpen, Author: "bot", AuthorIsBot: true},
		humanPR("o/signer", 3, "me"),       // own
		humanPR("o/elsewhere", 4, "alice"), // no project
		humanPR("o/signer", 5, "alice"),
		humanPR("o/signer", 6, "bob"),
		humanPR("o/signer", 7, "carol"),
	}}

	// Two creations happen (the cap), each with its own project lookup, order
	// scan, worktree, two panes and two saves.
	projects.EXPECT().Get(mock.Anything, projectID).Return(project, nil).Twice()
	repo.EXPECT().List(mock.Anything).Return([]domain.Session{taken}, nil).Twice()
	var refs []string
	git.EXPECT().CreateDetachedWorktree(mock.Anything, "/repo/signer", mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ string, ref string, _ string) { refs = append(refs, ref) }).Return(nil).Twice()
	var commands []string
	tmux.EXPECT().CreateSession(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ string, _ string, cmd string) {
			if cmd != "" {
				commands = append(commands, cmd)
			}
		}).Return("x", nil).Times(4)
	repo.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Twice()
	projects.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Twice()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.AutoReviewPullRequests(context.Background(), AutoReviewPullRequestsRequest{
		Inbox:           inbox,
		CommandTemplate: `claude "/ose-pr-review {{number}}"`,
		MaxPerPoll:      2,
	})

	if err != nil {
		t.Fatalf("AutoReviewPullRequests() error = %v", err)
	}
	if len(resp.Created) != 2 {
		t.Fatalf("Created = %d sessions, want 2 (the cap)", len(resp.Created))
	}
	if len(refs) != 2 || refs[0] != "pull/5/head" || refs[1] != "pull/6/head" {
		t.Fatalf("worktree refs = %v, want the first two eligible PRs in inbox order", refs)
	}
	if len(commands) != 2 || commands[0] != `claude "/ose-pr-review 5"` || commands[1] != `claude "/ose-pr-review 6"` {
		t.Fatalf("agent commands = %v", commands)
	}
}

func TestSessionService_AutoReviewPullRequests_OneFailureDoesNotStopTheRest(t *testing.T) {
	projectID := uuid.New()
	project := testutil.MakeProject("/repo/signer", "signer")
	project.ID = projectID
	repo, projects, tmux, git := newSessionMocks(t)
	projects.EXPECT().List(mock.Anything).Return([]domain.Project{project}, nil).Once()
	repo.EXPECT().List(mock.Anything).Return(nil, nil).Times(3)
	projects.EXPECT().Get(mock.Anything, projectID).Return(project, nil).Twice()
	git.EXPECT().CreateDetachedWorktree(mock.Anything, "/repo/signer", "pull/5/head", mock.Anything).Return(errors.New("fetch failed")).Once()
	git.EXPECT().CreateDetachedWorktree(mock.Anything, "/repo/signer", "pull/6/head", mock.Anything).Return(nil).Once()
	tmux.EXPECT().CreateSession(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("x", nil).Twice()
	repo.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()
	projects.EXPECT().Save(mock.Anything, mock.Anything).Return(nil).Once()

	svc := newTestSessionService(repo, projects, tmux, git, testLogger())
	resp, err := svc.AutoReviewPullRequests(context.Background(), AutoReviewPullRequestsRequest{
		Inbox:           domain.ReviewInbox{Viewer: "me", PullRequests: []domain.PullRequest{humanPR("o/signer", 5, "a"), humanPR("o/signer", 6, "b")}},
		CommandTemplate: "claude {{number}}",
		MaxPerPoll:      5,
	})

	if err != nil {
		t.Fatalf("AutoReviewPullRequests() error = %v", err)
	}
	if len(resp.Created) != 1 || resp.Created[0].ReviewPR.Number != 6 {
		t.Fatalf("Created = %+v, want only PR 6", resp.Created)
	}
}
