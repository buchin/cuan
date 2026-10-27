package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/server"
)

func TestSystemMode_DefaultMarketing(t *testing.T) {
	cfg := &config.Config{
		AppMode:    "marketing",
		AppVersion: "0.1.0",
		BaseDomain: "cuan.dojo.cc",
	}
	srv := server.NewServer(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/system/mode", nil)
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		AppMode    string `json:"app_mode"`
		Version    string `json:"version"`
		BaseDomain string `json:"base_domain"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.AppMode != "marketing" {
		t.Errorf("expected app_mode 'marketing', got %q", resp.AppMode)
	}
	if resp.Version != "0.1.0" {
		t.Errorf("expected version '0.1.0', got %q", resp.Version)
	}
	if resp.BaseDomain != "cuan.dojo.cc" {
		t.Errorf("expected base_domain 'cuan.dojo.cc', got %q", resp.BaseDomain)
	}
}

func TestSystemMode_StudioMode(t *testing.T) {
	cfg := &config.Config{
		AppMode:    "studio",
		AppVersion: "0.2.0-beta",
		BaseDomain: "custom.domain.com",
	}
	srv := server.NewServer(cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/system/mode", nil)
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		AppMode    string `json:"app_mode"`
		Version    string `json:"version"`
		BaseDomain string `json:"base_domain"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.AppMode != "studio" {
		t.Errorf("expected app_mode 'studio', got %q", resp.AppMode)
	}
	if resp.Version != "0.2.0-beta" {
		t.Errorf("expected version '0.2.0-beta', got %q", resp.Version)
	}
	if resp.BaseDomain != "custom.domain.com" {
		t.Errorf("expected base_domain 'custom.domain.com', got %q", resp.BaseDomain)
	}

	// Test V1 alias
	v1Req := httptest.NewRequest(http.MethodGet, "/api/v1/system/mode", nil)
	v1RR := httptest.NewRecorder()
	srv.Router().ServeHTTP(v1RR, v1Req)
	if v1RR.Code != http.StatusOK {
		t.Fatalf("expected /api/v1/system/mode to return 200, got %d", v1RR.Code)
	}
}

func TestSystemMode_DynamicSetter(t *testing.T) {
	cfg := &config.Config{
		AppMode:    "marketing",
		AppVersion: "0.1.0",
		BaseDomain: "cuan.dojo.cc",
	}
	srv := server.NewServer(cfg, nil)

	if srv.AppMode() != "marketing" {
		t.Errorf("expected AppMode() to be 'marketing', got %s", srv.AppMode())
	}

	srv.SetAppMode("studio")
	if srv.AppMode() != "studio" {
		t.Errorf("expected AppMode() to be 'studio', got %s", srv.AppMode())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/system/mode", nil)
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	var resp struct {
		AppMode string `json:"app_mode"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.AppMode != "studio" {
		t.Errorf("expected response app_mode to reflect setter 'studio', got %s", resp.AppMode)
	}
}

func TestConfig_AppModeEnvParsing(t *testing.T) {
	// 1. Default when unset
	os.Unsetenv("APP_MODE")
	c1 := config.Load()
	if c1.AppMode != "marketing" {
		t.Errorf("expected default AppMode to be 'marketing', got %q", c1.AppMode)
	}

	// 2. Explicit studio
	os.Setenv("APP_MODE", "studio")
	c2 := config.Load()
	if c2.AppMode != "studio" {
		t.Errorf("expected AppMode to be 'studio', got %q", c2.AppMode)
	}

	// 3. Case insensitive STUDIO
	os.Setenv("APP_MODE", "STUDIO")
	c3 := config.Load()
	if c3.AppMode != "studio" {
		t.Errorf("expected AppMode to be 'studio' when env is 'STUDIO', got %q", c3.AppMode)
	}

	// 4. Fallback for invalid mode
	os.Setenv("APP_MODE", "unsupported_mode")
	c4 := config.Load()
	if c4.AppMode != "marketing" {
		t.Errorf("expected AppMode fallback to 'marketing' for invalid mode, got %q", c4.AppMode)
	}

	// Cleanup
	os.Unsetenv("APP_MODE")
}

func TestSystemMode_SubpageNavigationIntegrity(t *testing.T) {
	cfg := &config.Config{
		AppMode:    "studio",
		AppVersion: "0.1.0",
		BaseDomain: "cuan.dojo.cc",
		StaticDir:  "../../web/dist",
	}
	// Try with local web/dist path
	if _, err := os.Stat("web/dist"); err == nil {
		cfg.StaticDir = "web/dist"
	}
	srv := server.NewServer(cfg, nil)

	subpages := []string{
		"/",
		"/studio",
		"/marketing",
		"/sponsor",
		"/sponsors",
		"/admin",
		"/terms",
		"/privacy",
	}

	for _, page := range subpages {
		req := httptest.NewRequest(http.MethodGet, page, nil)
		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected HTTP 200 for subpage %s, got %d", page, rr.Code)
		}
	}
}

