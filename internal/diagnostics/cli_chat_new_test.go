package diagnostics

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatNewDefaultPublicAssistant(t *testing.T) {
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")

	var receivedPayload map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture" {
			t.Errorf("expected Bearer fixture, got %s", r.Header.Get("Authorization"))
		}
		if r.Method == "POST" && r.URL.Path == "/api/v1/chat/conversations" {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &receivedPayload)
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":              "topic_12345",
				"conversation_id": "12345",
				"room_id":         "12345",
				"agent_id":        "agent_cuan",
				"agent_name":      "@cuan",
				"title":           "New conversation",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	old := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "fixture", HTTPClient: server.Client()}
	t.Cleanup(func() { globalCuanClient = old })

	var out, errs bytes.Buffer
	code := RunChatCLI([]string{"new"}, &out, &errs)
	if code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, errs.String())
	}

	// Verify that the agent ID was defaulted to agent_cuan without forcing persona selection
	if receivedPayload["agent_id"] != "agent_cuan" {
		t.Fatalf("expected agent_id agent_cuan, got %q", receivedPayload["agent_id"])
	}

	output := out.String()
	if !strings.Contains(output, "Conversation created successfully:") {
		t.Fatalf("expected success message, got: %s", output)
	}
	if !strings.Contains(output, "Agent:        @cuan") {
		t.Fatalf("expected agent display name @cuan, got: %s", output)
	}
	if !strings.Contains(output, "Room ID:      12345") {
		t.Fatalf("expected room ID 12345, got: %s", output)
	}
}

func TestChatNewDynamicAssistantIdentity(t *testing.T) {
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")

	// Simulate user having edited the assistant's name to @teman in settings
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/v1/chat/conversations" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":              "topic_dyn123",
				"conversation_id": "dyn123",
				"room_id":         "dyn123",
				"agent_id":        "agent_cuan",
				"agent_name":      "@teman",
				"title":           "Work planning",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	old := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "fixture", HTTPClient: server.Client()}
	t.Cleanup(func() { globalCuanClient = old })

	var out, errs bytes.Buffer
	code := RunChatCLI([]string{"new", "--title", "Work planning"}, &out, &errs)
	if code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, errs.String())
	}

	output := out.String()
	// Must display dynamic name from active settings, not hardcoded @cuan or @sm
	if !strings.Contains(output, "Agent:        @teman") {
		t.Fatalf("expected dynamic agent name @teman, got: %s", output)
	}
	if strings.Contains(output, "@sm") {
		t.Fatalf("output should not contain @sm, got: %s", output)
	}
}

func TestChatNewDynamicAssistantIdentityResolvedFromAPI(t *testing.T) {
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")

	// Simulate conversation endpoint not returning agent_name, so CLI resolves it via GET /api/v1/agents/agent_cuan
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/v1/chat/conversations" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"conversation_id": "conv_fallback_1",
				"room_id":         "conv_fallback_1",
				"agent_id":        "agent_cuan",
			})
			return
		}
		if r.Method == "GET" && r.URL.Path == "/api/v1/agents/agent_cuan" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"agent": map[string]any{
					"id":   "agent_cuan",
					"name": "@asisten_kustom",
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	old := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "fixture", HTTPClient: server.Client()}
	t.Cleanup(func() { globalCuanClient = old })

	var out, errs bytes.Buffer
	code := RunChatCLI([]string{"new"}, &out, &errs)
	if code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, errs.String())
	}

	output := out.String()
	if !strings.Contains(output, "Agent:        @asisten_kustom") {
		t.Fatalf("expected dynamically resolved name @asisten_kustom, got: %s", output)
	}
}

func TestChatNewLegacyAliasesCompatibility(t *testing.T) {
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")

	testCases := []struct {
		flagValue string
		wantAgent string
	}{
		{"@sm", "agent_cuan"},
		{"@dev", "agent_cuan"},
		{"@support", "agent_cuan"},
		{"@cuan", "agent_cuan"},
		{"agent_sm", "agent_cuan"},
	}

	for _, tc := range testCases {
		t.Run("alias_"+tc.flagValue, func(t *testing.T) {
			var capturedAgentID string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" && r.URL.Path == "/api/v1/chat/conversations" {
					var payload map[string]string
					body, _ := io.ReadAll(r.Body)
					_ = json.Unmarshal(body, &payload)
					capturedAgentID = payload["agent_id"]
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(map[string]any{
						"conversation_id": "legacy_conv",
						"room_id":         "legacy_conv",
						"agent_id":        capturedAgentID,
						"agent_name":      "@cuan",
					})
					return
				}
				http.NotFound(w, r)
			}))
			defer server.Close()

			old := globalCuanClient
			globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "fixture", HTTPClient: server.Client()}
			t.Cleanup(func() { globalCuanClient = old })

			var out, errs bytes.Buffer
			code := RunChatCLI([]string{"new", "--agent", tc.flagValue}, &out, &errs)
			if code != 0 {
				t.Fatalf("alias %s failed with code %d: %s", tc.flagValue, code, errs.String())
			}
			if capturedAgentID != tc.wantAgent {
				t.Fatalf("alias %s: expected agent_id %s, got %s", tc.flagValue, tc.wantAgent, capturedAgentID)
			}
		})
	}
}

func TestChatNewCustomAgent(t *testing.T) {
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")

	var capturedAgentID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/v1/chat/conversations" {
			var payload map[string]string
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &payload)
			capturedAgentID = payload["agent_id"]
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"conversation_id": "custom_conv",
				"room_id":         "custom_conv",
				"agent_id":        capturedAgentID,
				"agent_name":      "custom_specialist",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	old := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "fixture", HTTPClient: server.Client()}
	t.Cleanup(func() { globalCuanClient = old })

	var out, errs bytes.Buffer
	code := RunChatCLI([]string{"new", "--agent", "custom_specialist"}, &out, &errs)
	if code != 0 {
		t.Fatalf("custom agent failed with code %d: %s", code, errs.String())
	}
	if capturedAgentID != "custom_specialist" {
		t.Fatalf("expected agent_id custom_specialist, got %s", capturedAgentID)
	}
}

func TestChatNewHelpOutputClean(t *testing.T) {
	// 1. Check `cuan chat new --help`
	var outNew, errsNew bytes.Buffer
	codeNew := RunChatCLI([]string{"new", "--help"}, &outNew, &errsNew)
	if codeNew != 0 {
		t.Fatalf("expected exit code 0 for --help, got %d", codeNew)
	}
	combinedNew := outNew.String() + errsNew.String()
	if strings.Contains(combinedNew, "@sm") {
		t.Errorf("cuan chat new --help must not offer @sm, got: %s", combinedNew)
	}
	if strings.Contains(combinedNew, "@dev") {
		t.Errorf("cuan chat new --help must not offer @dev, got: %s", combinedNew)
	}
	if strings.Contains(combinedNew, "@support") {
		t.Errorf("cuan chat new --help must not offer @support, got: %s", combinedNew)
	}
	if strings.Contains(combinedNew, `default "@sm"`) {
		t.Errorf("cuan chat new --help must not default to @sm, got: %s", combinedNew)
	}

	// 2. Check `cuan chat help`
	var outChat, errsChat bytes.Buffer
	codeChat := RunChatCLI([]string{"help"}, &outChat, &errsChat)
	if codeChat != 0 {
		t.Fatalf("expected exit code 0 for help, got %d", codeChat)
	}
	combinedChat := outChat.String() + errsChat.String()
	if strings.Contains(combinedChat, "--agent @sm") {
		t.Errorf("cuan chat help must not recommend --agent @sm, got: %s", combinedChat)
	}
}

func TestChatNewJSONOutput(t *testing.T) {
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/v1/chat/conversations" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"conversation_id": "json_conv_123",
				"room_id":         "json_conv_123",
				"agent_id":        "agent_cuan",
				"agent_name":      "@cuan",
				"title":           "JSON Test",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	old := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: server.URL, AuthToken: "fixture", HTTPClient: server.Client()}
	t.Cleanup(func() { globalCuanClient = old })

	var out, errs bytes.Buffer
	code := RunChatCLI([]string{"new", "--json", "--title", "JSON Test"}, &out, &errs)
	if code != 0 {
		t.Fatalf("expected code 0, got %d: %s", code, errs.String())
	}

	var parsed map[string]any
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v, raw output: %s", err, out.String())
	}
	if parsed["agent_id"] != "agent_cuan" {
		t.Fatalf("expected agent_id agent_cuan, got %v", parsed["agent_id"])
	}
	if parsed["room_id"] != "json_conv_123" {
		t.Fatalf("expected room_id json_conv_123, got %v", parsed["room_id"])
	}
}
