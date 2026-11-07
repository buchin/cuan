package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestReviewSeparatesOwnerLimitsFromAutomaticVerification(t *testing.T) {
	reviewAuthorityContract(t)
}
func TestReviewAuthorityPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) { reviewAuthorityContract(t, database) })
}
func reviewAuthorityContract(t *testing.T, databases ...*sql.DB) {
	for _, scenario := range []string{"implementation", "owner audit", "later owner audit", "unavailable approval"} {
		t.Run(scenario, func(t *testing.T) {
			s, origin, d := phaseLineageSetup(t, databases...)
			ctx := context.Background()
			d.Instructions = "Cuan managed delegation v1.\nImplement feature, commit and deploy the approved result."
			if scenario == "owner audit" {
				d.Instructions = "Cuan managed delegation v1.\nScope: read-only repository. Audit the existing result. Do not push. Do not deploy."
			}
			d.Status = task.TaskStatusBlocked
			if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
				t.Fatal(err)
			}
			if scenario == "later owner audit" {
				request := origin
				request.MessageID = db.GenerateUUID()
				request.RequestID = db.GenerateUUID()
				request.CreatedAt = time.Now().UTC()
				message := &db.ProjectMessage{ID: request.MessageID, RoomID: request.RoomID, AuthorType: "user", Content: "Only audit this repository. Do not push or deploy.", CreatedAt: request.CreatedAt, UpdatedAt: request.CreatedAt}
				if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, request, message); err != nil {
					t.Fatal(err)
				}
				if _, err := s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{TaskID: d.ID, ProjectID: d.ProjectID, RequestID: request.RequestID, OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, AgentID: d.AssignedAgent, Revision: "Scope: read-only repository. Audit only. Do not push. Do not deploy.", InstructionsHash: task.LaunchInstructionsHash(d.Instructions), CreatedAt: time.Now().UTC(), AvailableAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
					t.Fatal(err)
				}
			}
			readonly := verificationReviewRevision("Inspect the actual result")
			requestID := db.GenerateUUID()
			launch, err := s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{TaskID: d.ID, ProjectID: d.ProjectID, RequestID: requestID, OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, AgentID: d.AssignedAgent, Revision: readonly, InstructionsHash: task.LaunchInstructionsHash(d.Instructions), CreatedAt: time.Now(), AvailableAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)})
			if err != nil {
				t.Fatal(err)
			}
			_ = s.taskSvc.Store().DeleteTask(ctx, "phase-blocker")
			queued := task.TaskStatusQueued
			if _, err := s.taskSvc.UpdateTask(ctx, d.ID, "coordinator", task.UpdateTaskRequest{Status: &queued}); err != nil {
				t.Fatal(err)
			}
			if claimed, err := s.taskSvc.ClaimLaunch(ctx, time.Now(), "review-scope", launch.ID); err != nil || claimed == nil {
				t.Fatal("claim", err)
			}
			a, err := s.taskSvc.CreateAttempt(ctx, d.ID, origin.OwnerID, task.CreateAttemptRequest{RunnerID: "fixture", RequestID: requestID, AgentName: d.AssignedAgent})
			if err != nil {
				t.Fatal(err)
			}
			d, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "unavailable approval" {
				s.projectStudioRepo = nil
			}
			approved, continuation := s.inboxApprovedTaskScope(ctx, d)
			current, err := s.taskSvc.AttemptPublicationPolicy(ctx, d.ID, a.ID)
			if err != nil || !current.ReadOnlyRepository || !current.DenyPush || !current.DenyDeploy {
				t.Fatal("verifier limits changed", current, err)
			}
			if scenario == "unavailable approval" {
				if approved != nil {
					t.Fatal("unavailable approval inferred permission")
				}
				return
			}
			wantReadOnly := scenario != "implementation"
			if approved == nil || approved.ReadOnlyRepository != wantReadOnly {
				t.Fatal("automatic verifier replaced owner scope", approved)
			}
			if scenario == "later owner audit" && continuation == "" {
				t.Fatal("later owner boundary lost")
			}
			var data map[string]json.RawMessage
			if err := json.Unmarshal([]byte(s.inboxReviewData(ctx, d)), &data); err != nil {
				t.Fatal(err)
			}
			var actual, ownerScope map[string]bool
			_ = json.Unmarshal(data["execution_scope"], &actual)
			_ = json.Unmarshal(data["approved_task_scope"], &ownerScope)
			var revision string
			_ = json.Unmarshal(data["current_execution_revision"], &revision)
			if !actual["read_only_repository"] || ownerScope["read_only_repository"] != wantReadOnly || revision != readonly {
				t.Fatal("review context conflated attempt and owner limits")
			}
		})
	}
}
