package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
)

func TestContactMemoryCRUDSearchAndPagination(t *testing.T) {
	s, token := setupTestChatServer(t)
	ctx := context.Background()
	c, _ := s.crmUpsert(ctx, "owner-memory-test", "Manual", "Budi", "budi@example.invalid")
	other, _ := s.crmUpsert(ctx, "owner-memory-other", "Manual", "Alice", "")
	call := func(method, path, body string, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if auth {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	base := "/api/v1/contacts/" + c.ID + "/memory"
	id := db.GenerateUUID()
	raw := `{"request_id":"` + id + `","kind":"fact","body":"His child likes dinosaurs"}`
	if w := call("POST", base, raw, false); w.Code != 401 {
		t.Fatal("public write", w.Code)
	}
	for i := 0; i < 2; i++ {
		if w := call("POST", base, raw, true); w.Code != 201 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if w := call("POST", base, strings.Replace(raw, "dinosaurs", "horses", 1), true); w.Code != 409 {
		t.Fatal("conflicting retry", w.Code)
	}
	for _, bad := range []string{`{"request_id":"bad","kind":"note","body":"ok"}`, `{"request_id":"` + db.GenerateUUID() + `","kind":"birthday","body":"02-30"}`, `{"request_id":"` + db.GenerateUUID() + `","kind":"note","body":"","source_room_id":"spoof"}`} {
		if w := call("POST", base, bad, true); w.Code != 400 {
			t.Fatal("invalid write", w.Code)
		}
	}
	if w := call("PATCH", "/api/v1/contacts/"+other.ID+"/memory/"+id, `{"kind":"fact","body":"wrong person"}`, true); w.Code != 404 {
		t.Fatal("cross-contact edit", w.Code)
	}
	if w := call("PATCH", base+"/"+id, `{"kind":"fact","body":"His child likes dinosaurs and cycling"}`, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	for i := 0; i < 61; i++ {
		now := time.Now().UTC()
		_, err := s.memorySave(ctx, ContactMemory{ID: db.GenerateUUID(), ContactID: c.ID, Kind: "note", Body: fmt.Sprintf("Meeting %d", i), RecordedBy: "You", CreatedAt: now, UpdatedAt: now})
		if err != nil {
			t.Fatal(err)
		}
	}
	var list struct {
		Items   []ContactMemory
		HasMore bool `json:"has_more"`
	}
	w := call("GET", base, "", true)
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Items) != 50 || !list.HasMore {
		t.Fatal("pagination", w.Body.String())
	}
	w = call("GET", base+"?offset=50", "", true)
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Items) != 12 || list.HasMore {
		t.Fatal("older records unreachable")
	}
	hits, err := s.searchContactMemory(ctx, "", "dinosaur")
	if err != nil || len(hits) != 1 || hits[0].ContactID != c.ID || hits[0].Entry.RecordedBy != "You" {
		t.Fatal("old memory search", hits, err)
	}
	if hits, err := s.searchContactMemory(ctx, other.ID, "dinosaur"); err != nil || len(hits) != 0 {
		t.Fatal("scoped search leak", err)
	}
	w = call("DELETE", base+"/"+id, "", true)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	if hits, _ := s.searchContactMemory(ctx, "", "dinosaur"); len(hits) != 0 {
		t.Fatal("deleted memory recalled")
	}
	if w := call("POST", base, raw, true); w.Code != 409 {
		t.Fatal("deleted note resurrected by retry", w.Code)
	}
}

func TestInboxContactMemoryAuthorizationRecallAndSearch(t *testing.T) {
	s, _ := setupTestChatServer(t)
	ctx := context.Background()
	now := time.Now().UTC()
	c, _ := s.crmUpsert(ctx, "memory-budi", "Manual", "Budi", "budi@example.invalid")
	room := &db.ProjectRoom{ID: db.GenerateUUID(), ProjectID: "default", Title: "Memory test", CreatedAt: now, UpdatedAt: now}
	if err := s.projectStudioRepo.CreateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}
	q := db.InboxRequest{RoomID: room.ID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), OwnerID: "owner", AgentID: "agent-memory", State: "completed", CreatedAt: now, Deadline: now.Add(time.Hour)}
	message := &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: "Remember Budi: his child likes dinosaurs", CreatedAt: now, UpdatedAt: now}
	if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, message); err != nil {
		t.Fatal(err)
	}
	action := func(contact, authorization string) string {
		return "```cuan-memory\n" + fmt.Sprintf(`{"contact_id":%q,"kind":"note","body":"His child likes dinosaurs","authorization_message_id":%q}`, contact, authorization) + "\n```"
	}
	if out := s.applyInboxContactMemory(ctx, q, action(c.ID, db.GenerateUUID())); !strings.Contains(out, "not saved") {
		t.Fatal("foreign authorization", out)
	}
	for i := 0; i < 2; i++ {
		if out := s.applyInboxContactMemory(ctx, q, action(c.ID, q.MessageID)); !strings.HasPrefix(out, "Saved to") {
			t.Fatal(out)
		}
	}
	memories, _ := s.memoryList(ctx, c.ID, "", 0)
	if len(memories) != 1 || memories[0].SourceMessageID != q.MessageID || memories[0].SourceRoomID != q.RoomID {
		t.Fatal("replay/provenance", memories)
	}
	prompt, err := s.inboxContext(ctx, q)
	if err != nil || !strings.Contains(prompt, "dinosaurs") || !strings.Contains(prompt, c.ID) {
		t.Fatal("missing recalled memory", err)
	}
	search := "```cuan-search\n" + fmt.Sprintf(`{"query":"dinosaur","contact_id":"","authorization_message_id":%q}`, q.MessageID) + "\n```"
	result, searched, err := s.inboxMemorySearchResults(ctx, q, search)
	if err != nil || !searched || !strings.Contains(result, "dinosaurs") || !strings.Contains(result, "source_message_id") {
		t.Fatal("search provenance", result, err)
	}
	foreign := q
	foreign.OwnerID = ""
	if _, _, err := s.inboxMemorySearchResults(ctx, foreign, search); err == nil {
		t.Fatal("public memory search")
	}
	duplicate, _ := s.crmUpsert(ctx, "other-budi", "Manual", "Budi", "")
	if out := s.applyInboxContactMemory(ctx, q, action(duplicate.ID, q.MessageID)); !strings.Contains(out, "ambiguous") {
		t.Fatal("ambiguous contact save", out)
	}
	contacts, _ := s.crmList(ctx)
	matches := matchingMemoryContacts(contacts, q.MessageID, []db.ProjectMessage{{ID: q.MessageID, AuthorType: "user", Content: "Remember budi@example.invalid"}})
	if len(matches) != 1 || matches[0].ID != c.ID {
		t.Fatal("email disambiguation")
	}
}

func TestInboxMemorySearchRound(t *testing.T) {
	s, _, call, home := inboxFixture(t)
	ctx := context.Background()
	c, _ := s.crmUpsert(ctx, "networking-fixture", "Manual", "Budi", "")
	now := time.Now().UTC()
	s.memorySave(ctx, ContactMemory{ID: db.GenerateUUID(), ContactID: c.ID, Kind: "fact", Body: "His child likes dinosaurs", RecordedBy: "You", CreatedAt: now, UpdatedAt: now})
	// A fake provider requests a read-only search on its first call, then verifies
	// that real retrieved provenance is passed to the final answer stage.
	script := `#!/usr/bin/env python3
import json,sys,pathlib,os,re
p=pathlib.Path(os.environ['HOME'])/'search-calls'
n=int(p.read_text()) if p.exists() else 0
p.write_text(str(n+1))
prompt=sys.argv[2]
if n==0:
 target=re.search(r'Target message ID: ([a-f0-9-]+)',prompt).group(1)
 response='` + "```cuan-search\\n" + `'+json.dumps({'query':'dinosaur','contact_id':'','authorization_message_id':target})+'` + "\\n```" + `'
else:
 assert 'Read-only contact memory search results' in prompt and 'His child likes dinosaurs' in prompt and 'source_message_id' in prompt
 response='Budi: his child likes dinosaurs. Source: saved contact note.'
print(json.dumps({'event':'result','result':{'status':'SUCCESS','response':response}}))
`
	path := filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy")
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	w := call("POST", "/api/v1/chat/conversations/agent_lifecycle/messages", `{"request_id":"`+db.GenerateUUID()+`","message":"Who has a child interested in dinosaurs?"}`)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result struct{ Request db.InboxRequest }
	json.Unmarshal(w.Body.Bytes(), &result)
	waitInboxState(t, s, result.Request.RoomID, result.Request.RequestID, "completed")
	messages, _ := s.projectStudioRepo.ListMessagesByRoomID(ctx, result.Request.RoomID)
	found := false
	for _, m := range messages {
		if m.AuthorType == "agent" && strings.Contains(m.Content, "Budi:") {
			found = true
		}
	}
	count, _ := os.ReadFile(filepath.Join(home, "search-calls"))
	if !found || string(count) != "2" {
		t.Fatal("search round failed", found, string(count))
	}
}

func TestInboxSearchFinishesWithoutRepeatedTools(t *testing.T) {
	for _, scenario := range []string{"empty", "summary-tool"} {
		t.Run(scenario, func(t *testing.T) {
			s, _, call, home := inboxFixture(t)
			if scenario == "summary-tool" {
				c, _ := s.crmUpsert(context.Background(), "fixture", "Manual", "Budi", "")
				now := time.Now().UTC()
				s.memorySave(context.Background(), ContactMemory{ID: db.GenerateUUID(), ContactID: c.ID, Kind: "fact", Body: "Likes dinosaurs", CreatedAt: now, UpdatedAt: now})
			}
			script := `#!/usr/bin/env python3
import json,sys,pathlib,os,re,time
p=pathlib.Path(os.environ['HOME'])/'bounded-search-calls'
n=int(p.read_text()) if p.exists() else 0
p.write_text(str(n+1))
if n==0:
 target=re.search(r'Target message ID: ([a-f0-9-]+)',sys.argv[2]).group(1)
 response='` + "\x60\x60\x60cuan-search\\n" + `'+json.dumps({'query':'dinosaur','contact_id':'','authorization_message_id':target})+'` + "\\n\x60\x60\x60" + `'
 print(json.dumps({'event':'result','result':{'status':'SUCCESS','response':response}}))
else:
 assert '--dangerously-skip-permissions' not in sys.argv and '--mode' in sys.argv
 print(json.dumps({'event':'step_update','step_update':{'step_index':2,'state':'ACTIVE','step_type':'tool','tool_name':'run_command'}}),flush=True)
 time.sleep(15)
`
			binary := filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy")
			if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			w := call("POST", "/api/v1/chat/conversations/agent_lifecycle/messages", `{"request_id":"`+db.GenerateUUID()+`","message":"brp ip server kita"}`)
			if w.Code != 202 {
				t.Fatal(w.Body.String())
			}
			var result struct{ Request db.InboxRequest }
			json.Unmarshal(w.Body.Bytes(), &result)
			waitInboxState(t, s, result.Request.RoomID, result.Request.RequestID, "completed")
			messages, _ := s.projectStudioRepo.ListMessagesByRoomID(context.Background(), result.Request.RoomID)
			if len(messages) != 2 || !strings.Contains(messages[1].Content, "Saya") {
				t.Fatal("missing useful localized fallback")
			}
			count, _ := os.ReadFile(filepath.Join(home, "bounded-search-calls"))
			expected := "1"
			if scenario == "summary-tool" {
				expected = "2"
			}
			if string(count) != expected {
				t.Fatal("unbounded tool/search stage", string(count))
			}
		})
	}
}
