package task

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestLaunchMemoryContract(t *testing.T) {
	store := NewMemoryStore()
	s := NewService(store, nil)
	created, e := s.CreateTask(context.Background(), "owner", CreateTaskRequest{Title: "Authorized fixture", Instructions: "Preserve this scope"})
	if e != nil {
		t.Fatal(e)
	}
	launchContract(t, store, created.ID, "91000000-0000-4000-8000-000000000001")
}
func launchContract(t *testing.T, store LaunchStore, taskID, roomID string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	job := TaskLaunch{TaskID: taskID, ProjectID: "launch-project", RequestID: "stable-launch", OwnerID: "owner", RoomID: roomID, SourceRequestID: "accepted-user-request", AgentID: "agent", InstructionsHash: "immutable-scope", CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)}
	// Concurrent re-acceptance must retain the same durable intent.
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j, e := store.SaveLaunch(ctx, job)
			if e != nil || j.ID != LaunchID(taskID, job.RequestID) {
				t.Error(j, e)
			}
		}()
	}
	wg.Wait()
	changed := job
	changed.Revision = "New unauthorized scope"
	if _, e := store.SaveLaunch(ctx, changed); !errors.Is(e, ErrRequestConflict) {
		t.Fatal("reused ID changed intent", e)
	}
	// Exactly one dispatcher owns a due launch, including across store instances.
	var mu sync.Mutex
	var claims []*TaskLaunch
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			j, e := store.ClaimLaunch(ctx, now, fmt.Sprint("worker-", n))
			if e != nil {
				t.Error(e)
			}
			if j != nil {
				mu.Lock()
				claims = append(claims, j)
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if len(claims) != 1 {
		t.Fatal("concurrent launch claims", len(claims))
	}
	old := claims[0]
	newer, e := store.ClaimLaunch(ctx, now.Add(2*time.Minute), "replacement")
	if e != nil || newer == nil || newer.ID != old.ID || newer.Checks != 2 {
		t.Fatal("expired worker not recovered", newer, e)
	}
	if e = store.FinishLaunch(ctx, old.ID, old.LeaseToken, "started", now, ""); !errors.Is(e, ErrLaunchLease) {
		t.Fatal("stale worker completed replacement", e)
	}
	later := now.Add(5 * time.Minute)
	if e = store.FinishLaunch(ctx, newer.ID, newer.LeaseToken, "queued", later, "All session slots are in use."); e != nil {
		t.Fatal(e)
	}
	if j, e := store.ClaimLaunch(ctx, now.Add(3*time.Minute), "too-early"); e != nil || j != nil {
		t.Fatal("backoff ignored", j, e)
	}
	due, e := store.ClaimLaunch(ctx, later, "next")
	if e != nil || due == nil {
		t.Fatal(due, e)
	}
	n, e := store.CancelLaunches(ctx, taskID, later)
	if e != nil || n != 1 {
		t.Fatal("cancel failed", n, e)
	}
	if e = store.FinishLaunch(ctx, due.ID, due.LeaseToken, "started", later, ""); !errors.Is(e, ErrLaunchLease) {
		t.Fatal("cancel overwritten", e)
	}
	if attempts, ok := store.(Store); ok {
		e := attempts.CreateAttempt(ctx, &TaskAttempt{ID: attemptRequestID(taskID, job.RequestID), TaskID: taskID, RunnerID: "local_terminal", AgentName: "agent", Status: AttemptStatusQueued, CreatedAt: now})
		if !errors.Is(e, ErrInvalidTaskInput) {
			t.Fatal("cancelled launch admitted a late attempt", e)
		}
	}
	saved, e := store.SaveLaunch(ctx, job)
	if e != nil || saved.State != "cancelled" {
		t.Fatal("cancel resurrected by replay", saved, e)
	}
	if j, e := store.ClaimLaunch(ctx, later.Add(time.Minute), "after-cancel"); e != nil || j != nil {
		t.Fatal("cancelled intent claimed", j, e)
	}
	job.RequestID = "expiring-launch"
	job.ExpiresAt = now.Add(time.Second)
	exp, e := store.SaveLaunch(ctx, job)
	if e != nil {
		t.Fatal(e)
	}
	if j, e := store.ClaimLaunch(ctx, now.Add(time.Minute), "expired"); e != nil || j != nil {
		t.Fatal("expired intent claimed", j, e)
	}
	actual, e := store.GetLaunch(ctx, exp.ID)
	if e != nil || actual.State != "stopped" || actual.SafeError == "" {
		t.Fatal("expiry not recorded", actual, e)
	}
	if attempts, ok := store.(Store); ok {
		current, e := attempts.GetTask(ctx, taskID)
		if e != nil {
			t.Fatal(e)
		}
		job.RequestID = "valid-start"
		job.ProjectID = current.ProjectID
		if job.ProjectID == "" {
			current.ProjectID = "launch-project"
			if e = attempts.UpdateTask(ctx, current); e != nil {
				t.Fatal(e)
			}
			job.ProjectID = current.ProjectID
		}
		job.InstructionsHash = LaunchInstructionsHash(current.Instructions)
		job.ExpiresAt = now.Add(time.Hour)
		saved, e := store.SaveLaunch(ctx, job)
		if e != nil {
			t.Fatal(e)
		}
		owned, e := store.ClaimLaunch(ctx, now, "admit-worker", saved.ID)
		if e != nil || owned == nil {
			t.Fatal(owned, e)
		}
		attempt := &TaskAttempt{ID: attemptRequestID(taskID, job.RequestID), TaskID: taskID, RunnerID: "local_terminal", AgentName: "agent", Status: AttemptStatusQueued, CreatedAt: now}
		if e = attempts.CreateAttempt(ctx, attempt); e != nil {
			t.Fatal("valid claimed intent rejected", e)
		}
		if e = store.FinishLaunch(ctx, owned.ID, owned.LeaseToken, "started", now, ""); e != nil {
			t.Fatal(e)
		}
	}

}

func TestLaunchProjectConcurrencySerialExecution(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)
	projectID := "proj_queue_unit_1"

	t1, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{
		ProjectID:    projectID,
		Title:        "Project Task 1",
		Instructions: "Instructions 1",
	})
	if err != nil {
		t.Fatal(err)
	}

	t2, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{
		ProjectID:    projectID,
		Title:        "Project Task 2",
		Instructions: "Instructions 2",
	})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	l1, err := svc.SaveLaunch(ctx, TaskLaunch{
		TaskID:           t1.ID,
		ProjectID:        projectID,
		RequestID:        "launch-1",
		SourceRequestID:  "source-req-1",
		OwnerID:          "owner",
		RoomID:           "room-1",
		AgentID:          "agent-1",
		InstructionsHash: LaunchInstructionsHash(t1.Instructions),
		CreatedAt:        now,
		AvailableAt:      now,
		ExpiresAt:        now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	l2, err := svc.SaveLaunch(ctx, TaskLaunch{
		TaskID:           t2.ID,
		ProjectID:        projectID,
		RequestID:        "launch-2",
		SourceRequestID:  "source-req-2",
		OwnerID:          "owner",
		RoomID:           "room-1",
		AgentID:          "agent-1",
		InstructionsHash: LaunchInstructionsHash(t2.Instructions),
		CreatedAt:        now.Add(time.Second),
		AvailableAt:      now.Add(time.Second),
		ExpiresAt:        now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	// 1. Claim launch: First launch on the project should be claimed
	claimed1, err := svc.ClaimLaunch(ctx, now.Add(2*time.Second), "worker-token-1")
	if err != nil || claimed1 == nil || claimed1.ID != l1.ID {
		t.Fatalf("expected l1 to be claimed, got %v, err=%v", claimed1, err)
	}

	// 2. Mark launch 1 as started
	err = svc.FinishLaunch(ctx, claimed1.ID, claimed1.LeaseToken, "started", now, "")
	if err != nil {
		t.Fatalf("finish launch 1 failed: %v", err)
	}

	// 3. Project should be active now
	active, err := svc.IsProjectActive(ctx, projectID, "")
	if err != nil || !active {
		t.Fatalf("expected project to be active, got %v, err=%v", active, err)
	}

	// 4. Claim next launch: should return nil because project is busy
	claimed2, err := svc.ClaimLaunch(ctx, now.Add(3*time.Second), "worker-token-2")
	if err != nil {
		t.Fatalf("claim launch error: %v", err)
	}
	if claimed2 != nil {
		t.Fatalf("expected no claim while project is busy, but got %v", claimed2)
	}

	// 5. Complete launch 1
	err = svc.FinishTaskLaunches(ctx, t1.ID, "stopped")
	if err != nil {
		t.Fatalf("finish task 1 launches failed: %v", err)
	}

	// 6. Claim next launch: now launch 2 should be claimed
	claimedNext, err := svc.ClaimLaunch(ctx, now.Add(4*time.Second), "worker-token-3")
	if err != nil || claimedNext == nil || claimedNext.ID != l2.ID {
		t.Fatalf("expected l2 to be claimed after l1 finished, got %v, err=%v", claimedNext, err)
	}
}

func TestTaskTransitionAutoCleansLaunches(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)

	projectID := "proj-auto-clean"
	t1, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{
		Title:        "Task 1",
		Instructions: "Do work",
		ProjectIDs:   []string{projectID},
	})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	l1, err := svc.SaveLaunch(ctx, TaskLaunch{
		TaskID:           t1.ID,
		ProjectID:        projectID,
		RequestID:        "req-auto-clean-1",
		SourceRequestID:  "req-source-1",
		OwnerID:          "owner",
		RoomID:           "room-1",
		AgentID:          "agent-1",
		InstructionsHash: LaunchInstructionsHash(t1.Instructions),
		CreatedAt:        now,
		AvailableAt:      now,
		ExpiresAt:        now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Claim and start
	claimed, err := svc.ClaimLaunch(ctx, now, "worker-token-1")
	if err != nil || claimed == nil || claimed.ID != l1.ID {
		t.Fatalf("expected claimed l1, got %v, err=%v", claimed, err)
	}
	err = svc.FinishLaunch(ctx, claimed.ID, claimed.LeaseToken, "started", now, "")
	if err != nil {
		t.Fatal(err)
	}

	// Project is active
	active, err := svc.IsProjectActive(ctx, projectID, "")
	if err != nil || !active {
		t.Fatalf("expected project active, got %v", active)
	}

	// Move task to review (e.g. agent completed implementation)
	reviewStatus := TaskStatusReview
	summary := "Work finished"
	completeNote := "Verified test"
	_, err = svc.UpdateTask(ctx, t1.ID, "agent", UpdateTaskRequest{
		Status:       &reviewStatus,
		Summary:      &summary,
		CompleteNote: &completeNote,
	})
	if err != nil {
		t.Fatalf("update task to review failed: %v", err)
	}

	// Launch should have been automatically cleaned to stopped!
	updatedLaunch, err := svc.GetLaunch(ctx, l1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedLaunch.State != "stopped" {
		t.Fatalf("expected launch state to be 'stopped', got '%s'", updatedLaunch.State)
	}

	// Project should now be NOT active
	active, err = svc.IsProjectActive(ctx, projectID, "")
	if err != nil || active {
		t.Fatalf("expected project to be free/inactive, but got active=%v", active)
	}
}

func TestClaimLaunchDoesNotBlockOnOrphanStartedLaunch(t *testing.T) {
	store := NewMemoryStore()
	svc := NewService(store, nil)
	ctx := context.Background()
	projectID := "proj-orphan-test"

	t1, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{
		Title:        "Task 1",
		Instructions: "First task",
		ProjectIDs:   []string{projectID},
	})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	l1, err := svc.SaveLaunch(ctx, TaskLaunch{
		TaskID:           t1.ID,
		ProjectID:        projectID,
		RequestID:        "req-orphan-1",
		SourceRequestID:  "req-source-orphan-1",
		OwnerID:          "owner",
		RoomID:           "room-1",
		AgentID:          "agent-1",
		InstructionsHash: LaunchInstructionsHash(t1.Instructions),
		CreatedAt:        now,
		AvailableAt:      now,
		ExpiresAt:        now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Claim and start l1
	claimed1, err := svc.ClaimLaunch(ctx, now, "worker-1")
	if err != nil || claimed1 == nil || claimed1.ID != l1.ID {
		t.Fatalf("expected claimed l1, got %v, err=%v", claimed1, err)
	}
	if err := svc.FinishLaunch(ctx, claimed1.ID, claimed1.LeaseToken, "started", now, ""); err != nil {
		t.Fatal(err)
	}

	// Simulate orphan condition: task 1 became done without finishing the launch (e.g., server restart or external update)
	doneTask, err := store.GetTask(ctx, t1.ID)
	if err != nil {
		t.Fatal(err)
	}
	doneTask.Status = TaskStatusDone
	if err := store.UpdateTask(ctx, doneTask); err != nil {
		t.Fatal(err)
	}

	// Create Task 2 in the same project
	t2, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{
		Title:        "Task 2",
		Instructions: "Second task",
		ProjectIDs:   []string{projectID},
	})
	if err != nil {
		t.Fatal(err)
	}

	l2, err := svc.SaveLaunch(ctx, TaskLaunch{
		TaskID:           t2.ID,
		ProjectID:        projectID,
		RequestID:        "req-orphan-2",
		SourceRequestID:  "req-source-orphan-2",
		OwnerID:          "owner",
		RoomID:           "room-2",
		AgentID:          "agent-2",
		InstructionsHash: LaunchInstructionsHash(t2.Instructions),
		CreatedAt:        now,
		AvailableAt:      now,
		ExpiresAt:        now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	// ClaimLaunch should NOT be blocked by l1 because t1 is done. It should claim l2!
	claimed2, err := svc.ClaimLaunch(ctx, now, "worker-2")
	if err != nil {
		t.Fatalf("unexpected error claiming launch: %v", err)
	}
	if claimed2 == nil {
		t.Fatal("expected l2 to be claimed, but ClaimLaunch returned nil (blocked by orphan launch)")
	}
	if claimed2.ID != l2.ID {
		t.Fatalf("expected claimed launch %s, got %s", l2.ID, claimed2.ID)
	}

	// Verify l1 was reconciled to stopped
	reconciledL1, err := svc.GetLaunch(ctx, l1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reconciledL1.State != "stopped" {
		t.Fatalf("expected orphan launch l1 to be reconciled to 'stopped', got '%s'", reconciledL1.State)
	}
}

func TestReconcileStartupAndOrphanLaunches(t *testing.T) {
	store := NewMemoryStore()
	svc := NewService(store, nil)
	ctx := context.Background()
	projectID := "proj-startup-reconcile"

	t1, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{
		Title:        "Task 1",
		Instructions: "Task before crash",
		ProjectIDs:   []string{projectID},
	})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	l1, err := svc.SaveLaunch(ctx, TaskLaunch{
		TaskID:           t1.ID,
		ProjectID:        projectID,
		RequestID:        "req-crash-1",
		SourceRequestID:  "req-source-crash-1",
		OwnerID:          "owner",
		RoomID:           "room-1",
		AgentID:          "agent-1",
		InstructionsHash: LaunchInstructionsHash(t1.Instructions),
		CreatedAt:        now,
		AvailableAt:      now,
		ExpiresAt:        now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	claimed1, err := svc.ClaimLaunch(ctx, now, "worker-1")
	if err != nil || claimed1 == nil {
		t.Fatalf("failed to claim l1: %v", err)
	}
	if err := svc.FinishLaunch(ctx, claimed1.ID, claimed1.LeaseToken, "started", now, ""); err != nil {
		t.Fatal(err)
	}

	// Reconcile startup launches
	if err := svc.ReconcileStartupLaunches(ctx, now); err != nil {
		t.Fatalf("ReconcileStartupLaunches failed: %v", err)
	}

	l1After, err := svc.GetLaunch(ctx, l1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if l1After.State != "stopped" {
		t.Fatalf("expected l1 state 'stopped' after startup reconciliation, got '%s'", l1After.State)
	}
}

func TestReconcileStaleQueuedTerminalTasks(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)
	now := time.Now().UTC()

	terminalTask, err := svc.CreateTask(ctx, "user", CreateTaskRequest{
		Title:        "Terminal · My Project",
		ProjectIDs:   []string{"p_terminal"},
		Instructions: "Interactive terminal session",
	})
	if err != nil {
		t.Fatal(err)
	}

	qStatus := TaskStatusQueued
	_, err = svc.UpdateTask(ctx, terminalTask.ID, "user", UpdateTaskRequest{Status: &qStatus})
	if err != nil {
		t.Fatal(err)
	}

	l, err := svc.SaveLaunch(ctx, TaskLaunch{
		TaskID:           terminalTask.ID,
		ProjectID:        "p_terminal",
		RequestID:        "term-req-1",
		SourceRequestID:  "term-source-1",
		OwnerID:          "user",
		RoomID:           "room-term",
		AgentID:          "agent-term",
		InstructionsHash: "hash-term",
		AvailableAt:      now.Add(-6 * time.Minute),
		ExpiresAt:        now.Add(time.Hour),
		CreatedAt:        now.Add(-6 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}

	store.mu.Lock()
	if tt := store.tasks[terminalTask.ID]; tt != nil {
		tt.CreatedAt = now.Add(-6 * time.Minute)
	}
	store.mu.Unlock()

	if err := svc.ReconcileOrphanLaunches(ctx, now); err != nil {
		t.Fatalf("ReconcileOrphanLaunches failed: %v", err)
	}

	updatedTask, err := svc.GetTaskDetail(ctx, terminalTask.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedTask.Status != "cancelled" {
		t.Fatalf("expected stale terminal task status 'cancelled', got '%s'", updatedTask.Status)
	}

	updatedLaunch, err := svc.GetLaunch(ctx, l.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedLaunch.State != "stopped" {
		t.Fatalf("expected launch state 'stopped', got '%s'", updatedLaunch.State)
	}
}

func TestBlockedTerminalTaskAutoCancelled(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)
	now := time.Now().UTC()

	terminalTask, err := svc.CreateTask(ctx, "user", CreateTaskRequest{
		Title:        "Terminal · cuan",
		ProjectIDs:   []string{"p_terminal"},
		Instructions: "Interactive terminal session",
	})
	if err != nil {
		t.Fatal(err)
	}

	bStatus := TaskStatusBlocked
	_, err = svc.UpdateTask(ctx, terminalTask.ID, "user", UpdateTaskRequest{Status: &bStatus})
	if err != nil {
		t.Fatal(err)
	}

	l, err := svc.SaveLaunch(ctx, TaskLaunch{
		TaskID:           terminalTask.ID,
		ProjectID:        "p_terminal",
		RequestID:        "term-req-blocked",
		SourceRequestID:  "term-source-blocked",
		OwnerID:          "user",
		RoomID:           "room-term",
		AgentID:          "agent-term",
		InstructionsHash: "hash-term",
		AvailableAt:      now.Add(-10 * time.Minute),
		ExpiresAt:        now.Add(time.Hour),
		CreatedAt:        now.Add(-10 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}

	store.mu.Lock()
	if tt := store.tasks[terminalTask.ID]; tt != nil {
		tt.CreatedAt = now.Add(-10 * time.Minute)
		tt.UpdatedAt = now.Add(-10 * time.Minute)
	}
	store.mu.Unlock()

	if err := svc.ReconcileOrphanLaunches(ctx, now); err != nil {
		t.Fatalf("ReconcileOrphanLaunches failed: %v", err)
	}

	updatedTask, err := svc.GetTaskDetail(ctx, terminalTask.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedTask.Status != "cancelled" {
		t.Fatalf("expected blocked terminal task status 'cancelled', got '%s'", updatedTask.Status)
	}

	updatedLaunch, err := svc.GetLaunch(ctx, l.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedLaunch.State != "stopped" {
		t.Fatalf("expected launch state 'stopped', got '%s'", updatedLaunch.State)
	}
}

func TestIsProjectActiveQueuedTaskDoesNotBlock(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)

	created, err := svc.CreateTask(ctx, "user", CreateTaskRequest{
		Title:        "Queued Task",
		ProjectIDs:   []string{"p_queued_test"},
		Instructions: "Waiting to run",
	})
	if err != nil {
		t.Fatal(err)
	}
	qStatus := TaskStatusQueued
	_, err = svc.UpdateTask(ctx, created.ID, "user", UpdateTaskRequest{Status: &qStatus})
	if err != nil {
		t.Fatal(err)
	}

	active, err := svc.IsProjectActive(ctx, "p_queued_test", "")
	if err != nil {
		t.Fatal(err)
	}
	if active {
		t.Fatal("expected IsProjectActive to be false for queued task, but got true")
	}
}

func TestReconcileGhostQueuedTaskResurrection(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)
	now := time.Now().UTC()

	created, err := svc.CreateTask(ctx, "user", CreateTaskRequest{
		Title:        "Investigasi Task",
		ProjectIDs:   []string{"p_ghost_test"},
		Instructions: "Fix issues",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Task status is queued
	qStatus := TaskStatusQueued
	_, err = svc.UpdateTask(ctx, created.ID, "user", UpdateTaskRequest{Status: &qStatus})
	if err != nil {
		t.Fatal(err)
	}

	// But its launch was erroneously marked stopped with 'Task is in terminal status.'
	l, err := svc.SaveLaunch(ctx, TaskLaunch{
		TaskID:           created.ID,
		ProjectID:        "p_ghost_test",
		RequestID:        "req_ghost_1",
		OwnerID:          "owner",
		RoomID:           "room",
		SourceRequestID:  "req_source_1",
		AgentID:          "agent_sm",
		InstructionsHash: LaunchInstructionsHash("Fix issues"),
		CreatedAt:        now,
		AvailableAt:      now,
		ExpiresAt:        now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	store.mu.Lock()
	if savedLaunch, ok := store.launches[l.ID]; ok {
		savedLaunch.State = "stopped"
		savedLaunch.SafeError = "Task is in terminal status."
	}
	store.mu.Unlock()

	// Run self-healing
	if err := svc.ReconcileOrphanLaunches(ctx, now); err != nil {
		t.Fatalf("ReconcileOrphanLaunches failed: %v", err)
	}

	// Launch should be resurrected back to queued with clear safe_error
	resurrected, err := svc.GetLaunch(ctx, l.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resurrected.State != "queued" {
		t.Fatalf("expected launch state 'queued', got '%s'", resurrected.State)
	}
	if resurrected.SafeError != "" {
		t.Fatalf("expected empty safe error after resurrection, got '%s'", resurrected.SafeError)
	}
}

func TestReconcileGhostQueuedTaskResurrectionAfterServerRestart(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)
	now := time.Now().UTC()

	created, err := svc.CreateTask(ctx, "user", CreateTaskRequest{
		Title:        "Mermaid Integration Task",
		ProjectIDs:   []string{"p_mermaid_test"},
		Instructions: "Render diagrams",
	})
	if err != nil {
		t.Fatal(err)
	}

	qStatus := TaskStatusQueued
	_, err = svc.UpdateTask(ctx, created.ID, "user", UpdateTaskRequest{Status: &qStatus})
	if err != nil {
		t.Fatal(err)
	}

	l, err := svc.SaveLaunch(ctx, TaskLaunch{
		TaskID:           created.ID,
		ProjectID:        "p_mermaid_test",
		RequestID:        "req_mermaid_1",
		OwnerID:          "owner",
		RoomID:           "room",
		SourceRequestID:  "req_source_1",
		AgentID:          "agent_sm",
		InstructionsHash: LaunchInstructionsHash("Render diagrams"),
		CreatedAt:        now.Add(-2 * time.Hour),
		AvailableAt:      now.Add(-2 * time.Hour),
		ExpiresAt:        now.Add(-1 * time.Hour), // Expired!
	})
	if err != nil {
		t.Fatal(err)
	}

	store.mu.Lock()
	if savedLaunch, ok := store.launches[l.ID]; ok {
		savedLaunch.State = "stopped"
		savedLaunch.SafeError = "Execution stopped during server restart."
	}
	store.mu.Unlock()

	// Run self-healing
	if err := svc.ReconcileOrphanLaunches(ctx, now); err != nil {
		t.Fatalf("ReconcileOrphanLaunches failed: %v", err)
	}

	// Launch should be resurrected back to queued with clear safe_error and renewed expiration
	resurrected, err := svc.GetLaunch(ctx, l.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resurrected.State != "queued" {
		t.Fatalf("expected launch state 'queued', got '%s'", resurrected.State)
	}
	if resurrected.SafeError != "" {
		t.Fatalf("expected empty safe error after resurrection, got '%s'", resurrected.SafeError)
	}
	if !resurrected.ExpiresAt.After(now) {
		t.Fatalf("expected launch ExpiresAt to be after now, got '%s'", resurrected.ExpiresAt)
	}

	// It should now be claimable by ClaimLaunch!
	claimed, err := svc.ClaimLaunch(ctx, now, "worker-token")
	if err != nil {
		t.Fatalf("ClaimLaunch failed: %v", err)
	}
	if claimed == nil || claimed.ID != l.ID {
		t.Fatalf("expected ClaimLaunch to claim resurrected launch %s, got %+v", l.ID, claimed)
	}
}

func TestReconcileStartupLaunches_RevivesQueuedTask(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)
	now := time.Now().UTC()

	created, err := svc.CreateTask(ctx, "user", CreateTaskRequest{
		Title:        "Task needing recovery on startup",
		Instructions: "Do recovery",
		ProjectIDs:   []string{"p_startup"},
	})
	if err != nil {
		t.Fatal(err)
	}
	qStatus := TaskStatusQueued
	_, err = svc.UpdateTask(ctx, created.ID, "user", UpdateTaskRequest{Status: &qStatus})
	if err != nil {
		t.Fatal(err)
	}

	l, err := svc.SaveLaunch(ctx, TaskLaunch{
		TaskID:           created.ID,
		ProjectID:        "p_startup",
		RequestID:        "req_startup_1",
		OwnerID:          "owner",
		RoomID:           "room",
		SourceRequestID:  "req_source_startup_1",
		AgentID:          "agent_sm",
		InstructionsHash: LaunchInstructionsHash("Do recovery"),
		CreatedAt:        now.Add(-1 * time.Hour),
		AvailableAt:      now.Add(-1 * time.Hour),
		ExpiresAt:        now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Case A: Launch was in 'started' state when server crashed
	store.mu.Lock()
	store.launches[l.ID].State = "started"
	store.mu.Unlock()

	if err := svc.ReconcileStartupLaunches(ctx, now); err != nil {
		t.Fatalf("ReconcileStartupLaunches failed: %v", err)
	}

	revived, err := svc.GetLaunch(ctx, l.ID)
	if err != nil {
		t.Fatal(err)
	}
	if revived.State != "queued" {
		t.Fatalf("expected revived launch state 'queued', got '%s'", revived.State)
	}

	// Case B: Launch was previously marked 'stopped' during earlier crash
	store.mu.Lock()
	store.launches[l.ID].State = "stopped"
	store.launches[l.ID].SafeError = "Execution stopped during server restart."
	store.mu.Unlock()

	if err := svc.ReconcileStartupLaunches(ctx, now); err != nil {
		t.Fatalf("ReconcileStartupLaunches failed: %v", err)
	}

	revivedAgain, err := svc.GetLaunch(ctx, l.ID)
	if err != nil {
		t.Fatal(err)
	}
	if revivedAgain.State != "queued" {
		t.Fatalf("expected revivedAgain launch state 'queued', got '%s'", revivedAgain.State)
	}
	if revivedAgain.SafeError != "" {
		t.Fatalf("expected empty safe_error, got '%s'", revivedAgain.SafeError)
	}
}

func TestLaunchMultiLaneConcurrency(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)
	now := time.Now().UTC()
	projectID := "proj-multilane-test"

	// 1. Create a code task and start it
	codeTask, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{
		Title:             "Feature implementation",
		ProjectIDs:        []string{projectID},
		Instructions:      "Write code",
		ExecutionCategory: ExecutionCategoryCode,
	})
	if err != nil {
		t.Fatal(err)
	}
	codeDoing := TaskStatusDoing
	if _, err := svc.UpdateTask(ctx, codeTask.ID, "runner", UpdateTaskRequest{Status: &codeDoing}); err != nil {
		t.Fatal(err)
	}

	// 2. Create a readonly task with queued launch in the same project
	readTask, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{
		Title:             "Inspect system status and telemetry",
		ProjectIDs:        []string{projectID},
		Instructions:      "Inspect status",
		ExecutionCategory: ExecutionCategoryReadOnly,
	})
	if err != nil {
		t.Fatal(err)
	}
	readQueued := TaskStatusQueued
	if _, err := svc.UpdateTask(ctx, readTask.ID, "runner", UpdateTaskRequest{Status: &readQueued}); err != nil {
		t.Fatal(err)
	}
	readLaunch, err := svc.SaveLaunch(ctx, TaskLaunch{
		TaskID:           readTask.ID,
		ProjectID:        projectID,
		RequestID:        "req_read_1",
		SourceRequestID:  "req_read_source_1",
		OwnerID:          "owner",
		RoomID:           "room-1",
		AgentID:          "agent-1",
		InstructionsHash: LaunchInstructionsHash("Inspect status"),
		CreatedAt:        now,
		AvailableAt:      now,
		ExpiresAt:        now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	// 3. ClaimLaunch: The readonly task launch MUST be claimable concurrently even while codeTask is doing!
	claimedRead, err := svc.ClaimLaunch(ctx, now, "worker-read")
	if err != nil {
		t.Fatalf("ClaimLaunch for readonly task failed: %v", err)
	}
	if claimedRead == nil || claimedRead.ID != readLaunch.ID {
		t.Fatalf("expected readonly launch %s to be claimed concurrently with active code task, got %+v", readLaunch.ID, claimedRead)
	}

	// 4. Create another code task: It MUST be blocked while codeTask is doing
	codeTask2, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{
		Title:             "Second feature implementation",
		ProjectIDs:        []string{projectID},
		Instructions:      "Write more code",
		ExecutionCategory: ExecutionCategoryCode,
	})
	if err != nil {
		t.Fatal(err)
	}
	codeQueued := TaskStatusQueued
	if _, err := svc.UpdateTask(ctx, codeTask2.ID, "runner", UpdateTaskRequest{Status: &codeQueued}); err != nil {
		t.Fatal(err)
	}
	codeLaunch2, err := svc.SaveLaunch(ctx, TaskLaunch{
		TaskID:           codeTask2.ID,
		ProjectID:        projectID,
		RequestID:        "req_code_2",
		SourceRequestID:  "req_code_source_2",
		OwnerID:          "owner",
		RoomID:           "room-1",
		AgentID:          "agent-1",
		InstructionsHash: LaunchInstructionsHash("Write more code"),
		CreatedAt:        now,
		AvailableAt:      now,
		ExpiresAt:        now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	claimedCode2, err := svc.ClaimLaunch(ctx, now, "worker-code-2")
	if err != nil {
		t.Fatalf("ClaimLaunch unexpected error: %v", err)
	}
	if claimedCode2 != nil {
		t.Fatalf("expected second code task launch %s to be blocked by active code task, but it was claimed: %+v", codeLaunch2.ID, claimedCode2)
	}

	// 5. Active database migration acts as exclusive barrier: blocks all tasks (including readonly)
	migTask, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{
		Title:             "Database migration for payments",
		ProjectIDs:        []string{"proj-mig-test"},
		Instructions:      "Run migration script",
		ExecutionCategory: ExecutionCategoryCode,
	})
	if err != nil {
		t.Fatal(err)
	}
	migDoing := TaskStatusDoing
	if _, err := svc.UpdateTask(ctx, migTask.ID, "runner", UpdateTaskRequest{Status: &migDoing}); err != nil {
		t.Fatal(err)
	}

	readTaskMig, err := svc.CreateTask(ctx, "owner", CreateTaskRequest{
		Title:             "Inspect system status",
		ProjectIDs:        []string{"proj-mig-test"},
		Instructions:      "Inspect status",
		ExecutionCategory: ExecutionCategoryReadOnly,
	})
	if err != nil {
		t.Fatal(err)
	}
	readMigQueued := TaskStatusQueued
	if _, err := svc.UpdateTask(ctx, readTaskMig.ID, "runner", UpdateTaskRequest{Status: &readMigQueued}); err != nil {
		t.Fatal(err)
	}
	readMigLaunch, err := svc.SaveLaunch(ctx, TaskLaunch{
		TaskID:           readTaskMig.ID,
		ProjectID:        "proj-mig-test",
		RequestID:        "req_read_mig_1",
		SourceRequestID:  "req_read_mig_source_1",
		OwnerID:          "owner",
		RoomID:           "room-1",
		AgentID:          "agent-1",
		InstructionsHash: LaunchInstructionsHash("Inspect status"),
		CreatedAt:        now,
		AvailableAt:      now,
		ExpiresAt:        now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	claimedMigRead, err := svc.ClaimLaunch(ctx, now, "worker-mig-read")
	if err != nil {
		t.Fatalf("ClaimLaunch error: %v", err)
	}
	if claimedMigRead != nil {
		t.Fatalf("expected readonly task launch %s to be blocked by active migration, but got %+v", readMigLaunch.ID, claimedMigRead)
	}
}

func TestRequestConflictLaunchRemainsStoppedDuringRecovery(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)
	now := time.Now().UTC()
	created, err := svc.CreateTask(ctx, "user", CreateTaskRequest{Title: "Preserve rejected request", ProjectIDs: []string{"p"}, Instructions: "approved scope"})
	if err != nil {
		t.Fatal(err)
	}
	status := TaskStatusQueued
	if _, err = svc.UpdateTask(ctx, created.ID, "user", UpdateTaskRequest{Status: &status}); err != nil {
		t.Fatal(err)
	}
	launch, err := svc.SaveLaunch(ctx, TaskLaunch{TaskID: created.ID, ProjectID: "p", RequestID: "request", OwnerID: "owner", RoomID: "room", SourceRequestID: "source", AgentID: "agent_cuan", InstructionsHash: LaunchInstructionsHash("approved scope"), CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := svc.ClaimLaunch(ctx, now, "worker", launch.ID)
	if err != nil || claimed == nil {
		t.Fatalf("claim: %v, %v", claimed, err)
	}
	if err = svc.FinishLaunch(ctx, claimed.ID, claimed.LeaseToken, "stopped", now, ErrRequestConflict.Error()); err != nil {
		t.Fatal(err)
	}
	for _, recover := range []func(context.Context, time.Time) error{svc.ReconcileOrphanLaunches, svc.ReconcileStartupLaunches} {
		if err = recover(ctx, now); err != nil {
			t.Fatal(err)
		}
		saved, err := svc.GetLaunch(ctx, launch.ID)
		if err != nil || saved.State != "stopped" || saved.SafeError != ErrRequestConflict.Error() {
			t.Fatalf("rejected launch resurrected: %+v, %v", saved, err)
		}
	}
	if next, err := svc.ClaimLaunch(ctx, now.Add(time.Minute), "worker2"); err != nil || next != nil {
		t.Fatalf("rejected launch claimed: %+v, %v", next, err)
	}
}

func TestReconcileZombieTasksInDoingStatus(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)
	now := time.Now().UTC()

	// 1. Task with summary but left in doing without running attempt or active launch -> heals to review
	taskWithSummary, err := svc.CreateTask(ctx, "user", CreateTaskRequest{
		Title:        "Feature Completed but Review Stalled",
		ProjectIDs:   []string{"proj_1"},
		Instructions: "Deploy broadcast",
	})
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.tasks[taskWithSummary.ID].Status = TaskStatusDoing
	store.tasks[taskWithSummary.ID].Summary = "Implemented feature and verified tests"
	store.tasks[taskWithSummary.ID].UpdatedAt = now.Add(-3 * time.Minute)
	store.mu.Unlock()

	// 2. Task without summary and left in doing without active execution -> heals to blocked
	taskWithoutSummary, err := svc.CreateTask(ctx, "user", CreateTaskRequest{
		Title:        "Feature Crashed Midway",
		ProjectIDs:   []string{"proj_1"},
		Instructions: "Deploy crawler",
	})
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.tasks[taskWithoutSummary.ID].Status = TaskStatusDoing
	store.tasks[taskWithoutSummary.ID].UpdatedAt = now.Add(-3 * time.Minute)
	store.mu.Unlock()

	// 3. Task with an actively running attempt -> stays doing
	taskRunning, err := svc.CreateTask(ctx, "user", CreateTaskRequest{
		Title:        "Feature Currently Running",
		ProjectIDs:   []string{"proj_1"},
		Instructions: "Deploy backend",
	})
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.tasks[taskRunning.ID].Status = TaskStatusDoing
	store.tasks[taskRunning.ID].UpdatedAt = now.Add(-3 * time.Minute)
	store.attempts["att_live"] = &TaskAttempt{
		ID:        "att_live",
		TaskID:    taskRunning.ID,
		Status:    AttemptStatusRunning,
		CreatedAt: now.Add(-time.Minute),
	}
	store.mu.Unlock()

	// Run ReconcileOrphanLaunches
	if err := svc.ReconcileOrphanLaunches(ctx, now); err != nil {
		t.Fatalf("ReconcileOrphanLaunches failed: %v", err)
	}

	// Verify task 1 healed to review
	t1, err := svc.GetTaskDetail(ctx, taskWithSummary.ID)
	if err != nil || t1.Status != TaskStatusReview {
		t.Fatalf("expected task 1 healed to 'review', got status '%s', err: %v", t1.Status, err)
	}

	// Verify task 2 healed to blocked
	t2, err := svc.GetTaskDetail(ctx, taskWithoutSummary.ID)
	if err != nil || t2.Status != TaskStatusBlocked {
		t.Fatalf("expected task 2 healed to 'blocked', got status '%s', err: %v", t2.Status, err)
	}

	// Verify task 3 stayed doing
	t3, err := svc.GetTaskDetail(ctx, taskRunning.ID)
	if err != nil || t3.Status != TaskStatusDoing {
		t.Fatalf("expected task 3 to stay 'doing', got status '%s', err: %v", t3.Status, err)
	}
}

func TestZombieTaskDoesNotBlockNextQueuedTask(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil)
	projectID := "proj-truth-lock-test"

	// 1. Create a zombie code task (status = 'doing', but NO running attempt and NO active launch)
	zombieTask, err := svc.CreateTask(ctx, "user", CreateTaskRequest{
		Title:             "Zombie Task Left In Doing",
		ProjectIDs:        []string{projectID},
		Instructions:      "Some work",
		ExecutionCategory: ExecutionCategoryCode,
	})
	if err != nil {
		t.Fatal(err)
	}
	doing := TaskStatusDoing
	if _, err := svc.UpdateTask(ctx, zombieTask.ID, "runner", UpdateTaskRequest{Status: &doing}); err != nil {
		t.Fatal(err)
	}

	// 2. Check IsProjectLaneActive for the project
	// Because the zombie task has no running attempt and no active launch lease, lane should NOT be active!
	active, err := svc.IsProjectLaneActive(ctx, projectID, ExecutionCategoryCode, "")
	if err != nil {
		t.Fatal(err)
	}
	if active {
		t.Fatalf("expected IsProjectLaneActive to return false for zombie task without running attempt or active launch lease, got true")
	}

	// 3. Now add a running attempt to the task
	store.mu.Lock()
	store.attempts["att_live_1"] = &TaskAttempt{
		ID:        "att_live_1",
		TaskID:    zombieTask.ID,
		Status:    AttemptStatusRunning,
		CreatedAt: time.Now().UTC(),
	}
	store.mu.Unlock()

	// Now lane SHOULD be active
	activeLive, err := svc.IsProjectLaneActive(ctx, projectID, ExecutionCategoryCode, "")
	if err != nil {
		t.Fatal(err)
	}
	if !activeLive {
		t.Fatalf("expected IsProjectLaneActive to return true when running attempt exists, got false")
	}
}
