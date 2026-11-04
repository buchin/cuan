package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
	"github.com/go-chi/chi/v5"
)

func TestCancelledWorkCannotApplyOrDeployFromStaleCallback(t *testing.T) {
	for _, scenario := range []string{"cancelled-task", "canceled-alias", "canceled-attempt", "cancel-requested", "canceled-at"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			store := task.NewMemoryStore()
			svc := task.NewService(store, nil)
			item, err := svc.CreateTask(ctx, "owner", task.CreateTaskRequest{ProjectID: "project", Title: "Implement feature", Instructions: "Implement and deploy the approved feature"})
			if err != nil {
				t.Fatal(err)
			}
			item.Status, item.BranchName, item.CompleteNote = task.TaskStatusReview, "task/obsolete", "Historical report claiming success"
			if scenario == "cancelled-task" {
				item.Status = "cancelled"
			}
			if scenario == "canceled-alias" {
				item.Status = "canceled"
			}
			if err = store.UpdateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			if scenario != "cancelled-task" && scenario != "canceled-alias" {
				now := time.Now().UTC()
				a := &task.TaskAttempt{ID: "attempt", TaskID: item.ID, Status: task.AttemptStatusSucceeded, CreatedAt: now}
				if scenario == "canceled-attempt" {
					a.Status = task.AttemptStatusCanceled
				}
				if scenario == "cancel-requested" {
					a.CancelRequested = true
				}
				if scenario == "canceled-at" {
					a.CanceledAt = &now
				}
				if err = store.CreateAttempt(ctx, a); err != nil {
					t.Fatal(err)
				}
			}
			// No provider/project services: crossing the publication boundary would
			// panic instead of silently passing after a real merge was attempted.
			s := &Server{taskSvc: svc}
			if _, err = s.applyTaskInternal(ctx, item.ID, item.ProjectID, item.Title, "old summary", item.BranchName, 0); err == nil {
				t.Fatal("cancelled branch applied")
			}
			s.autoApplyAndDeployTask(ctx, item.ID, item.ProjectID)
			s.triggerAutoDeploy(ctx, item.ProjectID, item.ID)
			r := httptest.NewRequest(http.MethodPost, "/tasks/"+item.ID+"/apply", nil)
			route := chi.NewRouteContext()
			route.URLParams.Add("id", item.ID)
			r = r.WithContext(context.WithValue(ctx, chi.RouteCtxKey, route))
			w := httptest.NewRecorder()
			s.handleApplyTask(w, r)
			if w.Code < 400 {
				t.Fatal("owner apply bypassed cancellation", w.Code)
			}
			after, err := svc.GetTaskDetail(ctx, item.ID)
			if err != nil || after.Status != item.Status || after.CompleteNote != item.CompleteNote {
				t.Fatal("denied publication changed evidence", err)
			}
		})
	}
}
