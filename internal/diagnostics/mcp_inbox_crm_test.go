package diagnostics

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInboxCRMMCPUsesOnlyRequestSnapshot(t *testing.T) {
	snapshot := InboxCRMSnapshot{OwnerID: "owner", RoomID: "room", RequestID: "request", CurrentContactID: "contact", ObservedAt: time.Now(), Contacts: []ContactSummary{{ID: "contact", Name: "Customer", Email: "customer@example.invalid", SessionUUID: "room", GuestUUID: "shared"}, {ID: "other", Name: "Other", Email: "other@example.invalid", GuestUUID: "shared"}}, Memories: map[string]json.RawMessage{"contact": json.RawMessage(`[{"body":"Owner recorded fact"}]`)}}
	raw, _ := json.Marshal(snapshot)
	path := filepath.Join(t.TempDir(), "snapshot.json")
	os.WriteFile(path, raw, 0600)
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_contact","arguments":{"contact_id":"current"}}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_contacts","arguments":{"query":"other@example.invalid"}}}
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_contact","arguments":{"contact_id":"other"}}}
{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get_contact","arguments":{"contact_id":"shared"}}}
{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"inspect_database_query","arguments":{"query":"SELECT 1"}}}
{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"update_task_status","arguments":{}}}
`
	var out, stderr bytes.Buffer
	if RunInboxCRMMCP(path, strings.NewReader(input), &out, &stderr) != 0 {
		t.Fatal(stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 7 {
		t.Fatal("lost MCP responses", out.String())
	}
	var tools struct {
		Result struct {
			Tools []mcpToolDefinition `json:"tools"`
		} `json:"result"`
	}
	json.Unmarshal([]byte(lines[0]), &tools)
	if len(tools.Result.Tools) != 3 {
		t.Fatal("unexpected tools", lines[0])
	}
	for _, tool := range tools.Result.Tools {
		if tool.Name != "web_fetch" && tool.Name != "list_contacts" && tool.Name != "get_contact" {
			t.Fatal("ambient tool exposed", tool.Name)
		}
	}
	if !strings.Contains(lines[1], "customer@example.invalid") || !strings.Contains(lines[1], "Owner recorded fact") || strings.Contains(lines[1], "other@example.invalid") {
		t.Fatal("current contact context lost", lines[1])
	}
	if !strings.Contains(lines[2], "other@example.invalid") || strings.Contains(lines[2], "customer@example.invalid") {
		t.Fatal("search failed", lines[2])
	}
	if !strings.Contains(lines[3], `memories_loaded\":false`) {
		t.Fatal("unfetched memory treated as empty", lines[3])
	}
	if !strings.Contains(lines[4], `"isError":true`) {
		t.Fatal("ambiguous guest identity accepted", lines[4])
	}
	for _, line := range lines[5:] {
		if !strings.Contains(line, `"code":-32601`) {
			t.Fatal("hidden tool callable", line)
		}
	}
	// A fresh MCP process cannot load a foreign/stale path or fall back to the
	// host database/Owner API when the per-turn context is absent.
	for _, mode := range []string{"missing", "expired", "public owner"} {
		altered := snapshot
		target := path
		if mode == "missing" {
			target = path + "missing"
		}
		if mode == "expired" {
			altered.ObservedAt = time.Now().Add(-time.Hour)
		}
		if mode == "public owner" {
			altered.OwnerID = ""
		}
		raw, _ := json.Marshal(altered)
		os.WriteFile(path, raw, 0600)
		out.Reset()
		stderr.Reset()
		if RunInboxCRMMCP(target, strings.NewReader(input), &out, &stderr) == 0 || out.Len() != 0 {
			t.Fatal("invalid context exposed native tools", mode)
		}
	}
}
