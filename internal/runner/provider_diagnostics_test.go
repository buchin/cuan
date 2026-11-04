package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderStorageDiagnosticRequiresLocalProviderRecord(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(t.TempDir(), "provider.log")
	record := "E1104 18:49:38.610926 736 task_handle.go:150] failed mkdir " + home + "/.gemini/antigravity-cli/brain/task: no space left on device\n"
	for _, fixture := range []struct {
		name, text string
		want       bool
	}{
		{"actual", record, true},
		{"late", strings.Repeat("x", maxProviderDiagnosticRead*2) + "\n" + record, true},
		{"quoted", "user prompt: no space left on device\n", false},
		{"other path", strings.ReplaceAll(record, home, "/another-profile"), false},
		{"old outside tail", record + strings.Repeat("x", maxProviderDiagnosticRead*2), false},
		{"generic exit", "exit code 2\n", false},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(fixture.text), 0600); err != nil {
				t.Fatal(err)
			}
			if got := providerStorageExhausted(path, home); got != fixture.want {
				t.Fatalf("classification %t want %t", got, fixture.want)
			}
		})
	}
	if providerStorageExhausted("/does-not-exist", home) || providerStorageExhausted(path, "") {
		t.Fatal("missing evidence inferred storage failure")
	}
}
