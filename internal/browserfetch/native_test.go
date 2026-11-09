package browserfetch

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Explicit opt-in uses an independently installed official engine, not an
// ambient browser or a provider/Fural profile. CI unit tests do not download it.
func TestNativeObscuraRejectsPrivateNetworkAndExecutesJavaScript(t *testing.T) {
	binary := os.Getenv("CUAN_TEST_OBSCURA")
	if binary == "" {
		t.Skip("requires explicit official Obscura runtime")
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><title>Cuan fixture</title><main id="result">Initial</main><script>setTimeout(()=>document.getElementById('result').textContent='CUAN_JS_RENDERED',50)</script>`)
	}))
	defer server.Close()
	r := Runtime{Binary: binary, LockFile: filepath.Join(t.TempDir(), "browser.lock")}
	if _, err := r.Fetch(context.Background(), server.URL); err == nil || requests != 0 {
		t.Fatal("private destination reached", requests, err)
	}
	// Only this synthetic local fixture opts into local networking. The production
	// adapter supplies no private-network flag or custom executable arguments.
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	local := fixtureRuntime(t, "exec "+quote(binary)+" \"$@\" --allow-private-network\n")
	result, err := local.Fetch(context.Background(), server.URL)
	if err != nil || !strings.Contains(result.Content, "CUAN_JS_RENDERED") {
		t.Fatal("JavaScript result not observed", result, err)
	}
}
