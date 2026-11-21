package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestCuanRunnerDispatcherCutover_AttemptQueuedAndClaimed(t *testing.T) {
	srv, token := setupTestChatServer(t)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	ctx := context.Background()
	taskID := "task-cuan-runner-cutover-test"
	_ = srv.taskSvc.Store().CreateTask(ctx, &task.Task{
		ID:           taskID,
		Title:        "Cutover Task for cuan-runner",
		Instructions: "Verify dispatcher cutover to cuan-runner daemon",
	})
	_ = srv.agentRepo.CreateAgent(ctx, &cuanDB.Agent{
		ID:      "worker-runner",
		Name:    "Worker Runner",
		Harness: "agy",
		Enabled: true,
		Status:  cuanDB.AgentStatusActive,
	})

	// 1. Dispatch task launch via POST /api/v1/tasks/{id}/run-agent (default without specifying local_terminal)
	runURL := ts.URL + "/api/v1/tasks/" + taskID + "/run-agent"
	reqPayload := `{"agent_id":"worker-runner","prompt":"Execute via cuan-runner"}`
	req, err := http.NewRequestWithContext(ctx, "POST", runURL, bytes.NewReader([]byte(reqPayload)))
	if err != nil {
		t.Fatalf("failed to create run-agent request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("run-agent request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		t.Fatalf("expected 200 OK for run-agent, got %d: %v", resp.StatusCode, errBody)
	}

	var runResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&runResp); err != nil {
		t.Fatalf("failed to decode run-agent response: %v", err)
	}

	// Verify attempt is created with RunnerID: "cuan-runner" and status "queued"
	if runResp["status"] != string(task.AttemptStatusQueued) {
		t.Errorf("expected status '%s', got '%v'", task.AttemptStatusQueued, runResp["status"])
	}
	if runResp["runner_id"] != "cuan-runner" {
		t.Errorf("expected runner_id 'cuan-runner', got '%v'", runResp["runner_id"])
	}
	attemptID, ok := runResp["attempt_id"].(string)
	if !ok || attemptID == "" {
		t.Fatalf("expected non-empty attempt_id in response")
	}

	// Verify in database / task store
	detail, err := srv.taskSvc.GetTaskDetail(ctx, taskID)
	if err != nil {
		t.Fatalf("GetTaskDetail failed: %v", err)
	}
	if detail.Status != task.TaskStatusQueued {
		t.Errorf("expected task status '%s', got '%s'", task.TaskStatusQueued, detail.Status)
	}
	if detail.LatestAttempt == nil {
		t.Fatalf("expected task to have a latest attempt")
	}
	if detail.LatestAttempt.RunnerID != "cuan-runner" {
		t.Errorf("expected attempt RunnerID 'cuan-runner', got '%s'", detail.LatestAttempt.RunnerID)
	}
	if detail.LatestAttempt.Status != task.AttemptStatusQueued {
		t.Errorf("expected attempt status '%s', got '%s'", task.AttemptStatusQueued, detail.LatestAttempt.Status)
	}

	// 2. cuan-runner daemon claims the queued attempt from the database queue
	claimResp, err := srv.runnerSvc.Claim(ctx, runner.ClaimTaskRequest{
		RunnerID:        "cuan-runner",
		SupportedAgents: []string{"agy", "worker-runner"},
	}, 30*time.Second)
	if err != nil {
		t.Fatalf("runnerSvc.Claim failed: %v", err)
	}
	if !claimResp.Claimed {
		t.Fatalf("expected cuan-runner to claim the queued attempt, but claim was false")
	}
	if claimResp.Attempt == nil || claimResp.Attempt.ID != attemptID {
		t.Fatalf("expected claimed attempt to match %s, got %+v", attemptID, claimResp.Attempt)
	}
	if claimResp.Attempt.Status != task.AttemptStatusRunning {
		t.Errorf("expected claimed attempt status '%s', got '%s'", task.AttemptStatusRunning, claimResp.Attempt.Status)
	}
	if claimResp.Attempt.RunnerID != "cuan-runner" {
		t.Errorf("expected claimed attempt RunnerID 'cuan-runner', got '%s'", claimResp.Attempt.RunnerID)
	}

	// Verify task transitioned to doing after claim
	detailAfterClaim, err := srv.taskSvc.GetTaskDetail(ctx, taskID)
	if err != nil {
		t.Fatalf("GetTaskDetail after claim failed: %v", err)
	}
	if detailAfterClaim.Status != task.TaskStatusDoing {
		t.Errorf("expected task status '%s' after claim, got '%s'", task.TaskStatusDoing, detailAfterClaim.Status)
	}
}

func TestTaskLaunchQueue_CutoverToCuanRunner(t *testing.T) {
	srv, _ := setupTestChatServer(t)
	ctx := context.Background()

	now := time.Now().UTC()
	msgID := cuanDB.GenerateUUID()
	reqID := cuanDB.GenerateUUID()
	roomID := "room_launch_01"

	instructions := "Verify task_launch_queue cuts over to cuan-runner"
	taskID := "task-launch-cuan-runner"
	projID := "proj_cutover"
	if srv.projectSvc != nil && srv.projectSvc.Store() != nil {
		_ = srv.projectSvc.Store().CreateProject(ctx, &project.Project{
			ID:   projID,
			Name: "Cutover Project",
		})
	}
	if srv.projectRepo != nil {
		_ = srv.projectRepo.CreateProject(ctx, &cuanDB.Project{
			ID:   projID,
			Name: "Cutover Project",
		})
	}
	_ = srv.taskSvc.Store().CreateTask(ctx, &task.Task{
		ID:              taskID,
		Title:           "Launch Queue Task",
		Instructions:    instructions,
		ProjectID:       projID,
		SourceRoomID:    roomID,
		SourceMessageID: msgID,
	})
	_ = srv.agentRepo.CreateAgent(ctx, &cuanDB.Agent{
		ID:      "agent_launch",
		Name:    "Agent Launch",
		Harness: "agy",
		Enabled: true,
		Status:  cuanDB.AgentStatusActive,
	})

	savedLaunch, err := srv.taskSvc.SaveLaunch(ctx, task.TaskLaunch{
		TaskID:           taskID,
		ProjectID:        projID,
		RequestID:        reqID,
		OwnerID:          "owner",
		RoomID:           roomID,
		SourceRequestID:  reqID,
		AgentID:          "agent_launch",
		InstructionsHash: task.LaunchInstructionsHash(instructions),
		CreatedAt:        now,
		AvailableAt:      now,
		ExpiresAt:        now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("SaveLaunch failed: %v", err)
	}

	claimedLaunch, err := srv.taskSvc.ClaimLaunch(ctx, now, "lease_01", savedLaunch.ID)
	if err != nil {
		t.Fatalf("ClaimLaunch failed: %v", err)
	}
	if claimedLaunch == nil {
		t.Fatalf("expected launch to be claimed")
	}

	// Setup inbox request for authorization
	if repo, ok := srv.projectStudioRepo.(cuanDB.InboxRequestRepository); ok {
		_, _, _ = repo.AcceptInboxRequest(ctx, cuanDB.InboxRequest{
			RequestID: reqID,
			MessageID: msgID,
			OwnerID:   "owner",
			RoomID:    roomID,
			State:     "queued",
			Deadline:  now.Add(time.Hour),
		}, &cuanDB.ProjectMessage{
			ID:         msgID,
			RoomID:     roomID,
			AuthorType: "user",
			Content:    "Run execution",
			CreatedAt:  now,
		})
	}

	// Execute queued task launch
	result := srv.executeQueuedTaskLaunch(ctx, *claimedLaunch, false)
	t.Logf("executeQueuedTaskLaunch result: %s", result)

	// Verify attempt created with cuan-runner and status queued
	detail, err := srv.taskSvc.GetTaskDetail(ctx, taskID)
	if err != nil {
		t.Fatalf("GetTaskDetail failed: %v", err)
	}
	if detail.LatestAttempt == nil {
		t.Fatalf("expected latest attempt to exist")
	}
	if detail.LatestAttempt.RunnerID != "cuan-runner" {
		t.Errorf("expected RunnerID 'cuan-runner', got '%s'", detail.LatestAttempt.RunnerID)
	}
	if detail.LatestAttempt.Status != task.AttemptStatusQueued {
		t.Errorf("expected attempt status '%s', got '%s'", task.AttemptStatusQueued, detail.LatestAttempt.Status)
	}
}
