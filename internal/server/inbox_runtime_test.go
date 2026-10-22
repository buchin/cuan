package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
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
