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
		{"cooldown_safe", "failed", "The provider could not reply.", "provider_cooldown", "check_usage"},
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

func automaticRecoveryFixture() InboxRequest {
	now := time.Now().UTC()
	started := now.Add(-10 * time.Second)
	return InboxRequest{OwnerID: "owner", AgentID: "agent", RoomID: GenerateUUID(), RequestID: GenerateUUID(), MessageID: GenerateUUID(), ReplyID: GenerateUUID(), PayloadHash: "original", State: "failed", SafeError: "The provider could not reply.", CreatedAt: started, StartedAt: &started, Deadline: now.Add(time.Minute),
		Steps: []InboxStep{
			{Seq: 1, Kind: "recovery_policy", Text: "safe-reply-v1", At: started},
			{Seq: 2, Kind: "provider", At: started},
			{Seq: 3, Kind: "recovery_safe", At: now.Add(-4 * time.Second)},
			{Seq: 4, Kind: "finished", At: now.Add(-3 * time.Second)},
		}}
}

func TestInboxAutomaticRecoveryPolicyIsBoundedAndFresh(t *testing.T) {
	base := automaticRecoveryFixture()
	now := time.Now()
	if q := InboxAutomaticRecoveryCandidate([]InboxRequest{base}, now); q == nil || q.RequestID != base.RequestID {
		t.Fatal("safe fresh request not eligible")
	}
	for _, scenario := range []string{"legacy", "no-proof", "quota", "auth", "tool", "unknown", "cancelled", "backoff", "stale", "next-message", "child", "busy"} {
		t.Run(scenario, func(t *testing.T) {
			q := base
			q.Steps = append([]InboxStep{}, base.Steps...)
			rows := []InboxRequest{}
			switch scenario {
			case "legacy":
				q.Steps = q.Steps[1:]
			case "no-proof":
				q.Steps = append(q.Steps[:2], q.Steps[3:]...)
			case "quota":
				q.SafeError = "Provider quota is limited."
			case "auth":
				q.SafeError = "Check the provider's sign-in status."
			case "tool":
				q.Steps = append(q.Steps, InboxStep{Kind: "runtime_tool_started"})
			case "unknown":
				q.SafeError = "The result is unknown."
			case "cancelled":
				q.State = "cancelled"
			case "backoff":
				q.Steps[3].At = now
			case "stale":
				q.Steps[3].At = now.Add(-3 * time.Minute)
			case "next-message":
				other := base
				other.RequestID = GenerateUUID()
				other.MessageID = GenerateUUID()
				other.State = "completed"
				other.CreatedAt = now
				rows = append(rows, other)
			case "child":
				other := base
				other.RequestID = InboxRetryID(base.RequestID)
				other.CreatedAt = now
				rows = append(rows, other)
			case "busy":
				other := base
				other.RequestID = GenerateUUID()
				other.State = "running"
				other.CreatedAt = now.Add(-time.Minute)
				rows = append(rows, other)
			}
			rows = append(rows, q)
			if InboxAutomaticRecoveryCandidate(rows, now) != nil {
				t.Fatal("unsafe/unbounded automatic recovery")
			}
		})
	}
}

func TestInboxAutomaticRecoverySharesManualIdempotency(t *testing.T) {
	r := NewMemoryProjectStudioRepository()
	ctx := context.Background()
	q := automaticRecoveryFixture()
	if _, _, err := r.AcceptInboxRequest(ctx, q, &ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, Content: "original", CreatedAt: q.CreatedAt}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	fresh := 0
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			var child InboxRequest
			var created bool
			var err error
			if index%2 == 0 {
				child, created, err = r.AutoRetryInboxRequest(ctx, q.RoomID, q.RequestID, q.OwnerID)
			} else {
				child, created, err = r.RetryInboxRequest(ctx, q.RoomID, q.RequestID, q.OwnerID)
			}
			if err != nil {
				t.Error(err)
				return
			}
			if child.RequestID != InboxRetryID(q.RequestID) || child.MessageID != q.MessageID {
				t.Error("lost correlation")
			}
			if created {
				mu.Lock()
				fresh++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if fresh != 1 {
		t.Fatalf("created %d children", fresh)
	}
	all, _ := r.ListInboxRequests(ctx, q.RoomID)
	if len(all) != 2 {
		t.Fatal("duplicate child")
	}
	// A late new user message prevents a stale recovery decision under the lock.
	next := automaticRecoveryFixture()
	next.RoomID = q.RoomID
	next.CreatedAt = time.Now()
	next.State = "completed"
	r.AcceptInboxRequest(ctx, next, &ProjectMessage{ID: next.MessageID, RoomID: q.RoomID})
	// A separate safe failure (without an existing child) must be rejected.
	stale := automaticRecoveryFixture()
	stale.RoomID = q.RoomID
	r.AcceptInboxRequest(ctx, stale, &ProjectMessage{ID: stale.MessageID, RoomID: q.RoomID})
	if _, _, err := r.AutoRetryInboxRequest(ctx, q.RoomID, stale.RequestID, q.OwnerID); err == nil {
		t.Fatal("overtook later user message")
	}
}

func TestInboxReadOnlyToolRecoveryNeedsFinalProof(t *testing.T) {
	q := automaticRecoveryFixture()
	q.Steps = append(q.Steps[:2], append([]InboxStep{{Kind: "runtime_tool_finished", Text: "Read file"}}, q.Steps[2:]...)...)
	if InboxRecoveryAction(q) != "try_again" || InboxAutomaticRecoveryCandidate([]InboxRequest{q}, time.Now()) == nil {
		t.Fatal("verified read-only recovery rejected")
	}
	q.Steps = append(q.Steps, InboxStep{Kind: "runtime_tool_started", Text: "Unknown later tool"})
	if InboxRecoveryAction(q) != "check_status" {
		t.Fatal("earlier proof authorized later tool activity")
	}
}

func TestLocationRestrictionDoesNotRetryAfterNoEffectProof(t *testing.T) {
	q := InboxRequest{State: "failed", SafeError: "The provider rejected execution because of a location restriction. Google sign-in may still be saved.", Steps: []InboxStep{{Kind: "recovery_safe"}}}
	if got := InboxRecoveryAction(q); got != "check_status" {
		t.Fatal("persistent provider restriction retried or treated as login failure", got)
	}
}
