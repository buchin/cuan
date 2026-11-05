package server

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
)

func TestGitHubConnectionObservationRefreshesReview(t *testing.T) { githubObservationContract(t, nil) }
func TestGitHubConnectionObservationRefreshesReviewPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) { githubObservationContract(t, database) })
}

func githubObservationContract(t *testing.T, database *sql.DB) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	ctx := context.Background()
	var s *Server
	var origin db.InboxRequest
	var d *task.TaskDetailResponse
	if database == nil {
		s, origin, d = phaseLineageSetup(t)
	} else {
		s, origin, d = phaseLineageSetup(t, database)
	}
	s.inboxStopping.Store(true)
	d.Status = task.TaskStatusBlocked
	d.Instructions = "Cuan managed delegation v1.\nInspect repository setup using the registered project connection; do not deploy."
	if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
		t.Fatal(err)
	}
	assessment := inboxReviewEvaluation{Decision: "blocked", Finding: "GitHub is not connected", Explanation: "GitHub access is required", RemainingWork: "external"}
	if _, err := s.saveReviewAssessment(ctx, d, origin, assessment); err != nil {
		t.Fatal(err)
	}
	before := inboxReviewEvidenceDigest(d)
	s.observeInboxGitHubConnection(ctx, d, origin)
	_, observed := githubObservation(d)
	if observed == nil || observed.State != "disconnected" || inboxReviewEvidenceDigest(d) == before {
		t.Fatal("saved prerequisite was not observed")
	}
	count := len(d.Activities)
	s.observeInboxGitHubConnection(ctx, d, origin)
	if len(d.Activities) != count {
		t.Fatal("unchanged connection created more evidence")
	}
	path, err := githubConnectionPath(origin.OwnerID)
	if err != nil {
		t.Fatal(err)
	}
	if err := git.SaveConnection(path, s.cfg.AuthSecret, "registered-provider-fixture", git.Account{ID: 1, Login: "registered-owner"}); err != nil {
		t.Fatal(err)
	}
	before = inboxReviewEvidenceDigest(d)
	s.observeInboxGitHubConnection(ctx, d, origin)
	_, observed = githubObservation(d)
	if observed == nil || observed.State != "connected" || observed.Account != "registered-owner" || observed.RepositoryAccessVerified || inboxReviewEvidenceDigest(d) == before {
		t.Fatal("changed connection did not refresh exact evidence")
	}
	if data := s.inboxReviewData(ctx, d); strings.Contains(data, "registered-provider-fixture") || !strings.Contains(data, `"state":"connected"`) || !strings.Contains(data, `"repository_access_verified":false`) {
		t.Fatal("review lost honest connection context or exposed credentials")
	}
	// The observation changes review evidence, never implementation or execution.
	for _, change := range []string{"owner", "cancel", "scope"} {
		copy := *d
		request := origin
		switch change {
		case "owner":
			request.OwnerID = "foreign"
		case "cancel":
			attempt := *d.LatestAttempt
			attempt.CancelRequested = true
			copy.LatestAttempt = &attempt
		case "scope":
			copy.Instructions += "Changed scope"
		}
		before := len(copy.Activities)
		s.observeInboxGitHubConnection(ctx, &copy, request)
		if len(copy.Activities) != before {
			t.Fatal("unauthorized observation recorded", change)
		}
	}
	after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil || after.Status != task.TaskStatusBlocked || len(after.Attempts) != len(d.Attempts) {
		t.Fatal("observation changed task state or attempts", err)
	}
	// A new connected result gets one durable, tool-free review. The approval and
	// task remain the same; execution still needs ordinary correction admission.
	s.queueInboxTaskReview(ctx, after)
	s.queueInboxTaskReview(ctx, after)
	requests, err := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, origin.RoomID)
	if err != nil {
		t.Fatal(err)
	}
	queued := 0
	for _, q := range requests {
		if q.State != "queued" || q.MessageID == origin.MessageID {
			continue
		}
		if s.inboxReviewedTask(ctx, q) != nil {
			queued++
			prompt, ok := s.inboxTaskReviewPrompt(ctx, q)
			if !ok || !strings.Contains(prompt, `"state":"connected"`) {
				t.Fatal("queued review lacked fresh connection")
			}
		}
	}
	if queued != 1 {
		t.Fatal("fresh review missing or duplicated", queued)
	}
}
