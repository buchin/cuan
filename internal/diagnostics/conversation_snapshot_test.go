package diagnostics

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConversationSnapshotPreservesRecoveryState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Error("missing authorization")
		}
		w.Write([]byte(`{"messages":[],"task_count":0,"requests":[{"request_id":"fixture","state":"queued","safe_error":"Waiting for available AI capacity.","recovery_action":"check_usage"}]}`))
	}))
	defer server.Close()
	c := &CuanClient{BaseURL: server.URL, AuthToken: "fixture-token", HTTPClient: server.Client()}
	previous := globalCuanClient
	globalCuanClient = c
	t.Cleanup(func() { globalCuanClient = previous })
	snapshot, err := c.GetConversationSnapshot(context.Background(), "room", 10)
	if err != nil {
		t.Fatal(err)
	}
	requests := snapshot["requests"].([]map[string]any)
	if len(requests) != 1 || requests[0]["recovery_action"] != "check_usage" {
		t.Fatal("recovery information lost", snapshot)
	}
	var out, errors bytes.Buffer
	if status := RunChatCLI([]string{"messages", "room"}, &out, &errors); status != 0 || !strings.Contains(out.String(), "queued") || !strings.Contains(out.String(), "Waiting for available AI capacity") {
		t.Fatal(status, out.String(), errors.String())
	}
}
