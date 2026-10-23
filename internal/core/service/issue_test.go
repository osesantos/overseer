package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/dnlopes/overseer/internal/core/domain"
	"github.com/dnlopes/overseer/internal/testutil/mocks"
)

func TestIssueService_ListTriageInbox_MarksSettingsAndFetchesTrackedKeysThatLeftTheQuery(t *testing.T) {
	port := mocks.NewMockIssueTrackerPort(t)
	settings := mocks.NewMockTriageSettingsRepository(t)
	port.EXPECT().ListIssues(mock.Anything, "jql").Return([]domain.Issue{
		{Key: "RDODCF-1", StatusCategory: domain.IssueStatusNew},
		{Key: "RDODCF-2", StatusCategory: domain.IssueStatusNew},
		{Key: "RDINC-3", StatusCategory: domain.IssueStatusInProgress},
	}, nil).Once()
	port.EXPECT().GetIssues(mock.Anything, []string{"RDODCF-9"}).Return([]domain.Issue{
		{Key: "RDODCF-9", StatusCategory: domain.IssueStatusDone},
	}, nil).Once()
	settings.EXPECT().Load(mock.Anything).Return(domain.TriageSettings{Dismissed: []string{"RDODCF-2"}, Handled: []string{"RDINC-3"}}, nil).Once()

	resp, err := NewIssueService(port, settings, testLogger()).ListTriageInbox(context.Background(), ListTriageInboxRequest{
		Query:       "jql",
		TrackedKeys: []string{"RDODCF-1", "RDODCF-9"},
	})

	if err != nil {
		t.Fatalf("ListTriageInbox() error = %v", err)
	}
	got := resp.Inbox.Issues
	if len(got) != 4 {
		t.Fatalf("Issues = %+v, want the 3 listed plus the tracked RDODCF-9", got)
	}
	if got[0].Dismissed || !got[1].Dismissed || !got[2].Handled || got[3].Key != "RDODCF-9" {
		t.Fatalf("Issues = %+v", got)
	}
}

func TestIssueService_ListTriageInbox_NoTrackedKeysMissing_SkipsGetIssues(t *testing.T) {
	port := mocks.NewMockIssueTrackerPort(t)
	settings := mocks.NewMockTriageSettingsRepository(t)
	port.EXPECT().ListIssues(mock.Anything, "jql").Return([]domain.Issue{{Key: "RDODCF-1"}}, nil).Once()
	settings.EXPECT().Load(mock.Anything).Return(domain.TriageSettings{}, nil).Once()

	_, err := NewIssueService(port, settings, testLogger()).ListTriageInbox(context.Background(), ListTriageInboxRequest{
		Query: "jql", TrackedKeys: []string{"RDODCF-1"},
	})

	if err != nil {
		t.Fatalf("ListTriageInbox() error = %v", err)
	}
}

func TestIssueService_ListTriageInbox_TrackerFailure_ReturnsWrappedError(t *testing.T) {
	port := mocks.NewMockIssueTrackerPort(t)
	boom := errors.New("acli down")
	port.EXPECT().ListIssues(mock.Anything, "jql").Return(nil, boom).Once()

	_, err := NewIssueService(port, mocks.NewMockTriageSettingsRepository(t), testLogger()).
		ListTriageInbox(context.Background(), ListTriageInboxRequest{Query: "jql"})

	if !errors.Is(err, boom) {
		t.Fatalf("ListTriageInbox() error = %v, want it to wrap %v", err, boom)
	}
}

func TestIssueService_ResolveAutoTriage(t *testing.T) {
	off := false
	tests := []struct {
		name     string
		settings domain.TriageSettings
		def      bool
		want     bool
	}{
		{name: "no override uses the config default", def: true, want: true},
		{name: "persisted override wins", settings: domain.TriageSettings{AutoTriage: &off}, def: true, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := mocks.NewMockTriageSettingsRepository(t)
			settings.EXPECT().Load(mock.Anything).Return(tt.settings, nil).Once()

			resp, err := NewIssueService(mocks.NewMockIssueTrackerPort(t), settings, testLogger()).
				ResolveAutoTriage(context.Background(), ResolveAutoTriageRequest{Default: tt.def})

			if err != nil || resp.On != tt.want {
				t.Fatalf("ResolveAutoTriage() = %v, %v; want %v", resp.On, err, tt.want)
			}
		})
	}
}

func TestIssueService_SetAutoTriage_Persists(t *testing.T) {
	settings := mocks.NewMockTriageSettingsRepository(t)
	settings.EXPECT().SetAutoTriage(mock.Anything, false).Return(nil).Once()

	if _, err := NewIssueService(mocks.NewMockIssueTrackerPort(t), settings, testLogger()).
		SetAutoTriage(context.Background(), SetAutoTriageRequest{On: false}); err != nil {
		t.Fatalf("SetAutoTriage() error = %v", err)
	}
}

func TestIssueService_Dismiss(t *testing.T) {
	settings := mocks.NewMockTriageSettingsRepository(t)
	settings.EXPECT().Dismiss(mock.Anything, "RDODCF-1").Return(nil).Once()
	svc := NewIssueService(mocks.NewMockIssueTrackerPort(t), settings, testLogger())

	if _, err := svc.Dismiss(context.Background(), DismissIssueRequest{Key: "RDODCF-1"}); err != nil {
		t.Fatalf("Dismiss() error = %v", err)
	}
	if _, err := svc.Dismiss(context.Background(), DismissIssueRequest{Key: " "}); !errors.Is(err, domain.ErrIssueEmptyKey) {
		t.Fatalf("Dismiss(blank) error = %v, want ErrIssueEmptyKey", err)
	}
}

func TestIssueService_OpenInBrowser(t *testing.T) {
	port := mocks.NewMockIssueTrackerPort(t)
	port.EXPECT().OpenInBrowser(mock.Anything, "RDINC-3").Return(nil).Once()

	if _, err := NewIssueService(port, mocks.NewMockTriageSettingsRepository(t), testLogger()).
		OpenInBrowser(context.Background(), OpenIssueInBrowserRequest{Key: "RDINC-3"}); err != nil {
		t.Fatalf("OpenInBrowser() error = %v", err)
	}
}
