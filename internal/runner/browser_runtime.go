package runner

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
)

// Browser binaries are platform tooling outside task repositories and private
// AI account profiles. Setup performs the real browser rendering readiness check.
func browserRuntimeEnvironment(env map[string]string, requiresVision bool) string {
	cli, err := exec.LookPath("cuan-browser")
	if err != nil {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(cli)
	if err != nil || filepath.Base(resolved) != "cli.js" {
		return ""
	}
	module := filepath.Dir(resolved)
	raw, err := os.ReadFile(filepath.Join(module, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(raw, &pkg) != nil || pkg.Name != "playwright-core" {
		return ""
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	env["PLAYWRIGHT_BROWSERS_PATH"] = filepath.Join(cache, "ms-playwright")
	env["CUAN_BROWSER_CLI"] = cli
	env["CUAN_BROWSER_MODULE"] = module
	if !requiresVision {
		return ""
	}
	return "\n\nNative screenshot browser is available through CUAN_BROWSER_CLI (official Playwright CLI) and CUAN_BROWSER_MODULE (Node require path). Use this existing browser instead of searching the filesystem or installing another browser. Render the actual changed application at the required viewports; readiness checks and HTML mockups are not task evidence. Write outputs to CUAN_TASK_ARTIFACTS_DIR and attach session:PATH. Use isolated fixture data where appropriate; never borrow an ambient browser login or user credentials. Keep verification servers and caches outside a read-only repository.\n"
}
