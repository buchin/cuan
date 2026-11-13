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

	"github.com/buchin/cuan/internal/task"
)

const InspectionDependencyFailurePrefix = task.InspectionDependencyFailurePrefix

// Prepare only known frontend roots in an owned private workspace. npm
// receives no task capability, provider credentials, user npm config or scripts.
// Source identity is revalidated by the caller after preparation.
func prepareInspectionDependencies(parent context.Context, root string, managedCache ...string) error {
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
		if !filepath.IsAbs(directory) {
			return fmt.Errorf("verification dependency directory must be private")
		}
		dirInfo, err := os.Lstat(directory)
		if err != nil || !dirInfo.IsDir() || dirInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("verification dependency directory must be private")
		}
		resolved, err := filepath.EvalSymlinks(directory)
		if err != nil {
			return fmt.Errorf("verification dependency directory must be private")
		}
		parentResolved, err := filepath.EvalSymlinks(filepath.Dir(directory))
		if err != nil || resolved != filepath.Join(parentResolved, filepath.Base(directory)) {
			return fmt.Errorf("verification dependency directory must be private")
		}
		manifestPath := filepath.Join(directory, "package.json")
		manifest, err := os.Lstat(manifestPath)
		if err != nil || !manifest.Mode().IsRegular() || manifest.Size() > 8<<20 {
			return fmt.Errorf("verification dependency manifest must be a regular file")
		}
		manifestRaw, err := os.ReadFile(manifestPath)
		if err != nil {
			return fmt.Errorf("verification dependency manifest must be readable")
		}
		var manifestParsed map[string]any
		if json.Unmarshal(manifestRaw, &manifestParsed) != nil {
			return fmt.Errorf("verification dependency manifest must be valid json")
		}
		modules, modulesErr := os.Lstat(filepath.Join(directory, "node_modules"))
		if modulesErr == nil && !modules.IsDir() {
			return fmt.Errorf("verification dependencies must not use shared links")
		}
		ignored := exec.CommandContext(parent, "git", "-C", directory, "check-ignore", "--quiet", "--", "node_modules/")
		if ignored.Run() != nil {
			return fmt.Errorf("verification dependency output must be ignored by the repository")
		}
		lockFingerprint := dependencyLockFingerprint(manifestRaw, raw)
		if modulesErr == nil && modules.IsDir() {
			if checkLockfileWarm(directory, "node_modules", lockFingerprint) {
				continue
			}
		}
		privateHome, err := os.MkdirTemp("", "cuan-verification-npm-")
		if err != nil {
			return fmt.Errorf("prepare private dependency environment: %w", err)
		}
		ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
		cache := filepath.Join(privateHome, "cache")
		if len(managedCache) > 0 && managedCache[0] != "" {
			cache = managedCache[0]
		}
		cmd := exec.CommandContext(ctx, "npm", "ci", "--ignore-scripts", "--no-audit", "--no-fund", "--prefer-offline", "--userconfig", filepath.Join(privateHome, "user.npmrc"), "--globalconfig", filepath.Join(privateHome, "global.npmrc"))
		cmd.Dir = directory
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + privateHome, "CI=true", "NPM_CONFIG_CACHE=" + cache}
		var output verifierOutputTail
		cmd.Stdout = &output
		cmd.Stderr = &output
		err = cmd.Run()
		cancel()
		_ = os.RemoveAll(privateHome)
		if err != nil {
			return fmt.Errorf("%s: %s", InspectionDependencyFailurePrefix, nativeReleaseDiagnostic(output.String(), nil))
		}
		if recErr := recordLockfileFingerprint(directory, "node_modules", lockFingerprint); recErr != nil {
			return fmt.Errorf("record dependency fingerprint: %w", recErr)
		}
	}
	return nil
}
