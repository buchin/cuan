package server

import (
	"testing"
	"time"
)

func TestInboxRequestExecutionDeadlineHonorsDurableLifetime(t *testing.T) {
	now := time.Now()
	for _, c := range []struct {
		name    string
		durable time.Time
		cap     time.Duration
		want    time.Time
	}{
		{"claimed six-minute request", now.Add(6 * time.Minute), 10 * time.Minute, now.Add(6 * time.Minute)},
		{"configured shorter cap", now.Add(6 * time.Minute), time.Minute, now.Add(time.Minute)},
		{"already expired", now.Add(-time.Second), 10 * time.Minute, now.Add(-time.Second)},
		{"legacy missing deadline", time.Time{}, 10 * time.Minute, now.Add(10 * time.Minute)},
	} {
		if got := inboxRequestExecutionDeadline(now, c.durable, c.cap); !got.Equal(c.want) {
			t.Fatal(c.name, got, c.want)
		}
	}
}
