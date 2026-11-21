package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
)

func TestInboxRuntimeStreamObservedEventsAndRedaction(t *testing.T) {
	var trace []string
	s := &inboxRuntimeStream{step: func(k, v string) { trace = append(trace, k+":"+v) }}
	lines := []string{
		`{"event":"init","conversation_id":"11111111-2222-3333-4444-555555555555","init":{"cwd":"private-token"}}`,
		`{"event":"step_update","step_update":{"step_index":1,"state":"ACTIVE","step_type":"agent_response","text_delta":"private-reasoning"}}`,
		`{"event":"step_update","step_update":{"step_index":2,"state":"ACTIVE","step_type":"tool","tool_name":"view_file","tool_info":{"parameters":{"AbsolutePath":"private-token"}}}}`,
		`{"event":"step_update","step_update":{"step_index":2,"state":"ERROR","step_type":"tool","tool_name":"view_file","duration_seconds":0.2,"tool_info":{"error":"private-error"}}}`,
		`{"event":"step_update","step_update":{"step_index":2,"state":"ERROR","step_type":"tool","tool_name":"view_file","duration_seconds":0.2}}`,
		`{"event":"step_update","step_update":{"step_index":3,"state":"DONE","step_type":"tool","tool_name":"private-name","tool_info":{"output":"private-output"}}}`,
		`{"event":"step_update","step_update":{"step_index":4,"state":"DONE","step_type":"agent_response","text_delta":"private-reasoning","duration_seconds":1.4}}`,
		`{"event":"result","result":{"status":"SUCCESS","response":"Jawaban ✓"}}`,
	}
	raw := strings.Join(lines, "\r\n")
	for i := 0; i < len(raw); i += 7 {
		end := i + 7
		if end > len(raw) {
			end = len(raw)
		}
		if n, err := s.Write([]byte(raw[i:end])); err != nil || n != end-i {
			t.Fatal(n, err)
		}
	}
	final, failure, _ := s.finish()
	out, decodedFailure, _ := decodeInboxAgyResult(final, "")
	if failure != "" || decodedFailure != "" || out != "Jawaban ✓" || s.toolFailures != 1 {
		t.Fatal("runtime result", failure, decodedFailure, s.toolFailures)
	}
	joined := strings.Join(trace, "\n")
	if strings.Contains(joined, "private") || !strings.Contains(joined, "runtime_tool_failed:Tool #2 · Read file failed (0.2 s).") || !strings.Contains(joined, "runtime_tool_finished:Tool #3 · Tool activity ended.") {
		t.Fatal("unsafe or inaccurate trace", trace)
	}
	if strings.Count(joined, "runtime_tool_failed:") != 1 {
		t.Fatal("duplicate tool event")
	}
	if s.toolDetailEvents != 3 || s.toolInputEvents != 1 || s.toolOutputEvents != 1 || s.toolErrorEvents != 1 {
		t.Fatal("missing or duplicated native detail presence", s.toolDetailEvents, s.toolInputEvents, s.toolOutputEvents, s.toolErrorEvents)
	}
}

func TestInboxRuntimeCountersSurviveTimelineLimit(t *testing.T) {
	s := &inboxRuntimeStream{step: func(string, string) {}}
	for i := 0; i < 80; i++ {
		s.Write([]byte(fmt.Sprintf(`{"event":"step_update","step_update":{"step_index":%d,"state":"ERROR","step_type":"tool","tool_name":"read_file","tool_info":{"error":"PRIVATE_ERROR","parameters":{"path":"PRIVATE_PATH"},"output":"PRIVATE_CONTENT"}}}`+"\n", i)))
	}
	s.finish()
	if !s.truncated || s.toolCalls != 80 || s.toolFailures != 80 || s.toolInputEvents != 80 || s.toolOutputEvents != 80 || s.toolErrorEvents != 80 {
		t.Fatal("timeline limit lost aggregate evidence")
	}
}
func TestInboxRuntimeStreamRejectsIncompleteAndMalformed(t *testing.T) {
	for _, c := range []struct{ raw, want string }{
		{`{"event":"init"}`, "runtime_missing_result"},
		{"private-text\n", "runtime_invalid"},
		{"{\"event\":\"result\",\"result\":{\"status\":\"SUCCESS\",\"response\":\"ok\"}}\n{\"event\":\"init\"}", "runtime_invalid"},
		{`{"event":"result","result":null}`, "runtime_invalid"},
	} {
		s := &inboxRuntimeStream{}
		s.Write([]byte(c.raw))
		out, failure, reason := s.finish()
		if out != "" || failure == "" || reason != c.want || strings.Contains(failure, "private") {
			t.Fatal("bad protocol accepted", reason, failure)
		}
	}
	s := &inboxRuntimeStream{}
	s.Write([]byte(strings.Repeat("x", inboxRuntimeLineLimit+1)))
	s.Write([]byte("more private"))
	if len(s.pending) != 0 {
		t.Fatal("unbounded buffer")
	}
	_, _, reason := s.finish()
	if reason != "runtime_overflow" {
		t.Fatal(reason)
	}
}
func TestInboxRuntimeStreamTraceCapKeepsFinalResult(t *testing.T) {
	count := 0
	s := &inboxRuntimeStream{step: func(k, v string) { count++ }}
	for i := 0; i < 100; i++ {
		s.Write([]byte(fmt.Sprintf("{\"event\":\"step_update\",\"step_update\":{\"step_index\":%d,\"state\":\"DONE\",\"step_type\":\"tool\",\"tool_name\":\"view_file\"}}\n", i)))
	}
	final, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": "ok"}})
	s.Write(final)
	out, failure, _ := s.finish()
	if failure != "" || out == "" || count != 65 || !s.truncated {
		t.Fatal("trace cap discarded result", count, failure)
	}
}

func TestInboxRuntimeDeniedActionsAreSafeAndNotSuccessEvidence(t *testing.T) {
	var trace []string
	s := &inboxRuntimeStream{step: func(k, v string) { trace = append(trace, k+":"+v) }}
	s.Write([]byte(`{"event":"result","result":{"status":"SUCCESS","response":"","denied_actions":[{"action":"private-command","display_name":"private-path"}]}}`))
	raw, failure, _ := s.finish()
	if failure != "" {
		t.Fatal(failure)
	}
	reply, failure, reason := decodeInboxAgyResult(raw, "private-stderr")
	if reply != "" || reason != "tool_permission_denied" || !strings.Contains(failure, "tool permission was denied") {
		t.Fatal(reply, failure, reason)
	}
	joined := strings.Join(trace, "\n")
	if !strings.Contains(joined, "runtime_permission_denied") || strings.Contains(joined+failure, "private") {
		t.Fatal("unsafe or missing denial", joined, failure)
	}
}

func TestNativeQuotaErrorMessageIsNotToolActivity(t *testing.T) {
	var labels []string
	stream := &inboxRuntimeStream{step: func(kind, text string) { labels = append(labels, kind+text) }}
	stream.Write([]byte(`{"event":"init","conversation_id":"3dd79168-31b5-4bf1-bfb0-2cad9751306c"}
{"event":"step_update","step_update":{"step_index":0,"state":"DONE","step_type":"user_input"}}
{"event":"step_update","step_update":{"step_index":1,"state":"DONE","step_type":"error_message","text":"PRIVATE_PROVIDER_ERROR"}}
{"event":"result","result":{"status":"ERROR","message":"RESOURCE_EXHAUSTED"}}
`))
	raw, failure, _ := stream.finish()
	if failure != "" || raw == "" || stream.unsafeTool || stream.invalid || stream.overflow {
		t.Fatal("native error lifecycle blocked safe fallback", failure, stream.unsafeTool)
	}
	if strings.Contains(strings.Join(labels, " "), "PRIVATE_PROVIDER_ERROR") {
		t.Fatal("provider diagnostic leaked")
	}
	stream = &inboxRuntimeStream{}
	stream.Write([]byte(`{"event":"step_update","step_update":{"step_type":"error_message","tool_name":"run_command"}}
`))
	stream.finish()
	if !stream.unsafeTool {
		t.Fatal("tool-bearing error allowed replay")
	}
}

// Native Gemini transcripts use read_file; preserve safe read-only failover
// without broadening replay permission to arbitrary tool aliases.
func TestNativeReadFileQuotaActivity(t *testing.T) {
	for _, name := range []string{"read_file", "view_file", "write_file", "run_command", "unrecognized_tool", ""} {
		t.Run(name, func(t *testing.T) {
			var labels []string
			stream := &inboxRuntimeStream{step: func(k, v string) { labels = append(labels, k+":"+v) }}
			for _, state := range []string{"ACTIVE", "DONE", "ERROR"} {
				row := map[string]any{"event": "step_update", "step_update": map[string]any{"step_index": 2, "state": state, "step_type": "tool", "tool_name": name, "tool_info": map[string]any{"output": "PRIVATE_FILE_CONTENT"}}}
				b, _ := json.Marshal(row)
				stream.Write(append(b, '\n'))
			}
			stream.Write([]byte(`{"event":"result","result":{"status":"ERROR","message":"RESOURCE_EXHAUSTED"}}`))
			_, failure, _ := stream.finish()
			safe := name == "read_file" || name == "view_file"
			if failure != "" || stream.unsafeTool == safe {
				t.Fatal("wrong replay classification", name, stream.unsafeTool, failure)
			}
			trace := strings.Join(labels, "\n")
			if strings.Contains(trace, "PRIVATE") {
				t.Fatal("file output leaked")
			}
			if safe && !strings.Contains(trace, "Read file failed") {
				t.Fatal("read label missing", trace)
			}
		})
	}
}

func TestInboxRuntimeStreamToolBudgetExceeded(t *testing.T) {
	cancelled := false
	var traces []string
	stream := &inboxRuntimeStream{
		maxToolCalls: 3,
		cancel:       func() { cancelled = true },
		step:         func(k, v string) { traces = append(traces, k+":"+v) },
	}
	// Tool 1
	stream.Write([]byte(`{"event":"step_update","step_update":{"step_index":1,"state":"ACTIVE","step_type":"tool","tool_name":"read_file"}}` + "\n"))
	stream.Write([]byte(`{"event":"step_update","step_update":{"step_index":1,"state":"DONE","step_type":"tool","tool_name":"read_file"}}` + "\n"))
	// Tool 2
	stream.Write([]byte(`{"event":"step_update","step_update":{"step_index":2,"state":"ACTIVE","step_type":"tool","tool_name":"view_file"}}` + "\n"))
	stream.Write([]byte(`{"event":"step_update","step_update":{"step_index":2,"state":"DONE","step_type":"tool","tool_name":"view_file"}}` + "\n"))
	// Tool 3
	stream.Write([]byte(`{"event":"step_update","step_update":{"step_index":3,"state":"ACTIVE","step_type":"tool","tool_name":"grep_search"}}` + "\n"))
	stream.Write([]byte(`{"event":"step_update","step_update":{"step_index":3,"state":"DONE","step_type":"tool","tool_name":"grep_search"}}` + "\n"))

	if cancelled || stream.toolBudgetExceeded || stream.toolCalls != 3 {
		t.Fatalf("expected 3 tools within budget, cancelled=%v budgetExceeded=%v count=%d", cancelled, stream.toolBudgetExceeded, stream.toolCalls)
	}

	// Tool 4 exceeds budget
	stream.Write([]byte(`{"event":"step_update","step_update":{"step_index":4,"state":"ACTIVE","step_type":"tool","tool_name":"read_file"}}` + "\n"))

	if !cancelled {
		t.Fatal("expected cancel callback to be invoked on exceeding tool budget")
	}
	if !stream.toolBudgetExceeded {
		t.Fatal("expected toolBudgetExceeded to be true")
	}

	out, failure, reason := stream.finish()
	if out != "" || reason != "tool_budget_exceeded" || !strings.Contains(failure, "tool budget exceeded") {
		t.Fatalf("unexpected finish result: out=%q failure=%q reason=%q", out, failure, reason)
	}
}

func TestInspectionToolsRuntimeStream(t *testing.T) {
	tools := []string{
		"inspect_git_status",
		"inspect_git_log",
		"inspect_git_diff",
		"inspect_git_branches",
		"inspect_system_health",
		"inspect_typecheck",
		"search_web",
		"web_fetch",
		"http_request",
		"browser_fetch",
		"list_projects",
		"get_project",
		"get_github_connection",
		"list_tasks",
		"get_task",
		"create_task_draft",
		"list_inbox_conversations",
		"get_inbox_messages",
		"list_contacts",
		"get_contact",
		"list_scheduled_jobs",
		"get_system_settings",
		"list_apps",
		"list_skills",
		"toggle_agent_skill",
		"inspect_database_query",
		"inspect_database_schema",
		"inspect_app_logs",
		"call_mcp_tool",
	}

	for _, name := range tools {
		t.Run(name, func(t *testing.T) {
			var labels []string
			stream := &inboxRuntimeStream{step: func(k, v string) { labels = append(labels, k+":"+v) }}
			for _, state := range []string{"ACTIVE", "DONE"} {
				row := map[string]any{
					"event": "step_update",
					"step_update": map[string]any{
						"step_index": 2,
						"state":      state,
						"step_type":  "tool",
						"tool_name":  name,
					},
				}
				b, _ := json.Marshal(row)
				stream.Write(append(b, '\n'))
			}
			stream.Write([]byte(`{"event":"result","result":{"status":"SUCCESS","response":"ok"}}` + "\n"))
			_, failure, _ := stream.finish()
			if failure != "" {
				t.Fatalf("unexpected failure for safe tool %s: %s", name, failure)
			}
			if stream.unsafeTool {
				t.Fatalf("tool %s should be allowed and not marked unsafe", name)
			}
			trace := strings.Join(labels, "\n")
			if !strings.Contains(trace, "runtime_tool_started") || !strings.Contains(trace, "runtime_tool_finished") {
				t.Fatalf("missing trace for tool %s: %s", name, trace)
			}
		})
	}
}

func TestInboxRuntimeStreamPrefixedMCPTools(t *testing.T) {
	prefixed := []string{
		"cuan-inspect/list_projects",
		"cuan-inspect:get_inbox_messages",
		"cuan-platform-mcp/list_tasks",
		"cuan-platform-mcp:inspect_database_query",
		"cuan-inspect/list_inbox_conversations",
		"cuan-inspect:get_system_settings",
	}
	for _, tool := range prefixed {
		t.Run(tool, func(t *testing.T) {
			var labels []string
			stream := &inboxRuntimeStream{step: func(k, v string) { labels = append(labels, k+":"+v) }}
			for _, state := range []string{"ACTIVE", "DONE"} {
				row := map[string]any{
					"event": "step_update",
					"step_update": map[string]any{
						"step_index": 2,
						"state":      state,
						"step_type":  "tool",
						"tool_name":  tool,
					},
				}
				b, _ := json.Marshal(row)
				stream.Write(append(b, '\n'))
			}
			stream.Write([]byte(`{"event":"result","result":{"status":"SUCCESS","response":"ok"}}` + "\n"))
			_, failure, _ := stream.finish()
			if failure != "" {
				t.Fatalf("unexpected failure for prefixed MCP tool %s: %s", tool, failure)
			}
			if stream.unsafeTool {
				t.Fatalf("prefixed tool %s should be allowed and safe", tool)
			}
			trace := strings.Join(labels, "\n")
			if !strings.Contains(trace, "runtime_tool_started") {
				t.Fatalf("missing trace for tool %s: %s", tool, trace)
			}
		})
	}
}

func TestInboxRuntimeStreamAdaptiveBatching(t *testing.T) {
	cancelled := false
	var traces []string
	stream := &inboxRuntimeStream{
		maxToolCalls:          3,
		adaptiveBatching:      true,
		inspectionBatchFactor: 3,
		cancel:                func() { cancelled = true },
		step:                  func(k, v string) { traces = append(traces, k+":"+v) },
	}

	inspectionTools := []string{
		"read_file", "view_file", "grep_search",
		"list_dir", "find_by_name", "inspect_git_status",
		"inspect_git_log", "inspect_database_query", "inspect_system_health",
	}

	// 9 inspection calls should fit within 3 turn units (9 / 3 = 3)
	for i, tool := range inspectionTools {
		stepIdx := i + 1
		stream.Write([]byte(fmt.Sprintf(`{"event":"step_update","step_update":{"step_index":%d,"state":"ACTIVE","step_type":"tool","tool_name":"%s"}}`+"\n", stepIdx, tool)))
		stream.Write([]byte(fmt.Sprintf(`{"event":"step_update","step_update":{"step_index":%d,"state":"DONE","step_type":"tool","tool_name":"%s"}}`+"\n", stepIdx, tool)))
	}

	if cancelled || stream.toolBudgetExceeded || stream.toolCalls != 9 || stream.inspectionCalls != 9 {
		t.Fatalf("expected 9 inspection tools within adaptive budget, cancelled=%v budgetExceeded=%v count=%d", cancelled, stream.toolBudgetExceeded, stream.toolCalls)
	}

	// 10th tool call exceeds effective turn budget ((10+2)/3 = 4 > 3)
	stream.Write([]byte(`{"event":"step_update","step_update":{"step_index":10,"state":"ACTIVE","step_type":"tool","tool_name":"read_file"}}` + "\n"))

	if !cancelled {
		t.Fatal("expected cancel callback to be invoked on exceeding adaptive budget")
	}
	if !stream.toolBudgetExceeded {
		t.Fatal("expected toolBudgetExceeded to be true")
	}
}

func TestInboxRuntimeStreamConsecutiveFailureLoopProtection(t *testing.T) {
	cancelled := false
	var traces []string
	stream := &inboxRuntimeStream{
		maxToolCalls:          10,
		adaptiveBatching:      true,
		inspectionBatchFactor: 3,
		cancel:                func() { cancelled = true },
		step:                  func(k, v string) { traces = append(traces, k+":"+v) },
	}

	// Emit 5 consecutive errors on tool calls
	for i := 1; i <= 5; i++ {
		stream.Write([]byte(fmt.Sprintf(`{"event":"step_update","step_update":{"step_index":%d,"state":"ACTIVE","step_type":"tool","tool_name":"read_file"}}`+"\n", i)))
		stream.Write([]byte(fmt.Sprintf(`{"event":"step_update","step_update":{"step_index":%d,"state":"ERROR","step_type":"tool","tool_name":"read_file"}}`+"\n", i)))
	}

	if !cancelled || !stream.toolBudgetExceeded {
		t.Fatalf("expected consecutive failure loop protection to trigger, cancelled=%v budgetExceeded=%v", cancelled, stream.toolBudgetExceeded)
	}
	joined := strings.Join(traces, "\n")
	if !strings.Contains(joined, "Consecutive tool error loop detected") {
		t.Fatalf("expected consecutive error loop trace, got: %s", joined)
	}
}

func TestPublicWebRecoveryRetainsObservedFactsWithoutPrivateReads(t *testing.T) {
	t.Setenv("CUAN_TEST_SECRET", "private-sentinel-credential")
	s := &inboxRuntimeStream{details: newInboxToolDetailRecorder("")}
	events := []string{
		`{"event":"step_update","step_update":{"step_index":2,"state":"ACTIVE","step_type":"tool","tool_name":"call_mcp_tool","tool_info":{"parameters":{"ServerName":"cuan-inspect","ToolName":"web_fetch","Arguments":{"url":"https://example.com"}}}}}`,
		`{"event":"step_update","step_update":{"step_index":2,"state":"DONE","step_type":"tool","tool_name":"call_mcp_tool","tool_info":{"output":{"content":"Provider offers public IPv4 and IPv6","secret":"private-sentinel-credential"}}}}`,
		`{"event":"step_update","step_update":{"step_index":3,"state":"DONE","step_type":"tool","tool_name":"read_file","tool_info":{"parameters":{"Path":"private-notes.txt"},"output":"private-file-sentinel"}}}`,
	}
	for _, event := range events {
		s.Write([]byte(event + "\n"))
	}
	notes := s.publicWebRecoveryContext()
	if !strings.Contains(notes, "Provider offers public IPv4 and IPv6") || !strings.Contains(notes, "https://example.com") {
		t.Fatal("lost completed web evidence", notes)
	}
	if strings.Contains(notes, "private-file-sentinel") || strings.Contains(notes, "private-sentinel-credential") {
		t.Fatal("private output entered research recovery")
	}
}

func TestInboxRuntimeStreamCapturesFinishPayload(t *testing.T) {
	s := &inboxRuntimeStream{}
	event := `{"event":"step_update","step_update":{"step_index":1,"state":"DONE","step_type":"tool","tool_name":"finish","tool_info":{"parameters":{"decision":"pass","explanation":"All good"}}}}`
	s.Write([]byte(event + "\n"))

	raw, failure, reason := s.finish()
	if failure != "" || reason != "" {
		t.Fatalf("expected clean finish from captured finish payload, got failure=%q reason=%q", failure, reason)
	}
	var res inboxAgyResult
	if err := json.Unmarshal([]byte(raw), &res); err != nil || res.Status != "SUCCESS" {
		t.Fatalf("expected successful inboxAgyResult, got raw=%s, err=%v", raw, err)
	}
	if !strings.Contains(res.Response, "All good") {
		t.Fatalf("expected response to contain All good, got %s", res.Response)
	}
}

func TestDecodeInboxAgyResultUsesFinishPayloadOnTimeout(t *testing.T) {
	raw := `{"status":"ERROR","reason":"print_timeout","response":""}`
	finishPayload := `{"decision":"pass","explanation":"Finished successfully"}`

	out, failure, reason := decodeInboxAgyResult(raw, "", finishPayload)
	if failure != "" || reason != "success" {
		t.Fatalf("expected success with finishPayload, got failure=%q reason=%q", failure, reason)
	}
	if out != finishPayload {
		t.Fatalf("expected %s, got %s", finishPayload, out)
	}
}

func TestParseInboxReviewEvaluationUnquotesValues(t *testing.T) {
	raw := `{"decision":"\"pass\"","explanation":"\"Semua kriteria terpenuhi.\"","finding":"\"Lulus\"","remaining_work":"\"none\"","requested_outcome":"\"completed\"","revision":"\"\"","source_review":"\"pass\"","visual_inspection":"\"Tampilan normal\""}`
	eval, err := parseInboxReviewEvaluation(raw)
	if err != nil {
		t.Fatalf("parseInboxReviewEvaluation failed: %v", err)
	}
	if eval.Decision != "pass" {
		t.Fatalf("expected decision pass, got %q", eval.Decision)
	}
	if eval.Explanation != "Semua kriteria terpenuhi." {
		t.Fatalf("expected unquoted explanation, got %q", eval.Explanation)
	}
}

func TestMatchInboxRequestIDOrRetry(t *testing.T) {
	original := "req-12345"
	retry1 := db.InboxRetryID(original)
	retry2 := db.InboxRetryID(retry1)
	retry3 := db.InboxRetryID(retry2)

	if !matchInboxRequestIDOrRetry(original, original) {
		t.Fatal("failed to match original")
	}
	if !matchInboxRequestIDOrRetry(retry1, original) {
		t.Fatal("failed to match retry level 1")
	}
	if !matchInboxRequestIDOrRetry(retry2, original) {
		t.Fatal("failed to match retry level 2")
	}
	if !matchInboxRequestIDOrRetry(retry3, original) {
		t.Fatal("failed to match retry level 3")
	}
	if matchInboxRequestIDOrRetry("random-id", original) {
		t.Fatal("matched random id")
	}
}

func TestInboxRuntimeStreamAllowsNativeSystemMessagesAndThoughts(t *testing.T) {
	stream := &inboxRuntimeStream{
		step: func(string, string) {},
	}
	events := []string{
		`{"event":"init","conversation_id":"11111111-2222-3333-4444-555555555555"}`,
		`{"event":"step_update","step_update":{"step_index":0,"state":"DONE","step_type":"user_input"}}`,
		`{"event":"step_update","step_update":{"step_index":1,"state":"DONE","step_type":"system_message","text":"You have read 7 files in this turn."}}`,
		`{"event":"step_update","step_update":{"step_index":2,"state":"DONE","step_type":"thought","text":"Thinking about user request..."}}`,
		`{"event":"step_update","step_update":{"step_index":3,"state":"DONE","step_type":"system","text":"System event notification."}}`,
		`{"event":"step_update","step_update":{"step_index":4,"state":"DONE","step_type":"tool","tool_name":"read_file","tool_info":{"parameters":{"Path":"web/src/features/inbox/InboxView.tsx"}}}}`,
		`{"event":"step_update","step_update":{"step_index":5,"state":"DONE","step_type":"tool","tool_name":"finish","tool_info":{"parameters":{"reply":"Jawaban lengkap","topic_title":"","action":null}}}}`,
		`{"event":"result","result":{"status":"SUCCESS","response":"{\"reply\":\"Jawaban lengkap\",\"topic_title\":\"\",\"action\":null}"}}`,
	}
	for _, ev := range events {
		stream.Write([]byte(ev + "\n"))
	}
	raw, failure, reason := stream.finish()
	if stream.unsafeTool {
		t.Fatalf("expected unsafeTool=false for benign system/thought events, got unsafeTool=true")
	}
	if failure != "" {
		t.Fatalf("expected clean finish, got failure=%s, reason=%s", failure, reason)
	}
	reply, decodeFail, _ := decodeInboxAgyResult(raw, "")
	if decodeFail != "" || reply == "" {
		t.Fatalf("failed to decode valid reply: %s", decodeFail)
	}
}

func TestInboxRuntimeStreamDirectCoordinatorAllowsAttachmentsAndFinish(t *testing.T) {
	// Scenario: directCoordinator with 1 attachment => maxToolCalls = 2, disallowTools = false
	stream := &inboxRuntimeStream{
		step:         func(string, string) {},
		maxToolCalls: 2,
	}
	events := []string{
		`{"event":"init","conversation_id":"11111111-2222-3333-4444-555555555555"}`,
		`{"event":"step_update","step_update":{"step_index":0,"state":"DONE","step_type":"user_input"}}`,
		`{"event":"step_update","step_update":{"step_index":1,"state":"DONE","step_type":"tool","tool_name":"read_file","tool_info":{"parameters":{"Path":"attachment.jpg"}}}}`,
		`{"event":"step_update","step_update":{"step_index":2,"state":"DONE","step_type":"tool","tool_name":"finish","tool_info":{"parameters":{"reply":"Jawaban visual","topic_title":"","action":null}}}}`,
		`{"event":"result","result":{"status":"SUCCESS","response":"{\"reply\":\"Jawaban visual\",\"topic_title\":\"\",\"action\":null}"}}`,
	}
	for _, ev := range events {
		stream.Write([]byte(ev + "\n"))
	}
	raw, failure, reason := stream.finish()
	if stream.unsafeTool || stream.toolBudgetExceeded {
		t.Fatalf("expected valid execution, got unsafeTool=%v, toolBudgetExceeded=%v", stream.unsafeTool, stream.toolBudgetExceeded)
	}
	if failure != "" {
		t.Fatalf("expected clean finish, got failure=%s, reason=%s", failure, reason)
	}
	reply, decodeFail, _ := decodeInboxAgyResult(raw, "")
	if decodeFail != "" || reply == "" {
		t.Fatalf("failed to decode valid reply: %s", decodeFail)
	}
}

func TestInboxRuntimeStreamProgressiveDeltasNoFinishDuplicate(t *testing.T) {
	var deltas []string
	stream := &inboxRuntimeStream{
		step: func(string, string) {},
		onDelta: func(d string) {
			deltas = append(deltas, d)
		},
	}
	events := []string{
		`{"event":"init","conversation_id":"11111111-2222-3333-4444-555555555555"}`,
		`{"event":"step_update","step_update":{"step_index":0,"state":"DONE","step_type":"user_input"}}`,
		`{"event":"step_update","step_update":{"step_index":1,"state":"ACTIVE","step_type":"agent_response","text_delta":"{\"reply\":\"Halo "}}`,
		`{"event":"step_update","step_update":{"step_index":1,"state":"ACTIVE","step_type":"agent_response","text_delta":"dunia!\",\"topic_title\":\"\",\"action\":null}"}}`,
		`{"event":"step_update","step_update":{"step_index":2,"state":"DONE","step_type":"tool","tool_name":"finish","tool_info":{"parameters":{"reply":"Halo dunia!","topic_title":"","action":null}}}}`,
		`{"event":"result","result":{"status":"SUCCESS","response":"{\"reply\":\"Halo dunia!\",\"topic_title\":\"\",\"action\":null}"}}`,
	}
	for _, ev := range events {
		stream.Write([]byte(ev + "\n"))
	}
	raw, failure, reason := stream.finish()
	if failure != "" {
		t.Fatalf("expected clean finish, got failure=%s, reason=%s", failure, reason)
	}
	reply, decodeFail, _ := decodeInboxAgyResult(raw, "")
	if decodeFail != "" || !strings.Contains(reply, "Halo dunia!") {
		t.Fatalf("expected reply to contain 'Halo dunia!', got %q (decodeFail: %s)", reply, decodeFail)
	}

	combinedDelta := strings.Join(deltas, "")
	if combinedDelta != "Halo dunia!" {
		t.Fatalf("expected combined deltas to equal 'Halo dunia!', got %q (deltas: %#v)", combinedDelta, deltas)
	}
	if len(deltas) != 2 {
		t.Fatalf("expected exactly 2 deltas during progressive streaming without finish duplication, got %d: %#v", len(deltas), deltas)
	}
}
