package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestInboxTaskDraftParsingAndProvenance(t *testing.T) {
	valid := "Ini draft, belum tersimpan.\n```cuan-task\n{\"title\":\"Rapikan chat\",\"description\":\"Scope: chat mobile.\\nDoD: tidak overflow.\",\"project_id\":\"p1\"}\n```"
	text, draft := splitInboxTaskDraft("agent", valid)
	if draft == nil || draft.Title != "Rapikan chat" || text != "Ini draft, belum tersimpan." {
		t.Fatal(text, draft)
	}
	for _, c := range []struct{ author, content string }{
		{"user", valid}, {"system", valid}, {"agent", valid + "\nUnexpected trailing text"},
		{"agent", "```cuan-task\nnot json\n```"}, {"agent", "```cuan-task\n{\"title\":\"Title\",\"description\":\"\"}\n```"},
		{"agent", valid + "\n" + valid},
		{"agent", "```cuan-task\n{\"title\":\"" + strings.Repeat("a", 201) + "\",\"description\":\"desc\"}\n```"},
	} {
		text, draft := splitInboxTaskDraft(c.author, c.content)
		if draft != nil || text != c.content {
			t.Fatal("untrusted or invalid draft promoted", c.author)
		}
	}
}

func TestInboxTaskDraftConversationAndConfirmation(t *testing.T) {
	s, _, call, home := inboxFixture(t)
	ctx := context.Background()
	path := "/api/v1/chat/conversations/agent_lifecycle/messages"
	firstID := db.GenerateUUID()
	w := call("POST", path, `{"request_id":"`+firstID+`","message":"Perbaiki header chat mobile, pertahankan warna hijau. DoD: tidak overflow di 360px."}`)
	var first struct{ Request db.InboxRequest }
	json.Unmarshal(w.Body.Bytes(), &first)
	waitInboxState(t, s, first.Request.RoomID, firstID, "completed")
	content := "Draft berdasarkan pembahasan.\n```cuan-task\n{\"title\":\"Perbaiki header chat mobile\",\"description\":\"Pertahankan hijau. Kriteria selesai: tidak overflow di 360px.\",\"project_id\":\"fixture-project\"}\n```"
	raw, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": coordinatorFixtureResponse(content)}})
	script := "#!/bin/sh\nprintf '%s' \"$2\" > \"$HOME/draft-prompt\"\nprintf '%s' '" + string(raw) + "'\n"
	bin := strings.Split(os.Getenv("PATH"), ":")[0]
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	id := db.GenerateUUID()
	w = call("POST", path, `{"request_id":"`+id+`","message":"Bikinin task dari pembahasan tadi"}`)
	var accepted struct{ Request db.InboxRequest }
	json.Unmarshal(w.Body.Bytes(), &accepted)
	waitInboxState(t, s, accepted.Request.RoomID, id, "completed")
	prompt, _ := os.ReadFile(filepath.Join(home, "draft-prompt"))
	if !strings.Contains(string(prompt), "pertahankan warna hijau") || !strings.Contains(string(prompt), "task_draft") {
		t.Fatal("context/contract missing")
	}
	tasks, _ := s.taskSvc.ListTasks(ctx, task.TaskFilter{})
	if len(tasks) != 0 {
		t.Fatal("draft created work without confirmation")
	}
	w = call("GET", path, "")
	var snapshot struct{ Messages []ChatMessageDTO }
	json.Unmarshal(w.Body.Bytes(), &snapshot)
	var reply ChatMessageDTO
	for _, m := range snapshot.Messages {
		if m.UUID == accepted.Request.ReplyID {
			reply = m
		}
	}
	if reply.TaskDraft == nil || strings.Contains(reply.Content, "cuan-task") || reply.TaskDraft.ProjectID != "fixture-project" {
		t.Fatal("draft not recovered in snapshot", w.Body.String())
	}
	s.projectSvc.Store().CreateProject(ctx, &project.Project{ID: "fixture-project", Name: "Fixture", ProjectType: "blank", CreatedAt: time.Now(), UpdatedAt: time.Now()})
	taskPath := "/api/v1/chat/conversations/agent_lifecycle/messages/" + reply.UUID + "/task"
	for _, description := range []string{"", strings.Repeat("a", 6001)} {
		data, _ := json.Marshal(map[string]string{"project_id": "fixture-project", "title": "Title", "description": description})
		if bad := call("POST", taskPath, string(data)); bad.Code != 400 {
			t.Fatal("invalid edited description accepted", bad.Code)
		}
	}
	for i := 0; i < 3; i++ {
		w = call("POST", taskPath, `{"project_id":"fixture-project","title":"Edited title","description":"Edited scope and acceptance criteria."}`)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	tasks, _ = s.taskSvc.ListTasks(ctx, task.TaskFilter{})
	if len(tasks) != 1 || tasks[0].Title != "Edited title" || !strings.Contains(tasks[0].Instructions, "Edited scope") || strings.Contains(tasks[0].Instructions, "cuan-task") || tasks[0].Status != "backlog" || tasks[0].SourceMessageID != reply.UUID {
		t.Fatal("wrong confirmed task", tasks)
	}
	detail, _ := s.taskSvc.GetTaskDetail(ctx, tasks[0].ID)
	if detail.LatestAttempt != nil {
		t.Fatal("creation started execution")
	}
}
