package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBYOKProvider_InitializationAndRegistry(t *testing.T) {
	providers := []string{
		ProviderOpenAI,
		ProviderDeepSeek,
		ProviderAnthropic,
		ProviderGrok,
		ProviderOpenRouter,
		ProviderGroq,
		ProviderOllama,
	}

	reg := DefaultRegistry()
	for _, pType := range providers {
		p, err := reg.Get(pType)
		if err != nil {
			t.Fatalf("expected provider %q in DefaultRegistry, got error: %v", pType, err)
		}
		if p.Type() != pType {
			t.Errorf("expected type %q, got %q", pType, p.Type())
		}
		if p.Name() == "" {
			t.Errorf("expected non-empty name for provider %q", pType)
		}

		models, err := p.ListModels(context.Background(), "mock_key")
		if err != nil {
			t.Fatalf("ListModels failed for %q: %v", pType, err)
		}
		if len(models) == 0 {
			t.Errorf("expected models for %q, got empty", pType)
		}
	}
}

func TestBYOKProvider_VerifyConnection_MockAndValidation(t *testing.T) {
	ctx := context.Background()
	p := NewUniversalBYOKProvider(ProviderDeepSeek)

	// 1. Empty key check
	probe, err := p.VerifyConnection(ctx, "")
	if err != ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials on empty key, got: %v", err)
	}
	if probe == nil || probe.Valid {
		t.Errorf("expected probe.Valid to be false on empty key")
	}

	// 2. Invalid key check
	probe, err = p.VerifyConnection(ctx, "invalid-key-xyz")
	if err == nil || !strings.Contains(err.Error(), "connection verification failed") {
		t.Errorf("expected ErrConnectionVerification on invalid key, got: %v", err)
	}
	if probe == nil || probe.Valid {
		t.Errorf("expected probe.Valid to be false on invalid key")
	}

	// 3. Mock key check
	probe, err = p.VerifyConnection(ctx, "mock_test_key_123")
	if err != nil {
		t.Fatalf("unexpected error on mock key: %v", err)
	}
	if !probe.Valid {
		t.Errorf("expected probe.Valid to be true on mock key")
	}
	if probe.LatencyMs <= 0 {
		t.Errorf("expected positive latency, got %d", probe.LatencyMs)
	}
	if len(probe.AvailableModels) == 0 {
		t.Errorf("expected available models in probe result")
	}
}

func TestBYOKProvider_OpenAICompatible_LiveHTTPSimulation(t *testing.T) {
	// Create mock HTTP server simulating OpenAI /chat/completions and /models
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer sk-live-deepseek-key-12345" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"message": "Incorrect API key provided"},
			})
			return
		}

		switch r.URL.Path {
		case "/models":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "deepseek-chat"},
					{"id": "deepseek-reasoner"},
				},
			})
		case "/chat/completions":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			isStream, _ := req["stream"].(bool)
			if isStream {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Halo \"}}]}\n\n"))
				w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"dari DeepSeek!\"}}]}\n\n"))
				w.Write([]byte("data: [DONE]\n\n"))
				return
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"id":    "chatcmpl-test",
				"model": "deepseek-chat",
				"choices": []map[string]any{
					{
						"index": 0,
						"message": map[string]any{
							"role":    "assistant",
							"content": "Halo! Saya DeepSeek V3 yang super hemat.",
						},
						"finish_reason": "stop",
					},
				},
				"usage": map[string]any{
					"prompt_tokens":     12,
					"completion_tokens": 18,
					"total_tokens":      30,
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	p := NewUniversalBYOKProvider(ProviderDeepSeek, server.URL)
	ctx := context.Background()

	// 1. Verify Connection against test server
	probe, err := p.VerifyConnection(ctx, "sk-live-deepseek-key-12345")
	if err != nil {
		t.Fatalf("VerifyConnection failed: %v", err)
	}
	if !probe.Valid {
		t.Errorf("expected valid probe result, got false")
	}
	if len(probe.AvailableModels) != 2 {
		t.Errorf("expected 2 models, got %d", len(probe.AvailableModels))
	}

	// 2. Generate content (non-streaming)
	genResp, err := p.Generate(ctx, "sk-live-deepseek-key-12345", AIProviderRequest{
		Model:  "deepseek-chat",
		Prompt: "Halo DeepSeek",
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if !strings.Contains(genResp.Content, "super hemat") {
		t.Errorf("unexpected content: %q", genResp.Content)
	}
	if genResp.Usage.TotalTokens != 30 {
		t.Errorf("expected 30 total tokens, got %d", genResp.Usage.TotalTokens)
	}

	// 3. GenerateContent interface wrapper
	text, err := p.GenerateContent(ctx, "sk-live-deepseek-key-12345", "deepseek-chat", "System prompt", "User prompt")
	if err != nil {
		t.Fatalf("GenerateContent failed: %v", err)
	}
	if !strings.Contains(text, "super hemat") {
		t.Errorf("unexpected text: %q", text)
	}

	// 4. Stream content (SSE)
	var chunks []string
	streamResp, err := p.Stream(ctx, "sk-live-deepseek-key-12345", AIProviderRequest{
		Model:  "deepseek-chat",
		Prompt: "Streaming test",
	}, func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}
	if streamResp.Content != "Halo dari DeepSeek!" {
		t.Errorf("unexpected streamed content: %q", streamResp.Content)
	}
	if len(chunks) != 2 {
		t.Errorf("expected 2 chunks, got %d: %v", len(chunks), chunks)
	}

	// 5. Unauthorized key error check
	_, err = p.Generate(ctx, "sk-wrong-key-999", AIProviderRequest{Prompt: "test"})
	if err == nil {
		t.Fatal("expected error on unauthorized key, got nil")
	}
	if !strings.Contains(err.Error(), "Incorrect API key") && !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 or error message, got: %v", err)
	}
}

func TestBYOKProvider_Anthropic_LiveHTTPSimulation(t *testing.T) {
	// Create mock HTTP server simulating Anthropic /v1/messages and /v1/models
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("x-api-key")
		if apiKey != "sk-ant-valid-key-9999" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{
				"type": "error",
				"error": map[string]any{
					"type":    "authentication_error",
					"message": "invalid x-api-key",
				},
			})
			return
		}

		if r.Header.Get("anthropic-version") == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("missing anthropic-version header"))
			return
		}

		switch r.URL.Path {
		case "/models":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "claude-3-5-sonnet-20241022"},
				},
			})
		case "/messages":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			isStream, _ := req["stream"].(bool)
			if isStream {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello from \"}}\n\n"))
				w.Write([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Claude 3.5!\"}}\n\n"))
				w.Write([]byte("data: [DONE]\n\n"))
				return
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"id":    "msg_test123",
				"type":  "message",
				"model": "claude-3-5-sonnet-20241022",
				"content": []map[string]any{
					{
						"type": "text",
						"text": "Hello! I am Claude 3.5 Sonnet running via Anthropic Messages API.",
					},
				},
				"stop_reason": "end_turn",
				"usage": map[string]any{
					"input_tokens":  25,
					"output_tokens": 40,
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	p := NewUniversalBYOKProvider(ProviderAnthropic, server.URL)
	ctx := context.Background()

	// 1. Verify Connection
	probe, err := p.VerifyConnection(ctx, "sk-ant-valid-key-9999")
	if err != nil {
		t.Fatalf("Anthropic VerifyConnection failed: %v", err)
	}
	if !probe.Valid {
		t.Errorf("expected valid probe result")
	}

	// 2. Generate content (non-streaming)
	resp, err := p.Generate(ctx, "sk-ant-valid-key-9999", AIProviderRequest{
		Model:        "claude-3-5-sonnet-20241022",
		SystemPrompt: "You are a helpful assistant.",
		Prompt:       "Hi Claude",
	})
	if err != nil {
		t.Fatalf("Anthropic Generate failed: %v", err)
	}
	if !strings.Contains(resp.Content, "Claude 3.5 Sonnet") {
		t.Errorf("unexpected Anthropic content: %q", resp.Content)
	}
	if resp.Usage.InputTokens != 25 || resp.Usage.OutputTokens != 40 || resp.Usage.TotalTokens != 65 {
		t.Errorf("unexpected token counts: %+v", resp.Usage)
	}

	// 3. Stream content
	var chunks []string
	streamResp, err := p.Stream(ctx, "sk-ant-valid-key-9999", AIProviderRequest{
		Model:  "claude-3-5-sonnet-20241022",
		Prompt: "Stream me",
	}, func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("Anthropic Stream failed: %v", err)
	}
	if streamResp.Content != "Hello from Claude 3.5!" {
		t.Errorf("unexpected Anthropic streamed content: %q", streamResp.Content)
	}
	if len(chunks) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(chunks))
	}
}

func TestIsModelVisionCapable_BYOK(t *testing.T) {
	tests := []struct {
		provider string
		model    string
		expected bool
	}{
		{"openai", "gpt-4o", true},
		{"openai", "gpt-4o-mini", true},
		{"openai", "o1", true},
		{"openai", "o3-mini", false},
		{"deepseek", "deepseek-chat", false},
		{"deepseek", "deepseek-reasoner", false},
		{"anthropic", "claude-3-5-sonnet-20241022", true},
		{"anthropic", "claude-3-5-haiku-20241022", true},
		{"claude", "claude-3-opus-20240229", true},
		{"grok", "grok-2-latest", true},
		{"grok", "grok-beta", false},
		{"ollama", "llava:13b", true},
		{"ollama", "llama3", false},
	}

	for _, tt := range tests {
		got := IsModelVisionCapable(tt.provider, tt.model)
		if got != tt.expected {
			t.Errorf("IsModelVisionCapable(%q, %q) = %v, want %v", tt.provider, tt.model, got, tt.expected)
		}
	}
}
