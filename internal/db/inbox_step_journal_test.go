package db

import (
	"context"
	"database/sql"
	"errors"
	"math/rand"
	"reflect"
	"testing"
	"time"
)

func TestInboxStepSnapshotPreservesRecoveryAcrossLongHistories(t *testing.T) {
	random := rand.New(rand.NewSource(17))
	kinds := []string{"runtime_tool_started", "provider", "recovery_safe", "runtime_permission_denied", "runtime_tool_budget_exceeded", "provider_cooldown", "finished", "context", "runtime_tool_finished", "recovery_policy"}
	now := time.Now().UTC()
	for trial := 0; trial < 50; trial++ {
		full := []InboxStep{{Seq: 1, Kind: "retry_reconcile", At: now}}
		snapshot := append([]InboxStep(nil), full...)
		for seq := 2; seq <= 1000; seq++ {
			text := []string{"stage", "quota exhausted", "tool limit reached", "safe-reply-v1"}[random.Intn(4)]
			s := InboxStep{Seq: seq, Kind: kinds[random.Intn(len(kinds))], Text: text, At: now}
			full = append(full, s)
			snapshot = compactInboxSteps(append(snapshot, s))
			original := InboxRequest{State: "failed", SafeError: "Provider is unavailable.", OwnerID: "owner", AgentID: "agent", Steps: full}
			compact := original
			compact.Steps = snapshot
			if len(snapshot) > InboxStepSnapshotLimit || snapshot[len(snapshot)-1].Seq != seq || InboxRecoveryAction(original) != InboxRecoveryAction(compact) || InboxRetryNeedsReconciliation(original) != InboxRetryNeedsReconciliation(compact) || (InboxAutomaticRecoveryCandidate([]InboxRequest{original}, now.Add(3*time.Second)) != nil) != (InboxAutomaticRecoveryCandidate([]InboxRequest{compact}, now.Add(3*time.Second)) != nil) {
				t.Fatalf("recovery projection changed semantics trial %d seq %d", trial, seq)
			}
		}
	}
}

func TestInboxStepJournalRetainsLateEvidenceAndPagesWithoutReplay(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryProjectStudioRepository()
	now := time.Now()
	q := InboxRequest{RoomID: "room", RequestID: "request", OwnerID: "owner", MessageID: "message", State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour), Steps: []InboxStep{{Seq: 1, Kind: "retry_reconcile", At: now}}}
	if _, _, err := r.AcceptInboxRequest(ctx, q, &ProjectMessage{ID: q.MessageID}); err != nil {
		t.Fatal(err)
	}
	q, _, _ = r.ClaimInboxRequest(ctx, q.RoomID, q.RequestID)
	for i := 0; i < 240; i++ {
		if err := r.AppendInboxStep(ctx, q.RoomID, q.RequestID, "context", "stage"); err != nil {
			t.Fatal(err)
		}
	}
	r.AppendInboxStep(ctx, q.RoomID, q.RequestID, "runtime_permission_denied", "denied")
	q.State = "failed"
	r.FinishInboxRequest(ctx, q, nil)
	rows, _ := r.ListInboxRequests(ctx, q.RoomID)
	if len(rows[0].Steps) > 100 || InboxRecoveryAction(rows[0]) != "check_status" || !InboxRetryNeedsReconciliation(rows[0]) {
		t.Fatal("late safety evidence lost")
	}
	var all []InboxStep
	cursor := 0
	for {
		page, err := r.ListInboxSteps(ctx, q.RoomID, q.RequestID, "owner", cursor, 17)
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, page.Steps...)
		cursor = page.NextSeq
		if !page.HasMore {
			break
		}
	}
	if len(all) != 243 || all[241].Kind != "runtime_permission_denied" || all[242].Kind != "finished" {
		t.Fatalf("late events missing: %d", len(all))
	}
	for i, s := range all {
		if s.Seq != i+1 {
			t.Fatal("non-contiguous journal", i, s.Seq)
		}
	}
	r.FinishInboxRequest(ctx, q, nil)
	r.AppendInboxStep(ctx, q.RoomID, q.RequestID, "provider", "must not append after completion")
	page, _ := r.ListInboxSteps(ctx, q.RoomID, q.RequestID, "owner", 240, 17)
	if !reflect.DeepEqual(page.Steps, all[240:]) {
		t.Fatal("terminal request mutated")
	}
	if _, err := r.ListInboxSteps(ctx, q.RoomID, q.RequestID, "other", 0, 17); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("foreign owner exposed", err)
	}
}

func TestInboxStepSnapshotDoesNotEraseLateUnsafeWorkOrLegacyUncertainty(t *testing.T) {
	now := time.Now().UTC()
	for _, tail := range [][]InboxStep{
		{{Kind: "runtime_tool_started"}, {Kind: "recovery_safe"}},
		{{Kind: "recovery_safe"}, {Kind: "runtime_tool_started"}},
		{{Kind: "runtime_tool_started"}, {Kind: "recovery_safe"}, {Kind: "provider"}},
		{{Kind: "trace_history_incomplete"}, {Kind: "recovery_safe"}},
	} {
		steps := []InboxStep{{Seq: 1, Kind: "recovery_policy", Text: "safe-reply-v1", At: now}}
		for i := 0; i < 180; i++ {
			steps = append(steps, InboxStep{Seq: len(steps) + 1, Kind: "context", At: now})
		}
		for _, s := range tail {
			s.Seq = len(steps) + 1
			s.At = now
			steps = append(steps, s)
		}
		for i := 0; i < 180; i++ {
			steps = append(steps, InboxStep{Seq: len(steps) + 1, Kind: "context", At: now})
		}
		steps = append(steps, InboxStep{Seq: len(steps) + 1, Kind: "finished", At: now})
		full := InboxRequest{State: "failed", OwnerID: "owner", AgentID: "agent", Steps: steps}
		compact := full
		compact.Steps = compactInboxSteps(steps)
		if InboxRecoveryAction(full) != InboxRecoveryAction(compact) || (InboxAutomaticRecoveryCandidate([]InboxRequest{full}, now.Add(3*time.Second)) != nil) != (InboxAutomaticRecoveryCandidate([]InboxRequest{compact}, now.Add(3*time.Second)) != nil) {
			t.Fatal("unsafe recovery after compaction", tail)
		}
	}
}
