package runner

import (
	"context"
	"encoding/json"
	"github.com/buchin/cuan/internal/task"
	"os"
	"os/exec"
	"path/filepath"
	"time"
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
	cache := os.Getenv("XDG_CACHE_HOME")
	if cache == "" {
		var err error
		cache, err = os.UserCacheDir()
		if err != nil {
			return ""
		}
	}
	env["PLAYWRIGHT_BROWSERS_PATH"] = filepath.Join(cache, "ms-playwright")
	env["CUAN_BROWSER_CLI"] = cli
	env["CUAN_BROWSER_MODULE"] = module
	if !requiresVision {
		return ""
	}
	return "\n\nNative screenshot browser is available through CUAN_BROWSER_CLI (official Playwright CLI) and CUAN_BROWSER_MODULE (Node require path). When the original outcome requires browser observations, use this existing browser instead of searching the filesystem or installing another browser. Capture the actual application at any originally required viewports; readiness checks and HTML mockups are not task evidence. Write outputs to CUAN_TASK_ARTIFACTS_DIR and attach session:PATH. Use isolated fixture data where appropriate; never borrow an ambient browser login or user credentials. Keep verification servers and caches outside a read-only repository. Static image inspection guidance: If the task only requires inspecting, analyzing, or comparing existing image files or attachments already present on disk, inspect them directly using model vision (such as read_file) instead of launching a browser or Chromium.\n"
}

// A real render is cheap and independent of provider accounts. Probe results
// are cached asynchronously so heartbeat lease renewal never waits for Chromium.
func probeBrowserRuntime(parent context.Context) *task.VerificationRuntime {
	env := map[string]string{}
	browserRuntimeEnvironment(env, false)
	module := env["CUAN_BROWSER_MODULE"]
	if module == "" {
		return nil
	}
	node, err := exec.LookPath("node")
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	script := `const p=require(process.argv[1]);const crypto=require('crypto');(async()=>{const browser=await p.chromium.launch({headless:true});try{const page=await browser.newPage({viewport:{width:390,height:844}});await page.setContent('<html><body>Browser verification</body></html>');const png=await page.screenshot();if(png.length<100)throw Error();console.log(JSON.stringify({version:require(process.argv[1]+'/package.json').version,image:crypto.createHash('sha256').update(png).digest('hex')}))}finally{await browser.close()}})().catch(()=>process.exit(1))`
	cmd := exec.CommandContext(ctx, node, "-e", script, module)
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	var out boundedFingerprintOutput
	cmd.Stdout = &out
	if cmd.Run() != nil || out.overflow {
		return nil
	}
	var result struct {
		Version string `json:"version"`
		Image   string `json:"image"`
	}
	if json.Unmarshal(out.data, &result) != nil || len(result.Image) != 64 || result.Version == "" {
		return nil
	}
	evidence := &task.VerificationRuntime{Fingerprint: task.VerificationSummaryHash("browser-render:v1:" + result.Version + ":" + result.Image), Version: result.Version, CheckedAt: time.Now().UTC()}
	if !evidence.Fresh(time.Now()) {
		return nil
	}
	return evidence
}

const browserFullProbeInterval = 1 * time.Hour
const browserCheckInterval = 3 * time.Minute

func quickVerifyBrowserRuntime(expectedVersion string) bool {
	env := map[string]string{}
	browserRuntimeEnvironment(env, false)
	module := env["CUAN_BROWSER_MODULE"]
	if module == "" {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(module, "package.json"))
	if err != nil {
		return false
	}
	var pkg struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if json.Unmarshal(raw, &pkg) != nil || pkg.Name != "playwright-core" || pkg.Version != expectedVersion {
		return false
	}
	return true
}

func (r *Runner) browserRuntimeEvidence(ctx context.Context) *task.VerificationRuntime {
	r.browserMu.Lock()
	defer r.browserMu.Unlock()
	now := time.Now()

	// If verified evidence exists and was confirmed recently by full probe,
	// fast-path: verify package integrity without re-launching Chromium.
	if r.browserEvidence != nil && !r.browserLastFullProbe.IsZero() && now.Sub(r.browserLastFullProbe) < browserFullProbeInterval {
		if quickVerifyBrowserRuntime(r.browserEvidence.Version) {
			r.browserEvidence.CheckedAt = now.UTC()
			copy := *r.browserEvidence
			return &copy
		}
		// Browser uninstalled or corrupted; invalidate cache
		r.browserEvidence = nil
	}

	if !r.browserProbing && !now.Before(r.browserNextProbe) {
		r.browserProbing = true
		r.browserNextProbe = now.Add(browserCheckInterval)
		go func() {
			evidence := probeBrowserRuntime(ctx)
			r.browserMu.Lock()
			r.browserEvidence = evidence
			if evidence != nil {
				r.browserLastFullProbe = time.Now()
			}
			r.browserProbing = false
			r.browserMu.Unlock()
		}()
	}
	if !r.browserEvidence.Fresh(now) {
		return nil
	}
	copy := *r.browserEvidence
	return &copy
}
func (s *RunnerService) BrowserRuntime(ctx context.Context, runnerID string) *task.VerificationRuntime {
	info, err := s.store.GetRunner(ctx, runnerID)
	if err != nil || info == nil || info.LastHeartbeatAt.Before(time.Now().Add(-45*time.Second)) || info.LastHeartbeatAt.After(time.Now().Add(5*time.Second)) {
		return nil
	}
	evidence := info.RuntimeEvidence.BrowserRuntime
	if !evidence.Fresh(time.Now()) {
		return nil
	}
	copy := *evidence
	return &copy
}
