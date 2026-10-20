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
