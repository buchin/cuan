package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestCodePassRequiresSeparateVerifier(t *testing.T) {
	for _, scenario := range []string{"missing source checks", "first successful implementation", "independent criterion pending", "pass with independent criterion pending", "unclassified revision", "native handoff"} {
		ready := scenario != "missing source checks"
		t.Run(scenario, func(t *testing.T) {
			s, d, q := stalledReviewFixture(t, "completed")
			ctx := context.Background()
			store := s.taskSvc.Store()
			if err := store.CreateTask(ctx, &task.Task{ID: "busy", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
				t.Fatal(err)
			}
			row, _ := store.GetTask(ctx, d.ID)
			row.ExecutionCategory = task.ExecutionCategoryCode
			row.Instructions += "\nDo not push. Do not deploy."
			row.CommitSHA = strings.Repeat("a", 40)
			if strings.Contains(scenario, "criterion pending") {
				row.DoDChecklist = []task.DoDItem{{ID: "independent-check", Text: "Independently verify the exact implemented result", Completed: false, Evidence: "Awaiting separate verification"}}
			}
			if err := store.UpdateTask(ctx, row); err != nil {
				t.Fatal(err)
			}
			stamp := time.Now()
			for _, event := range []struct{ kind, state string }{{"agent_report", "review"}, {"agent_result_kind", "implementation"}} {
				if err := store.CreateActivity(ctx, &task.TaskActivity{ID: event.kind, TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: event.kind, NewStatus: event.state, Message: "Current implementation evidence", CreatedAt: stamp}); err != nil {
					t.Fatal(err)
				}
			}
			if ready {
				raw, _ := json.Marshal(task.RunnerVerificationRecord{Summary: "Project verification: test.sh --fast (exit 0)"})
				if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "source-checks", TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: task.ActivityTypeRunnerVerification, Author: d.LatestAttempt.RunnerID, Message: string(raw), CreatedAt: time.Now()}); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
			t.Logf("source kind=%s checks=%v gaps=%v report=%s", before.ResultKind, task.ProjectChecksPassed(before), task.AcceptanceGaps(&before.Task), inboxReportEvidence(before).Provenance)
			raw := `{"decision":"pass","finding":"Code ready","explanation":"Passed","remaining_work":"none","revision":""}`
			if scenario == "independent criterion pending" {
				raw = `{"decision":"revise","finding":"Independent evidence pending","explanation":"Needs verification","remaining_work":"verification","revision":"Independently inspect the exact source commit and verify original criteria without implementation changes."}`
			}
			if scenario == "unclassified revision" {
				raw = `{"decision":"revise","explanation":"Source tests passed; independent verification pending","revision":"Verify the exact source before native release."}`
			}
			if scenario == "native handoff" {
				var routed bool
				raw, routed = s.inboxCodeVerificationHandoff(ctx, q)
				if !routed {
					t.Fatal("ready source still needs preliminary model inference")
				}
			}
			if ready {
				s.taskSvc = task.NewService(reviewLaunchFailureStore{Store: store, LaunchStore: store.(task.LaunchStore), failure: errors.New("temporary storage outage")}, s.projectSvc)
				retry := false
				bounded := context.WithValue(ctx, inboxHandoffPersistenceKey{}, &retry)
				out := s.applyInboxReviewEvaluation(bounded, q, raw)
				if !retry || strings.Contains(out, "verifikasi tambahan") {
					t.Fatal("lost durable scheduling retry or promised execution", out, retry)
				}
				s.taskSvc = task.NewService(store, s.projectSvc)
			}
			out := s.applyInboxReviewEvaluation(ctx, q, raw)
			after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
			if err != nil || after.Status == task.TaskStatusDone {
				t.Fatalf("code accepted without independent execution: %v %s", err, out)
			}
			launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, task.VerificationRequestID(d.ID, row.CommitSHA, d.LatestAttempt.ID)))
			if ready {
				if err != nil || launch == nil || launch.State != "queued" {
					t.Fatalf("first attempt did not schedule verifier: %v %+v %s", err, launch, out)
				}
				proof, err := task.VerificationIntentForLaunch(&after.Task, launch)
				if err != nil || proof == nil || proof.SourceAttemptID != d.LatestAttempt.ID || proof.SourceCommit != row.CommitSHA {
					t.Fatal("verifier lost exact source", err)
				}
				s.applyInboxReviewEvaluation(ctx, q, raw)
			} else if launch != nil {
				t.Fatal("unverified source admitted")
			}
			if len(after.Attempts) != 1 {
				t.Fatal("changed attempts before native claim")
			}
			if strings.Contains(scenario, "criterion pending") && (len(after.DoDChecklist) != 1 || after.DoDChecklist[0].Completed || after.DoDChecklist[0].ID != "independent-check") {
				t.Fatal("verification admission waived or replaced original criterion")
			}
		})
	}
}

func TestVerificationLaunchDoesNotWrapWritableOwnerContinuation(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "completed")
	ctx := context.Background()
	store := s.taskSvc.Store()
	if err := store.CreateTask(ctx, &task.Task{ID: "busy", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
		t.Fatal(err)
	}
	row, _ := store.GetTask(ctx, d.ID)
	row.ExecutionCategory = task.ExecutionCategoryCode
	row.Instructions += "\nDo not push. Do not deploy."
	row.CommitSHA = strings.Repeat("d", 40)
	if err := store.UpdateTask(ctx, row); err != nil {
		t.Fatal(err)
	}
	// Simulate an owner-approved writable continuation prior to the current attempt.
	ownerReqID := db.GenerateUUID()
	ownerMsgID := db.GenerateUUID()
	ownerAt := d.LatestAttempt.CreatedAt.Add(-2 * time.Minute)
	ownerReq := db.InboxRequest{OwnerID: q.OwnerID, AgentID: q.AgentID, RoomID: q.RoomID, RequestID: ownerReqID, MessageID: ownerMsgID, ReplyID: db.GenerateUUID(), State: "completed", CreatedAt: ownerAt, Deadline: ownerAt.Add(time.Hour)}
	if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, ownerReq, &db.ProjectMessage{ID: ownerMsgID, RoomID: q.RoomID, AuthorType: "user", Content: "Lanjut push ke main", CreatedAt: ownerAt}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{TaskID: d.ID, ProjectID: d.ProjectID, RequestID: ownerReqID, OwnerID: q.OwnerID, RoomID: q.RoomID, SourceRequestID: ownerReqID, AgentID: q.AgentID, InstructionsHash: task.LaunchInstructionsHash(row.Instructions), Revision: "Jalankan dalam mode eksekusi penuh (bukan read-only), lalu commit dan push ke origin/main.", CreatedAt: ownerAt, AvailableAt: ownerAt, ExpiresAt: ownerAt.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	_ = s.taskSvc.FinishTaskLaunches(ctx, d.ID, "stopped")
	stamp := time.Now()
	for _, event := range []struct{ kind, state string }{{"agent_report", "review"}, {"agent_result_kind", "implementation"}} {
		if err := store.CreateActivity(ctx, &task.TaskActivity{ID: event.kind, TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: event.kind, NewStatus: event.state, Message: "Current implementation evidence", CreatedAt: stamp}); err != nil {
			t.Fatal(err)
		}
	}
	rawCheck, _ := json.Marshal(task.RunnerVerificationRecord{Summary: "Project verification: npm run build (exit 0)"})
	if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "source-checks", TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: task.ActivityTypeRunnerVerification, Author: d.LatestAttempt.RunnerID, Message: string(rawCheck), CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	handoff, routed := s.inboxCodeVerificationHandoff(ctx, q)
	if !routed {
		t.Fatal("expected code verification handoff")
	}
	var eval inboxReviewEvaluation
	if err := json.Unmarshal([]byte(handoff), &eval); err != nil {
		t.Fatal(err)
	}
	before, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.saveReviewAssessment(ctx, before, q, eval); err != nil {
		t.Fatal(err)
	}
	notice, scheduled := s.startVerificationFollowup(ctx, q, before, eval)
	if !scheduled {
		t.Fatalf("expected verification followup to schedule: %s", notice)
	}
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, task.VerificationRequestID(d.ID, row.CommitSHA, d.LatestAttempt.ID)))
	if err != nil || launch == nil {
		t.Fatalf("expected verification launch: %v", err)
	}
	if strings.Contains(launch.Revision, "Binding approved continuation:") || strings.Contains(launch.Revision, "mode eksekusi penuh") {
		t.Fatalf("verification launch polluted by writable owner continuation: %s", launch.Revision)
	}
	if !strings.HasPrefix(launch.Revision, "Scope: read-only repository. Do not push. Do not deploy.\n"+task.VerificationIntentMarker) {
		t.Fatalf("verification launch lost read-only prefix: %s", launch.Revision)
	}
	policy := executionpolicy.FromInstructions(launch.Revision)
	if !policy.ReadOnlyRepository || !policy.DenyPush || !policy.DenyDeploy {
		t.Fatalf("verification launch lost read-only/no-push/no-deploy policy: %+v", policy)
	}
}

func TestIndependentCodeReviewMissing_SkipsDefaultProjectWithoutRepo(t *testing.T) {
	ctx := context.Background()
	s, _, _, _ := inboxFixture(t)
	projects := project.NewMemoryStore()
	if err := projects.CreateProject(ctx, &project.Project{ID: "default", Name: "Default"}); err != nil {
		t.Fatal(err)
	}
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	d := &task.TaskDetailResponse{
		Task: task.Task{
			ID:                "default-ops-task",
			ProjectID:         "default",
			Title:             "Optimasi & Pembersihan Ruang Penyimpanan",
			ExecutionCategory: task.ExecutionCategoryCode,
			Status:            task.TaskStatusReview,
		},
	}
	if s.independentCodeReviewMissing(ctx, d) {
		t.Fatal("expected independentCodeReviewMissing to return false for default project without repository")
	}
}
