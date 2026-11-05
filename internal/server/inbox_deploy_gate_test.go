package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/db"
	"golang.org/x/sys/unix"
)

func TestInboxDeploymentGatePreservesActiveAndQueuedRequests(t *testing.T) {
	ctx := context.Background()
	repo := db.NewMemoryProjectStudioRepository()
	room := db.GenerateUUID()
	if err := repo.CreateRoom(ctx, &db.ProjectRoom{ID: room, ProjectID: "default", Title: "Drain fixture"}); err != nil {
		t.Fatal(err)
	}
	s := &Server{cfg: &config.Config{WorkspacesDir: filepath.Join(t.TempDir(), "projects")}}
	accept := func() db.InboxRequest {
		t.Helper()
		q := db.InboxRequest{RoomID: room, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), OwnerID: "owner", PayloadHash: db.GenerateUUID(), State: "queued", CreatedAt: time.Now(), Deadline: time.Now().Add(time.Hour)}
		saved, _, err := repo.AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: room, AuthorType: "user", Content: "Preserve this message"})
		if err != nil {
			t.Fatal(err)
		}
		return saved
	}
	first := accept()
	active, claimed, err := s.claimInboxForDispatch(ctx, repo, room, first.RequestID)
	if err != nil || !claimed {
		t.Fatal(err)
	}
	gate, err := os.OpenFile(filepath.Join(filepath.Dir(s.cfg.WorkspacesDir), "inbox-admission.lock"), os.O_RDWR, 0660)
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Close()
	if err = unix.Flock(int(gate.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatal("claim lock leaked into active execution", err)
	}
	second := accept()
	if _, claimed, err = s.claimInboxForDispatch(ctx, repo, room, second.RequestID); err == nil || claimed {
		t.Fatal("deployment admitted a new provider execution")
	}
	active.State = "completed"
	if err = repo.FinishInboxRequest(ctx, active, &db.ProjectMessage{ID: active.ReplyID, RoomID: room, AuthorType: "agent", Content: "Existing execution finished"}); err != nil {
		t.Fatal(err)
	}
	requests, _ := repo.ListInboxRequests(ctx, room)
	if len(requests) != 2 || requests[0].State != "completed" || requests[1].State != "queued" {
		t.Fatal("drain cancelled work or discarded the queued request", requests)
	}
	if err = unix.Flock(int(gate.Fd()), unix.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	resumed, claimed, err := s.claimInboxForDispatch(ctx, repo, room, second.RequestID)
	if err != nil || !claimed || resumed.MessageID != second.MessageID {
		t.Fatal("queued work did not resume under original identity", err)
	}
	if _, claimed, err = s.claimInboxForDispatch(ctx, repo, room, second.RequestID); err != nil || claimed {
		t.Fatal("queued message executed twice", err)
	}
}
