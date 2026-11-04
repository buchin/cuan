package task

import (
	"context"
	"fmt"
	"sort"
	"testing"
)

func TestInboxDeliveryCandidatesAvoidAcceptedHistoryAndBoardReadModels(t *testing.T) {
	ctx := context.Background()
	store := &countedRecoveryStore{MemoryStore: NewMemoryStore()}
	for i := 0; i < 7000; i++ {
		id := fmt.Sprintf("accepted-%d", i)
		if err := store.CreateTask(ctx, &Task{ID: id, Status: TaskStatusDone, SourceRoomID: "room", Instructions: managedInboxTaskPrefix + "Approved work", Summary: "Accepted", CompleteNote: "Verified"}); err != nil {
			t.Fatal(err)
		}
		if err := store.CreateAttempt(ctx, &TaskAttempt{ID: "att-" + id, TaskID: id, Status: AttemptStatusSucceeded}); err != nil {
			t.Fatal(err)
		}
	}
	for _, status := range []string{TaskStatusBacklog, TaskStatusQueued, TaskStatusPending, TaskStatusDoing, TaskStatusReview, TaskStatusBlocked, TaskStatusFailed, "cancelled"} {
		if err := store.CreateTask(ctx, &Task{ID: status, Status: status, SourceRoomID: "room", Instructions: managedInboxTaskPrefix + "Approved work"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []*Task{{ID: "manual", Status: TaskStatusDone, SourceRoomID: "room", Instructions: managedInboxTaskPrefix, Summary: "Owner result", CompleteNote: "Verified outside session"}, {ID: "empty-done", Status: TaskStatusDone, SourceRoomID: "room", Instructions: managedInboxTaskPrefix}, {ID: "unmanaged", Status: TaskStatusBlocked, SourceRoomID: "room", Instructions: "Separate task"}, {ID: "no-room", Status: TaskStatusBlocked, Instructions: managedInboxTaskPrefix}} {
		if err := store.CreateTask(ctx, row); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := NewService(store, nil).InboxDeliveryTasks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	sort.Strings(ids)
	if fmt.Sprint(ids) != "[backlog blocked cancelled doing failed manual pending queued review]" {
		t.Fatal("wrong delivery candidates", ids)
	}
	if store.fullLists != 0 || store.latestReads != 0 {
		t.Fatal("delivery loaded full history or individual latest attempts")
	}
}
func TestEmptyInboxDeliveryCandidatesNeverBecomeAnUnrestrictedList(t *testing.T) {
	store := &countedRecoveryStore{MemoryStore: NewMemoryStore()}
	if err := store.CreateTask(context.Background(), &Task{ID: "unrelated", Status: TaskStatusDone}); err != nil {
		t.Fatal(err)
	}
	rows, err := NewService(store, nil).InboxDeliveryTasks(context.Background())
	if err != nil || len(rows) != 0 || store.fullLists != 0 {
		t.Fatal("empty IDs loaded unrelated history", rows, err)
	}
}

type legacyDeliveryStore struct{ Store }

func (s legacyDeliveryStore) GetLatestAttempt(context.Context, string) (*TaskAttempt, error) {
	return nil, ErrAttemptNotFound
}
func TestInboxDeliveryLegacyStorePreservesOwnerCompletionWithoutAttempt(t *testing.T) {
	base := NewMemoryStore()
	ctx := context.Background()
	if err := base.CreateTask(ctx, &Task{ID: "manual", Status: TaskStatusDone, SourceRoomID: "room", Instructions: managedInboxTaskPrefix, Summary: "Owner result", CompleteNote: "Verified"}); err != nil {
		t.Fatal(err)
	}
	rows, err := NewService(legacyDeliveryStore{Store: base}, nil).InboxDeliveryTasks(ctx)
	if err != nil || len(rows) != 1 || rows[0].ID != "manual" {
		t.Fatal("legacy owner completion omitted", rows, err)
	}
}
