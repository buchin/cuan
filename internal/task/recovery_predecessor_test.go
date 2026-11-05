package task

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestRecoveryAttemptPredecessorMemory(t *testing.T) {
	recoveryPredecessorContract(t, NewMemoryStore(), "recovery-predecessor")
}

func recoveryPredecessorContract(t *testing.T, store Store, project string) {
	t.Helper()
	ctx := context.Background()
	guarded := store.(interface {
		CreateAttemptAfter(context.Context, *TaskAttempt, string) error
	})
	now := time.Now().UTC().Truncate(time.Microsecond)
	for n, c := range []struct {
		name          string
		newer, cancel bool
		state         string
		want          bool
	}{
		{name: "current failure", state: AttemptStatusFailed, want: true},
		{name: "current interruption", state: AttemptStatusInterrupted, want: true},
		{name: "superseded", newer: true, state: AttemptStatusFailed},
		{name: "cancel requested", cancel: true, state: AttemptStatusFailed},
		{name: "running source", state: AttemptStatusRunning},
		{name: "succeeded source", state: AttemptStatusSucceeded},
	} {
		id := fmt.Sprintf("recovery_predecessor_%d", n)
		item := &Task{ID: id, ProjectID: project, Title: id, Status: TaskStatusQueued, CreatedAt: now}
		if err := store.CreateTask(ctx, item); err != nil {
			t.Fatal(err)
		}
		source := &TaskAttempt{ID: id + "_source", TaskID: id, RunnerID: "fixture", AgentName: "agent", Status: c.state, CancelRequested: c.cancel, CreatedAt: now.Add(-2 * time.Second)}
		if err := store.CreateAttempt(ctx, source); err != nil {
			t.Fatal(err)
		}
		if c.newer {
			if err := store.CreateAttempt(ctx, &TaskAttempt{ID: id + "_newer", TaskID: id, RunnerID: "fixture", AgentName: "agent", Status: AttemptStatusFailed, CreatedAt: now.Add(-time.Second)}); err != nil {
				t.Fatal(err)
			}
		}
		a := &TaskAttempt{ID: id + "_retry", TaskID: id, RunnerID: "cuan-runner", AgentName: "agent", Status: AttemptStatusQueued, CreatedAt: now}
		err := guarded.CreateAttemptAfter(ctx, a, source.ID)
		if c.want && err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !c.want && !errors.Is(err, ErrRequestConflict) {
			t.Fatalf("%s: unsafe reservation result %v", c.name, err)
		}
		if !c.want {
			if _, err := store.GetAttempt(ctx, a.ID); !errors.Is(err, ErrAttemptNotFound) {
				t.Fatalf("%s reserved an attempt: %v", c.name, err)
			}
		}
		saved, _ := store.GetAttempt(ctx, source.ID)
		if saved.Status != c.state || saved.CancelRequested != c.cancel {
			t.Fatal("reservation changed historical source")
		}
	}
	// Two independent supervisors race the same source: only one may reserve.
	id := "recovery_predecessor_race"
	if err := store.CreateTask(ctx, &Task{ID: id, ProjectID: project, Title: id, Status: TaskStatusQueued, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	source := &TaskAttempt{ID: id + "_source", TaskID: id, RunnerID: "fixture", AgentName: "agent", Status: AttemptStatusFailed, CreatedAt: now.Add(-time.Second)}
	if err := store.CreateAttempt(ctx, source); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start
			results <- guarded.CreateAttemptAfter(ctx, &TaskAttempt{ID: fmt.Sprintf("%s_%d", id, n), TaskID: id, RunnerID: "cuan-runner", AgentName: "agent", Status: AttemptStatusQueued, CreatedAt: now}, source.ID)
		}(n)
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrRequestConflict) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("reserved %d retries for one source", successes)
	}
}
