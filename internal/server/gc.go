package server

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/taskprocess"
)

func diskUsedPercent(path string) (float64, bool) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil || stat.Blocks == 0 {
		return 0, false
	}
	used := float64(stat.Blocks-stat.Bavail) / float64(stat.Blocks) * 100.0
	return used, true
}

// runTaskWorkspaceGC prunes stale task workspaces, reconstructed workspaces,
// release workspaces, and merged cuan/* branch refs.
func (s *Server) runTaskWorkspaceGC() {
	base := filepath.Join("data", "projects")
	if s.cfg.ProjectsDir != "" {
		base = s.cfg.ProjectsDir
	}
	workspacesBase := filepath.Join(base, "task-workspaces")

	taskMaxAge := 48 * time.Hour
	reconstructedMaxAge := 7 * 24 * time.Hour
	batchLimit := 10
	releaseKeep := 5
	if usedPct, ok := diskUsedPercent(base); ok && usedPct > 80.0 {
		taskMaxAge = 6 * time.Hour
		reconstructedMaxAge = 24 * time.Hour
		batchLimit = 25
		releaseKeep = 3
		log.Printf("[gc] disk usage %.1f%% > 80%%: activating aggressive workspace GC", usedPct)
	}

	gc := git.NewClient()
	pruned, err := gc.PruneStaleWorkspaces(workspacesBase, taskMaxAge, "", batchLimit)
	if err != nil {
		log.Printf("[gc] task workspace prune error: %v", err)
	} else if pruned > 0 {
		log.Printf("[gc] task workspace GC: pruned %d stale workspace(s)", pruned)
	}

	reconstructedBase := filepath.Join(base, "reconstructed-workspaces")
	if rPruned, rErr := gc.PruneReconstructedWorkspaces(reconstructedBase, reconstructedMaxAge, batchLimit); rErr != nil {
		log.Printf("[gc] reconstructed workspace prune error: %v", rErr)
	} else if rPruned > 0 {
		log.Printf("[gc] reconstructed workspace GC: pruned %d stale workspace(s)", rPruned)
	}

	s.runReleaseWorkspaceGC(filepath.Join(base, "release-workspaces"), releaseKeep)

	if s.projectSvc != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if projects, pErr := s.projectSvc.List(ctx); pErr == nil {
			for _, p := range projects {
				if p == nil || strings.TrimSpace(p.LocalPath) == "" {
					continue
				}
				if bPruned, bErr := gc.PruneMergedCuanBranches(ctx, p.LocalPath, p.DefaultBranch); bErr != nil {
					log.Printf("[gc] branch prune error for %s: %v", p.ID, bErr)
				} else if bPruned > 0 {
					log.Printf("[gc] branch GC for %s: pruned %d merged cuan/* branch(es)", p.ID, bPruned)
				}
			}
		}
	}
}

// runReleaseWorkspaceGC rotates old release-workspaces directories, keeping the
// newest `keep` entries and skipping any workspace with an active verification lock
// or modified within the last hour.
func (s *Server) runReleaseWorkspaceGC(releaseWorkspacesDir string, keep int) {
	entries, err := os.ReadDir(releaseWorkspacesDir)
	if err != nil {
		return
	}
	type rwEntry struct {
		name    string
		path    string
		modTime time.Time
	}
	var dirs []rwEntry
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		info, iErr := e.Info()
		if iErr != nil {
			continue
		}
		dirs = append(dirs, rwEntry{
			name:    e.Name(),
			path:    filepath.Join(releaseWorkspacesDir, e.Name()),
			modTime: info.ModTime(),
		})
	}
	if len(dirs) <= keep {
		return
	}
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].modTime.After(dirs[j].modTime)
	})
	pruned := 0
	for _, d := range dirs[keep:] {
		if time.Since(d.modTime) < time.Hour {
			continue
		}
		lockFile := filepath.Join(d.path, ".git", "cuan-verification.lock")
		if active, aErr := taskprocess.Active(lockFile); aErr == nil && active {
			continue
		}
		if err := os.RemoveAll(d.path); err == nil {
			pruned++
		}
	}
	if pruned > 0 {
		log.Printf("[gc] release-workspaces GC: pruned %d old release workspace(s)", pruned)
	}
}

// runReleaseGC removes old release directories under /opt/cuan/releases, keeping
// the 3 most recently modified releases plus the currently active one.
func (s *Server) runReleaseGC() {
	releasesDir := "/opt/cuan/releases"
	if _, err := os.Stat(releasesDir); os.IsNotExist(err) {
		return // not a production deployment; skip
	}

	currentLink := "/opt/cuan/current"
	currentResolved := ""
	if resolved, err := filepath.EvalSymlinks(currentLink); err == nil {
		currentResolved = resolved
	}
	previousLink := "/opt/cuan/previous"
	previousResolved := ""
	if resolved, err := filepath.EvalSymlinks(previousLink); err == nil {
		previousResolved = resolved
	}

	entries, err := os.ReadDir(releasesDir)
	if err != nil {
		log.Printf("[gc] release GC: cannot read releases dir: %v", err)
		return
	}

	type release struct {
		name    string
		path    string
		modTime time.Time
	}
	var releases []release
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		p := filepath.Join(releasesDir, e.Name())
		// Never touch the active or previous release
		if p == currentResolved || p == previousResolved {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		releases = append(releases, release{name: e.Name(), path: p, modTime: info.ModTime()})
	}

	const keepCount = 3
	if len(releases) <= keepCount {
		return
	}

	// Sort newest first
	sort.Slice(releases, func(i, j int) bool {
		return releases[i].modTime.After(releases[j].modTime)
	})

	toDelete := releases[keepCount:]
	for _, r := range toDelete {
		if err := os.RemoveAll(r.path); err != nil {
			log.Printf("[gc] release GC: failed to remove %s: %v", r.name, err)
		} else {
			log.Printf("[gc] release GC: removed old release %s", r.name)
		}
	}
}

// runLogGC truncates oversized nginx and postgresql log files that exceed 200 MB.
// This is a last-resort safeguard; proper log rotation should be configured via logrotate.
const logGCMaxBytes = 200 * 1024 * 1024 // 200 MB

var logGCPaths = []string{
	"/var/log/nginx",
	"/var/log/postgresql",
}

func (s *Server) runLogGC() {
	for _, dir := range logGCPaths {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			// Only truncate active (non-rotated) log files
			if strings.Contains(name, ".gz") || strings.Contains(name, ".1") ||
				strings.Contains(name, ".2") || strings.Contains(name, ".3") {
				continue
			}
			p := filepath.Join(dir, name)
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.Size() > logGCMaxBytes {
				if err := os.Truncate(p, 0); err != nil {
					log.Printf("[gc] log GC: failed to truncate %s: %v", p, err)
				} else {
					log.Printf("[gc] log GC: truncated oversized log %s (was %.1f MB)", p, float64(info.Size())/(1024*1024))
				}
			}
		}
	}
}
