package db

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestInboxLifecycleFIFOAndCancellation(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryProjectStudioRepository()
	room := GenerateUUID()
	now := time.Now().UTC()
	accept := func(offset time.Duration) InboxRequest {
		q := InboxRequest{RoomID: room, RequestID: GenerateUUID(), MessageID: GenerateUUID(), ReplyID: GenerateUUID(), PayloadHash: "same", State: "queued", CreatedAt: now.Add(offset), Deadline: now.Add(time.Hour)}
		m := &ProjectMessage{ID: q.MessageID, RoomID: room, Content: "fixture", CreatedAt: q.CreatedAt}
		if _, fresh, e := r.AcceptInboxRequest(ctx, q, m); e != nil || !fresh {
			t.Fatal(e)
		}
		return q
	}
	first := accept(0)
	second := accept(time.Millisecond)
	if _, ok, e := r.ClaimInboxRequest(ctx, room, second.RequestID); e != nil || ok {
		t.Fatal("overtook FIFO head", e)
	}
	winners := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok, e := r.ClaimInboxRequest(ctx, room, first.RequestID); e != nil {
				t.Error(e)
			} else if ok {
				mu.Lock()
				winners++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if winners != 1 {
		t.Fatal("claim winners", winners)
	}
	if err := r.AppendInboxStep(ctx, room, first.RequestID, "provider", "Waiting for fixture"); err != nil {
		t.Fatal(err)
	}
	if _, e := r.CancelInboxRequest(ctx, room, first.RequestID); e != nil {
		t.Fatal(e)
	}
	if _, ok, _ := r.ClaimInboxRequest(ctx, room, second.RequestID); ok {
		t.Fatal("started while cancellation pending")
	}
	first.State = "completed"
	if e := r.FinishInboxRequest(ctx, first, &ProjectMessage{ID: first.ReplyID, RoomID: room, Content: "late output"}); e != nil {
		t.Fatal(e)
	}
	all, _ := r.ListInboxRequests(ctx, room)
	if all[0].State != "cancelled" || len(all[0].Steps) != 2 || all[0].Steps[1].Kind != "finished" {
		t.Fatalf("lost cancellation/steps: %+v", all[0])
	}
	messages, _ := r.ListMessagesByRoomID(ctx, room)
	if len(messages) != 2 {
		t.Fatal("persisted late cancelled reply", len(messages))
	}
	if _, ok, e := r.ClaimInboxRequest(ctx, room, second.RequestID); e != nil || !ok {
		t.Fatal("FIFO did not continue", e)
	}
	first.State = "running"
	if e := r.FinishInboxRequest(ctx, first, nil); e != nil {
		t.Fatal(e)
	}
	all, _ = r.ListInboxRequests(ctx, room)
	if all[0].State != "cancelled" {
		t.Fatal("revived terminal state")
	}
	// Snapshot mutation must not change stored history.
	all[0].Steps[0].Text = "modified"
	all, _ = r.ListInboxRequests(ctx, room)
	if all[0].Steps[0].Text == "modified" {
		t.Fatal("shared mutable snapshot")
	}
}
func TestInboxLifecycleQueueCapacityAndIdempotency(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryProjectStudioRepository()
	room := GenerateUUID()
	var original InboxRequest
	for i := 0; i < 20; i++ {
		q := InboxRequest{RoomID: room, RequestID: GenerateUUID(), MessageID: GenerateUUID(), State: "queued", PayloadHash: "same", CreatedAt: time.Now()}
		if i == 0 {
			original = q
		}
		if _, _, e := r.AcceptInboxRequest(ctx, q, &ProjectMessage{ID: q.MessageID, RoomID: room}); e != nil {
			t.Fatal(e)
		}
	}
	if _, fresh, e := r.AcceptInboxRequest(ctx, original, &ProjectMessage{}); e != nil || fresh {
		t.Fatal("retry blocked at capacity", e)
	}
	original.PayloadHash = "different"
	if _, _, e := r.AcceptInboxRequest(ctx, original, &ProjectMessage{}); !errors.Is(e, ErrInboxRequestConflict) {
		t.Fatal("conflict not rejected", e)
	}
	original.RequestID = GenerateUUID()
	if _, _, e := r.AcceptInboxRequest(ctx, original, &ProjectMessage{}); !errors.Is(e, ErrInboxQueueFull) {
		t.Fatal("capacity not enforced", e)
	}
}

func TestInboxFinalTimestampSurvivesFullTimeline(t *testing.T) {
	r := NewMemoryProjectStudioRepository()
	ctx := context.Background()
	now := time.Now().UTC()
	q := InboxRequest{RoomID: GenerateUUID(), RequestID: GenerateUUID(), MessageID: GenerateUUID(), ReplyID: GenerateUUID(), State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	r.AcceptInboxRequest(ctx, q, &ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, CreatedAt: now})
	q, _, _ = r.ClaimInboxRequest(ctx, q.RoomID, q.RequestID)
	for i := 0; i < 120; i++ {
		r.AppendInboxStep(ctx, q.RoomID, q.RequestID, "runtime_tool_started", "Fixture")
	}
	q.State = "interrupted"
	r.FinishInboxRequest(ctx, q, nil)
	all, _ := r.ListInboxRequests(ctx, q.RoomID)
	if len(all[0].Steps) != 101 || all[0].Steps[100].Kind != "finished" {
		t.Fatal("final timestamp lost at activity limit")
	}
	r.FinishInboxRequest(ctx, q, nil)
	all, _ = r.ListInboxRequests(ctx, q.RoomID)
	if len(all[0].Steps) != 101 {
		t.Fatal("duplicate final timestamp")
	}
}
