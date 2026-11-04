package db

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestInboxDiscoveryFindsPendingAndRecentRecoveryWithoutIdleRooms(t *testing.T) {
	repo := NewMemoryProjectStudioRepository()
	now := time.Now().UTC()
	for i := 0; i < 300; i++ {
		id := GenerateUUID()
		repo.rooms[id] = &ProjectRoom{ID: id, Title: "Idle history"}
	}
	// Durable requests are authoritative even without a project-room row.
	for _, q := range []InboxRequest{
		{RoomID: "pending", RequestID: "1", State: "queued", CreatedAt: now.Add(-time.Hour)},
		{RoomID: "pending", RequestID: "2", State: "queued", CreatedAt: now},
		{RoomID: "recent", RequestID: "3", State: "failed", CreatedAt: now, Steps: []InboxStep{{Kind: "finished", At: now.Add(-time.Minute)}}},
		{RoomID: "old", RequestID: "4", State: "failed", CreatedAt: now.Add(-time.Hour), Steps: []InboxStep{{Kind: "finished", At: now.Add(-3 * time.Minute)}}},
		{RoomID: "canceled", RequestID: "5", State: "cancelled", CreatedAt: now},
	} {
		repo.requests[q.RoomID+q.RequestID] = q
	}
	got, err := repo.ListInboxDispatchRooms(context.Background(), now)
	if err != nil || !reflect.DeepEqual(got, []string{"pending", "recent"}) {
		t.Fatal(got, err)
	}
}
