package server

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/terminal"
)

func TestZombieReaper_ReconcilesDeadProcessAndUnblocksProjectLane(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()

	projectID := "proj-zombie-test"
	taskID := "task-zombie-1"

	// 1. Create a task in doing status directly in store
	err := s.taskSvc.Store().CreateTask(ctx, &task.Task{
		ID:                taskID,
		Title:             "Zombie task",
		Instructions:      "Fix something",
		Status:            task.TaskStatusDoing,
		ProjectID:         projectID,
		ProjectIDs:        []string{projectID},
		ExecutionCategory: task.ExecutionCategoryCode,
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// 2. Create an attempt with status 'running'
	attempt, err := s.taskSvc.CreateAttempt(ctx, taskID, "tester", task.CreateAttemptRequest{
		RunnerID: "local_terminal",
	})
	if err != nil {
		t.Fatalf("failed to create attempt: %v", err)
	}
	attempt, err = s.taskSvc.UpdateAttemptStatus(ctx, attempt.ID, "tester", task.UpdateAttemptRequest{
		Status: task.AttemptStatusRunning,
	})
	if err != nil {
		t.Fatalf("failed to update attempt status to running: %v", err)
	}

	// 3. Create a launch and set its state to 'started'
	now := time.Now().UTC()
	l1, err := s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{
		TaskID:           taskID,
		ProjectID:        projectID,
		RequestID:        "req-orphan-1",
		SourceRequestID:  "req-source-orphan-1",
		OwnerID:          "owner",
		RoomID:           "room-1",
		AgentID:          "agent-1",
		InstructionsHash: task.LaunchInstructionsHash("Fix something"),
		CreatedAt:        now,
		AvailableAt:      now,
		ExpiresAt:        now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("failed to create launch: %v", err)
	}
	claimed, err := s.taskSvc.ClaimLaunch(ctx, now, "worker-1")
	if err != nil || claimed == nil {
		t.Fatalf("failed to claim launch: %v", err)
	}
	if err := s.taskSvc.FinishLaunch(ctx, claimed.ID, claimed.LeaseToken, "started", now, ""); err != nil {
		t.Fatalf("failed to start launch: %v", err)
	}

	// 4. In terminal manager, simulate that the project has an active session pointing to this task
	// But without a running OS process
	_, _ = s.terminalManager.AcquireProjectExecutionLease(projectID)

	// Check that the project lane is currently considered active/blocked
	laneActive, err := s.taskSvc.IsProjectLaneActive(ctx, projectID, task.ExecutionCategoryCode, "")
	if err != nil || !laneActive {
		t.Fatalf("expected project lane to be active/blocked before reaper, got active=%v, err=%v", laneActive, err)
	}

	// 5. Trigger ReconcileZombies
	if err := s.ReconcileZombies(ctx); err != nil {
		t.Fatalf("ReconcileZombies returned error: %v", err)
	}

	// 6. Verify attempt was marked as failed
	reapedAttempt, err := s.taskSvc.GetAttempt(ctx, attempt.ID)
	if err != nil {
		t.Fatalf("failed to get reaped attempt: %v", err)
	}
	if reapedAttempt.Status != task.AttemptStatusFailed {
		t.Fatalf("expected attempt status 'failed', got '%s'", reapedAttempt.Status)
	}

	// 7. A failed execution leaves the task blocked for recovery.
	reapedTask, err := s.taskSvc.Store().GetTask(ctx, taskID)
	if err != nil {
		t.Fatalf("failed to get reaped task: %v", err)
	}
	if reapedTask.Status != task.TaskStatusBlocked {
		t.Fatalf("expected task status 'blocked', got '%s'", reapedTask.Status)
	}

	// 8. Verify launch was marked as stopped
	reapedLaunch, err := s.taskSvc.GetLaunch(ctx, l1.ID)
	if err != nil {
		t.Fatalf("failed to get reaped launch: %v", err)
	}
	if reapedLaunch.State != "stopped" {
		t.Fatalf("expected launch state 'stopped', got '%s'", reapedLaunch.State)
	}

	// 9. Verify project lane is now completely UNBLOCKED and open
	laneActiveAfter, err := s.taskSvc.IsProjectLaneActive(ctx, projectID, task.ExecutionCategoryCode, "")
	if err != nil {
		t.Fatalf("error checking project lane active: %v", err)
	}
	if laneActiveAfter {
		t.Fatalf("expected project lane to be unblocked/free after reaper, but it is still active")
	}
}

func TestZombieReaper_ReconcilesHardTimeout(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()

	projectID := "proj-timeout-test"
	taskID := "task-timeout-1"

	err := s.taskSvc.Store().CreateTask(ctx, &task.Task{
		ID:                taskID,
		Title:             "Timed out task",
		Instructions:      "Long job",
		Status:            task.TaskStatusDoing,
		ProjectID:         projectID,
		ProjectIDs:        []string{projectID},
		ExecutionCategory: task.ExecutionCategoryCode,
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	attempt, err := s.taskSvc.CreateAttempt(ctx, taskID, "tester", task.CreateAttemptRequest{
		RunnerID: "local_terminal",
	})
	if err != nil {
		t.Fatalf("failed to create attempt: %v", err)
	}

	// Set attempt startedAt to 25 minutes ago
	oldTime := time.Now().UTC().Add(-25 * time.Minute)
	attempt.Status = task.AttemptStatusRunning
	attempt.CreatedAt = oldTime
	attempt.StartedAt = &oldTime
	_ = s.taskSvc.Store().UpdateAttempt(ctx, attempt)

	// Trigger ReconcileZombies (default timeout 20m)
	if err := s.ReconcileZombies(ctx); err != nil {
		t.Fatalf("ReconcileZombies failed: %v", err)
	}

	reapedAttempt, err := s.taskSvc.GetAttempt(ctx, attempt.ID)
	if err != nil {
		t.Fatalf("failed to get attempt: %v", err)
	}
	if reapedAttempt.Status != task.AttemptStatusFailed {
		t.Fatalf("expected timed out attempt to be marked 'failed', got '%s'", reapedAttempt.Status)
	}

	reapedTask, err := s.taskSvc.Store().GetTask(ctx, taskID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}
	if reapedTask.Status != task.TaskStatusBlocked {
		t.Fatalf("expected task status 'blocked', got '%s'", reapedTask.Status)
	}

	laneActive, err := s.taskSvc.IsProjectLaneActive(ctx, projectID, task.ExecutionCategoryCode, "")
	if err != nil || laneActive {
		t.Fatalf("expected project lane to be unblocked, got active=%v, err=%v", laneActive, err)
	}
}

func TestZombieReaper_PreservesProjectTerminalTask(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()

	projectID := "proj-preserve-test"
	taskID := "task-preserve-1"

	// 1. Create a dedicated project terminal task
	err := s.taskSvc.Store().CreateTask(ctx, &task.Task{
		ID:                taskID,
		Title:             "Terminal · cuan",
		Instructions:      "Interactive terminal",
		Status:            task.TaskStatusDoing,
		ProjectID:         projectID,
		ProjectIDs:        []string{projectID},
		ExecutionCategory: task.ExecutionCategoryCode,
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// 2. Create an attempt with status 'running'
	attempt, err := s.taskSvc.CreateAttempt(ctx, taskID, "tester", task.CreateAttemptRequest{
		RunnerID: "local_terminal",
	})
	if err != nil {
		t.Fatalf("failed to create attempt: %v", err)
	}
	attempt, err = s.taskSvc.UpdateAttemptStatus(ctx, attempt.ID, "tester", task.UpdateAttemptRequest{
		Status: task.AttemptStatusRunning,
	})
	if err != nil {
		t.Fatalf("failed to update attempt status to running: %v", err)
	}

	// Simulate expired execution deadline (created hours ago)
	past := time.Now().UTC().Add(-2 * time.Hour)
	attempt.CreatedAt = past
	attempt.ExecutionDeadline = &past
	_ = s.taskSvc.Store().UpdateAttempt(ctx, attempt)

	// 3. Start a persistent project session under the project ID
	tempDir := t.TempDir()
	sess, err := s.terminalManager.GetOrCreateProjectSession(ctx, projectID, tempDir, "/bin/sh", 80, 24)
	if err != nil {
		t.Fatalf("failed to create project session: %v", err)
	}

	// 4. Run ReconcileZombies while project terminal PTY is alive
	if err := s.ReconcileZombies(ctx); err != nil {
		t.Fatalf("ReconcileZombies failed: %v", err)
	}

	// 5. Verify the attempt and task were NOT reaped
	runningAttempt, err := s.taskSvc.GetAttempt(ctx, attempt.ID)
	if err != nil {
		t.Fatalf("failed to get attempt: %v", err)
	}
	if runningAttempt.Status != task.AttemptStatusRunning {
		t.Fatalf("expected running attempt to be preserved, got '%s'", runningAttempt.Status)
	}

	runningTask, err := s.taskSvc.Store().GetTask(ctx, taskID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}
	if runningTask.Status != task.TaskStatusDoing {
		t.Fatalf("expected task status to remain 'doing', got '%s'", runningTask.Status)
	}

	// 6. Now close the project session and verify it is reaped
	_ = sess.Close()
	if err := s.ReconcileZombies(ctx); err != nil {
		t.Fatalf("ReconcileZombies failed: %v", err)
	}

	reapedAttempt, err := s.taskSvc.GetAttempt(ctx, attempt.ID)
	if err != nil {
		t.Fatalf("failed to get attempt: %v", err)
	}
	if reapedAttempt.Status != task.AttemptStatusFailed {
		t.Fatalf("expected closed attempt to be marked 'failed', got '%s'", reapedAttempt.Status)
	}
}

func TestZombieReaper_PreservesDecoupledProjectSocketWorker(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()

	projectID := "proj-preserve-sock-test"
	taskID := "task-preserve-sock-1"

	// 1. Create a dedicated project terminal task
	err := s.taskSvc.Store().CreateTask(ctx, &task.Task{
		ID:                taskID,
		Title:             "Terminal · test",
		Instructions:      "Interactive terminal",
		Status:            task.TaskStatusDoing,
		ProjectID:         projectID,
		ProjectIDs:        []string{projectID},
		ExecutionCategory: task.ExecutionCategoryCode,
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// 2. Create an attempt with status 'running'
	attempt, err := s.taskSvc.CreateAttempt(ctx, taskID, "tester", task.CreateAttemptRequest{
		RunnerID: "local_terminal",
	})
	if err != nil {
		t.Fatalf("failed to create attempt: %v", err)
	}
	attempt, err = s.taskSvc.UpdateAttemptStatus(ctx, attempt.ID, "tester", task.UpdateAttemptRequest{
		Status: task.AttemptStatusRunning,
	})
	if err != nil {
		t.Fatalf("failed to update attempt status to running: %v", err)
	}

	// Simulate expired execution deadline (created hours ago)
	past := time.Now().UTC().Add(-2 * time.Hour)
	attempt.CreatedAt = past
	attempt.ExecutionDeadline = &past
	_ = s.taskSvc.Store().UpdateAttempt(ctx, attempt)

	// 3. Simulate decoupled project runner Unix domain socket
	sockPath := terminal.ProjectSocketPath(projectID)
	_ = os.Remove(sockPath)
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("failed to create fake decoupled socket: %v", err)
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(sockPath)
	}()

	// 4. Run ReconcileZombies while decoupled Unix socket is active
	if err := s.ReconcileZombies(ctx); err != nil {
		t.Fatalf("ReconcileZombies failed: %v", err)
	}

	// 5. Verify attempt was NOT reaped
	runningAttempt, err := s.taskSvc.GetAttempt(ctx, attempt.ID)
	if err != nil {
		t.Fatalf("failed to get attempt: %v", err)
	}
	if runningAttempt.Status != task.AttemptStatusRunning {
		t.Fatalf("expected running attempt to be preserved for decoupled socket, got '%s'", runningAttempt.Status)
	}

	runningTask, err := s.taskSvc.Store().GetTask(ctx, taskID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}
	if runningTask.Status != task.TaskStatusDoing {
		t.Fatalf("expected task status to remain 'doing', got '%s'", runningTask.Status)
	}
}
