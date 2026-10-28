package db

import (
	"context"
	"errors"
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
		{"tool_budget_error", "failed", "Provider tool budget exceeded for chat.", "runtime_tool_started", "continue_analysis"},
		{"tool_budget_step", "failed", "The provider could not reply.", "runtime_tool_budget_exceeded", "continue_analysis"},
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

func TestManualRetryPermitsUsageAndAgentFailures(t *testing.T) {
	r := NewMemoryProjectStudioRepository()
	ctx := context.Background()
	now := time.Now().UTC()

	// 1. Quota failure (check_usage)
	quotaReq := InboxRequest{
		RoomID:    "room1",
		RequestID: "req-quota",
		MessageID: "msg-quota",
		OwnerID:   "owner1",
		AgentID:   "agent1",
		State:     "failed",
		SafeError: "All connected AI accounts reached provider usage limits. Provider quota is limited.",
		CreatedAt: now,
		Steps:     []InboxStep{{Seq: 1, Kind: "provider_cooldown", Text: "usage limit reached", At: now}},
	}
	if got := InboxRecoveryAction(quotaReq); got != "check_usage" {
		t.Fatalf("expected check_usage, got %s", got)
	}
	r.AcceptInboxRequest(ctx, quotaReq, &ProjectMessage{ID: quotaReq.MessageID, RoomID: quotaReq.RoomID, Content: "test quota", CreatedAt: now})

	// Automatic retry must reject check_usage
	if _, _, err := r.AutoRetryInboxRequest(ctx, quotaReq.RoomID, quotaReq.RequestID, quotaReq.OwnerID); !errors.Is(err, ErrInboxRetryUnavailable) {
		t.Fatalf("expected ErrInboxRetryUnavailable for auto-retry on check_usage, got %v", err)
	}
	// Manual retry must succeed for check_usage
	child, created, err := r.RetryInboxRequest(ctx, quotaReq.RoomID, quotaReq.RequestID, quotaReq.OwnerID)
	if err != nil || !created || child.State != "queued" {
		t.Fatalf("expected manual retry to succeed for check_usage, got child=%+v, created=%v, err=%v", child, created, err)
	}

	// 2. Unsafe failure (check_status) must be rejected for both manual and auto
	unsafeReq := InboxRequest{
		RoomID:    "room2",
		RequestID: "req-unsafe",
		MessageID: "msg-unsafe",
		OwnerID:   "owner1",
		AgentID:   "agent1",
		State:     "failed",
		SafeError: "The result is unknown.",
		CreatedAt: now,
		Steps:     []InboxStep{{Seq: 1, Kind: "runtime_tool_started", Text: "rm file", At: now}},
	}
	if got := InboxRecoveryAction(unsafeReq); got != "check_status" {
		t.Fatalf("expected check_status, got %s", got)
	}
	r.AcceptInboxRequest(ctx, unsafeReq, &ProjectMessage{ID: unsafeReq.MessageID, RoomID: unsafeReq.RoomID, Content: "test unsafe", CreatedAt: now})

	if _, _, err := r.RetryInboxRequest(ctx, unsafeReq.RoomID, unsafeReq.RequestID, unsafeReq.OwnerID); !errors.Is(err, ErrInboxRetryUnavailable) {
		t.Fatalf("expected ErrInboxRetryUnavailable for manual retry on check_status, got %v", err)
	}

	// 3. Interrupted request allows manual retry but rejects auto-retry
	interruptedReq := InboxRequest{
		RoomID:    "room3",
		RequestID: "req-interrupted",
		MessageID: "msg-interrupted",
		OwnerID:   "owner1",
		AgentID:   "agent1",
		State:     "interrupted",
		SafeError: "The process was interrupted or timed out. The provider's result is unknown. The request was not rerun.",
		CreatedAt: now,
		Steps:     []InboxStep{{Seq: 1, Kind: "recovery_safe", Text: "safe", At: now}},
	}
	r.AcceptInboxRequest(ctx, interruptedReq, &ProjectMessage{ID: interruptedReq.MessageID, RoomID: interruptedReq.RoomID, Content: "test interrupted", CreatedAt: now})
	if _, _, err := r.AutoRetryInboxRequest(ctx, interruptedReq.RoomID, interruptedReq.RequestID, interruptedReq.OwnerID); !errors.Is(err, ErrInboxRetryUnavailable) {
		t.Fatalf("expected ErrInboxRetryUnavailable for auto-retry on interrupted, got %v", err)
	}
	childInterrupted, createdInterrupted, err := r.RetryInboxRequest(ctx, interruptedReq.RoomID, interruptedReq.RequestID, interruptedReq.OwnerID)
	if err != nil || !createdInterrupted || childInterrupted.State != "queued" {
		t.Fatalf("expected manual retry to succeed for interrupted, got child=%+v, created=%v, err=%v", childInterrupted, createdInterrupted, err)
	}
}

func TestInboxRetryOnToolBudgetExceeded(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	r := NewMemoryProjectStudioRepository()

	req := InboxRequest{
		RoomID:    "room-budget",
		RequestID: "req-budget-1",
		MessageID: "msg-budget",
		OwnerID:   "owner1",
		AgentID:   "agent1",
		State:     "failed",
		SafeError: "Provider tool budget exceeded for chat.",
		CreatedAt: now,
		Steps: []InboxStep{
			{Seq: 1, Kind: "runtime_tool_started", Text: "Tool #1 · read_file started.", At: now},
			{Seq: 2, Kind: "runtime_tool_finished", Text: "Tool #1 · read_file finished.", At: now},
			{Seq: 3, Kind: "runtime_tool_budget_exceeded", Text: "Tool limit of 12 calls reached for coordinator chat. Finishing response.", At: now},
			{Seq: 4, Kind: "recovery_safe", Text: "No modifying action was performed. A new reply attempt is available.", At: now},
		},
	}
	r.AcceptInboxRequest(ctx, req, &ProjectMessage{ID: req.MessageID, RoomID: req.RoomID, Content: "test tool budget query", CreatedAt: now})

	action := InboxRecoveryAction(req)
	if action != "continue_analysis" {
		t.Fatalf("expected recovery action 'continue_analysis', got %q", action)
	}

	child, created, err := r.RetryInboxRequest(ctx, req.RoomID, req.RequestID, req.OwnerID)
	if err != nil {
		t.Fatalf("expected retry to succeed on tool budget exceeded, got err: %v", err)
	}
	if !created {
		t.Fatalf("expected child request to be newly created")
	}
	if child.State != "queued" {
		t.Fatalf("expected child request to be queued, got state: %s", child.State)
	}
	if child.MessageID != req.MessageID {
		t.Fatalf("expected child request to keep original message ID %s, got %s", req.MessageID, child.MessageID)
	}
}
