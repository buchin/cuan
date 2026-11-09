package server

import (
	"context"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/scheduler"
)

func TestStorageMaintenanceUsesExistingSchedulerAndCompletedEventBus(t *testing.T) {
	s := NewServer(&config.Config{Environment: "testing"}, nil)
	defer s.terminalManager.CloseAll()
	bus := runner.NewEventBus()
	s.runnerSvc.SetEventBus(bus)
	calls := 0
	s.registerStorageMaintenanceHandler(true, func(context.Context) error { calls++; return nil })
	before, _ := s.scheduler.GetJob(storageMaintenanceJob)
	bus.PublishCompleted(context.Background(), runner.AttemptCompletedEvent{Status: "failed"})
	deadline := time.Now().Add(time.Second)
	for {
		job, _ := s.scheduler.GetJob(storageMaintenanceJob)
		if job.NextRunAt.Before(*before.NextRunAt) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("completion event did not request maintenance")
		}
		time.Sleep(time.Millisecond)
	}
	scheduled, _ := s.scheduler.GetJob(storageMaintenanceJob)
	for i := 0; i < 100; i++ {
		s.requestStorageMaintenance()
	}
	after, _ := s.scheduler.GetJob(storageMaintenanceJob)
	if !after.NextRunAt.Equal(*scheduled.NextRunAt) || calls != 0 {
		t.Fatal("events started parallel cleanup or postponed request")
	}
	if err := s.scheduler.Trigger(context.Background(), storageMaintenanceJob); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("existing scheduler did not execute worker")
	}
	if after.Interval != 15*time.Minute || after.ScheduleType != scheduler.ScheduleInterval {
		t.Fatal("periodic recovery missing")
	}
}
