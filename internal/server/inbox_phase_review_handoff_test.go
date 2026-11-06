package server

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestReviewedPhaseHandoffUsesOriginalApprovalWithoutAnotherProvider(t *testing.T) {
	for _, scenario := range []string{"valid", "withdrawn approval", "foreign owner", "existing launch", "stopped launch", "manual block"} {
		t.Run(scenario, func(t *testing.T) { reviewedPhaseHandoffContract(t, scenario) })
	}
}

func TestReviewedPhaseHandoffPostgres(t *testing.T) {
	for _, scenario := range []string{"valid", "withdrawn approval", "existing launch", "stopped launch"} {
		t.Run(scenario, func(t *testing.T) {
			withPhasePostgres(t, func(database *sql.DB) { reviewedPhaseHandoffContract(t, scenario, database) })
		})
	}
}

func reviewedPhaseHandoffContract(t *testing.T, scenario string, databases ...*sql.DB) {
	s, owner, parent, child, _ := phaseLineageFixture(t, databases...)
	s.inboxStopping.Store(true)
	ctx := context.Background()
	projects := project.NewMemoryStore()
	if err := projects.CreateProject(ctx, &project.Project{ID: child.ProjectID, RepoURL: "https://github.com/fixture/dojo", DefaultBranch: "main"}); err != nil {
		t.Fatal(err)
	}
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	if err := s.taskSvc.FinishTaskLaunches(ctx, child.ID, "stopped"); err != nil {
		t.Fatal(err)
	}
	a := &task.TaskAttempt{ID: "handoff-source", TaskID: child.ID, AgentName: owner.AgentID, Status: task.AttemptStatusSucceeded, CreatedAt: time.Now()}
	if err := s.taskSvc.Store().CreateAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	child.Status, child.Summary, child.CompleteNote = task.TaskStatusReview, "API fixture result", "Fixture verification"
	if scenario == "manual block" {
		child.Status = task.TaskStatusBlocked
	}
	if err := s.taskSvc.Store().UpdateTask(ctx, &child.Task); err != nil {
		t.Fatal(err)
	}
	current, _ := s.taskSvc.GetTaskDetail(ctx, child.ID)
	key := "task-review:" + a.ID
	now := time.Now()
	q := db.InboxRequest{OwnerID: owner.OwnerID, AgentID: owner.AgentID, RoomID: child.SourceRoomID, RequestID: delegationMessageID(key), MessageID: delegationMessageID(key + ":input"), ReplyID: delegationMessageID(key + ":reply"), PayloadHash: delegationMessageID(key), State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	if scenario == "foreign owner" {
		q.OwnerID = "foreign"
	}
	repo := s.projectStudioRepo.(db.InboxRequestRepository)
	if _, _, err := repo.AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "system", AuthorID: "cuan_work_review", Content: "Review [/tasks/" + child.ID + "]", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	q.State = "completed"
	if err := repo.FinishInboxRequest(ctx, q, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{ID: q.ReplyID, RoomID: q.RoomID, AuthorType: "agent", AuthorID: q.AgentID, Content: "Old authorization error", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	decision := inboxReviewEvaluation{Decision: "revise", RemainingWork: "implementation", Finding: "An approved API gap remains", Explanation: "The API result needs a scoped correction.", Revision: "Correct the approved API gap without changing unrelated work.", ManagerAction: "continue", ManagerReason: "Original phase approval covers the correction"}
	if _, err := s.saveReviewAssessment(ctx, current, q, decision); err != nil {
		t.Fatal(err)
	}
	if scenario == "withdrawn approval" {
		m, _ := s.projectStudioRepo.GetMessageByID(ctx, parent.SourceMessageID)
		m.Content = "Plan withdrawn"
		if err := s.projectStudioRepo.UpdateMessage(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	if scenario == "existing launch" || scenario == "stopped launch" {
		if _, err := s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{TaskID: child.ID, ProjectID: child.ProjectID, OwnerID: owner.OwnerID, RoomID: q.RoomID, RequestID: q.RequestID, SourceRequestID: owner.RequestID, AgentID: q.AgentID, InstructionsHash: task.LaunchInstructionsHash(child.Instructions), Revision: decision.Revision, CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
		if scenario == "stopped launch" {
			if err := s.taskSvc.FinishTaskLaunches(ctx, child.ID, "stopped"); err != nil {
				t.Fatal(err)
			}
		}
	}
	current, _ = s.taskSvc.GetTaskDetail(ctx, child.ID)
	rows, _ := repo.ListInboxRequests(ctx, q.RoomID)
	recovered := s.recoverReviewedPhaseHandoff(ctx, current, rows)
	if recovered != (scenario == "valid") {
		t.Fatal("incorrect handoff recovery", scenario, recovered)
	}
	after, _ := s.taskSvc.GetTaskDetail(ctx, child.ID)
	if len(after.Attempts) != 1 || after.LatestAttempt.ID != a.ID || after.LatestAttempt.Status != task.AttemptStatusSucceeded {
		t.Fatal("handoff replayed execution")
	}
	reply, _ := s.projectStudioRepo.GetMessageByID(ctx, q.ReplyID)
	if scenario == "valid" {
		launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(child.ID, q.RequestID))
		if err != nil || launch.State != "queued" || launch.SourceRequestID != owner.RequestID {
			t.Fatal("phase correction did not use original approval", launch, err)
		}
		if _, authorized, err := s.authorizeSavedTaskLaunch(ctx, *launch, after); err != nil || !authorized {
			t.Fatal("corrected phase launch is not authorized", authorized, err)
		}
		limits := executionpolicy.FromInstructions(child.Instructions).Restrict(executionpolicy.FromInstructions(launch.Revision))
		if !limits.DenyPush || !limits.DenyDeploy || reply.Content == "Old authorization error" {
			t.Fatal("scope or reply not reconciled")
		}
		if s.recoverReviewedPhaseHandoff(ctx, after, rows) {
			t.Fatal("durable handoff replayed")
		}
	} else if reply.Content != "Old authorization error" {
		t.Fatal("failed recovery changed reply")
	}
	reviews := reviewRequests(t, s, q.RoomID)
	if len(reviews) != 1 || reviews[0].State != "completed" {
		t.Fatal("recovery called or replayed provider")
	}
}
