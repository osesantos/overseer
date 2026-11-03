package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestWithPendingReviews(t *testing.T) {
	ref := PullRequestRef{Repo: "OutSystems/repo", Number: 7}
	pendingInbox := ReviewInbox{PullRequests: []PullRequest{{Repo: ref.Repo, Number: ref.Number, ViewerReviewPending: true}}}
	submittedInbox := ReviewInbox{PullRequests: []PullRequest{{Repo: ref.Repo, Number: ref.Number}}}
	review := Session{ID: uuid.New(), ReviewPR: ref}
	ordinary := Session{ID: uuid.New()}

	tests := []struct {
		name    string
		sess    Session
		status  *AgentStatusKind
		inbox   ReviewInbox
		want    AgentStatusKind
		present bool
	}{
		{name: "idle review with pending review", sess: review, status: kindPtr(AgentStatusIdle), inbox: pendingInbox, want: AgentStatusReviewPending, present: true},
		{name: "dead review with pending review", sess: review, status: kindPtr(AgentStatusDead), inbox: pendingInbox, want: AgentStatusReviewPending, present: true},
		{name: "unknown review with pending review", sess: review, status: kindPtr(AgentStatusUnknown), inbox: pendingInbox, want: AgentStatusReviewPending, present: true},
		{name: "no status yet", sess: review, inbox: pendingInbox, want: AgentStatusReviewPending, present: true},
		{name: "running agent wins", sess: review, status: kindPtr(AgentStatusRunning), inbox: pendingInbox, want: AgentStatusRunning, present: true},
		{name: "waiting agent wins", sess: review, status: kindPtr(AgentStatusWaiting), inbox: pendingInbox, want: AgentStatusWaiting, present: true},
		{name: "pending review beats a prose question", sess: review, status: kindPtr(AgentStatusAsking), inbox: pendingInbox, want: AgentStatusReviewPending, present: true},
		{name: "review already submitted", sess: review, status: kindPtr(AgentStatusIdle), inbox: submittedInbox, want: AgentStatusIdle, present: true},
		{name: "PR left the inbox", sess: review, status: kindPtr(AgentStatusIdle), want: AgentStatusIdle, present: true},
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

			got, ok := WithPendingReviews(statuses, []Session{tt.sess}, tt.inbox)[tt.sess.ID]

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
