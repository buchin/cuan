package server

import (
	"context"
	"github.com/buchin/cuan/internal/db"
	"testing"
	"time"
)

func TestInboxCapacityBackoffKeepsSameRequestAndStopsAtBound(t *testing.T) {
	now := time.Now()
	q := db.InboxRequest{State: "queued", SafeError: inboxQuotaNoBackup}
	for attempt, want := range []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 5 * time.Minute, 5 * time.Minute} {
		q.Steps = append(q.Steps, db.InboxStep{Kind: "finished", At: now})
		if got := inboxCapacityRetryDelay(q.Steps); got != want {
			t.Fatalf("attempt%d: got%v want%v", attempt+1, got, want)
		}
		if !inboxCapacityWaiting(q, now.Add(want-time.Millisecond)) || inboxCapacityWaiting(q, now.Add(want)) {
			t.Fatal("retry boundary incorrect")
		}
	}
	q.State = "interrupted"
	if inboxCapacityWaiting(q, now) {
		t.Fatal("uncertain request treated as safe capacity retry")
	}
}

func TestInboxCapacityBackoffSurvivesSnapshotCompaction(t *testing.T) {
	ctx := context.Background()
	repo := db.NewMemoryProjectStudioRepository()
	now := time.Now()
	q := db.InboxRequest{RoomID: "room", RequestID: "request", MessageID: "message", State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	repo.AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID})
	for cycle := 0; cycle < 12; cycle++ {
		q, _, _ = repo.ClaimInboxRequest(ctx, q.RoomID, q.RequestID)
		for i := 0; i < 130; i++ {
			repo.AppendInboxStep(ctx, q.RoomID, q.RequestID, "context", "stage")
		}
		q.State = "queued"
		q.SafeError = inboxQuotaNoBackup
		repo.FinishInboxRequest(ctx, q, nil)
		rows, _ := repo.ListInboxRequests(ctx, q.RoomID)
		q = rows[0]
		want := 30 * time.Second * time.Duration(1<<min(cycle, 4))
		if want > 5*time.Minute {
			want = 5 * time.Minute
		}
		if len(q.Steps) > 100 || inboxCapacityRetryDelay(q.Steps) != want || !inboxCapacityWaiting(q, time.Now()) {
			t.Fatalf("backoff changed at cycle %d", cycle)
		}
	}
}
