package runner

import (
	"context"
	"log"
	"os"
	"sync"
	"time"
)

const DefaultDrainFile = "/tmp/cuan-drain"

// DrainCoordinator manages the graceful drain state of the worker daemon.
// When draining, the worker stops claiming new tasks from the queue
// and allows active in-flight tasks to finish to completion without interruption.
type DrainCoordinator struct {
	mu           sync.RWMutex
	draining     bool
	drainReason  string
	drainFile    string
	onDrainStart []func(reason string)
}

func NewDrainCoordinator(drainFile string) *DrainCoordinator {
	if drainFile == "" {
		if envFile := os.Getenv("CUAN_DRAIN_FILE"); envFile != "" {
			drainFile = envFile
		} else {
			drainFile = DefaultDrainFile
		}
	}
	return &DrainCoordinator{
		drainFile: drainFile,
	}
}

// OnDrain registers a callback function to be called when graceful drain is triggered.
func (dc *DrainCoordinator) OnDrain(fn func(reason string)) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.onDrainStart = append(dc.onDrainStart, fn)
}

// RequestDrain initiates the graceful drain process with a specified reason.
func (dc *DrainCoordinator) RequestDrain(reason string) {
	dc.mu.Lock()
	if dc.draining {
		dc.mu.Unlock()
		return
	}
	dc.draining = true
	if reason == "" {
		reason = "graceful drain requested"
	}
	dc.drainReason = reason
	callbacks := append([]func(string){}, dc.onDrainStart...)
	dc.mu.Unlock()

	log.Printf("[DrainCoordinator] Graceful drain activated: %s. New task claims blocked; allowing in-flight tasks to complete.", reason)
	for _, cb := range callbacks {
		cb(reason)
	}
}

// IsDraining returns true if the worker is in graceful drain mode.
func (dc *DrainCoordinator) IsDraining() bool {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return dc.draining
}

// DrainReason returns the reason graceful drain was triggered.
func (dc *DrainCoordinator) DrainReason() string {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return dc.drainReason
}

// DrainFile returns the configured drain flag file path.
func (dc *DrainCoordinator) DrainFile() string {
	return dc.drainFile
}

// CheckDrainFile checks if the drain flag file exists on disk.
func (dc *DrainCoordinator) CheckDrainFile() bool {
	if dc.drainFile == "" {
		return false
	}
	if _, err := os.Stat(dc.drainFile); err == nil {
		if !dc.IsDraining() {
			dc.RequestDrain("drain flag file detected at " + dc.drainFile)
		}
		return true
	}
	return false
}

// StartMonitor periodically checks for the drain flag file until context is canceled.
func (dc *DrainCoordinator) StartMonitor(ctx context.Context, checkInterval time.Duration) {
	if checkInterval <= 0 {
		checkInterval = 1 * time.Second
	}
	// Initial check
	dc.CheckDrainFile()

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dc.CheckDrainFile()
		}
	}
}
