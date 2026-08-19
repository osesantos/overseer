package main

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/dnlopes/overseer/internal/adapters/primary/boardhttp"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/dashboard"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/jobs"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/shared"
	"github.com/dnlopes/overseer/internal/adapters/primary/tui/styles"
	"github.com/dnlopes/overseer/internal/adapters/secondary/agentstatus/claudecode"
	"github.com/dnlopes/overseer/internal/adapters/secondary/agentstatus/opencode"
	agentstatusregistry "github.com/dnlopes/overseer/internal/adapters/secondary/agentstatus/registry"
	claudeadapter "github.com/dnlopes/overseer/internal/adapters/secondary/claude"
	"github.com/dnlopes/overseer/internal/adapters/secondary/git"
	githubcli "github.com/dnlopes/overseer/internal/adapters/secondary/github"
	"github.com/dnlopes/overseer/internal/adapters/secondary/storage"
	"github.com/dnlopes/overseer/internal/adapters/secondary/swarmboard"
	"github.com/dnlopes/overseer/internal/adapters/secondary/tmux"
	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/core/service"
	"github.com/dnlopes/overseer/internal/shared/config"
	"github.com/dnlopes/overseer/internal/shared/logger"
	"github.com/dnlopes/overseer/internal/shared/paths"
)

const pullRequestRefreshInterval = time.Minute

// boardShutdownTimeout bounds how long we wait for in-flight agent requests to
// drain when the TUI exits.
const boardShutdownTimeout = 5 * time.Second

func main() {
	cfg, err := config.Load(paths.ConfigFile())
	if err != nil {
		fmt.Fprintf(os.Stderr, "overseer: load config: %v\n", err)
		os.Exit(1)
	}

	resolver := paths.NewResolver(cfg.Storage.DataDir)

	log, logCloser, err := logger.New(resolver.LogFile(), cfg.Logging.Level)
	if err != nil {
		fmt.Fprintf(os.Stderr, "overseer: initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logCloser.Close()

	launchers, err := cfg.DomainLaunchers()
	if err != nil {
		log.Error("resolve launchers", "error", err)
		os.Exit(1)
	}
	var defaultLauncher domain.Launcher
	if len(launchers) > 0 {
		defaultLauncher = launchers[0]
	}

	labels, err := cfg.DomainLabels()
	if err != nil {
		log.Error("resolve labels", "error", err)
		os.Exit(1)
	}

	store, err := storage.New(resolver.DataFile(), log)
	if err != nil {
		log.Error("initialize storage", "error", err)
		os.Exit(1)
	}

	tmuxAdapter, err := tmux.New(log)
	if err != nil {
		log.Error("initialize tmux", "error", err)
		os.Exit(1)
	}
	if err := tmuxAdapter.EnsureExtendedKeys(context.Background()); err != nil {
		log.Warn("tmux: could not enable extended-keys", "error", err)
	}
	if err := tmuxAdapter.EnsureMouseMode(context.Background()); err != nil {
		log.Warn("tmux: could not enable mouse mode", "error", err)
	}

	gitAdapter, err := git.New(log)
	if err != nil {
		log.Error("initialize git", "error", err)
		os.Exit(1)
	}

	githubAdapter := githubcli.New(log)

	swarmBoard := swarmboard.New(resolver, log)

	sessionSvc := service.NewSessionService(store.Sessions(), store.Projects(), tmuxAdapter, gitAdapter, swarmBoard, resolver, defaultLauncher, cfg.EditorCommand, log)
	projectSvc := service.NewProjectService(store.Projects(), gitAdapter, log)
	prSvc := service.NewPullRequestService(githubAdapter, log)
	swarmSvc := service.NewSwarmService(swarmBoard, swarmBoard, store.Sessions(), tmuxAdapter, resolver, cfg.Swarm.MaxMessagesPerSession, log)

	// The board server is what lets the agent processes talk to each other. It
	// binds loopback only; boardURL is empty when swarm mode is off, which is the
	// signal the TUI uses to refuse swarm session creation.
	var boardURL string
	if cfg.Swarm.Enabled {
		boardServer := boardhttp.New(swarmSvc, log)
		boardURL, err = boardServer.Start(cfg.Swarm.BoardAddr)
		if err != nil {
			log.Error("start swarm board server", "error", err)
			os.Exit(1)
		}
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), boardShutdownTimeout)
			defer cancel()
			if err := boardServer.Shutdown(shutdownCtx); err != nil {
				log.Warn("swarm board server shutdown", "error", err)
			}
		}()
	}

	// Overseer Agent (optional: gracefully disabled when claude is not on PATH).
	var overseerSvc *service.OverseerService
	claudeAgent, err := claudeadapter.New(log)
	if err != nil {
		log.Warn("overseer agent disabled: claude binary not found", "error", err)
	} else {
		overseerSvc = service.NewOverseerService(claudeAgent, log)
	}

	agentStatusRegistry := agentstatusregistry.New()
	agentStatusRegistry.Register(claudecode.NewPaneDetector(tmuxAdapter))
	agentStatusRegistry.Register(opencode.NewPaneDetector(tmuxAdapter))
	agentStatusSvc := service.NewAgentStatusService(store.Sessions(), tmuxAdapter, agentStatusRegistry, log)

	prJob := buildPullRequestJob(sessionSvc, projectSvc, prSvc)
	schedulerJobs := []jobs.Job{prJob}
	if cfg.AgentStatus.Enabled {
		schedulerJobs = append(schedulerJobs, buildAgentStatusJob(agentStatusSvc, cfg.AgentStatus.RefreshInterval))
	}
	if cfg.Swarm.Enabled {
		schedulerJobs = append(schedulerJobs, buildSwarmNudgeJob(swarmSvc, cfg.Swarm.NudgeDebounce))
	}
	scheduler := jobs.New(schedulerJobs...)

	previewRefresh, err := cfg.PreviewRefreshDuration()
	if err != nil {
		log.Error("resolve preview refresh interval", "error", err)
		os.Exit(1)
	}

	discoveryPaths, err := cfg.ExpandedDiscoveryPaths()
	if err != nil {
		log.Warn("could not resolve project discovery paths, skipping autodiscovery", "error", err)
		discoveryPaths = nil
	}

	s := styles.NewWithTheme(cfg.Theme, cfg.DisableEmoji)
	dash := dashboard.New(s, *sessionSvc, *projectSvc, overseerSvc, swarmSvc, scheduler, launchers, labels, cfg.Dashboard.MinWidth, cfg.Dashboard.MinHeight, previewRefresh, discoveryPaths, boardURL, cfg.Swarm.MaxAgents)
	p := tea.NewProgram(altScreenModel{inner: dash})

	if _, err := p.Run(); err != nil {
		log.Error("run tui", "error", err)
		os.Exit(1)
	}
}

func buildPullRequestJob(
	sessionSvc *service.SessionService,
	projectSvc *service.ProjectService,
	prSvc *service.PullRequestService,
) jobs.Job {
	return jobs.Job{
		ID:       "pr-status-refresh",
		Interval: pullRequestRefreshInterval,
		Run: func() tea.Cmd {
			return shared.Request(
				func(ctx context.Context) (pollData, error) {
					sessionsResp, err := sessionSvc.List(ctx, service.ListSessionsRequest{})
					if err != nil {
						return pollData{}, fmt.Errorf("list sessions: %w", err)
					}
					projectsResp, err := projectSvc.List(ctx, service.ListProjectsRequest{})
					if err != nil {
						return pollData{}, fmt.Errorf("list projects: %w", err)
					}
					return pollData{Sessions: sessionsResp.Sessions, Projects: projectsResp.Projects}, nil
				},
				func(data pollData, err error) tea.Msg {
					if err != nil {
						return nil
					}
					return shared.JobsBatchMsg{Cmds: fanOutPRFetches(prSvc, data)}
				},
			)
		},
	}
}

// buildSwarmNudgeJob drives the swarm heartbeat. Agent CLIs do not loop, so a
// post from one agent would otherwise sit unread; this job periodically pokes
// every agent whose board has moved. The interval doubles as the debounce
// window: a burst of posts inside one tick costs a single round of pokes.
func buildSwarmNudgeJob(svc *service.SwarmService, interval time.Duration) jobs.Job {
	return jobs.Job{
		ID:       "swarm-nudge-flush",
		Interval: interval,
		Run: func() tea.Cmd {
			return shared.Request(
				func(ctx context.Context) (service.FlushSwarmNudgesResponse, error) {
					// Submitting freshly typed briefings comes first: a new swarm's
					// agents are still starting up when they are briefed, so the
					// Enter that submits their prompt has to be retried here.
					if _, err := svc.SubmitBriefings(ctx, service.SubmitSwarmBriefingsRequest{}); err != nil {
						return service.FlushSwarmNudgesResponse{}, err
					}
					return svc.FlushNudges(ctx, service.FlushSwarmNudgesRequest{})
				},
				func(_ service.FlushSwarmNudgesResponse, _ error) tea.Msg {
					// Nothing for the UI to react to: failures are logged in the
					// service, per-agent, so one dead pane stays visible without
					// interrupting the operator.
					return nil
				},
			)
		},
	}
}

func buildAgentStatusJob(svc *service.AgentStatusService, interval time.Duration) jobs.Job {
	return jobs.Job{
		ID:       "agent-status-refresh",
		Interval: interval,
		Run: func() tea.Cmd {
			return shared.Request(
				func(ctx context.Context) (service.PollAllAgentStatusesResponse, error) {
					return svc.PollAll(ctx, service.PollAllAgentStatusesRequest{})
				},
				func(resp service.PollAllAgentStatusesResponse, err error) tea.Msg {
					return shared.AgentStatusesUpdatedMsg{Statuses: resp.Statuses, Err: err}
				},
			)
		},
	}
}

type pollData struct {
	Sessions []domain.Session
	Projects []domain.Project
}

func fanOutPRFetches(prSvc *service.PullRequestService, data pollData) []tea.Cmd {
	projectByID := make(map[uuid.UUID]domain.Project, len(data.Projects))
	for _, p := range data.Projects {
		projectByID[p.ID] = p
	}
	cmds := make([]tea.Cmd, 0, len(data.Sessions))
	for _, sess := range data.Sessions {
		project, ok := projectByID[sess.ProjectID]
		if !ok {
			continue
		}
		if !sess.HasWorktree() {
			continue
		}
		sid, branch, repoPath := sess.ID, sess.Branch, project.Path
		cmds = append(cmds, shared.Request(
			func(ctx context.Context) (domain.PullRequest, error) {
				resp, err := prSvc.GetForBranch(ctx, service.GetPullRequestForBranchRequest{
					RepoPath: repoPath,
					Branch:   branch,
				})
				return resp.PullRequest, err
			},
			func(pr domain.PullRequest, err error) tea.Msg {
				return shared.PRStatusUpdatedMsg{SessionID: sid, PR: pr, Err: err}
			},
		))
	}
	return cmds
}

type altScreenModel struct {
	inner tea.Model
}

func (m altScreenModel) Init() tea.Cmd {
	return m.inner.Init()
}

func (m altScreenModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	inner, cmd := m.inner.Update(msg)
	m.inner = inner
	return m, cmd
}

func (m altScreenModel) View() tea.View {
	v := m.inner.View()
	v.AltScreen = true
	return v
}
