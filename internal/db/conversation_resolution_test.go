package db

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestConversationResolutionPreservesHistoryAndReopensOnNewMessage(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryProjectStudioRepository()
	room := &ProjectRoom{ProjectID: "project", Title: "Customer request"}
	if err := repo.CreateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}
	before := time.Now().UTC().Add(-time.Minute)
	first := &ProjectMessage{RoomID: room.ID, AuthorType: "user", Content: "Keep my history", CreatedAt: before}
	if err := repo.CreateMessage(ctx, first); err != nil {
		t.Fatal(err)
	}
	cutoff := before.Add(time.Second)
	if err := repo.SetRoomResolved(ctx, room.ID, &cutoff); err != nil {
		t.Fatal(err)
	}
	saved, _ := repo.GetRoomByID(ctx, room.ID)
	if !saved.IsResolved(first.CreatedAt) || !saved.UpdatedAt.Equal(room.UpdatedAt) {
		t.Fatal("resolution changed ordering or was not saved")
	}
	// Returned values cannot mutate shared resolution state.
	*saved.ResolvedAt = before.Add(-time.Hour)
	saved, _ = repo.GetRoomByID(ctx, room.ID)
	if !saved.IsResolved(first.CreatedAt) {
		t.Fatal("resolution pointer escaped repository")
	}
	second := &ProjectMessage{RoomID: room.ID, AuthorType: "user", Content: "Additional scope", CreatedAt: cutoff.Add(time.Second)}
	if err := repo.CreateMessage(ctx, second); err != nil {
		t.Fatal(err)
	}
	if saved.IsResolved(second.CreatedAt) {
		t.Fatal("new message did not reopen conversation")
	}
	if err := repo.SetRoomResolved(ctx, room.ID, &cutoff); !errors.Is(err, ErrConversationChanged) {
		t.Fatal("stale resolution accepted", err)
	}
	messages, err := repo.ListMessagesByRoomID(ctx, room.ID)
	if err != nil || len(messages) != 2 {
		t.Fatal("resolution lost history", err)
	}
	if err := repo.SetRoomResolved(ctx, room.ID, nil); err != nil {
		t.Fatal(err)
	}
	saved, _ = repo.GetRoomByID(ctx, room.ID)
	if saved.ResolvedAt != nil || saved.IsResolved(second.CreatedAt) {
		t.Fatal("explicit reopening failed")
	}
}
