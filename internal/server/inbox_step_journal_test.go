package server

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/buchin/cuan/internal/db"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestInboxStepJournalAPIAuthorizationPagingAndPrivateDetails(t *testing.T) {
	s, token := setupTestChatServer(t)
	s.inboxStopping.Store(true)
	owner, _ := s.authSvc.ValidateToken(context.Background(), token)
	ctx := context.Background()
	now := time.Now()
	room := db.GenerateUUID()
	if err := s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: room, ProjectID: "default", Title: "Trace fixture", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	store := s.projectStudioRepo.(db.InboxLifecycleRepository)
	q := db.InboxRequest{RoomID: room, RequestID: db.GenerateUUID(), OwnerID: owner.ID, MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	if _, _, err := store.AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: room}); err != nil {
		t.Fatal(err)
	}
	store.ClaimInboxRequest(ctx, room, q.RequestID)
	for i := 0; i < 120; i++ {
		store.AppendInboxStep(ctx, room, q.RequestID, "context", "stage")
	}
	store.AppendInboxStep(ctx, room, q.RequestID, "runtime_tool_finished", "Tool ended.\n\nInput: .env credentials")
	path := "/api/v1/chat/conversations/" + room + "/requests/" + q.RequestID + "/steps"
	call := func(endpoint, auth string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", endpoint, nil)
		if auth != "" {
			req.Header.Set("Authorization", "Bearer "+auth)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, req)
		return w
	}
	if w := call(path, ""); w.Code != 401 {
		t.Fatal("unauthenticated trace", w.Code)
	}
	if w := call(strings.Replace(path, room, db.GenerateUUID(), 1), token); w.Code != 404 {
		t.Fatal("cross-room trace", w.Code)
	}
	for _, query := range []string{"?after_seq=-1", "?limit=201", "?limit=0", "?after_seq=bad"} {
		if w := call(path+query, token); w.Code != 400 {
			t.Fatal("invalid cursor", w.Code)
		}
	}
	w := call(path+"?after_seq=100&limit=7", token)
	var page db.InboxStepPage
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &page) != nil || len(page.Steps) != 7 || page.NextSeq != 107 || !page.HasMore {
		t.Fatal(w.Code, w.Body.String())
	}
	redacted := call(path+"?after_seq=120", token)
	if redacted.Code != 200 || !strings.Contains(redacted.Body.String(), "Sensitive file details withheld") || strings.Contains(redacted.Body.String(), ".env credentials") {
		t.Fatal("private file detail exposed", redacted.Code)
	}
	foreign := q
	foreign.RequestID = db.GenerateUUID()
	foreign.MessageID = db.GenerateUUID()
	foreign.OwnerID = "another-owner"
	store.AcceptInboxRequest(ctx, foreign, &db.ProjectMessage{ID: foreign.MessageID, RoomID: room})
	if w := call(strings.Replace(path, q.RequestID, foreign.RequestID, 1), token); w.Code != 404 {
		t.Fatal("foreign owner trace", w.Code)
	}
}

type failingInboxJournal struct {
	db.InboxLifecycleRepository
	kind string
}

func (r failingInboxJournal) AppendInboxStep(ctx context.Context, room, id, kind, text string) error {
	if kind == r.kind {
		return errors.New("fixture journal unavailable")
	}
	return r.InboxLifecycleRepository.AppendInboxStep(ctx, room, id, kind, text)
}

func TestInboxJournalFailureNeverSavesACompletedReply(t *testing.T) {
	for _, kind := range []string{"accepted", "persistence"} {
		t.Run(kind, func(t *testing.T) {
			s, _ := setupTestChatServer(t)
			ctx := context.Background()
			now := time.Now()
			room := db.GenerateUUID()
			if err := s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: room, ProjectID: "default", Title: "Failure fixture", CreatedAt: now, UpdatedAt: now}); err != nil {
				t.Fatal(err)
			}
			repo := s.projectStudioRepo.(db.InboxLifecycleRepository)
			q := db.InboxRequest{RoomID: room, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
			repo.AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: room})
			q, ok, err := repo.ClaimInboxRequest(ctx, room, q.RequestID)
			if err != nil || !ok {
				t.Fatal(err)
			}
			s.runInboxRequest(failingInboxJournal{InboxLifecycleRepository: repo, kind: kind}, q)
			rows, err := repo.ListInboxRequests(ctx, room)
			if err != nil || len(rows) != 1 || rows[0].State != "interrupted" || db.InboxRecoveryAction(rows[0]) != "check_status" {
				t.Fatal("unsafe finalization after trace failure", rows, err)
			}
		})
	}
}
