package server

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func stalledReviewFixture(t *testing.T, state string, executionStatus ...string) (*Server, *task.TaskDetailResponse, db.InboxRequest) {
	t.Helper()
	s, _, _, _ := inboxFixture(t)
	s.inboxStopping.Store(true)
	ctx := context.Background()
	now := time.Now().UTC()
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	repo := s.projectStudioRepo.(db.InboxRequestRepository)
	origin := db.InboxRequest{OwnerID: "owner", AgentID: "agent_lifecycle", RoomID: room.RoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), PayloadHash: db.GenerateUUID(), State: "queued", CreatedAt: now.Add(-time.Hour), Deadline: now.Add(time.Hour)}
	if _, _, err := repo.AcceptInboxRequest(ctx, origin, &db.ProjectMessage{ID: origin.MessageID, RoomID: room.RoomID, AuthorType: "user", Content: "Compare existing findings and report them", CreatedAt: origin.CreatedAt}); err != nil {
		t.Fatal(err)
	}
	origin.State = "completed"
	if err := repo.FinishInboxRequest(ctx, origin, nil); err != nil {
		t.Fatal(err)
	}
	item := &task.Task{ID: "review-recovery-task", ProjectID: "default", SourceRoomID: room.RoomID, SourceMessageID: origin.MessageID, AssignedAgent: origin.AgentID, Title: "Compare retained findings", Instructions: "Cuan managed delegation v1.\nCompare existing findings and report them", Status: task.TaskStatusReview, Summary: "Existing findings compared", CompleteNote: "The recorded inputs differ in two named sections", CreatedAt: now, UpdatedAt: now}
	if err := s.taskSvc.Store().CreateTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	attempt := &task.TaskAttempt{ID: "review-recovery-attempt", TaskID: item.ID, RunnerID: "fixture", AgentName: origin.AgentID, Status: task.AttemptStatusSucceeded, ExitCode: new(int), CreatedAt: now}
	if len(executionStatus) > 0 {
		attempt.Status = executionStatus[0]
	}
	if err := s.taskSvc.Store().CreateAttempt(ctx, attempt); err != nil {
		t.Fatal(err)
	}
	key := "task-review:" + attempt.ID
	q := db.InboxRequest{OwnerID: origin.OwnerID, AgentID: origin.AgentID, RoomID: room.RoomID, RequestID: delegationMessageID(key), MessageID: delegationMessageID(key + ":input"), ReplyID: delegationMessageID(key + ":reply"), PayloadHash: delegationMessageID(key), State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	if _, _, err := repo.AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "system", AuthorID: "cuan_work_review", Content: "Legacy review [/tasks/" + item.ID + "]: UNKNOWN_OLD_TOOL_OPERATION", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	// The old provider process is terminal, including unknown historical tool use.
	s.projectStudioRepo.(db.InboxLifecycleRepository).AppendInboxStep(ctx, q.RoomID, q.RequestID, "runtime_tool_started", "Unknown old operation")
	q.State = state
	q.SafeError = "Provider process stopped; result unknown"
	if err := repo.FinishInboxRequest(ctx, q, nil); err != nil {
		t.Fatal(err)
	}
	detail, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s, detail, q
}
func reviewRequests(t *testing.T, s *Server, room string) []db.InboxRequest {
	t.Helper()
	rows, err := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(context.Background(), room)
	if err != nil {
		t.Fatal(err)
	}
	var result []db.InboxRequest
	for _, q := range rows {
		if q.MessageID != "" {
			m, _ := s.projectStudioRepo.GetMessageByID(context.Background(), q.MessageID)
			if m != nil && m.AuthorID == "cuan_work_review" {
				result = append(result, q)
			}
		}
	}
	return result
}
func TestStalledReviewReconcilesCurrentSnapshotWithoutReplayingOldProvider(t *testing.T) {
	for _, state := range []string{"failed", "interrupted", "completed"} {
		t.Run(state, func(t *testing.T) {
			s, d, old := stalledReviewFixture(t, state)
			ctx := context.Background()
			// This fixture compares retained findings; it does not implement code.
			d.ExecutionCategory = task.ExecutionCategoryReadOnly
			if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
				t.Fatal(err)
			}
			// This acceptance fixture represents a finished native read-only audit,
			// not merely an old task-level summary without attempt-bound proof.
			finished := time.Now().UTC()
			d.LatestAttempt.FinishedAt = &finished
			if err := s.taskSvc.Store().UpdateAttempt(ctx, d.LatestAttempt); err != nil {
				t.Fatal(err)
			}
			for _, a := range []*task.TaskActivity{
				{ID: "reconciled-current-report", TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: "agent_report", Author: d.AssignedAgent, NewStatus: task.TaskStatusReview, Message: d.Summary + "\n" + d.CompleteNote, CreatedAt: finished},
				{ID: "reconciled-unchanged-receipt", TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: "repository_scope_verified", Author: d.LatestAttempt.RunnerID, NewStatus: "read_only", Message: "Native fixture verified unchanged audit scope", CreatedAt: finished},
			} {
				if err := s.taskSvc.Store().CreateActivity(ctx, a); err != nil {
					t.Fatal(err)
				}
			}
			var scans sync.WaitGroup
			for i := 0; i < 8; i++ {
				scans.Add(1)
				go func() { defer scans.Done(); s.queueInboxTaskReview(ctx, d) }()
			}
			scans.Wait()
			rows := reviewRequests(t, s, old.RoomID)
			if len(rows) != 2 {
				t.Fatalf("recovery duplicated: %d", len(rows))
			}
			var recovery db.InboxRequest
			for _, q := range rows {
				if q.RequestID != old.RequestID {
					recovery = q
				} else if q.State != state {
					t.Fatal("old request was replayed")
				}
			}
			snapshot, ok := s.inboxTaskReviewPrompt(ctx, recovery)
			if !ok || !strings.Contains(snapshot, d.CompleteNote) || strings.Contains(snapshot, "UNKNOWN_OLD_TOOL_OPERATION") || recovery.State != "queued" {
				t.Fatal("recovery did not use current tool-free snapshot", snapshot)
			}
			after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
			if after.Status != d.Status || len(after.Attempts) != 1 {
				t.Fatal("snapshot creation started work or changed task status")
			}
			// A properly bound snapshot applies the decision, instead of merely displaying it.
			output := s.applyInboxReviewEvaluation(ctx, recovery, `{"decision":"pass","explanation":"The supplied findings cover the approved comparison.","revision":""}`)
			after, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
			if after.Status != task.TaskStatusDone || !strings.Contains(output, "supplied findings") {
				t.Fatal("recovered review was only displayed", output)
			}
		})
	}
}
func TestStalledReviewRecoveryIsBoundedAndTracksOnlyNewEvidence(t *testing.T) {
	s, d, old := stalledReviewFixture(t, "interrupted")
	ctx := context.Background()
	repo := s.projectStudioRepo.(db.InboxRequestRepository)
	for round := 0; round < 4; round++ {
		s.queueInboxTaskReview(ctx, d)
		rows := reviewRequests(t, s, old.RoomID)
		want := min(round+2, 3)
		if len(rows) != want {
			t.Fatalf("round%d: got%d want%d", round, len(rows), want)
		}
		for _, q := range rows {
			if q.State == "queued" {
				q.State = "interrupted"
				if err := repo.FinishInboxRequest(ctx, q, nil); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	d.Status = task.TaskStatusBlocked
	s.queueInboxTaskReview(ctx, d)
	if len(reviewRequests(t, s, old.RoomID)) != 3 {
		t.Fatal("administrative change reset budget")
	}
	d.CompleteNote += "; new verified evidence"
	s.queueInboxTaskReview(ctx, d)
	if len(reviewRequests(t, s, old.RoomID)) != 4 {
		t.Fatal("genuinely new evidence not reviewed")
	}
}
func TestStalledReviewRecoveryPreservesCancellationAndNewExecution(t *testing.T) {
	for _, state := range []string{"canceled", "cancel-requested", "cancelled-launch", "queued-launch", "claimed-launch", "queued-task", "running", "done"} {
		t.Run(state, func(t *testing.T) {
			s, d, old := stalledReviewFixture(t, "interrupted")
			switch state {
			case "canceled":
				d.LatestAttempt.Status = task.AttemptStatusCanceled
			case "cancel-requested":
				d.LatestAttempt.CancelRequested = true
			case "cancelled-launch":
				d.Launch = &task.LaunchInfo{State: "cancelled"}
			case "queued-launch":
				d.Launch = &task.LaunchInfo{State: "queued"}
			case "claimed-launch":
				d.Launch = &task.LaunchInfo{State: "claimed"}
			case "queued-task":
				d.Status = task.TaskStatusQueued
			case "running":
				d.LatestAttempt.Status = task.AttemptStatusRunning
			case "done":
				d.Status = task.TaskStatusDone
			}
			s.queueInboxTaskReview(context.Background(), d)
			if len(reviewRequests(t, s, old.RoomID)) != 1 {
				t.Fatal(fmt.Sprint("terminal or active state created recovery: ", state))
			}
		})
	}
}

func TestStalledReviewRecoveryRequiresStoredOwnerApproval(t *testing.T) {
	s, d, old := stalledReviewFixture(t, "interrupted")
	ctx := context.Background()
	requests, err := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, old.RoomID)
	if err != nil {
		t.Fatal(err)
	}
	var origin db.InboxRequest
	for _, q := range requests {
		if q.MessageID == d.SourceMessageID {
			origin = q
		}
	}
	if !s.reviewSnapshotRecoveryAuthorized(ctx, d, origin) {
		t.Fatal("valid owner approval rejected")
	}
	for _, kind := range []string{"missing", "canceled", "foreign-room", "foreign-message"} {
		copy := origin
		switch kind {
		case "missing":
			copy.OwnerID = ""
		case "canceled":
			copy.State = "cancelled"
		case "foreign-room":
			copy.RoomID = "other-room"
		case "foreign-message":
			copy.MessageID = old.MessageID
		}
		if s.reviewSnapshotRecoveryAuthorized(ctx, d, copy) {
			t.Fatal("invalid approval admitted", kind)
		}
	}
}

func TestReviewChainAdvancesDownCompletedDerivedReviews(t *testing.T) {
	s, d, old := stalledReviewFixture(t, "completed")
	ctx := context.Background()
	repo := s.projectStudioRepo.(db.InboxRequestRepository)

	// Step 1: Base review is completed without ownership policy -> queues :ownership-assessment:
	s.queueInboxTaskReview(ctx, d)
	rows := reviewRequests(t, s, old.RoomID)
	if len(rows) != 2 {
		t.Fatalf("expected 2 review requests, got %d", len(rows))
	}

	// Complete the derived ownership-assessment review
	var derived *db.InboxRequest
	for i := range rows {
		if rows[i].RequestID != old.RequestID {
			derived = &rows[i]
			break
		}
	}
	if derived == nil {
		t.Fatal("derived review not found")
	}
	derived.State = "completed"
	if err := repo.FinishInboxRequest(ctx, *derived, nil); err != nil {
		t.Fatal(err)
	}

	// Step 2: Calling queueInboxTaskReview without evidence change should not queue anything new
	s.queueInboxTaskReview(ctx, d)
	rows2 := reviewRequests(t, s, old.RoomID)
	if len(rows2) != 2 {
		t.Fatalf("expected still 2 review requests, got %d", len(rows2))
	}

	// Step 3: Evidence changes -> chain loop must traverse completed base & ownership reviews and queue :evidence:
	d.CompleteNote += "; updated test evidence note"
	s.queueInboxTaskReview(ctx, d)
	rows3 := reviewRequests(t, s, old.RoomID)
	if len(rows3) != 3 {
		t.Fatalf("expected 3 review requests after evidence change, got %d", len(rows3))
	}
}

func TestQueueInboxTaskReview_ChainedEvaluationV2(t *testing.T) {
	s, d, old := stalledReviewFixture(t, "completed", task.AttemptStatusSucceeded)
	ctx := context.Background()
	repo := s.projectStudioRepo.(db.InboxRequestRepository)

	// Update old message so it has ownership policy
	oldMsg, err := s.projectStudioRepo.GetMessageByID(ctx, old.MessageID)
	if err != nil || oldMsg == nil {
		t.Fatal("failed to get old message")
	}
	digest := inboxReviewEvidenceDigest(d)
	oldMsg.Content = "Review the result for [" + d.Title + "](/tasks/" + d.ID + ").\nReview evidence digest: " + digest + "\n" + inboxOwnershipPolicy
	if err := s.projectStudioRepo.UpdateMessage(ctx, oldMsg); err != nil {
		t.Fatal(err)
	}

	// First queue call: queues :evaluation:
	s.queueInboxTaskReview(ctx, d)

	reqs, err := repo.ListInboxRequests(ctx, old.RoomID)
	if err != nil {
		t.Fatal(err)
	}
	var evalReq *db.InboxRequest
	for i := range reqs {
		if reqs[i].State == "queued" {
			evalReq = &reqs[i]
			break
		}
	}
	if evalReq == nil {
		t.Fatalf("expected queued :evaluation: request, got none among %d requests", len(reqs))
	}

	// Mark the :evaluation: request as completed
	evalReq.State = "completed"
	if err := repo.FinishInboxRequest(ctx, *evalReq, nil); err != nil {
		t.Fatal(err)
	}

	// Second queue call: chain loop encounters completed :evaluation: and must advance to :evaluation-v2:
	s.queueInboxTaskReview(ctx, d)

	reqs2, err := repo.ListInboxRequests(ctx, old.RoomID)
	if err != nil {
		t.Fatal(err)
	}
	var v2Req *db.InboxRequest
	for i := range reqs2 {
		if reqs2[i].State == "queued" && reqs2[i].RequestID != evalReq.RequestID {
			v2Req = &reqs2[i]
			break
		}
	}
	if v2Req == nil {
		t.Fatalf("expected queued :evaluation-v2: request, got none among %d requests", len(reqs2))
	}
	if bound := s.inboxReviewedTask(ctx, *v2Req); bound == nil || bound.ID != d.ID {
		t.Fatalf("evaluation-v2 request failed to bind task: got %+v", bound)
	}
}

