package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOnboardingPrerequisitesDoNotExecuteProvider(t *testing.T) {
	bin := t.TempDir()
	marker := filepath.Join(bin, "executed")
	for _, name := range []string{"agy", "bwrap"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\ntouch "+marker+"\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	got := onboardingRuntimePrerequisites()
	if got["cli_available"] != true || got["sandbox_available"] != true || got["execution_verified"] != false {
		t.Fatal(got)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("A readiness inspection executed a command")
	}
	if len(got) != 5 {
		t.Fatal("Unexpected runtime fields; do not expose private host details", got)
	}
	if err := os.Remove(filepath.Join(bin, "agy")); err != nil {
		t.Fatal(err)
	}
	got = onboardingRuntimePrerequisites()
	if got["cli_available"] != false || got["sandbox_available"] != true {
		t.Fatal(got)
	}
}
