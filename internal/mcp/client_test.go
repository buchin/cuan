package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMCPClient_DiscoverTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp/tools" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-secret-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"tools": []ToolDefinition{
				{
					Name:        "tebar_broadcast_message",
					Description: "Send automated WhatsApp notification through Tebar.app",
					InputSchema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"phone":   map[string]interface{}{"type": "string"},
							"message": map[string]interface{}{"type": "string"},
						},
						"required": []string{"phone", "message"},
					},
				},
				{
					Name:        "tebar_check_order",
					Description: "Inspect buyer order status on Tebar.app",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(ServerConfig{
		ID:        "tebar_prod",
		Name:      "Tebar.app Gateway",
		BaseURL:   server.URL,
		APIKey:    "test-secret-key",
		Transport: TransportHTTP,
	}, server.Client())

	ctx := context.Background()
	tools, err := client.DiscoverTools(ctx)
	if err != nil {
		t.Fatalf("unexpected DiscoverTools error: %v", err)
	}

	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0].Name != "tebar_broadcast_message" {
		t.Errorf("expected first tool tebar_broadcast_message, got %s", tools[0].Name)
	}

	// Verify caching
	cached := client.GetCachedTools()
	if len(cached) != 2 {
		t.Errorf("expected 2 cached tools, got %d", len(cached))
	}
}

func TestMCPClient_ExecuteTool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp/tools/call" {
			http.NotFound(w, r)
			return
		}

		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)

		toolName, _ := req["name"].(string)
		if toolName != "tebar_broadcast_message" {
			http.Error(w, "unknown tool", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ToolCallResponse{
			Content: []ToolContent{
				{
					Type: "text",
					Text: "Message successfully queued for delivery to +628123456789 (MessageID: msg_9941)",
				},
			},
			IsError: false,
		})
	}))
	defer server.Close()

	client := NewClient(ServerConfig{
		ID:      "tebar_prod",
		BaseURL: server.URL,
	}, server.Client())

	resp, err := client.ExecuteTool(context.Background(), "tebar_broadcast_message", map[string]interface{}{
		"phone":   "+628123456789",
		"message": "Halo, pesanan Anda #1029 sedang diproses!",
	})
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	if resp.IsError {
		t.Errorf("expected IsError = false, got true")
	}
	if len(resp.Content) == 0 || resp.Content[0].Text == "" {
		t.Fatalf("expected non-empty response content: %+v", resp)
	}
}
