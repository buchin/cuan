package task

import (
	"context"
	"testing"
	"time"
)

func TestResultKindDoesNotLeakAcrossAttemptsOrReports(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	stamp := time.Now().UTC()
	attempt := "attempt1"
	for _, a := range []*TaskActivity{
		{ID: "kind", TaskID: "t", AttemptID: &attempt, ActivityType: "agent_result_kind", NewStatus: "answer", CreatedAt: stamp},
		{ID: "report", TaskID: "t", AttemptID: &attempt, ActivityType: "agent_report", NewStatus: "review", CreatedAt: stamp},
	} {
		if err := store.CreateActivity(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	got, err := store.LatestResultClassifications(ctx, []string{"t"})
	if err != nil || resultKindForAttempt(got["t"], &TaskAttempt{ID: attempt}) != "answer" {
		t.Fatal(got, err)
	}
	if resultKindForAttempt(got["t"], &TaskAttempt{ID: "new-attempt"}) != "" {
		t.Fatal("old answer leaked into new execution")
	}
	store.CreateActivity(ctx, &TaskActivity{ID: "later", TaskID: "t", AttemptID: &attempt, ActivityType: "agent_report", NewStatus: "review", CreatedAt: stamp.Add(time.Second)})
	got, err = store.LatestResultClassifications(ctx, []string{"t"})
	if err != nil || got["t"].Kind != "" {
		t.Fatal("stale classification reused for an untyped later report", got, err)
	}
}
