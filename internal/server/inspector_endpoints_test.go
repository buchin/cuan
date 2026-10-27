package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/inspector"
)

func setupTestServerForInspector(t *testing.T) *Server {
	t.Helper()
	tempBase, err := os.MkdirTemp("", "cuan-srv-inspect-*")
	if err != nil {
		t.Fatalf("create temp base: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempBase)
	})

	cfg := &config.Config{
		AuthSecret: "test-auth-secret-32-chars-long!",
	}
	srv := NewServer(cfg, nil)

	// Override base dir on engine to isolated temp directory
	if srv.InspectorEngine() != nil {
		srv.SetInspectorEngine(inspector.NewEngine(
			inspector.WithProjectStore(srv.ProjectService().Store()),
			inspector.WithStudioRepo(srv.projectStudioRepo),
			inspector.WithAgentRepo(srv.agentRepo),
			inspector.WithBaseDir(tempBase),
		))
	}

	return srv
}

func createTestZipPayload(t *testing.T, files map[string]string) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write zip entry %s: %v", name, err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}

	return buf.Bytes()
}

func createMultipartZipRequest(t *testing.T, targetURL string, fieldName, filename string, zipBytes []byte, extraFields map[string]string) *http.Request {
	t.Helper()
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	if len(zipBytes) > 0 {
		part, err := writer.CreateFormFile(fieldName, filename)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := part.Write(zipBytes); err != nil {
			t.Fatalf("write form file: %v", err)
		}
	}

	for k, v := range extraFields {
		if err := writer.WriteField(k, v); err != nil {
			t.Fatalf("write field %s: %v", k, err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, targetURL, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestInspectZipEndpoint_Success(t *testing.T) {
	srv := setupTestServerForInspector(t)

	laravelZip := createTestZipPayload(t, map[string]string{
		"artisan":       "#!/usr/bin/env php\n<?php",
		"composer.json": `{"name": "cuan/laravel-app", "require": {"laravel/framework": "^11.0"}}`,
		".env.example":  "APP_KEY=\nDB_CONNECTION=mysql\nDB_DATABASE=app\n",
		"database.sql":  "CREATE TABLE users (id int) ENGINE=InnoDB;",
		"README.md":     "# Laravel Script\n\n## Installation Guide\nRun composer install.",
	})

	endpoints := []string{
		"/api/v1/inspector/inspect-zip",
		"/api/inspector/inspect-zip",
	}

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			req := createMultipartZipRequest(t, ep, "file", "laravel-app.zip", laravelZip, nil)
			w := httptest.NewRecorder()

			srv.Router().ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
			}

			var resp struct {
				Status             string                     `json:"status"`
				Inspection         inspector.InspectionResult `json:"inspection"`
				RecommendedModules []string                   `json:"recommended_modules"`
				RecommendedTasks   []inspector.TaskTemplate   `json:"recommended_tasks"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Status != "success" {
				t.Errorf("expected status 'success', got %q", resp.Status)
			}
			if resp.Inspection.TechStack.PrimaryStack != inspector.StackLaravel {
				t.Errorf("expected laravel, got %q", resp.Inspection.TechStack.PrimaryStack)
			}
			if len(resp.RecommendedModules) != 4 {
				t.Errorf("expected 4 recommended modules, got %d", len(resp.RecommendedModules))
			}
			if len(resp.RecommendedTasks) != 4 {
				t.Errorf("expected 4 recommended tasks, got %d", len(resp.RecommendedTasks))
			}
		})
	}
}

func TestInspectZipEndpoint_ZipSlip_Rejected(t *testing.T) {
	srv := setupTestServerForInspector(t)

	slipZip := createTestZipPayload(t, map[string]string{
		"nested/../../traversal.txt": "evil content",
	})

	req := createMultipartZipRequest(t, "/api/v1/inspector/inspect-zip", "file", "malicious.zip", slipZip, nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for zip slip, got %d. Body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "zip slip") {
		t.Errorf("expected error to mention zip slip, got: %s", w.Body.String())
	}
}

func TestInspectZipEndpoint_MissingFile(t *testing.T) {
	srv := setupTestServerForInspector(t)

	req := createMultipartZipRequest(t, "/api/v1/inspector/inspect-zip", "wrong_field", "test.zip", []byte("bad"), nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", w.Code)
	}
}

func TestCreateProjectFromZipEndpoint_Success(t *testing.T) {
	srv := setupTestServerForInspector(t)

	nextZip := createTestZipPayload(t, map[string]string{
		"package.json":   `{"name": "cuan-next", "dependencies": {"next": "^14.0", "react": "^18.0"}}`,
		"next.config.js": `module.exports = {};`,
		".env.example":   `NEXT_PUBLIC_API_URL=https://api.example.com\nDATABASE_URL=postgres://localhost\n`,
		"README.md":      `# Next.js SaaS Script\n\n## Setup\nRun npm install and npm run build.`,
	})

	endpoints := []string{
		"/api/v1/projects/from-zip",
		"/api/projects/from-zip",
	}

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			req := createMultipartZipRequest(t, ep, "file", "next-saas.zip", nextZip, map[string]string{
				"name": "My Next SaaS",
			})
			w := httptest.NewRecorder()

			srv.Router().ServeHTTP(w, req)

			if w.Code != http.StatusCreated {
				t.Fatalf("expected 201 Created, got %d. Body: %s", w.Code, w.Body.String())
			}

			var resp inspector.CreateFromZipResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Status != "success" {
				t.Errorf("expected status 'success', got %q", resp.Status)
			}
			if resp.Project == nil {
				t.Fatal("expected project not nil")
			}
			if resp.Project.Name != "My Next SaaS" {
				t.Errorf("expected project name 'My Next SaaS', got %q", resp.Project.Name)
			}
			if resp.Project.SourceType != "zip" {
				t.Errorf("expected source_type 'zip', got %q", resp.Project.SourceType)
			}
			if resp.Room.Title != "Script Setup & Onboarding" {
				t.Errorf("expected room title 'Script Setup & Onboarding', got %q", resp.Room.Title)
			}
			if resp.Agent.Name != "DevOps Setup Assistant" {
				t.Errorf("expected agent 'DevOps Setup Assistant', got %q", resp.Agent.Name)
			}
			if len(resp.Tasks) != 4 {
				t.Fatalf("expected 4 setup tasks, got %d", len(resp.Tasks))
			}

			for _, task := range resp.Tasks {
				if len(task.DoDChecklist) == 0 {
					t.Errorf("task %q has empty DoD checklist", task.Title)
				}
			}
		})
	}
}

func TestCreateProjectFromZipEndpoint_ZipSlip_Rejected(t *testing.T) {
	srv := setupTestServerForInspector(t)

	slipZip := createTestZipPayload(t, map[string]string{
		"../escape.txt": "evil payload",
	})

	req := createMultipartZipRequest(t, "/api/v1/projects/from-zip", "file", "slip.zip", slipZip, map[string]string{
		"name": "Malicious Project",
	})
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d. Body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "zip slip") {
		t.Errorf("expected error to mention zip slip, got: %s", w.Body.String())
	}
}
