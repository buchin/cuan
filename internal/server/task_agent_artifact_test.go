package server

import (
	"context"
	"crypto/sha256"
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

func TestTaskAgentArtifactDownloadAndGuardrails(t *testing.T) {
	s, token, _, _ := inboxFixture(t)
	ctx := context.Background()
	owner, err := s.authSvc.ValidateToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}

	room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	now := time.Now().UTC()
	q := db.InboxRequest{
		OwnerID:     owner.ID,
		AgentID:     "agent_lifecycle",
		RoomID:      room.RoomID,
		RequestID:   db.GenerateUUID(),
		MessageID:   db.GenerateUUID(),
		ReplyID:     db.GenerateUUID(),
		State:       "completed",
		CreatedAt:   now,
		Deadline:    now.Add(time.Hour),
		PayloadHash: db.GenerateUUID(),
	}
	_, _, err = s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, &db.ProjectMessage{
		ID:         q.MessageID,
		RoomID:     q.RoomID,
		AuthorType: "user",
		Content:    "Deliver verification evidence",
		CreatedAt:  now,
	})
	if err != nil {
		t.Fatal(err)
	}

	saved, err := s.taskSvc.CreateTaskFromMessage(ctx, owner.ID, q.MessageID, task.CreateTaskRequest{
		SourceRoomID:  q.RoomID,
		ProjectID:     "default",
		Title:         "Task One",
		Instructions:  "Deliver test files",
		AssignedAgent: q.AgentID,
	})
	if err != nil {
		t.Fatal(err)
	}

	a, err := s.taskSvc.CreateAttempt(ctx, saved.ID, owner.ID, task.CreateAttemptRequest{
		RunnerID:  "local_terminal",
		AgentName: q.AgentID,
	})
	if err != nil {
		t.Fatal(err)
	}

	doing := task.TaskStatusDoing
	s.taskSvc.UpdateTask(ctx, saved.ID, owner.ID, task.UpdateTaskRequest{Status: &doing})
	s.taskSvc.UpdateAttemptStatus(ctx, a.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning})

	workspace := t.TempDir()
	binaryData := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x10\x00\x00\x00\x10\x08\x06\x00\x00\x00\x1f\xf3\xffa" + strings.Repeat("\x00\xff\xee\xdd", 100))
	imgName := "screenshot.png"
	if err := os.WriteFile(filepath.Join(workspace, imgName), binaryData, 0600); err != nil {
		t.Fatal(err)
	}

	textData := []byte("# Complete Report\nDetailed verification findings\n" + strings.Repeat("content line\n", 50))
	mdName := "full_report.md"
	if err := os.WriteFile(filepath.Join(workspace, mdName), textData, 0600); err != nil {
		t.Fatal(err)
	}

	_, _, capability, cleanup, err := s.issueTaskAgent(context.WithValue(ctx, taskAgentWorkspaceKey{}, workspace), saved.ID, a.ID, q.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// Deliver both files in attempt 1
	reportReq := httptest.NewRequest("POST", "/api/v1/task-agent/"+saved.ID, strings.NewReader(`{"state":"done","summary":"Deliverables ready","complete_note":"Verified files","artifacts":["screenshot.png","full_report.md"]}`))
	reportReq.Header.Set("Authorization", "Bearer "+capability)
	reportW := httptest.NewRecorder()
	s.Router().ServeHTTP(reportW, reportReq)
	if reportW.Code != 200 {
		t.Fatalf("failed to deliver artifacts: %d %s", reportW.Code, reportW.Body.String())
	}

	// Read task to get delivered attachment IDs
	readReq := httptest.NewRequest("GET", "/api/v1/task-agent/"+saved.ID, nil)
	readReq.Header.Set("Authorization", "Bearer "+capability)
	readW := httptest.NewRecorder()
	s.Router().ServeHTTP(readW, readReq)
	var taskView struct {
		DeliveredArtifacts []map[string]string `json:"delivered_artifacts"`
	}
	if err := json.Unmarshal(readW.Body.Bytes(), &taskView); err != nil || len(taskView.DeliveredArtifacts) != 2 {
		t.Fatalf("expected 2 delivered artifacts, got %d (code %d)", len(taskView.DeliveredArtifacts), readW.Code)
	}

	var imgAttID, mdAttID string
	for _, art := range taskView.DeliveredArtifacts {
		if art["name"] == imgName {
			imgAttID = art["attachment_id"]
		} else if art["name"] == mdName {
			mdAttID = art["attachment_id"]
		}
	}
	if imgAttID == "" || mdAttID == "" {
		t.Fatalf("missing attachment IDs: img=%q md=%q", imgAttID, mdAttID)
	}

	// 1. Test downloading complete binary file (PNG image)
	t.Run("DownloadBinaryImage", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/task-agent/"+saved.ID+"/artifacts/"+imgAttID, nil)
		req.Header.Set("Authorization", "Bearer "+capability)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if w.Header().Get("Content-Type") != "image/png" {
			t.Errorf("expected Content-Type image/png, got %s", w.Header().Get("Content-Type"))
		}
		if !strings.Contains(w.Header().Get("Content-Disposition"), imgName) {
			t.Errorf("expected Content-Disposition to contain %s, got %s", imgName, w.Header().Get("Content-Disposition"))
		}
		if string(w.Body.Bytes()) != string(binaryData) {
			t.Errorf("downloaded binary content does not match original bytes")
		}
	})

	// 2. Test downloading complete text file (Markdown report)
	t.Run("DownloadTextReport", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/task-agent/"+saved.ID+"/artifacts/"+mdAttID, nil)
		req.Header.Set("Authorization", "Bearer "+capability)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if !strings.HasPrefix(w.Header().Get("Content-Type"), "text/markdown") {
			t.Errorf("expected Content-Type text/markdown, got %s", w.Header().Get("Content-Type"))
		}
		if string(w.Body.Bytes()) != string(textData) {
			t.Errorf("downloaded text content does not match original bytes")
		}
	})

	// 3. Test alias endpoint /attachments/{id}
	t.Run("AttachmentAliasEndpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/task-agent/"+saved.ID+"/attachments/"+mdAttID, nil)
		req.Header.Set("Authorization", "Bearer "+capability)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, req)
		if w.Code != 200 || string(w.Body.Bytes()) != string(textData) {
			t.Fatalf("alias endpoint failed: %d", w.Code)
		}
	})

	// 4. Test metadata query
	t.Run("MetadataQuery", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/task-agent/"+saved.ID+"/artifacts/"+imgAttID+"?metadata=true", nil)
		req.Header.Set("Authorization", "Bearer "+capability)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200 for metadata, got %d: %s", w.Code, w.Body.String())
		}
		var meta map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &meta); err != nil {
			t.Fatal(err)
		}
		if meta["attachment_id"] != imgAttID || meta["name"] != imgName || meta["media_type"] != "image/png" {
			t.Fatalf("unexpected metadata content: %+v", meta)
		}
	})

	// 5. Test access boundary: Unauthenticated / missing token
	t.Run("RejectUnauthenticated", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/task-agent/"+saved.ID+"/artifacts/"+imgAttID, nil)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, req)
		if w.Code != 401 {
			t.Errorf("expected 401 for unauthenticated request, got %d", w.Code)
		}
	})

	// 6. Test access boundary: Expired grant
	t.Run("RejectExpiredGrant", func(t *testing.T) {
		digest := sha256.Sum256([]byte(capability))
		if val, ok := s.taskAgentGrants.Load(digest); ok {
			g := val.(taskAgentGrant)
			g.Expires = time.Now().Add(-1 * time.Hour)
			s.taskAgentGrants.Store(digest, g)
		}
		req := httptest.NewRequest("GET", "/api/v1/task-agent/"+saved.ID+"/artifacts/"+imgAttID, nil)
		req.Header.Set("Authorization", "Bearer "+capability)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, req)
		if w.Code != 401 {
			t.Errorf("expected 401 for expired grant, got %d", w.Code)
		}
		// restore grant expiration
		if val, ok := s.taskAgentGrants.Load(digest); ok {
			g := val.(taskAgentGrant)
			g.Expires = time.Now().Add(1 * time.Hour)
			s.taskAgentGrants.Store(digest, g)
		}
	})

	// 7. Test access boundary: Wrong task ID
	t.Run("RejectWrongTaskID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/task-agent/foreign-task-id/artifacts/"+imgAttID, nil)
		req.Header.Set("Authorization", "Bearer "+capability)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, req)
		if w.Code != 403 {
			t.Errorf("expected 403 for wrong task ID, got %d", w.Code)
		}
	})

	// 8. Test access boundary: Path traversal and invalid attachment ID
	t.Run("RejectPathTraversalAttachmentID", func(t *testing.T) {
		for _, invalidID := range []string{"..%2Fescape", "escape/part", "id%20with%20spaces", "invalid!id"} {
			req := httptest.NewRequest("GET", "/api/v1/task-agent/"+saved.ID+"/artifacts/"+invalidID, nil)
			req.Header.Set("Authorization", "Bearer "+capability)
			w := httptest.NewRecorder()
			s.Router().ServeHTTP(w, req)
			if w.Code != 400 && w.Code != 404 {
				t.Errorf("expected 400/404 for invalid attachment ID %q, got %d: %s", invalidID, w.Code, w.Body.String())
			}
		}
	})

	// 9. Test access boundary: Inactive / ended attempt returns 409
	t.Run("RejectEndedAttempt", func(t *testing.T) {
		// Mark attempt 1 as succeeded
		s.taskSvc.UpdateAttemptStatus(ctx, a.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusSucceeded})

		req := httptest.NewRequest("GET", "/api/v1/task-agent/"+saved.ID+"/artifacts/"+imgAttID, nil)
		req.Header.Set("Authorization", "Bearer "+capability)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, req)
		if w.Code != 409 {
			t.Errorf("expected 409 for inactive attempt, got %d: %s", w.Code, w.Body.String())
		}
	})

	// 10. Test verifier in later attempt can download artifacts delivered by earlier attempt
	var capLater string
	var aLater *task.TaskAttempt
	t.Run("LaterAttemptVerifierCanDownloadPriorAttemptArtifact", func(t *testing.T) {
		aLater, err = s.taskSvc.CreateAttempt(ctx, saved.ID, owner.ID, task.CreateAttemptRequest{
			RunnerID:  "local_terminal",
			AgentName: "verifier",
		})
		if err != nil {
			t.Fatalf("create verifier attempt: %v", err)
		}
		s.taskSvc.UpdateAttemptStatus(ctx, aLater.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning})

		workspaceLater := t.TempDir()
		var cleanLater func()
		_, _, capLater, cleanLater, err = s.issueTaskAgent(context.WithValue(ctx, taskAgentWorkspaceKey{}, workspaceLater), saved.ID, aLater.ID, "verifier")
		if err != nil {
			t.Fatal(err)
		}
		defer cleanLater()

		req := httptest.NewRequest("GET", "/api/v1/task-agent/"+saved.ID+"/artifacts/"+mdAttID, nil)
		req.Header.Set("Authorization", "Bearer "+capLater)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected later verifier to download prior artifact, got %d: %s", w.Code, w.Body.String())
		}
		if string(w.Body.Bytes()) != string(textData) {
			t.Errorf("verifier downloaded content mismatch")
		}
	})

	// 11. Test access boundary: Attachment belonging to a different task
	t.Run("RejectOtherTaskAttachment", func(t *testing.T) {
		q2 := db.InboxRequest{
			OwnerID:     owner.ID,
			AgentID:     "agent_lifecycle",
			RoomID:      room.RoomID,
			RequestID:   db.GenerateUUID(),
			MessageID:   db.GenerateUUID(),
			ReplyID:     db.GenerateUUID(),
			State:       "completed",
			CreatedAt:   now,
			Deadline:    now.Add(time.Hour),
			PayloadHash: db.GenerateUUID(),
		}
		_, _, err = s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q2, &db.ProjectMessage{
			ID:         q2.MessageID,
			RoomID:     q2.RoomID,
			AuthorType: "user",
			Content:    "Create other task",
			CreatedAt:  now,
		})
		if err != nil {
			t.Fatal(err)
		}

		saved2, err := s.taskSvc.CreateTaskFromMessage(ctx, owner.ID, q2.MessageID, task.CreateTaskRequest{
			SourceRoomID:  q2.RoomID,
			ProjectID:     "default",
			Title:         "Task Two",
			Instructions:  "Other task instructions",
			AssignedAgent: q2.AgentID,
		})
		if err != nil {
			t.Fatal(err)
		}
		a2, err := s.taskSvc.CreateAttempt(ctx, saved2.ID, owner.ID, task.CreateAttemptRequest{
			RunnerID:  "local_terminal",
			AgentName: q2.AgentID,
		})
		if err != nil {
			t.Fatal(err)
		}
		s.taskSvc.UpdateTask(ctx, saved2.ID, owner.ID, task.UpdateTaskRequest{Status: &doing})
		s.taskSvc.UpdateAttemptStatus(ctx, a2.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning})

		workspace2 := t.TempDir()
		otherFile := "other_task_secret.txt"
		os.WriteFile(filepath.Join(workspace2, otherFile), []byte("SECRET OF TASK TWO"), 0600)
		_, _, cap2, clean2, err := s.issueTaskAgent(context.WithValue(ctx, taskAgentWorkspaceKey{}, workspace2), saved2.ID, a2.ID, q2.AgentID)
		if err != nil {
			t.Fatal(err)
		}
		defer clean2()

		rep2 := httptest.NewRequest("POST", "/api/v1/task-agent/"+saved2.ID, strings.NewReader(`{"state":"done","summary":"Done","complete_note":"Note","artifacts":["other_task_secret.txt"]}`))
		rep2.Header.Set("Authorization", "Bearer "+cap2)
		rw2 := httptest.NewRecorder()
		s.Router().ServeHTTP(rw2, rep2)
		if rw2.Code != 200 {
			t.Fatalf("failed delivery for task 2: %d", rw2.Code)
		}

		viewReq2 := httptest.NewRequest("GET", "/api/v1/task-agent/"+saved2.ID, nil)
		viewReq2.Header.Set("Authorization", "Bearer "+cap2)
		viewW2 := httptest.NewRecorder()
		s.Router().ServeHTTP(viewW2, viewReq2)
		var view2 struct {
			DeliveredArtifacts []map[string]string `json:"delivered_artifacts"`
		}
		json.Unmarshal(viewW2.Body.Bytes(), &view2)
		if len(view2.DeliveredArtifacts) == 0 {
			t.Fatal("task 2 delivered artifacts missing")
		}
		taskTwoAttID := view2.DeliveredArtifacts[0]["attachment_id"]

		// Now attempt to download Task Two's attachment using Task One's active verifier capability:
		_, _, cap1, clean1, err := s.issueTaskAgent(ctx, saved.ID, aLater.ID, "verifier")
		if err != nil {
			t.Fatal(err)
		}
		defer clean1()

		badReq := httptest.NewRequest("GET", "/api/v1/task-agent/"+saved.ID+"/artifacts/"+taskTwoAttID, nil)
		badReq.Header.Set("Authorization", "Bearer "+cap1)
		badW := httptest.NewRecorder()
		s.Router().ServeHTTP(badW, badReq)
		if badW.Code != 404 {
			t.Errorf("expected 404 when requesting another task's attachment, got %d: %s", badW.Code, badW.Body.String())
		}
	})

	// 12. Test rejecting Range requests (no partial content)
	t.Run("RejectRangeRequests", func(t *testing.T) {
		_, _, capRange, cleanRange, err := s.issueTaskAgent(ctx, saved.ID, aLater.ID, "verifier")
		if err != nil {
			t.Fatal(err)
		}
		defer cleanRange()

		req := httptest.NewRequest("GET", "/api/v1/task-agent/"+saved.ID+"/artifacts/"+imgAttID, nil)
		req.Header.Set("Authorization", "Bearer "+capRange)
		req.Header.Set("Range", "bytes=0-10")
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, req)
		if w.Code != 400 {
			t.Errorf("expected 400 when requesting range, got %d: %s", w.Code, w.Body.String())
		}
	})
}
