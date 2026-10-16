package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/buchin/cuan/internal/config"
)

func TestHealthz(t *testing.T) {
	cfg := &config.Config{}
	srv := NewServer(cfg)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse json response: %v", err)
	}

	if body["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %v", body["status"])
	}
}

func TestApiInfo(t *testing.T) {
	cfg := &config.Config{}
	srv := NewServer(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/info", nil)
	rr := httptest.NewRecorder()

	srv.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse json response: %v", err)
	}

	if body["license"] != "Apache-2.0" {
		t.Errorf("expected license Apache-2.0, got %v", body["license"])
	}
	if body["status"] != "running" {
		t.Errorf("expected status running, got %v", body["status"])
	}
}
