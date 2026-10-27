package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestRunProjectVerificationUsesRepositoryVerifier(t *testing.T) {
	root := t.TempDir()
	testDir := filepath.Join(root, "scripts")
	if err := os.Mkdir(testDir, 0700); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(testDir, "test.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'verification-pass\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	result := runProjectVerification(context.Background(), root)
	if !result.Passed || !strings.Contains(result.Summary, "verification-pass") {
		t.Fatalf("unexpected verification result: %+v", result)
	}
}

func TestRunner_Metrics(t *testing.T) {
	metrics := CollectHostMetrics()
	t.Logf("Host metrics: CPU=%.2f%%, MemUsed=%d bytes, MemTotal=%d bytes, DiskUsed=%d bytes, DiskTotal=%d bytes",
		metrics.CPUPercent, metrics.MemoryUsedBytes, metrics.MemoryTotalBytes, metrics.DiskUsedBytes, metrics.DiskTotalBytes)

	if metrics.MemoryTotalBytes <= 0 {
		t.Errorf("Expected positive memory total, got %d", metrics.MemoryTotalBytes)
	}
	if metrics.DiskTotalBytes <= 0 {
		t.Errorf("Expected positive disk total, got %d", metrics.DiskTotalBytes)
	}
}

func TestRunner_QueueAndConcurrencyLimit(t *testing.T) {
	taskStore := task.NewMemoryStore()
	runnerStore := NewMemoryStore(taskStore)
	ctx := context.Background()

	// Create 2 tasks
	t1 := &task.Task{
		ID:        "task_queue_1",
		Title:     "First Task in Queue",
		Status:    task.TaskStatusBacklog,
		CreatedAt: time.Now().UTC().Add(-10 * time.Minute),
	}
	t2 := &task.Task{
		ID:        "task_queue_2",
		Title:     "Second Task in Queue",
		Status:    task.TaskStatusBacklog,
		CreatedAt: time.Now().UTC().Add(-5 * time.Minute),
	}
	_ = taskStore.CreateTask(ctx, t1)
	_ = taskStore.CreateTask(ctx, t2)

	// Create 2 queued attempts
	a1 := &task.TaskAttempt{
		ID:        "att_q1",
		TaskID:    t1.ID,
		Status:    task.AttemptStatusQueued,
		CreatedAt: t1.CreatedAt,
	}
	a2 := &task.TaskAttempt{
		ID:        "att_q2",
		TaskID:    t2.ID,
		Status:    task.AttemptStatusQueued,
		CreatedAt: t2.CreatedAt,
	}
	_ = taskStore.CreateAttempt(ctx, a1)
	_ = taskStore.CreateAttempt(ctx, a2)

	runnerID := "runner_test_01"

	// 1. Runner 01 claims first task
	claimedAttempt, claimedTask, err := runnerStore.ClaimNextQueuedAttempt(ctx, runnerID, []string{"agy"}, 10*time.Second)
	if err != nil {
		t.Fatalf("Claim failed: %v", err)
	}
	if claimedAttempt == nil || claimedTask == nil {
		t.Fatalf("Expected task to be claimed")
	}
	if claimedAttempt.ID != a1.ID {
		t.Fatalf("Expected FIFO oldest attempt %s, got %s", a1.ID, claimedAttempt.ID)
	}
	if claimedAttempt.Status != task.AttemptStatusRunning {
		t.Errorf("Expected claimed attempt status 'running', got '%s'", claimedAttempt.Status)
	}
	t.Logf("Runner %s successfully claimed oldest attempt %s (Task: '%s')", runnerID, claimedAttempt.ID, claimedTask.Title)

	// 2. Concurrency Limit Check: Runner 01 tries to claim again while busy
	secondClaim, _, err := runnerStore.ClaimNextQueuedAttempt(ctx, runnerID, []string{"agy"}, 10*time.Second)
	if err != nil {
		t.Fatalf("Second claim error: %v", err)
	}
	if secondClaim != nil {
		t.Fatalf("Concurrency limit violated! Runner claimed second task while already running %s", claimedAttempt.ID)
	}
	t.Logf("Concurrency limit verified: runner %s cannot claim a second task while busy", runnerID)

	// 3. Verify second attempt remains safely queued in DB
	att2, err := taskStore.GetAttempt(ctx, a2.ID)
	if err != nil {
		t.Fatalf("Get attempt 2 failed: %v", err)
	}
	if att2.Status != task.AttemptStatusQueued {
		t.Errorf("Expected attempt 2 to remain 'queued', got '%s'", att2.Status)
	}
	t.Logf("Second task attempt %s remains queued waiting for next runner/turn", att2.ID)
}

func TestRunner_DuplicateClaimProtection(t *testing.T) {
	taskStore := task.NewMemoryStore()
	runnerStore := NewMemoryStore(taskStore)
	ctx := context.Background()

	// 1 single task and attempt queued
	t1 := &task.Task{
		ID:        "task_race_1",
		Title:     "High Concurrency Race Task",
		Status:    task.TaskStatusBacklog,
		CreatedAt: time.Now().UTC(),
	}
	_ = taskStore.CreateTask(ctx, t1)

	att1 := &task.TaskAttempt{
		ID:        "att_race_1",
		TaskID:    t1.ID,
		Status:    task.AttemptStatusQueued,
		CreatedAt: t1.CreatedAt,
	}
	_ = taskStore.CreateAttempt(ctx, att1)

	// Concurrently attempt to claim with 10 different runners
	concurrency := 10
	var wg sync.WaitGroup
	var successfulClaims int32
	var winnerRunner string
	var winnerMu sync.Mutex

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		runnerID := fmt.Sprintf("runner_race_%02d", i)
		go func(rID string) {
			defer wg.Done()
			claimedAtt, _, err := runnerStore.ClaimNextQueuedAttempt(ctx, rID, nil, 15*time.Second)
			if err == nil && claimedAtt != nil {
				atomic.AddInt32(&successfulClaims, 1)
				winnerMu.Lock()
				winnerRunner = rID
				winnerMu.Unlock()
			}
		}(runnerID)
	}

	wg.Wait()

	if successfulClaims != 1 {
		t.Fatalf("Duplicate-claim protection failed! Expected exactly 1 claim, but got %d claims", successfulClaims)
	}
	t.Logf("Duplicate-claim test PASSED: Exactly 1 runner ('%s') claimed attempt %s out of %d concurrent runner attempts.",
		winnerRunner, att1.ID, concurrency)
}

func TestRunner_LeaseExpirationAndInterruptedRecovery(t *testing.T) {
	taskStore := task.NewMemoryStore()
	runnerStore := NewMemoryStore(taskStore)
	ctx := context.Background()

	t1 := &task.Task{
		ID:        "task_lease_1",
		Title:     "Lease Expiration Task",
		Status:    task.TaskStatusDoing,
		CreatedAt: time.Now().UTC(),
	}
	_ = taskStore.CreateTask(ctx, t1)

	expiredLease := time.Now().UTC().Add(-10 * time.Second) // expired 10s ago
	att1 := &task.TaskAttempt{
		ID:             "att_lease_1",
		TaskID:         t1.ID,
		RunnerID:       "runner_disconnected",
		Status:         task.AttemptStatusRunning,
		LeaseExpiresAt: &expiredLease,
		CreatedAt:      time.Now().UTC().Add(-30 * time.Second),
	}
	_ = taskStore.CreateAttempt(ctx, att1)

	// Reconcile expired leases
	interrupted, err := runnerStore.MarkInterruptedLeases(ctx)
	if err != nil {
		t.Fatalf("MarkInterruptedLeases failed: %v", err)
	}

	if len(interrupted) != 1 {
		t.Fatalf("Expected 1 interrupted attempt, got %d", len(interrupted))
	}

	resAtt, _ := taskStore.GetAttempt(ctx, att1.ID)
	if resAtt.Status != task.AttemptStatusInterrupted {
		t.Errorf("Expected attempt status 'interrupted', got '%s'", resAtt.Status)
	}

	// Verify task is now in review / recoverable status
	resTask, _ := taskStore.GetTask(ctx, t1.ID)
	if resTask.Status != task.TaskStatusReview {
		t.Errorf("Expected task status 'review', got '%s'", resTask.Status)
	}

	// Verify no automatic rerun occurred (no new attempt created)
	attempts, _ := taskStore.ListAttempts(ctx, t1.ID)
	if len(attempts) != 1 {
		t.Errorf("CRITICAL DoD VIOLATION: auto-rerun occurred without explicit human trigger! Attempts count: %d", len(attempts))
	}

	t.Logf("Disconnect/Lease expiration test PASSED: attempt marked '%s', task marked '%s', zero auto-rerun.",
		resAtt.Status, resTask.Status)
}

func TestRunner_ProjectConcurrencyLock(t *testing.T) {
	taskStore := task.NewMemoryStore()
	runnerStore := NewMemoryStore(taskStore)
	ctx := context.Background()

	projectID := "proj_strict_lock_test"

	// Task 1 for projectID
	t1 := &task.Task{
		ID:        "task_proj_1",
		ProjectID: projectID,
		Title:     "Project Task 1",
		Status:    task.TaskStatusBacklog,
		CreatedAt: time.Now().UTC().Add(-10 * time.Minute),
	}
	_ = taskStore.CreateTask(ctx, t1)
	a1 := &task.TaskAttempt{
		ID:        "att_p1",
		TaskID:    t1.ID,
		Status:    task.AttemptStatusQueued,
		CreatedAt: t1.CreatedAt,
	}
	_ = taskStore.CreateAttempt(ctx, a1)

	// Task 2 for same projectID
	t2 := &task.Task{
		ID:        "task_proj_2",
		ProjectID: projectID,
		Title:     "Project Task 2",
		Status:    task.TaskStatusBacklog,
		CreatedAt: time.Now().UTC().Add(-5 * time.Minute),
	}
	_ = taskStore.CreateTask(ctx, t2)
	a2 := &task.TaskAttempt{
		ID:        "att_p2",
		TaskID:    t2.ID,
		Status:    task.AttemptStatusQueued,
		CreatedAt: t2.CreatedAt,
	}
	_ = taskStore.CreateAttempt(ctx, a2)

	runner1 := "runner_p1"
	runner2 := "runner_p2"

	// 1. Runner 1 claims task 1
	claimed1, _, err := runnerStore.ClaimNextQueuedAttempt(ctx, runner1, []string{"agy"}, 10*time.Second)
	if err != nil || claimed1 == nil || claimed1.ID != a1.ID {
		t.Fatalf("Runner 1 failed to claim task 1: %v", err)
	}
	t.Logf("Runner 1 successfully claimed Task 1 (attempt: %s)", claimed1.ID)

	// 2. Runner 2 tries to claim next queued task for the same project while Runner 1 is executing
	claimed2, _, err := runnerStore.ClaimNextQueuedAttempt(ctx, runner2, []string{"agy"}, 10*time.Second)
	if err != nil {
		t.Fatalf("Runner 2 claim returned error: %v", err)
	}
	if claimed2 != nil {
		t.Fatalf("CRITICAL CONCURRENCY VIOLATION: Runner 2 claimed task %s while project %s was locked by runner 1!", claimed2.TaskID, projectID)
	}
	t.Logf("Strict Project Concurrency Lock verified: Runner 2 safely blocked from claiming Task 2 while project %s is active", projectID)

	// 3. Complete Task 1
	claimed1.Status = task.AttemptStatusSucceeded
	_ = taskStore.UpdateAttempt(ctx, claimed1)
	doneStatus := task.TaskStatusDone
	_ = taskStore.UpdateTask(ctx, &task.Task{ID: t1.ID, ProjectID: projectID, Status: doneStatus})

	// 4. Runner 2 tries again now that project lock is released
	claimed2After, _, err := runnerStore.ClaimNextQueuedAttempt(ctx, runner2, []string{"agy"}, 10*time.Second)
	if err != nil {
		t.Fatalf("Runner 2 second claim returned error: %v", err)
	}
	if claimed2After == nil || claimed2After.ID != a2.ID {
		t.Fatalf("Expected Runner 2 to claim Task 2 after Task 1 completed, got: %+v", claimed2After)
	}
	t.Logf("Project Concurrency Lock handover verified: Runner 2 successfully claimed Task 2 (attempt: %s) after Task 1 completed", claimed2After.ID)
}

type mockProjectFinder struct {
	projects map[string]*project.ProjectResponse
}

func (m *mockProjectFinder) Get(ctx context.Context, id string) (*project.ProjectResponse, error) {
	if p, ok := m.projects[id]; ok {
		return p, nil
	}
	return nil, nil
}

func TestRunner_AutoDeployEnforcement(t *testing.T) {
	ctx := context.Background()
	taskStore := task.NewMemoryStore()
	runnerStore := NewMemoryStore(taskStore)

	finder := &mockProjectFinder{
		projects: map[string]*project.ProjectResponse{
			"proj_autodeploy": {
				ID:       "proj_autodeploy",
				Metadata: json.RawMessage(`{"cuan_settings":{"auto_deploy":true}}`),
			},
		},
	}
	svc := NewService(runnerStore, taskStore, finder)

	// Create task and attempt
	tk := &task.Task{
		ID:        "task_ad_1",
		ProjectID: "proj_autodeploy",
		Title:     "Feature needing deployment",
		Status:    task.TaskStatusDoing,
	}
	_ = taskStore.CreateTask(ctx, tk)
	att := &task.TaskAttempt{
		ID:     "att_ad_1",
		TaskID: tk.ID,
		Status: task.AttemptStatusRunning,
	}
	_ = taskStore.CreateAttempt(ctx, att)

	// Attempt completed as Succeeded without live deployment evidence
	req := CompleteAttemptRequest{
		RunnerID:             "runner_1",
		Status:               task.AttemptStatusSucceeded,
		ExitCode:             0,
		AgentReportConfirmed: true,
		AgentReportRequired:  true,
		CommitSHA:            "abc1234",
		BranchName:           "cuan/task-ad1",
	}
	completedAtt, err := svc.Complete(ctx, att.ID, req)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if completedAtt.Status != task.AttemptStatusFailed {
		t.Fatalf("Expected attempt to fail auto_deploy compliance, got status '%s'", completedAtt.Status)
	}
	if !strings.Contains(completedAtt.ErrorMessage, "auto_deploy") {
		t.Fatalf("Expected auto_deploy error message, got: %s", completedAtt.ErrorMessage)
	}

	// Now with live deployment confirmed
	att2 := &task.TaskAttempt{
		ID:     "att_ad_2",
		TaskID: tk.ID,
		Status: task.AttemptStatusRunning,
	}
	_ = taskStore.CreateAttempt(ctx, att2)
	req2 := CompleteAttemptRequest{
		RunnerID:                "runner_1",
		Status:                  task.AttemptStatusSucceeded,
		ExitCode:                0,
		AgentReportConfirmed:    true,
		AgentReportRequired:     true,
		CommitSHA:               "abc1234",
		BranchName:              "cuan/task-ad1",
		LiveDeploymentConfirmed: true,
	}
	completedAtt2, err := svc.Complete(ctx, att2.ID, req2)
	if err != nil {
		t.Fatalf("Complete 2 failed: %v", err)
	}
	if completedAtt2.Status != task.AttemptStatusSucceeded {
		t.Fatalf("Expected attempt to succeed with live deployment confirmed, got: %s (err: %s)", completedAtt2.Status, completedAtt2.ErrorMessage)
	}
}
