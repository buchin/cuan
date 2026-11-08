package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Prepare locked PHP dependencies in this private workspace only. Never run
// Composer scripts/plugins or inherit provider, task, Git or Composer auth.
func preparePHPDependencies(parent context.Context, root string) error {
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
			VendorDir string `json:"vendor-dir"`
		} `json:"config"`
		Repositories json.RawMessage `json:"repositories"`
	}
	if err := read("composer.json", &manifest); err != nil {
		return err
	}
	if manifest.Config.VendorDir != "" && manifest.Config.VendorDir != "vendor" {
		return fmt.Errorf("automatic PHP setup requires the private vendor directory")
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
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "composer", "--no-plugins", "--no-scripts", "install", "--prefer-dist", "--no-interaction", "--no-progress")
	cmd.Dir = root
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "COMPOSER_HOME=" + home, "COMPOSER_CACHE_DIR=" + filepath.Join(home, "cache"), "COMPOSER_NO_INTERACTION=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0"}
	var output verifierOutputTail
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: PHP locked dependency preparation: %s", InspectionDependencyFailurePrefix, nativeReleaseDiagnostic(output.String(), nil))
	}
	return nil
}
