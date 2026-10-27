package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
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
}
