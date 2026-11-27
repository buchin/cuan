package server

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/scheduler"
)

const storageMaintenanceJob = "storage_maintenance"

var installedStorageMaintenanceMu sync.Mutex

func (s *Server) registerStorageMaintenance() {
	if s.scheduler == nil {
		return
	}
	if s.cfg != nil && (s.cfg.Environment == "testing" || s.cfg.Environment == "test") {
		s.registerStorageMaintenanceHandler(true, func(context.Context) error { return nil })
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
		now := time.Now()
		s.markAcceptedVerificationSnapshots(ctx)
		workspaceCursor = s.runWorkspaceRetention(ctx, workspaceCursor, now)
		s.pruneIdleStorageArtifacts(now)
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
	if !installedStorageMaintenanceMu.TryLock() {
		// A maintenance service start is already underway or active; skip duplicate trigger.
		return nil
	}
	defer installedStorageMaintenanceMu.Unlock()

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

func (s *Server) pruneIdleStorageArtifacts(now time.Time) {
	if s.cfg != nil {
		if s.cfg.WorkspacesDir != "" {
			pruneOldArtifactDirs(filepath.Join(s.cfg.WorkspacesDir, "verification"), 24*time.Hour, 5, now)
		}
		if s.cfg.ProjectsDir != "" {
			pruneOldArtifactDirs(filepath.Join(s.cfg.ProjectsDir, "verification-snapshots"), 24*time.Hour, 5, now)
			pruneOldArtifactDirs(filepath.Join(s.cfg.ProjectsDir, "release-workspaces"), 24*time.Hour, 3, now)
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		pruneProviderAccountArtifacts(filepath.Join(home, ".local", "share", "cuan", "accounts"), 48*time.Hour, 5, now)
	}
}

func pruneOldArtifactDirs(root string, maxAge time.Duration, keepNewest int, now time.Time) int {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0
	}
	type dirItem struct {
		path    string
		modTime time.Time
	}
	var items []dirItem
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		items = append(items, dirItem{path: filepath.Join(root, name), modTime: info.ModTime()})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].modTime.After(items[j].modTime)
	})
	removed := 0
	for i, item := range items {
		if i < keepNewest || now.Sub(item.modTime) < maxAge {
			continue
		}
		if os.RemoveAll(item.path) == nil {
			removed++
		}
	}
	return removed
}

func pruneProviderAccountArtifacts(accountsRoot string, maxAge time.Duration, keepNewest int, now time.Time) int {
	accounts, err := os.ReadDir(accountsRoot)
	if err != nil {
		return 0
	}
	removed := 0
	for _, acct := range accounts {
		if !acct.IsDir() || !strings.HasPrefix(acct.Name(), "acct_") {
			continue
		}
		cliRoot := filepath.Join(accountsRoot, acct.Name(), "home", ".gemini", "antigravity-cli")
		for _, sub := range []string{"conversations", "brain"} {
			removed += pruneOldArtifactDirs(filepath.Join(cliRoot, sub), maxAge, keepNewest, now)
		}
	}
	return removed
}
