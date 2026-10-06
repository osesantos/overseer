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

type Commander interface {
	Run(ctx context.Context, dir string, args ...string) (stdout []byte, stderr []byte, err error)
}

type Adapter struct {
	cmd    Commander
	logger *slog.Logger
}

func New(logger *slog.Logger) *Adapter {
	return &Adapter{cmd: realCommander{}, logger: logger}
}

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
