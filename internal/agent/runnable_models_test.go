package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeCatalogIsScopedCachedAndNotInference(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FURAL_AGENT_TOKEN", "must-not-inherit")
	bin := t.TempDir()
	script := `#!/bin/sh
set -eu
[ "$#" = 1 ] && [ "$1" = models ] || exit 31
[ -z "${FURAL_AGENT_TOKEN:-}" ] || exit 32
printf 'called\n' >> "$HOME/catalog-calls"
printf 'gemini-current\tCurrent Gemini\nclaude-sonnet-current\tSonnet\nrandom-invalid\tNot a model\n'
`
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	a, err := CreateProviderAccount("owner", "Fixture")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := ProviderAccountHome("owner", a.ID)
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	models, ok := nativeAccountModelCatalog(context.Background(), "owner", a.ID)
	if !ok || !models["gemini-current"] || !models["claude-sonnet-current"] || models["random-invalid"] {
		t.Fatal("invalid native catalog", models, ok)
	}
	if _, ok := nativeAccountModelCatalog(context.Background(), "other", a.ID); ok {
		t.Fatal("cross-owner catalog")
	}
	if _, ok := nativeAccountModelCatalog(context.Background(), "owner", a.ID); !ok {
		t.Fatal("cached catalog unavailable")
	}
	nativeCatalogCache.Lock()
	for key, entry := range nativeCatalogCache.entries {
		if strings.HasPrefix(key, home+"\x00") {
			entry.at = time.Now().Add(-6 * time.Minute)
			nativeCatalogCache.entries[key] = entry
		}
	}
	nativeCatalogCache.Unlock()
	release, err := TryLeaseProviderAccount("owner", a.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, ok := nativeAccountModelCatalog(context.Background(), "owner", a.ID); !ok {
		t.Fatal("active task made a second session lose its supported catalog")
	}
	nativeCatalogCache.Lock()
	for key, entry := range nativeCatalogCache.entries {
		if strings.HasPrefix(key, home+"\x00") {
			entry.at = time.Now().Add(-25 * time.Hour)
			nativeCatalogCache.entries[key] = entry
		}
	}
	nativeCatalogCache.Unlock()
	if _, ok := nativeAccountModelCatalog(context.Background(), "owner", a.ID); ok {
		t.Fatal("catalog older than24hours reused while busy")
	}
	data, _ := os.ReadFile(filepath.Join(home, "catalog-calls"))
	if strings.Count(string(data), "called") != 1 {
		t.Fatal("catalog was not cached")
	}
}

func TestNativeCatalogRejectsFailureAndOverflow(t *testing.T) {
	for _, body := range []string{"exit 3", "printf 'not-a-model\\n'", "head -c 70000 /dev/zero"} {
		t.Run(body, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			bin := t.TempDir()
			os.WriteFile(filepath.Join(bin, "agy"), []byte("#!/bin/sh\n"+body+"\n"), 0700)
			t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
			a, err := CreateProviderAccount("owner", "Fixture")
			if err != nil {
				t.Fatal(err)
			}
			home, _ := ProviderAccountHome("owner", a.ID)
			os.MkdirAll(home, 0700)
			if _, ok := nativeAccountModelCatalog(context.Background(), "owner", a.ID); ok {
				t.Fatal("invalid catalog accepted")
			}
		})
	}
}
