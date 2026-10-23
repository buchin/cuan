package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
