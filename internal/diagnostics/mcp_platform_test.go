package diagnostics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestPlatformMCPToolsList(t *testing.T) {
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	toolsListReq := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n"

	input := bytes.NewBufferString(initReq + toolsListReq)
	var out, errOut bytes.Buffer

	code := RunMCP(input, &out, &errOut)
	if code != 0 {
		t.Fatalf("RunMCP returned code %d", code)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}

	var listResp jsonRPCResponse
	if err := json.Unmarshal([]byte(lines[1]), &listResp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	resMap, ok := listResp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected result map: %+v", listResp.Result)
	}
	toolsSlice, ok := resMap["tools"].([]any)
	if !ok {
		t.Fatalf("expected tools slice: %+v", resMap["tools"])
	}

	expectedTools := []string{
		"inspect_git_status",
		"inspect_git_log",
		"inspect_git_diff",
		"inspect_git_branches",
		"inspect_system_health",
		"inspect_typecheck",
		"list_projects",
		"get_project",
		"list_tasks",
		"get_task",
		"create_task_draft",
		"list_inbox_conversations",
		"get_inbox_messages",
		"list_contacts",
		"get_contact",
		"list_scheduled_jobs",
		"get_system_settings",
	}

	foundMap := make(map[string]bool)
	for _, raw := range toolsSlice {
		if tm, ok := raw.(map[string]any); ok {
			if name, ok := tm["name"].(string); ok {
				foundMap[name] = true
			}
		}
	}

	for _, expected := range expectedTools {
		if !foundMap[expected] {
			t.Errorf("missing expected MCP tool: %s", expected)
		}
	}
}

func TestPlatformMCPToolsExecution(t *testing.T) {
	testCalls := []struct {
		name string
		args string
	}{
		{"list_projects", `{"limit":5}`},
		{"get_project", `{"project_id":"dojo"}`},
		{"list_tasks", `{"status":"doing","limit":5}`},
		{"get_task", `{"task_id":"test-task-123"}`},
		{"create_task_draft", `{"project_id":"dojo","title":"Test Draft Task","instructions":"Inspect payment flow","priority":"high","dod_checklist":["Verified payment"]}`},
		{"list_inbox_conversations", `{"limit":5}`},
		{"get_inbox_messages", `{"room_id":"test-room-uuid"}`},
		{"list_contacts", `{"query":"test"}`},
		{"get_contact", `{"contact_id":"test-contact-id"}`},
		{"list_scheduled_jobs", `{}`},
		{"get_system_settings", `{}`},
	}

	var reqs strings.Builder
	reqs.WriteString(`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{}}` + "\n")
	for i, tc := range testCalls {
		reqs.WriteString(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"%s","arguments":%s}}`+"\n", i+1, tc.name, tc.args))
	}

	input := bytes.NewBufferString(reqs.String())
	var out, errOut bytes.Buffer

	code := RunMCP(input, &out, &errOut)
	if code != 0 {
		t.Fatalf("RunMCP returned non-zero exit code: %d", code)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < len(testCalls)+1 {
		t.Fatalf("expected %d response lines, got %d", len(testCalls)+1, len(lines))
	}

	for i, tc := range testCalls {
		line := lines[i+1]
		var resp jsonRPCResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("[%s] unmarshal response error: %v", tc.name, err)
		}
		if resp.Error != nil {
			t.Errorf("[%s] returned JSON-RPC error: %+v", tc.name, resp.Error)
		}
		resMap, ok := resp.Result.(map[string]any)
		if !ok {
			t.Errorf("[%s] expected result map, got: %+v", tc.name, resp.Result)
			continue
		}
		content, ok := resMap["content"].([]any)
		if !ok || len(content) == 0 {
			t.Errorf("[%s] expected content array in tool result: %+v", tc.name, resMap)
		}
	}
}
