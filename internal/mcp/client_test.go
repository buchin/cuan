package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

func TestMCPClient_StreamableHTTP_DiscoverAndExecuteJSON(t *testing.T) {
	var mu sync.Mutex
	var methods []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" {
			t.Errorf("unexpected path %s; want /mcp", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %s; want POST", r.Method)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Authorization") != "Bearer tb_live_test" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		accept := r.Header.Get("Accept")
		if !strings.Contains(accept, "application/json") || !strings.Contains(accept, "text/event-stream") {
			t.Errorf("expected Accept header with json and event-stream, got %q", accept)
		}

		var req struct {
			JSONRPC string                 `json:"jsonrpc"`
			ID      json.RawMessage        `json:"id"`
			Method  string                 `json:"method"`
			Params  map[string]interface{} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		mu.Lock()
		methods = append(methods, req.Method)
		mu.Unlock()

		switch req.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "sess-tebar-001")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]interface{}{
					"protocolVersion": "2025-03-26",
					"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
					"serverInfo":      map[string]string{"name": "tebar-mcp", "version": "1.0.0"},
				},
			})
		case "notifications/initialized":
			if got := r.Header.Get("Mcp-Session-Id"); got != "sess-tebar-001" {
				t.Errorf("expected Mcp-Session-Id sess-tebar-001 on notification, got %q", got)
			}
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			if got := r.Header.Get("Mcp-Session-Id"); got != "sess-tebar-001" {
				t.Errorf("expected Mcp-Session-Id sess-tebar-001 on tools/list, got %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "tebar_schedule_post",
							"description": "Schedule a post across social platforms",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"caption": map[string]interface{}{"type": "string"},
								},
							},
						},
					},
				},
			})
		case "tools/call":
			if got := r.Header.Get("Mcp-Session-Id"); got != "sess-tebar-001" {
				t.Errorf("expected Mcp-Session-Id sess-tebar-001 on tools/call, got %q", got)
			}
			name, _ := req.Params["name"].(string)
			args, _ := req.Params["arguments"].(map[string]interface{})
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]interface{}{
					"content": []map[string]interface{}{
						{"type": "text", "text": fmt.Sprintf("scheduled %s: %v", name, args["caption"])},
					},
					"isError": false,
				},
			})
		default:
			http.Error(w, "unknown method", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	client := NewClient(ServerConfig{
		ID:      "tebar",
		Name:    "Tebar",
		BaseURL: server.URL + "/mcp",
		APIKey:  "tb_live_test",
	}, server.Client())

	tools, err := client.DiscoverTools(context.Background())
	if err != nil {
		t.Fatalf("DiscoverTools failed: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "tebar_schedule_post" {
		t.Fatalf("unexpected tools: %+v", tools)
	}
	if tools[0].InputSchema == nil || tools[0].InputSchema["type"] != "object" {
		t.Fatalf("expected inputSchema to be parsed into InputSchema, got %+v", tools[0].InputSchema)
	}

	resp, err := client.ExecuteTool(context.Background(), "tebar_schedule_post", map[string]interface{}{
		"caption": "Promo Jumat Berkah",
	})
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}
	if resp.IsError || len(resp.Content) != 1 || !strings.Contains(resp.Content[0].Text, "Promo Jumat Berkah") {
		t.Fatalf("unexpected ExecuteTool response: %+v", resp)
	}

	expectedOrder := []string{"initialize", "notifications/initialized", "tools/list", "tools/call"}
	if strings.Join(methods, ",") != strings.Join(expectedOrder, ",") {
		t.Fatalf("unexpected RPC call sequence: got %v, want %v", methods, expectedOrder)
	}
}

func TestMCPClient_StreamableHTTP_SSEResponseAndPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}

		var req struct {
			ID     json.RawMessage        `json:"id"`
			Method string                 `json:"method"`
			Params map[string]interface{} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		writeSSE := func(payload interface{}) {
			b, _ := json.Marshal(payload)
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprintf(w, ": keepalive\n\nevent: message\ndata: %s\n\n", string(b))
		}

		switch req.Method {
		case "initialize":
			writeSSE(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  map[string]interface{}{"protocolVersion": "2025-03-26"},
			})
		case "notifications/initialized":
			w.WriteHeader(http.StatusNoContent)
		case "tools/list":
			cursor, _ := req.Params["cursor"].(string)
			if cursor == "" {
				writeSSE(map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      req.ID,
					"result": map[string]interface{}{
						"tools":      []map[string]interface{}{{"name": "tool_one", "description": "First"}},
						"nextCursor": "page_2",
					},
				})
				return
			}
			writeSSE(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]interface{}{
					"tools": []map[string]interface{}{{"name": "tool_two", "description": "Second"}},
				},
			})
		case "tools/call":
			writeSSE(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]interface{}{
					"content": []map[string]interface{}{{"type": "text", "text": "rate limit reached"}},
					"isError": true,
				},
			})
		}
	}))
	defer server.Close()

	client := NewClient(ServerConfig{
		ID:      "sse_mcp",
		BaseURL: server.URL + "/mcp/",
	}, server.Client())

	tools, err := client.DiscoverTools(context.Background())
	if err != nil {
		t.Fatalf("DiscoverTools SSE failed: %v", err)
	}
	if len(tools) != 2 || tools[0].Name != "tool_one" || tools[1].Name != "tool_two" {
		t.Fatalf("expected 2 paginated tools, got %+v", tools)
	}

	resp, err := client.ExecuteTool(context.Background(), "tool_one", nil)
	if err != nil {
		t.Fatalf("ExecuteTool SSE failed: %v", err)
	}
	if !resp.IsError || len(resp.Content) == 0 || resp.Content[0].Text != "rate limit reached" {
		t.Fatalf("unexpected SSE tool response: %+v", resp)
	}
}

func TestMCPClient_StreamableHTTP_NoDuplicateMCPPathOn404(t *testing.T) {
	var requestedPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.Method+" "+r.URL.Path)
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(ServerConfig{
		ID:      "tebar",
		BaseURL: server.URL + "/mcp",
	}, server.Client())

	_, err := client.DiscoverTools(context.Background())
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	for _, p := range requestedPaths {
		if strings.Contains(p, "/mcp/mcp/tools") {
			t.Fatalf("must not request duplicated /mcp/mcp/tools path, got %v", requestedPaths)
		}
	}
	if len(requestedPaths) != 1 || requestedPaths[0] != "POST /mcp" {
		t.Fatalf("expected single POST /mcp attempt, got %v", requestedPaths)
	}
}

func TestMCPClient_StreamableHTTP_ErrorsAndSessionRecovery(t *testing.T) {
	t.Run("jsonrpc error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				ID     json.RawMessage `json:"id"`
				Method string          `json:"method"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			w.Header().Set("Content-Type", "application/json")
			if req.Method == "initialize" {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      req.ID,
					"result":  map[string]interface{}{},
				})
				return
			}
			if req.Method == "notifications/initialized" {
				w.WriteHeader(http.StatusAccepted)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"error": map[string]interface{}{
					"code":    -32601,
					"message": "Method not found",
				},
			})
		}))
		defer server.Close()

		client := NewClient(ServerConfig{BaseURL: server.URL + "/mcp"}, server.Client())
		_, err := client.ExecuteTool(context.Background(), "missing", nil)
		if err == nil || !strings.Contains(err.Error(), "Method not found") {
			t.Fatalf("expected json-rpc error message, got %v", err)
		}
	})

	t.Run("session expired 404 recovery", func(t *testing.T) {
		sessionCounter := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				ID     json.RawMessage `json:"id"`
				Method string          `json:"method"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			switch req.Method {
			case "initialize":
				sessionCounter++
				w.Header().Set("Mcp-Session-Id", fmt.Sprintf("sess-%d", sessionCounter))
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      req.ID,
					"result":  map[string]interface{}{},
				})
			case "notifications/initialized":
				w.WriteHeader(http.StatusAccepted)
			case "tools/call":
				if r.Header.Get("Mcp-Session-Id") == "sess-1" {
					http.Error(w, "session expired", http.StatusNotFound)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      req.ID,
					"result": map[string]interface{}{
						"content": []map[string]interface{}{{"type": "text", "text": "recovered"}},
					},
				})
			}
		}))
		defer server.Close()

		client := NewClient(ServerConfig{BaseURL: server.URL + "/mcp"}, server.Client())
		if err := client.ensureInitialized(context.Background()); err != nil {
			t.Fatalf("init failed: %v", err)
		}
		resp, err := client.ExecuteTool(context.Background(), "ping", nil)
		if err != nil {
			t.Fatalf("expected recovery after expired session, got %v", err)
		}
		if len(resp.Content) != 1 || resp.Content[0].Text != "recovered" {
			t.Fatalf("unexpected recovered response: %+v", resp)
		}
	})
}

func TestMCPClient_AuthRequiredAndBearerNormalization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" {
			http.NotFound(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer tb_live_valid" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized","error_description":"Send Authorization: Bearer <API key or OAuth access token>"}`))
			return
		}
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  map[string]interface{}{"protocolVersion": "2025-03-26"},
			})
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]interface{}{
					"tools": []map[string]interface{}{{"name": "tebar_send", "description": "Send"}},
				},
			})
		}
	}))
	defer server.Close()

	// 1. Missing API key -> errors.Is(err, ErrAuthRequired)
	noKeyClient := NewClient(ServerConfig{BaseURL: server.URL + "/mcp"}, server.Client())
	_, err := noKeyClient.DiscoverTools(context.Background())
	if !errors.Is(err, ErrAuthRequired) {
		t.Fatalf("expected ErrAuthRequired for 401 without api_key, got %v", err)
	}

	// 2. Legacy 403 -> errors.Is(err, ErrAuthRequired)
	legacyForbidden := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mcp/tools" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		http.NotFound(w, r)
	}))
	defer legacyForbidden.Close()
	legacyClient := NewClient(ServerConfig{BaseURL: legacyForbidden.URL}, legacyForbidden.Client())
	_, err = legacyClient.DiscoverTools(context.Background())
	if !errors.Is(err, ErrAuthRequired) {
		t.Fatalf("expected ErrAuthRequired for legacy 403, got %v", err)
	}

	// 3. API key with accidental "Bearer " prefix is normalized and succeeds
	prefixedClient := NewClient(ServerConfig{
		BaseURL: server.URL + "/mcp",
		APIKey:  "Bearer tb_live_valid",
	}, server.Client())
	tools, err := prefixedClient.DiscoverTools(context.Background())
	if err != nil {
		t.Fatalf("expected prefixed Bearer token to succeed, got %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "tebar_send" {
		t.Fatalf("unexpected tools: %+v", tools)
	}
}
