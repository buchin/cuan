package task

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"
)

type countedRecoveryStore struct {
	*MemoryStore
	fullLists, latestReads int
}

func (s *countedRecoveryStore) ListTasks(ctx context.Context, f TaskFilter) ([]*Task, error) {
	if f.IDs == nil {
		s.fullLists++
	}
	return s.MemoryStore.ListTasks(ctx, f)
}
func (s *countedRecoveryStore) GetLatestAttempt(ctx context.Context, id string) (*TaskAttempt, error) {
	s.latestReads++
	return s.MemoryStore.GetLatestAttempt(ctx, id)
}
func recoveryCandidateFixture(t *testing.T) (*Service, *countedRecoveryStore) {
	t.Helper()
	ctx := context.Background()
	store := &countedRecoveryStore{MemoryStore: NewMemoryStore()}
	now := time.Now().UTC()
	for i := 0; i < 7000; i++ {
		if err := store.CreateTask(ctx, &Task{ID: fmt.Sprintf("done-%05d", i), Status: TaskStatusDone}); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct{ id, status, attempt, runner string }{
		{"stale-board", TaskStatusQueued, AttemptStatusRunning, "daemon"},
		{"stale-doing", TaskStatusDoing, AttemptStatusFailed, "local_terminal"},
		{"local-queued", TaskStatusQueued, AttemptStatusQueued, "local_terminal"},
		{"daemon-queued", TaskStatusQueued, AttemptStatusQueued, "daemon"},
		{"review", TaskStatusReview, AttemptStatusSucceeded, "daemon"},
	} {
		if err := store.CreateTask(ctx, &Task{ID: row.id, Status: row.status, CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
		lease, deadline := now.Add(time.Minute), now.Add(25*time.Minute)
		if err := store.CreateAttempt(ctx, &TaskAttempt{ID: "att-" + row.id, TaskID: row.id, Status: row.attempt, RunnerID: row.runner, AttemptNumber: 1, CreatedAt: now, StartedAt: &now, LeaseExpiresAt: &lease, ExecutionDeadline: &deadline}); err != nil {
			t.Fatal(err)
		}
	}
	return NewService(store, nil), store
}

func TestRecoveryCandidatesExcludeSettledHistoryAndIncludeStaleBoards(t *testing.T) {
	service, store := recoveryCandidateFixture(t)
	rows, err := service.RecoveryTasks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	sort.Strings(ids)
	if fmt.Sprint(ids) != "[local-queued stale-board stale-doing]" {
		t.Fatal("incorrect reconciliation candidates", ids)
	}
	if store.fullLists != 0 || store.latestReads != 0 {
		t.Fatal("selection loaded all task history")
	}
}

func TestRecoverySweepsDoNotReadThousandsOfFinishedAttempts(t *testing.T) {
	for _, kind := range []string{"startup", "local leases", "zombies"} {
		t.Run(kind, func(t *testing.T) {
			service, store := recoveryCandidateFixture(t)
			ctx := context.Background()
			var err error
			switch kind {
			case "startup":
				err = service.RecoverStartupLocalAttempts(ctx)
			case "local leases":
				err = service.RecoverLocalAttempts(ctx)
			case "zombies":
				err = service.ReconcileZombieAttempts(ctx, time.Hour, func(string) bool { return true })
			}
			if err != nil {
				t.Fatal(err)
			}
			if store.fullLists != 0 || store.latestReads > 12 {
				t.Fatalf("history scan: full lists=%d latest reads=%d", store.fullLists, store.latestReads)
			}
			saved, _ := store.GetTask(ctx, "done-00001")
			running, _ := store.GetAttempt(ctx, "att-stale-board")
			if saved.Status != TaskStatusDone || running.Status != AttemptStatusRunning {
				t.Fatal("reconciliation changed settled work or live process")
			}
		})
	}
}
func TestEmptyRecoverySelectionNeverFallsBackToAllTasks(t *testing.T) {
	store := &countedRecoveryStore{MemoryStore: NewMemoryStore()}
	store.CreateTask(context.Background(), &Task{ID: "done", Status: TaskStatusDone})
	rows, err := NewService(store, nil).RecoveryTasks(context.Background())
	if err != nil || len(rows) != 0 || store.fullLists != 0 {
		t.Fatal("empty candidates became an unrestricted list")
	}
}
