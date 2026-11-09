package browserfetch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// Explicit opt-in uses an independently installed official engine, not an
// ambient browser or a provider/Fural profile. CI unit tests do not download it.
func TestNativeObscuraRejectsPrivateNetworkAndExecutesJavaScript(t *testing.T) {
	binary := os.Getenv("CUAN_TEST_OBSCURA")
	if binary == "" {
		t.Skip("requires explicit official Obscura runtime")
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><title>Cuan fixture</title><main id="result">Initial</main><script>setTimeout(()=>document.getElementById('result').textContent='CUAN_JS_RENDERED',50)</script>`)
	}))
	defer server.Close()
	sandbox, err := exec.LookPath("bwrap")
	if err != nil {
		t.Fatal("explicit native test requires bubblewrap")
	}
	r := Runtime{Binary: binary, LockFile: filepath.Join(t.TempDir(), "browser.lock"), Sandbox: sandbox, Loader: os.Getenv("CUAN_TEST_OBSCURA_LOADER"), LibraryDir: os.Getenv("CUAN_TEST_OBSCURA_LIBRARIES")}
	if _, err := r.Fetch(context.Background(), server.URL); err == nil || requests.Load() != 0 {
		t.Fatal("private destination reached", requests.Load(), err)
	}
	// Probe script-originated network and file access from an isolated local fixture.
	// No private-network opt-in is used here. Public tool arguments never accept
	// file URLs or evaluation scripts; these are controlled integration fixtures.
	secret := filepath.Join(t.TempDir(), "provider-auth")
	if err := os.WriteFile(secret, []byte("HOST_PRIVATE_SENTINEL"), 0600); err != nil {
		t.Fatal(err)
	}
	var command string
	var args []string
	for _, target := range []string{server.URL, "file://" + secret} {
		encoded, _ := json.Marshal(target)
		script := `try{fetch(` + string(encoded) + `).then(r=>r.text()).then(()=>document.getElementById('probe').textContent='ACCESS_SUCCEEDED',()=>document.getElementById('probe').textContent='ACCESS_BLOCKED')}catch(e){document.getElementById('probe').textContent='ACCESS_BLOCKED'}`
		home := t.TempDir()
		if err := os.WriteFile(filepath.Join(home, "probe.html"), []byte(`<!doctype html><main id="probe">PENDING</main><script>`+script+`</script>`), 0600); err != nil {
			t.Fatal(err)
		}
		command, args, err = r.sandboxCommand(home, []string{"fetch", "file:///home/browser/probe.html", "--dump", "text", "--wait", "1", "--timeout", "5"})
		if err != nil {
			t.Fatal(err)
		}
		probe := exec.Command(command, args...)
		probe.Env = []string{"PATH=/usr/bin:/bin"}
		probeOutput, err := probe.CombinedOutput()
		if err != nil || requests.Load() != 0 || !strings.Contains(string(probeOutput), "ACCESS_BLOCKED") || strings.Contains(string(probeOutput), "ACCESS_SUCCEEDED") {
			t.Fatal("script-originated boundary failed", string(probeOutput), requests.Load(), err)
		}
	}

	// Only this synthetic local fixture opts into local networking. The production
	// adapter supplies no private-network flag or custom executable arguments.
	home := t.TempDir()
	command, args, err = r.sandboxCommand(home, []string{"fetch", server.URL, "--dump", "text", "--timeout", "10", "--storage-dir", "/home/browser/storage", "--allow-private-network"})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(command, args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "CUAN_JS_RENDERED") {
		t.Fatal("JavaScript result not observed", string(output), err)
	}
}
