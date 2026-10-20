package db

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestInboxRecoveryPolicy(t *testing.T) {
	for _, item := range []struct{ name, state, failure, kind, want string }{
		{"setup", "failed", "The chat sandbox is unavailable.", "", "try_again"},
		{"provider", "failed", "The provider could not reply.", "recovery_safe", "try_again"},
		{"quota", "failed", "Provider quota is limited.", "recovery_safe", "check_usage"},
		{"login", "failed", "Check the provider's sign-in status.", "", "check_agent"},
		{"unknown", "failed", "The result is unknown.", "recovery_safe", "check_status"},
		{"tool", "failed", "The provider could not reply.", "runtime_tool_started", "check_status"},
		{"legacy", "failed", "Something failed.", "", "check_status"},
		{"interrupted", "interrupted", "", "recovery_safe", "check_status"},
		{"done", "completed", "", "recovery_safe", ""},
	} {
		t.Run(item.name, func(t *testing.T) {
			q := InboxRequest{State: item.state, SafeError: item.failure, Steps: []InboxStep{{Kind: item.kind}}}
			if got := InboxRecoveryAction(q); got != item.want {
				t.Fatalf("%s != %s", got, item.want)
			}
		})
	}
}

func TestInboxRetryAtomicOriginalMessage(t *testing.T) {
	r := NewMemoryProjectStudioRepository()
	exerciseInboxRetry(t, r, r)
}

func exerciseInboxRetry(t *testing.T, reader InboxLifecycleRepository, retry InboxRetryRepository) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	q := InboxRequest{OwnerID: "owner", AgentID: "agent", RoomID: GenerateUUID(), RequestID: GenerateUUID(), MessageID: GenerateUUID(), ReplyID: GenerateUUID(), PayloadHash: "original-with-attachments", State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	// SQL fixtures create their own room and call the exported helper through server tests.
	m := &ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: "original", CreatedAt: now}
	if _, _, err := reader.AcceptInboxRequest(ctx, q, m); err != nil {
		t.Fatal(err)
	}
	q.State = "failed"
	q.SafeError = "The chat sandbox is unavailable."
	if err := reader.FinishInboxRequest(ctx, q, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := retry.RetryInboxRequest(ctx, q.RoomID, q.RequestID, "other"); err == nil {
		t.Fatal("cross-owner retry accepted")
	}
	if _, _, err := retry.RetryInboxRequest(ctx, GenerateUUID(), q.RequestID, q.OwnerID); err == nil {
		t.Fatal("cross-room retry accepted")
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	fresh := 0
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			child, created, err := retry.RetryInboxRequest(ctx, q.RoomID, q.RequestID, q.OwnerID)
			if err != nil {
				t.Error(err)
				return
			}
			if child.MessageID != q.MessageID || child.RequestID != InboxRetryID(q.RequestID) || child.ReplyID == q.ReplyID {
				t.Error("original correlation lost")
			}
			if created {
				mu.Lock()
				fresh++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if fresh != 1 {
		t.Fatalf("created %d retries", fresh)
	}
	all, err := reader.ListInboxRequests(ctx, q.RoomID)
	if err != nil || len(all) != 2 {
		t.Fatal(all, err)
	}
	if _, _, err := retry.RetryInboxRequest(ctx, q.RoomID, InboxRetryID(q.RequestID), q.OwnerID); err == nil {
		t.Fatal("running/queued retry accepted")
	}
}
