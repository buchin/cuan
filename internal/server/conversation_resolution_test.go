package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestConversationResolutionRequiresCompletedWorkAndPreservesReadState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, token := setupTestChatServer(t)
	ctx := context.Background()
	room := &db.ProjectRoom{ProjectID: "default", Title: "Resolved fixture", Topic: "visitor"}
	if err := s.projectStudioRepo.CreateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(-time.Minute)
	msg := &db.ProjectMessage{RoomID: room.ID, AuthorType: "user", Content: "Keep the delivered result", CreatedAt: at}
	if err := s.projectStudioRepo.CreateMessage(ctx, msg); err != nil {
		t.Fatal(err)
	}
	call := func(path, body, bearer string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("PATCH", path, strings.NewReader(body))
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	path := "/api/v1/chat/conversations/" + room.ID + "/resolution"
	body := `{"resolved":true,"observed_at":"` + at.Format(time.RFC3339Nano) + `"}`
	if w := call(path, body, ""); w.Code != 401 {
		t.Fatal("unauthenticated resolution", w.Code)
	}
	if w := call("/api/v1/item-preferences?kind=conversation", `{"id":"`+room.ID+`","read":false}`, token); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := call(path, body, token); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	saved, err := s.projectStudioRepo.GetRoomByID(ctx, room.ID)
	if err != nil || !saved.IsResolved(at) {
		t.Fatal("resolution was not shared room state", err)
	}
	owner, _ := s.authSvc.ValidateToken(ctx, token)
	prefs, err := readItemPreferences(owner.ID)
	if err != nil || !prefs["conversation:"+room.ID].Unread {
		t.Fatal("resolution changed read status", err)
	}
	if w := call(path, `{"resolved":false}`, token); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	item := &task.Task{ID: "resolution-pending-task", SourceRoomID: room.ID, Title: "Pending work", Status: "todo", CreatedAt: at, UpdatedAt: at}
	if err := s.taskSvc.Store().CreateTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	if w := call(path, body, token); w.Code != 409 {
		t.Fatal("unfinished task resolved", w.Code, w.Body.String())
	}
	// Resolving with cancel_unfinished: true cancels the pending task and succeeds.
	cancelBody := `{"resolved":true,"observed_at":"` + at.Format(time.RFC3339Nano) + `","cancel_unfinished":true}`
	if w := call(path, cancelBody, token); w.Code != 200 {
		t.Fatal("cancel_unfinished resolution failed", w.Code, w.Body.String())
	}
	cancelledTask, err := s.taskSvc.Store().GetTask(ctx, item.ID)
	if err != nil || (cancelledTask.Status != "cancelled" && cancelledTask.Status != "canceled") {
		t.Fatal("task was not cancelled on resolution", err, cancelledTask)
	}
	// Reopen for the rest of the test
	if w := call(path, `{"resolved":false}`, token); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	item.Status = "done"
	if err := s.taskSvc.Store().UpdateTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	if w := call(path, body, token); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	// A canceled run is terminal even if its historical board row stays blocked.
	item.Status = "blocked"
	if err := s.taskSvc.Store().UpdateTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.Store().CreateAttempt(ctx, &task.TaskAttempt{ID: "resolution-canceled", TaskID: item.ID, Status: "canceled", CreatedAt: at}); err != nil {
		t.Fatal(err)
	}
	if w := call(path, body, token); w.Code != 200 {
		t.Fatal("native cancellation disagreed with list eligibility", w.Code, w.Body.String())
	}
	unchanged, err := s.taskSvc.Store().GetTask(ctx, item.ID)
	if err != nil || unchanged.Status != "blocked" {
		t.Fatal("resolution rewrote task history", err)
	}
	newer := &db.ProjectMessage{RoomID: room.ID, AuthorType: "user", Content: "Additional scope", CreatedAt: at.Add(time.Second)}
	if err := s.projectStudioRepo.CreateMessage(ctx, newer); err != nil {
		t.Fatal(err)
	}
	if w := call(path, body, token); w.Code != 409 {
		t.Fatal("new activity lost", w.Code, w.Body.String())
	}
	messages, _ := s.projectStudioRepo.ListMessagesByRoomID(ctx, room.ID)
	if len(messages) != 2 {
		t.Fatal("resolution deleted history")
	}
	// Existing JSON consumers continue to receive the shared status field.
	raw, _ := json.Marshal(saved.ToDTO())
	if !strings.Contains(string(raw), "resolved_at") {
		t.Fatal("missing DTO resolution")
	}
}
