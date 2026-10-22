package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestCRMSearchNotesDistinctRankAndIsolation(t *testing.T) {
	s, token := setupTestChatServer(t)
	ctx := context.Background()
	now := time.Now().UTC()
	budi, _ := s.crmUpsert(ctx, "budi-search", "Manual", "Budi", "")
	direct, _ := s.crmUpsert(ctx, "direct-search", "Manual", "Dinosaur Society", "")
	for i := 0; i < 65; i++ {
		_, err := s.memorySave(ctx, ContactMemory{ID: db.GenerateUUID(), ContactID: budi.ID, Kind: "note", Body: fmt.Sprintf("Long ago his child liked dinosaur toys %d", i), RecordedBy: "You", CreatedAt: now.Add(time.Duration(i) * time.Second), UpdatedAt: now})
		if err != nil {
			t.Fatal(err)
		}
	}
	items, err := s.crmSearch(ctx, "dinosaur", "", "", 0)
	if err != nil || len(items) != 2 || items[0].ID != direct.ID || items[1].ID != budi.ID || items[1].SearchNote == nil {
		t.Fatal("distinct identity ranking", items, err)
	}
	if items, err := s.crmSearch(ctx, "dinosaur", "VIP", "", 0); err != nil || len(items) != 0 {
		t.Fatal("tag filter", err)
	}
	call := func(method, path string, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		if auth {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	for _, path := range []string{"/api/v1/contacts/search?q=dinosaur", "/api/v1/search?q=dinosaur"} {
		if w := call("GET", path, false); w.Code != 401 {
			t.Fatal("public search", w.Code)
		}
	}
	w := call("GET", "/api/v1/contacts/search?q=dinosaur", true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "search_note") {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, q := range []string{"", "x"} {
		if w := call("GET", "/api/v1/search?q="+q, true); w.Code != 400 {
			t.Fatal("global short query", w.Code)
		}
	}
	// The sidebar query is literal; SQL wildcard characters are not operators.
	items, err = s.crmSearch(ctx, "%", "", "", 0)
	if err != nil || len(items) != 0 {
		t.Fatal("wildcard matched all records", err)
	}
	w = call("GET", "/api/v1/search?q=dinosaur", true)
	var result struct{ Groups []UniversalSearchGroup }
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result.Groups) != 5 {
		t.Fatal(w.Body.String())
	}
	for _, g := range result.Groups {
		if g.Kind == "notes" && (len(g.Items) != 5 || !g.HasMore || !strings.Contains(g.Items[0].URL, "note_query=")) {
			t.Fatal("bounded note results", g)
		}
	}
	w = call("GET", "/api/v1/search?q=dinosaur&offset=5", true)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
}

func TestUniversalSearchProjectTaskAndInboxScope(t *testing.T) {
	s, token := setupTestChatServer(t)
	ctx := context.Background()
	now := time.Now().UTC()
	s.projectSvc.Store().CreateProject(ctx, &project.Project{ID: "search-project", Name: "Search dinosaur project", CreatedAt: now, UpdatedAt: now})
	work, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{Title: "Search dinosaur task", ProjectID: "search-project", Instructions: "Review saved networking notes"})
	if err != nil {
		t.Fatal(err)
	}
	room := &db.ProjectRoom{ID: db.GenerateUUID(), ProjectID: "default", Title: "Networking", Topic: "agent-topic:agent_sm", CreatedAt: now, UpdatedAt: now}
	if err = s.projectStudioRepo.CreateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}
	if err = s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{ID: db.GenerateUUID(), RoomID: room.ID, AuthorType: "user", Content: "Discuss the dinosaur event", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	hidden := &db.ProjectRoom{ID: db.GenerateUUID(), ProjectID: "default", Title: "Dinosaur internal task room", Topic: "private-project-discussion", CreatedAt: now, UpdatedAt: now}
	s.projectStudioRepo.CreateRoom(ctx, hidden)
	r := httptest.NewRequest("GET", "/api/v1/search?q=dinosaur", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, r)
	var result struct{ Groups []UniversalSearchGroup }
	json.Unmarshal(w.Body.Bytes(), &result)
	found := map[string]bool{}
	for _, g := range result.Groups {
		if g.Error != "" {
			t.Fatal(g.Error)
		}
		for _, item := range g.Items {
			if item.ID == hidden.ID {
				t.Fatal("non-Inbox room linked as a conversation")
			}
			if item.ID == room.ID {
				found["room"] = true
			}
			if item.ID == work.ID {
				found["task"] = true
			}
			if item.ID == "search-project" {
				found["project"] = true
			}
		}
	}
	if !found["room"] || !found["task"] || !found["project"] {
		t.Fatal("missing categories", w.Body.String())
	}
	// Query strings and note contents are returned as JSON/plain text, never HTML.
	data, _ := json.Marshal(UniversalSearchItem{Title: "<img src=x>", URL: "/contacts?contact=ok"})
	if bytes.Contains(data, []byte("<img")) {
		t.Fatal("unsafe JSON escaping")
	}
}
