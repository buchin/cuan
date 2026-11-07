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

func TestSavedManagerVerificationAdmissionRecoversWithoutModelReplay(t *testing.T) {
	for _, scenario := range []string{"valid", "stale", "cancelled", "existing stopped", "missing policy"} {
		t.Run(scenario, func(t *testing.T) {
			s, d := managerFixture(t)
			ctx := context.Background()
			store := s.taskSvc.Store()
			row, _ := store.GetTask(ctx, d.ID)
			row.Instructions += "\nApproval message: " + d.SourceMessageID
			if err := store.UpdateTask(ctx, row); err != nil {
				t.Fatal(err)
			}
			d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
			repo := s.projectStudioRepo.(db.InboxRequestRepository)
			requests, _ := repo.ListInboxRequests(ctx, d.SourceRoomID)
			var origin db.InboxRequest
			for _, q := range requests {
				if q.MessageID == d.SourceMessageID {
					origin = q
				}
			}
			key := managerReviewKey(d)
			now := time.Now().UTC()
			q := db.InboxRequest{OwnerID: origin.OwnerID, AgentID: origin.AgentID, RoomID: origin.RoomID, RequestID: delegationMessageID(key), MessageID: delegationMessageID(key + ":input"), ReplyID: delegationMessageID(key + ":reply"), PayloadHash: delegationMessageID(key), State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
			policy := inboxOwnershipPolicy
			if scenario == "missing policy" {
				policy = "Legacy unstructured review"
			}
			if _, _, err := repo.AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "system", AuthorID: "cuan_work_review", Content: policy + "\nReview [/tasks/" + d.ID + "]", CreatedAt: now, UpdatedAt: now}); err != nil {
				t.Fatal(err)
			}
			q.State = "completed"
			if err := repo.FinishInboxRequest(ctx, q, nil); err != nil {
				t.Fatal(err)
			}
			decision := inboxReviewEvaluation{Decision: "revise", RemainingWork: "verification", Finding: "Actual component screenshot missing", Explanation: "Complete original evidence", Revision: "Capture the actual original component using existing access.", ManagerAction: "replan", ManagerReason: "The previous general fixture did not cover this component."}
			if _, err := s.saveReviewAssessment(ctx, d, q, decision); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "stale":
				row.Summary += " Changed evidence"
				if err := store.UpdateTask(ctx, row); err != nil {
					t.Fatal(err)
				}
			case "cancelled":
				row.Status = "cancelled"
				if err := store.UpdateTask(ctx, row); err != nil {
					t.Fatal(err)
				}
			case "existing stopped":
				j, err := s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{TaskID: d.ID, ProjectID: d.ProjectID, RequestID: q.RequestID, SourceRequestID: origin.RequestID, OwnerID: origin.OwnerID, RoomID: origin.RoomID, AgentID: d.AssignedAgent, InstructionsHash: task.LaunchInstructionsHash(d.Instructions), Revision: verificationReviewRevision(decision.Revision), CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := store.(task.LaunchStore).ClaimLaunch(ctx, now, "stopped", j.ID); err != nil {
					t.Fatal(err)
				}
				if err := s.taskSvc.FinishLaunch(ctx, j.ID, "stopped", "stopped", now, "Existing failed launch"); err != nil {
					t.Fatal(err)
				}
			}
			if err := store.CreateTask(ctx, &task.Task{ID: "busy", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
				t.Fatal(err)
			}
			d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
			requests, _ = repo.ListInboxRequests(ctx, d.SourceRoomID)
			if got := s.recoverManagerVerificationHandoff(ctx, d, origin, requests); got != (scenario == "valid") {
				t.Fatal("incorrect saved decision recovery", got, scenario)
			}
			if scenario != "valid" {
				return
			}
			launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID))
			if err != nil || launch == nil || launch.State != "queued" || !strings.Contains(launch.Revision, decision.Revision) {
				t.Fatal("missing exact saved manager admission", err)
			}
			p := executionpolicy.FromInstructions(launch.Revision)
			if !p.ReadOnlyRepository || !p.DenyPush || !p.DenyDeploy {
				t.Fatal("recovered handoff lost verification boundaries")
			}
			s.recoverManagerVerificationHandoff(ctx, d, origin, requests)
			after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
			afterRequests, _ := repo.ListInboxRequests(ctx, d.SourceRoomID)
			if len(after.Attempts) != 3 || len(afterRequests) != len(requests) || after.Status == task.TaskStatusDone {
				t.Fatal("recovery replayed a model request, reset history or accepted work")
			}
		})
	}
}
