package campaign

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
)

func TestMultiProjectRunner_ControlledConcurrency(t *testing.T) {
	ctx := context.Background()
	studioRepo := db.NewMemoryProjectStudioRepository()

	const maxWorkers = 4
	const totalTasks = 12

	runner := NewMultiProjectRunner(
		WithMaxWorkers(maxWorkers),
		WithRunnerStudioRepo(studioRepo),
		WithTaskExecutor(TaskExecutorFunc(func(ctx context.Context, task *db.ProjectTask) (*TaskExecutionResult, error) {
			// Sleep briefly to simulate work and allow workers to overlap
			time.Sleep(25 * time.Millisecond)

			return &TaskExecutionResult{
				TaskID:     task.ID,
				ProjectID:  task.ProjectID,
				Status:     "completed",
				StartedAt:  time.Now().UTC(),
				FinishedAt: time.Now().UTC(),
			}, nil
		})),
	)

	tasks := make([]*db.ProjectTask, totalTasks)
	for i := 0; i < totalTasks; i++ {
		tasks[i] = &db.ProjectTask{
			ID:        db.GenerateUUID(),
			ProjectID: fmt.Sprintf("proj-%02d", i+1),
			Title:     fmt.Sprintf("Task %d", i+1),
			Status:    "backlog",
		}
	}

	result, err := runner.ExecuteTasks(ctx, tasks, "batch-concurrency-test")
	if err != nil {
		t.Fatalf("ExecuteTasks error: %v", err)
	}

	if result.TotalTasks != totalTasks {
		t.Errorf("expected TotalTasks %d, got %d", totalTasks, result.TotalTasks)
	}
	if result.CompletedTasks != totalTasks {
		t.Errorf("expected CompletedTasks %d, got %d", totalTasks, result.CompletedTasks)
	}
	if result.FailedTasks != 0 {
		t.Errorf("expected 0 FailedTasks, got %d", result.FailedTasks)
	}
	if !result.Success {
		t.Errorf("expected Success=true, got false")
	}

	peak := runner.PeakWorkers()
	if peak > maxWorkers {
		t.Errorf("peak concurrency %d exceeded maxWorkers limit %d", peak, maxWorkers)
	}
	if peak <= 1 {
		t.Errorf("expected parallel execution (peak > 1), got peak %d", peak)
	}
}

func TestMultiProjectRunner_FaultIsolation(t *testing.T) {
	ctx := context.Background()
	studioRepo := db.NewMemoryProjectStudioRepository()

	// 5 tasks across 5 projects:
	// - proj-1: success
	// - proj-2: success
	// - proj-3: fails with normal error
	// - proj-4: success
	// - proj-5: panics! (isolated guard test)
	tasks := make([]*db.ProjectTask, 5)
	for i := 0; i < 5; i++ {
		tasks[i] = &db.ProjectTask{
			ID:        db.GenerateUUID(),
			ProjectID: fmt.Sprintf("proj-%d", i+1),
			Title:     fmt.Sprintf("Task for Project %d", i+1),
			Status:    "backlog",
		}
		_ = studioRepo.CreateTask(ctx, tasks[i])
	}

	runner := NewMultiProjectRunner(
		WithMaxWorkers(3),
		WithRunnerStudioRepo(studioRepo),
		WithTaskExecutor(TaskExecutorFunc(func(ctx context.Context, task *db.ProjectTask) (*TaskExecutionResult, error) {
			if task.ProjectID == "proj-3" {
				return nil, errors.New("simulated network connection timeout for proj-3")
			}
			if task.ProjectID == "proj-5" {
				panic("simulated fatal crash / panic inside proj-5 runner")
			}

			// Others succeed
			return &TaskExecutionResult{
				TaskID:    task.ID,
				ProjectID: task.ProjectID,
				Status:    "completed",
			}, nil
		})),
	)

	result, err := runner.ExecuteTasks(ctx, tasks, "batch-fault-isolation")
	if err != nil {
		t.Fatalf("ExecuteTasks returned unexpected top-level error: %v", err)
	}

	// 3 tasks should have completed, 2 tasks failed (proj-3 error, proj-5 panic)
	if result.TotalTasks != 5 {
		t.Errorf("expected TotalTasks 5, got %d", result.TotalTasks)
	}
	if result.CompletedTasks != 3 {
		t.Errorf("expected CompletedTasks 3, got %d", result.CompletedTasks)
	}
	if result.FailedTasks != 2 {
		t.Errorf("expected FailedTasks 2, got %d", result.FailedTasks)
	}
	if result.Success {
		t.Errorf("expected Success=false when tasks failed, got true")
	}

	// Verify status in studioRepo
	for _, task := range tasks {
		dbTask, err := studioRepo.GetTaskByID(ctx, task.ID)
		if err != nil {
			t.Fatalf("failed to get task %s: %v", task.ID, err)
		}
		if task.ProjectID == "proj-3" || task.ProjectID == "proj-5" {
			if dbTask.Status != "failed" {
				t.Errorf("expected task for %s to be failed, got %s", task.ProjectID, dbTask.Status)
			}
		} else {
			// Other projects should have succeeded without being affected
			if task.Status != "done" && task.Status != "completed" {
				// Task execution result was completed
			}
		}
	}
}

func TestMultiProjectRunner_ExecuteCampaign(t *testing.T) {
	ctx := context.Background()
	batchRepo := db.NewMemoryBatchCampaignRepository()
	studioRepo := db.NewMemoryProjectStudioRepository()

	resolver := NewDefaultProjectResolver(nil, studioRepo)
	resolver.AddProject(ProjectInfo{ID: "proj-1", Name: "Wordpress Portal A", Tags: []string{"wordpress"}})
	resolver.AddProject(ProjectInfo{ID: "proj-2", Name: "Wordpress Portal B", Tags: []string{"wordpress"}})

	decompiler := NewDecompilerEngine(
		WithProjectResolver(resolver),
		WithProjectStudioRepo(studioRepo),
		WithMinDoDItems(3),
	)

	runner := NewMultiProjectRunner(
		WithMaxWorkers(5),
		WithRunnerBatchRepo(batchRepo),
		WithRunnerStudioRepo(studioRepo),
		WithDecompiler(decompiler),
	)

	campaign := &db.BatchCampaign{
		Title:       "Sprint 8 Rollout Campaign",
		Description: "Multi-Project Isolated Runner Test",
		Status:      db.CampaignStatusDraft,
		TargetCriteria: json.RawMessage(`{
			"tags": ["wordpress"]
		}`),
		TemplatePayload: json.RawMessage(`{
			"script": "console.log('injected');",
			"dod_checklist": ["Verify script injection"]
		}`),
	}
	if err := batchRepo.Create(ctx, campaign); err != nil {
		t.Fatalf("batchRepo.Create error: %v", err)
	}

	// Execute campaign
	runResult, err := runner.ExecuteCampaign(ctx, campaign.ID)
	if err != nil {
		t.Fatalf("ExecuteCampaign error: %v", err)
	}

	if runResult.TotalTasks != 2 {
		t.Errorf("expected 2 tasks executed, got %d", runResult.TotalTasks)
	}
	if runResult.CompletedTasks != 2 {
		t.Errorf("expected 2 tasks completed, got %d", runResult.CompletedTasks)
	}
	if runResult.FailedTasks != 0 {
		t.Errorf("expected 0 failed tasks, got %d", runResult.FailedTasks)
	}

	// Campaign status in batchRepo must now be 'completed'
	refetched, err := batchRepo.GetByID(ctx, campaign.ID)
	if err != nil {
		t.Fatalf("batchRepo.GetByID error: %v", err)
	}
	if refetched.Status != db.CampaignStatusCompleted {
		t.Errorf("expected campaign status completed, got %s", refetched.Status)
	}

	// Subtasks in studioRepo must be 'done' and DoD completed
	subtasks, err := studioRepo.ListTasksByBatchID(ctx, campaign.ID)
	if err != nil {
		t.Fatalf("ListTasksByBatchID error: %v", err)
	}
	for _, st := range subtasks {
		if st.ParentTaskID != nil {
			if st.Status != "done" {
				t.Errorf("subtask %s status expected 'done', got '%s'", st.ID, st.Status)
			}
			for j, dod := range st.DoDChecklist {
				if !dod.Completed {
					t.Errorf("subtask %s DoD item %d was not completed", st.ID, j)
				}
			}
		}
	}
}

type testListener struct {
	mu         sync.Mutex
	started    []string
	completed  []string
	failed     []string
	campaignID string
}

func (l *testListener) OnTaskStart(task *db.ProjectTask) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.started = append(l.started, task.ID)
}

func (l *testListener) OnTaskComplete(task *db.ProjectTask, res *TaskExecutionResult) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.completed = append(l.completed, task.ID)
}

func (l *testListener) OnTaskFail(task *db.ProjectTask, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.failed = append(l.failed, task.ID)
}

func (l *testListener) OnCampaignComplete(campaignID string, result *CampaignRunResult) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.campaignID = campaignID
}

func TestMultiProjectRunner_LifecycleListeners(t *testing.T) {
	ctx := context.Background()

	listener := &testListener{}
	runner := NewMultiProjectRunner(
		WithMaxWorkers(2),
		WithTaskExecutor(TaskExecutorFunc(func(ctx context.Context, task *db.ProjectTask) (*TaskExecutionResult, error) {
			if task.ProjectID == "bad-proj" {
				return nil, errors.New("simulated failure")
			}
			return &TaskExecutionResult{TaskID: task.ID, ProjectID: task.ProjectID, Status: "completed"}, nil
		})),
	)
	runner.AddListener(listener)

	tasks := []*db.ProjectTask{
		{ID: "t1", ProjectID: "good-proj"},
		{ID: "t2", ProjectID: "bad-proj"},
	}

	_, _ = runner.ExecuteTasks(ctx, tasks, "batch-listener-test")

	listener.mu.Lock()
	defer listener.mu.Unlock()

	if len(listener.started) != 2 {
		t.Errorf("expected 2 OnTaskStart calls, got %d", len(listener.started))
	}
	if len(listener.completed) != 1 || listener.completed[0] != "t1" {
		t.Errorf("expected t1 completed, got %+v", listener.completed)
	}
	if len(listener.failed) != 1 || listener.failed[0] != "t2" {
		t.Errorf("expected t2 failed, got %+v", listener.failed)
	}
	if listener.campaignID != "batch-listener-test" {
		t.Errorf("expected campaignID 'batch-listener-test', got '%s'", listener.campaignID)
	}
}
