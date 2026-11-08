package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/buchin/cuan/internal/task"
)

// Prepare locked PHP dependencies in this private workspace only. Never run
// Composer scripts/plugins or inherit provider, task, Git or Composer auth.
func preparePHPDependencies(parent context.Context, root, cacheBase, owner string) error {
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil || resolved != filepath.Clean(root) {
		return fmt.Errorf("PHP dependency workspace must be private")
	}
	read := func(name string, value any) error {
		path := filepath.Join(root, name)
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 8<<20 {
			return fmt.Errorf("PHP dependency %s must be an available regular file", name)
		}
		raw, err := os.ReadFile(path)
		if err != nil || json.Unmarshal(raw, value) != nil {
			return fmt.Errorf("PHP dependency %s is invalid", name)
		}
		return nil
	}
	var manifest struct {
		Config struct {
			VendorDir     string `json:"vendor-dir"`
			CacheDir      string `json:"cache-dir"`
			CacheFilesDir string `json:"cache-files-dir"`
			CacheRepoDir  string `json:"cache-repo-dir"`
			CacheVCSDir   string `json:"cache-vcs-dir"`
		} `json:"config"`
		Repositories json.RawMessage `json:"repositories"`
	}
	if err := read("composer.json", &manifest); err != nil {
		return err
	}
	if manifest.Config.VendorDir != "" && manifest.Config.VendorDir != "vendor" {
		return fmt.Errorf("automatic PHP setup requires the private vendor directory")
	}
	if manifest.Config.CacheDir != "" || manifest.Config.CacheFilesDir != "" || manifest.Config.CacheRepoDir != "" || manifest.Config.CacheVCSDir != "" {
		return fmt.Errorf("automatic PHP setup requires managed private cache paths")
	}
	// Nonstandard repositories can require credentials or local/shared source.
	// Preserve them for explicit project-specific preparation, never guess auth.
	if len(manifest.Repositories) > 0 && string(manifest.Repositories) != "[]" && string(manifest.Repositories) != "{}" {
		return fmt.Errorf("automatic PHP setup requires project-specific preparation for custom repositories")
	}
	type dependency struct {
		Type string `json:"type"`
		Dist struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"dist"`
		Source struct {
			URL string `json:"url"`
		} `json:"source"`
	}
	var lock struct {
		Packages []dependency `json:"packages"`
		Dev      []dependency `json:"packages-dev"`
		Hash     string       `json:"content-hash"`
	}
	if err := read("composer.lock", &lock); err != nil {
		return err
	}
	if lock.Hash == "" || lock.Packages == nil {
		return fmt.Errorf("automatic PHP setup requires a complete Composer lock")
	}
	publicURL := func(raw string) bool {
		u, e := url.Parse(raw)
		return e == nil && u.Scheme == "https" && u.Host != "" && u.User == nil
	}
	for _, d := range append(lock.Packages, lock.Dev...) {
		if d.Type == "metapackage" {
			continue
		}
		if (d.Dist.Type != "zip" && d.Dist.Type != "tar") || !publicURL(d.Dist.URL) || (d.Source.URL != "" && !publicURL(d.Source.URL)) {
			return fmt.Errorf("automatic PHP setup requires credential-free HTTPS package archives")
		}
	}
	if info, err := os.Lstat(filepath.Join(root, "vendor")); err == nil && !info.IsDir() {
		return fmt.Errorf("PHP dependencies must not use shared links")
	}
	if exec.CommandContext(parent, "git", "-C", root, "check-ignore", "--quiet", "--", "vendor/").Run() != nil {
		return fmt.Errorf("PHP dependency output must be ignored by the repository")
	}
	if _, err := exec.LookPath("composer"); err != nil {
		return fmt.Errorf("%s: Composer executable is unavailable in the runner runtime", InspectionDependencyFailurePrefix)
	}
	home, err := os.MkdirTemp("", "cuan-verification-composer-")
	if err != nil {
		return fmt.Errorf("private Composer environment unavailable")
	}
	defer os.RemoveAll(home)
	if cacheBase == "" || owner == "" {
		return fmt.Errorf("private owner Composer cache unavailable")
	}
	directory, err := managedOwnerCacheDirectory(cacheBase, owner, []string{".", "composer-cache", "composer-cache/files"})
	if err != nil {
		return fmt.Errorf("private owner Composer cache unavailable")
	}
	cache := filepath.Join(directory, "composer-cache")
	lease, err := acquireBuildCacheLease(parent, cache)
	if err != nil {
		return fmt.Errorf("private Composer cache lease unavailable: %w", err)
	}
	defer lease.Close()
	// Let Composer enforce its own archive size/age limits as well as pressure
	// maintenance. Repository/VCS metadata stays disposable with the clean HOME.
	config, _ := json.Marshal(map[string]any{"config": map[string]any{
		"cache-files-maxsize": "256MiB", "cache-files-ttl": 15552000,
		"cache-repo-dir": filepath.Join(home, "repo"), "cache-vcs-dir": filepath.Join(home, "vcs"),
	}})
	if err := os.WriteFile(filepath.Join(home, "config.json"), config, 0600); err != nil {
		return err
	}
	budget := 4 * time.Minute
	if deadline, ok := parent.Deadline(); ok && time.Until(deadline)/2 < budget {
		budget = time.Until(deadline) / 2
	}
	ctx, cancel := context.WithTimeout(parent, budget)
	defer cancel()
	cmd := exec.CommandContext(ctx, "composer", "--no-plugins", "--no-scripts", "install", "--prefer-dist", "--no-interaction", "--no-progress")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = time.Second
	cmd.Dir = root
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "COMPOSER_HOME=" + home, "COMPOSER_CACHE_DIR=" + cache, "COMPOSER_NO_INTERACTION=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0"}
	var output verifierOutputTail
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		diagnosis := "installer failed"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			diagnosis = "setup deadline exceeded; downloaded archives retained for bounded recovery"
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			diagnosis = "setup canceled"
		}
		return fmt.Errorf("%s: PHP locked dependency preparation (%s): %s", InspectionDependencyFailurePrefix, diagnosis, nativeReleaseDiagnostic(output.String(), nil))
	}
	return nil
}

// Reports the preparation policy actually installed in this runner. This is
// readiness for a bounded setup recovery, never proof of installed dependencies.
func (r *Runner) phpPreparationEvidence() *task.DependencyPreparationRuntime {
	if r.cfg.WorkspacesDir == "" {
		return nil
	}
	if _, err := exec.LookPath("composer"); err != nil {
		return nil
	}
	if _, err := exec.LookPath("php"); err != nil {
		return nil
	}
	return &task.DependencyPreparationRuntime{Policy: task.PHPPreparationPolicy, CheckedAt: time.Now().UTC()}
}
func (s *RunnerService) DependencyPreparationRuntime(ctx context.Context, runnerID string) *task.DependencyPreparationRuntime {
	value, ok := s.dependencyPreparations.Load(runnerID)
	if !ok {
		return nil
	}
	evidence, ok := value.(task.DependencyPreparationRuntime)
	if !ok || !evidence.Fresh(time.Now()) {
		return nil
	}
	info, err := s.store.GetRunner(ctx, runnerID)
	if err != nil || info == nil || info.LastHeartbeatAt.Before(time.Now().Add(-45*time.Second)) {
		return nil
	}
	return &evidence
}

type dependencyPreparationClaimKey struct{}
