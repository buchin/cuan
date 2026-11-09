package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestScheduler_RegistrationAndListing(t *testing.T) {
	s := New()

	var runCount int32
	job := &Job{
		ID:           "test_job",
		Name:         "Test Job",
		ScheduleType: ScheduleInterval,
		Interval:     10 * time.Minute,
		Enabled:      true,
		Handler: func(ctx context.Context) error {
			atomic.AddInt32(&runCount, 1)
			return nil
		},
	}

	if err := s.Register(job); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	jobs := s.ListJobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	j, ok := s.GetJob("test_job")
	if !ok || j.Name != "Test Job" {
		t.Fatalf("expected to get test_job, got %v", j)
	}

	if j.NextRunAt == nil {
		t.Fatalf("expected NextRunAt to be calculated")
	}
}

func TestScheduler_TriggerSuccessAndFailure(t *testing.T) {
	s := New()

	var executed bool
	jobSuccess := &Job{
		ID:           "job_success",
		Name:         "Success",
		ScheduleType: ScheduleInterval,
		Interval:     1 * time.Hour,
		Handler: func(ctx context.Context) error {
			executed = true
			return nil
		},
	}

	jobFail := &Job{
		ID:           "job_fail",
		Name:         "Fail",
		ScheduleType: ScheduleInterval,
		Interval:     1 * time.Hour,
		Handler: func(ctx context.Context) error {
			return errors.New("boom")
		},
	}

	_ = s.Register(jobSuccess)
	_ = s.Register(jobFail)

	ctx := context.Background()
	if err := s.Trigger(ctx, "job_success"); err != nil {
		t.Fatalf("unexpected trigger error: %v", err)
	}
	if !executed {
		t.Fatalf("expected handler to execute")
	}

	j, _ := s.GetJob("job_success")
	if j.LastStatus != StatusSuccess {
		t.Fatalf("expected StatusSuccess, got %s", j.LastStatus)
	}

	err := s.Trigger(ctx, "job_fail")
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected boom error, got %v", err)
	}

	jFail, _ := s.GetJob("job_fail")
	if jFail.LastStatus != StatusFailed || jFail.LastError != "boom" {
		t.Fatalf("expected StatusFailed with error boom, got %s / %s", jFail.LastStatus, jFail.LastError)
	}
}

func TestScheduler_PanicRecovery(t *testing.T) {
	s := New()

	jobPanic := &Job{
		ID:           "job_panic",
		Name:         "Panic Job",
		ScheduleType: ScheduleInterval,
		Interval:     1 * time.Hour,
		Handler: func(ctx context.Context) error {
			panic("unexpected crash")
		},
	}

	_ = s.Register(jobPanic)

	err := s.Trigger(context.Background(), "job_panic")
	if err == nil {
		t.Fatalf("expected error from panic, got nil")
	}

	j, _ := s.GetJob("job_panic")
	if j.LastStatus != StatusFailed {
		t.Fatalf("expected StatusFailed, got %s", j.LastStatus)
	}
}

func TestCalculateNextRun_DailyAndWeekly(t *testing.T) {
	// Reference: 2026-10-27 10:00 (Tuesday)
	ref := time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)

	// Daily at 03:00 -> already passed today, should be tomorrow 2026-10-28 03:00
	jobDailyPast := &Job{
		ScheduleType: ScheduleDaily,
		TimeOfDay:    "03:00",
	}
	nextDaily := calculateNextRun(jobDailyPast, ref)
	if nextDaily.Day() != 28 || nextDaily.Hour() != 3 {
		t.Errorf("expected daily next run 2026-10-28 03:00, got %v", nextDaily)
	}

	// Daily at 15:30 -> later today 2026-10-27 15:30
	jobDailyFuture := &Job{
		ScheduleType: ScheduleDaily,
		TimeOfDay:    "15:30",
	}
	nextFuture := calculateNextRun(jobDailyFuture, ref)
	if nextFuture.Day() != 27 || nextFuture.Hour() != 15 || nextFuture.Minute() != 30 {
		t.Errorf("expected daily next run 2026-10-27 15:30, got %v", nextFuture)
	}

	// Weekly on Wednesday (DayOfWeek = 3) at 09:00 -> tomorrow 2026-10-28 09:00
	jobWeeklyWed := &Job{
		ScheduleType: ScheduleWeekly,
		DayOfWeek:    3,
		TimeOfDay:    "09:00",
	}
	nextWeekly := calculateNextRun(jobWeeklyWed, ref)
	if nextWeekly.Day() != 28 || nextWeekly.Hour() != 9 {
		t.Errorf("expected weekly Wednesday 2026-10-28 09:00, got %v", nextWeekly)
	}
}

func TestEventRequestsCoalesceWithoutPostponingDueWork(t *testing.T) {
	s := New()
	calls := 0
	if err := s.Register(&Job{ID: "maintenance", Enabled: true, ScheduleType: ScheduleInterval, Interval: time.Hour, Handler: func(context.Context) error { calls++; return nil }}); err != nil {
		t.Fatal(err)
	}
	if err := s.Request("maintenance", time.Second); err != nil {
		t.Fatal(err)
	}
	first, _ := s.GetJob("maintenance")
	for i := 0; i < 100; i++ {
		if err := s.Request("maintenance", time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	after, _ := s.GetJob("maintenance")
	if !after.NextRunAt.Equal(*first.NextRunAt) || calls != 0 {
		t.Fatal("request burst postponed work or executed inline")
	}
	if err := s.Trigger(context.Background(), "maintenance"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("coalesced job did not run once", calls)
	}
	if err := s.Request("missing", 0); err == nil {
		t.Fatal("missing job accepted")
	}
}
