package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
)

func TestHeartbeatExtendsDeadlineOnActiveProgressAndCapsAtSafetyCeiling(t *testing.T) {
	t.Setenv("CUAN_TASK_MAX_EXECUTION_CAP", "30m")
	t.Setenv("CUAN_TASK_IDLE_TIMEOUT", "7m")

	ctx := context.Background()
	taskStore := task.NewMemoryStore()
	runnerStore := NewMemoryStore(taskStore)
	svc := NewService(runnerStore, taskStore)

	now := time.Now().UTC()
	started := now.Add(-15 * time.Minute)
	initialDeadline := now.Add(2 * time.Minute)
	lease := now.Add(time.Minute)

	item := &task.Task{
		ID:                "task-active-lease",
		Title:             "Long test suite",
		ExecutionCategory: task.ExecutionCategoryCode,
		Status:            task.TaskStatusDoing,
		CreatedAt:         started,
		UpdatedAt:         started,
	}
	if err := taskStore.CreateTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	att := &task.TaskAttempt{
		ID:                "att-active-lease",
		TaskID:            item.ID,
		RunnerID:          "runner-1",
		Status:            task.AttemptStatusRunning,
		AttemptNumber:     1,
		StartedAt:         &started,
		CreatedAt:         started,
		LeaseExpiresAt:    &lease,
		ExecutionDeadline: &initialDeadline,
	}
	if err := taskStore.CreateAttempt(ctx, att); err != nil {
		t.Fatal(err)
	}

	resp, err := svc.Heartbeat(ctx, HeartbeatRequest{
		RunnerID:                 "runner-1",
		CurrentAttemptID:         att.ID,
		ActiveAttemptIDs:         []string{att.ID},
		ActiveProgressAttemptIDs: []string{att.ID},
	}, time.Minute)
	if err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}
	extended, ok := resp.ExtendedDeadlines[att.ID]
	if !ok {
		t.Fatal("expected extended deadline in heartbeat response for active attempt")
	}
	if !extended.After(initialDeadline) {
		t.Fatalf("expected extended deadline %v to be after initial deadline %v", extended, initialDeadline)
	}

	updated, err := taskStore.GetAttempt(ctx, att.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ExecutionDeadline == nil || !updated.ExecutionDeadline.Equal(extended) {
		t.Fatalf("persisted execution deadline %v does not match response %v", updated.ExecutionDeadline, extended)
	}

	// Verify safety ceiling: even with huge leaseDuration, deadline never exceeds started + 30m.
	t.Setenv("CUAN_TASK_MAX_EXECUTION_CAP", "20m")
	resp2, err := svc.Heartbeat(ctx, HeartbeatRequest{
		RunnerID:                 "runner-1",
		CurrentAttemptID:         att.ID,
		ActiveAttemptIDs:         []string{att.ID},
		ActiveProgressAttemptIDs: []string{att.ID},
	}, 25*time.Minute)
	if err != nil {
		t.Fatalf("second heartbeat failed: %v", err)
	}
	maxCap := started.Add(20 * time.Minute)
	if got := resp2.ExtendedDeadlines[att.ID]; got.After(maxCap) {
		t.Fatalf("extended deadline %v exceeded safety ceiling %v", got, maxCap)
	}
}

func TestHeartbeatDoesNotExtendDeadlineWhenInactive(t *testing.T) {
	ctx := context.Background()
	taskStore := task.NewMemoryStore()
	runnerStore := NewMemoryStore(taskStore)
	svc := NewService(runnerStore, taskStore)

	now := time.Now().UTC()
	started := now.Add(-5 * time.Minute)
	initialDeadline := now.Add(2 * time.Minute)
	lease := now.Add(time.Minute)

	item := &task.Task{
		ID:                "task-inactive-lease",
		Title:             "Hung process",
		ExecutionCategory: task.ExecutionCategoryCode,
		Status:            task.TaskStatusDoing,
		CreatedAt:         started,
		UpdatedAt:         started,
	}
	if err := taskStore.CreateTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	att := &task.TaskAttempt{
		ID:                "att-inactive-lease",
		TaskID:            item.ID,
		RunnerID:          "runner-1",
		Status:            task.AttemptStatusRunning,
		AttemptNumber:     1,
		StartedAt:         &started,
		CreatedAt:         started,
		LeaseExpiresAt:    &lease,
		ExecutionDeadline: &initialDeadline,
	}
	if err := taskStore.CreateAttempt(ctx, att); err != nil {
		t.Fatal(err)
	}

	resp, err := svc.Heartbeat(ctx, HeartbeatRequest{
		RunnerID:         "runner-1",
		CurrentAttemptID: att.ID,
		ActiveAttemptIDs: []string{att.ID},
	}, time.Minute)
	if err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}
	if len(resp.ExtendedDeadlines) != 0 {
		t.Fatalf("expected no extended deadlines when inactive, got %+v", resp.ExtendedDeadlines)
	}
	updated, _ := taskStore.GetAttempt(ctx, att.ID)
	if updated.ExecutionDeadline == nil || !updated.ExecutionDeadline.Equal(initialDeadline) {
		t.Fatalf("expected deadline to remain %v, got %v", initialDeadline, updated.ExecutionDeadline)
	}
}

func TestAdaptiveDeadlineContextExtendsAndStopsAtCeiling(t *testing.T) {
	now := time.Now()
	actx, cancel := newAdaptiveDeadlineContext(context.Background(), now.Add(80*time.Millisecond), now.Add(220*time.Millisecond))
	defer cancel()

	// Extend past the initial 80ms deadline.
	actx.Extend(now.Add(160 * time.Millisecond))
	time.Sleep(110 * time.Millisecond)
	if actx.Err() != nil {
		t.Fatalf("context canceled prematurely at initial deadline: %v", actx.Err())
	}

	// Attempt to extend past the 220ms safety ceiling; must clamp and cancel.
	actx.Extend(now.Add(5 * time.Second))
	select {
	case <-actx.Done():
		if actx.Err() != context.DeadlineExceeded {
			t.Fatalf("expected DeadlineExceeded at safety ceiling, got %v", actx.Err())
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("adaptive context did not stop at safety ceiling")
	}
}

func TestProjectVerifierActivityWatchdogExtendsActiveAndStopsHung(t *testing.T) {
	t.Run("active_output_survives_idle_window", func(t *testing.T) {
		root := t.TempDir()
		script := filepath.Join(root, "active.sh")
		if err := os.WriteFile(script, []byte("#!/bin/sh\ni=0\nwhile [ $i -lt 5 ]; do\n  echo \"test step $i\"\n  sleep 0.05\n  i=$((i+1))\ndone\n"), 0700); err != nil {
			t.Fatal(err)
		}
		res := runProjectVerifierCommandWithWatchdog(context.Background(), root, nil, 120*time.Millisecond, 2*time.Second, script)
		if !res.Passed {
			t.Fatalf("expected active verifier to pass past idle window, got: %+v", res)
		}
	})

	t.Run("hung_process_stopped_by_idle_watchdog", func(t *testing.T) {
		root := t.TempDir()
		script := filepath.Join(root, "hung.sh")
		if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 30\n"), 0700); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		res := runProjectVerifierCommandWithWatchdog(context.Background(), root, nil, 100*time.Millisecond, 2*time.Second, script)
		if res.Passed || !strings.Contains(res.Error, "Project verification stalled") {
			t.Fatalf("expected stalled error for hung process, got: %+v", res)
		}
		if elapsed := time.Since(start); elapsed > time.Second {
			t.Fatalf("hung process took too long to stop: %v", elapsed)
		}
	})

	t.Run("safety_ceiling_stops_runaway_active_process", func(t *testing.T) {
		root := t.TempDir()
		script := filepath.Join(root, "runaway.sh")
		if err := os.WriteFile(script, []byte("#!/bin/sh\nwhile true; do\n  echo \"tick\"\n  sleep 0.03\ndone\n"), 0700); err != nil {
			t.Fatal(err)
		}
		res := runProjectVerifierCommandWithWatchdog(context.Background(), root, nil, 500*time.Millisecond, 180*time.Millisecond, script)
		if res.Passed || !strings.Contains(res.Error, "safety ceiling") {
			t.Fatalf("expected safety ceiling error for runaway active process, got: %+v", res)
		}
	})
}

func TestRunnerSendHeartbeatReportsActivityAndExtendsContext(t *testing.T) {
	t.Setenv("CUAN_TASK_IDLE_TIMEOUT", "150ms")

	var mu sync.Mutex
	var captured []HeartbeatRequest
	extendedTarget := time.Now().UTC().Add(10 * time.Minute)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/runner/heartbeat" {
			var req HeartbeatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
				mu.Lock()
				captured = append(captured, req)
				mu.Unlock()
			}
			_ = json.NewEncoder(w).Encode(HeartbeatResponse{
				Status:            "ok",
				LeaseDurationSec:  30,
				ExtendedDeadlines: map[string]time.Time{"att-live": extendedTarget},
			})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{RunnerID: "runner-hb-test", RunnerServerURL: srv.URL}
	r := NewRunner(cfg, &detachedTestAdapter{})

	var extendedTo time.Time
	liveAct := &activeAttempt{
		attempt:        &task.TaskAttempt{ID: "att-live"},
		extendDeadline: func(d time.Time) { extendedTo = d },
	}
	liveAct.lastActivity.Store(time.Now().UnixNano())

	staleAct := &activeAttempt{
		attempt: &task.TaskAttempt{ID: "att-stale"},
	}
	staleAct.lastActivity.Store(time.Now().Add(-time.Second).UnixNano())

	r.activeAttempts["att-live"] = liveAct
	r.activeAttempts["att-stale"] = staleAct

	r.sendHeartbeat(context.Background(), "test-host")

	mu.Lock()
	defer mu.Unlock()
	if len(captured) != 1 {
		t.Fatalf("expected 1 heartbeat request, got %d", len(captured))
	}
	gotProgress := captured[0].ActiveProgressAttemptIDs
	if len(gotProgress) != 1 || gotProgress[0] != "att-live" {
		t.Fatalf("expected only att-live in ActiveProgressAttemptIDs, got %v", gotProgress)
	}
	if !extendedTo.Equal(extendedTarget) {
		t.Fatalf("expected runner to apply extended deadline %v, got %v", extendedTarget, extendedTo)
	}
}
