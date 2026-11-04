package server

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

type reviewLaunchFailureStore struct {
	task.Store
	task.LaunchStore
	failure error
}

func (s reviewLaunchFailureStore) SaveLaunch(context.Context, task.TaskLaunch) (*task.TaskLaunch, error) {
	return nil, s.failure
}

func TestInboxReviewLaunchPersistenceFailureRetainsRetryWithoutFalsePromise(t *testing.T) {
	for _, scenario := range []string{"storage", "conflict", "missing task"} {
		t.Run(scenario, func(t *testing.T) {
			s, d, q := stalledReviewFixture(t, "completed")
			ctx := context.Background()
			underlying := s.taskSvc.Store()
			failure := errors.New("temporary launch storage failure")
			if scenario == "conflict" {
				failure = task.ErrRequestConflict
			}
			if scenario == "missing task" {
				failure = task.ErrTaskNotFound
			}
			failing := reviewLaunchFailureStore{Store: underlying, LaunchStore: underlying.(task.LaunchStore), failure: failure}
			s.taskSvc = task.NewService(failing, s.projectSvc)
			retry := false
			ctx = context.WithValue(ctx, inboxHandoffPersistenceKey{}, &retry)
			raw := `{"decision":"revise","explanation":"I will continue the work automatically.","revision":"Inspect the saved workspace and complete the missing acceptance evidence within approved scope."}`
			output := s.applyInboxReviewEvaluation(ctx, q, raw)
			if strings.Contains(output, "I will continue") || !strings.Contains(output, "launch could not be recorded") {
				t.Fatal("reply claimed continuation without a durable launch", output)
			}
			if retry != (scenario == "storage") {
				t.Fatal("incorrect persistence retry signal", scenario, retry)
			}
			if _, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID)); err == nil {
				t.Fatal("failed save left a launch")
			}
			if scenario != "storage" {
				return
			}
			// Re-evaluate the same saved review after storage recovers; a busy project lane
			// keeps it queued without starting an external provider in the test.
			s.taskSvc = task.NewService(underlying, s.projectSvc)
			if err := underlying.CreateTask(ctx, &task.Task{ID: "review-retry-busy", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
				t.Fatal(err)
			}
			retry = false
			s.applyInboxReviewEvaluation(ctx, q, raw)
			launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID))
			if err != nil || launch.State != "queued" || retry {
				t.Fatal("same review did not recover its durable launch", launch, err, retry)
			}
			s.applyInboxReviewEvaluation(ctx, q, raw)
			after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
			if err != nil || len(after.Attempts) != 1 {
				t.Fatal("review retry created another execution", err)
			}
		})
	}
}

func TestInboxReviewExhaustedBudgetCannotPromiseContinuation(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "completed")
	ctx := context.Background()
	for _, id := range []string{"older-revision-one", "older-revision-two"} {
		if err := s.taskSvc.Store().CreateAttempt(ctx, &task.TaskAttempt{ID: id, TaskID: d.ID, Status: task.AttemptStatusFailed, CreatedAt: d.LatestAttempt.CreatedAt.Add(-time.Minute)}); err != nil {
			t.Fatal(err)
		}
	}
	output := s.applyInboxReviewEvaluation(ctx, q, `{"decision":"revise","explanation":"Cuan akan melanjutkan perbaikan secara otomatis.","revision":"Complete the original checks."}`)
	if strings.Contains(output, "akan melanjutkan") || !strings.Contains(output, "Batas perbaikan otomatis sudah tercapai") {
		t.Fatal("exhausted budget retained a false continuation promise", output)
	}
	if _, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID)); err == nil {
		t.Fatal("budget exhaustion queued another execution")
	}
	after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil || after.Status != task.TaskStatusBlocked {
		t.Fatal("budget result not persisted", err)
	}
}

func TestInboxReviewLaunchStorageOutageKeepsOriginalRequestQueued(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "queued")
	s.inboxStopping.Store(false)
	ctx := context.Background()
	underlying := s.taskSvc.Store()
	s.taskSvc = task.NewService(reviewLaunchFailureStore{Store: underlying, LaunchStore: underlying.(task.LaunchStore), failure: errors.New("temporary launch write outage")}, s.projectSvc)
	review := `{"decision":"revise","explanation":"I will continue automatically.","revision":"Complete missing verification in the original workspace."}`
	response, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": review}})
	binary := filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nprintf '%s\\n' '"+string(response)+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	repo := s.projectStudioRepo.(db.InboxLifecycleRepository)
	claimed, ok, err := repo.ClaimInboxRequest(ctx, q.RoomID, q.RequestID)
	if err != nil || !ok {
		t.Fatal("could not claim review", err)
	}
	s.runInboxRequest(repo, claimed)
	requests, err := repo.ListInboxRequests(ctx, q.RoomID)
	if err != nil {
		t.Fatal(err)
	}
	var actual *db.InboxRequest
	for i := range requests {
		if requests[i].RequestID == q.RequestID {
			actual = &requests[i]
		}
	}
	if actual == nil || actual.State != "queued" || actual.SafeError != inboxHandoffPersistenceWaiting {
		t.Fatalf("review lost on launch failure: %+v", actual)
	}
	if !inboxCapacityWaiting(*actual, time.Now()) {
		t.Fatal("storage failure retries without backoff")
	}
	if reply, _ := s.projectStudioRepo.GetMessageByID(ctx, q.ReplyID); reply != nil {
		t.Fatal("premature final reply persisted", reply.Content)
	}
	after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil || len(after.Attempts) != 1 {
		t.Fatal("persistence retry executed another attempt", err)
	}
}

func TestInboxDeterministicExhaustedBudgetUsesConciseStoppedOutcome(t *testing.T) {
	code := 2
	latest := &task.TaskAttempt{ID: "failed", Status: task.AttemptStatusFailed, ExitCode: &code, ErrorMessage: "exit code 2"}
	d := &task.TaskDetailResponse{LatestAttempt: latest, Attempts: []*task.TaskAttempt{{Status: task.AttemptStatusFailed}, {Status: task.AttemptStatusFailed}, latest}}
	decision := evaluateTaskDoDDeterministic(d)
	if decision.Decision != "blocked" || decision.Explanation != inboxCorrectionLimitExplanation || decision.Revision != "" {
		t.Fatalf("fallback misrepresented the stopped outcome: %+v", decision)
	}
}
