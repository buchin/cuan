package runner

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectionDependenciesUsePrivateLockedSetup(t *testing.T) {
	for _, scenario := range []string{"valid", "shared dependencies", "linked lock", "tracked output", "failed installer", "git dependency", "file dependency", "embedded credentials"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			bin := t.TempDir()
			write := func(p, s string) {
				t.Helper()
				if err := os.WriteFile(p, []byte(s), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
				t.Fatal(err, string(out))
			}
			write(filepath.Join(root, "package.json"), `{"name":"fixture","version":"1.0.0"}`)
			write(filepath.Join(root, "package-lock.json"), `{"lockfileVersion":3,"packages":{}}`)
			for name, endpoint := range map[string]string{"git dependency": "git+ssh://example.invalid/repository", "file dependency": "file:../../shared", "embedded credentials": "https://user:private@example.invalid/package.tgz"} {
				if scenario == name {
					write(filepath.Join(root, "package-lock.json"), `{"lockfileVersion":3,"packages":{"node_modules/example":{"resolved":"`+endpoint+`"}}}`)
				}
			}
			if scenario != "tracked output" {
				write(filepath.Join(root, ".gitignore"), "node_modules/\n")
			}
			if scenario == "shared dependencies" {
				if err := os.Symlink(t.TempDir(), filepath.Join(root, "node_modules")); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "linked lock" {
				_ = os.Remove(filepath.Join(root, "package-lock.json"))
				if err := os.Symlink(filepath.Join(root, "package.json"), filepath.Join(root, "package-lock.json")); err != nil {
					t.Fatal(err)
				}
			}
			script := "#!/bin/sh\n[ -z \"$CUAN_AUTH_TOKEN$NPM_TOKEN$CUAN_TASK_CAPABILITY_JSON\" ] || exit 9\n[ \"$HOME\" != \"$ORIGINAL_HOME\" ] || exit 10\nprintf '%s\\n' \"$@\" > setup-arguments\nprintf '%s' \"$HOME\" > setup-home\n"
			if scenario == "failed installer" {
				script += "printf 'npm ERR code ENOSPC\\nsecret=private-installer-secret\\n' >&2\nexit 7\n"
			}
			write(filepath.Join(bin, "npm"), script)
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("CUAN_AUTH_TOKEN", "private")
			t.Setenv("NPM_TOKEN", "private")
			t.Setenv("CUAN_TASK_CAPABILITY_JSON", "private")
			t.Setenv("ORIGINAL_HOME", os.Getenv("HOME"))
			err := prepareInspectionDependencies(context.Background(), root)
			if (err == nil) != (scenario == "valid") {
				t.Fatalf("unexpected setup acceptance: %v", err)
			}
			if scenario == "failed installer" && (!strings.Contains(err.Error(), "ENOSPC") || strings.Contains(err.Error(), "private-installer-secret")) {
				t.Fatal("missing useful or leaked installer diagnostic", err)
			}
			args, readErr := os.ReadFile(filepath.Join(root, "setup-arguments"))
			if scenario == "valid" {
				if readErr != nil || !strings.Contains(string(args), "ci\n--ignore-scripts\n--no-audit\n--no-fund\n") {
					t.Fatal("installer did not use locked script-free setup")
				}
				home, _ := os.ReadFile(filepath.Join(root, "setup-home"))
				if _, err := os.Stat(string(home)); !os.IsNotExist(err) {
					t.Fatal("private installer home was retained")
				}
			} else if scenario != "failed installer" && readErr == nil {
				t.Fatal("unsafe layout launched installer")
			}
		})
	}
}

func TestInspectionDependenciesInstallFromWarmCacheWithoutNetworkOrScripts(t *testing.T) {
	npm, err := exec.LookPath("npm")
	if err != nil {
		t.Skip("npm is required for the real offline cache fixture")
	}
	cache := filepath.Join(t.TempDir(), "npm-cache")
	var archive bytes.Buffer
	zipped := gzip.NewWriter(&archive)
	packed := tar.NewWriter(zipped)
	manifest := []byte(`{"name":"cuan-offline-fixture","version":"1.0.0","scripts":{"postinstall":"touch ../forbidden-lifecycle"}}`)
	if err := packed.WriteHeader(&tar.Header{Name: "package/package.json", Mode: 0600, Size: int64(len(manifest))}); err != nil {
		t.Fatal(err)
	}
	if _, err := packed.Write(manifest); err != nil {
		t.Fatal(err)
	}
	if err := packed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zipped.Close(); err != nil {
		t.Fatal(err)
	}
	tarball := filepath.Join(t.TempDir(), "fixture.tgz")
	if err := os.WriteFile(tarball, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	seed := exec.Command(npm, "cache", "add", tarball, "--cache", cache, "--ignore-scripts")
	seed.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir()}
	if out, err := seed.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	bin := t.TempDir()
	// Force the actual npm installer offline: success cannot depend on a request
	// to the deliberately nonexistent package URL or on an ambient user cache.
	wrapper := "#!/bin/sh\nexec '" + strings.ReplaceAll(npm, "'", "'\\''") + "' --offline \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "npm"), []byte(wrapper), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	digest := sha512.Sum512(archive.Bytes())
	lock, _ := json.Marshal(map[string]any{"name": "offline-project", "version": "1.0.0", "lockfileVersion": 3, "packages": map[string]any{
		"":                                  map[string]any{"name": "offline-project", "version": "1.0.0", "dependencies": map[string]string{"cuan-offline-fixture": "1.0.0"}},
		"node_modules/cuan-offline-fixture": map[string]any{"version": "1.0.0", "resolved": "https://registry.npmjs.org/cuan-offline-fixture/-/cuan-offline-fixture-1.0.0.tgz", "integrity": "sha512-" + base64.StdEncoding.EncodeToString(digest[:]), "hasInstallScript": true},
	}})
	for range 2 { // Developer and verifier have separate outputs, one download cache.
		root := t.TempDir()
		if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
			t.Fatal(err, string(out))
		}
		for name, data := range map[string][]byte{"package.json": []byte(`{"name":"offline-project","version":"1.0.0","dependencies":{"cuan-offline-fixture":"1.0.0"}}`), "package-lock.json": lock, ".gitignore": []byte("node_modules/\n")} {
			if err := os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		if err := prepareInspectionDependencies(context.Background(), root, cache); err != nil {
			t.Fatal(err)
		}
		if data, err := os.ReadFile(filepath.Join(root, "node_modules/cuan-offline-fixture/package.json")); err != nil || !bytes.Equal(data, manifest) {
			t.Fatal("warm cache did not produce the locked package", err)
		}
		if data, err := os.ReadFile(filepath.Join(root, "package-lock.json")); err != nil || !bytes.Equal(data, lock) {
			t.Fatal("installation changed the source lock", err)
		}
		if _, err := os.Stat(filepath.Join(root, "node_modules/forbidden-lifecycle")); !os.IsNotExist(err) {
			t.Fatal("installer executed dependency lifecycle scripts")
		}
	}
}
