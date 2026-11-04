// Package github provides a PullRequestPort implementation backed by the
// gh CLI (https://cli.github.com).
package github

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dnlopes/overseer/internal/core/domain"
)

var _ domain.PullRequestPort = (*Adapter)(nil)

const prJSONFields = "number,title,state,isDraft,url,headRefName,author,additions,deletions,changedFiles,statusCheckRollup,updatedAt,body,baseRefName,labels"

// prDetailsJSONFields adds what only the PR page shows; the per-minute status
// poll gets the review decision and merge state from reviewStatusQuery.
const prDetailsJSONFields = prJSONFields + ",files,reviewDecision,mergeStateStatus,latestReviews,reviewRequests"

// reviewStatusQuery is the per-minute poll's GraphQL half: thread counts, merge
// state and where the viewer stands. Pending reviews are only visible to their
// author, so any PENDING review counted here is the viewer's. mergeStateStatus
// answers UNKNOWN until GitHub has computed it, then is cached until the PR moves.
// ponytail: first 100 threads / 50 comments each; paginate if a PR ever outgrows that.
const reviewStatusQuery = `query($owner: String!, $repo: String!, $number: Int!) {
  viewer { login }
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      reviewDecision
      mergeStateStatus
      viewerLatestReview { state }
      viewerLatestReviewRequest { id }
      pendingReviews: reviews(first: 1, states: PENDING) { totalCount }
      reviewThreads(first: 100) {
        nodes { isResolved comments(first: 50) { nodes { author { __typename login } } } }
      }
    }
  }
}`

// pullRequestDetailsQuery feeds the PR page: the conversation and every thread
// with its bodies. It is separate from reviewStatusQuery so the per-minute
// status poll never pays for bodies. Same first-100 caps as the lean query.
const pullRequestDetailsQuery = `query($owner: String!, $repo: String!, $number: Int!) {
  viewer { login }
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      comments(first: 100) { nodes { author { __typename login } body createdAt } }
      reviews(first: 100) { nodes { author { __typename login } body state submittedAt } }
      reviewThreads(first: 100) {
        nodes { isResolved isOutdated path line comments(first: 50) { nodes { author { __typename login } body createdAt diffHunk } } }
      }
    }
  }
}`

// ponytail: first 100 PRs and 50 reviews each; the inbox was 69 when this was written.
// The query stays lean because GitHub cuts GraphQL off at about 10s and answers 504:
// no diff stats (the page never shows them) and checks as the rollup's single state.
const requestedReviewsQuery = `query($q: String!) {
  search(type: ISSUE, first: 100, query: $q) {
    issueCount
    nodes { ... on PullRequest {
      number title url isDraft state createdAt updatedAt headRefName
      author { login __typename }
      repository { nameWithOwner }
      reviews(first: 50) { nodes { author { login } state } }
      commits(last: 1) { nodes { commit { statusCheckRollup { state } } } }
    } }
  }
}`

type Commander interface {
	Run(ctx context.Context, dir string, args ...string) (stdout []byte, stderr []byte, err error)
}

// searchRetries and searchRetryDelay cover GitHub search intermittently
// answering an empty page for a query that has results (observed during a
// "degraded performance" incident: 59, 0, 59 for the same query), and
// GitHub-side 5xx failures such as a gateway timeout.
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
	stdout, err = a.queryPullRequest(ctx, repoPath, pr, reviewStatusQuery)
	if err == nil {
		pr, err = withReviewStatus(stdout, pr)
	}
	if err != nil {
		a.logger.WarnContext(ctx, "fetch pr review status", "pr", pr.Number, "err", err)
	}
	return pr, nil
}

// ListRequestedReviews resolves the viewer first because GraphQL search does not
// expand "@me"; "review-requested:<login>" already includes team requests.
// GitHub drops the request once the viewer submits a review, so the PRs they
// already reviewed come from a second search (search has no OR across qualifiers).
func (a *Adapter) ListRequestedReviews(ctx context.Context) (domain.ReviewInbox, error) {
	stdout, stderr, err := a.cmd.Run(ctx, "", "api", "user", "--jq", ".login")
	if err != nil {
		return domain.ReviewInbox{}, fmt.Errorf("gh api user: %w (stderr: %s)", err, strings.TrimSpace(string(stderr)))
	}
	viewer := strings.TrimSpace(string(stdout))

	requested, err := a.searchPullRequests(ctx, "is:pr is:open review-requested:"+viewer, viewer, true)
	if err != nil {
		return domain.ReviewInbox{}, fmt.Errorf("requested reviews: %w", err)
	}
	reviewed, err := a.searchPullRequests(ctx, "is:pr is:open reviewed-by:"+viewer+" -review-requested:"+viewer+" -author:"+viewer, viewer, false)
	if err != nil {
		return domain.ReviewInbox{}, fmt.Errorf("reviewed pull requests: %w", err)
	}
	return domain.ReviewInbox{Viewer: viewer, PullRequests: append(requested, reviewed...)}, nil
}

// searchPullRequests retries a GitHub 5xx, and an empty page when retryEmpty is
// set, since search intermittently answers one for a query that has results.
func (a *Adapter) searchPullRequests(ctx context.Context, q, viewer string, retryEmpty bool) ([]domain.PullRequest, error) {
	var prs []domain.PullRequest
	for attempt := 1; attempt <= searchRetries; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(a.retryDelay):
			}
		}
		stdout, stderr, err := a.cmd.Run(ctx, "",
			"api", "graphql",
			"-f", "query="+requestedReviewsQuery,
			"-f", "q="+q,
		)
		if err != nil {
			if attempt < searchRetries && isServerError(stderr) {
				a.logger.WarnContext(ctx, "review inbox search failed on GitHub's side, retrying", "q", q, "attempt", attempt, "stderr", strings.TrimSpace(string(stderr)))
				continue
			}
			return nil, fmt.Errorf("gh api graphql search: %w (stderr: %s)", err, strings.TrimSpace(string(stderr)))
		}
		var total int
		prs, total, err = parseRequestedReviews(stdout, viewer)
		if err != nil {
			return nil, err
		}
		if total > len(prs) {
			a.logger.WarnContext(ctx, "review inbox truncated by the search page size", "q", q, "total", total, "fetched", len(prs))
		}
		if len(prs) > 0 || !retryEmpty || attempt == searchRetries {
			break
		}
		a.logger.WarnContext(ctx, "review inbox came back empty, retrying", "attempt", attempt)
	}
	return prs, nil
}

// isServerError reports whether gh failed on a GitHub 5xx, which is worth retrying.
func isServerError(stderr []byte) bool {
	return bytes.Contains(stderr, []byte("HTTP 5"))
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
								State string `json:"state"`
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
	State string `json:"state"`
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
			// ponytail: the rollup state stands in as one check, so inbox counts are 0 or 1; only the glyph reads them.
			node.ghPRJSON.StatusCheckRollup = []ghCheckJSON{{State: node.Commits.Nodes[0].Commit.StatusCheckRollup.State}}
		}
		pr, err := fromGHJSON(node.ghPRJSON)
		if err != nil {
			return nil, 0, err
		}
		pr.Repo = node.Repository.NameWithOwner
		pr.AuthorIsBot = node.AuthorTyped.Typename == "Bot"
		pr.ReviewedByViewer = slices.ContainsFunc(node.Reviews.Nodes, func(r ghReviewJSON) bool { return r.Author.Login == viewer })
		pr.ViewerReviewPending = slices.ContainsFunc(node.Reviews.Nodes, func(r ghReviewJSON) bool {
			return r.Author.Login == viewer && r.State == "PENDING"
		})
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

// queryPullRequest runs one of the pull-request GraphQL queries, with owner and
// repo taken from the PR's URL.
func (a *Adapter) queryPullRequest(ctx context.Context, repoPath string, pr domain.PullRequest, query string) ([]byte, error) {
	owner, repo, err := ownerRepoFromURL(pr.URL)
	if err != nil {
		return nil, err
	}
	stdout, stderr, err := a.cmd.Run(ctx, repoPath,
		"api", "graphql",
		"-f", "query="+query,
		"-f", "owner="+owner,
		"-f", "repo="+repo,
		"-F", fmt.Sprintf("number=%d", pr.Number),
	)
	if err != nil {
		return nil, fmt.Errorf("gh api graphql pull request: %w (stderr: %s)", err, strings.TrimSpace(string(stderr)))
	}
	return stdout, nil
}

// GetDetails runs three gh commands in repoPath: the PR with its files, the
// unified diff, and the conversation query. The conversation is best-effort,
// like the per-minute review status: the page is still useful without it.
func (a *Adapter) GetDetails(ctx context.Context, repoPath string, number int) (domain.PullRequestDetails, error) {
	sel := strconv.Itoa(number)
	stdout, stderr, err := a.cmd.Run(ctx, repoPath, "pr", "view", sel, "--json", prDetailsJSONFields)
	if err != nil {
		return domain.PullRequestDetails{}, fmt.Errorf("gh pr view %s: %w (stderr: %s)", sel, err, strings.TrimSpace(string(stderr)))
	}
	var raw ghPRJSON
	if err := json.Unmarshal(stdout, &raw); err != nil {
		return domain.PullRequestDetails{}, fmt.Errorf("decode gh json: %w", err)
	}
	pr, err := fromGHJSON(raw)
	if err != nil {
		return domain.PullRequestDetails{}, err
	}
	files := make([]domain.PRFile, 0, len(raw.Files))
	for _, f := range raw.Files {
		files = append(files, domain.PRFile{Path: f.Path, Additions: f.Additions, Deletions: f.Deletions, ChangeType: f.ChangeType})
	}

	diff, stderr, err := a.cmd.Run(ctx, repoPath, "pr", "diff", sel)
	if err != nil {
		return domain.PullRequestDetails{}, fmt.Errorf("gh pr diff %s: %w (stderr: %s)", sel, err, strings.TrimSpace(string(stderr)))
	}
	details := domain.PullRequestDetails{PullRequest: pr, Files: files, Diff: string(diff)}

	data, err := a.queryPullRequest(ctx, repoPath, pr, pullRequestDetailsQuery)
	if err != nil {
		a.logger.WarnContext(ctx, "fetch pr conversation", "pr", number, "err", err)
		return details, nil
	}
	var conv ghThreadsJSON
	if err := json.Unmarshal(data, &conv); err != nil {
		a.logger.WarnContext(ctx, "decode pr conversation", "pr", number, "err", err)
		return details, nil
	}
	details.Conversation = parseConversation(conv.Data.Repository.PullRequest)
	details.Threads = parseThreads(conv.Data.Repository.PullRequest.ReviewThreads.Nodes)
	return details, nil
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

// ghCommentJSON decodes an issue comment, a review (State, SubmittedAt) or a
// thread comment (DiffHunk); the fields a query did not ask for stay empty.
type ghCommentJSON struct {
	Author struct {
		Typename string `json:"__typename"`
		Login    string `json:"login"`
	} `json:"author"`
	Body        string `json:"body"`
	State       string `json:"state"`
	CreatedAt   string `json:"createdAt"`
	SubmittedAt string `json:"submittedAt"`
	DiffHunk    string `json:"diffHunk"`
}

type ghThreadJSON struct {
	IsResolved bool   `json:"isResolved"`
	IsOutdated bool   `json:"isOutdated"`
	Path       string `json:"path"`
	Line       int    `json:"line"`
	Comments   struct {
		Nodes []ghCommentJSON `json:"nodes"`
	} `json:"comments"`
}

type ghPullRequestJSON struct {
	ReviewDecision     string `json:"reviewDecision"`
	MergeStateStatus   string `json:"mergeStateStatus"`
	ViewerLatestReview *struct {
		State string `json:"state"`
	} `json:"viewerLatestReview"`
	ViewerLatestReviewRequest *struct{} `json:"viewerLatestReviewRequest"`
	PendingReviews            struct {
		TotalCount int `json:"totalCount"`
	} `json:"pendingReviews"`
	Comments struct {
		Nodes []ghCommentJSON `json:"nodes"`
	} `json:"comments"`
	Reviews struct {
		Nodes []ghCommentJSON `json:"nodes"`
	} `json:"reviews"`
	ReviewThreads struct {
		Nodes []ghThreadJSON `json:"nodes"`
	} `json:"reviewThreads"`
}

// ghThreadsJSON decodes both reviewStatusQuery and pullRequestDetailsQuery.
type ghThreadsJSON struct {
	Data struct {
		Viewer struct {
			Login string `json:"login"`
		} `json:"viewer"`
		Repository struct {
			PullRequest ghPullRequestJSON `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
}

func (c ghCommentJSON) toDomain() domain.PRComment {
	at := c.CreatedAt
	if at == "" {
		at = c.SubmittedAt
	}
	t, _ := time.Parse(time.RFC3339, at)
	return domain.PRComment{
		Author:      c.Author.Login,
		AuthorIsBot: c.Author.Typename == "Bot",
		Body:        c.Body,
		CreatedAt:   t,
		ReviewState: c.State,
	}
}

func parseThreads(nodes []ghThreadJSON) []domain.PRReviewThread {
	threads := make([]domain.PRReviewThread, 0, len(nodes))
	for _, th := range nodes {
		thread := domain.PRReviewThread{Resolved: th.IsResolved, Outdated: th.IsOutdated, Path: th.Path, Line: th.Line}
		for _, cm := range th.Comments.Nodes {
			c := cm.toDomain()
			thread.Comments = append(thread.Comments, c)
			if !c.AuthorIsBot {
				thread.Authors = append(thread.Authors, c.Author)
			}
			if thread.DiffHunk == "" {
				thread.DiffHunk = cm.DiffHunk
			}
		}
		threads = append(threads, thread)
	}
	return threads
}

// parseConversation merges issue comments and the reviews that carry a body,
// oldest first.
func parseConversation(pr ghPullRequestJSON) []domain.PRComment {
	out := make([]domain.PRComment, 0, len(pr.Comments.Nodes)+len(pr.Reviews.Nodes))
	for _, c := range pr.Comments.Nodes {
		out = append(out, c.toDomain())
	}
	for _, r := range pr.Reviews.Nodes {
		if strings.TrimSpace(r.Body) != "" {
			out = append(out, r.toDomain())
		}
	}
	slices.SortStableFunc(out, func(a, b domain.PRComment) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return out
}

// withReviewStatus returns pr with the reviewStatusQuery answer laid over it.
func withReviewStatus(data []byte, pr domain.PullRequest) (domain.PullRequest, error) {
	var raw ghThreadsJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return pr, fmt.Errorf("decode gh graphql json: %w", err)
	}
	gpr := raw.Data.Repository.PullRequest
	pr.Comments = domain.SummariseReviewThreads(parseThreads(gpr.ReviewThreads.Nodes), raw.Data.Viewer.Login)
	pr.ReviewDecision = domain.PRReviewDecision(gpr.ReviewDecision)
	pr.MergeState = domain.PRMergeState(gpr.MergeStateStatus)
	pr.ViewerReviewPending = gpr.PendingReviews.TotalCount > 0
	pr.ViewerReviewRequested = gpr.ViewerLatestReviewRequest != nil
	if gpr.ViewerLatestReview != nil {
		pr.ViewerReview = domain.PRReviewState(gpr.ViewerLatestReview.State)
	}
	return pr, nil
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
	Body              string        `json:"body"`
	BaseRefName       string        `json:"baseRefName"`
	Labels            []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Files            []ghFileJSON `json:"files"`
	ReviewDecision   string       `json:"reviewDecision"`
	MergeStateStatus string       `json:"mergeStateStatus"`
	LatestReviews    []struct {
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
		State string `json:"state"`
	} `json:"latestReviews"`
	// ReviewRequests mixes users (login) and teams (slug, name).
	ReviewRequests []struct {
		Login string `json:"login"`
		Slug  string `json:"slug"`
		Name  string `json:"name"`
	} `json:"reviewRequests"`
}

type ghFileJSON struct {
	Path       string `json:"path"`
	Additions  int    `json:"additions"`
	Deletions  int    `json:"deletions"`
	ChangeType string `json:"changeType"`
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
	pr.Body = raw.Body
	pr.BaseBranch = raw.BaseRefName
	for _, l := range raw.Labels {
		pr.Labels = append(pr.Labels, l.Name)
	}
	pr.Stats = domain.PRStats{
		Additions:    raw.Additions,
		Deletions:    raw.Deletions,
		ChangedFiles: raw.ChangedFiles,
	}
	pr.Checks = summariseChecks(raw.StatusCheckRollup)
	pr.ReviewDecision = domain.PRReviewDecision(raw.ReviewDecision)
	pr.MergeState = domain.PRMergeState(raw.MergeStateStatus)
	for _, r := range raw.LatestReviews {
		pr.Reviewers = append(pr.Reviewers, domain.PRReviewer{Login: r.Author.Login, State: domain.PRReviewState(r.State)})
	}
	for _, r := range raw.ReviewRequests {
		pr.Reviewers = append(pr.Reviewers, domain.PRReviewer{Login: cmp.Or(r.Login, r.Slug, r.Name), State: domain.PRReviewStateRequested})
	}
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
