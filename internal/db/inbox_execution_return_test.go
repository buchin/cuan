package db

import (
	"context"
	"testing"
	"time"
)

func TestExpiredInboxRetainsExecutorReturnWithoutAcceptingLateReply(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryProjectStudioRepository()
	now := time.Now().UTC()
	q := InboxRequest{RoomID: GenerateUUID(), RequestID: GenerateUUID(), MessageID: GenerateUUID(), ReplyID: GenerateUUID(), State: "running", CreatedAt: now.Add(-time.Minute), Deadline: now.Add(-time.Second)}
	if _, _, err := repo.AcceptInboxRequest(ctx, q, &ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, Content: "original"}); err != nil {
		t.Fatal(err)
	}
	rows, err := repo.ListInboxRequests(ctx, q.RoomID)
	if err != nil || len(rows) != 1 || rows[0].State != "interrupted" || inboxHasExecutionReturn(rows[0].Steps) {
		t.Fatal("deadline fabricated executor return", rows, err)
	}
	originalError := rows[0].SafeError
	q.State = "completed"
	late := &ProjectMessage{ID: q.ReplyID, RoomID: q.RoomID, Content: "unverified late result"}
	for i := 0; i < 2; i++ {
		if err := repo.FinishInboxRequest(ctx, q, late); err != nil {
			t.Fatal(err)
		}
	}
	rows, err = repo.ListInboxRequests(ctx, q.RoomID)
	if err != nil || rows[0].State != "interrupted" || rows[0].SafeError != originalError || len(rows[0].Steps) != 1 || !inboxHasExecutionReturn(rows[0].Steps) {
		t.Fatal("lost outcome or duplicated receipt", rows, err)
	}
	messages, err := repo.ListMessagesByRoomID(ctx, q.RoomID)
	if err != nil || len(messages) != 1 {
		t.Fatal("accepted late reply", messages, err)
	}
	if InboxAutomaticRecoveryCandidate(rows, time.Now()) != nil {
		t.Fatal("return receipt alone authorized retry")
	}
}

func TestExecutionReturnReceiptSurvivesTraceCompaction(t *testing.T) {
	steps := []InboxStep{{Seq: 1, Kind: "accepted"}, {Seq: 2, Kind: "execution_returned"}}
	for i := 3; i < 250; i++ {
		steps = append(steps, InboxStep{Seq: i, Kind: "diagnostic"})
	}
	compact := compactInboxSteps(steps)
	if !inboxHasExecutionReturn(compact) {
		t.Fatal("executor receipt discarded")
	}
}
