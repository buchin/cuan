package server

import (
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
