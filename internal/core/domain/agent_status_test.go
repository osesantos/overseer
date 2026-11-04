package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestWithReviewStates(t *testing.T) {
	ref := PullRequestRef{Repo: "OutSystems/repo", Number: 7}
	review := Session{ID: uuid.New(), ReviewPR: ref}
	ordinary := Session{ID: uuid.New()}
	pendingInbox := map[uuid.UUID]PullRequest{
		review.ID:   {Repo: ref.Repo, Number: ref.Number, ViewerReviewPending: true},
		ordinary.ID: {Number: 9, ViewerReviewPending: true},
	}
	submittedInbox := map[uuid.UUID]PullRequest{review.ID: {Repo: ref.Repo, Number: ref.Number}}
	approved := map[uuid.UUID]PullRequest{review.ID: {Repo: ref.Repo, Number: ref.Number, ViewerReview: PRReviewStateApproved}}
	reRequested := map[uuid.UUID]PullRequest{review.ID: {Repo: ref.Repo, Number: ref.Number, ViewerReview: PRReviewStateApproved, ViewerReviewRequested: true}}
	commented := map[uuid.UUID]PullRequest{review.ID: {Repo: ref.Repo, Number: ref.Number, ViewerReview: PRReviewStateCommented}}

	tests := []struct {
		name    string
		sess    Session
		status  *AgentStatusKind
		inbox   map[uuid.UUID]PullRequest
		want    AgentStatusKind
		present bool
	}{
		{name: "idle review with pending review", sess: review, status: kindPtr(AgentStatusIdle), inbox: pendingInbox, want: AgentStatusReviewPending, present: true},
		{name: "dead review with pending review", sess: review, status: kindPtr(AgentStatusDead), inbox: pendingInbox, want: AgentStatusReviewPending, present: true},
		{name: "unknown review with pending review", sess: review, status: kindPtr(AgentStatusUnknown), inbox: pendingInbox, want: AgentStatusReviewPending, present: true},
		{name: "no status yet", sess: review, inbox: pendingInbox, want: AgentStatusReviewPending, present: true},
		{name: "running agent wins", sess: review, status: kindPtr(AgentStatusRunning), inbox: pendingInbox, want: AgentStatusRunning, present: true},
		{name: "waiting agent wins", sess: review, status: kindPtr(AgentStatusWaiting), inbox: pendingInbox, want: AgentStatusWaiting, present: true},
		{name: "review already submitted", sess: review, status: kindPtr(AgentStatusIdle), inbox: submittedInbox, want: AgentStatusIdle, present: true},
		{name: "idle review the viewer approved", sess: review, status: kindPtr(AgentStatusIdle), inbox: approved, want: AgentStatusReviewApproved, present: true},
		{name: "running agent beats an approval", sess: review, status: kindPtr(AgentStatusRunning), inbox: approved, want: AgentStatusRunning, present: true},
		{name: "re-requested after approving", sess: review, status: kindPtr(AgentStatusIdle), inbox: reRequested, want: AgentStatusIdle, present: true},
		{name: "commented only", sess: review, status: kindPtr(AgentStatusIdle), inbox: commented, want: AgentStatusIdle, present: true},
		{name: "PR status not fetched yet", sess: review, status: kindPtr(AgentStatusIdle), want: AgentStatusIdle, present: true},
		{name: "ordinary session is untouched", sess: ordinary, status: kindPtr(AgentStatusIdle), inbox: pendingInbox, want: AgentStatusIdle, present: true},
		{name: "ordinary session gains no entry", sess: ordinary, inbox: pendingInbox},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statuses := map[uuid.UUID]AgentStatus{}
			if tt.status != nil {
				statuses[tt.sess.ID] = AgentStatus{Kind: *tt.status}
			}
			before := len(statuses)

			got, ok := WithReviewStates(statuses, []Session{tt.sess}, tt.inbox)[tt.sess.ID]

			if ok != tt.present || got.Kind != tt.want {
				t.Fatalf("status = %q (present %v), want %q (present %v)", got.Kind, ok, tt.want, tt.present)
			}
			if len(statuses) != before || (tt.status != nil && statuses[tt.sess.ID].Kind != *tt.status) {
				t.Fatalf("input map was mutated: %v", statuses)
			}
		})
	}
}

func kindPtr(k AgentStatusKind) *AgentStatusKind { return &k }
