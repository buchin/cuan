package runner

import (
	"testing"
)

func TestAcquireRunnerInstanceLock_PreventsDuplicateRunnerID(t *testing.T) {
	dir := t.TempDir()

	release1, err := AcquireRunnerInstanceLock(dir, "cuan-runner")
	if err != nil {
		t.Fatalf("first lock acquisition failed: %v", err)
	}
	defer release1()

	if _, err := AcquireRunnerInstanceLock(dir, "cuan-runner"); err == nil {
		t.Fatal("expected second lock acquisition for same runner-id to fail")
	}

	// Different runner-id must succeed independently
	releaseOther, err := AcquireRunnerInstanceLock(dir, "runner-01")
	if err != nil {
		t.Fatalf("lock acquisition for distinct runner-id failed: %v", err)
	}
	releaseOther()

	// Releasing first lock allows re-acquiring
	release1()
	release2, err := AcquireRunnerInstanceLock(dir, "cuan-runner")
	if err != nil {
		t.Fatalf("lock re-acquisition after release failed: %v", err)
	}
	release2()
}
