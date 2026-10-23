// Package jira provides an IssueTrackerPort implementation backed by the
// Atlassian CLI (acli).
package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dnlopes/overseer/internal/core/domain"
)

var _ domain.IssueTrackerPort = (*Adapter)(nil)

const (
	browseURL = "https://outsystemsrd.atlassian.net/browse/"
	// acli search rejects `created`, so search only lists keys and each issue
	// comes from a view, which accepts every field. Search asks for summary
	// because `--fields key` alone answers a list of nulls.
	searchFields = "summary"
	viewFields   = "summary,status,assignee,priority,created"
	// ponytail: one page of 100; the triage inbox was 14 open tickets when this was written.
	searchLimit = 100
	// viewConcurrency bounds the parallel `acli workitem view` calls (~0.8s each).
	viewConcurrency = 4
	createdLayout   = "2006-01-02T15:04:05.000-0700"
)

type Commander interface {
	Run(ctx context.Context, args ...string) (stdout []byte, stderr []byte, err error)
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

func (a *Adapter) ListIssues(ctx context.Context, query string) ([]domain.Issue, error) {
	stdout, stderr, err := a.cmd.Run(ctx, "jira", "workitem", "search",
		"--jql", query, "--fields", searchFields, "--limit", strconv.Itoa(searchLimit), "--json")
	if err != nil {
		return nil, fmt.Errorf("acli jira workitem search: %w (stderr: %s)", err, strings.TrimSpace(string(stderr)))
	}
	var hits []*struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(stdout, &hits); err != nil {
		return nil, fmt.Errorf("decode acli search json: %w", err)
	}
	if len(hits) == searchLimit {
		a.logger.WarnContext(ctx, "triage inbox truncated by the search page size", "limit", searchLimit)
	}
	keys := make([]string, 0, len(hits))
	for _, h := range hits {
		if h == nil || h.Key == "" {
			return nil, fmt.Errorf("acli search answered an issue without a key: %w", domain.ErrIssueEmptyKey)
		}
		keys = append(keys, h.Key)
	}
	return a.GetIssues(ctx, keys)
}

// GetIssues never fails on one issue: a ticket that cannot be viewed comes back
// as its key alone, which is never eligible for auto-triage and never "done".
func (a *Adapter) GetIssues(ctx context.Context, keys []string) ([]domain.Issue, error) {
	issues := make([]domain.Issue, len(keys))
	sem := make(chan struct{}, viewConcurrency)
	var wg sync.WaitGroup
	for i, key := range keys {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			issue, err := a.view(ctx, key)
			if err != nil {
				a.logger.WarnContext(ctx, "could not view issue", "key", key, "error", err)
				issue = stubIssue(key)
			}
			issues[i] = issue
		}()
	}
	wg.Wait()
	return issues, ctx.Err()
}

func (a *Adapter) view(ctx context.Context, key string) (domain.Issue, error) {
	stdout, stderr, err := a.cmd.Run(ctx, "jira", "workitem", "view", key, "--fields", viewFields, "--json")
	if err != nil {
		return domain.Issue{}, fmt.Errorf("acli jira workitem view: %w (stderr: %s)", err, strings.TrimSpace(string(stderr)))
	}
	return parseView(stdout)
}

type acliIssueJSON struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
		Status  struct {
			Name           string `json:"name"`
			StatusCategory struct {
				Key string `json:"key"`
			} `json:"statusCategory"`
		} `json:"status"`
		Assignee *struct {
			DisplayName string `json:"displayName"`
		} `json:"assignee"`
		Priority *struct {
			Name string `json:"name"`
		} `json:"priority"`
		Created string `json:"created"`
	} `json:"fields"`
}

func parseView(data []byte) (domain.Issue, error) {
	var raw acliIssueJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return domain.Issue{}, fmt.Errorf("decode acli view json: %w", err)
	}
	issue := stubIssue(raw.Key)
	issue.Summary = raw.Fields.Summary
	issue.Status = raw.Fields.Status.Name
	issue.StatusCategory = domain.IssueStatusCategory(raw.Fields.Status.StatusCategory.Key)
	if raw.Fields.Assignee != nil {
		issue.Assignee = raw.Fields.Assignee.DisplayName
	}
	if raw.Fields.Priority != nil {
		issue.Priority = raw.Fields.Priority.Name
	}
	if t, err := time.Parse(createdLayout, raw.Fields.Created); err == nil {
		issue.CreatedAt = t
	}
	return issue, nil
}

func stubIssue(key string) domain.Issue {
	return domain.Issue{Key: key, Project: domain.IssueProjectKey(key), URL: browseURL + key}
}

// OpenInBrowser hands the key to `acli jira workitem view --web`, which uses the system opener.
func (a *Adapter) OpenInBrowser(ctx context.Context, key string) error {
	if _, stderr, err := a.cmd.Run(ctx, "jira", "workitem", "view", key, "--web"); err != nil {
		return fmt.Errorf("acli jira workitem view --web: %w (stderr: %s)", err, strings.TrimSpace(string(stderr)))
	}
	return nil
}

type realCommander struct{}

func (realCommander) Run(ctx context.Context, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, "acli", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}
