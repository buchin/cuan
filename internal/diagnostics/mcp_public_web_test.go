package diagnostics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicWebMCPCannotReachInternalService(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		fmt.Fprint(w, "private-service-content")
	}))
	defer server.Close()
	input := strings.NewReader(fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"web_fetch","arguments":{"url":%q}}}`+"\n", server.URL))
	var out, stderr bytes.Buffer
	if RunPublicWebMCP(input, &out, &stderr) != 0 {
		t.Fatal(stderr.String())
	}
	if called || !strings.Contains(out.String(), `"isError":true`) || strings.Contains(out.String(), "private-service-content") {
		t.Fatal("internal destination boundary failed", out.String())
	}
}

func TestPublicWebMCPDiscoveryAndDispatchBoundary(t *testing.T) {
	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"update_task_status","arguments":{"task_id":"must-not-change","status":"done"}}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"inspect_database_query","arguments":{"query":"SELECT 1"}}}
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"web_fetch","arguments":{"url":"file:///etc/passwd"}}}
`)
	var out, stderr bytes.Buffer
	if RunPublicWebMCP(input, &out, &stderr) != 0 {
		t.Fatal(stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 4 {
		t.Fatal(out.String())
	}
	var discovered struct {
		Result struct {
			Tools []mcpToolDefinition `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &discovered); err != nil {
		t.Fatal(err)
	}
	if len(discovered.Result.Tools) != 1 || discovered.Result.Tools[0].Name != "web_fetch" {
		t.Fatal("unexpected public tools", lines[0])
	}
	for _, line := range lines[1:3] {
		var response jsonRPCResponse
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			t.Fatal(err)
		}
		if response.Error == nil || response.Error.Code != -32601 {
			t.Fatal("hidden tool remained callable", line)
		}
	}
	// The allowed tool reaches the real fetch implementation, including its URL
	// policy. It must reject a local-file URL without exposing file contents.
	if !strings.Contains(lines[3], `"isError":true`) || strings.Contains(lines[3], "root:x:") {
		t.Fatal("public fetch did not retain URL policy", lines[3])
	}
}
