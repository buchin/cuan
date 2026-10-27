package campaign

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
)

func TestClassifyTaskStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"backlog", "pending"},
		{"pending", "pending"},
		{"queued", "pending"},
		{"todo", "pending"},
		{"draft", "pending"},
		{"doing", "running"},
		{"running", "running"},
		{"in_progress", "running"},
		{"review", "review"},
		{"in_review", "review"},
		{"done", "completed"},
		{"completed", "completed"},
		{"success", "completed"},
		{"failed", "failed"},
		{"blocked", "failed"},
		{"error", "failed"},
		{"unknown_status", "pending"},
	}

	for _, tc := range tests {
		got := ClassifyTaskStatus(tc.input)
		if got != tc.expected {
			t.Errorf("ClassifyTaskStatus(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestRollupAggregator_CalculateRollup(t *testing.T) {
	aggregator := NewRollupAggregator()
	campaignID := db.GenerateUUID()

	// 1. Empty tasks -> draft status and 0%
	emptyRollup := aggregator.CalculateRollup([]*db.ProjectTask{}, campaignID)
	if emptyRollup.TotalTasks != 0 || emptyRollup.TotalTarget != 0 {
		t.Fatalf("expected 0 tasks, got %d", emptyRollup.TotalTasks)
	}
	if emptyRollup.SuccessPercentage != 0.0 {
		t.Fatalf("expected 0%% success percentage, got %f", emptyRollup.SuccessPercentage)
	}
	if emptyRollup.Status != "draft" {
		t.Fatalf("expected status 'draft', got %q", emptyRollup.Status)
	}

	// 2. 10 subtasks with mixed status
	tasks := make([]*db.ProjectTask, 10)
	for i := 0; i < 10; i++ {
		status := "backlog"
		switch {
		case i < 4:
			status = "done" // 4 completed
		case i == 4:
			status = "doing" // 1 running
		case i == 5:
			status = "review" // 1 review
		case i == 6:
			status = "failed" // 1 failed
		default:
			status = "backlog" // 3 pending
		}

		tasks[i] = &db.ProjectTask{
			ID:        fmt.Sprintf("task-%d", i),
			ProjectID: fmt.Sprintf("proj-%d", i),
			BatchID:   &campaignID,
			Status:    status,
		}
	}

	m := aggregator.CalculateRollup(tasks, campaignID)
	if m.TotalTasks != 10 || m.TotalTarget != 10 {
		t.Errorf("expected total 10, got total_tasks=%d, total_target=%d", m.TotalTasks, m.TotalTarget)
	}
	if m.CompletedTasks != 4 {
		t.Errorf("expected 4 completed, got %d", m.CompletedTasks)
	}
	if m.RunningTasks != 1 {
		t.Errorf("expected 1 running, got %d", m.RunningTasks)
	}
	if m.ReviewTasks != 1 {
		t.Errorf("expected 1 review, got %d", m.ReviewTasks)
	}
	if m.FailedTasks != 1 {
		t.Errorf("expected 1 failed, got %d", m.FailedTasks)
	}
	if m.PendingTasks != 3 {
		t.Errorf("expected 3 pending, got %d", m.PendingTasks)
	}
	if m.SuccessPercentage != 40.0 {
		t.Errorf("expected 40.0%% success percentage, got %f", m.SuccessPercentage)
	}
	if m.Status != "running" {
		t.Errorf("expected status 'running', got %q", m.Status)
	}

	// 3. All 10 tasks completed -> 100% and 'completed' status
	for i := 0; i < 10; i++ {
		tasks[i].Status = "done"
	}
	completedRollup := aggregator.CalculateRollup(tasks, campaignID)
	if completedRollup.CompletedTasks != 10 || completedRollup.SuccessPercentage != 100.0 {
		t.Errorf("expected 100%% completed, got %f%%", completedRollup.SuccessPercentage)
	}
	if completedRollup.Status != "completed" {
		t.Errorf("expected 'completed' status, got %q", completedRollup.Status)
	}

	// 4. All failed -> 0% and 'failed' status
	for i := 0; i < 10; i++ {
		tasks[i].Status = "failed"
	}
	failedRollup := aggregator.CalculateRollup(tasks, campaignID)
	if failedRollup.FailedTasks != 10 || failedRollup.SuccessPercentage != 0.0 {
		t.Errorf("expected 0%% success, got %f%%", failedRollup.SuccessPercentage)
	}
	if failedRollup.Status != "failed" {
		t.Errorf("expected 'failed' status, got %q", failedRollup.Status)
	}
}

func TestRollupAggregator_ParentEpicFiltering(t *testing.T) {
	aggregator := NewRollupAggregator()
	campaignID := db.GenerateUUID()
	parentEpicID := db.GenerateUUID()

	// 1 Parent Epic task + 3 Child Subtasks
	epicTask := &db.ProjectTask{
		ID:           parentEpicID,
		ProjectID:    "proj-1",
		BatchID:      &campaignID,
		ParentTaskID: nil, // Epic has no parent
		Title:        "[Campaign Epic] Batch Rollout",
		Status:       "doing",
	}

	child1 := &db.ProjectTask{
		ID:           "sub-1",
		ProjectID:    "proj-1",
		BatchID:      &campaignID,
		ParentTaskID: &parentEpicID,
		Title:        "[proj-1] Subtask",
		Status:       "done",
	}
	child2 := &db.ProjectTask{
		ID:           "sub-2",
		ProjectID:    "proj-2",
		BatchID:      &campaignID,
		ParentTaskID: &parentEpicID,
		Title:        "[proj-2] Subtask",
		Status:       "doing",
	}
	child3 := &db.ProjectTask{
		ID:           "sub-3",
		ProjectID:    "proj-3",
		BatchID:      &campaignID,
		ParentTaskID: &parentEpicID,
		Title:        "[proj-3] Subtask",
		Status:       "backlog",
	}

	tasks := []*db.ProjectTask{epicTask, child1, child2, child3}
	rollup := aggregator.CalculateRollup(tasks, campaignID)

	// Total targets should be 3 (excluding the parent epic)
	if rollup.TotalTasks != 3 || rollup.TotalTarget != 3 {
		t.Fatalf("expected 3 total targets (excluding epic), got %d", rollup.TotalTasks)
	}
	if rollup.CompletedTasks != 1 {
		t.Errorf("expected 1 completed, got %d", rollup.CompletedTasks)
	}
	if rollup.RunningTasks != 1 {
		t.Errorf("expected 1 running, got %d", rollup.RunningTasks)
	}
	if rollup.PendingTasks != 1 {
		t.Errorf("expected 1 pending, got %d", rollup.PendingTasks)
	}
}

func TestRollupAggregator_InMemoryAndRepositoryRefresh(t *testing.T) {
	ctx := context.Background()
	studioRepo := db.NewMemoryProjectStudioRepository()
	aggregator := NewRollupAggregator(WithAggregatorStudioRepo(studioRepo))

	campaignID := db.GenerateUUID()

	// Seed 5 tasks into studioRepo
	for i := 0; i < 5; i++ {
		task := &db.ProjectTask{
			ID:        db.GenerateUUID(),
			ProjectID: db.GenerateUUID(),
			BatchID:   &campaignID,
			Title:     fmt.Sprintf("Task %d", i),
			Status:    "backlog",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := studioRepo.CreateTask(ctx, task); err != nil {
			t.Fatalf("failed to create seed task: %v", err)
		}
	}

	// Refresh from repository
	rollup, err := aggregator.RefreshFromRepository(ctx, campaignID)
	if err != nil {
		t.Fatalf("RefreshFromRepository failed: %v", err)
	}
	if rollup == nil || rollup.TotalTasks != 5 {
		t.Fatalf("expected 5 tasks from repository, got %v", rollup)
	}
	if rollup.PendingTasks != 5 {
		t.Errorf("expected 5 pending tasks, got %d", rollup.PendingTasks)
	}

	// Get cached rollup
	cached := aggregator.GetRollup(campaignID)
	if cached == nil || cached.TotalTasks != 5 {
		t.Fatalf("expected cached rollup with 5 tasks, got %v", cached)
	}
}

func TestRollupAggregator_RunnerLifecycleListeners(t *testing.T) {
	aggregator := NewRollupAggregator(WithAggregatorHeartbeat(50 * time.Millisecond))
	campaignID := db.GenerateUUID()

	// Subscribe to campaign SSE events
	clientChan := aggregator.Subscribe(campaignID)
	defer aggregator.Unsubscribe(campaignID, clientChan)

	if count := aggregator.SubscriberCount(campaignID); count != 1 {
		t.Fatalf("expected 1 subscriber, got %d", count)
	}

	task1 := &db.ProjectTask{
		ID:        "sub-task-1",
		ProjectID: "proj-alpha",
		BatchID:   &campaignID,
		Title:     "[proj-alpha] Rollout",
		Status:    "backlog",
	}
	task2 := &db.ProjectTask{
		ID:        "sub-task-2",
		ProjectID: "proj-beta",
		BatchID:   &campaignID,
		Title:     "[proj-beta] Rollout",
		Status:    "backlog",
	}

	// 1. OnCampaignStart
	aggregator.OnCampaignStart(campaignID, []*db.ProjectTask{task1, task2})

	// 2. OnTaskStart on task1
	aggregator.OnTaskStart(task1)

	// Verify task1 is running
	r := aggregator.GetRollup(campaignID)
	if r.RunningTasks != 1 || r.PendingTasks != 1 {
		t.Errorf("expected 1 running & 1 pending, got running=%d, pending=%d", r.RunningTasks, r.PendingTasks)
	}

	// 3. OnTaskComplete on task1
	aggregator.OnTaskComplete(task1, &TaskExecutionResult{
		TaskID:     task1.ID,
		ProjectID:  task1.ProjectID,
		Status:     "completed",
		DurationMs: 15,
	})

	r = aggregator.GetRollup(campaignID)
	if r.CompletedTasks != 1 || r.SuccessPercentage != 50.0 {
		t.Errorf("expected 1 completed & 50%% success, got completed=%d, pct=%f", r.CompletedTasks, r.SuccessPercentage)
	}

	// 4. OnTaskStart & OnTaskFail on task2
	aggregator.OnTaskStart(task2)
	aggregator.OnTaskFail(task2, fmt.Errorf("connection timeout on target"))

	r = aggregator.GetRollup(campaignID)
	if r.FailedTasks != 1 || r.CompletedTasks != 1 {
		t.Errorf("expected 1 failed & 1 completed, got failed=%d, completed=%d", r.FailedTasks, r.CompletedTasks)
	}

	// 5. OnCampaignComplete
	aggregator.OnCampaignComplete(campaignID, &CampaignRunResult{
		CampaignID:     campaignID,
		BatchID:        campaignID,
		TotalTasks:     2,
		CompletedTasks: 1,
		FailedTasks:    1,
		Success:        false,
	})

	r = aggregator.GetRollup(campaignID)
	if r.Status != "failed" {
		t.Errorf("expected campaign final status 'failed', got %q", r.Status)
	}

	// Collect received events from clientChan
	var receivedTypes []string
	timeout := time.After(200 * time.Millisecond)

collectLoop:
	for {
		select {
		case ev, ok := <-clientChan:
			if !ok {
				break collectLoop
			}
			receivedTypes = append(receivedTypes, ev.Type)
		case <-timeout:
			break collectLoop
		}
	}

	// Verify required SSE event types were emitted:
	// 'campaign.progress', 'task.status_changed', 'campaign.completed'
	hasProgress := false
	hasStatusChanged := false
	hasCompleted := false

	for _, tName := range receivedTypes {
		switch tName {
		case EventCampaignProgress:
			hasProgress = true
		case EventTaskStatusChanged:
			hasStatusChanged = true
		case EventCampaignCompleted:
			hasCompleted = true
		}
	}

	if !hasProgress {
		t.Errorf("expected event 'campaign.progress' to be emitted, got types: %v", receivedTypes)
	}
	if !hasStatusChanged {
		t.Errorf("expected event 'task.status_changed' to be emitted, got types: %v", receivedTypes)
	}
	if !hasCompleted {
		t.Errorf("expected event 'campaign.completed' to be emitted, got types: %v", receivedTypes)
	}
}

func TestRollupAggregator_ConcurrentSubscribersAndCleanup(t *testing.T) {
	aggregator := NewRollupAggregator(WithAggregatorHeartbeat(20 * time.Millisecond))
	campaignID := db.GenerateUUID()

	const numSubscribers = 10
	channels := make([]chan CampaignEvent, numSubscribers)

	// Subscribe 10 clients concurrently
	var wg sync.WaitGroup
	for i := 0; i < numSubscribers; i++ {
		idx := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			channels[idx] = aggregator.Subscribe(campaignID)
		}()
	}
	wg.Wait()

	if count := aggregator.SubscriberCount(campaignID); count != numSubscribers {
		t.Fatalf("expected %d subscribers, got %d", numSubscribers, count)
	}

	// Broadcast an event
	aggregator.Broadcast(campaignID, EventCampaignProgress, map[string]string{"msg": "test"})

	// Verify all subscribers received the broadcast
	for i, ch := range channels {
		select {
		case ev := <-ch:
			if ev.Type != EventCampaignProgress {
				t.Errorf("subscriber %d received wrong event: %s", i, ev.Type)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("subscriber %d timed out waiting for event", i)
		}
	}

	// Unsubscribe all clients concurrently
	for i := 0; i < numSubscribers; i++ {
		idx := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			aggregator.Unsubscribe(campaignID, channels[idx])
		}()
	}
	wg.Wait()

	// Verify subscriber map is completely cleared (zero memory leaks)
	if count := aggregator.SubscriberCount(campaignID); count != 0 {
		t.Fatalf("expected 0 subscribers after cleanup, got %d", count)
	}
}

func TestMultiProjectRunner_IntegratedWithAggregator(t *testing.T) {
	ctx := context.Background()
	campaignID := db.GenerateUUID()

	aggregator := NewRollupAggregator(WithAggregatorHeartbeat(50 * time.Millisecond))

	runner := NewMultiProjectRunner(
		WithMaxWorkers(3),
		WithRunnerAggregator(aggregator),
	)

	// Create 5 dummy tasks
	tasks := make([]*db.ProjectTask, 5)
	for i := 0; i < 5; i++ {
		tasks[i] = &db.ProjectTask{
			ID:        fmt.Sprintf("integrated-task-%d", i),
			ProjectID: fmt.Sprintf("proj-%d", i),
			BatchID:   &campaignID,
			Title:     fmt.Sprintf("Subtask %d", i),
			Status:    "backlog",
			DoDChecklist: []db.DoDItem{
				{Text: "Item 1", Completed: false},
				{Text: "Item 2", Completed: false},
			},
		}
	}

	res, err := runner.ExecuteTasks(ctx, tasks, campaignID)
	if err != nil {
		t.Fatalf("ExecuteTasks failed: %v", err)
	}
	if !res.Success || res.CompletedTasks != 5 {
		t.Fatalf("expected 5 completed tasks, got %d", res.CompletedTasks)
	}

	// Verify aggregator has final state 100% completed
	rollup := aggregator.GetRollup(campaignID)
	if rollup == nil {
		t.Fatalf("expected non-nil rollup in aggregator")
	}
	if rollup.TotalTasks != 5 || rollup.CompletedTasks != 5 {
		t.Errorf("expected 5/5 completed in rollup, got %d/%d", rollup.CompletedTasks, rollup.TotalTasks)
	}
	if rollup.SuccessPercentage != 100.0 {
		t.Errorf("expected 100%% success percentage, got %f", rollup.SuccessPercentage)
	}
	if rollup.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", rollup.Status)
	}
}
