package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
	"github.com/go-chi/chi/v5"
)

func TestHandleApplyTask_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/buchin/cuan/pulls/42/merge", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"sha":     "sha_apply_123456",
			"merged":  true,
			"message": "Pull Request successfully merged",
		})
	})

	mockGH := httptest.NewServer(mux)
	defer mockGH.Close()

	gitClient := git.NewClient()
	gitClient.SetAPIBaseURL(mockGH.URL)

	cfg := &config.Config{
		AuthSecret: "server-test-secret-32-bytes-long",
	}

	memProject := project.NewMemoryStore()
	projectSvc := project.NewService(memProject, gitClient, cfg.AuthSecret, t.TempDir())

	ctx := context.Background()
	proj := &project.Project{
		ID:            "proj_cuan_test",
		Name:          "Cuan Website",
		RepoOwner:     "buchin",
		RepoName:      "cuan",
		DefaultBranch: "main",
		RepoURL:       "https://github.com/buchin/cuan",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := memProject.CreateProject(ctx, proj); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	taskStore := task.NewMemoryStore()
	taskSvc := task.NewService(taskStore, nil)

	createdTask, err := taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{
		ProjectID:    proj.ID,
		Title:        "Improve mobile checkout",
		Instructions: "Fix mobile padding on checkout page",
	})
	if err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	prNum := 42
	prURL := "https://github.com/buchin/cuan/pull/42"
	branch := "cuan/task-checkout"
	reviewStatus := "review"
	_, err = taskSvc.UpdateTask(ctx, createdTask.ID, "runner", task.UpdateTaskRequest{
		PRNumber:   &prNum,
		PRURL:      &prURL,
		BranchName: &branch,
		Status:     &reviewStatus,
	})
	if err != nil {
		t.Fatalf("failed to set task PR fields: %v", err)
	}

	srv := &Server{
		cfg:        cfg,
		projectSvc: projectSvc,
		taskSvc:    taskSvc,
	}

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/tasks/%s/apply", createdTask.ID), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", createdTask.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	srv.handleApplyTask(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var res map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse json response: %v", err)
	}

	if res["applied"] != true {
		t.Fatalf("expected applied: true, got %v", res["applied"])
	}
	if res["sha"] != "sha_apply_123456" {
		t.Fatalf("expected sha 'sha_apply_123456', got %v", res["sha"])
	}

	// Verify task in DB is marked as done with applied note
	updatedTask, err := taskSvc.GetTaskDetail(ctx, createdTask.ID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}
	if updatedTask.Status != "done" {
		t.Fatalf("expected task status 'done', got '%s'", updatedTask.Status)
	}
	if !strings.Contains(updatedTask.CompleteNote, "Perubahan telah berhasil diterapkan") {
		t.Fatalf("expected completion note to mention applied, got '%s'", updatedTask.CompleteNote)
	}
}

// Publication restrictions must stop before touching provider/project services.
// Leaving projectSvc nil makes accidental execution fail instead of passing a
// test that only checks a message after a merge already happened.
func TestApplyTaskRespectsPublicationScope(t *testing.T) {
	for _, instructions := range []string{"Do not push this branch.", "No deployment. Keep local work.", "Do not push or deploy."} {
		t.Run(instructions, func(t *testing.T) {
			ctx := context.Background()
			svc := task.NewService(task.NewMemoryStore(), nil)
			created, err := svc.CreateTask(ctx, "owner", task.CreateTaskRequest{ProjectID: "project", Title: "Local work", Instructions: instructions})
			if err != nil {
				t.Fatal(err)
			}
			branch, status, note := "task/local", "review", "Local artifact verified."
			if _, err := svc.UpdateTask(ctx, created.ID, "runner", task.UpdateTaskRequest{BranchName: &branch, Status: &status, CompleteNote: &note}); err != nil {
				t.Fatal(err)
			}
			s := &Server{taskSvc: svc}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+created.ID+"/apply", nil)
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", created.ID)
			req = req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))
			rec := httptest.NewRecorder()
			s.handleApplyTask(rec, req)
			if rec.Code != http.StatusForbidden || !json.Valid(rec.Body.Bytes()) {
				t.Fatalf("expected explicit JSON denial: %d %s", rec.Code, rec.Body.String())
			}
			if _, err := s.applyTaskInternal(ctx, created.ID, "project", "stale title", "stale summary", branch, 0); err == nil {
				t.Fatal("internal merge ignored persisted scope")
			}
			s.autoApplyAndDeployTask(ctx, created.ID, "project")
			s.triggerAutoDeploy(ctx, "project", created.ID)
			after, err := svc.GetTaskDetail(ctx, created.ID)
			if err != nil {
				t.Fatal(err)
			}
			if after.Status != status || after.CompleteNote != note {
				t.Fatalf("denied publication changed task: %s %s", after.Status, after.CompleteNote)
			}
		})
	}
}

func TestApplyTaskRejectsChangedProjectBeforePublication(t *testing.T) {
	ctx := context.Background()
	svc := task.NewService(task.NewMemoryStore(), nil)
	created, err := svc.CreateTask(ctx, "owner", task.CreateTaskRequest{ProjectID: "current", Title: "Work", Instructions: "Implement feature"})
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{taskSvc: svc}
	if _, err := s.applyTaskInternal(ctx, created.ID, "stale", "Work", "", "branch", 0); err == nil {
		t.Fatal("stale project accepted")
	}
	s.triggerAutoDeploy(ctx, "stale", created.ID)
}
