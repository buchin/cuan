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
	if strings.Contains(joined, "private") || !strings.Contains(joined, "runtime_tool_failed:Tool #2 · Baca file gagal (0.2 dtk).") || !strings.Contains(joined, "runtime_tool_finished:Tool #3 · Tool selesai.") {
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
