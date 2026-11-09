package db

import (
	"context"
	"testing"
	"time"
)

func timeoutRecoveryFixture() InboxRequest {
	q := automaticRecoveryFixture()
	q.State = "interrupted"
	q.SafeError = interruptedInbox
	q.Steps = []InboxStep{
		{Seq: 1, Kind: "recovery_policy", Text: "safe-reply-timeout-reconcile-v2", At: q.CreatedAt},
		{Seq: 2, Kind: "provider", At: q.CreatedAt},
		{Seq: 3, Kind: "runtime_tool_started", Text: "unknown effect", At: q.CreatedAt},
		{Seq: 4, Kind: "execution_timeout_returned", At: time.Now().Add(-3 * time.Second)},
	}
	return q
}

func TestInboxTimeoutRecoveryRequiresFreshReturnedExecutor(t *testing.T) {
	now := time.Now()
	base := timeoutRecoveryFixture()
	if InboxAutomaticRecoveryCandidate([]InboxRequest{base}, now) == nil {
		t.Fatal("fresh opted-in timeout not reconciled")
	}
	for _, scenario := range []string{"legacy", "no-return", "generic-return", "stale", "backoff", "incomplete", "permission", "cancelled", "busy", "newer", "sibling", "provider-restarted"} {
		t.Run(scenario, func(t *testing.T) {
			q := base
			q.Steps = append([]InboxStep(nil), base.Steps...)
			rows := []InboxRequest{}
			switch scenario {
			case "legacy":
				q.Steps = q.Steps[1:]
			case "no-return":
				q.Steps = q.Steps[:3]
			case "generic-return":
				q.Steps[3].Kind = "execution_returned"
			case "stale":
				q.Steps[3].At = now.Add(-3 * time.Minute)
			case "backoff":
				q.Steps[3].At = now
			case "incomplete":
				q.Steps = append(q.Steps, InboxStep{Kind: "trace_history_incomplete"})
			case "permission":
				q.Steps = append(q.Steps, InboxStep{Kind: "runtime_permission_denied"})
			case "cancelled":
				q.State = "cancelled"
			case "busy", "newer", "sibling":
				other := base
				other.RequestID = GenerateUUID()
				other.CreatedAt = now
				if scenario == "busy" {
					other.State = "running"
					other.CreatedAt = now.Add(-time.Hour)
				}
				if scenario == "newer" {
					other.MessageID = GenerateUUID()
					other.State = "completed"
				}
				rows = append(rows, other)
			case "provider-restarted":
				q.Steps = append(q.Steps, InboxStep{Kind: "provider"})
			}
			rows = append(rows, q)
			if InboxAutomaticRecoveryCandidate(rows, now) != nil {
				t.Fatal("unsafe recovery allowed")
			}
		})
	}
}

func TestInboxTimeoutAutoRetryPreservesReconciliationAndManualIdentity(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryProjectStudioRepository()
	q := timeoutRecoveryFixture()
	if _, _, err := repo.AcceptInboxRequest(ctx, q, &ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, Content: "original"}); err != nil {
		t.Fatal(err)
	}
	child, fresh, err := repo.AutoRetryInboxRequest(ctx, q.RoomID, q.RequestID, q.OwnerID)
	if err != nil || !fresh || !InboxRetryNeedsReconciliation(child) || child.MessageID != q.MessageID {
		t.Fatal("lost reconciliation contract", child, fresh, err)
	}
	manual, created, err := repo.RetryInboxRequest(ctx, q.RoomID, q.RequestID, q.OwnerID)
	if err != nil || created || manual.RequestID != child.RequestID {
		t.Fatal("manual retry duplicated recovery", manual, err)
	}
	rows, _ := repo.ListInboxRequests(ctx, q.RoomID)
	if len(rows) != 2 || InboxAutomaticRecoveryCandidate(rows, time.Now()) != nil {
		t.Fatal("retry allowance renewed", rows)
	}
}

func TestInboxCancellationSurvivesDeadlineInspection(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryProjectStudioRepository()
	q := timeoutRecoveryFixture()
	q.State = "running"
	q.Deadline = time.Now().Add(-time.Second)
	if _, _, err := repo.AcceptInboxRequest(ctx, q, &ProjectMessage{ID: q.MessageID, RoomID: q.RoomID}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CancelInboxRequest(ctx, q.RoomID, q.RequestID); err != nil {
		t.Fatal(err)
	}
	rows, err := repo.ListInboxRequests(ctx, q.RoomID)
	if err != nil || rows[0].State != "cancelled" {
		t.Fatal("deadline erased cancellation", rows, err)
	}
	q.State = "interrupted"
	q.ExecutionTimedOut = true
	if err := repo.FinishInboxRequest(ctx, q, nil); err != nil {
		t.Fatal(err)
	}
	rows, _ = repo.ListInboxRequests(ctx, q.RoomID)
	if rows[0].State != "cancelled" || InboxAutomaticRecoveryCandidate(rows, time.Now()) != nil {
		t.Fatal("cancelled work recovered", rows)
	}
}
