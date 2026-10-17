package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
)

func TestProjectTasks_CRUD(t *testing.T) {
	s := newTestServerWithMemoryDB(t)
	projectID := "proj-" + db.GenerateUUID()[:8]

	// 1. Create Task (Success)
	checklist := []db.DoDItem{
		{ID: "1", Text: "Setup unit tests", Completed: false},
		{ID: "2", Text: "Implement SSE engine", Completed: false},
	}
	createPayload := map[string]interface{}{
		"title":            "Implement S07.2 REST Handlers",
		"description":      "HTTP endpoints for rooms, messages, tasks, and real-time SSE stream",
		"status":           "backlog",
		"priority":         "high",
		"dod_checklist":    checklist,
		"tags":             []string{"backend", "api", "sse"},
		"agent_output":     "Initial outline",
		"execution_status": "idle",
	}
	bodyBytes, _ := json.Marshal(createPayload)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/projects/%s/tasks", projectID), bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 created, got %d: %s", rec.Code, rec.Body.String())
	}

	var createResp struct {
		Task db.ProjectTaskDTO `json:"task"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("unmarshal create task response: %v", err)
	}
	if createResp.Task.ID == "" || createResp.Task.Title != "Implement S07.2 REST Handlers" || createResp.Task.AgentOutput != "Initial outline" || createResp.Task.ExecutionStatus != "idle" {
		t.Fatalf("unexpected task created: %+v", createResp.Task)
	}
	taskID := createResp.Task.ID

	// 2. Validation Errors
	// Empty title
	emptyTitleBody, _ := json.Marshal(map[string]interface{}{"title": "   "})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/projects/%s/tasks", projectID), bytes.NewReader(emptyTitleBody))
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty title, got %d", rec.Code)
	}

	// Invalid status
	invalidStatusBody, _ := json.Marshal(map[string]interface{}{"title": "Valid Title", "status": "unknown"})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/projects/%s/tasks", projectID), bytes.NewReader(invalidStatusBody))
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid status, got %d", rec.Code)
	}

	// Invalid priority
	invalidPriorityBody, _ := json.Marshal(map[string]interface{}{"title": "Valid Title", "priority": "critical"})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/projects/%s/tasks", projectID), bytes.NewReader(invalidPriorityBody))
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid priority, got %d", rec.Code)
	}

	// 3. Get Task by ID (/api/tasks/{id} and /api/projects/tasks/{id})
	for _, path := range []string{
		fmt.Sprintf("/api/tasks/%s", taskID),
		fmt.Sprintf("/api/projects/tasks/%s", taskID),
	} {
		req = httptest.NewRequest(http.MethodGet, path, nil)
		rec = httptest.NewRecorder()
		s.Router().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", path, rec.Code)
		}
		var getResp struct {
			Task db.ProjectTaskDTO `json:"task"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &getResp)
		if getResp.Task.ID != taskID {
			t.Fatalf("expected task id %s, got %s", taskID, getResp.Task.ID)
		}
	}

	// 4. Update Task (PUT /api/tasks/{id})
	newTitle := "Updated S07.2 Task Title"
	newPriority := "urgent"
	newOutput := "Generated 5 articles successfully"
	newExec := "completed"
	updateBody, _ := json.Marshal(map[string]interface{}{
		"title":            &newTitle,
		"priority":         &newPriority,
		"agent_output":     &newOutput,
		"execution_status": &newExec,
	})
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/tasks/%s", taskID), bytes.NewReader(updateBody))
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on update, got %d: %s", rec.Code, rec.Body.String())
	}
	var updateResp struct {
		Task db.ProjectTaskDTO `json:"task"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &updateResp)
	if updateResp.Task.Title != newTitle || updateResp.Task.Priority != newPriority || updateResp.Task.AgentOutput != newOutput || updateResp.Task.ExecutionStatus != newExec {
		t.Fatalf("expected updated task values, got %+v", updateResp.Task)
	}

	// 5. Patch Task Status (PATCH /api/tasks/{id}/status and /api/projects/tasks/{id}/status)
	for _, status := range []string{"doing", "review", "done"} {
		patchBody, _ := json.Marshal(map[string]interface{}{"status": status})
		req = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/tasks/%s/status", taskID), bytes.NewReader(patchBody))
		rec = httptest.NewRecorder()
		s.Router().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 on patch status %s, got %d: %s", status, rec.Code, rec.Body.String())
		}
		var patchResp struct {
			Task db.ProjectTaskDTO `json:"task"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &patchResp)
		if patchResp.Task.Status != status {
			t.Fatalf("expected patched status %s, got %s", status, patchResp.Task.Status)
		}
	}

	// 6. Delete Task (DELETE /api/tasks/{id})
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/tasks/%s", taskID), nil)
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete, got %d", rec.Code)
	}

	// Verify task deleted
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/tasks/%s", taskID), nil)
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for deleted task, got %d", rec.Code)
	}
}

func TestProjectTasks_QueryFiltering(t *testing.T) {
	s := newTestServerWithMemoryDB(t)
	projectID := "proj-" + db.GenerateUUID()[:8]

	// Create a parent task
	parentTask := &db.ProjectTask{
		ProjectID: projectID,
		Title:     "Parent Epic: Studio Foundation",
		Status:    "doing",
		Priority:  "urgent",
		Tags:      []string{"epic", "foundation"},
	}
	_ = s.ProjectStudioRepository().CreateTask(context.Background(), parentTask)

	// Create child task 1 (fan-out subtask)
	childTask1 := &db.ProjectTask{
		ProjectID:       projectID,
		Title:           "Subtask 1: Database Schema",
		Status:          "done",
		Priority:        "high",
		ParentTaskID:    &parentTask.ID,
		BatchID:         strPtr("batch-multisite-01"),
		Tags:            []string{"wordpress", "db"},
		ExecutionStatus: "completed",
	}
	_ = s.ProjectStudioRepository().CreateTask(context.Background(), childTask1)

	// Create child task 2 (fan-out subtask)
	childTask2 := &db.ProjectTask{
		ProjectID:       projectID,
		Title:           "Subtask 2: REST Handlers",
		Status:          "doing",
		Priority:        "high",
		ParentTaskID:    &parentTask.ID,
		BatchID:         strPtr("batch-multisite-01"),
		Tags:            []string{"wordpress", "api"},
		ExecutionStatus: "running",
	}
	_ = s.ProjectStudioRepository().CreateTask(context.Background(), childTask2)

	// Create an independent batch task
	batchTask := &db.ProjectTask{
		ProjectID:       projectID,
		Title:           "AdSense Integration",
		Status:          "backlog",
		Priority:        "normal",
		BatchID:         strPtr("batch-monetization-02"),
		Tags:            []string{"adsense", "revenue"},
		ExecutionStatus: "idle",
	}
	_ = s.ProjectStudioRepository().CreateTask(context.Background(), batchTask)

	// Helper to query tasks
	queryTasks := func(queryString string) []db.ProjectTaskDTO {
		url := fmt.Sprintf("/api/projects/%s/tasks%s", projectID, queryString)
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("query %s failed: status %d", queryString, rec.Code)
		}
		var resp struct {
			Tasks []db.ProjectTaskDTO `json:"tasks"`
			Count int                 `json:"count"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		return resp.Tasks
	}

	// 1. Filter by parent_task_id (fan-out hierarchy)
	tasks := queryTasks(fmt.Sprintf("?parent_task_id=%s", parentTask.ID))
	if len(tasks) != 2 {
		t.Fatalf("expected 2 child tasks for parent, got %d", len(tasks))
	}

	// 2. Filter by batch_id
	tasks = queryTasks("?batch_id=batch-multisite-01")
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks for batch-multisite-01, got %d", len(tasks))
	}

	tasks = queryTasks("?batch_id=batch-monetization-02")
	if len(tasks) != 1 || tasks[0].Title != "AdSense Integration" {
		t.Fatalf("expected 1 task for batch-monetization-02, got %d", len(tasks))
	}

	// 3. Filter by tag selector
	tasks = queryTasks("?tag=wordpress")
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks with tag 'wordpress', got %d", len(tasks))
	}

	tasks = queryTasks("?tag=adsense")
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task with tag 'adsense', got %d", len(tasks))
	}

	// 4. Filter by status
	tasks = queryTasks("?status=doing")
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks in doing status, got %d", len(tasks))
	}

	// 5. Combined filters: ?status=doing&tag=wordpress
	tasks = queryTasks("?status=doing&tag=wordpress")
	if len(tasks) != 1 || tasks[0].ID != childTask2.ID {
		t.Fatalf("expected childTask2 with doing & wordpress, got %d", len(tasks))
	}

	// 6. Filter by execution_status
	tasks = queryTasks("?execution_status=running")
	if len(tasks) != 1 || tasks[0].ID != childTask2.ID {
		t.Fatalf("expected childTask2 with execution_status 'running', got %d", len(tasks))
	}
}

func TestProjectTasks_SSEBroadcasting(t *testing.T) {
	s := newTestServerWithMemoryDB(t)
	s.RoomEventBroker().SetHeartbeatInterval(100 * time.Millisecond)

	projectID := "proj-" + db.GenerateUUID()[:8]
	room := &db.ProjectRoom{
		ProjectID: projectID,
		Title:     "Task Events Test Room",
	}
	_ = s.ProjectStudioRepository().CreateRoom(context.Background(), room)

	ts := httptest.NewServer(s.Router())
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/rooms/%s/events", ts.URL, room.ID), nil)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("sse client connect: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	readEvent := func(timeout time.Duration) (string, string, error) {
		type result struct {
			evType string
			data   string
			err    error
		}
		ch := make(chan result, 1)
		go func() {
			var currentEvent, currentData string
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					ch <- result{err: err}
					return
				}
				line = strings.TrimRight(line, "\r\n")
				if strings.HasPrefix(line, "event:") {
					currentEvent = strings.TrimSpace(line[6:])
				} else if strings.HasPrefix(line, "data:") {
					currentData = strings.TrimSpace(line[5:])
				} else if line == "" && (currentEvent != "" || currentData != "") {
					ch <- result{evType: currentEvent, data: currentData}
					return
				}
			}
		}()

		select {
		case res := <-ch:
			return res.evType, res.data, res.err
		case <-time.After(timeout):
			return "", "", fmt.Errorf("readEvent timed out after %v", timeout)
		}
	}

	// 1. Initial event: connected
	evType, _, err := readEvent(2 * time.Second)
	if err != nil || evType != "connected" {
		t.Fatalf("expected initial connected event, got '%s', err: %v", evType, err)
	}

	// 2. Create Task bound to this room
	taskPayload := map[string]interface{}{
		"room_id":  room.ID,
		"title":    "Real-Time Task via SSE",
		"status":   "backlog",
		"priority": "normal",
	}
	bodyBytes, _ := json.Marshal(taskPayload)
	postReq, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/projects/%s/tasks", ts.URL, projectID), bytes.NewReader(bodyBytes))
	postReq.Header.Set("Content-Type", "application/json")
	postResp, _ := ts.Client().Do(postReq)
	if postResp.StatusCode != http.StatusCreated {
		t.Fatalf("create task failed: status %d", postResp.StatusCode)
	}
	var createdTask struct {
		Task db.ProjectTaskDTO `json:"task"`
	}
	_ = json.NewDecoder(postResp.Body).Decode(&createdTask)
	_ = postResp.Body.Close()

	// 3. Receive task.created event over SSE stream
	evType, data, err := readEvent(2 * time.Second)
	if err != nil || evType != "task.created" {
		t.Fatalf("expected task.created event, got '%s', err: %v", evType, err)
	}
	if !strings.Contains(data, createdTask.Task.ID) {
		t.Fatalf("expected data to contain task id, got %s", data)
	}

	// 4. Patch Task Status -> task.status_changed event
	patchBody, _ := json.Marshal(map[string]interface{}{"status": "doing"})
	patchReq, _ := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/api/tasks/%s/status", ts.URL, createdTask.Task.ID), bytes.NewReader(patchBody))
	patchReq.Header.Set("Content-Type", "application/json")
	patchResp, _ := ts.Client().Do(patchReq)
	_ = patchResp.Body.Close()

	evType, data, err = readEvent(2 * time.Second)
	if err != nil || evType != "task.status_changed" {
		t.Fatalf("expected task.status_changed event, got '%s', err: %v", evType, err)
	}
	if !strings.Contains(data, "doing") {
		t.Fatalf("expected data to contain 'doing', got %s", data)
	}

	// 5. Delete Task -> task.deleted event
	delReq, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/tasks/%s", ts.URL, createdTask.Task.ID), nil)
	delResp, _ := ts.Client().Do(delReq)
	_ = delResp.Body.Close()

	evType, data, err = readEvent(2 * time.Second)
	if err != nil || evType != "task.deleted" {
		t.Fatalf("expected task.deleted event, got '%s', err: %v", evType, err)
	}
	if !strings.Contains(data, createdTask.Task.ID) {
		t.Fatalf("expected data to contain deleted task id, got %s", data)
	}
}

func TestProjectTasks_V1AliasEndpoints(t *testing.T) {
	s := newTestServerWithMemoryDB(t)
	projectID := "proj-" + db.GenerateUUID()[:8]

	// 1. Create task via /api/v1/projects/{id}/tasks
	body, _ := json.Marshal(map[string]interface{}{
		"title":    "V1 Task Endpoint Test",
		"status":   "backlog",
		"priority": "normal",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/tasks", projectID), bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 on /api/v1/projects/{id}/tasks, got %d", rec.Code)
	}

	var createResp struct {
		Task db.ProjectTaskDTO `json:"task"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &createResp)
	taskID := createResp.Task.ID

	// 2. Get task via /api/v1/projects/tasks/{id}
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/tasks/%s", taskID), nil)
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/v1/projects/tasks/{id}, got %d", rec.Code)
	}

	// 3. Patch task status via /api/v1/tasks/{id}/status
	patchBody, _ := json.Marshal(map[string]interface{}{"status": "doing"})
	req = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/tasks/%s/status", taskID), bytes.NewReader(patchBody))
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/v1/tasks/{id}/status, got %d", rec.Code)
	}

	// 4. Update task via /api/v1/projects/tasks/{id}
	updateBody, _ := json.Marshal(map[string]interface{}{"description": "Updated via v1"})
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/projects/tasks/%s", taskID), bytes.NewReader(updateBody))
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/v1/projects/tasks/{id}, got %d", rec.Code)
	}

	// 5. Delete task via /api/v1/projects/tasks/{id}
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/projects/tasks/%s", taskID), nil)
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete /api/v1/projects/tasks/{id}, got %d", rec.Code)
	}
}


