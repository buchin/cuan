package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Prepare only known frontend roots in the owned verification snapshot. npm
// receives no task capability, provider credentials, user npm config or scripts.
// Source identity is revalidated by the caller after preparation.
func prepareInspectionDependencies(parent context.Context, root string) error {
	for _, relative := range []string{".", "web"} {
		directory := filepath.Join(root, relative)
		lock := filepath.Join(directory, "package-lock.json")
		info, err := os.Lstat(lock)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() || info.Size() > 8<<20 {
			return fmt.Errorf("verification dependency lock must be a regular file")
		}
		raw, err := os.ReadFile(lock)
		var dependencyLock struct {
			Version  int `json:"lockfileVersion"`
			Packages map[string]struct {
				Resolved string `json:"resolved"`
				Link     bool   `json:"link"`
				Bundled  bool   `json:"inBundle"`
			} `json:"packages"`
		}
		if err != nil || json.Unmarshal(raw, &dependencyLock) != nil || dependencyLock.Version < 2 || dependencyLock.Packages == nil {
			return fmt.Errorf("verification requires a modern npm dependency lock")
		}
		for name, dependency := range dependencyLock.Packages {
			if name == "" {
				continue
			}
			endpoint, err := url.Parse(dependency.Resolved)
			if dependency.Link || (!dependency.Bundled && (err != nil || endpoint.Scheme != "https" || endpoint.Host == "" || endpoint.User != nil)) {
				return fmt.Errorf("automatic verification setup requires credential-free HTTPS package sources")
			}
		}
		resolved, err := filepath.EvalSymlinks(directory)
		if err != nil || resolved != filepath.Clean(directory) {
			return fmt.Errorf("verification dependency directory must be private")
		}
		manifest, err := os.Lstat(filepath.Join(directory, "package.json"))
		if err != nil || !manifest.Mode().IsRegular() {
			return fmt.Errorf("verification dependency manifest must be a regular file")
		}
		if modules, err := os.Lstat(filepath.Join(directory, "node_modules")); err == nil && !modules.IsDir() {
			return fmt.Errorf("verification dependencies must not use shared links")
		}
		ignored := exec.CommandContext(parent, "git", "-C", directory, "check-ignore", "--quiet", "--", "node_modules/")
		if ignored.Run() != nil {
			return fmt.Errorf("verification dependency output must be ignored by the repository")
		}
		privateHome, err := os.MkdirTemp("", "cuan-verification-npm-")
		if err != nil {
			return fmt.Errorf("prepare private dependency environment: %w", err)
		}
		ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
		cmd := exec.CommandContext(ctx, "npm", "ci", "--ignore-scripts", "--no-audit", "--no-fund", "--userconfig", filepath.Join(privateHome, "user.npmrc"), "--globalconfig", filepath.Join(privateHome, "global.npmrc"))
		cmd.Dir = directory
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + privateHome, "CI=true", "NPM_CONFIG_CACHE=" + filepath.Join(privateHome, "cache")}
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		err = cmd.Run()
		cancel()
		_ = os.RemoveAll(privateHome)
		if err != nil {
			return fmt.Errorf("locked frontend dependency preparation failed; saved source is preserved")
		}
	}
	return nil
}
