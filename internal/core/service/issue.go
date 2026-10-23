package service

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/dnlopes/overseer/internal/core/domain"
)

type IssueService struct {
	port     domain.IssueTrackerPort
	settings domain.TriageSettingsRepository
	logger   *slog.Logger
}

func NewIssueService(port domain.IssueTrackerPort, settings domain.TriageSettingsRepository, logger *slog.Logger) *IssueService {
	return &IssueService{port: port, settings: settings, logger: logger}
}

type ListTriageInboxRequest struct {
	Query string
	// TrackedKeys are the issues triage sessions exist for; the ones the query no
	// longer returns are fetched by key so a finished triage can be noticed.
	TrackedKeys []string
}

type ListTriageInboxResponse struct {
	Inbox domain.TriageInbox
}

// ListTriageInbox is the query's issues plus the tracked ones it left out, with
// the operator's dismissed and handled issues marked as such.
func (s *IssueService) ListTriageInbox(ctx context.Context, req ListTriageInboxRequest) (ListTriageInboxResponse, error) {
	issues, err := s.port.ListIssues(ctx, req.Query)
	if err != nil {
		return ListTriageInboxResponse{}, fmt.Errorf("list triage issues: %w", err)
	}
	listed := func(key string) bool {
		return slices.ContainsFunc(issues, func(i domain.Issue) bool { return i.Key == key })
	}
	if missing := slices.DeleteFunc(slices.Clone(req.TrackedKeys), listed); len(missing) > 0 {
		tracked, err := s.port.GetIssues(ctx, missing)
		if err != nil {
			return ListTriageInboxResponse{}, fmt.Errorf("fetch tracked issues: %w", err)
		}
		issues = append(issues, tracked...)
	}
	settings, err := s.settings.Load(ctx)
	if err != nil {
		return ListTriageInboxResponse{}, fmt.Errorf("load triage settings: %w", err)
	}
	for i := range issues {
		issues[i].Dismissed = slices.Contains(settings.Dismissed, issues[i].Key)
		issues[i].Handled = slices.Contains(settings.Handled, issues[i].Key)
	}
	s.logger.InfoContext(ctx, "triage inbox fetched", "count", len(issues),
		"dismissed", len(settings.Dismissed), "handled", len(settings.Handled))
	return ListTriageInboxResponse{Inbox: domain.TriageInbox{Issues: issues}}, nil
}

type ResolveAutoTriageRequest struct {
	// Default is the config value, used until the operator toggles it.
	Default bool
}

type ResolveAutoTriageResponse struct {
	On bool
}

// ResolveAutoTriage returns the persisted toggle when the operator set one,
// otherwise the config default.
func (s *IssueService) ResolveAutoTriage(ctx context.Context, req ResolveAutoTriageRequest) (ResolveAutoTriageResponse, error) {
	settings, err := s.settings.Load(ctx)
	if err != nil {
		return ResolveAutoTriageResponse{}, fmt.Errorf("load triage settings: %w", err)
	}
	if settings.AutoTriage != nil {
		return ResolveAutoTriageResponse{On: *settings.AutoTriage}, nil
	}
	return ResolveAutoTriageResponse{On: req.Default}, nil
}

type SetAutoTriageRequest struct {
	On bool
}

type SetAutoTriageResponse struct{}

// SetAutoTriage persists the toggle so it survives restarts.
func (s *IssueService) SetAutoTriage(ctx context.Context, req SetAutoTriageRequest) (SetAutoTriageResponse, error) {
	if err := s.settings.SetAutoTriage(ctx, req.On); err != nil {
		return SetAutoTriageResponse{}, fmt.Errorf("persist auto-triage toggle: %w", err)
	}
	s.logger.InfoContext(ctx, "auto-triage toggled", "on", req.On)
	return SetAutoTriageResponse{}, nil
}

type DismissIssueRequest struct {
	Key string
}

type DismissIssueResponse struct{}

// Dismiss hides an issue from the triage inbox for good.
func (s *IssueService) Dismiss(ctx context.Context, req DismissIssueRequest) (DismissIssueResponse, error) {
	key := strings.TrimSpace(req.Key)
	if key == "" {
		return DismissIssueResponse{}, domain.ErrIssueEmptyKey
	}
	if err := s.settings.Dismiss(ctx, key); err != nil {
		return DismissIssueResponse{}, fmt.Errorf("dismiss issue %s: %w", key, err)
	}
	s.logger.InfoContext(ctx, "issue dismissed", "key", key)
	return DismissIssueResponse{}, nil
}

type OpenIssueInBrowserRequest struct {
	Key string
}

type OpenIssueInBrowserResponse struct{}

func (s *IssueService) OpenInBrowser(ctx context.Context, req OpenIssueInBrowserRequest) (OpenIssueInBrowserResponse, error) {
	if err := s.port.OpenInBrowser(ctx, req.Key); err != nil {
		return OpenIssueInBrowserResponse{}, fmt.Errorf("open issue in browser: %w", err)
	}
	return OpenIssueInBrowserResponse{}, nil
}
