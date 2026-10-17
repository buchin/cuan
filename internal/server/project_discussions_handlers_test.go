package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/db"
)

func newTestServerWithMemoryDB(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{
		AuthSecret:  "test-secret-32-chars-long-security",
		AppMode:     "marketing",
		AppVersion:  "0.7.2-test",
		BaseDomain:  "cuan.test",
	}
	s := NewServer(cfg, nil)
	return s
}

func TestProjectRooms_CRUD(t *testing.T) {
	s := newTestServerWithMemoryDB(t)
	projectID := "proj-" + db.GenerateUUID()[:8]

	// 1. Create Room (Success)
	createBody := map[string]interface{}{
		"title": "Sprint 7 Planning Discussion",
		"topic": "Architecture & Task Breakdown for Project Studio",
	}
	bodyBytes, _ := json.Marshal(createBody)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/projects/%s/rooms", projectID), bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var createResp struct {
		Room db.ProjectRoomDTO `json:"room"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("unmarshal create room response: %v", err)
	}
	if createResp.Room.ID == "" || createResp.Room.Title != "Sprint 7 Planning Discussion" {
		t.Fatalf("unexpected room created: %+v", createResp.Room)
	}
	roomID := createResp.Room.ID

	// 2. Create Room with empty title (Validation Error)
	invalidBody, _ := json.Marshal(map[string]interface{}{"title": "   ", "topic": "test"})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/projects/%s/rooms", projectID), bytes.NewReader(invalidBody))
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty title, got %d", rec.Code)
	}

	// 3. List Rooms for Project
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/projects/%s/rooms", projectID), nil)
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var listResp struct {
		Rooms []db.ProjectRoomDTO `json:"rooms"`
		Count int                 `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list rooms: %v", err)
	}
	if listResp.Count != 1 || listResp.Rooms[0].ID != roomID {
		t.Fatalf("expected 1 room with id %s, got %d", roomID, listResp.Count)
	}

	// 4. Get Room by ID (/api/rooms/{id} and /api/projects/rooms/{id})
	for _, path := range []string{
		fmt.Sprintf("/api/rooms/%s", roomID),
		fmt.Sprintf("/api/projects/rooms/%s", roomID),
	} {
		req = httptest.NewRequest(http.MethodGet, path, nil)
		rec = httptest.NewRecorder()
		s.Router().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", path, rec.Code)
		}
		var getResp struct {
			Room db.ProjectRoomDTO `json:"room"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &getResp)
		if getResp.Room.ID != roomID {
			t.Fatalf("expected room id %s, got %s", roomID, getResp.Room.ID)
		}
	}

	// 5. Update Room
	updateBody, _ := json.Marshal(map[string]interface{}{
		"title": "Sprint 7 Execution & Real-Time Sync",
		"topic": "Updated Topic",
	})
	req = httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/rooms/%s", roomID), bytes.NewReader(updateBody))
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on update, got %d: %s", rec.Code, rec.Body.String())
	}
	var updateResp struct {
		Room db.ProjectRoomDTO `json:"room"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &updateResp)
	if updateResp.Room.Title != "Sprint 7 Execution & Real-Time Sync" {
		t.Fatalf("expected updated title, got %s", updateResp.Room.Title)
	}

	// 6. Delete Room
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/rooms/%s", roomID), nil)
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete, got %d", rec.Code)
	}

	// Verify room is deleted
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/rooms/%s", roomID), nil)
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for deleted room, got %d", rec.Code)
	}
}

func TestProjectMessages_CRUD(t *testing.T) {
	s := newTestServerWithMemoryDB(t)
	projectID := "proj-" + db.GenerateUUID()[:8]

	// Create room first
	room := &db.ProjectRoom{
		ProjectID: projectID,
		Title:     "Dev Chat Room",
		Topic:     "General development",
	}
	if err := s.ProjectStudioRepository().CreateRoom(context.Background(), room); err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	// 1. Post Message (Success)
	msgPayload := map[string]interface{}{
		"author_type": "user",
		"author_id":   "usr-buchin",
		"author_name": "Buchin",
		"content":     "Sprint 7 is officially running. Let's build the SSE engine!",
	}
	bodyBytes, _ := json.Marshal(msgPayload)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/rooms/%s/messages", room.ID), bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 created, got %d: %s", rec.Code, rec.Body.String())
	}
	var msgResp struct {
		Message db.ProjectMessageDTO `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &msgResp); err != nil {
		t.Fatalf("unmarshal message response: %v", err)
	}
	if msgResp.Message.ID == "" || msgResp.Message.Content != msgPayload["content"] {
		t.Fatalf("unexpected message: %+v", msgResp.Message)
	}
	firstMsgID := msgResp.Message.ID

	// 2. Post Threaded Reply via alias /api/projects/rooms/{id}/messages
	replyPayload := map[string]interface{}{
		"author_type": "agent",
		"author_id":   "agent-buchin",
		"author_name": "Scrum Master",
		"content":     "Acknowledged. Event streaming and REST handlers are in progress.",
		"reply_to_id": firstMsgID,
	}
	replyBytes, _ := json.Marshal(replyPayload)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/projects/rooms/%s/messages", room.ID), bytes.NewReader(replyBytes))
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 created for reply, got %d: %s", rec.Code, rec.Body.String())
	}
	var replyResp struct {
		Message db.ProjectMessageDTO `json:"message"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &replyResp)
	if replyResp.Message.ReplyToID == nil || *replyResp.Message.ReplyToID != firstMsgID {
		t.Fatalf("expected reply_to_id %s, got %v", firstMsgID, replyResp.Message.ReplyToID)
	}
	secondMsgID := replyResp.Message.ID

	// 3. Validation errors
	// Empty content
	invalidReq, _ := json.Marshal(map[string]interface{}{"content": "  "})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/rooms/%s/messages", room.ID), bytes.NewReader(invalidReq))
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty content, got %d", rec.Code)
	}

	// Invalid author type
	invalidAuthor, _ := json.Marshal(map[string]interface{}{"content": "Valid content", "author_type": "alien"})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/rooms/%s/messages", room.ID), bytes.NewReader(invalidAuthor))
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid author_type, got %d", rec.Code)
	}

	// 4. List Messages in Room
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/rooms/%s/messages", room.ID), nil)
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var listResp struct {
		Messages []db.ProjectMessageDTO `json:"messages"`
		Count    int                    `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list messages: %v", err)
	}
	if listResp.Count != 2 {
		t.Fatalf("expected 2 messages, got %d", listResp.Count)
	}

	// 5. Delete Message
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/messages/%s", secondMsgID), nil)
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete message, got %d", rec.Code)
	}

	// Verify remaining count is 1
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/rooms/%s/messages", room.ID), nil)
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	_ = json.Unmarshal(rec.Body.Bytes(), &listResp)
	if listResp.Count != 1 {
		t.Fatalf("expected 1 message remaining, got %d", listResp.Count)
	}
}

func TestProjectDiscussions_RealTimeSSE_StreamingAndHeartbeat(t *testing.T) {
	s := newTestServerWithMemoryDB(t)
	// Configure fast heartbeat for test execution speed (50ms instead of 15s)
	s.RoomEventBroker().SetHeartbeatInterval(50 * time.Millisecond)

	projectID := "proj-" + db.GenerateUUID()[:8]
	room := &db.ProjectRoom{
		ProjectID: projectID,
		Title:     "SSE Real-Time Room",
		Topic:     "Testing SSE streaming engine",
	}
	if err := s.ProjectStudioRepository().CreateRoom(context.Background(), room); err != nil {
		t.Fatalf("create room: %v", err)
	}

	ts := httptest.NewServer(s.Router())
	defer ts.Close()

	// Connect SSE client with context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/rooms/%s/events", ts.URL, room.ID), nil)
	if err != nil {
		t.Fatalf("create sse request: %v", err)
	}

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("sse request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for sse stream, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("expected text/event-stream content type, got %s", resp.Header.Get("Content-Type"))
	}

	reader := bufio.NewReader(resp.Body)

	// Helper to read SSE events
	readEvent := func(timeout time.Duration) (string, string, error) {
		type result struct {
			evType string
			data   string
			err    error
		}
		ch := make(chan result, 1)

		go func() {
			var currentEvent, currentData string
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					ch <- result{err: err}
					return
				}
				line = strings.TrimRight(line, "\r\n")
				if strings.HasPrefix(line, "event:") {
					currentEvent = strings.TrimSpace(line[6:])
				} else if strings.HasPrefix(line, "data:") {
					currentData = strings.TrimSpace(line[5:])
				} else if strings.HasPrefix(line, ":") {
					// Comment / heartbeat
					currentEvent = "comment"
					currentData = strings.TrimSpace(line[1:])
				} else if line == "" && (currentEvent != "" || currentData != "") {
					ch <- result{evType: currentEvent, data: currentData}
					return
				}
			}
		}()

		select {
		case res := <-ch:
			return res.evType, res.data, res.err
		case <-time.After(timeout):
			return "", "", fmt.Errorf("readEvent timed out after %v", timeout)
		}
	}

	// 1. Initial event must be 'connected'
	evType, data, err := readEvent(2 * time.Second)
	if err != nil {
		t.Fatalf("failed reading initial connected event: %v", err)
	}
	if evType != "connected" {
		t.Fatalf("expected initial event 'connected', got '%s'", evType)
	}
	if !strings.Contains(data, room.ID) {
		t.Fatalf("expected connected event data to contain room id, got %s", data)
	}

	// Verify subscriber count is 1
	if count := s.RoomEventBroker().SubscriberCount(room.ID); count != 1 {
		t.Fatalf("expected 1 active subscriber, got %d", count)
	}

	// 2. Broadcast a new message via POST /api/rooms/{id}/messages
	postMsgPayload := map[string]interface{}{
		"author_type": "user",
		"author_name": "Buchin",
		"content":     "Hello over real-time SSE stream!",
	}
	pBytes, _ := json.Marshal(postMsgPayload)
	postReq, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/rooms/%s/messages", ts.URL, room.ID), bytes.NewReader(pBytes))
	postReq.Header.Set("Content-Type", "application/json")
	postResp, postErr := ts.Client().Do(postReq)
	if postErr != nil || postResp.StatusCode != http.StatusCreated {
		t.Fatalf("failed posting message: %v, status: %d", postErr, postResp.StatusCode)
	}
	_ = postResp.Body.Close()

	// 3. Receive message.created event on SSE stream
	evType, data, err = readEvent(2 * time.Second)
	if err != nil {
		t.Fatalf("failed reading broadcast event: %v", err)
	}
	if evType != "message.created" {
		t.Fatalf("expected event 'message.created', got '%s'", evType)
	}
	if !strings.Contains(data, "Hello over real-time SSE stream!") {
		t.Fatalf("expected data to contain message content, got %s", data)
	}

	// 4. Verify heartbeat arrival (either ping event or heartbeat comment)
	evType, data, err = readEvent(2 * time.Second)
	if err != nil {
		t.Fatalf("failed reading heartbeat: %v", err)
	}
	if evType != "ping" && evType != "comment" {
		t.Fatalf("expected heartbeat ping or comment, got '%s': %s", evType, data)
	}

	// 5. Cancel context and verify graceful disconnect cleanup
	cancel()
	time.Sleep(100 * time.Millisecond)

	if count := s.RoomEventBroker().SubscriberCount(room.ID); count != 0 {
		t.Fatalf("expected subscriber count to be 0 after disconnect, got %d", count)
	}
}

func TestProjectDiscussions_Distiller(t *testing.T) {
	s := newTestServerWithMemoryDB(t)
	projectID := "proj-" + db.GenerateUUID()[:8]

	room := &db.ProjectRoom{
		ProjectID: projectID,
		Title:     "Sprint 7 Distillation Discussion",
		Topic:     "Requirements gathering for REST API & SSE",
	}
	_ = s.ProjectStudioRepository().CreateRoom(context.Background(), room)

	// Post discussion messages with checklist items
	messages := []string{
		"Tim, berikut rencana implementasi S07.2:\n- [ ] Implement REST endpoints di project_discussions_handlers.go\n- [ ] Buat SSE streaming engine dengan heartbeat 15s\n- [x] Selesaikan database migration 000012",
		"DoD tambahan:\nDoD: Unit test targeted 100% PASS\n- [ ] Berkas audit di docs/sprints/evidence/S7/",
	}

	for _, content := range messages {
		msg := &db.ProjectMessage{
			RoomID:     room.ID,
			AuthorType: "user",
			AuthorName: "Buchin",
			Content:    content,
		}
		_ = s.ProjectStudioRepository().CreateMessage(context.Background(), msg)
	}

	// Call Distiller endpoint: POST /api/rooms/{id}/distill
	distillPayload := map[string]interface{}{
		"title": "S07.2 Task Generated from Chat",
		"tags":  []string{"studio", "distiller-verified"},
	}
	pBytes, _ := json.Marshal(distillPayload)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/rooms/%s/distill", room.ID), bytes.NewReader(pBytes))
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Task                  db.ProjectTaskDTO `json:"task"`
		DistilledFromMessages int               `json:"distilled_from_messages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal distill response: %v", err)
	}

	if resp.Task.Title != "S07.2 Task Generated from Chat" {
		t.Fatalf("expected title 'S07.2 Task Generated from Chat', got '%s'", resp.Task.Title)
	}
	if len(resp.Task.DoDChecklist) < 3 {
		t.Fatalf("expected at least 3 DoD items extracted, got %d: %+v", len(resp.Task.DoDChecklist), resp.Task.DoDChecklist)
	}

	// Verify task exists in repository
	storedTask, err := s.ProjectStudioRepository().GetTaskByID(context.Background(), resp.Task.ID)
	if err != nil || storedTask == nil {
		t.Fatalf("expected stored task, got err: %v", err)
	}
}

func TestProjectDiscussions_V1AliasEndpoints(t *testing.T) {
	s := newTestServerWithMemoryDB(t)
	projectID := "proj-" + db.GenerateUUID()[:8]

	// Create room via /api/v1/projects/{id}/rooms
	body, _ := json.Marshal(map[string]interface{}{
		"title": "V1 Alias Room",
		"topic": "Testing v1 prefix",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/rooms", projectID), bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 on /api/v1/projects/{id}/rooms, got %d", rec.Code)
	}

	var createResp struct {
		Room db.ProjectRoomDTO `json:"room"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &createResp)
	roomID := createResp.Room.ID

	// Get room via /api/v1/rooms/{id}
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/rooms/%s", roomID), nil)
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/v1/rooms/{id}, got %d", rec.Code)
	}

	// Post message via /api/v1/rooms/{id}/messages
	msgBody, _ := json.Marshal(map[string]interface{}{
		"author_type": "user",
		"author_name": "Buchin",
		"content":     "Hello v1!",
	})
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/rooms/%s/messages", roomID), bytes.NewReader(msgBody))
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 on /api/v1/rooms/{id}/messages, got %d", rec.Code)
	}

	// List messages via /api/v1/rooms/{id}/messages
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/rooms/%s/messages", roomID), nil)
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/v1/rooms/{id}/messages, got %d", rec.Code)
	}
}
