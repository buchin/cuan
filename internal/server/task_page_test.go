package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/buchin/cuan/internal/task"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTaskBoardAPIIsBoundedAndKeepsLegacyList(t *testing.T) {
	svc := task.NewService(task.NewMemoryStore(), nil)
	ctx := context.Background()
	for i := 0; i < 7; i++ {
		if _, err := svc.CreateTask(ctx, "owner", task.CreateTaskRequest{Title: fmt.Sprintf("Board work %02d", i), Instructions: "Approved scope"}); err != nil {
			t.Fatal(err)
		}
	}
	s := &Server{taskSvc: svc}
	read := func(url string) map[string]json.RawMessage {
		t.Helper()
		rec := httptest.NewRecorder()
		s.handleListTasks(rec, httptest.NewRequest(http.MethodGet, url, nil))
		if rec.Code != 200 {
			t.Fatalf("list failed: %d %s", rec.Code, rec.Body.String())
		}
		var body map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body
	}
	paged := read("/api/v1/tasks?view=board&page=2&page_size=2&sort=title")
	var rows []task.TaskItemResponse
	var total int
	if err := json.Unmarshal(paged["tasks"], &rows); err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(paged["total"], &total)
	if len(rows) != 2 || total != 7 || rows[0].Title != "Board work 02" {
		t.Fatal("API ignored page/order", len(rows), total)
	}
	legacy := read("/api/v1/tasks")
	json.Unmarshal(legacy["tasks"], &rows)
	if len(rows) != 7 {
		t.Fatal("legacy list changed")
	}

	limitOne := read("/api/v1/tasks?limit=1")
	json.Unmarshal(limitOne["tasks"], &rows)
	json.Unmarshal(limitOne["total"], &total)
	if len(rows) != 1 || total != 1 {
		t.Fatalf("expected 1 task with limit 1, got %d (total %d)", len(rows), total)
	}

	limitTwo := read("/api/v1/tasks?limit=2")
	json.Unmarshal(limitTwo["tasks"], &rows)
	json.Unmarshal(limitTwo["total"], &total)
	if len(rows) != 2 || total != 2 {
		t.Fatalf("expected 2 tasks with limit 2, got %d (total %d)", len(rows), total)
	}
}
