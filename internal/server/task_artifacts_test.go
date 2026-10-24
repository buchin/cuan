package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestTaskArtifactsCannotEscapeOrExposePrivateFiles(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "private.txt")
	os.WriteFile(outside, []byte("PRIVATE_OUTSIDE"), 0600)
	os.WriteFile(filepath.Join(dir, "report.html"), []byte("<p>Delivered</p>"), 0600)
	os.WriteFile(filepath.Join(dir, ".env"), []byte("PRIVATE_ENV"), 0600)
	os.Symlink(outside, filepath.Join(dir, "escape.txt"))
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for _, path := range []string{"../private.txt", outside, ".env", "nested/.git/config", "secret.pem", "escape.txt"} {
		if _, err := readTaskArtifacts(root, "task", "attempt", []string{path}); err == nil {
			t.Fatal("private/escaped artifact accepted", path)
		}
	}
	files, err := readTaskArtifacts(root, "task", "attempt", []string{"report.html"})
	if err != nil || len(files) != 1 {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "report.html"), []byte("CHANGED_AFTER_SNAPSHOT"), 0600)
	if string(files[0].data) != "<p>Delivered</p>" {
		t.Fatal("snapshot changed with source")
	}
	if _, err := readTaskArtifacts(root, "task", "attempt", []string{"report.html", "report.html"}); err == nil {
		t.Fatal("duplicate path accepted")
	}
	os.WriteFile(filepath.Join(dir, "large.txt"), nil, 0600)
	os.Truncate(filepath.Join(dir, "large.txt"), attachmentLimit+1)
	if _, err := readTaskArtifacts(root, "task", "attempt", []string{"large.txt"}); err == nil {
		t.Fatal("oversize file accepted")
	}
}

func TestTaskArtifactReportDeliversPrivateDownloadInInbox(t *testing.T) {
	s, token, call, _ := inboxFixture(t)
	ctx := context.Background()
	owner, err := s.authSvc.ValidateToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	now := time.Now().UTC()
	q := db.InboxRequest{OwnerID: owner.ID, AgentID: "agent_lifecycle", RoomID: room.RoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "completed", CreatedAt: now, Deadline: now.Add(time.Hour), PayloadHash: db.GenerateUUID()}
	_, _, err = s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: "Create a downloadable HTML file", CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := s.taskSvc.CreateTaskFromMessage(ctx, owner.ID, q.MessageID, task.CreateTaskRequest{SourceRoomID: q.RoomID, ProjectID: "default", Title: "Deliver HTML", Instructions: "Approved HTML output", AssignedAgent: q.AgentID})
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.taskSvc.CreateAttempt(ctx, saved.ID, owner.ID, task.CreateAttemptRequest{RunnerID: "local_terminal", AgentName: q.AgentID})
	if err != nil {
		t.Fatal(err)
	}
	doing := task.TaskStatusDoing
	s.taskSvc.UpdateTask(ctx, saved.ID, owner.ID, task.UpdateTaskRequest{Status: &doing})
	s.taskSvc.UpdateAttemptStatus(ctx, a.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning})
	workspace := t.TempDir()
	original := "<!doctype html><title>Offline output</title><p>Works offline</p>"
	os.WriteFile(filepath.Join(workspace, "output.html"), []byte(original), 0600)
	_, _, capability, cleanup, err := s.issueTaskAgent(context.WithValue(ctx, taskAgentWorkspaceKey{}, workspace), saved.ID, a.ID, q.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	report := func(paths string) *httptest.ResponseRecorder {
		request := httptest.NewRequest("POST", "/api/v1/task-agent/"+saved.ID, strings.NewReader(`{"state":"done","summary":"HTML ready","complete_note":"Verified offline file","artifacts":`+paths+`}`))
		request.Header.Set("Authorization", "Bearer "+capability)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, request)
		return w
	}
	if w := report(`["../private.txt"]`); w.Code != 400 {
		t.Fatal("escaped output accepted", w.Code, w.Body.String())
	}
	if w := report(`["output.html"]`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := report(`["output.html"]`); w.Code != 200 {
		t.Fatal("identical delivery was not idempotent", w.Code, w.Body.String())
	}
	detail, _ := s.taskSvc.GetTaskDetail(ctx, saved.ID)
	var assessment map[string]any
	if err := json.Unmarshal([]byte(s.inboxReviewData(ctx, detail)), &assessment); err != nil {
		t.Fatal(err)
	}
	delivered, ok := assessment["delivered_files"].([]any)
	if !ok || len(delivered) != 1 {
		t.Fatal("coordinator did not receive real delivery metadata", assessment)
	}
	messages, err := s.getFormattedMessagesResult(ctx, q.RoomID, "")
	if err != nil {
		t.Fatal(err)
	}
	var attachment conversationAttachment
	for _, m := range messages {
		if len(m.Attachments) > 0 {
			attachment = m.Attachments[0]
		}
	}
	if attachment.Name != "output.html" {
		raw, _ := json.Marshal(messages)
		t.Fatal("download not surfaced in Inbox", string(raw))
	}
	os.WriteFile(filepath.Join(workspace, "output.html"), []byte("REPLACED_SOURCE"), 0600)
	download := call("GET", attachment.URL, "")
	if download.Code != 200 || download.Body.String() != original || !strings.HasPrefix(download.Header().Get("Content-Disposition"), "attachment;") {
		t.Fatal("immutable HTML download failed", download.Code, download.Header())
	}
	preview := call("GET", attachment.URL+"?preview=1", "")
	if preview.Code != 200 || preview.Body.String() != original || !strings.HasPrefix(preview.Header().Get("Content-Disposition"), "inline;") {
		t.Fatal("inline preview failed", preview.Code, preview.Header())
	}
	unauthorized := httptest.NewRecorder()
	s.Router().ServeHTTP(unauthorized, httptest.NewRequest("GET", attachment.URL, nil))
	if unauthorized.Code != 401 {
		t.Fatal("download did not require sign-in", unauthorized.Code)
	}
	if _, _, err := loadAttachment("another-owner", attachment.ID); err == nil {
		t.Fatal("file crossed owner boundary")
	}
}
