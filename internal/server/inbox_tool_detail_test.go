package server

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInboxToolDetailRetainsUsefulEvidenceAndRedactsBeforeBounding(t *testing.T) {
	t.Setenv("COMMAND_CODE_API_KEY", "fixture-provider-credential")
	r := newInboxToolDetailRecorder("/workspace/project")
	r.observe(2, "view_file", map[string]json.RawMessage{"parameters": json.RawMessage(`{"AbsolutePath":"/workspace/project/README.md","token":"short"}`)})
	r.observe(2, "view_file", map[string]json.RawMessage{"output": json.RawMessage(`"# Project guide\nfixture-provider-credential\npassword=private-value\nUseful verification result"`), "error": json.RawMessage(`{"message":"File not found","authorization":"private-header"}`)})
	body := r.body(2)
	for _, want := range []string{"README.md", "Project guide", "Useful verification result", "File not found", "[REDACTED]"} {
		if !strings.Contains(body, want) {
			t.Fatal("lost diagnostic", want, body)
		}
	}
	for _, forbidden := range []string{"/workspace/project", "fixture-provider-credential", "private-value", "private-header", `"short"`} {
		if strings.Contains(body, forbidden) {
			t.Fatal("private diagnostic", forbidden)
		}
	}
	r.observe(3, "read_file", map[string]json.RawMessage{"output": json.RawMessage(`"{\"api_key\":\"nested-credential\",\"message\":\"Verification failed\"}"`)})
	if strings.Contains(r.body(3), "nested-credential") || !strings.Contains(r.body(3), "Verification failed") {
		t.Fatal("embedded JSON redaction failed")
	}
	r.observe(5, "get_project", map[string]json.RawMessage{"output": json.RawMessage(`{"variables":[{"key":"PROVIDER_SECRET","value":"row-credential"}]}`)})
	if strings.Contains(r.body(5), "row-credential") {
		t.Fatal("labelled environment value leaked")
	}
	r.observe(4, "read_file", map[string]json.RawMessage{"output": json.RawMessage(`"` + strings.Repeat("界", 2100) + `"`)})
	if !strings.Contains(r.body(4), "[Excerpt truncated]") || len([]rune(r.body(4))) > 2050 {
		t.Fatal("excerpt not bounded")
	}
}

func TestInboxToolDetailWithholdsSensitiveFilesAndNonToolContent(t *testing.T) {
	r := newInboxToolDetailRecorder("/workspace")
	r.observe(1, "read_file", map[string]json.RawMessage{"parameters": json.RawMessage(`{"path":"/workspace/.env.production"}`), "output": json.RawMessage(`"arbitrary unlabelled credential"`)})
	if strings.Contains(r.body(1), "arbitrary") || !strings.Contains(r.body(1), "Sensitive file contents not retained") {
		t.Fatal("sensitive file archived")
	}
	for _, tool := range []string{"finish", "cuan/finish", "cuan:finish", "unknown-tool"} {
		r.observe(2, tool, map[string]json.RawMessage{"output": json.RawMessage(`"PRIVATE_REPLY"`)})
		if r.body(2) != "" {
			t.Fatal("non-inspection detail archived", tool)
		}
	}
	for i := 3; i < 100; i++ {
		r.observe(i, "read_file", map[string]json.RawMessage{"output": json.RawMessage(`"bounded"`)})
	}
	if len(r.steps) != 24 {
		t.Fatal("unbounded diagnostic steps", len(r.steps))
	}
}

func TestInboxToolDetailUsesExistingOwnerTimelineWithoutReasoning(t *testing.T) {
	var trace []string
	s := &inboxRuntimeStream{details: newInboxToolDetailRecorder("/workspace"), step: func(k, v string) { trace = append(trace, k+":"+v) }}
	s.Write([]byte(`{"event":"step_update","step_update":{"step_index":1,"state":"DONE","step_type":"agent_response","text_delta":"PRIVATE_REASONING"}}` + "\n"))
	s.Write([]byte(`{"event":"step_update","step_update":{"step_index":2,"state":"ACTIVE","step_type":"tool","tool_name":"read_file","tool_info":{"parameters":{"path":"/workspace/README.md"}}}}` + "\n"))
	s.Write([]byte(`{"event":"step_update","step_update":{"step_index":2,"state":"ERROR","step_type":"tool","tool_name":"read_file","tool_info":{"error":"File not found"}}}` + "\n"))
	joined := strings.Join(trace, "\n")
	if strings.Contains(joined, "PRIVATE_REASONING") || !strings.Contains(joined, "\n\nInput\n") || !strings.Contains(joined, "Error\nFile not found") {
		t.Fatal("missing safe owner detail", joined)
	}
}
