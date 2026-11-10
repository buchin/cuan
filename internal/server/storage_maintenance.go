package server

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"time"

	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/scheduler"
)

const storageMaintenanceJob = "storage_maintenance"

func (s *Server) registerStorageMaintenance() {
	if s.scheduler == nil {
		return
	}
	// Native installation supplies this one fixed maintenance worker. Other hosts
	// retain their own maintenance setup rather than acquiring new privileges.
	_, err := os.Stat("/etc/systemd/system/cuan-runtime-cache-maintenance.service")
	if err == nil {
		s.registerStorageMaintenanceHandler(true, runInstalledStorageMaintenance)
	} else {
		// Repo-copy cleanup is native Go and available on ordinary OSS installs.
		// Privileged cache maintenance remains optional and installation-owned.
		s.registerStorageMaintenanceHandler(true, func(context.Context) error { return nil })
	}
}

func (s *Server) registerStorageMaintenanceHandler(enabled bool, handler scheduler.HandlerFunc) {
	workspaceCursor := 0
	_ = s.scheduler.Register(&scheduler.Job{ID: storageMaintenanceJob, Name: "Storage maintenance", Description: "Reclaim idle generated caches while preserving active work and saved results", ScheduleType: scheduler.ScheduleInterval, Interval: 15 * time.Minute, Timeout: 220 * time.Second, Enabled: enabled, Handler: func(ctx context.Context) error {
		s.markAcceptedVerificationSnapshots(ctx)
		workspaceCursor = s.runWorkspaceRetention(ctx, workspaceCursor, time.Now())
		return handler(ctx)
	}})
	bus := runner.GlobalEventBus
	if s.runnerSvc != nil && s.runnerSvc.EventBus() != nil {
		bus = s.runnerSvc.EventBus()
	}
	if bus != nil {
		bus.SubscribeCompleted(func(_ context.Context, _ runner.AttemptCompletedEvent) error {
			s.requestStorageMaintenance()
			return nil
		})
	}
}

func (s *Server) requestStorageMaintenance() {
	if s.scheduler != nil {
		_ = s.scheduler.Request(storageMaintenanceJob, 10*time.Second)
	}
}

func runInstalledStorageMaintenance(ctx context.Context) error {
	// Fixed argv only. The native installer grants precisely this service action;
	// no task/provider paths, environment, shell or agent prose enter the command.
	command := exec.CommandContext(ctx, "/usr/bin/sudo", "-n", "/usr/bin/systemctl", "start", "cuan-runtime-cache-maintenance.service")
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	command.Dir = "/"
	if err := command.Run(); err != nil {
		return errors.New("Storage maintenance did not finish successfully; inspect the installed maintenance service")
	}
	return nil
}
