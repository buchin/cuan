package browserfetch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func manifestFixture(t *testing.T) (string, Manifest) {
	t.Helper()
	runtime := fixtureRuntime(t, "printf 'fixture'\n")
	if err := os.WriteFile(runtime.LockFile, nil, 0600); err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(runtime.Binary)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(binary)
	manifest := Manifest{Runtime: runtime, Version: Version, ArchiveSHA256: strings.Repeat("a", 64), BinarySHA256: hex.EncodeToString(hash[:])}
	path := filepath.Join(t.TempDir(), "runtime.json")
	raw, _ := json.Marshal(manifest)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path, manifest
}

func TestBrowserManifestVerifiesActualSelectedRuntime(t *testing.T) {
	path, expected := manifestFixture(t)
	actual, err := LoadManifest(path)
	if err != nil || actual.Binary != expected.Binary || actual.LockFile != expected.LockFile {
		t.Fatal(actual, err)
	}
}

func TestBrowserManifestRejectsUnverifiedSelection(t *testing.T) {
	for _, scenario := range []string{"binary-changed", "binary-missing", "binary-symlink", "version", "checksum", "public-manifest", "manifest-symlink", "unknown-field", "trailing-json", "missing-lock", "lock-symlink", "relative-binary", "missing-sandbox", "incomplete-loader"} {
		t.Run(scenario, func(t *testing.T) {
			path, m := manifestFixture(t)
			switch scenario {
			case "binary-changed":
				os.WriteFile(m.Binary, []byte("modified"), 0700)
			case "binary-missing":
				os.Remove(m.Binary)
			case "binary-symlink":
				original := m.Binary + ".retained"
				os.Rename(m.Binary, original)
				os.Symlink(original, m.Binary)
			case "version":
				m.Version = "unverified"
			case "checksum":
				m.BinarySHA256 = "not-a-hash"
			case "public-manifest":
				os.Chmod(path, 0644)
			case "manifest-symlink":
				original := path + ".retained"
				os.Rename(path, original)
				os.Symlink(original, path)
			case "unknown-field":
				raw, _ := os.ReadFile(path)
				raw = append(raw[:len(raw)-1], []byte(`,"command":"arbitrary"}`)...)
				os.WriteFile(path, raw, 0600)
			case "trailing-json":
				file, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
				file.WriteString(` {"extra":true}`)
				file.Close()
			case "missing-lock":
				os.Remove(m.LockFile)
			case "lock-symlink":
				target := filepath.Join(t.TempDir(), "other")
				os.WriteFile(target, nil, 0600)
				os.Remove(m.LockFile)
				os.Symlink(target, m.LockFile)
			case "relative-binary":
				m.Binary = "relative-engine"
			case "missing-sandbox":
				m.Sandbox = filepath.Join(t.TempDir(), "missing")
			case "incomplete-loader":
				m.Loader = "/explicit/loader"
			}
			if scenario == "version" || scenario == "checksum" || scenario == "relative-binary" || scenario == "missing-sandbox" || scenario == "incomplete-loader" {
				raw, _ := json.Marshal(m)
				os.WriteFile(path, raw, 0600)
			}
			if actual, err := LoadManifest(path); err == nil || actual != nil {
				t.Fatal("unverified runtime accepted", actual, err)
			}
		})
	}
}

func TestInstalledBrowserManifest(t *testing.T) {
	path := os.Getenv("CUAN_TEST_BROWSER_MANIFEST")
	if path == "" {
		t.Skip("requires explicit installed runtime manifest")
	}
	if _, err := LoadManifest(path); err != nil {
		t.Fatal("installed manifest is not usable", err)
	}
}
