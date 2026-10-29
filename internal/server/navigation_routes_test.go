package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/buchin/cuan/internal/config"
)

func TestNavigationRoutes_ActiveServing(t *testing.T) {
	distPath, err := filepath.Abs("../../web/dist")
	if err != nil {
		t.Fatalf("failed to get dist path: %v", err)
	}

	cfg := &config.Config{
		Environment: "test",
		StaticDir:   distPath,
	}

	srv := NewServer(cfg, nil)
	srv.setupStaticRoutes()

	routes := []struct {
		name       string
		path       string
		expectCode int
	}{
		{name: "Root route", path: "/", expectCode: http.StatusOK},
		{name: "Chat route", path: "/chat", expectCode: http.StatusOK},
		{name: "Studio route", path: "/studio", expectCode: http.StatusOK},
		{name: "Inbox route", path: "/inbox", expectCode: http.StatusOK},
		{name: "Tasks route", path: "/tasks", expectCode: http.StatusOK},
		{name: "Projects route", path: "/projects", expectCode: http.StatusOK},
		{name: "Account route", path: "/account", expectCode: http.StatusOK},
		{name: "Account Contacts submenu route", path: "/account/contacts", expectCode: http.StatusOK},
		{name: "Legacy Contacts route compatibility", path: "/contacts", expectCode: http.StatusOK},
		{name: "Settings route", path: "/settings", expectCode: http.StatusOK},
	}

	for _, tc := range routes {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()

			srv.Router().ServeHTTP(rec, req)

			if rec.Code != tc.expectCode {
				t.Fatalf("expected HTTP %d for route %s, got %d", tc.expectCode, tc.path, rec.Code)
			}

			// Validate response body contains html
			body := rec.Body.String()
			if len(body) == 0 {
				t.Fatalf("expected non-empty response body for route %s", tc.path)
			}
		})
	}
}
