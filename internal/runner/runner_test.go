package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
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
	if resTask.Status != task.TaskStatusBlocked {
		t.Errorf("Expected task status 'blocked', got '%s'", resTask.Status)
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

func (m *mockProjectFinder) TaskContext(ctx context.Context, id, instructions string) (*project.ProjectResponse, error) {
	return m.Get(ctx, id)
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
		ID: "att_ad_1", RunnerID: "runner_1",
		TaskID: tk.ID,
		Status: task.AttemptStatusRunning,
	}
	_ = taskStore.CreateAttempt(ctx, att)

	// Attempt completed as Succeeded without live deployment evidence
	changed, exempt := true, false
	req := CompleteAttemptRequest{
		RunnerID:                "runner_1",
		Status:                  task.AttemptStatusSucceeded,
		ExitCode:                0,
		AgentReportConfirmed:    true,
		AgentReportRequired:     true,
		CommitSHA:               "abc1234",
		BranchName:              "cuan/task-ad1",
		HasCodeChanges:          &changed,
		RequiresDeploy:          &exempt,
		LiveDeploymentConfirmed: true,
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

	// Now with live deployment confirmed; a new claim returns the board to doing.
	tk.Status = task.TaskStatusDoing
	tk.UpdatedAt = time.Now().UTC()
	taskStore.UpdateTask(ctx, tk)
	att2 := &task.TaskAttempt{
		ID: "att_ad_2", RunnerID: "runner_1",
		TaskID:    tk.ID,
		Status:    task.AttemptStatusRunning,
		CreatedAt: time.Now(),
	}
	_ = taskStore.CreateAttempt(ctx, att2)
	req2 := CompleteAttemptRequest{
		RunnerID:                "runner_1",
		Status:                  task.AttemptStatusSucceeded,
		ExitCode:                0,
		AgentReportConfirmed:    true,
		AgentReportRequired:     true,
		CommitSHA:               strings.Repeat("a", 40),
		BranchName:              "cuan/task-ad1",
		LiveDeploymentConfirmed: true,
	}
	receipt, _ := json.Marshal(task.DeploymentReceipt{Version: 1, TaskID: tk.ID, AttemptID: att2.ID, CommitSHA: req2.CommitSHA, ReleaseSHA: req2.CommitSHA, Environment: "production", Health: "healthy"})
	if err := taskStore.CreateActivity(ctx, &task.TaskActivity{ID: task.DeploymentReceiptID(tk.ID, att2.ID, req2.CommitSHA), TaskID: tk.ID, AttemptID: &att2.ID, ActivityType: "deployment_verified", Author: "cuan-deploy", Message: string(receipt), CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	completedAtt2, err := svc.Complete(ctx, att2.ID, req2)
	if err != nil {
		t.Fatalf("Complete 2 failed: %v", err)
	}
	if completedAtt2.Status != task.AttemptStatusSucceeded {
		t.Fatalf("Expected attempt to succeed with live deployment confirmed, got: %s (err: %s)", completedAtt2.Status, completedAtt2.ErrorMessage)
	}
}

func TestRunner_AutoDeployCompliance_NonCodeInvestigation_SucceedsWithoutDeploy(t *testing.T) {
	ctx := context.Background()
	taskStore := task.NewMemoryStore()
	runnerStore := NewMemoryStore(taskStore)

	finder := &mockProjectFinder{
		projects: map[string]*project.ProjectResponse{
			"proj_autodeploy_investigation": {
				ID:       "proj_autodeploy_investigation",
				Metadata: json.RawMessage(`{"cuan_settings":{"auto_deploy":true}}`),
			},
		},
	}
	svc := NewService(runnerStore, taskStore, finder)

	// Investigation task without code modifications
	tk := &task.Task{
		ID:                "task_investigation_1",
		ProjectID:         "proj_autodeploy_investigation",
		Title:             "Investigasi kegagalan integrasi pihak ketiga",
		Instructions:      "Lakukan analisis log dan RCA penyebab kegagalan pembayaran",
		Summary:           "RCA teknis selesai: timeout koneksi dari payment gateway",
		CompleteNote:      "Hasil investigasi dan draf solusi untuk pelanggan telah siap.",
		ExecutionCategory: task.ExecutionCategoryReadOnly,
		Status:            task.TaskStatusDoing,
	}
	_ = taskStore.CreateTask(ctx, tk)

	att := &task.TaskAttempt{
		ID: "att_inv_1", RunnerID: "runner_1",
		TaskID: tk.ID,
		Status: task.AttemptStatusRunning,
	}
	_ = taskStore.CreateAttempt(ctx, att)

	noDeploy := false
	req := CompleteAttemptRequest{
		RunnerID:             "runner_1",
		Status:               task.AttemptStatusSucceeded,
		ExitCode:             0,
		AgentReportConfirmed: true,
		AgentReportRequired:  true,
		RequiresDeploy:       &noDeploy,
		DiffSummary:          "",
		CommitSHA:            "",
	}

	completedAtt, err := svc.Complete(ctx, att.ID, req)
	if err != nil {
		t.Fatalf("Complete investigation attempt failed: %v", err)
	}

	// Smart Compliance Gate: Must succeed without live deployment requirement
	if completedAtt.Status != task.AttemptStatusSucceeded {
		t.Fatalf("Expected investigation attempt to succeed without deploy, got status '%s' (err: %s)", completedAtt.Status, completedAtt.ErrorMessage)
	}

	// Verify task is marked done
	updatedTask, err := taskStore.GetTask(ctx, tk.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if updatedTask.Status != task.TaskStatusDone && updatedTask.Status != task.TaskStatusReview {
		t.Fatalf("Expected task status 'done' or 'review', got '%s'", updatedTask.Status)
	}
}

func TestRunner_MetadataInference(t *testing.T) {
	// Test inference of execution categories
	tests := []struct {
		title        string
		instructions string
		expected     string
	}{
		{"Build user registration API", "Implement new auth flow with JWT", task.ExecutionCategoryCode},
		{"Grant access to admin", "grant access to user test@example.com for finance role", task.ExecutionCategoryOps},
		{"Run artisan cache clear", "Execute php artisan cache:clear and route:clear", task.ExecutionCategoryOps},
		{"Sync contacts data", "Trigger contact sync from CRM endpoint", task.ExecutionCategoryOps},
		{"Audit user access logs", "Inspect and audit authentication telemetry logs", task.ExecutionCategoryReadOnly},
		{"Query system performance", "Review database query status and telemetry", task.ExecutionCategoryReadOnly},
		{"Database migration for payments", "Run migration to alter table orders add column status", task.ExecutionCategoryCode},
		{"Integrasi Renderer Diagram Mermaid Visual di Chat Cuan", "Integrasi pustaka mermaid.js pada MarkdownRenderer dengan fitur view interaktif", task.ExecutionCategoryCode},
	}

	for _, tc := range tests {
		cat := task.InferExecutionCategory(tc.title, tc.instructions)
		if cat != tc.expected {
			t.Errorf("InferExecutionCategory(%q, %q) = %q; expected %q", tc.title, tc.instructions, cat, tc.expected)
		}
	}
}

func TestRunner_MultiLaneQueue_Concurrency(t *testing.T) {
	ctx := context.Background()
	taskStore := task.NewMemoryStore()
	runnerStore := NewMemoryStore(taskStore)
	projectID := "proj_multilane_test"

	// 1. Setup Task 1: CODE task (e.g. coding / build)
	tCode1 := &task.Task{
		ID:                "task_code_1",
		ProjectID:         projectID,
		Title:             "Implement Payment Feature",
		Instructions:      "Build payment gateway integration",
		ExecutionCategory: task.ExecutionCategoryCode,
		Status:            task.TaskStatusBacklog,
		CreatedAt:         time.Now().UTC().Add(-10 * time.Minute),
	}
	_ = taskStore.CreateTask(ctx, tCode1)
	aCode1 := &task.TaskAttempt{
		ID:        "att_code_1",
		TaskID:    tCode1.ID,
		Status:    task.AttemptStatusQueued,
		CreatedAt: tCode1.CreatedAt,
	}
	_ = taskStore.CreateAttempt(ctx, aCode1)

	// 2. Setup Task 2: OPS task (e.g. artisan command / grant access / contact sync)
	tOps1 := &task.Task{
		ID:                "task_ops_1",
		ProjectID:         projectID,
		Title:             "Grant Access to Accountant",
		Instructions:      "grant access to account finance@example.com",
		ExecutionCategory: task.ExecutionCategoryOps,
		Status:            task.TaskStatusBacklog,
		CreatedAt:         time.Now().UTC().Add(-8 * time.Minute),
	}
	_ = taskStore.CreateTask(ctx, tOps1)
	aOps1 := &task.TaskAttempt{
		ID:        "att_ops_1",
		TaskID:    tOps1.ID,
		Status:    task.AttemptStatusQueued,
		CreatedAt: tOps1.CreatedAt,
	}
	_ = taskStore.CreateAttempt(ctx, aOps1)

	// 3. Setup Task 3: READONLY task (e.g. audit / analysis)
	tRead1 := &task.Task{
		ID:                "task_read_1",
		ProjectID:         projectID,
		Title:             "Audit Authentication Logs",
		Instructions:      "audit and review telemetry logs",
		ExecutionCategory: task.ExecutionCategoryReadOnly,
		Status:            task.TaskStatusBacklog,
		CreatedAt:         time.Now().UTC().Add(-6 * time.Minute),
	}
	_ = taskStore.CreateTask(ctx, tRead1)
	aRead1 := &task.TaskAttempt{
		ID:        "att_read_1",
		TaskID:    tRead1.ID,
		Status:    task.AttemptStatusQueued,
		CreatedAt: tRead1.CreatedAt,
	}
	_ = taskStore.CreateAttempt(ctx, aRead1)

	// 4. Setup Task 4: Second CODE task for same project (to verify git deployment lock)
	tCode2 := &task.Task{
		ID:                "task_code_2",
		ProjectID:         projectID,
		Title:             "Second Feature Coding",
		Instructions:      "Build another feature",
		ExecutionCategory: task.ExecutionCategoryCode,
		Status:            task.TaskStatusBacklog,
		CreatedAt:         time.Now().UTC().Add(-4 * time.Minute),
	}
	_ = taskStore.CreateTask(ctx, tCode2)
	aCode2 := &task.TaskAttempt{
		ID:        "att_code_2",
		TaskID:    tCode2.ID,
		Status:    task.AttemptStatusQueued,
		CreatedAt: tCode2.CreatedAt,
	}
	_ = taskStore.CreateAttempt(ctx, aCode2)

	// 5. Setup Task 5: Second OPS task for same project (to verify operational lock serialization)
	tOps2 := &task.Task{
		ID:                "task_ops_2",
		ProjectID:         projectID,
		Title:             "Sync Contacts CRM",
		Instructions:      "php artisan contacts:sync",
		ExecutionCategory: task.ExecutionCategoryOps,
		Status:            task.TaskStatusBacklog,
		CreatedAt:         time.Now().UTC().Add(-2 * time.Minute),
	}
	_ = taskStore.CreateTask(ctx, tOps2)
	aOps2 := &task.TaskAttempt{
		ID:        "att_ops_2",
		TaskID:    tOps2.ID,
		Status:    task.AttemptStatusQueued,
		CreatedAt: tOps2.CreatedAt,
	}
	_ = taskStore.CreateAttempt(ctx, aOps2)

	// --- VERIFICATION STEP A: Code Task 1 is claimed by Runner A ---
	runnerA := "runner_node_a"
	claimedCode1, taskC1, err := runnerStore.ClaimNextQueuedAttempt(ctx, runnerA, []string{"agy"}, 10*time.Second)
	if err != nil || claimedCode1 == nil || claimedCode1.ID != aCode1.ID {
		t.Fatalf("Runner A failed to claim Code Task 1: %v", err)
	}
	if taskC1.ExecutionCategory != task.ExecutionCategoryCode {
		t.Fatalf("Expected category 'code', got '%s'", taskC1.ExecutionCategory)
	}
	t.Logf("✓ Code Task 1 (%s) claimed by Runner A (Git deployment lock held)", claimedCode1.ID)

	// --- VERIFICATION STEP B: Runner B claims OPS Task 1 concurrently (Non-Blocking Ops Lane!) ---
	runnerB := "runner_node_b"
	claimedOps1, taskO1, err := runnerStore.ClaimNextQueuedAttempt(ctx, runnerB, []string{"agy"}, 10*time.Second)
	if err != nil || claimedOps1 == nil {
		t.Fatalf("CRITICAL: Ops task was blocked by running code task! err: %v", err)
	}
	if claimedOps1.ID != aOps1.ID {
		t.Fatalf("Expected oldest ops task %s, got %s", aOps1.ID, claimedOps1.ID)
	}
	if taskO1.ExecutionCategory != task.ExecutionCategoryOps {
		t.Fatalf("Expected category 'ops', got '%s'", taskO1.ExecutionCategory)
	}
	t.Logf("✓ Non-Blocking Ops Lane Verified: Ops Task 1 (%s) claimed and running PARALLEL with Code Task 1!", claimedOps1.ID)

	// --- VERIFICATION STEP C: Runner C claims READONLY Task 1 concurrently (Shared Reader Pool!) ---
	runnerC := "runner_node_c"
	claimedRead1, taskR1, err := runnerStore.ClaimNextQueuedAttempt(ctx, runnerC, []string{"agy"}, 10*time.Second)
	if err != nil || claimedRead1 == nil {
		t.Fatalf("CRITICAL: Readonly task was blocked! err: %v", err)
	}
	if claimedRead1.ID != aRead1.ID {
		t.Fatalf("Expected oldest readonly task %s, got %s", aRead1.ID, claimedRead1.ID)
	}
	if taskR1.ExecutionCategory != task.ExecutionCategoryReadOnly {
		t.Fatalf("Expected category 'readonly', got '%s'", taskR1.ExecutionCategory)
	}
	t.Logf("✓ Shared Reader Pool Verified: Readonly Task 1 (%s) claimed and running PARALLEL with Code & Ops!", claimedRead1.ID)

	// --- VERIFICATION STEP D: Git Deployment Lock strictly blocks second Code Task ---
	runnerD := "runner_node_d"
	claimedCode2, _, err := runnerStore.ClaimNextQueuedAttempt(ctx, runnerD, []string{"agy"}, 10*time.Second, []string{task.ExecutionCategoryCode})
	if err != nil {
		t.Fatalf("Runner D claim error: %v", err)
	}
	if claimedCode2 != nil {
		t.Fatalf("CRITICAL CONCURRENCY VIOLATION: Second code task was claimed while another code task is running!")
	}
	t.Logf("✓ Git Deployment Lock Verified: Second code task blocked from collision while Code Task 1 is active")

	// --- VERIFICATION STEP E: Operational Lock strictly blocks second Ops Task (Protects DB mutations) ---
	claimedOps2, _, err := runnerStore.ClaimNextQueuedAttempt(ctx, runnerD, []string{"agy"}, 10*time.Second, []string{task.ExecutionCategoryOps})
	if err != nil {
		t.Fatalf("Runner D claim error: %v", err)
	}
	if claimedOps2 != nil {
		t.Fatalf("CRITICAL CONCURRENCY VIOLATION: Second ops task was claimed while Ops Task 1 is active (DB mutation race condition)!")
	}
	t.Logf("✓ Operational Mutation Lock Verified: Second ops task safely queued to prevent concurrent DB mutations")

	// --- VERIFICATION STEP F: Finish Ops Task 1 -> Handover to second Ops Task ---
	claimedOps1.Status = task.AttemptStatusSucceeded
	_ = taskStore.UpdateAttempt(ctx, claimedOps1)
	doneStatus := task.TaskStatusDone
	_ = taskStore.UpdateTask(ctx, &task.Task{ID: tOps1.ID, ProjectID: projectID, ExecutionCategory: task.ExecutionCategoryOps, Status: doneStatus})

	claimedOps2After, _, err := runnerStore.ClaimNextQueuedAttempt(ctx, runnerD, []string{"agy"}, 10*time.Second, []string{task.ExecutionCategoryOps})
	if err != nil || claimedOps2After == nil || claimedOps2After.ID != aOps2.ID {
		t.Fatalf("Expected Runner D to claim Ops Task 2 after Ops Task 1 completed, got: %+v (err: %v)", claimedOps2After, err)
	}
	t.Logf("✓ Operational Lock Handover Verified: Ops Task 2 claimed successfully after Ops Task 1 finished")

	// --- VERIFICATION STEP G: Schema Migration Exclusivity Barrier ---
	// Create an active schema migration task
	tMig := &task.Task{
		ID:                "task_mig_1",
		ProjectID:         projectID,
		Title:             "Database Schema Migration",
		Instructions:      "Run migration: alter table users add column phone_number text",
		ExecutionCategory: task.ExecutionCategoryCode,
		Status:            task.TaskStatusDoing,
	}
	_ = taskStore.CreateTask(ctx, tMig)
	aMig := &task.TaskAttempt{
		ID:        "att_mig_1",
		TaskID:    tMig.ID,
		RunnerID:  "runner_mig",
		Status:    task.AttemptStatusRunning,
		CreatedAt: time.Now().UTC(),
	}
	_ = taskStore.CreateAttempt(ctx, aMig)

	// Queue a new Ops task
	tOps3 := &task.Task{
		ID:                "task_ops_3",
		ProjectID:         projectID,
		Title:             "Artisan Command with Active Migration",
		Instructions:      "php artisan cache:clear",
		ExecutionCategory: task.ExecutionCategoryOps,
		Status:            task.TaskStatusBacklog,
	}
	_ = taskStore.CreateTask(ctx, tOps3)
	aOps3 := &task.TaskAttempt{
		ID:        "att_ops_3",
		TaskID:    tOps3.ID,
		Status:    task.AttemptStatusQueued,
		CreatedAt: time.Now().UTC(),
	}
	_ = taskStore.CreateAttempt(ctx, aOps3)

	runnerE := "runner_node_e"
	claimedDuringMig, _, err := runnerStore.ClaimNextQueuedAttempt(ctx, runnerE, []string{"agy"}, 10*time.Second, []string{task.ExecutionCategoryOps})
	if err != nil {
		t.Fatalf("Claim check during migration error: %v", err)
	}
	if claimedDuringMig != nil {
		t.Fatalf("CRITICAL VIOLATION: Ops task was claimed during active database schema migration!")
	}
	t.Logf("✓ Schema Migration Barrier Verified: Ops tasks held safely during active database schema migration")

	// Complete migration
	aMig.Status = task.AttemptStatusSucceeded
	_ = taskStore.UpdateAttempt(ctx, aMig)
	_ = taskStore.UpdateTask(ctx, &task.Task{ID: tMig.ID, ProjectID: projectID, Status: task.TaskStatusDone})

	// Also finish Ops Task 2 so ops lane is clear
	claimedOps2After.Status = task.AttemptStatusSucceeded
	_ = taskStore.UpdateAttempt(ctx, claimedOps2After)
	_ = taskStore.UpdateTask(ctx, &task.Task{ID: tOps2.ID, ProjectID: projectID, ExecutionCategory: task.ExecutionCategoryOps, Status: task.TaskStatusDone})

	claimedAfterMig, _, err := runnerStore.ClaimNextQueuedAttempt(ctx, runnerE, []string{"agy"}, 10*time.Second, []string{task.ExecutionCategoryOps})
	if err != nil || claimedAfterMig == nil || claimedAfterMig.ID != aOps3.ID {
		t.Fatalf("Expected Ops Task 3 to be claimed after schema migration completed, got: %+v (err: %v)", claimedAfterMig, err)
	}
	t.Logf("✓ Migration Resume Verified: Ops Task 3 proceeded automatically after migration finished")
}

type blockingMockAdapter struct {
	executingCode chan struct{}
	executingOps  chan struct{}
	releaseCode   chan struct{}
	releaseOps    chan struct{}
}

func (b *blockingMockAdapter) Name() string             { return "agy" }
func (b *blockingMockAdapter) Version() (string, error) { return "1.0.0", nil }
func (b *blockingMockAdapter) Execute(ctx context.Context, req agent.ExecutionRequest) (*agent.ExecutionResult, error) {
	if req.TaskID == "task_code_d" {
		close(b.executingCode)
		select {
		case <-b.releaseCode:
		case <-ctx.Done():
		}
	} else if req.TaskID == "task_ops_d" {
		close(b.executingOps)
		select {
		case <-b.releaseOps:
		case <-ctx.Done():
		}
	}
	return &agent.ExecutionResult{
		Status:   "SUCCEEDED",
		ExitCode: 0,
	}, nil
}

func TestRunner_Daemon_ParallelLanesExecution(t *testing.T) {
	adapter := &blockingMockAdapter{
		executingCode: make(chan struct{}),
		executingOps:  make(chan struct{}),
		releaseCode:   make(chan struct{}),
		releaseOps:    make(chan struct{}),
	}

	codeClaimed := false
	opsClaimed := false
	var serverMu sync.Mutex

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverMu.Lock()
		defer serverMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/claim") {
			var claimReq ClaimTaskRequest
			_ = json.NewDecoder(r.Body).Decode(&claimReq)

			wantsCode := false
			wantsOps := false
			for _, l := range claimReq.AvailableLanes {
				if l == task.ExecutionCategoryCode {
					wantsCode = true
				}
				if l == task.ExecutionCategoryOps {
					wantsOps = true
				}
			}

			if wantsCode && !codeClaimed {
				codeClaimed = true
				_ = json.NewEncoder(w).Encode(ClaimTaskResponse{
					Claimed: true,
					Attempt: &task.TaskAttempt{ID: "att_code_d", TaskID: "task_code_d", Status: task.AttemptStatusQueued},
					Task: &task.Task{
						ID:                "task_code_d",
						Title:             "Code task",
						ExecutionCategory: task.ExecutionCategoryCode,
					},
				})
				return
			}

			if wantsOps && !opsClaimed {
				opsClaimed = true
				_ = json.NewEncoder(w).Encode(ClaimTaskResponse{
					Claimed: true,
					Attempt: &task.TaskAttempt{ID: "att_ops_d", TaskID: "task_ops_d", Status: task.AttemptStatusQueued},
					Task: &task.Task{
						ID:                "task_ops_d",
						Title:             "Ops task",
						ExecutionCategory: task.ExecutionCategoryOps,
					},
				})
				return
			}

			_ = json.NewEncoder(w).Encode(ClaimTaskResponse{Claimed: false})
			return
		}

		if strings.Contains(r.URL.Path, "/complete") || strings.Contains(r.URL.Path, "/logs") || strings.Contains(r.URL.Path, "/heartbeat") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
	}))
	defer ts.Close()

	cfg := &config.Config{
		RunnerID:        "daemon_test_runner",
		RunnerServerURL: ts.URL,
		RunnerInterval:  100 * time.Millisecond,
	}
	r := NewRunner(cfg, adapter, ts.Client())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. First poll: Code lane available, claims code task
	r.pollAndExecute(ctx)
	select {
	case <-adapter.executingCode:
		t.Log("✓ Daemon started Code Task in background goroutine")
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout waiting for code task to start")
	}

	// 2. Second poll: While Code task is RUNNING, Ops lane is free -> claims ops task concurrently!
	r.pollAndExecute(ctx)
	select {
	case <-adapter.executingOps:
		t.Log("✓ Daemon started Ops Task CONCURRENTLY while Code Task is executing!")
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout waiting for ops task to start concurrently")
	}

	// 3. Verify that both attempts are registered as active simultaneously in runner
	r.mu.Lock()
	activeCount := len(r.activeAttempts)
	_, hasCodeAttempt := r.activeAttempts["att_code_d"]
	_, hasOpsAttempt := r.activeAttempts["att_ops_d"]
	r.mu.Unlock()

	if activeCount != 2 || !hasCodeAttempt || !hasOpsAttempt {
		t.Fatalf("Expected 2 active concurrent attempts (att_code_d and att_ops_d), got: %d (code=%v, ops=%v)", activeCount, hasCodeAttempt, hasOpsAttempt)
	}
	t.Log("✓ Verified Runner has 2 active concurrent attempts across separate lanes")

	// 4. Release tasks and let them finalize cleanly
	close(adapter.releaseOps)
	close(adapter.releaseCode)

	// Wait for cleanup
	for i := 0; i < 50; i++ {
		time.Sleep(50 * time.Millisecond)
		r.mu.Lock()
		remaining := len(r.activeAttempts)
		r.mu.Unlock()
		if remaining == 0 {
			break
		}
	}

	r.mu.Lock()
	finalCount := len(r.activeAttempts)
	r.mu.Unlock()
	if finalCount != 0 {
		t.Fatalf("Expected 0 active attempts after cleanup, got %d", finalCount)
	}
	t.Log("✓ Both lanes cleanly finished and released mutexes")
}

func TestRunner_TaskNoDeployOverridesProjectCompletionGate(t *testing.T) {
	ctx := context.Background()
	store := task.NewMemoryStore()
	finder := &mockProjectFinder{projects: map[string]*project.ProjectResponse{"project": {ID: "project", Metadata: json.RawMessage(`{"cuan_settings":{"auto_deploy":true}}`)}}}
	svc := NewService(NewMemoryStore(store), store, finder)
	item := &task.Task{ID: "local-guide", ProjectID: "project", Title: "Create a local guide", Instructions: "Only one documentation file. Jangan push atau deploy.", ExecutionCategory: task.ExecutionCategoryCode, Status: task.TaskStatusDoing}
	if err := store.CreateTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	attempt := &task.TaskAttempt{ID: "local-attempt", TaskID: item.ID, RunnerID: "runner", Status: task.AttemptStatusRunning}
	if err := store.CreateAttempt(ctx, attempt); err != nil {
		t.Fatal(err)
	}
	changed, requires := true, true
	result, err := svc.Complete(ctx, attempt.ID, CompleteAttemptRequest{RunnerID: "runner", Status: task.AttemptStatusSucceeded, AgentReportRequired: true, AgentReportConfirmed: true, HasCodeChanges: &changed, RequiresDeploy: &requires, DiffSummary: "one local doc file"})
	if err != nil || result.Status != task.AttemptStatusSucceeded {
		t.Fatalf("local scoped delivery required forbidden deployment: %+v %v", result, err)
	}
}
