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
