package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestPublicationReviewCannotAcceptUnpublishedResult(t *testing.T) {
	for _, scenario := range []string{"verification revision", "implementation revision", "open PR", "report claim", "wrong receipt", "verified receipt", "unrelated running build", "exact project build", "foreign project build", "owner forbids deployment"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			s, _, _, _ := inboxFixture(t)
			projects := project.NewMemoryStore()
			repo := "https://github.com/buchin/cuan"
			if scenario == "foreign project build" {
				repo = "https://github.com/buchin/other"
			}
			if err := projects.CreateProject(ctx, &project.Project{ID: "publication-project", Name: "Project", RepoURL: repo, Metadata: registeredReleaseMetadata(t)}); err != nil {
				t.Fatal(err)
			}
			s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
			commit := strings.Repeat("c", 40)
			now := time.Now().UTC()
			d := &task.TaskDetailResponse{Task: task.Task{ID: "publication-task", ProjectID: "publication-project", Title: "Implement and deploy", CommitSHA: commit, ExecutionCategory: task.ExecutionCategoryCode, PRNumber: 24, Instructions: "Implement and deploy the approved feature.", CompleteNote: "Implementation tests passed."}, ResultKind: "implementation", LatestAttempt: &task.TaskAttempt{ID: "publication-attempt", TaskID: "publication-task", Status: task.AttemptStatusSucceeded, CreatedAt: now.Add(-time.Minute)}}
			if scenario == "report claim" {
				d.CompleteNote = "The deployment is live and verified. All criteria passed."
			}
			if scenario == "verified receipt" || scenario == "wrong receipt" {
				receiptCommit := commit
				if scenario == "wrong receipt" {
					receiptCommit = strings.Repeat("d", 40)
				}
				raw, _ := json.Marshal(task.DeploymentReceipt{Version: 1, TaskID: d.ID, AttemptID: d.LatestAttempt.ID, CommitSHA: receiptCommit, ReleaseSHA: receiptCommit, Environment: "production", Health: "healthy"})
				d.Activities = []*task.TaskActivity{{ID: task.DeploymentReceiptID(d.ID, d.LatestAttempt.ID, receiptCommit), TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: "deployment_verified", Author: "cuan-deploy", Message: string(raw), CreatedAt: now.Add(-time.Second)}}
			}
			if strings.Contains(scenario, "build") {
				running := commit
				if scenario == "unrelated running build" {
					running = strings.Repeat("d", 40)
				}
				s.cfg = &config.Config{Environment: "production", SourceRepository: "buchin/cuan", CommitSHA: running}
			}
			if scenario == "owner forbids deployment" {
				d.Instructions = "Do not deploy. Return verified local changes."
			}
			if err := s.taskSvc.Store().CreateTask(ctx, &d.Task); err != nil {
				t.Fatal(err)
			}
			if err := s.taskSvc.Store().CreateAttempt(ctx, d.LatestAttempt); err != nil {
				t.Fatal(err)
			}
			input := inboxReviewEvaluation{Decision: "pass", Finding: "Everything passed", RemainingWork: "none"}
			if strings.HasSuffix(scenario, "revision") {
				input = inboxReviewEvaluation{Decision: "revise", RemainingWork: strings.Fields(scenario)[0], Finding: "Deployment did not happen", Explanation: "Deploy again to obtain a receipt", Revision: "Check the approved feature", VisualInspection: "Observed existing view"}
			}
			result := s.requirePublicationReview(ctx, d, input)
			if observed := s.inboxReviewPublication(ctx, d); observed != nil && !observed.Verified && missingPublicationConnection(observed.ConnectionState) {
				if result.Decision != "blocked" || result.RemainingWork != "external" || result.ManagerAction != "wait" || result.Revision != "" {
					t.Fatal("missing connection launched another repair", result)
				}
				return
			}
			if scenario == "implementation revision" {
				if result.RemainingWork != "verification" || !strings.Contains(result.Revision, input.Finding) || !strings.Contains(result.Revision, input.Revision) || !strings.HasPrefix(result.Revision, "First reconcile") || strings.Contains(result.Explanation, "Deploy again") || !strings.Contains(result.Finding, "belum bisa dipastikan") {
					t.Fatal("implementation gap or publication reconciliation lost")
				}
				return
			}
			if scenario == "verification revision" && (!strings.Contains(result.Revision, input.Revision) || result.VisualInspection != input.VisualInspection) {
				t.Fatal("verification obligations lost")
			}
			allowed := scenario == "verified receipt" || scenario == "exact project build" || scenario == "owner forbids deployment"
			if (result.Decision == "pass") != allowed {
				t.Fatalf("publication gate result %+v", result)
			}
			if !allowed && (result.RemainingWork != "verification" || !strings.Contains(result.Finding, "belum bisa dipastikan")) {
				t.Fatal("publication gap was lost")
			}
			if d.CommitSHA != commit || d.LatestAttempt.Status != task.AttemptStatusSucceeded {
				t.Fatal("review changed historical result")
			}
		})
	}
}

func TestLegacyPublicationAcceptanceReevaluatesWithoutExecution(t *testing.T) {
	for _, scenario := range []string{"missing publication", "verified receipt", "owner done", "owner prohibition", "cancelled approval", "current instructions", "clone scope"} {
		t.Run(scenario, func(t *testing.T) {
			s, d, oldReview := stalledReviewFixture(t, "completed")
			ctx := context.Background()
			projects := project.NewMemoryStore()
			if err := projects.CreateProject(ctx, &project.Project{ID: d.ProjectID, Name: "Project", RepoURL: "https://github.com/buchin/cuan"}); err != nil {
				t.Fatal(err)
			}
			s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
			scope := "Implement and deploy to production."
			if scenario == "clone scope" {
				scope = "Clone the registered repository and verify its structure."
			}
			if scenario == "owner prohibition" {
				scope = "Do not deploy. Return only local results."
			}
			d.Instructions = "Cuan managed delegation v1.\nApproved implementation.\nApproval message: " + d.SourceMessageID + "\nUser authorization: " + scope + "\nConversation: " + d.SourceRoomID + "\nReport actual progress and verification evidence. Stay within this scope; do not deploy unless authorized."
			if scenario == "current instructions" {
				d.Instructions = strings.Replace(d.Instructions, "; do not deploy unless authorized.", ".", 1)
			}
			d.CommitSHA = strings.Repeat("c", 40)
			if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
				t.Fatal(err)
			}
			origin, err := s.projectStudioRepo.GetMessageByID(ctx, d.SourceMessageID)
			if err != nil {
				t.Fatal(err)
			}
			origin.Content = scope
			if err := s.projectStudioRepo.UpdateMessage(ctx, origin); err != nil {
				t.Fatal(err)
			}
			if scenario == "cancelled approval" {
				q := db.InboxRequest{OwnerID: oldReview.OwnerID, AgentID: d.AssignedAgent, RoomID: d.SourceRoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "queued", PayloadHash: "cancelled-approval", CreatedAt: time.Now().Add(-time.Hour), Deadline: time.Now().Add(time.Hour)}
				repo := s.projectStudioRepo.(db.InboxRequestRepository)
				if _, _, err := repo.AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: scope, CreatedAt: q.CreatedAt}); err != nil {
					t.Fatal(err)
				}
				if changed, err := s.projectStudioRepo.(db.InboxLifecycleRepository).CancelInboxRequest(ctx, q.RoomID, q.RequestID); err != nil || !changed {
					t.Fatal("cancel approval", err)
				}
				d.Instructions = strings.Replace(d.Instructions, d.SourceMessageID, q.MessageID, 1)
				d.SourceMessageID = q.MessageID
				if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
					t.Fatal(err)
				}
			}
			d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
			if _, err := s.saveReviewAssessment(ctx, d, oldReview, inboxReviewEvaluation{Decision: "pass", RemainingWork: "none", Finding: "Local branch passed", Explanation: "Local branch passed"}); err != nil {
				t.Fatal(err)
			}
			if scenario == "verified receipt" {
				raw, _ := json.Marshal(task.DeploymentReceipt{Version: 1, TaskID: d.ID, AttemptID: d.LatestAttempt.ID, CommitSHA: d.CommitSHA, ReleaseSHA: d.CommitSHA, Environment: "production", Health: "healthy"})
				if err := s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{ID: task.DeploymentReceiptID(d.ID, d.LatestAttempt.ID, d.CommitSHA), TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: "deployment_verified", Author: "cuan-deploy", Message: string(raw), CreatedAt: time.Now().UTC()}); err != nil {
					t.Fatal(err)
				}
			}
			done := task.TaskStatusDone
			author := "coordinator"
			if scenario == "owner done" {
				author = "owner"
			}
			if _, err := s.taskSvc.UpdateTask(ctx, d.ID, author, task.UpdateTaskRequest{Status: &done}); err != nil {
				t.Fatal(err)
			}
			s.reconcileLegacyPublicationAcceptances(ctx)
			s.reconcileLegacyPublicationAcceptances(ctx)
			after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
			want := task.TaskStatusDone
			if scenario == "missing publication" {
				want = task.TaskStatusReview
			}
			if after.Status != want || len(after.Attempts) != 1 || after.LatestAttempt.Status != task.AttemptStatusSucceeded {
				t.Fatalf("audit replayed or changed result: %+v", after.LatestAttempt)
			}
			if after.Instructions != d.Instructions || after.CommitSHA != d.CommitSHA {
				t.Fatal("audit rewrote approved scope or result commit")
			}
			if scenario == "missing publication" {
				s.queueInboxTaskReview(ctx, after)
				s.queueInboxTaskReview(ctx, after)
				rows := reviewRequests(t, s, d.SourceRoomID)
				if len(rows) != 2 {
					t.Fatalf("publication review missing or duplicated: %d", len(rows))
				}
				var fresh db.InboxRequest
				for _, q := range rows {
					if q.RequestID != oldReview.RequestID {
						fresh = q
					}
				}
				if s.inboxReviewedTask(ctx, fresh) == nil {
					t.Fatal("new publication review does not bind task")
				}
				message, err := s.projectStudioRepo.GetMessageByID(ctx, fresh.MessageID)
				if err != nil || !strings.Contains(message.Content, inboxPublicationReviewPolicy) || strings.Contains(message.Content, "Stay within this scope; do not deploy unless authorized.") {
					t.Fatal("review lost corrected publication context")
				}
			}
		})
	}
}
