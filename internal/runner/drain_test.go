package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDrainCoordinator_RequestDrainAndCallback(t *testing.T) {
	dc := NewDrainCoordinator("/tmp/nonexistent-test-drain-file")
	if dc.IsDraining() {
		t.Fatal("expected coordinator to start non-draining")
	}

	called := false
	var reasonReceived string
	dc.OnDrain(func(reason string) {
		called = true
		reasonReceived = reason
	})

	dc.RequestDrain("deploy reload signal")
	if !dc.IsDraining() {
		t.Fatal("expected coordinator to be draining after RequestDrain")
	}
	if !called {
		t.Fatal("expected OnDrain callback to be called")
	}
	if reasonReceived != "deploy reload signal" {
		t.Fatalf("expected reason 'deploy reload signal', got '%s'", reasonReceived)
	}

	// RequestDrain is idempotent
	calledTwice := false
	dc.OnDrain(func(reason string) {
		calledTwice = true
	})
	dc.RequestDrain("another reason")
	if calledTwice {
		t.Fatal("expected second RequestDrain call to be no-op")
	}
}

func TestDrainCoordinator_CheckDrainFile(t *testing.T) {
	tmpDir := t.TempDir()
	flagPath := filepath.Join(tmpDir, "cuan-drain")

	dc := NewDrainCoordinator(flagPath)
	if dc.CheckDrainFile() {
		t.Fatal("expected CheckDrainFile to be false before file creation")
	}

	// Create drain flag file
	if err := os.WriteFile(flagPath, []byte("drain"), 0600); err != nil {
		t.Fatal(err)
	}

	if !dc.CheckDrainFile() {
		t.Fatal("expected CheckDrainFile to be true after file creation")
	}
	if !dc.IsDraining() {
		t.Fatal("expected coordinator to be in draining state after flag detection")
	}

	// Remove drain flag file (simulating deploy completion)
	_ = os.Remove(flagPath)
	if dc.CheckDrainFile() {
		t.Fatal("expected CheckDrainFile to be false after file removal")
	}
	if dc.IsDraining() {
		t.Fatal("expected coordinator to clear draining state after flag removal")
	}
}

func TestDrainCoordinator_StartMonitor(t *testing.T) {
	tmpDir := t.TempDir()
	flagPath := filepath.Join(tmpDir, "cuan-drain-monitor")

	dc := NewDrainCoordinator(flagPath)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go dc.StartMonitor(ctx, 20*time.Millisecond)

	time.Sleep(50 * time.Millisecond)
	if dc.IsDraining() {
		t.Fatal("expected coordinator to not be draining yet")
	}

	// Touch flag
	_ = os.WriteFile(flagPath, []byte("drain"), 0600)

	// Wait for monitor to detect
	deadline := time.Now().Add(1 * time.Second)
	for {
		if dc.IsDraining() {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for monitor to detect drain flag")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
