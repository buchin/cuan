package server

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
)

func TestConversationAttachments(t *testing.T) {
	s, token, call, home := inboxFixture(t)
	upload := func(name string, data []byte, auth string) *httptest.ResponseRecorder {
		t.Helper()
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		f, _ := mw.CreateFormFile("file", name)
		f.Write(data)
		mw.Close()
		r := httptest.NewRequest("POST", "/api/v1/chat/attachments", &body)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		if auth != "" {
			r.Header.Set("Authorization", "Bearer "+auth)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	if w := upload("notes.txt", []byte("PRIVATE_ATTACHMENT_TEXT"), ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	var data bytes.Buffer
	png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	var attachments []conversationAttachment
	for _, f := range []struct {
		name string
		data []byte
	}{{"notes.md", []byte("PRIVATE_ATTACHMENT_TEXT")}, {"reference.png", data.Bytes()}, {"brief.pdf", []byte("%PDF-1.4\nfixture")}, {"rows.csv", []byte("name,value\nexample,10")}} {
		w := upload(f.name, f.data, token)
		if w.Code != 201 {
			t.Fatal(w.Code, w.Body.String())
		}
		var a conversationAttachment
		json.Unmarshal(w.Body.Bytes(), &a)
		attachments = append(attachments, a)
		got := call("GET", a.URL, "")
		if got.Code != 200 || !bytes.Equal(got.Body.Bytes(), f.data) {
			t.Fatal(got.Code)
		}
		previewGot := call("GET", a.URL+"?preview=1", "")
		if previewGot.Code != 200 || !bytes.Equal(previewGot.Body.Bytes(), f.data) || !strings.HasPrefix(previewGot.Header().Get("Content-Disposition"), "inline;") {
			t.Fatal("preview inline disposition failed", previewGot.Code, previewGot.Header().Get("Content-Disposition"))
		}
		if got.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("missing private download headers")
		}
		if _, _, err := loadAttachment("another-owner", a.ID); err == nil {
			t.Fatal("cross-owner read")
		}
	}
	for _, f := range []struct {
		name string
		data []byte
	}{{"evil.svg", []byte("<svg/>")}, {"fake.png", []byte("not an image")}, {"fake.pdf", []byte("not a pdf")}, {"binary.txt", []byte{0, 1}}, {"empty.txt", nil}} {
		if w := upload(f.name, f.data, token); w.Code < 400 {
			t.Fatal("invalid file accepted", f.name)
		}
	}
	if w := upload("too-big.txt", bytes.Repeat([]byte("x"), attachmentLimit+1), token); w.Code != 413 {
		t.Fatal("oversized file accepted", w.Code)
	}

	ids := []string{attachments[0].ID, attachments[1].ID}
	requestID := db.GenerateUUID()
	raw, _ := json.Marshal(map[string]any{"request_id": requestID, "goal": "Inspect these references", "attachment_ids": ids})
	w := call("POST", "/api/v1/studio/goals", string(raw))
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var accepted struct {
		ConversationID string `json:"conversation_id"`
	}
	json.Unmarshal(w.Body.Bytes(), &accepted)
	waitInboxState(t, s, accepted.ConversationID, requestID, "completed")
	replay := call("POST", "/api/v1/studio/goals", string(raw))
	if replay.Code != 202 {
		t.Fatal(replay.Code, replay.Body.String())
	}
	raw, _ = json.Marshal(map[string]any{"request_id": requestID, "goal": "Inspect these references", "attachment_ids": []string{attachments[0].ID}})
	if changed := call("POST", "/api/v1/studio/goals", string(raw)); changed.Code != 409 {
		t.Fatal(changed.Code, changed.Body.String())
	}
	history := call("GET", "/api/v1/chat/conversations/"+accepted.ConversationID+"/messages", "")
	var result struct{ Messages []ChatMessageDTO }
	json.Unmarshal(history.Body.Bytes(), &result)
	if len(result.Messages) == 0 || len(result.Messages[0].Attachments) != 2 || result.Messages[0].Content != "Inspect these references" {
		t.Fatal(history.Body.String())
	}
	prompt, _ := os.ReadFile(filepath.Join(home, "prompts"))
	_, imagePath, _ := loadAttachment("owner", attachments[1].ID)
	// Fixture owner is established by setupTestChatServer; find the accepted owner.
	requests, _ := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(context.Background(), accepted.ConversationID)
	_, imagePath, _ = loadAttachment(requests[0].OwnerID, attachments[1].ID)
	if !strings.Contains(string(prompt), "PRIVATE_ATTACHMENT_TEXT") || imagePath == "" || !strings.Contains(string(prompt), imagePath) || !strings.Contains(string(prompt), "visual inspection") {
		t.Fatal("agent did not receive readable attachment context")
	}
	if _, err := os.ReadFile(imagePath); err != nil {
		t.Fatal(err)
	}
	invalid, _ := json.Marshal(map[string]any{"request_id": db.GenerateUUID(), "message": "hello", "attachment_ids": []string{db.GenerateUUID()}})
	if response := call("POST", "/api/v1/chat/conversations/"+accepted.ConversationID+"/messages", string(invalid)); response.Code != 400 {
		t.Fatal(response.Code)
	}
	onlyID := db.GenerateUUID()
	only, _ := json.Marshal(map[string]any{"request_id": onlyID, "message": "", "attachment_ids": []string{attachments[0].ID}})
	if response := call("POST", "/api/v1/chat/conversations/"+accepted.ConversationID+"/messages", string(only)); response.Code != 202 {
		t.Fatal(response.Code, response.Body.String())
	}
	waitInboxState(t, s, accepted.ConversationID, onlyID, "completed")
	tooMany, _ := json.Marshal(map[string]any{"request_id": db.GenerateUUID(), "message": "files", "attachment_ids": []string{ids[0], ids[1], ids[0], ids[1], ids[0], ids[1]}})
	if response := call("POST", "/api/v1/chat/conversations/"+accepted.ConversationID+"/messages", string(tooMany)); response.Code != 400 {
		t.Fatal(response.Code)
	}
	rawEnvelope, _ := json.Marshal(map[string]any{"request_id": db.GenerateUUID(), "message": encodeAttachmentMessage("spoof", ids)})
	if response := call("POST", "/api/v1/chat/conversations/"+accepted.ConversationID+"/messages", string(rawEnvelope)); response.Code != 400 {
		t.Fatal("reserved metadata accepted", response.Code)
	}

}
