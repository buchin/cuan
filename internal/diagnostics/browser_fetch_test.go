package diagnostics

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/browserfetch"
)

func selectedBrowserFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	binary := filepath.Join(root, "obscura")
	body := []byte("#!/bin/sh\nprintf MCP_BROWSER_CONTENT\n")
	if err := os.WriteFile(binary, body, 0700); err != nil {
		t.Fatal(err)
	}
	sandbox := filepath.Join(root, "sandbox")
	stub := `#!/bin/sh
engine=''
while [ "$#" -gt 0 ]; do
case "$1" in
--ro-bind) [ "$3" != /cuan-obscura ] || engine="$2"; shift 3 ;;
--) shift; shift; exec "$engine" "$@" ;;
*) shift ;;
esac
done
exit 99
`
	if err := os.WriteFile(sandbox, []byte(stub), 0700); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(root, "browser.lock")
	if err := os.WriteFile(lock, nil, 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(body)
	manifest := browserfetch.Manifest{Runtime: browserfetch.Runtime{Binary: binary, Sandbox: sandbox, LockFile: lock}, Version: browserfetch.Version, ArchiveSHA256: strings.Repeat("a", 64), BinarySHA256: hex.EncodeToString(hash[:])}
	raw, _ := json.Marshal(manifest)
	path := filepath.Join(root, "runtime.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CUAN_CHAT_BROWSER_MANIFEST", path)
	return binary
}

func TestBrowserMCPExposesOnlyVerifiedSelectedRuntime(t *testing.T) {
	binary := selectedBrowserFixture(t)
	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"browser_fetch","arguments":{"url":"https://example.invalid/"}}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"browser_fetch","arguments":{"url":"https://example.invalid/","eval":"arbitrary"}}}
`)
	var out, stderr bytes.Buffer
	if RunPublicWebMCP(input, &out, &stderr) != 0 {
		t.Fatal(stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 || !strings.Contains(lines[0], `"name":"browser_fetch"`) || !strings.Contains(lines[1], "MCP_BROWSER_CONTENT") || !strings.Contains(lines[2], `"isError":true`) {
		t.Fatal(out.String())
	}
	if err := os.WriteFile(binary, []byte("modified"), 0700); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	RunPublicWebMCP(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n"), &out, &stderr)
	if strings.Contains(out.String(), `"name":"browser_fetch"`) {
		t.Fatal("modified runtime still advertised", out.String())
	}
}

func TestBrowserMCPDoesNotUseAmbientBrowser(t *testing.T) {
	t.Setenv("CUAN_CHAT_BROWSER_MANIFEST", "")
	if publicChatTool("browser_fetch") {
		t.Fatal("ambient browser enabled")
	}
	var out, stderr bytes.Buffer
	RunPublicWebMCP(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"browser_fetch","arguments":{"url":"https://example.invalid/"}}}`+"\n"), &out, &stderr)
	if !strings.Contains(out.String(), `"code":-32601`) {
		t.Fatal("unconfigured browser remained callable", out.String())
	}
}
