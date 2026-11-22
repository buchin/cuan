package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstaboxSettingsPersistenceAndMasking(t *testing.T) {
	srv, token := setupTestChatServer(t)
	srv.cfg.WorkspacesDir = filepath.Join(t.TempDir(), "workspaces")

	// 1. Initial settings should be empty / unconfigured
	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings/instabox", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Status   string              `json:"status"`
		Settings InstaboxSettingsDTO `json:"settings"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}
	if resp.Settings.HasAPIKey {
		t.Errorf("expected HasAPIKey to be false initially")
	}

	// 2. Save new settings
	enabled := true
	savePayload := map[string]any{
		"endpoint_url": "https://instabox.dropsugar.test:9222",
		"api_key":      "insta_secret_token_1234567890",
		"enabled":      &enabled,
	}
	rawSave, _ := json.Marshal(savePayload)
	saveReq := httptest.NewRequest(http.MethodPost, "/api/v1/settings/instabox", bytes.NewReader(rawSave))
	saveReq.Header.Set("Authorization", "Bearer "+token)
	saveReq.Header.Set("Content-Type", "application/json")
	wSave := httptest.NewRecorder()
	srv.Router().ServeHTTP(wSave, saveReq)

	if wSave.Code != http.StatusOK {
		t.Fatalf("expected 200 on save, got %d: %s", wSave.Code, wSave.Body.String())
	}

	// 3. Verify saved settings via GET: API key must be masked
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/settings/instabox", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w2, req2)

	var resp2 struct {
		Status   string              `json:"status"`
		Settings InstaboxSettingsDTO `json:"settings"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}

	if !resp2.Settings.HasAPIKey {
		t.Errorf("expected HasAPIKey to be true after save")
	}
	if resp2.Settings.MaskedAPIKey == "insta_secret_token_1234567890" {
		t.Errorf("API key was exposed in plain text!")
	}
	if !strings.HasPrefix(resp2.Settings.MaskedAPIKey, "insta") && !strings.Contains(resp2.Settings.MaskedAPIKey, "*") {
		t.Errorf("expected masked API key format, got %s", resp2.Settings.MaskedAPIKey)
	}
	if resp2.Settings.EndpointURL != "https://instabox.dropsugar.test:9222" {
		t.Errorf("expected endpoint_url 'https://instabox.dropsugar.test:9222', got %s", resp2.Settings.EndpointURL)
	}
	if !resp2.Settings.Enabled {
		t.Errorf("expected enabled to be true")
	}

	// 4. Validate endpoint with mock reachable server
	mockRemote := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte(`{"version":"1.0"}`))
	}))
	defer mockRemote.Close()

	valPayload := map[string]any{
		"endpoint_url": mockRemote.URL,
	}
	rawVal, _ := json.Marshal(valPayload)
	valReq := httptest.NewRequest(http.MethodPost, "/api/v1/settings/instabox/validate", bytes.NewReader(rawVal))
	valReq.Header.Set("Authorization", "Bearer "+token)
	valReq.Header.Set("Content-Type", "application/json")
	wVal := httptest.NewRecorder()
	srv.Router().ServeHTTP(wVal, valReq)

	if wVal.Code != http.StatusOK {
		t.Fatalf("expected 200 on validate, got %d: %s", wVal.Code, wVal.Body.String())
	}

	var valResp struct {
		Status string `json:"status"`
		Valid  bool   `json:"valid"`
	}
	if err := json.Unmarshal(wVal.Body.Bytes(), &valResp); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}
	if !valResp.Valid {
		t.Errorf("expected mock endpoint to be valid")
	}
}
