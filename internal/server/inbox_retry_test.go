package server

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/buchin/cuan/internal/db"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInboxRetryAPIUsesOriginalAndChecksOwnership(t *testing.T) {
	s, token := setupTestChatServer(t)
	s.inboxStopping.Store(true)
	owner, _ := s.authSvc.ValidateToken(context.Background(), token)
	ctx := context.Background()
	now := time.Now().UTC()
	room := db.GenerateUUID()
	if err := s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: room, ProjectID: "default", Title: "Fixture", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	store := s.projectStudioRepo.(db.InboxLifecycleRepository)
	q := db.InboxRequest{OwnerID: owner.ID, AgentID: "agent_fixture", RoomID: room, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), PayloadHash: "attachment-snapshot", State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	if _, _, err := store.AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: room, AuthorType: "user", Content: "original message", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	q.State = "failed"
	q.SafeError = "The chat sandbox is unavailable."
	if err := store.FinishInboxRequest(ctx, q, nil); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/chat/conversations/" + room + "/requests/" + q.RequestID + "/retry"
	call := func(path, auth string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", path, strings.NewReader("{}"))
		if auth != "" {
			req.Header.Set("Authorization", "Bearer "+auth)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, req)
		return w
	}
	if w := call(path, ""); w.Code != 401 {
		t.Fatal("unauthenticated retry", w.Code)
	}
	if w := call(strings.Replace(path, room, db.GenerateUUID(), 1), token); w.Code != 404 {
		t.Fatal("cross-room retry", w.Code)
	}
	for i := 0; i < 3; i++ {
		w := call(path, token)
		if (i == 0 && w.Code != 202) || (i > 0 && w.Code != 200) {
			t.Fatal(w.Code, w.Body.String())
		}
		var body struct{ Request db.InboxRequest }
		json.Unmarshal(w.Body.Bytes(), &body)
		if body.Request.MessageID != q.MessageID || body.Request.RequestID != db.InboxRetryID(q.RequestID) {
			t.Fatal("correlation changed")
		}
	}
	all, _ := store.ListInboxRequests(ctx, room)
	if len(all) != 2 {
		t.Fatal("duplicate retry")
	}
	messages, _ := s.projectStudioRepo.ListMessagesByRoomID(ctx, room)
	if len(messages) != 1 {
		t.Fatal("duplicate user message")
	}
	childPath := strings.Replace(path, q.RequestID, db.InboxRetryID(q.RequestID), 1)
	if w := call(childPath, token); w.Code != 409 {
		t.Fatal("active retry duplicated", w.Code)
	}
}

func TestInboxRetryCompletesOnceWithOriginalContext(t *testing.T) {
	s, token, call, home := inboxFixture(t)
	owner, _ := s.authSvc.ValidateToken(context.Background(), token)
	w := call("GET", "/api/v1/chat/conversations/agent_lifecycle/messages", "")
	var conversation struct {
		RoomID string `json:"room_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &conversation); err != nil || conversation.RoomID == "" {
		t.Fatal(w.Body.String())
	}
	now := time.Now().UTC()
	q := db.InboxRequest{OwnerID: owner.ID, AgentID: "agent_lifecycle", RoomID: conversation.RoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), PayloadHash: "same", State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	store := s.projectStudioRepo.(db.InboxLifecycleRepository)
	if _, _, err := store.AcceptInboxRequest(context.Background(), q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: "ORIGINAL_INTENT_FIXTURE", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	q.State = "failed"
	q.SafeError = "The chat sandbox is unavailable."
	if err := store.FinishInboxRequest(context.Background(), q, nil); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/chat/conversations/" + q.RoomID + "/requests/" + q.RequestID + "/retry"
	for i := 0; i < 3; i++ {
		w = call("POST", path, "{}")
		if w.Code != 200 && w.Code != 202 {
			t.Fatal(w.Body.String())
		}
	}
	waitInboxState(t, s, q.RoomID, db.InboxRetryID(q.RequestID), "completed")
	messages, _ := s.projectStudioRepo.ListMessagesByRoomID(context.Background(), q.RoomID)
	if len(messages) != 2 {
		t.Fatalf("want original user and one reply, got %d", len(messages))
	}
	prompt, err := os.ReadFile(filepath.Join(home, "prompts"))
	if err != nil || strings.Count(string(prompt), "---TURN---") != 1 || !strings.Contains(string(prompt), "ORIGINAL_INTENT_FIXTURE") {
		t.Fatal("retry lost context or duplicated provider execution", err)
	}
}

func TestInboxUncertainRetryReevaluatesOldMessageOnce(t *testing.T) {
	s, token, call, home := inboxFixture(t)
	ctx := context.Background()
	owner, _ := s.authSvc.ValidateToken(ctx, token)
	conversation := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	now := time.Now().UTC()
	q := db.InboxRequest{OwnerID: owner.ID, AgentID: "agent_lifecycle", RoomID: conversation.RoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), PayloadHash: "retained-original", State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	store := s.projectStudioRepo.(db.InboxLifecycleRepository)
	if _, _, err := store.AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: "OLD_ORIGINAL_QUESTION", CreatedAt: now.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 25; i++ {
		if err := s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{ID: db.GenerateUUID(), RoomID: q.RoomID, AuthorType: "agent", Content: "Later context", CreatedAt: now.Add(time.Duration(i) * time.Millisecond)}); err != nil {
			t.Fatal(err)
		}
	}
	store.AppendInboxStep(ctx, q.RoomID, q.RequestID, "runtime_tool_started", "Uncertain previous action")
	q.State = "failed"
	q.SafeError = "The provider reached a usage limit after tool activity that cannot be safely repeated."
	if err := store.FinishInboxRequest(ctx, q, nil); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/chat/conversations/" + q.RoomID + "/requests/" + q.RequestID + "/retry"
	for i := 0; i < 3; i++ {
		w := call("POST", path, "{}")
		if w.Code != 200 && w.Code != 202 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	waitInboxState(t, s, q.RoomID, db.InboxRetryID(q.RequestID), "completed")
	rows, _ := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, q.RoomID)
	if len(rows) != 2 || !db.InboxRetryNeedsReconciliation(rows[1]) {
		t.Fatal("reconciliation was duplicated or lost", len(rows))
	}
	prompt, err := os.ReadFile(filepath.Join(home, "prompts"))
	if err != nil || strings.Count(string(prompt), "---TURN---") != 1 || !strings.Contains(string(prompt), "OLD_ORIGINAL_QUESTION") || !strings.Contains(string(prompt), inboxRetryReconcileInstruction) {
		t.Fatal("fresh original-context evaluation missing", err)
	}
}

func TestInboxUncertainRetryPreservesImageAttachmentAndContext(t *testing.T) {
	s, token, call, home := inboxFixture(t)
	ctx := context.Background()
	owner, _ := s.authSvc.ValidateToken(ctx, token)

	// Upload a real PNG image attachment for the owner.
	var imgBuf bytes.Buffer
	if err := png.Encode(&imgBuf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", "screenshot.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(imgBuf.Bytes()); err != nil {
		t.Fatal(err)
	}
	mw.Close()
	uploadReq := httptest.NewRequest("POST", "/api/v1/chat/attachments", &body)
	uploadReq.Header.Set("Content-Type", mw.FormDataContentType())
	uploadReq.Header.Set("Authorization", "Bearer "+token)
	uploadRec := httptest.NewRecorder()
	s.Router().ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != 201 {
		t.Fatalf("attachment upload failed: %d %s", uploadRec.Code, uploadRec.Body.String())
	}
	var att conversationAttachment
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &att); err != nil || att.ID == "" {
		t.Fatalf("unexpected attachment response: %v %s", err, uploadRec.Body.String())
	}

	conversation := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	now := time.Now().UTC()
	q := db.InboxRequest{
		OwnerID:     owner.ID,
		AgentID:     "agent_lifecycle",
		RoomID:      conversation.RoomID,
		RequestID:   db.GenerateUUID(),
		MessageID:   db.GenerateUUID(),
		ReplyID:     db.GenerateUUID(),
		PayloadHash: "retained-image-retry",
		State:       "queued",
		CreatedAt:   now,
		Deadline:    now.Add(time.Hour),
	}
	store := s.projectStudioRepo.(db.InboxLifecycleRepository)
	origMsg := &db.ProjectMessage{
		ID:         q.MessageID,
		RoomID:     q.RoomID,
		AuthorType: "user",
		Content:    encodeAttachmentMessage("Ini harusnya done ga sih kok iconnya masih mata ungu", []string{att.ID}),
		CreatedAt:  now.Add(-time.Hour),
	}
	if _, _, err := store.AcceptInboxRequest(ctx, q, origMsg); err != nil {
		t.Fatal(err)
	}
	// Add >20 newer user messages to verify retry still retains the target message's attachment.
	for i := 0; i < 22; i++ {
		msgID := db.GenerateUUID()
		laterReq := db.InboxRequest{
			OwnerID:     owner.ID,
			AgentID:     "agent_lifecycle",
			RoomID:      q.RoomID,
			RequestID:   db.GenerateUUID(),
			MessageID:   msgID,
			ReplyID:     db.GenerateUUID(),
			PayloadHash: db.GenerateUUID(),
			State:       "completed",
			CreatedAt:   now.Add(time.Duration(i+1) * time.Millisecond),
			Deadline:    now.Add(time.Hour),
		}
		if _, _, err := store.AcceptInboxRequest(ctx, laterReq, &db.ProjectMessage{
			ID:         msgID,
			RoomID:     q.RoomID,
			AuthorType: "user",
			Content:    "Later follow-up",
			CreatedAt:  now.Add(time.Duration(i+1) * time.Millisecond),
		}); err != nil {
			t.Fatal(err)
		}
	}
	store.AppendInboxStep(ctx, q.RoomID, q.RequestID, "runtime_tool_started", "Uncertain previous action")
	q.State = "failed"
	q.SafeError = "The provider reached a usage limit after tool activity that cannot be safely repeated."
	if err := store.FinishInboxRequest(ctx, q, nil); err != nil {
		t.Fatal(err)
	}

	path := "/api/v1/chat/conversations/" + q.RoomID + "/requests/" + q.RequestID + "/retry"
	w := call("POST", path, "{}")
	if w.Code != 200 && w.Code != 202 {
		t.Fatalf("retry failed: %d %s", w.Code, w.Body.String())
	}
	waitInboxState(t, s, q.RoomID, db.InboxRetryID(q.RequestID), "completed")

	rows, _ := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, q.RoomID)
	var retryReq *db.InboxRequest
	for i := range rows {
		if rows[i].RequestID == db.InboxRetryID(q.RequestID) {
			retryReq = &rows[i]
			break
		}
	}
	if retryReq == nil || !db.InboxRetryNeedsReconciliation(*retryReq) {
		t.Fatalf("expected reconciliation retry request, got %+v", retryReq)
	}
	if gotPaths := s.inboxAttachmentPaths(ctx, *retryReq); len(gotPaths) == 0 {
		t.Fatalf("expected inboxAttachmentPaths to retain attachment for retried message")
	}

	prompt, err := os.ReadFile(filepath.Join(home, "prompts"))
	if err != nil {
		t.Fatal(err)
	}
	p := string(prompt)
	if !strings.Contains(p, "Ini harusnya done ga sih kok iconnya masih mata ungu") ||
		!strings.Contains(p, att.ID) ||
		!strings.Contains(p, "You may ONLY inspect the supplied user attachments using read_file") ||
		strings.Contains(p, "Do not call inspection or execution tools in chat.") {
		t.Fatalf("reconcile retry prompt missing attachment context or tool guidance: %s", p)
	}
}
