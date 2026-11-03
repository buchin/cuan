package server

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/git"
)

// runTaskWorkspaceGC prunes stale task workspaces under data/projects/task-workspaces.
// It removes workspaces older than 48 hours and caps total workspaces at 10.
func (s *Server) runTaskWorkspaceGC() {
	base := filepath.Join("data", "projects")
	if s.cfg.ProjectsDir != "" {
		base = s.cfg.ProjectsDir
	}
	workspacesBase := filepath.Join(base, "task-workspaces")

	gc := git.NewClient()
	pruned, err := gc.PruneStaleWorkspaces(workspacesBase, 48*time.Hour, "", 10)
	if err != nil {
		log.Printf("[gc] task workspace prune error: %v", err)
		return
	}
	if pruned > 0 {
		log.Printf("[gc] task workspace GC: pruned %d stale workspace(s)", pruned)
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
