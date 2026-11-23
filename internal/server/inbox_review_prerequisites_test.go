package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func prerequisiteReviewFixture(t *testing.T) (*Server, *task.TaskDetailResponse) {
	t.Helper()
	s, d, _ := stalledReviewFixture(t, "completed")
	projects := project.NewMemoryStore()
	if err := projects.CreateProject(context.Background(), &project.Project{ID: d.ProjectID, Name: "Default"}); err != nil {
		t.Fatal(err)
	}
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	ctx := context.Background()
	now := time.Now().UTC()
	if err := s.taskSvc.Store().CreateAttempt(ctx, &task.TaskAttempt{ID: "earlier-one", TaskID: d.ID, Status: task.AttemptStatusFailed, CreatedAt: d.LatestAttempt.CreatedAt.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	current := *d.LatestAttempt
	current.ID = "prerequisite-attempt"
	current.CreatedAt = now
	d.LatestAttempt = &current
	d.LatestAttempt.Status = task.AttemptStatusFailed
	d.LatestAttempt.FinishedAt = &now
	d.LatestAttempt.ErrorMessage = "Registered PHPUnit target or runtime is unavailable outside the private workspace"
	if err := s.taskSvc.Store().CreateAttempt(ctx, d.LatestAttempt); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(task.RunnerVerificationRecord{Summary: ""})
	for _, event := range []*task.TaskActivity{
		{ID: "native-final-report", TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: "agent_report", NewStatus: "review", Message: "Existing findings verified; no implementation changes needed.", CreatedAt: now},
		{ID: "verification:" + d.LatestAttempt.ID, TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: task.ActivityTypeRunnerVerification, Author: d.LatestAttempt.RunnerID, Message: string(raw), CreatedAt: now},
	} {
		if err := s.taskSvc.Store().CreateActivity(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	s.queueInboxTaskReview(ctx, after)
	return s, after
}

func TestNativePrerequisiteGuardRejectsModelPassBeforeSavingContinuation(t *testing.T) {
	s, d := prerequisiteReviewFixture(t)
	ctx := context.Background()
	requests := reviewRequests(t, s, d.SourceRoomID)
	q := requests[len(requests)-1]
	if err := s.taskSvc.Store().CreateTask(ctx, &task.Task{ID: "busy", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
		t.Fatal(err)
	}
	s.applyInboxReviewEvaluation(ctx, q, `{"decision":"pass","finding":"All done","explanation":"Accepted","revision":"","remaining_work":"none","requested_outcome":"completed"}`)
	after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	saved, ok := reviewAssessment(after, q.RequestID)
	if !ok || saved.Decision != "revise" || saved.RequestedOutcome != "unfinished" || saved.ManagerAction != "replan" || saved.ManagerReason == "" || !strings.Contains(saved.Revision, "Composer lock") {
		t.Fatal("stored pass lost native failure", saved)
	}
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID))
	if err != nil || launch == nil || launch.State != "queued" || launch.Revision != saved.Revision || after.Status == task.TaskStatusDone {
		t.Fatal("scoped recovery not persisted", err, launch, after.Status)
	}
	s.applyInboxReviewEvaluation(ctx, q, `{"decision":"pass","finding":"All done","explanation":"Accepted","revision":""}`)
	repeated, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	count := 0
	for _, e := range repeated.Activities {
		if e.ActivityType == reviewAssessmentActivity {
			count++
		}
	}
	if count != 1 || repeated.Status == task.TaskStatusDone {
		t.Fatal("retry changed or duplicated native assessment")
	}
}

func TestPrerequisiteGuardRequiresCurrentNativeCompletion(t *testing.T) {
	for _, scenario := range []string{"current", "old report", "wrong runner", "old attempt", "cancelled", "test assertion failure", "succeeded"} {
		t.Run(scenario, func(t *testing.T) {
			_, d := prerequisiteReviewFixture(t)
			for _, e := range d.Activities {
				if scenario == "old report" && e.ID == "native-final-report" {
					other := "older"
					e.AttemptID = &other
				}
				if e.ID == "verification:"+d.LatestAttempt.ID {
					if scenario == "wrong runner" {
						e.Author = "agent"
					}
					if scenario == "old attempt" {
						other := "older"
						e.AttemptID = &other
					}
				}
			}
			switch scenario {
			case "cancelled":
				d.LatestAttempt.CancelRequested = true
			case "test assertion failure":
				d.LatestAttempt.ErrorMessage = "Project verification failed"
			case "succeeded":
				d.LatestAttempt.Status = task.AttemptStatusSucceeded
			}
			decision := inboxReviewEvaluation{Decision: "pass", RemainingWork: "none"}
			got := requireNativePrerequisiteReview(d, decision)
			if (got.Decision == "revise") != (scenario == "current") {
				t.Fatal("untrusted state became prerequisite continuation", got)
			}
		})
	}
}

func TestPrerequisitePolicyRefreshesOnlyAffectedReviewDigest(t *testing.T) {
	_, d := prerequisiteReviewFixture(t)
	current := inboxReviewEvidenceDigest(d)
	d.LatestAttempt.ErrorMessage = "Project verification failed"
	previous := inboxReviewEvidenceDigest(d)
	if current == previous {
		t.Fatal("legacy pass assessment remained current after prerequisite policy change")
	}
	d.LatestAttempt.ErrorMessage = "Another ordinary failure"
	if inboxReviewEvidenceDigest(d) != previous {
		t.Fatal("unaffected failure gained a policy replay")
	}
}
