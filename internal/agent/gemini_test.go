package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGeminiProvider_MockKeyVerification(t *testing.T) {
	provider := NewGeminiProvider(WithOfflineMock(true))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	probe, err := provider.VerifyConnection(ctx, "mock-gemini-key-12345")
	if err != nil {
		t.Fatalf("unexpected error verifying mock key: %v", err)
	}

	if !probe.Valid {
		t.Fatalf("expected probe.Valid to be true, got false")
	}

	if probe.Provider != "google" {
		t.Errorf("expected provider google, got %s", probe.Provider)
	}

	if len(probe.AvailableModels) == 0 {
		t.Fatalf("expected available models in probe result, got 0")
	}

	hasGemini15Pro := false
	for _, m := range probe.AvailableModels {
		if m.ID == "gemini-1.5-pro" {
			hasGemini15Pro = true
			if !m.IsDefault {
				t.Errorf("expected gemini-1.5-pro to be default model")
			}
		}
	}

	if !hasGemini15Pro {
		t.Errorf("expected gemini-1.5-pro model in probe result")
	}
}

func TestGeminiProvider_EmptyAndInvalidKeys(t *testing.T) {
	provider := NewGeminiProvider()
	ctx := context.Background()

	// Empty key
	probeEmpty, err := provider.VerifyConnection(ctx, "")
	if err != ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials on empty key, got %v", err)
	}
	if probeEmpty != nil && probeEmpty.Valid {
		t.Errorf("expected probeEmpty.Valid to be false")
	}

	// Invalid key
	probeInvalid, err := provider.VerifyConnection(ctx, "invalid-key-xyz")
	if err == nil {
		t.Errorf("expected error on invalid key, got nil")
	}
	if probeInvalid != nil && probeInvalid.Valid {
		t.Errorf("expected probeInvalid.Valid to be false")
	}
}

func TestGeminiProvider_LiveHTTPEndpointSimulation(t *testing.T) {
	// Mock Google Gemini API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "valid-live-gemini-key" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]interface{}{
					{
						"name":            "models/gemini-1.5-pro",
						"displayName":     "Gemini 1.5 Pro Test",
						"description":     "Test Pro Model",
						"inputTokenLimit": 2097152,
						"outputTokenLimit": 8192,
						"supportedGenerationMethods": []string{"generateContent"},
					},
					{
						"name":            "models/gemini-1.5-flash",
						"displayName":     "Gemini 1.5 Flash Test",
						"description":     "Test Flash Model",
						"inputTokenLimit": 1048576,
						"outputTokenLimit": 8192,
						"supportedGenerationMethods": []string{"generateContent"},
					},
					{
						"name":            "models/embedding-001",
						"displayName":     "Embedding 001",
						"description":     "Embedding Model",
					},
				},
			})
			return
		}

		// Bad API key simulation
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    400,
				"message": "API key not valid. Please pass a valid API key.",
				"status":  "INVALID_ARGUMENT",
			},
		})
	}))
	defer mockServer.Close()

	provider := NewGeminiProvider(
		WithBaseURL(mockServer.URL),
		WithHTTPClient(mockServer.Client()),
		WithOfflineMock(false),
	)

	ctx := context.Background()

	// 1. Valid key probe
	probe, err := provider.VerifyConnection(ctx, "valid-live-gemini-key")
	if err != nil {
		t.Fatalf("unexpected error verifying valid key: %v", err)
	}

	if !probe.Valid {
		t.Errorf("expected valid probe, got false")
	}

	if len(probe.AvailableModels) != 2 { // Only gemini- prefixed models should be kept
		t.Errorf("expected 2 gemini models, got %d", len(probe.AvailableModels))
	}

	// 2. Invalid key probe
	badProbe, err := provider.VerifyConnection(ctx, "some-rejected-key")
	if err == nil {
		t.Errorf("expected error for rejected key, got nil")
	}

	if badProbe.Valid {
		t.Errorf("expected invalid probe, got true")
	}

	if !strings.Contains(badProbe.ErrorMessage, "API key not valid") {
		t.Errorf("expected error message to contain 'API key not valid', got: %s", badProbe.ErrorMessage)
	}
}

func TestGeminiProvider_ListModels(t *testing.T) {
	provider := NewGeminiProvider(WithOfflineMock(true))
	ctx := context.Background()

	models, err := provider.ListModels(ctx, "test-key")
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	if len(models) == 0 {
		t.Fatalf("expected non-empty model list")
	}
}

func TestGeminiProvider_GenerateContent(t *testing.T) {
	ctx := context.Background()

	// 1. Offline mock mode
	provider := NewGeminiProvider(WithOfflineMock(true))
	out, err := provider.GenerateContent(ctx, "mock-gemini-key", "gemini-1.5-pro", "You are an SEO expert.", "Keyword research for best ai tools")
	if err != nil {
		t.Fatalf("unexpected error on GenerateContent: %v", err)
	}
	if !strings.Contains(out, "SEO Keyword Research") {
		t.Errorf("expected generated output to contain 'SEO Keyword Research', got: %s", out)
	}

	// 2. Empty key
	_, err = provider.GenerateContent(ctx, "", "gemini-1.5-pro", "", "test prompt")
	if err != ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials on empty key, got %v", err)
	}

	// 3. Invalid key
	_, err = provider.GenerateContent(ctx, "invalid-key-999", "gemini-1.5-pro", "", "test prompt")
	if err == nil {
		t.Errorf("expected error on invalid key, got nil")
	}

	// 4. Live HTTP server simulation for live path
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "generateContent") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		key := r.URL.Query().Get("key")
		if key != "valid-live-key" {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"code":    403,
					"message": "API key not valid",
					"status":  "PERMISSION_DENIED",
				},
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"role": "model",
						"parts": []map[string]interface{}{
							{"text": "# Live Generated Markdown Output\n\nContent generated successfully via Gemini API."},
						},
					},
					"finishReason": "STOP",
				},
			},
		})
	}))
	defer ts.Close()

	liveProv := NewGeminiProvider(WithBaseURL(ts.URL), WithOfflineMock(false))
	liveOut, err := liveProv.GenerateContent(ctx, "valid-live-key", "gemini-1.5-pro", "system instructions", "user prompt")
	if err != nil {
		t.Fatalf("live generate failed: %v", err)
	}
	if !strings.Contains(liveOut, "Live Generated Markdown Output") {
		t.Errorf("expected live output, got: %s", liveOut)
	}
}

