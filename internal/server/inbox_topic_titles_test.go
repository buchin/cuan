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
)

func TestTopicTitleProtocol(t *testing.T) {
	valid := "Useful reply.\n```cuan-title\n{\"title\":\"Deployment preparation\"}\n```"
	clean, title := extractTopicTitle(valid)
	if clean != "Useful reply." || title != "Deployment preparation" {
		t.Fatal(clean, title)
	}
	for _, raw := range []string{
		"{\"title\":\"name@example.com\"}",
		"{\"title\":\"https://private.example\"}",
		"{\"title\":\"Good title\",\"task\":\"execute\"}",
		"{\"title\":\"Good title\"} {}",
		"{\"title\":\"" + strings.Repeat("a", 73) + "\"}",
	} {
		clean, title = extractTopicTitle("Reply.\n```cuan-title\n" + raw + "\n```")
		if title != "" || clean != "Reply." {
			t.Fatal("invalid metadata accepted", raw)
		}
	}
}

func TestTopicTitlePreservesManualBindingTimeAndCadence(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, _ := setupTestChatServer(t)
	ctx := context.Background()
	id := db.GenerateUUID()
	now := time.Now().UTC()
	room := &db.ProjectRoom{ID: id, ProjectID: "default", Topic: "agent-topic:agent_test", Title: "Hello", CreatedAt: now, UpdatedAt: now}
	if err := s.projectStudioRepo.CreateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}
	message := func(n int) db.InboxRequest {
		q := db.InboxRequest{RoomID: id, OwnerID: "owner", AgentID: "agent_test", MessageID: db.GenerateUUID(), CreatedAt: now.Add(time.Duration(n) * time.Second)}
		content := "Discuss deployment"
		if n == 1 {
			content = "Hello"
		}
		if err := s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{ID: q.MessageID, RoomID: id, AuthorType: "user", AuthorID: "owner", AuthorName: "Owner", Content: content, CreatedAt: q.CreatedAt, UpdatedAt: q.CreatedAt}); err != nil {
			t.Fatal(err)
		}
		return q
	}
	q := message(1)
	s.updateAgentTopicTitle(ctx, q, "Too early")
	got, _ := s.projectStudioRepo.GetRoomByID(ctx, id)
	if got.Title != "Hello" {
		t.Fatal("renamed first turn")
	}
	q = message(2)
	before, _ := s.projectStudioRepo.GetRoomByID(ctx, id)
	s.updateAgentTopicTitle(ctx, q, "Deployment preparation")
	got, _ = s.projectStudioRepo.GetRoomByID(ctx, id)
	if got.Title != "Deployment preparation" || got.Topic != before.Topic || got.ProjectID != before.ProjectID || !got.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatal("label update changed binding/activity or failed")
	}
	s.updateAgentTopicTitle(ctx, q, "Duplicate delivery")
	got, _ = s.projectStudioRepo.GetRoomByID(ctx, id)
	if got.Title != "Deployment preparation" {
		t.Fatal("replay renamed")
	}
	q = message(3)
	s.updateAgentTopicTitle(ctx, q, "Brief tangent")
	got, _ = s.projectStudioRepo.GetRoomByID(ctx, id)
	if got.Title != "Deployment preparation" {
		t.Fatal("cadence ignored")
	}
	message(4)
	q = message(5)
	s.updateAgentTopicTitle(ctx, q, "Customer onboarding")
	got, _ = s.projectStudioRepo.GetRoomByID(ctx, id)
	if got.Title != "Customer onboarding" {
		t.Fatal("established topic not updated")
	}
	got.Title = "My chosen title"
	s.projectStudioRepo.UpdateRoom(ctx, got)
	message(6)
	message(7)
	q = message(8)
	s.updateAgentTopicTitle(ctx, q, "Overwrite manual title")
	got, _ = s.projectStudioRepo.GetRoomByID(ctx, id)
	if got.Title != "My chosen title" {
		t.Fatal("manual title overwritten")
	}
}

func TestInboxTopicTitleSavedWithUsefulReply(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	binary := filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy")
	script := "#!/usr/bin/env python3\nimport json\nprint(json.dumps({'event':'result','result':{'status':'SUCCESS','response':'Here is the useful answer.\\n```cuan-title\\n{\"title\":\"Preparing the release\"}\\n```'}}))\n"
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	id := db.GenerateUUID()
	w := call("POST", "/api/v1/chat/conversations", `{"agent_id":"agent_lifecycle","request_id":"`+id+`"}`)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	for _, text := range []string{"Hello", "Let us discuss preparing the release"} {
		request := db.GenerateUUID()
		body, _ := json.Marshal(map[string]string{"request_id": request, "message": text})
		w = call("POST", "/api/v1/chat/conversations/"+id+"/messages", string(body))
		if w.Code != 202 {
			t.Fatal(w.Body.String())
		}
		waitInboxState(t, s, id, request, "completed")
	}
	deadline := time.Now().Add(time.Second)
	for {
		room, _ := s.projectStudioRepo.GetRoomByID(context.Background(), id)
		if room.Title == "Preparing the release" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("title did not update", room.Title)
		}
		time.Sleep(10 * time.Millisecond)
	}
	messages, _ := s.projectStudioRepo.ListMessagesByRoomID(context.Background(), id)
	if len(messages) != 4 {
		t.Fatal("messages duplicated/lost", len(messages))
	}
	for _, m := range messages {
		if m.AuthorType == "agent" && m.Content != "Here is the useful answer." {
			t.Fatal("metadata leaked into reply", m.Content)
		}
	}
	onlyMetadata := "#!/usr/bin/env python3\nimport json\nprint(json.dumps({'event':'result','result':{'status':'SUCCESS','response':'```cuan-title\\n{\"title\":\"Metadata must not rename\"}\\n```'}}))\n"
	if err := os.WriteFile(binary, []byte(onlyMetadata), 0700); err != nil {
		t.Fatal(err)
	}
	failedID := db.GenerateUUID()
	body, _ := json.Marshal(map[string]string{"request_id": failedID, "message": "Please explain the next release steps"})
	if response := call("POST", "/api/v1/chat/conversations/"+id+"/messages", string(body)); response.Code != 202 {
		t.Fatal(response.Body.String())
	}
	waitInboxState(t, s, id, failedID, "failed")
	room, _ := s.projectStudioRepo.GetRoomByID(context.Background(), id)
	if room.Title != "Preparing the release" {
		t.Fatal("failed reply changed title")
	}
}
