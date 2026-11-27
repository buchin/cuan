package diagnostics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func createTestTasksFixture() []map[string]any {
	return []map[string]any{
		{
			"id":             "task-fixture-1",
			"project_id":     "proj-alpha",
			"title":          "Task One (Doing)",
			"instructions":   "Do work 1",
			"status":         "doing",
			"priority":       "high",
			"assigned_agent": "agent_cuan",
			"updated_at":     "2026-11-09T08:00:00Z",
		},
		{
			"id":             "task-fixture-2",
			"project_id":     "proj-alpha",
			"title":          "Task Two (Doing)",
			"instructions":   "Do work 2",
			"status":         "doing",
			"priority":       "normal",
			"assigned_agent": "agent_cuan",
			"updated_at":     "2026-11-09T07:00:00Z",
		},
		{
			"id":             "task-fixture-3",
			"project_id":     "proj-beta",
			"title":          "Task Three (Done)",
			"instructions":   "Do work 3",
			"status":         "done",
			"priority":       "normal",
			"assigned_agent": "agent_cuan",
			"updated_at":     "2026-11-09T06:00:00Z",
		},
		{
			"id":             "task-fixture-4",
			"project_id":     "proj-beta",
			"title":          "Task Four (Doing)",
			"instructions":   "Do work 4",
			"status":         "doing",
			"priority":       "low",
			"assigned_agent": "agent_cuan",
			"updated_at":     "2026-11-09T05:00:00Z",
		},
	}
}

func setupMockTaskServer(t *testing.T, fixture []map[string]any) (*httptest.Server, func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/tasks") {
			http.NotFound(w, r)
			return
		}

		statusFilter := r.URL.Query().Get("status")
		projectFilter := r.URL.Query().Get("project_id")

		var filtered []map[string]any
		for _, task := range fixture {
			if statusFilter != "" && task["status"] != statusFilter {
				continue
			}
			if projectFilter != "" && task["project_id"] != projectFilter {
				continue
			}
			filtered = append(filtered, task)
		}

		// Even if a legacy server returned all matching tasks without slicing by limit,
		// our client and CLI layers must strictly enforce the positive limit.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tasks": filtered,
			"total": len(filtered),
		})
	}))

	prev := globalCuanClient
	globalCuanClient = &CuanClient{
		BaseURL:    server.URL,
		AuthToken:  "test-token",
		HTTPClient: server.Client(),
	}

	cleanup := func() {
		globalCuanClient = prev
		server.Close()
	}

	return server, cleanup
}

func TestTaskListLimitsJSON(t *testing.T) {
	fixture := createTestTasksFixture()
	_, cleanup := setupMockTaskServer(t, fixture)
	defer cleanup()

	// Test limit 1 with --json
	t.Run("limit 1 returns exactly 1 task in JSON", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := RunTaskCLI([]string{"list", "--limit", "1", "--json"}, &out, &errOut)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", code, errOut.String())
		}

		var res struct {
			Tasks []map[string]any `json:"tasks"`
			Total int              `json:"total"`
		}
		if err := json.Unmarshal(out.Bytes(), &res); err != nil {
			t.Fatalf("failed to unmarshal JSON output: %v\nOutput: %s", err, out.String())
		}

		if len(res.Tasks) != 1 {
			t.Fatalf("expected exactly 1 task, got %d", len(res.Tasks))
		}
		if res.Total != 1 {
			t.Fatalf("expected total 1, got %d", res.Total)
		}
		if res.Tasks[0]["id"] != "task-fixture-1" {
			t.Fatalf("expected first task to be task-fixture-1, got %v", res.Tasks[0]["id"])
		}
	})

	// Test limit 2 with --json
	t.Run("limit 2 returns exactly 2 tasks in JSON", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := RunTaskCLI([]string{"list", "--limit", "2", "--json"}, &out, &errOut)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", code, errOut.String())
		}

		var res struct {
			Tasks []map[string]any `json:"tasks"`
			Total int              `json:"total"`
		}
		if err := json.Unmarshal(out.Bytes(), &res); err != nil {
			t.Fatalf("failed to unmarshal JSON output: %v\nOutput: %s", err, out.String())
		}

		if len(res.Tasks) != 2 {
			t.Fatalf("expected exactly 2 tasks, got %d", len(res.Tasks))
		}
		if res.Total != 2 {
			t.Fatalf("expected total 2, got %d", res.Total)
		}
		if res.Tasks[0]["id"] != "task-fixture-1" || res.Tasks[1]["id"] != "task-fixture-2" {
			t.Fatalf("expected task-fixture-1 and task-fixture-2 in order, got %v and %v", res.Tasks[0]["id"], res.Tasks[1]["id"])
		}
	})
}

func TestTaskListLimitsHumanText(t *testing.T) {
	fixture := createTestTasksFixture()
	_, cleanup := setupMockTaskServer(t, fixture)
	defer cleanup()

	t.Run("limit 1 human text", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := RunTaskCLI([]string{"list", "--limit", "1"}, &out, &errOut)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", code, errOut.String())
		}

		outStr := out.String()
		if !strings.Contains(outStr, "Found 1 task(s):") {
			t.Fatalf("expected 'Found 1 task(s):', got: %s", outStr)
		}
		if !strings.Contains(outStr, "task-fixture-1") {
			t.Fatalf("expected task-fixture-1 in output, got: %s", outStr)
		}
		if strings.Contains(outStr, "task-fixture-2") {
			t.Fatalf("task-fixture-2 should not appear when limit is 1: %s", outStr)
		}
	})

	t.Run("limit 2 human text", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := RunTaskCLI([]string{"list", "--limit", "2"}, &out, &errOut)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", code, errOut.String())
		}

		outStr := out.String()
		if !strings.Contains(outStr, "Found 2 task(s):") {
			t.Fatalf("expected 'Found 2 task(s):', got: %s", outStr)
		}
		if !strings.Contains(outStr, "task-fixture-1") || !strings.Contains(outStr, "task-fixture-2") {
			t.Fatalf("expected task-fixture-1 and task-fixture-2, got: %s", outStr)
		}
		if strings.Contains(outStr, "task-fixture-3") {
			t.Fatalf("task-fixture-3 should not appear when limit is 2: %s", outStr)
		}
	})
}

func TestTaskListFilterIntegrity(t *testing.T) {
	fixture := createTestTasksFixture()
	_, cleanup := setupMockTaskServer(t, fixture)
	defer cleanup()

	t.Run("status filter with limit 1", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := RunTaskCLI([]string{"list", "--status", "done", "--limit", "1", "--json"}, &out, &errOut)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", code, errOut.String())
		}

		var res struct {
			Tasks []map[string]any `json:"tasks"`
			Total int              `json:"total"`
		}
		if err := json.Unmarshal(out.Bytes(), &res); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v", err)
		}

		if len(res.Tasks) != 1 || res.Total != 1 {
			t.Fatalf("expected 1 done task, got %d (total %d)", len(res.Tasks), res.Total)
		}
		if res.Tasks[0]["id"] != "task-fixture-3" || res.Tasks[0]["status"] != "done" {
			t.Fatalf("expected task-fixture-3 with status done, got %+v", res.Tasks[0])
		}
	})

	t.Run("project filter with limit 2", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := RunTaskCLI([]string{"list", "--project", "proj-alpha", "--limit", "2", "--json"}, &out, &errOut)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", code, errOut.String())
		}

		var res struct {
			Tasks []map[string]any `json:"tasks"`
			Total int              `json:"total"`
		}
		if err := json.Unmarshal(out.Bytes(), &res); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v", err)
		}

		if len(res.Tasks) != 2 || res.Total != 2 {
			t.Fatalf("expected 2 tasks for proj-alpha, got %d (total %d)", len(res.Tasks), res.Total)
		}
		for _, task := range res.Tasks {
			if task["project_id"] != "proj-alpha" {
				t.Fatalf("expected project_id proj-alpha, got %v", task["project_id"])
			}
		}
	})
}

func TestListTasksMCPHandlerLimits(t *testing.T) {
	fixture := createTestTasksFixture()
	_, cleanup := setupMockTaskServer(t, fixture)
	defer cleanup()

	ctx := context.Background()

	t.Run("ListTasks function with limit 1", func(t *testing.T) {
		res, err := ListTasks(ctx, "", "", 1)
		if err != nil {
			t.Fatalf("ListTasks error: %v", err)
		}

		resMap, ok := res.(map[string]any)
		if !ok {
			t.Fatalf("expected map[string]any, got %T", res)
		}
		tasks, ok := resMap["tasks"].([]map[string]any)
		if !ok {
			t.Fatalf("expected []map[string]any, got %T", resMap["tasks"])
		}
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}
		total, _ := resMap["total"].(int)
		if total != 1 {
			t.Fatalf("expected total 1, got %d", total)
		}
	})

	t.Run("ListTasks function with limit 2", func(t *testing.T) {
		res, err := ListTasks(ctx, "", "", 2)
		if err != nil {
			t.Fatalf("ListTasks error: %v", err)
		}

		resMap, ok := res.(map[string]any)
		if !ok {
			t.Fatalf("expected map[string]any, got %T", res)
		}
		tasks, ok := resMap["tasks"].([]map[string]any)
		if !ok {
			t.Fatalf("expected []map[string]any, got %T", resMap["tasks"])
		}
		if len(tasks) != 2 {
			t.Fatalf("expected 2 tasks, got %d", len(tasks))
		}
		total, _ := resMap["total"].(int)
		if total != 2 {
			t.Fatalf("expected total 2, got %d", total)
		}
	})

	t.Run("RunMCP tool call list_tasks with limit 1 and 2", func(t *testing.T) {
		reqs := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_tasks","arguments":{"limit":1}}}` + "\n" +
			`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_tasks","arguments":{"limit":2}}}` + "\n"

		var out, errOut bytes.Buffer
		code := RunMCP(bytes.NewBufferString(reqs), &out, &errOut)
		if code != 0 {
			t.Fatalf("RunMCP exited with code %d", code)
		}

		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		if len(lines) < 3 {
			t.Fatalf("expected at least 3 response lines, got %d: %s", len(lines), out.String())
		}

		// Check response to id:2 (limit 1)
		var resp1 jsonRPCResponse
		if err := json.Unmarshal([]byte(lines[1]), &resp1); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		resMap1 := resp1.Result.(map[string]any)
		content1 := resMap1["content"].([]any)[0].(map[string]any)
		text1 := content1["text"].(string)

		var toolData1 struct {
			Tasks []any `json:"tasks"`
			Total int   `json:"total"`
		}
		if err := json.Unmarshal([]byte(text1), &toolData1); err != nil {
			t.Fatalf("unmarshal tool text error: %v", err)
		}
		if len(toolData1.Tasks) != 1 || toolData1.Total != 1 {
			t.Fatalf("MCP tool list_tasks limit 1 expected 1 task and total 1, got %d and %d", len(toolData1.Tasks), toolData1.Total)
		}

		// Check response to id:3 (limit 2)
		var resp2 jsonRPCResponse
		if err := json.Unmarshal([]byte(lines[2]), &resp2); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		resMap2 := resp2.Result.(map[string]any)
		content2 := resMap2["content"].([]any)[0].(map[string]any)
		text2 := content2["text"].(string)

		var toolData2 struct {
			Tasks []any `json:"tasks"`
			Total int   `json:"total"`
		}
		if err := json.Unmarshal([]byte(text2), &toolData2); err != nil {
			t.Fatalf("unmarshal tool text error: %v", err)
		}
		if len(toolData2.Tasks) != 2 || toolData2.Total != 2 {
			t.Fatalf("MCP tool list_tasks limit 2 expected 2 tasks and total 2, got %d and %d", len(toolData2.Tasks), toolData2.Total)
		}
	})
}

func TestListTasksCompactAndPagination(t *testing.T) {
	longText := strings.Repeat("A", 3000)
	longErr := strings.Repeat("ERR_LOG_", 300)
	fixture := []map[string]any{
		{
			"id":             "task-big-1",
			"project_id":     "proj-alpha",
			"title":          "Task Big One",
			"instructions":   longText,
			"summary":        longText,
			"complete_note":  longText,
			"diff_summary":   longText,
			"test_summary":   longText,
			"dod_checklist":  []map[string]any{{"text": "Check 1", "completed": true, "evidence": longText}},
			"status":         "doing",
			"priority":       "high",
			"assigned_agent": "agent_cuan",
			"latest_attempt": map[string]any{
				"id":             "att-1",
				"status":         "failed",
				"attempt_number": 2,
				"error_message":  longErr,
			},
			"created_at": "2026-11-09T08:00:00Z",
			"updated_at": "2026-11-09T08:05:00Z",
		},
		{
			"id":         "task-big-2",
			"project_id": "proj-alpha",
			"title":      "Task Big Two",
			"status":     "done",
			"priority":   "normal",
			"updated_at": "2026-11-09T07:00:00Z",
		},
		{
			"id":         "task-big-3",
			"project_id": "proj-alpha",
			"title":      "Task Big Three",
			"status":     "review",
			"priority":   "normal",
			"updated_at": "2026-11-09T06:00:00Z",
		},
	}
	_, cleanup := setupMockTaskServer(t, fixture)
	defer cleanup()

	t.Run("MCP list_tasks default compact omits large fields and supports pagination", func(t *testing.T) {
		reqs := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_tasks","arguments":{"limit":2,"offset":0}}}` + "\n" +
			`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_tasks","arguments":{"limit":2,"offset":2}}}` + "\n" +
			`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_tasks","arguments":{"limit":1,"offset":0,"full":true}}}` + "\n"

		var out, errOut bytes.Buffer
		if code := RunMCP(bytes.NewBufferString(reqs), &out, &errOut); code != 0 {
			t.Fatalf("RunMCP exited with code %d: %s", code, errOut.String())
		}
		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		if len(lines) < 4 {
			t.Fatalf("expected 4 response lines, got %d", len(lines))
		}

		parsePayload := func(line string) map[string]any {
			t.Helper()
			var resp jsonRPCResponse
			if err := json.Unmarshal([]byte(line), &resp); err != nil {
				t.Fatalf("unmarshal rpc: %v", err)
			}
			resMap := resp.Result.(map[string]any)
			content := resMap["content"].([]any)[0].(map[string]any)
			var payload map[string]any
			if err := json.Unmarshal([]byte(content["text"].(string)), &payload); err != nil {
				t.Fatalf("unmarshal payload: %v", err)
			}
			return payload
		}

		page1 := parsePayload(lines[1])
		if page1["compact"] != true || page1["has_more"] != true {
			t.Fatalf("expected compact=true and has_more=true, got %+v", page1)
		}
		if int(page1["total_count"].(float64)) != 3 || int(page1["next_offset"].(float64)) != 2 {
			t.Fatalf("expected total_count=3 and next_offset=2, got total_count=%v next_offset=%v", page1["total_count"], page1["next_offset"])
		}
		tasks1 := page1["tasks"].([]any)
		if len(tasks1) != 2 {
			t.Fatalf("expected 2 tasks on page 1, got %d", len(tasks1))
		}
		first := tasks1[0].(map[string]any)
		if first["status"] != "doing" {
			t.Fatalf("expected status=doing, got %v", first["status"])
		}
		if _, hasInst := first["instructions"]; hasInst {
			t.Fatalf("compact list_tasks must omit instructions")
		}
		if _, hasNote := first["complete_note"]; hasNote {
			t.Fatalf("compact list_tasks must omit complete_note")
		}
		if summary, _ := first["summary"].(string); len(summary) >= len(longText) {
			t.Fatalf("expected truncated summary, got len %d", len(summary))
		}
		la := first["latest_attempt"].(map[string]any)
		if la["status"] != "failed" {
			t.Fatalf("expected latest_attempt.status=failed, got %v", la["status"])
		}
		if errMsg, _ := la["error_message"].(string); len(errMsg) >= len(longErr) {
			t.Fatalf("expected truncated error_message, got len %d", len(errMsg))
		}

		page2 := parsePayload(lines[2])
		if page2["has_more"] != false || int(page2["offset"].(float64)) != 2 {
			t.Fatalf("expected has_more=false and offset=2 on page 2, got %+v", page2)
		}
		tasks2 := page2["tasks"].([]any)
		if len(tasks2) != 1 || tasks2[0].(map[string]any)["id"] != "task-big-3" {
			t.Fatalf("expected task-big-3 on page 2, got %+v", tasks2)
		}

		fullPage := parsePayload(lines[3])
		if fullPage["compact"] != false {
			t.Fatalf("expected compact=false when full=true, got %v", fullPage["compact"])
		}
		fullFirst := fullPage["tasks"].([]any)[0].(map[string]any)
		if fullFirst["instructions"] != longText || fullFirst["complete_note"] != longText {
			t.Fatalf("expected full instructions and complete_note when full=true")
		}
	})

	t.Run("CLI task list --offset and --full", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := RunTaskCLI([]string{"list", "--limit", "1", "--offset", "1", "--json"}, &out, &errOut)
		if code != 0 {
			t.Fatalf("RunTaskCLI failed: %d %s", code, errOut.String())
		}
		var res map[string]any
		if err := json.Unmarshal(out.Bytes(), &res); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		tasks := res["tasks"].([]any)
		if len(tasks) != 1 || tasks[0].(map[string]any)["id"] != "task-big-2" {
			t.Fatalf("expected task-big-2 at offset 1, got %+v", tasks)
		}
		if int(res["total_count"].(float64)) != 3 || res["has_more"] != true {
			t.Fatalf("expected total_count=3 and has_more=true, got %+v", res)
		}
	})
}

func TestGetTaskCompactAndFull(t *testing.T) {
	longText := strings.Repeat("Z", 4000)
	attempts := make([]map[string]any, 0, 6)
	for i := 1; i <= 6; i++ {
		attempts = append(attempts, map[string]any{
			"id":             fmt.Sprintf("att-%d", i),
			"status":         "failed",
			"attempt_number": i,
			"error_message":  longText,
		})
	}
	activities := make([]map[string]any, 0, 12)
	for i := 1; i <= 12; i++ {
		activities = append(activities, map[string]any{
			"id":            fmt.Sprintf("act-%d", i),
			"activity_type": "comment",
			"author":        "agent_cuan",
			"message":       longText,
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tasks/task-huge-1" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":              "task-huge-1",
			"project_id":      "proj-alpha",
			"title":           "Huge Task",
			"status":          "blocked",
			"priority":        "high",
			"instructions":    longText,
			"summary":         longText,
			"complete_note":   longText,
			"diff_summary":    longText,
			"test_summary":    longText,
			"latest_progress": longText,
			"dod_checklist":   []map[string]any{{"id": "d1", "text": "DoD 1", "completed": false, "evidence": longText}},
			"latest_attempt":  attempts[0],
			"attempts":        attempts,
			"activities":      activities,
		})
	}))
	defer server.Close()

	prev := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "test-token", HTTPClient: server.Client()}
	defer func() { globalCuanClient = prev }()

	reqs := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_task","arguments":{"task_id":"task-huge-1"}}}` + "\n" +
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_task","arguments":{"task_id":"task-huge-1","full":true}}}` + "\n"

	var out, errOut bytes.Buffer
	if code := RunMCP(bytes.NewBufferString(reqs), &out, &errOut); code != 0 {
		t.Fatalf("RunMCP failed: %d %s", code, errOut.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	parseText := func(line string) (string, map[string]any) {
		t.Helper()
		var resp jsonRPCResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("unmarshal rpc: %v", err)
		}
		rawText := resp.Result.(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
		var payload map[string]any
		if err := json.Unmarshal([]byte(rawText), &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		return rawText, payload
	}

	compactRaw, compactRes := parseText(lines[1])
	fullRaw, fullRes := parseText(lines[2])

	if compactRes["compact"] != true || compactRes["truncated"] != true {
		t.Fatalf("expected compact=true and truncated=true, got compact=%v truncated=%v", compactRes["compact"], compactRes["truncated"])
	}
	if compactRes["status"] != "blocked" {
		t.Fatalf("expected status=blocked, got %v", compactRes["status"])
	}
	if la := compactRes["latest_attempt"].(map[string]any); la["status"] != "failed" {
		t.Fatalf("expected latest_attempt.status=failed, got %v", la["status"])
	}
	if len(compactRes["attempts"].([]any)) != 3 || int(compactRes["attempts_total"].(float64)) != 6 {
		t.Fatalf("expected 3 attempts with attempts_total=6, got %d (%v)", len(compactRes["attempts"].([]any)), compactRes["attempts_total"])
	}
	if len(compactRes["activities"].([]any)) != 5 || int(compactRes["activities_total"].(float64)) != 12 {
		t.Fatalf("expected 5 activities with activities_total=12, got %d (%v)", len(compactRes["activities"].([]any)), compactRes["activities_total"])
	}
	if len(compactRaw) >= len(fullRaw)/4 {
		t.Fatalf("expected compact payload (%d bytes) to be much smaller than full payload (%d bytes)", len(compactRaw), len(fullRaw))
	}

	if _, hasCompact := fullRes["compact"]; hasCompact {
		t.Fatalf("full get_task should not set compact=true")
	}
	if len(fullRes["attempts"].([]any)) != 6 || len(fullRes["activities"].([]any)) != 12 {
		t.Fatalf("expected all 6 attempts and 12 activities in full mode")
	}
	if fullRes["instructions"] != longText {
		t.Fatalf("expected full instructions in full mode")
	}
}
