package diagnostics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConversationIdentityUsesCreatingClient(t *testing.T) {
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")
	foreignReads := 0
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreignReads++
		_ = json.NewEncoder(w).Encode(map[string]any{"agent": map[string]string{"name": "@foreign"}})
	}))
	defer foreign.Close()
	old := globalCuanClient
	globalCuanClient = &CuanClient{BaseURL: foreign.URL, AuthToken: "foreign-owner", HTTPClient: foreign.Client()}
	t.Cleanup(func() { globalCuanClient = old })
	for _, mode := range []string{"metadata available", "metadata unavailable"} {
		t.Run(mode, func(t *testing.T) {
			reads := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer creating-owner" {
					t.Error("request crossed connection credentials")
				}
				if r.Method == http.MethodPost && r.URL.Path == "/api/v1/chat/conversations" {
					_ = json.NewEncoder(w).Encode(map[string]string{"conversation_id": "created-here", "agent_id": "agent_cuan"})
					return
				}
				if r.Method == http.MethodGet && r.URL.Path == "/api/v1/agents/agent_cuan" {
					reads++
					if mode == "metadata unavailable" {
						http.Error(w, "Unavailable", http.StatusServiceUnavailable)
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"agent": map[string]string{"name": "@configured-here"}})
					return
				}
				http.NotFound(w, r)
			}))
			defer server.Close()
			client := &CuanClient{BaseURL: server.URL, AuthToken: "creating-owner", HTTPClient: server.Client()}
			res, err := client.CreateConversation(context.Background(), "", "default", "")
			if err != nil {
				t.Fatal(err)
			}
			want := "@configured-here"
			if mode == "metadata unavailable" {
				want = "Assistant"
			}
			if res["agent_name"] != want || reads != 1 || foreignReads != 0 {
				t.Fatalf("wrong identity source: name=%v, creating reads=%d, foreign reads=%d", res["agent_name"], reads, foreignReads)
			}
		})
	}
}
