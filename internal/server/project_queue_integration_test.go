package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestProjectLevelTaskQueueAndLock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping project queue concurrency test in short mode")
	}
	s, _, call, _ := inboxFixture(t)
	prepareTaskReporter(t, s)

	ctx := context.Background()
	projectID := "proj_concurrency_serial_1"

	// Create a test project with a valid local path
	projDir := t.TempDir()
	store := project.NewMemoryStore()
	store.CreateProject(ctx, &project.Project{
		ID:        projectID,
		Name:      "Concurrency Test Project",
		LocalPath: projDir,
	})
	s.projectSvc = project.NewService(store, nil, "", t.TempDir())

	// 1. Create two tasks on the same project
	t1Payload := `{"request_id":"` + db.GenerateUUID() + `","project_id":"` + projectID + `","title":"Task 1","instructions":"Instructions 1"}`
	res1 := call("POST", "/api/v1/tasks", t1Payload)
	if res1.Code != http.StatusCreated {
		t.Fatalf("create task 1 failed: %d %s", res1.Code, res1.Body.String())
	}
	var task1 task.Task
	_ = json.Unmarshal(res1.Body.Bytes(), &task1)

	t2Payload := `{"request_id":"` + db.GenerateUUID() + `","project_id":"` + projectID + `","title":"Task 2","instructions":"Instructions 2"}`
	res2 := call("POST", "/api/v1/tasks", t2Payload)
	if res2.Code != http.StatusCreated {
		t.Fatalf("create task 2 failed: %d %s", res2.Code, res2.Body.String())
	}
	var task2 task.Task
	_ = json.Unmarshal(res2.Body.Bytes(), &task2)

	// 2. Start Task 1
	run1 := call("POST", "/api/v1/tasks/"+task1.ID+"/run-agent", `{"request_id":"`+db.GenerateUUID()+`","agent_id":"agent_lifecycle"}`)
	if run1.Code != http.StatusOK {
		t.Fatalf("start task 1 failed: %d %s", run1.Code, run1.Body.String())
	}

	// Verify project lock is active
	if !s.isProjectActive(ctx, projectID, "") {
		t.Fatalf("expected project to be active while task 1 is running")
	}

	// 3. Attempt to run Task 2 concurrently while Task 1 is running
	run2 := call("POST", "/api/v1/tasks/"+task2.ID+"/run-agent", `{"request_id":"`+db.GenerateUUID()+`","agent_id":"agent_lifecycle"}`)
	if run2.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for concurrent project task, got %d: %s", run2.Code, run2.Body.String())
	}
	if !strings.Contains(run2.Body.String(), "project_concurrency_busy") {
		t.Fatalf("expected code project_concurrency_busy, got %s", run2.Body.String())
	}

	// Verify Task 2 transitioned to 'queued' status
	detail2, err := s.taskSvc.GetTaskDetail(ctx, task2.ID)
	if err != nil {
		t.Fatalf("get task 2 detail failed: %v", err)
	}
	if detail2.Status != task.TaskStatusQueued && detail2.Status != "queued" {
		t.Fatalf("expected task 2 status to be queued, got %s", detail2.Status)
	}

	// 3b. Verify Scrum Master Live Runner State & API Endpoint
	resState := call("GET", "/api/v1/projects/"+projectID+"/runner-state", "")
	if resState.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from runner-state endpoint, got %d: %s", resState.Code, resState.Body.String())
	}
	var runnerState ProjectRunnerStateResponse
	if err := json.Unmarshal(resState.Body.Bytes(), &runnerState); err != nil {
		t.Fatalf("failed to decode runner state response: %v", err)
	}
	if !runnerState.IsLocked {
		t.Fatalf("expected is_locked to be true while task 1 is executing")
	}
	if runnerState.ActiveRunner == nil || runnerState.ActiveRunner.TaskID != task1.ID {
		t.Fatalf("expected active_runner to be task1 (%s), got: %+v", task1.ID, runnerState.ActiveRunner)
	}
	if len(runnerState.QueuedTasks) != 1 || runnerState.QueuedTasks[0].TaskID != task2.ID {
		t.Fatalf("expected 1 queued task (task2 %s), got: %+v", task2.ID, runnerState.QueuedTasks)
	}
	if runnerState.QueuedTasks[0].QueuePosition != 1 {
		t.Errorf("expected queue_position 1, got %d", runnerState.QueuedTasks[0].QueuePosition)
	}

	// Verify Scrum Master Prompt context contains live runner snapshot
	promptSnippet := s.formatProjectRunnerLiveState(ctx, projectID)
	if !strings.Contains(promptSnippet, "Concurrency Lock Status: [LOCKED]") {
		t.Fatalf("expected SM prompt snippet to contain LOCKED status, got: %s", promptSnippet)
	}
	if !strings.Contains(promptSnippet, task1.ID) {
		t.Fatalf("expected SM prompt snippet to contain active task ID %s, got: %s", task1.ID, promptSnippet)
	}
	if !strings.Contains(promptSnippet, task2.ID) {
		t.Fatalf("expected SM prompt snippet to contain queued task ID %s, got: %s", task2.ID, promptSnippet)
	}

	// 4. Wait for Task 1 to finish
	waitWorkspaceTask(t, s, task1.ID, "succeeded")

	// Let session and launches settle
	time.Sleep(100 * time.Millisecond)

	// 5. Now Task 2 can be executed serially
	run2After := call("POST", "/api/v1/tasks/"+task2.ID+"/run-agent", `{"request_id":"`+db.GenerateUUID()+`","agent_id":"agent_lifecycle"}`)
	if run2After.Code != http.StatusOK {
		t.Fatalf("expected task 2 to succeed after task 1 finished, got %d: %s", run2After.Code, run2After.Body.String())
	}

	// Wait for Task 2 to finish
	waitWorkspaceTask(t, s, task2.ID, "succeeded")

	// Verify history shows both tasks ran sequentially without overlapping
	h1 := s.terminalManager.GetTaskHistory(task1.ID)
	h2 := s.terminalManager.GetTaskHistory(task2.ID)
	if len(h1) == 0 || len(h2) == 0 {
		t.Fatalf("expected histories for both tasks, got h1=%d, h2=%d", len(h1), len(h2))
	}
	if h2[0].StartedAt.Before(h1[0].FinishedAt) {
		t.Errorf("expected serial execution: task 2 started at %v before task 1 finished at %v", h2[0].StartedAt, h1[0].FinishedAt)
	}

	// Let session and launches settle
	time.Sleep(150 * time.Millisecond)

	// Verify runner state after all tasks finish
	resStateAfter := call("GET", "/api/v1/projects/"+projectID+"/runner-state", "")
	var runnerStateAfter ProjectRunnerStateResponse
	_ = json.Unmarshal(resStateAfter.Body.Bytes(), &runnerStateAfter)
	if runnerStateAfter.IsLocked {
		t.Fatalf("expected is_locked to be false after tasks finish")
	}
	if len(runnerStateAfter.QueuedTasks) != 0 {
		t.Fatalf("expected 0 queued tasks after tasks finish, got %d", len(runnerStateAfter.QueuedTasks))
	}
}
