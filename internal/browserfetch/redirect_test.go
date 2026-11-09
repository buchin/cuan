package browserfetch

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Give a fixture a public IP only inside a disconnected network namespace.
// This exercises real redirect handling without permitting private networking,
// exposing a production endpoint or trusting an external redirect service.
func TestNativeObscuraRedirectBoundaryInPrivateNetworkNamespace(t *testing.T) {
	manifest := os.Getenv("CUAN_TEST_BROWSER_MANIFEST")
	if manifest == "" {
		t.Skip("requires explicit installed browser runtime")
	}
	unshare, err := exec.LookPath("unshare")
	if err != nil {
		t.Fatal("native redirect test requires unshare")
	}
	ip, err := exec.LookPath("ip")
	script := `set -eu
ip_cli=$1; shift
"$ip_cli" link set lo up
"$ip_cli" address add 8.8.8.8/32 dev lo
exec "$@"`
	if err != nil {
		ip, err = exec.LookPath("busybox")
		if err != nil {
			t.Fatal("native redirect test requires ip or BusyBox ip")
		}
		script = `set -eu
ip_cli=$1; shift
"$ip_cli" ip link set lo up
"$ip_cli" ip address add 8.8.8.8/32 dev lo
exec "$@"`
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, unshare, "--user", "--map-root-user", "--net", "/bin/sh", "-c", script, "fixture", ip, self, "-test.run=^TestNativeObscuraRedirectNamespaceHelper$", "-test.v")
	command.Env = []string{"PATH=/opt/cuan/runtime:/usr/bin:/bin:/usr/sbin:/sbin", "CUAN_TEST_BROWSER_MANIFEST=" + manifest, "CUAN_TEST_REDIRECT_HELPER=1"}
	output, err := command.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "REDIRECT_BOUNDARY_OK") {
		t.Fatal("native redirect boundary failed", string(output), err)
	}
	t.Log(string(output))
}

func TestNativeObscuraRedirectNamespaceHelper(t *testing.T) {
	if os.Getenv("CUAN_TEST_REDIRECT_HELPER") != "1" {
		t.Skip("runs only in a dedicated disconnected network namespace")
	}
	manifest, err := LoadManifest(os.Getenv("CUAN_TEST_BROWSER_MANIFEST"))
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	public := fmt.Sprintf("http://8.8.8.8:%d", port)
	private := fmt.Sprintf("http://127.0.0.1:%d/private", port)
	var privateCalls atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.Host, "127.0.0.1:") {
			privateCalls.Add(1)
			fmt.Fprint(w, "PRIVATE_SERVICE")
			return
		}
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, private, http.StatusFound)
		case "/script":
			target, _ := json.Marshal(public + "/redirect")
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<!doctype html><main id="probe">PENDING</main><script>try{fetch(`+string(target)+`).then(()=>document.getElementById('probe').textContent='REDIRECT_BYPASS',()=>document.getElementById('probe').textContent='REDIRECT_BLOCKED')}catch(e){document.getElementById('probe').textContent='REDIRECT_BLOCKED'}</script>`)
		default:
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<!doctype html><main>PUBLIC_CONTROL_OK</main>")
		}
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: time.Second}
	defer server.Close()
	go server.Serve(listener)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Get(public + "/redirect")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	location, err := url.Parse(response.Header.Get("Location"))
	if err != nil || response.StatusCode != 302 || location.String() != private {
		t.Fatal("fixture did not produce exact302 redirect")
	}
	control, err := manifest.Runtime.Fetch(context.Background(), public+"/control")
	if err != nil || !strings.Contains(control.Content, "PUBLIC_CONTROL_OK") {
		t.Fatal("public browser control failed", control, err)
	}
	result, err := manifest.Runtime.Fetch(context.Background(), public+"/redirect")
	if err == nil || result != nil || privateCalls.Load() != 0 {
		t.Fatal("navigation redirect reached private service", result, err, privateCalls.Load())
	}
	scriptResult, err := manifest.Runtime.Fetch(context.Background(), public+"/script")
	if err != nil || !strings.Contains(scriptResult.Content, "REDIRECT_BLOCKED") || strings.Contains(scriptResult.Content, "REDIRECT_BYPASS") || privateCalls.Load() != 0 {
		t.Fatal("script redirect reached private service", scriptResult, err, privateCalls.Load())
	}
	t.Log("REDIRECT_BOUNDARY_OK: exact302, public control, navigation and script redirects blocked; private service received zero calls")
}
