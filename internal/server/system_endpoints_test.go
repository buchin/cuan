package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/updater"
)

func TestSystemEndpoints_UpdateAndScheduler(t *testing.T) {
	cfg := &config.Config{
		AppVersion: "0.1.0",
		CommitSHA:  "testsha12345",
	}
	s := NewServer(cfg, nil)

	// 1. Test GET /api/system/update
	reqGet := httptest.NewRequest(http.MethodGet, "/api/system/update", nil)
	rrGet := httptest.NewRecorder()
	s.Router().ServeHTTP(rrGet, reqGet)

	if rrGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/system/update, got %d: %s", rrGet.Code, rrGet.Body.String())
	}

	var info updater.UpdateInfo
	if err := json.Unmarshal(rrGet.Body.Bytes(), &info); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if info.CurrentVersion != "0.1.0" {
		t.Errorf("expected version 0.1.0, got %s", info.CurrentVersion)
	}

	// 2. Test PATCH /api/system/update/settings
	enabled := false
	timeVal := "04:15"
	patchBody, _ := json.Marshal(updater.SettingsRequest{
		AutoUpdateEnabled: &enabled,
		AutoUpdateTime:    &timeVal,
	})
	reqPatch := httptest.NewRequest(http.MethodPatch, "/api/system/update/settings", bytes.NewReader(patchBody))
	rrPatch := httptest.NewRecorder()
	s.Router().ServeHTTP(rrPatch, reqPatch)

	if rrPatch.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from PATCH /api/system/update/settings, got %d", rrPatch.Code)
	}

	var patchedInfo updater.UpdateInfo
	_ = json.Unmarshal(rrPatch.Body.Bytes(), &patchedInfo)
	if patchedInfo.AutoUpdateEnabled != false || patchedInfo.AutoUpdateTime != "04:15" {
		t.Errorf("unexpected settings response: %+v", patchedInfo)
	}

	// 3. Test GET /api/system/scheduler/jobs
	reqSched := httptest.NewRequest(http.MethodGet, "/api/system/scheduler/jobs", nil)
	rrSched := httptest.NewRecorder()
	s.Router().ServeHTTP(rrSched, reqSched)

	if rrSched.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/system/scheduler/jobs, got %d", rrSched.Code)
	}

	var jobsResp struct {
		Jobs []map[string]interface{} `json:"jobs"`
	}
	if err := json.Unmarshal(rrSched.Body.Bytes(), &jobsResp); err != nil {
		t.Fatalf("failed to parse jobs: %v", err)
	}
	if len(jobsResp.Jobs) == 0 {
		t.Errorf("expected at least 1 registered scheduler job (system_update_check)")
	}

	// 4. Test POST /api/system/scheduler/jobs/system_update_check/trigger
	reqTrigger := httptest.NewRequest(http.MethodPost, "/api/system/scheduler/jobs/system_update_check/trigger", nil)
	rrTrigger := httptest.NewRecorder()
	s.Router().ServeHTTP(rrTrigger, reqTrigger)

	if rrTrigger.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d: %s", rrTrigger.Code, rrTrigger.Body.String())
	}

	// 5. Test Scheduled Tasks (CRUD + Toggle + Trigger)
	createTaskBody, _ := json.Marshal(map[string]interface{}{
		"title":         "Follow Up Pelanggan",
		"prompt":        "Kirim reminder ke pelanggan yang belum menyelesaikan transaksi",
		"schedule_type": "daily",
		"time_of_day":   "09:30",
	})
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/system/scheduler/tasks", bytes.NewReader(createTaskBody))
	rrCreate := httptest.NewRecorder()
	s.Router().ServeHTTP(rrCreate, reqCreate)

	if rrCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created from POST /api/system/scheduler/tasks, got %d: %s", rrCreate.Code, rrCreate.Body.String())
	}

	var createdTask map[string]interface{}
	_ = json.Unmarshal(rrCreate.Body.Bytes(), &createdTask)
	taskID, _ := createdTask["id"].(string)
	if taskID == "" {
		t.Fatalf("expected task id in created response")
	}

	// List tasks
	reqList := httptest.NewRequest(http.MethodGet, "/api/system/scheduler/tasks", nil)
	rrList := httptest.NewRecorder()
	s.Router().ServeHTTP(rrList, reqList)
	if rrList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET /api/system/scheduler/tasks, got %d", rrList.Code)
	}

	// Toggle task
	toggleBody, _ := json.Marshal(map[string]interface{}{"enabled": false})
	reqToggle := httptest.NewRequest(http.MethodPatch, "/api/system/scheduler/tasks/"+taskID+"/toggle", bytes.NewReader(toggleBody))
	rrToggle := httptest.NewRecorder()
	s.Router().ServeHTTP(rrToggle, reqToggle)
	if rrToggle.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from PATCH /api/system/scheduler/tasks/:id/toggle, got %d", rrToggle.Code)
	}

	// Trigger task
	reqTriggerTask := httptest.NewRequest(http.MethodPost, "/api/system/scheduler/tasks/"+taskID+"/trigger", nil)
	rrTriggerTask := httptest.NewRecorder()
	s.Router().ServeHTTP(rrTriggerTask, reqTriggerTask)
	if rrTriggerTask.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted from POST /api/system/scheduler/tasks/:id/trigger, got %d", rrTriggerTask.Code)
	}

	// Delete task
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/system/scheduler/tasks/"+taskID, nil)
	rrDel := httptest.NewRecorder()
	s.Router().ServeHTTP(rrDel, reqDel)
	if rrDel.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from DELETE /api/system/scheduler/tasks/:id, got %d", rrDel.Code)
	}
}
