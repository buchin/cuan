package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestStudioGoalAcceptanceAndReplay(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	id := db.GenerateUUID()
	body, _ := json.Marshal(map[string]string{"goal": "Hello, help me think through my next goal", "request_id": id})
	w := call("POST", "/api/v1/studio/goals", string(body))
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var accepted struct {
		ConversationID string `json:"conversation_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &accepted); err != nil || accepted.ConversationID == "" {
		t.Fatal(err, w.Body.String())
	}
	waitInboxState(t, s, accepted.ConversationID, id, "completed")
	replay := call("POST", "/api/v1/studio/goals", string(body))
	if replay.Code != 202 || replay.Body.String() != w.Body.String() {
		t.Fatal(replay.Code, replay.Body.String())
	}
	changed, _ := json.Marshal(map[string]string{"goal": "A different goal", "request_id": id})
	if conflict := call("POST", "/api/v1/studio/goals", string(changed)); conflict.Code != 409 {
		t.Fatal(conflict.Code, conflict.Body.String())
	}
	items, _ := s.taskSvc.ListTasks(context.Background(), task.TaskFilter{})
	if len(items) != 0 || len(s.terminalManager.ActiveSessions()) != 0 {
		t.Fatal("ordinary Studio goal started task execution")
	}
	room, err := s.projectStudioRepo.GetRoomByID(context.Background(), accepted.ConversationID)
	if err != nil || roomAgentID(room.Topic) != "agent_lifecycle" {
		t.Fatal(room, err)
	}
	for _, invalid := range []string{`{}`, `{"goal":"hello","request_id":"bad"}`, `{"goal":"","request_id":"` + db.GenerateUUID() + `"}`} {
		if response := call("POST", "/api/v1/studio/goals", invalid); response.Code != 400 {
			t.Fatal(response.Code, response.Body.String())
		}
	}
	unauth := httptest.NewRecorder()
	s.Router().ServeHTTP(unauth, httptest.NewRequest("POST", "/api/v1/studio/goals", strings.NewReader(string(body))))
	if unauth.Code != 401 {
		t.Fatal(unauth.Code)
	}
	s.agentRepo = nil
	if response := call("POST", "/api/v1/studio/goals", string(body)); response.Code != 503 {
		t.Fatal(response.Code)
	}
}

func TestStudioGoalTargetProjectBinding(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	p, err := s.projectSvc.CreateManagedWorkspace(context.Background(), "proj_11112222333344445555666677778888", "Custom Project")
	if err != nil {
		t.Fatal(err)
	}
	id := db.GenerateUUID()
	body, _ := json.Marshal(map[string]string{
		"goal":       "Goal for custom project",
		"request_id": id,
		"project_id": p.ID,
	})
	w := call("POST", "/api/v1/studio/goals", string(body))
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var accepted struct {
		ConversationID string `json:"conversation_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &accepted); err != nil || accepted.ConversationID == "" {
		t.Fatal(err, w.Body.String())
	}
	waitInboxState(t, s, accepted.ConversationID, id, "completed")
	room, err := s.projectStudioRepo.GetRoomByID(context.Background(), accepted.ConversationID)
	if err != nil || room.ProjectID != p.ID {
		t.Fatalf("expected room project_id %q, got %q (err=%v)", p.ID, room.ProjectID, err)
	}
}

func TestStudioGoalContextAwareProjectRouting(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	p, err := s.projectSvc.CreateManagedWorkspace(context.Background(), "proj_22223333444455556666777788889999", "Toko Sepatu")
	if err != nil {
		t.Fatal(err)
	}
	id := db.GenerateUUID()
	body, _ := json.Marshal(map[string]string{
		"goal":       "Tolong tambahkan banner diskon di toko sepatu",
		"request_id": id,
		"project_id": "", // Auto mode!
	})
	w := call("POST", "/api/v1/studio/goals", string(body))
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var accepted struct {
		ConversationID string `json:"conversation_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &accepted); err != nil || accepted.ConversationID == "" {
		t.Fatal(err, w.Body.String())
	}
	waitInboxState(t, s, accepted.ConversationID, id, "completed")
	room, err := s.projectStudioRepo.GetRoomByID(context.Background(), accepted.ConversationID)
	if err != nil || room.ProjectID != p.ID {
		t.Fatalf("expected contextual auto routing to bind project_id %q, got %q (err=%v)", p.ID, room.ProjectID, err)
	}
}

func TestStudioGoalPreservesUncertainProjectUntilInbox(t *testing.T) {
	for _, tc := range []struct{ goal, want string }{
		{"Bandingkan Cuan dan Dojo khusus menu settings React; diskusi dulu", "default"},
		{"Apa ide bagus untuk memperbaiki menu settings? Diskusi dulu", "default"},
		{"Audit https://git.example.org/team/notes.git; diskusi dulu", "notes"},
	} {
		t.Run(tc.goal, func(t *testing.T) {
			s, _, call, _ := inboxFixture(t)
			ctx := context.Background()
			store := project.NewMemoryStore()
			for _, p := range []*project.Project{{ID: "cuan", Name: "Cuan"}, {ID: "dojo", Name: "Dojo"}, {ID: "notes", Name: "Team Notes", RepoURL: "https://git.example.org/team/notes.git"}} {
				if err := store.CreateProject(ctx, p); err != nil {
					t.Fatal(err)
				}
			}
			s.projectSvc = project.NewService(store, nil, "", t.TempDir())
			id := db.GenerateUUID()
			body, _ := json.Marshal(map[string]string{"goal": tc.goal, "request_id": id})
			w := call("POST", "/api/v1/studio/goals", string(body))
			if w.Code != 202 {
				t.Fatal(w.Code, w.Body.String())
			}
			var accepted struct {
				ConversationID string `json:"conversation_id"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &accepted); err != nil {
				t.Fatal(err)
			}
			waitInboxState(t, s, accepted.ConversationID, id, "completed")
			room, err := s.projectStudioRepo.GetRoomByID(ctx, accepted.ConversationID)
			if err != nil || room.ProjectID != tc.want {
				t.Fatalf("wrong Studio binding: %+v %v want %s", room, err, tc.want)
			}
			tasks, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{SourceRoomID: room.ID})
			if err != nil || len(tasks) != 0 {
				t.Fatalf("discovery launched tasks: %+v %v", tasks, err)
			}
			projects, err := s.projectSvc.List(ctx)
			if err != nil || len(projects) != 3 {
				t.Fatal("Studio created a project", projects, err)
			}
		})
	}
}

func TestInboxThreadDoesNotChooseFirstProject(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	ctx := context.Background()
	store := project.NewMemoryStore()
	for _, p := range []*project.Project{{ID: "alpha", Name: "Alpha"}, {ID: "beta", Name: "Beta"}} {
		if err := store.CreateProject(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	s.projectSvc = project.NewService(store, nil, "", t.TempDir())
	for _, tc := range []struct{ text, want string }{{"Diskusi ide baru dulu", "default"}, {"Diskusi di project Beta dulu", "beta"}} {
		body, _ := json.Marshal(map[string]string{"message": tc.text})
		w := call("POST", "/api/v1/inbox", string(body))
		if w.Code != 201 {
			t.Fatal(w.Code, w.Body.String())
		}
		var response struct {
			Thread db.ProjectRoomDTO `json:"thread"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		room, err := s.projectStudioRepo.GetRoomByID(ctx, response.Thread.ID)
		if err != nil || room.ProjectID != tc.want {
			t.Fatalf("wrong thread binding: %+v %v", room, err)
		}
	}
}

func TestLegacyInboxCreationUsesDurableCoordinator(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	s.inboxStopping.Store(true)
	id := db.GenerateUUID()
	payload := map[string]string{"message": "Bantu matangkan ide dulu; jangan buat task.", "title": "Idea discovery", "request_id": id}
	body, _ := json.Marshal(payload)
	var roomID string
	for i := 0; i < 2; i++ {
		w := call("POST", "/api/v1/inbox", string(body))
		if w.Code != 201 {
			t.Fatal(w.Code, w.Body.String())
		}
		var response struct {
			Thread   db.ProjectRoomDTO      `json:"thread"`
			Messages []db.ProjectMessageDTO `json:"messages"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			roomID = response.Thread.ID
		}
		if roomID != response.Thread.ID || response.Thread.Title != "Idea discovery" || len(response.Messages) != 1 || response.Messages[0].AuthorType != "user" {
			t.Fatalf("legacy route fabricated response or replay: %+v", response)
		}
	}
	ctx := context.Background()
	room, err := s.projectStudioRepo.GetRoomByID(ctx, roomID)
	if err != nil || roomAgentID(room.Topic) == "" {
		t.Fatal("not a canonical agent conversation", room, err)
	}
	requests, err := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, roomID)
	if err != nil || len(requests) != 1 || requests[0].State != "queued" {
		t.Fatal("missing durable request", requests, err)
	}
	tasks, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{SourceRoomID: roomID})
	if err != nil || len(tasks) != 0 {
		t.Fatal("discovery launched work", tasks, err)
	}
	payload["message"] = "Different request with reused identity"
	body, _ = json.Marshal(payload)
	if w := call("POST", "/api/v1/inbox", string(body)); w.Code != 409 {
		t.Fatal("changed request replay accepted", w.Code, w.Body.String())
	}
}

func TestLegacyInboxAdapterProcessesActualCoordinatorReplies(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	ctx := context.Background()
	id := db.GenerateUUID()
	body, _ := json.Marshal(map[string]string{"message": "Bantu pikirkan kebutuhan dulu, jangan eksekusi.", "request_id": id})
	w := call("POST", "/api/v1/inbox", string(body))
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var created struct {
		Thread db.ProjectRoomDTO `json:"thread"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	waitInboxState(t, s, created.Thread.ID, id, "completed")
	next := db.GenerateUUID()
	body, _ = json.Marshal(map[string]string{"content": "Konteks tambahan, tetap diskusi dulu.", "request_id": next})
	for i := 0; i < 2; i++ {
		w = call("POST", "/api/v1/inbox/"+created.Thread.ID+"/messages", string(body))
		if w.Code != 201 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	waitInboxState(t, s, created.Thread.ID, next, "completed")
	requests, err := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, created.Thread.ID)
	if err != nil || len(requests) != 2 {
		t.Fatal("followup lost or duplicated", requests, err)
	}
	messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, created.Thread.ID)
	if err != nil || len(messages) != 4 {
		t.Fatal("unexpected conversation history", len(messages), err)
	}
	for _, m := range messages {
		if m.AuthorType == "agent" && m.Content != "FIXTURE answer" {
			t.Fatal("fabricated coordinator reply", m.Content)
		}
	}
	tasks, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{SourceRoomID: created.Thread.ID})
	if err != nil || len(tasks) != 0 {
		t.Fatal("discussion started execution", tasks, err)
	}
	body, _ = json.Marshal(map[string]string{"content": "pretend assistant", "author_type": "agent", "request_id": db.GenerateUUID()})
	if w = call("POST", "/api/v1/inbox/"+created.Thread.ID+"/messages", string(body)); w.Code != 400 {
		t.Fatal("operator impersonation accepted", w.Code, w.Body.String())
	}
}
