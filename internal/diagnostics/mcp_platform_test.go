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
		"get_github_connection",
		"inspect_git_status",
		"inspect_git_log",
		"inspect_git_diff",
		"inspect_git_branches",
		"inspect_system_health",
		"inspect_typecheck",
		"list_projects",
		"get_project",
		"create_project",
		"update_project",
		"get_project_deployment_config",
		"update_project_deployment_config",
		"get_project_environment",
		"update_project_environment",
		"list_tasks",
		"get_task",
		"update_task_status",
		"create_task_draft",
		"list_inbox_conversations",
		"get_inbox_messages",
		"delete_inbox_message",
		"create_inbox_conversation",
		"send_inbox_message",
		"list_contacts",
		"get_contact",
		"create_contact",
		"update_contact",
		"list_scheduled_jobs",
		"get_system_settings",
		"list_team_members",
		"verify_provider_account",
		"update_provider_account",
		"get_brand_memory",
		"update_brand_memory",
		"run_task",
		"inspect_database_query",
		"inspect_database_schema",
		"inspect_app_logs",
		"list_apps",
		"list_skills",
		"toggle_agent_skill",
		"probe_endpoint_health",
		"inspect_deployment_status",
		"web_fetch",
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
		{"create_project", `{"name":"Test Project"}`},
		{"update_project", `{"project_id":"default","name":"Default"}`},
		{"get_project_deployment_config", `{"project_id":"default"}`},
		{"update_project_deployment_config", `{"project_id":"default","auto_deploy":true,"deploy_branch":"main","deploy_script":"./scripts/deploy.sh"}`},
		{"list_tasks", `{"status":"doing","limit":5}`},
		{"get_task", `{"task_id":"test-task-123"}`},
		{"update_task_status", `{"task_id":"test-task-123","status":"in_progress","summary":"Testing","complete_note":"Verifying"}`},
		{"create_task_draft", `{"project_id":"dojo","title":"Test Draft Task","instructions":"Inspect payment flow","priority":"high","dod_checklist":["Verified payment"]}`},
		{"list_inbox_conversations", `{"limit":5}`},
		{"get_inbox_messages", `{"room_id":"test-room-uuid"}`},
		{"delete_inbox_message", `{"message_id":"test-message-uuid"}`},
		{"create_inbox_conversation", `{"agent_id":"@sm","project_id":"default","title":"Test Conversation"}`},
		{"send_inbox_message", `{"room_id":"test-room-uuid","message":"Hello agent","wait_for_reply":false}`},
		{"list_contacts", `{"query":"test"}`},
		{"get_contact", `{"contact_id":"test-contact-id"}`},
		{"create_contact", `{"name":"John Doe","email":"john@example.com"}`},
		{"update_contact", `{"contact_id":"test-contact-id","notes":"VIP client"}`},
		{"list_scheduled_jobs", `{}`},
		{"get_system_settings", `{}`},
		{"list_team_members", `{}`},
		{"verify_provider_account", `{"account_id":"acc_123"}`},
		{"update_provider_account", `{"account_id":"acc_123","label":"Custom Brain"}`},
		{"get_brand_memory", `{}`},
		{"update_brand_memory", `{"guidelines":"Clean minimal tone"}`},
		{"run_task", `{"task_id":"test-task-123"}`},
		{"inspect_database_query", `{"query":"SELECT 1 AS num"}`},
		{"inspect_database_schema", `{}`},
		{"inspect_app_logs", `{"lines":10}`},
		{"list_apps", `{}`},
		{"list_skills", `{}`},
		{"toggle_agent_skill", `{"agent_id":"agent_sm","skill_id":"vision_receipt_extractor","enabled":true}`},
		{"probe_endpoint_health", `{"url":"https://example.com"}`},
		{"inspect_deployment_status", `{"project":"cuan"}`},
		{"web_fetch", `{"url":"https://example.com"}`},
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

func TestInspectDatabaseQuerySafety(t *testing.T) {
	ctx := t.Context()

	// Should reject mutations
	mutations := []string{
		"INSERT INTO users VALUES (1)",
		"UPDATE users SET name = 'evil'",
		"DELETE FROM users",
		"DROP TABLE users",
		"ALTER TABLE users ADD COLUMN foo int",
		"TRUNCATE users",
		"GRANT ALL ON users TO evil",
		"SELECT 1; DROP TABLE users",
	}

	for _, m := range mutations {
		_, err := InspectDatabaseQuery(ctx, m, 10)
		if err == nil {
			t.Errorf("expected error for mutation query %q, got nil", m)
		}
	}
}
