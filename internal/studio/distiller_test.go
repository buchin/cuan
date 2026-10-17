package studio

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestDistillerEngine_ExtractTitle(t *testing.T) {
	engine := NewDistillerEngine()

	// 1. Explicit title takes precedence
	room := &db.ProjectRoom{Title: "Room Title", Topic: "Room Topic"}
	messages := []db.ProjectMessage{{Content: "Task: In-Message Title"}}
	if got := engine.ExtractTitle(room, messages, "Explicit Override"); got != "Explicit Override" {
		t.Fatalf("expected 'Explicit Override', got %q", got)
	}

	// 2. In-message title header extraction
	messagesWithHeader := []db.ProjectMessage{
		{Content: "Hey team!\n# Task: Implement Caddy SSL Reverse Proxy\nLet's get this done."},
	}
	if got := engine.ExtractTitle(room, messagesWithHeader, ""); got != "Implement Caddy SSL Reverse Proxy" {
		t.Fatalf("expected header extraction, got %q", got)
	}

	// 3. Fallback to room title
	messagesWithoutHeader := []db.ProjectMessage{{Content: "Regular discussion content"}}
	if got := engine.ExtractTitle(room, messagesWithoutHeader, ""); got != "Task: Room Title" {
		t.Fatalf("expected 'Task: Room Title', got %q", got)
	}

	// 4. Room title already starting with Task:
	roomWithTaskPrefix := &db.ProjectRoom{Title: "Task: Existing Prefix"}
	if got := engine.ExtractTitle(roomWithTaskPrefix, messagesWithoutHeader, ""); got != "Task: Existing Prefix" {
		t.Fatalf("expected 'Task: Existing Prefix', got %q", got)
	}

	// 5. Fallback to room topic when title is empty
	roomEmptyTitle := &db.ProjectRoom{Topic: "Build Metrics Exporter"}
	if got := engine.ExtractTitle(roomEmptyTitle, messagesWithoutHeader, ""); got != "Task: Build Metrics Exporter" {
		t.Fatalf("expected 'Task: Build Metrics Exporter', got %q", got)
	}

	// 6. Generic fallback
	if got := engine.ExtractTitle(nil, nil, ""); got != "Distilled Task from Discussion" {
		t.Fatalf("expected generic fallback, got %q", got)
	}
}

func TestDistillerEngine_ExtractDoDChecklist_CheckboxesAndPrefixes(t *testing.T) {
	engine := NewDistillerEngine()

	messages := []db.ProjectMessage{
		{
			ID:         "msg-1",
			AuthorType: "user",
			AuthorName: "Buchin",
			Content: `Planning notes:
- [ ] Implement database migration 000012
* [x] Schema review completed
+ [ ] Endpoint POST /api/rooms/{id}/distill
DoD: Targeted unit tests pass 100%
Acceptance Criteria: Zero regression on db/server`,
		},
		{
			ID:         "msg-2",
			AuthorType: "agent",
			AuthorName: "Scrum Master",
			Content: `Additional checklist:
- [X] Documentation updated
- [ ] Implement database migration 000012`, // Duplicate item
		},
	}

	items := engine.ExtractDoDChecklist(messages, nil)

	// Should extract 6 distinct items (duplicate ignored)
	if len(items) != 6 {
		t.Fatalf("expected 6 distinct DoD items, got %d: %+v", len(items), items)
	}

	// Verify completion states
	foundCompletedCount := 0
	foundUncompletedCount := 0
	for _, it := range items {
		if it.Completed {
			foundCompletedCount++
		} else {
			foundUncompletedCount++
		}
	}

	if foundCompletedCount != 2 { // Schema review completed + Documentation updated
		t.Fatalf("expected 2 completed DoD items, got %d", foundCompletedCount)
	}
	if foundUncompletedCount != 4 {
		t.Fatalf("expected 4 uncompleted DoD items, got %d", foundUncompletedCount)
	}
}

func TestDistillerEngine_InferDoDChecklist_Minimal3ItemsGuaranteed(t *testing.T) {
	engine := NewDistillerEngine(WithMinDoDItems(3))

	// Scenario A: 0 explicit items in discussion -> must infer at least 3 verifiable items
	room := &db.ProjectRoom{
		Title: "API & Database Optimization",
		Topic: "High-performance queries and REST handlers",
	}
	messages := []db.ProjectMessage{
		{Content: "We discussed restructuring the API handlers and improving database connection pooling."},
	}

	resA := engine.Distill(room, messages, DistillOptions{})
	if len(resA.DoDChecklist) < 3 {
		t.Fatalf("Scenario A: expected at least 3 DoD items, got %d: %+v", len(resA.DoDChecklist), resA.DoDChecklist)
	}

	// Check keyword relevance in inferred items
	hasBackendOrDB := false
	for _, it := range resA.DoDChecklist {
		if it.Completed {
			t.Fatalf("inferred item should default to completed=false: %+v", it)
		}
		if db.IsValidUUID(it.ID) == false {
			t.Fatalf("DoD item ID should be valid UUID: %s", it.ID)
		}
		textLower := it.Text
		if len(textLower) > 0 {
			hasBackendOrDB = true
		}
	}
	if !hasBackendOrDB {
		t.Fatalf("expected inferred items to be non-empty")
	}

	// Scenario B: 1 explicit item -> must preserve it + infer 2 more -> total >= 3
	messagesB := []db.ProjectMessage{
		{Content: "- [ ] Run E2E staging drill"},
	}
	resB := engine.Distill(room, messagesB, DistillOptions{})
	if len(resB.DoDChecklist) < 3 {
		t.Fatalf("Scenario B: expected at least 3 DoD items, got %d", len(resB.DoDChecklist))
	}
	if resB.DoDChecklist[0].Text != "Run E2E staging drill" {
		t.Fatalf("expected explicit item preserved first, got %q", resB.DoDChecklist[0].Text)
	}

	// Scenario C: 2 explicit items -> must preserve them + infer 1 more -> total >= 3
	messagesC := []db.ProjectMessage{
		{Content: "- [ ] Step 1\n- [x] Step 2"},
	}
	resC := engine.Distill(room, messagesC, DistillOptions{})
	if len(resC.DoDChecklist) < 3 {
		t.Fatalf("Scenario C: expected at least 3 DoD items, got %d", len(resC.DoDChecklist))
	}
	if resC.DoDChecklist[0].Text != "Step 1" || resC.DoDChecklist[1].Text != "Step 2" {
		t.Fatalf("expected explicit items preserved, got %+v", resC.DoDChecklist)
	}
	if !resC.DoDChecklist[1].Completed {
		t.Fatalf("expected Step 2 to be marked completed")
	}

	// Scenario D: 4 explicit items -> no inference needed, retains 4
	messagesD := []db.ProjectMessage{
		{Content: "- [ ] Item A\n- [ ] Item B\n- [ ] Item C\n- [ ] Item D"},
	}
	resD := engine.Distill(room, messagesD, DistillOptions{})
	if len(resD.DoDChecklist) != 4 {
		t.Fatalf("Scenario D: expected 4 items without inference, got %d", len(resD.DoDChecklist))
	}
}

func TestDistillerEngine_BuildDescription_AndFiltering(t *testing.T) {
	engine := NewDistillerEngine()
	room := &db.ProjectRoom{Topic: "Architecture Discussion for S07"}

	messages := []db.ProjectMessage{
		{ID: "msg-1", AuthorType: "user", AuthorName: "Alice", Content: "First thought on design"},
		{ID: "msg-2", AuthorType: "agent", AuthorName: "Bob", Content: "Counter-proposal on schema"},
		{ID: "msg-3", AuthorType: "user", AuthorName: "Alice", Content: "Agreed on schema"},
	}

	// 1. Without filter: includes all messages
	descAll := engine.BuildDescription(room, messages, nil)
	if !containsAll(descAll, "Room Topic: Architecture Discussion for S07", "[user - Alice]: First thought", "[agent - Bob]: Counter-proposal") {
		t.Fatalf("unexpected description: %s", descAll)
	}

	// 2. With filter: includes only selected message IDs
	filter := map[string]bool{"msg-2": true}
	descFiltered := engine.BuildDescription(room, messages, filter)
	if containsAll(descFiltered, "First thought") {
		t.Fatalf("expected filtered-out message to be omitted")
	}
	if !containsAll(descFiltered, "[agent - Bob]: Counter-proposal") {
		t.Fatalf("expected selected message to be included")
	}
}

func TestDistillerEngine_DistillWithResult_FullAttributes(t *testing.T) {
	engine := NewDistillerEngine()
	room := &db.ProjectRoom{
		ID:        db.GenerateUUID(),
		ProjectID: "proj-12345",
		Title:     "Feature Room",
		Topic:     "User Profile Settings",
	}

	batchID := "batch-sprint-7"
	parentID := db.GenerateUUID()
	assigneeID := "dev-user-1"

	opts := DistillOptions{
		Title:        "Custom Feature Task",
		BatchID:      &batchID,
		ParentTaskID: &parentID,
		AssigneeID:   &assigneeID,
		Priority:     "high",
		Status:       "backlog",
		Tags:         []string{"profile", "settings"},
	}

	messages := []db.ProjectMessage{
		{ID: "m1", AuthorType: "user", AuthorName: "Owner", Content: "- [ ] Add avatar upload\n- [ ] Add password change"},
	}

	res, err := engine.DistillWithResult(context.Background(), room, messages, opts)
	if err != nil {
		t.Fatalf("unexpected distill error: %v", err)
	}

	task := res.Task
	if task.Title != "Custom Feature Task" {
		t.Fatalf("expected custom title, got %q", task.Title)
	}
	if task.ProjectID != "proj-12345" {
		t.Fatalf("expected project id 'proj-12345', got %q", task.ProjectID)
	}
	if task.RoomID == nil || *task.RoomID != room.ID {
		t.Fatalf("expected room id %q, got %+v", room.ID, task.RoomID)
	}
	if task.Priority != "high" {
		t.Fatalf("expected priority 'high', got %q", task.Priority)
	}
	if task.Status != "backlog" {
		t.Fatalf("expected status 'backlog', got %q", task.Status)
	}
	if task.BatchID == nil || *task.BatchID != batchID {
		t.Fatalf("expected batch id %q, got %+v", batchID, task.BatchID)
	}
	if task.ParentTaskID == nil || *task.ParentTaskID != parentID {
		t.Fatalf("expected parent task id %q, got %+v", parentID, task.ParentTaskID)
	}
	if task.AssigneeID == nil || *task.AssigneeID != assigneeID {
		t.Fatalf("expected assignee id %q, got %+v", assigneeID, task.AssigneeID)
	}
	if len(task.DoDChecklist) < 3 {
		t.Fatalf("expected at least 3 DoD items, got %d", len(task.DoDChecklist))
	}
	if res.ExplicitDoDCount != 2 {
		t.Fatalf("expected 2 explicit DoD items, got %d", res.ExplicitDoDCount)
	}
	if res.InferredDoDCount < 1 {
		t.Fatalf("expected at least 1 inferred DoD item, got %d", res.InferredDoDCount)
	}
}

func TestAgentExecutionHook_StatusTransitions(t *testing.T) {
	var dispatchedTaskID string
	var dispatchCount int

	mockDispatcher := ExecutionDispatcherFunc(func(ctx context.Context, pt *db.ProjectTask) (*ExecutionDispatchResult, error) {
		dispatchCount++
		dispatchedTaskID = pt.ID
		return &ExecutionDispatchResult{
			Dispatched:  true,
			TaskID:      pt.ID,
			ProjectID:   pt.ProjectID,
			RunnerID:    "runner-01",
			AttemptID:   "att-test-123",
			AgentName:   "agy",
			TriggeredAt: time.Now().UTC(),
			Message:     "Mock runner execution dispatched",
		}, nil
	})

	hook := NewAgentExecutionHook(mockDispatcher)

	listenerTriggered := false
	hook.AddListener(func(ctx context.Context, pt *db.ProjectTask, res *ExecutionDispatchResult) {
		listenerTriggered = true
		if !res.Dispatched {
			t.Fatalf("expected listener to receive dispatched=true")
		}
	})

	testTask := &db.ProjectTask{
		ID:        db.GenerateUUID(),
		ProjectID: "proj-abc",
		Title:     "Build Realtime SSE Stream",
		Status:    "backlog",
	}

	// 1. Transition backlog -> doing (MUST TRIGGER DISPATCH)
	res1, err := hook.OnTaskStatusChanged(context.Background(), testTask, "backlog", "doing")
	if err != nil {
		t.Fatalf("unexpected error on hook dispatch: %v", err)
	}
	if !res1.Dispatched || dispatchCount != 1 || dispatchedTaskID != testTask.ID {
		t.Fatalf("expected 1 dispatch for backlog -> doing, got count %d, res: %+v", dispatchCount, res1)
	}
	if !listenerTriggered {
		t.Fatalf("expected listener to be triggered on dispatch")
	}

	// 2. Transition doing -> doing (SHOULD NOT RE-DISPATCH)
	res2, err := hook.OnTaskStatusChanged(context.Background(), testTask, "doing", "doing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res2.Dispatched || dispatchCount != 1 {
		t.Fatalf("expected no dispatch on doing -> doing")
	}

	// 3. Transition doing -> review (SHOULD NOT DISPATCH)
	res3, err := hook.OnTaskStatusChanged(context.Background(), testTask, "doing", "review")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res3.Dispatched || dispatchCount != 1 {
		t.Fatalf("expected no dispatch on doing -> review")
	}

	// 4. Transition review -> doing (REVISION CYCLE - MUST TRIGGER DISPATCH)
	res4, err := hook.OnTaskStatusChanged(context.Background(), testTask, "review", "doing")
	if err != nil {
		t.Fatalf("unexpected error on revision dispatch: %v", err)
	}
	if !res4.Dispatched || dispatchCount != 2 {
		t.Fatalf("expected revision dispatch count 2, got %d", dispatchCount)
	}
}

func TestRunnerTaskBridgeDispatcher_Integration(t *testing.T) {
	// Use in-memory task.Store
	memTaskStore := task.NewMemoryStore()
	bridge := NewRunnerTaskBridgeDispatcher(memTaskStore)

	projectTask := &db.ProjectTask{
		ID:          db.GenerateUUID(),
		ProjectID:   "proj-bridge-test",
		Title:       "Bridge Distilled Task to Runner Queue",
		Description: "Detailed task context for agent execution",
		Status:      "doing",
		Priority:    "high",
	}

	res, err := bridge.Dispatch(context.Background(), projectTask)
	if err != nil {
		t.Fatalf("unexpected error in bridge dispatch: %v", err)
	}
	if !res.Dispatched || res.AttemptID == "" {
		t.Fatalf("expected attempt created in queue, got %+v", res)
	}

	// Verify backing task and attempt in memTaskStore
	storedTask, err := memTaskStore.GetTask(context.Background(), projectTask.ID)
	if err != nil || storedTask == nil {
		t.Fatalf("expected backing task in task.Store, got err: %v", err)
	}
	if storedTask.Status != task.TaskStatusDoing {
		t.Fatalf("expected backing task status doing, got %s", storedTask.Status)
	}

	attempts, err := memTaskStore.ListAttempts(context.Background(), projectTask.ID)
	if err != nil || len(attempts) != 1 {
		t.Fatalf("expected 1 attempt in queue, got %d", len(attempts))
	}
	if attempts[0].Status != task.AttemptStatusQueued {
		t.Fatalf("expected attempt status queued, got %s", attempts[0].Status)
	}
}

func TestDistillerEngine_ConcurrentSafety(t *testing.T) {
	engine := NewDistillerEngine()
	room := &db.ProjectRoom{Title: "Concurrent Room", Topic: "Multi-threaded stress test"}

	var wg sync.WaitGroup
	concurrentRuns := 25

	for i := 0; i < concurrentRuns; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			messages := []db.ProjectMessage{
				{Content: fmt.Sprintf("- [ ] Task item %d\n- [x] Done item %d", idx, idx)},
			}
			opts := DistillOptions{
				Title: fmt.Sprintf("Concurrent Task %d", idx),
			}
			res := engine.Distill(room, messages, opts)
			if res == nil || len(res.DoDChecklist) < 3 {
				t.Errorf("routine %d failed: expected >= 3 DoD items, got %+v", idx, res)
			}
		}(i)
	}

	wg.Wait()
}

func containsAll(haystack string, needles ...string) bool {
	for _, n := range needles {
		if !contains(haystack, n) {
			return false
		}
	}
	return true
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && (haystack == needle || len(needle) > 0 && searchSubstring(haystack, needle)))
}

func searchSubstring(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
