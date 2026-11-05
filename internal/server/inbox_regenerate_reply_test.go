package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
)

func TestRegenerateReplyIsPrivateScopedAndIdempotent(t *testing.T) {
	s, token, room, _ := receiptFixture(t)
	s.inboxStopping.Store(true)
	ctx := context.Background()
	source := &db.ProjectMessage{ID: db.GenerateUUID(), RoomID: room, AuthorType: "agent", AuthorID: db.AgentIDCuan, Content: "```cuan-customer-reply\n{\"draft\":\"Halo, ane bantu cek ya.\"}\n```"}
	if err := s.projectStudioRepo.CreateMessage(ctx, source); err != nil {
		t.Fatal(err)
	}
	id := db.GenerateUUID()
	post := func(target string, extra map[string]any) *httptest.ResponseRecorder {
		t.Helper()
		body := map[string]any{"request_id": id, "regenerate_reply_id": target}
		for k, v := range extra {
			body[k] = v
		}
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/v1/chat/conversations/"+room+"/messages", strings.NewReader(string(raw)))
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, req)
		return w
	}
	for _, extra := range []map[string]any{nil, {"message": "untrusted client wording"}} {
		w := post(source.ID, extra)
		if w.Code != 202 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	requests, err := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, room)
	if err != nil || len(requests) != 1 || requests[0].AgentID == "" {
		t.Fatal("regenerate not accepted once as agent work", err)
	}
	prompt, err := s.inboxContext(ctx, requests[0])
	if err != nil || !strings.Contains(prompt, "Halo, ane bantu cek ya.") || !strings.Contains(prompt, "Do not send it") {
		t.Fatal("missing scoped draft context", err)
	}
	messages := s.getFormattedMessages(ctx, room, "")
	found := false
	for _, m := range messages {
		if m.UUID == requests[0].MessageID {
			found = true
			if !m.IsInternal || m.RegenerateReplyID != source.ID || m.Content != "Regenerate draft" {
				t.Fatal("regenerate exposed prompt or lost private metadata")
			}
		}
	}
	if !found {
		t.Fatal("missing action audit")
	}
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, httptest.NewRequest("GET", "/api/chat/messages?session_uuid="+room, nil))
	if w.Code != 200 || strings.Contains(w.Body.String(), requests[0].MessageID) || strings.Contains(w.Body.String(), "regenerate_reply_id") {
		t.Fatal("visitor saw private regenerate action")
	}
	if w := post(source.ID, map[string]any{"approve_message_id": db.GenerateUUID()}); w.Code != 400 {
		t.Fatal("combined approval accepted", w.Code)
	}
	foreign := &db.ProjectMessage{ID: db.GenerateUUID(), RoomID: db.GenerateUUID(), AuthorType: "agent", Content: source.Content}
	if err := s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: foreign.RoomID, ProjectID: "default", Title: "Other room"}); err != nil {
		t.Fatal(err)
	}
	if err := s.projectStudioRepo.CreateMessage(ctx, foreign); err != nil {
		t.Fatal(err)
	}
	if w := post(foreign.ID, nil); w.Code != 404 {
		t.Fatal("foreign draft accepted", w.Code)
	}
	if w := post(db.GenerateUUID(), nil); w.Code != 404 {
		t.Fatal("missing draft accepted", w.Code)
	}
}

func TestRegenerateWorkerReturnsDraftAndRejectsTaskAction(t *testing.T) {
	for _, valid := range []bool{true, false} {
		t.Run(fmt.Sprint(valid), func(t *testing.T) {
			s, token, call, home := inboxFixture(t)
			ctx := context.Background()
			owner, err := s.authSvc.ValidateToken(ctx, token)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.createTasteItem(owner.ID, "brand", "Voice", "Use ane and ente"); err != nil {
				t.Fatal(err)
			}
			resolved := s.resolveConversationContext(ctx, "agent_lifecycle", "")
			source := &db.ProjectMessage{ID: db.GenerateUUID(), RoomID: resolved.RoomID, AuthorType: "agent", AuthorID: "agent_lifecycle", Content: "```cuan-customer-reply\n{\"draft\":\"Source draft\"}\n```"}
			if err := s.projectStudioRepo.CreateMessage(ctx, source); err != nil {
				t.Fatal(err)
			}
			reply := coordinatorJSON("", "customer_reply", map[string]any{"draft": "Halo, ane bantu ya."})
			if !valid {
				reply = coordinatorJSON("", "task_draft", map[string]any{"title": "Unexpected task", "description": "Not authorized", "project_id": "default"})
			}
			event, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]any{"status": "SUCCESS", "response": reply}})
			binary, err := exec.LookPath("agy")
			if err != nil {
				t.Fatal(err)
			}
			script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$HOME/regenerate-args\"\nprintf '%s' '" + string(event) + "'\n"
			if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(map[string]string{"request_id": db.GenerateUUID(), "regenerate_reply_id": source.ID})
			w := call("POST", "/api/v1/chat/conversations/agent_lifecycle/messages", string(raw))
			if w.Code != 202 {
				t.Fatal(w.Code, w.Body.String())
			}
			var result struct{ Request db.InboxRequest }
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			state := "completed"
			if !valid {
				state = "failed"
			}
			waitInboxState(t, s, result.Request.RoomID, result.Request.RequestID, state)
			args, err := os.ReadFile(filepath.Join(home, "regenerate-args"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(args), "Use ane and ente") || !strings.Contains(string(args), "Do not use tools") {
				t.Fatal("missing taste or generation-only policy")
			}
			messages := s.getFormattedMessages(ctx, source.RoomID, "")
			drafts := 0
			for _, m := range messages {
				if m.ReplyDraft != "" {
					drafts++
				}
			}
			expected := 1
			if valid {
				expected = 2
			}
			if drafts != expected {
				t.Fatal("unexpected regenerated draft count", drafts)
			}
		})
	}
}

func TestLegacyGeneratedRegenerateCommandHasExactCompatibility(t *testing.T) {
	id := db.GenerateUUID()
	command := "@renamed Buat ulang draft balasan pelanggan dari pesan " + id + ". Gunakan taste personal saya yang tersimpan, sesuaikan dengan konteks pelanggan, pertahankan fakta yang didukung bukti. Kembalikan satu customer reply draft siap Review & Use. Jangan kirim ke pelanggan, jangan membuat task atau menjalankan ulang pekerjaan."
	if decodeRegenerateReplyID(command) != id {
		t.Fatal("old generated action not recognized")
	}
	for _, text := range []string{"Buat ulang draft balasan pelanggan dari pesan " + id, command + " and do something else", "> " + command} {
		if decodeRegenerateReplyID(text) != "" {
			t.Fatal("arbitrary prose interpreted as generated action")
		}
	}
}
