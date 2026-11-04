package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrowserRuntimeExposesInstalledOfficialToolOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	module := filepath.Join(root, "runtime", "playwright-core")
	for _, dir := range []string{bin, module} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	cli := filepath.Join(module, "cli.js")
	if err := os.WriteFile(cli, []byte("#!/usr/bin/env node\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(cli, filepath.Join(bin, "cuan-browser")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	env := map[string]string{}
	if browserRuntimeEnvironment(env, true) != "" || len(env) != 0 {
		t.Fatal("accepted unidentified package")
	}
	if err := os.WriteFile(filepath.Join(module, "package.json"), []byte(`{"name":"playwright-core"}`), 0600); err != nil {
		t.Fatal(err)
	}
	prompt := browserRuntimeEnvironment(env, true)
	if env["CUAN_BROWSER_MODULE"] != module || env["CUAN_BROWSER_CLI"] != filepath.Join(bin, "cuan-browser") || !strings.Contains(prompt, "instead of searching") {
		t.Fatal("missing installed tool context")
	}
	if env["PLAYWRIGHT_BROWSERS_PATH"] != filepath.Join(root, "cache", "ms-playwright") {
		t.Fatal("browser cache would depend on private AI HOME")
	}
	if browserRuntimeEnvironment(env, false) != "" {
		t.Fatal("unnecessary visual guidance for nonvisual task")
	}
}
