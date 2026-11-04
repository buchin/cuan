package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/task"
)

func TestOwnerContinuationHasBoundedIndependentReviewBudget(t *testing.T) {
	for _, scenario := range []string{"owner", "system", "unrelated", "cancelled", "foreign-owner", "changed-scope", "future"} {
		t.Run(scenario, func(t *testing.T) {
			s, _, _, _ := inboxFixture(t)
			ctx := context.Background()
			now := time.Now().UTC()
			room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
			item := &task.Task{ID: "review-budget-task", ProjectID: "project", Instructions: "Original approved scope", SourceRoomID: room.RoomID, SourceMessageID: "original", Status: task.TaskStatusBlocked}
			if err := s.taskSvc.Store().CreateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			current := &task.TaskAttempt{ID: "current", Status: task.AttemptStatusFailed, CreatedAt: now}
			d := &task.TaskDetailResponse{Task: *item, LatestAttempt: current, Attempts: []*task.TaskAttempt{{ID: "old1", Status: task.AttemptStatusFailed, CreatedAt: now.Add(-time.Hour)}, {ID: "old2", Status: task.AttemptStatusSucceeded, CreatedAt: now.Add(-time.Hour)}, current}}
			q := db.InboxRequest{RoomID: room.RoomID, OwnerID: "owner", AgentID: "agent_lifecycle", RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "queued", CreatedAt: now.Add(-time.Minute), Deadline: now.Add(time.Hour), PayloadHash: db.GenerateUUID()}
			author := "user"
			if scenario == "system" {
				author = "system"
			}
			if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: author, CreatedAt: q.CreatedAt}); err != nil {
				t.Fatal(err)
			}
			if scenario != "unrelated" {
				owner := q.OwnerID
				if scenario == "foreign-owner" {
					owner = "other"
				}
				hash := task.LaunchInstructionsHash(d.Instructions)
				if scenario == "changed-scope" {
					hash = task.LaunchInstructionsHash("Different work")
				}
				at := now.Add(-30 * time.Second)
				if scenario == "future" {
					at = now.Add(time.Minute)
				}
				_, err := s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{TaskID: d.ID, ProjectID: d.ProjectID, OwnerID: owner, RoomID: q.RoomID, RequestID: q.RequestID, SourceRequestID: "original", AgentID: q.AgentID, Revision: "Read-only audit. Do not push. Do not deploy. Verify the original remaining work", InstructionsHash: hash, CreatedAt: at, AvailableAt: at, ExpiresAt: at.Add(time.Hour)})
				if err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "cancelled" {
				if _, err := s.projectStudioRepo.(db.InboxLifecycleRepository).CancelInboxRequest(ctx, q.RoomID, q.RequestID); err != nil {
					t.Fatal(err)
				}
			}
			scoped := s.reviewBudgetDetail(ctx, d)
			want := 3
			if scenario == "owner" {
				// Keep launch preflight queued on a real occupied project lane. We test
				// the persisted intent, not an external provider or a manufactured success.
				if err := s.taskSvc.Store().CreateTask(ctx, &task.Task{ID: "busy-task", ProjectID: d.ProjectID, Status: task.TaskStatusDoing}); err != nil {
					t.Fatal(err)
				}
				review := db.InboxRequest{OwnerID: q.OwnerID, AgentID: q.AgentID, RoomID: q.RoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID()}
				out := s.startAuthorizedLaunch(ctx, review, d, review.RequestID, "original", "Correct the missing check and report evidence.")
				if !strings.Contains(out, "queued waiting") {
					t.Fatal(out)
				}
				job, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, review.RequestID))
				if err != nil {
					t.Fatal(err)
				}
				policy := executionpolicy.FromInstructions(job.Revision)
				if !policy.DenyPush || !policy.DenyDeploy || !strings.Contains(job.Revision, "Read-only audit.") {
					t.Fatal("automatic correction lost owner boundaries", job.Revision)
				}
				// A new accepted owner turn remains independent of prior revision text.
				fresh := q
				fresh.RequestID = db.GenerateUUID()
				fresh.MessageID = db.GenerateUUID()
				fresh.ReplyID = db.GenerateUUID()
				fresh.CreatedAt = time.Now().UTC()
				if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, fresh, &db.ProjectMessage{ID: fresh.MessageID, RoomID: fresh.RoomID, AuthorType: "user", CreatedAt: fresh.CreatedAt}); err != nil {
					t.Fatal(err)
				}
				s.startAuthorizedLaunch(ctx, fresh, d, fresh.RequestID, "original", "Continue original approved scope.")
				freshJob, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, fresh.RequestID))
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(freshJob.Revision, "Binding approved continuation:") {
					t.Fatal("old execution feedback replaced a new owner turn")
				}
				want = 1
			}
			if count := taskRevisionAttemptCount(scoped); count != want {
				t.Fatalf("budget %d want %d", count, want)
			}
			if len(d.Attempts) != 3 {
				t.Fatal("historical evidence mutated")
			}
			if scenario == "owner" {
				scoped.Attempts = append(scoped.Attempts, &task.TaskAttempt{Status: task.AttemptStatusFailed}, &task.TaskAttempt{Status: task.AttemptStatusFailed})
				if evaluateTaskDoDDeterministic(scoped).Decision != "blocked" {
					t.Fatal("new cycle is unbounded")
				}
			}
		})
	}
}

func TestRunnerVerificationReachesReviewAndRecoveryEvidence(t *testing.T) {
	d := &task.TaskDetailResponse{Task: task.Task{Summary: "Reported result", CompleteNote: "Reported checks", TestSummary: "Project verification: missing fixture-package"}, LatestAttempt: &task.TaskAttempt{ID: "attempt", Status: task.AttemptStatusFailed}}
	before := inboxReviewEvidenceDigest(d)
	if !strings.Contains(taskRecoveryContext(d), "missing fixture-package") {
		t.Fatal("recovery lost verifier output")
	}
	s, _, _, _ := inboxFixture(t)
	if !strings.Contains(s.inboxReviewData(context.Background(), d), "missing fixture-package") {
		t.Fatal("review lost verifier output")
	}
	d.TestSummary = "Project verification: a different concrete failure"
	if before == inboxReviewEvidenceDigest(d) {
		t.Fatal("new verification cannot trigger a fresh review")
	}
}
