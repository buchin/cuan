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
	return "\n\nNative screenshot browser is available through CUAN_BROWSER_CLI (official Playwright CLI) and CUAN_BROWSER_MODULE (Node require path). When the original outcome requires browser observations, use this existing browser instead of searching the filesystem or installing another browser. Capture the actual application at any originally required viewports; readiness checks and HTML mockups are not task evidence. Write outputs to CUAN_TASK_ARTIFACTS_DIR and attach session:PATH. Use isolated fixture data where appropriate; never borrow an ambient browser login or user credentials. Keep verification servers and caches outside a read-only repository.\n"
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
func (r *Runner) browserRuntimeEvidence(ctx context.Context) *task.VerificationRuntime {
	r.browserMu.Lock()
	defer r.browserMu.Unlock()
	now := time.Now()
	if !r.browserProbing && !now.Before(r.browserNextProbe) {
		r.browserProbing = true
		r.browserNextProbe = now.Add(5 * time.Minute)
		go func() {
			evidence := probeBrowserRuntime(ctx)
			r.browserMu.Lock()
			r.browserEvidence = evidence
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
