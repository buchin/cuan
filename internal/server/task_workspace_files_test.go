package server

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/buchin/cuan/internal/task"
)

func TestTaskWorkspaceFilesUploadAndUsage(t *testing.T) {
	srv, token := setupTestChatServer(t)

	// Create a test task first
	createBody := `{"title":"Test task for workspace files","prompt":"echo hello"}`
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewBufferString(createBody))
	reqCreate.Header.Set("Authorization", "Bearer "+token)
	reqCreate.Header.Set("Content-Type", "application/json")
	recCreate := httptest.NewRecorder()
	srv.Router().ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("failed to create task: %d %s", recCreate.Code, recCreate.Body.String())
	}

	var taskResp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(recCreate.Body.Bytes(), &taskResp)
	taskID := taskResp.ID

	// 1. Upload a file to the task workspace
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "dataset.csv")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("col1,col2\nval1,val2\n"))
	_ = writer.Close()

	reqUpload := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+taskID+"/files", &body)
	reqUpload.Header.Set("Authorization", "Bearer "+token)
	reqUpload.Header.Set("Content-Type", writer.FormDataContentType())
	recUpload := httptest.NewRecorder()
	srv.Router().ServeHTTP(recUpload, reqUpload)

	if recUpload.Code != http.StatusOK {
		t.Fatalf("upload failed: %d %s", recUpload.Code, recUpload.Body.String())
	}

	var uploadResp struct {
		OK       bool   `json:"ok"`
		Filename string `json:"filename"`
		Path     string `json:"path"`
		Size     int64  `json:"size"`
	}
	if err := json.Unmarshal(recUpload.Body.Bytes(), &uploadResp); err != nil || !uploadResp.OK {
		t.Fatalf("invalid upload response: %v", err)
	}
	if uploadResp.Filename != "dataset.csv" || uploadResp.Path != "./dataset.csv" {
		t.Fatalf("unexpected upload metadata: %+v", uploadResp)
	}

	// Verify file actually exists in workspace
	workDir, err := srv.terminalManager.ResolveTaskWorkspace(taskID)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(workDir, "dataset.csv"))
	if err != nil || !bytes.Contains(content, []byte("val1,val2")) {
		t.Fatalf("uploaded file missing or corrupted: %v", err)
	}

	// 2. Query usage endpoint
	reqUsage := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+taskID+"/usage", nil)
	reqUsage.Header.Set("Authorization", "Bearer "+token)
	recUsage := httptest.NewRecorder()
	srv.Router().ServeHTTP(recUsage, reqUsage)

	if recUsage.Code != http.StatusOK {
		t.Fatalf("usage failed: %d %s", recUsage.Code, recUsage.Body.String())
	}
	var usageResp struct {
		OK        bool `json:"ok"`
		Workspace struct {
			FilesCount int `json:"files_count"`
		} `json:"workspace"`
	}
	_ = json.Unmarshal(recUsage.Body.Bytes(), &usageResp)
	if !usageResp.OK || usageResp.Workspace.FilesCount < 1 {
		t.Fatalf("expected at least 1 file in workspace, got: %+v", usageResp)
	}

	// 3. Query diff endpoint
	reqDiff := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+taskID+"/diff", nil)
	reqDiff.Header.Set("Authorization", "Bearer "+token)
	recDiff := httptest.NewRecorder()
	srv.Router().ServeHTTP(recDiff, reqDiff)
	if recDiff.Code != http.StatusOK {
		t.Fatalf("diff failed: %d %s", recDiff.Code, recDiff.Body.String())
	}

	// 4. Test uploading via attempt ID resolution (e.g. att_...)
	att, err := srv.taskSvc.CreateAttempt(context.Background(), taskID, "owner", task.CreateAttemptRequest{
		RunnerID:  "local_terminal",
		AgentName: "test-agent",
	})
	if err == nil && att != nil {
		var body2 bytes.Buffer
		writer2 := multipart.NewWriter(&body2)
		part2, _ := writer2.CreateFormFile("file", "notes.txt")
		_, _ = part2.Write([]byte("workspace notes"))
		_ = writer2.Close()

		reqUpload2 := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+att.ID+"/files", &body2)
		reqUpload2.Header.Set("Authorization", "Bearer "+token)
		reqUpload2.Header.Set("Content-Type", writer2.FormDataContentType())
		recUpload2 := httptest.NewRecorder()
		srv.Router().ServeHTTP(recUpload2, reqUpload2)
		if recUpload2.Code != http.StatusOK {
			t.Fatalf("upload via attempt ID failed: %d %s", recUpload2.Code, recUpload2.Body.String())
		}
	}
}

func TestDedicatedProjectTerminalUploadResolvesProjectWorkspace(t *testing.T) {
	srv, token := setupTestChatServer(t)

	projBody := `{"name":"cuan-term-upload"}`
	reqProj := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(projBody))
	reqProj.Header.Set("Authorization", "Bearer "+token)
	reqProj.Header.Set("Content-Type", "application/json")
	recProj := httptest.NewRecorder()
	srv.Router().ServeHTTP(recProj, reqProj)
	if recProj.Code != http.StatusCreated {
		t.Fatalf("failed to create project: %d %s", recProj.Code, recProj.Body.String())
	}
	var projResp struct {
		ID        string `json:"id"`
		LocalPath string `json:"local_path"`
	}
	_ = json.Unmarshal(recProj.Body.Bytes(), &projResp)
	if projResp.ID == "" || projResp.LocalPath == "" {
		t.Fatalf("empty project id or local_path: %s", recProj.Body.String())
	}
	projDir := projResp.LocalPath
	if absProjDir, absErr := filepath.Abs(projDir); absErr == nil {
		projDir = absProjDir
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(projDir)
	})

	// Create dedicated project terminal task ("Terminal · cuan-term-upload") with no in-memory PTY session
	taskBody := `{"title":"Terminal · cuan-term-upload","project_id":"` + projResp.ID + `"}`
	reqTask := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewBufferString(taskBody))
	reqTask.Header.Set("Authorization", "Bearer "+token)
	reqTask.Header.Set("Content-Type", "application/json")
	recTask := httptest.NewRecorder()
	srv.Router().ServeHTTP(recTask, reqTask)
	if recTask.Code != http.StatusCreated {
		t.Fatalf("failed to create terminal task: %d %s", recTask.Code, recTask.Body.String())
	}
	var taskResp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(recTask.Body.Bytes(), &taskResp)

	// Upload a screenshot with spaces in filename (like macOS Screenshot)
	const fileName = "Screenshot 2026-11-26 at 15.47.41.png"
	const filePayload = "fake-png-bytes-12345"
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte(filePayload))
	_ = writer.Close()

	reqUpload := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+taskResp.ID+"/files", &body)
	reqUpload.Header.Set("Authorization", "Bearer "+token)
	reqUpload.Header.Set("Content-Type", writer.FormDataContentType())
	recUpload := httptest.NewRecorder()
	srv.Router().ServeHTTP(recUpload, reqUpload)
	if recUpload.Code != http.StatusOK {
		t.Fatalf("upload failed: %d %s", recUpload.Code, recUpload.Body.String())
	}

	var uploadResp struct {
		OK           bool   `json:"ok"`
		Filename     string `json:"filename"`
		Path         string `json:"path"`
		AbsPath      string `json:"abs_path"`
		WorkspaceDir string `json:"workspace_dir"`
	}
	if err := json.Unmarshal(recUpload.Body.Bytes(), &uploadResp); err != nil || !uploadResp.OK {
		t.Fatalf("invalid upload response: %v %s", err, recUpload.Body.String())
	}
	if uploadResp.Filename != fileName || uploadResp.Path != "./"+fileName {
		t.Fatalf("unexpected upload metadata: %+v", uploadResp)
	}
	expectedPath := filepath.Join(projDir, fileName)
	if uploadResp.AbsPath != expectedPath {
		t.Fatalf("expected abs_path %q, got %q (workspace_dir=%q)", expectedPath, uploadResp.AbsPath, uploadResp.WorkspaceDir)
	}
	gotBytes, err := os.ReadFile(expectedPath)
	if err != nil || string(gotBytes) != filePayload {
		t.Fatalf("expected file at %q with payload, err=%v", expectedPath, err)
	}

	// Verify file is NOT dumped into fallback workspaces/<taskID>
	fallbackWorkDir, _ := srv.terminalManager.ResolveTaskWorkspace(taskResp.ID)
	if _, statErr := os.Stat(filepath.Join(fallbackWorkDir, fileName)); statErr == nil {
		t.Fatalf("file should not be in fallback task workspace %q", fallbackWorkDir)
	}

	// Verify raw preview endpoint serves the file with spaces
	reqRaw := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+taskResp.ID+"/files/raw?path=./Screenshot%202026-11-26%20at%2015.47.41.png", nil)
	reqRaw.Header.Set("Authorization", "Bearer "+token)
	recRaw := httptest.NewRecorder()
	srv.Router().ServeHTTP(recRaw, reqRaw)
	if recRaw.Code != http.StatusOK || recRaw.Body.String() != filePayload {
		t.Fatalf("raw fetch failed: %d %q", recRaw.Code, recRaw.Body.String())
	}
}
