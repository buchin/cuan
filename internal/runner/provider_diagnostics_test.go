package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestHistoricalStorageEvidenceRequiresExactWorkspaceAndAttemptWindow(t *testing.T) {
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "task")
	started := time.Now().Add(-time.Hour).Truncate(time.Second)
	finished := started.Add(2 * time.Minute)
	directory := filepath.Join(home, ".gemini", "antigravity-cli", "log")
	os.MkdirAll(directory, 0700)
	path := filepath.Join(directory, started.In(time.Local).Format("cli-20060102_150405.log"))
	raw := "I1104 18:47:50.465845 1 server.go:313] Creating CLI server backend: product=antigravity workspaceDirs=[" + workspace + " " + workspace + "]\n" + "E1104 18:49:38.610926 736 task_handle.go:150] failed mkdir " + home + "/.gemini/antigravity-cli/brain/task: no space left on device\n"
	os.WriteFile(path, []byte(raw), 0600)
	os.Chtimes(path, finished, finished)
	if got := HistoricalProviderStorageEvidence(home, workspace, started, finished); len(got) != 64 {
		t.Fatal("exact historical provider process not diagnosed")
	}
	for _, scenario := range []string{"foreign workspace", "other attempt", "later write"} {
		t.Run(scenario, func(t *testing.T) {
			target, start := workspace, started
			if scenario == "foreign workspace" {
				target += "-other"
			}
			if scenario == "other attempt" {
				start = start.Add(-10 * time.Minute)
			}
			if scenario == "later write" {
				os.Chtimes(path, finished.Add(time.Minute), finished.Add(time.Minute))
				defer os.Chtimes(path, finished, finished)
			}
			if HistoricalProviderStorageEvidence(home, target, start, finished) != "" {
				t.Fatal("unbound log accepted")
			}
		})
	}
}
