// Package github provides a PullRequestPort implementation backed by the
// gh CLI (https://cli.github.com).
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/dnlopes/overseer/internal/core/domain"
)

var _ domain.PullRequestPort = (*Adapter)(nil)

const prJSONFields = "number,title,state,isDraft,url,headRefName,author,additions,deletions,changedFiles,statusCheckRollup,updatedAt"

// ponytail: first 100 threads / 50 comments each; paginate if a PR ever outgrows that.
const reviewThreadsQuery = `query($owner: String!, $repo: String!, $number: Int!) {
  viewer { login }
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      reviewThreads(first: 100) {
        nodes { isResolved comments(first: 50) { nodes { author { __typename login } } } }
      }
    }
  }
}`

// ponytail: first 100 PRs and 50 reviews each; the inbox was 69 when this was written.
const requestedReviewsQuery = `query($q: String!) {
  search(type: ISSUE, first: 100, query: $q) {
    issueCount
    nodes { ... on PullRequest {
      number title url isDraft state createdAt updatedAt headRefName
      author { login __typename }
      repository { nameWithOwner }
      additions deletions changedFiles
      reviews(first: 50) { nodes { author { login } } }
      commits(last: 1) { nodes { commit { statusCheckRollup { contexts(first: 100) { nodes {
        __typename
        ... on CheckRun { name status conclusion }
        ... on StatusContext { context state }
      } } } } } }
    } }
  }
}`

type Commander interface {
	Run(ctx context.Context, dir string, args ...string) (stdout []byte, stderr []byte, err error)
}

// searchRetries and searchRetryDelay cover GitHub search intermittently
// answering an empty page for a query that has results (observed during a
// "degraded performance" incident: 59, 0, 59 for the same query).
const (
	searchRetries    = 3
	searchRetryDelay = 2 * time.Second
)

type Adapter struct {
	cmd        Commander
	logger     *slog.Logger
	retryDelay time.Duration
}

func New(logger *slog.Logger) *Adapter {
	return &Adapter{cmd: realCommander{}, logger: logger, retryDelay: searchRetryDelay}
}

// NewWithCommander is for tests: it retries searches without waiting.
func NewWithCommander(cmd Commander, logger *slog.Logger) *Adapter {
	return &Adapter{cmd: cmd, logger: logger}
}

func (a *Adapter) GetForBranch(ctx context.Context, repoPath, branch string) (domain.PullRequest, error) {
	stdout, stderr, err := a.cmd.Run(ctx, repoPath,
		"pr", "view", branch,
		"--json", prJSONFields,
	)
	if err != nil {
		combined := strings.ToLower(string(stderr) + " " + string(stdout))
		if strings.Contains(combined, "no pull requests found") || strings.Contains(combined, "no open pull requests") {
			return domain.PullRequest{}, domain.ErrPullRequestNotFound
		}
		return domain.PullRequest{}, fmt.Errorf("gh pr view %s: %w (stderr: %s)", branch, err, strings.TrimSpace(string(stderr)))
	}
	pr, err := parseGHJSON(stdout)
	if err != nil || (pr.State != domain.PRStateOpen && pr.State != domain.PRStateDraft) {
		return pr, err
	}
	comments, err := a.reviewComments(ctx, repoPath, pr)
	if err != nil {
		a.logger.WarnContext(ctx, "fetch pr review threads", "pr", pr.Number, "err", err)
		return pr, nil
	}
	pr.Comments = comments
	return pr, nil
}

// ListRequestedReviews resolves the viewer first because GraphQL search does not
// expand "@me"; "review-requested:<login>" already includes team requests.
func (a *Adapter) ListRequestedReviews(ctx context.Context) (domain.ReviewInbox, error) {
	stdout, stderr, err := a.cmd.Run(ctx, "", "api", "user", "--jq", ".login")
	if err != nil {
		return domain.ReviewInbox{}, fmt.Errorf("gh api user: %w (stderr: %s)", err, strings.TrimSpace(string(stderr)))
	}
	viewer := strings.TrimSpace(string(stdout))

	var prs []domain.PullRequest
	for attempt := 1; attempt <= searchRetries; attempt++ {
		stdout, stderr, err = a.cmd.Run(ctx, "",
			"api", "graphql",
			"-f", "query="+requestedReviewsQuery,
			"-f", "q=is:pr is:open review-requested:"+viewer,
		)
		if err != nil {
			return domain.ReviewInbox{}, fmt.Errorf("gh api graphql requested reviews: %w (stderr: %s)", err, strings.TrimSpace(string(stderr)))
		}
		var total int
		prs, total, err = parseRequestedReviews(stdout, viewer)
		if err != nil {
			return domain.ReviewInbox{}, err
		}
		if total > len(prs) {
			a.logger.WarnContext(ctx, "review inbox truncated by the search page size", "total", total, "fetched", len(prs))
		}
		if len(prs) > 0 || attempt == searchRetries {
			break
		}
		a.logger.WarnContext(ctx, "review inbox came back empty, retrying", "attempt", attempt)
		select {
		case <-ctx.Done():
			return domain.ReviewInbox{}, ctx.Err()
		case <-time.After(a.retryDelay):
		}
	}
	return domain.ReviewInbox{Viewer: viewer, PullRequests: prs}, nil
}

type ghSearchJSON struct {
	Data struct {
		Search struct {
			IssueCount int `json:"issueCount"`
			Nodes      []struct {
				ghPRJSON
				CreatedAt  string `json:"createdAt"`
				Repository struct {
					NameWithOwner string `json:"nameWithOwner"`
				} `json:"repository"`
				AuthorTyped struct {
					Login    string `json:"login"`
					Typename string `json:"__typename"`
				} `json:"author"`
				Reviews struct {
					Nodes []ghReviewJSON `json:"nodes"`
				} `json:"reviews"`
				Commits struct {
					Nodes []struct {
						Commit struct {
							StatusCheckRollup *struct {
								Contexts struct {
									Nodes []ghCheckJSON `json:"nodes"`
								} `json:"contexts"`
							} `json:"statusCheckRollup"`
						} `json:"commit"`
					} `json:"nodes"`
				} `json:"commits"`
			} `json:"nodes"`
		} `json:"search"`
	} `json:"data"`
}

type ghReviewJSON struct {
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
}

func parseRequestedReviews(data []byte, viewer string) ([]domain.PullRequest, int, error) {
	var raw ghSearchJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, 0, fmt.Errorf("decode gh search json: %w", err)
	}
	prs := make([]domain.PullRequest, 0, len(raw.Data.Search.Nodes))
	for _, node := range raw.Data.Search.Nodes {
		node.ghPRJSON.Author.Login = node.AuthorTyped.Login
		if len(node.Commits.Nodes) > 0 && node.Commits.Nodes[0].Commit.StatusCheckRollup != nil {
			node.ghPRJSON.StatusCheckRollup = node.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts.Nodes
		}
		pr, err := fromGHJSON(node.ghPRJSON)
		if err != nil {
			return nil, 0, err
		}
		pr.Repo = node.Repository.NameWithOwner
		pr.AuthorIsBot = node.AuthorTyped.Typename == "Bot"
		pr.ReviewedByViewer = slices.ContainsFunc(node.Reviews.Nodes, func(r ghReviewJSON) bool { return r.Author.Login == viewer })
		if t, err := time.Parse(time.RFC3339, node.CreatedAt); err == nil {
			pr.CreatedAt = t
		}
		prs = append(prs, pr)
	}
	return prs, raw.Data.Search.IssueCount, nil
}

// OpenInBrowser hands the URL to `gh pr view --web`, which uses the system opener.
func (a *Adapter) OpenInBrowser(ctx context.Context, prURL string) error {
	if _, stderr, err := a.cmd.Run(ctx, "", "pr", "view", prURL, "--web"); err != nil {
		return fmt.Errorf("gh pr view --web: %w (stderr: %s)", err, strings.TrimSpace(string(stderr)))
	}
	return nil
}

// reviewComments is best-effort: a failure here must not hide the rest of the PR.
func (a *Adapter) reviewComments(ctx context.Context, repoPath string, pr domain.PullRequest) (domain.PRComments, error) {
	owner, repo, err := ownerRepoFromURL(pr.URL)
	if err != nil {
		return domain.PRComments{}, err
	}
	stdout, stderr, err := a.cmd.Run(ctx, repoPath,
		"api", "graphql",
		"-f", "query="+reviewThreadsQuery,
		"-f", "owner="+owner,
		"-f", "repo="+repo,
		"-F", fmt.Sprintf("number=%d", pr.Number),
	)
	if err != nil {
		return domain.PRComments{}, fmt.Errorf("gh api graphql review threads: %w (stderr: %s)", err, strings.TrimSpace(string(stderr)))
	}
	return summariseThreads(stdout)
}

func ownerRepoFromURL(prURL string) (string, string, error) {
	u, err := url.Parse(prURL)
	if err != nil {
		return "", "", fmt.Errorf("parse pr url %q: %w", prURL, err)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("pr url %q has no owner/repo", prURL)
	}
	return parts[0], parts[1], nil
}

type ghThreadsJSON struct {
	Data struct {
		Viewer struct {
			Login string `json:"login"`
		} `json:"viewer"`
		Repository struct {
			PullRequest struct {
				ReviewThreads struct {
					Nodes []struct {
						IsResolved bool `json:"isResolved"`
						Comments   struct {
							Nodes []struct {
								Author struct {
									Typename string `json:"__typename"`
									Login    string `json:"login"`
								} `json:"author"`
							} `json:"nodes"`
						} `json:"comments"`
					} `json:"nodes"`
				} `json:"reviewThreads"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
}

func summariseThreads(data []byte) (domain.PRComments, error) {
	var raw ghThreadsJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return domain.PRComments{}, fmt.Errorf("decode gh graphql json: %w", err)
	}
	nodes := raw.Data.Repository.PullRequest.ReviewThreads.Nodes
	threads := make([]domain.PRReviewThread, 0, len(nodes))
	for _, th := range nodes {
		var authors []string
		for _, cm := range th.Comments.Nodes {
			if cm.Author.Typename != "Bot" {
				authors = append(authors, cm.Author.Login)
			}
		}
		threads = append(threads, domain.PRReviewThread{Resolved: th.IsResolved, Authors: authors})
	}
	return domain.SummariseReviewThreads(threads, raw.Data.Viewer.Login), nil
}

type ghPRJSON struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	State       string `json:"state"`
	IsDraft     bool   `json:"isDraft"`
	URL         string `json:"url"`
	HeadRefName string `json:"headRefName"`
	Author      struct {
		Login string `json:"login"`
	} `json:"author"`
	Additions         int           `json:"additions"`
	Deletions         int           `json:"deletions"`
	ChangedFiles      int           `json:"changedFiles"`
	UpdatedAt         string        `json:"updatedAt"`
	StatusCheckRollup []ghCheckJSON `json:"statusCheckRollup"`
}

// ghCheckJSON covers both `gh pr view` rollup entries and GraphQL contexts:
// a CheckRun carries status/conclusion, a StatusContext only state.
type ghCheckJSON struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	State      string `json:"state"`
}

func parseGHJSON(data []byte) (domain.PullRequest, error) {
	var raw ghPRJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return domain.PullRequest{}, fmt.Errorf("decode gh json: %w", err)
	}
	return fromGHJSON(raw)
}

func fromGHJSON(raw ghPRJSON) (domain.PullRequest, error) {
	state, err := mapState(raw.State)
	if err != nil {
		return domain.PullRequest{}, err
	}
	// GitHub models draft as state=OPEN + isDraft=true; collapse to a
	// dedicated PRStateDraft so the rest of the system (view, styling)
	// only deals with a single state value.
	if raw.IsDraft && state == domain.PRStateOpen {
		state = domain.PRStateDraft
	}
	pr, err := domain.NewPullRequest(raw.Number, raw.Title, raw.HeadRefName, state)
	if err != nil {
		return domain.PullRequest{}, err
	}
	pr.URL = raw.URL
	pr.Author = raw.Author.Login
	pr.Stats = domain.PRStats{
		Additions:    raw.Additions,
		Deletions:    raw.Deletions,
		ChangedFiles: raw.ChangedFiles,
	}
	pr.Checks = summariseChecks(raw.StatusCheckRollup)
	if raw.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, raw.UpdatedAt); err == nil {
			pr.UpdatedAt = t
		}
	}
	return pr, nil
}

func mapState(state string) (domain.PRState, error) {
	switch strings.ToUpper(state) {
	case "OPEN":
		return domain.PRStateOpen, nil
	case "CLOSED":
		return domain.PRStateClosed, nil
	case "MERGED":
		return domain.PRStateMerged, nil
	case "DRAFT":
		return domain.PRStateDraft, nil
	default:
		return "", fmt.Errorf("unknown pull request state %q from gh", state)
	}
}

func summariseChecks(checks []ghCheckJSON) domain.PRChecks {
	c := domain.PRChecks{Total: len(checks)}
	for _, ck := range checks {
		concl := strings.ToUpper(ck.Conclusion)
		if concl == "" {
			concl = strings.ToUpper(ck.State)
		}
		switch concl {
		case "SUCCESS":
			c.Passing++
		case "FAILURE", "CANCELLED", "TIMED_OUT", "ERROR":
			c.Failing++
		case "SKIPPED", "NEUTRAL":
			c.Skipped++
		default:
			c.Pending++
		}
	}
	return c
}

type realCommander struct{}

func (realCommander) Run(ctx context.Context, dir string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}
