package agent

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ProcessMonitor periodically samples process RSS bytes until stopped.
type ProcessMonitor struct {
	pid      int
	peakRSS  int64
	stopChan chan struct{}
	mu       sync.Mutex
	done     chan struct{}
}

// StartProcessMonitor starts sampling memory usage for the given pid.
func StartProcessMonitor(pid int, sampleInterval time.Duration) *ProcessMonitor {
	pm := &ProcessMonitor{
		pid:      pid,
		stopChan: make(chan struct{}),
		done:     make(chan struct{}),
	}

	go pm.run(sampleInterval)
	return pm
}

func (pm *ProcessMonitor) run(interval time.Duration) {
	defer close(pm.done)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.stopChan:
			return
		case <-ticker.C:
			rss := readRSS(pm.pid)
			pm.mu.Lock()
			if rss > pm.peakRSS {
				pm.peakRSS = rss
			}
			pm.mu.Unlock()
		}
	}
}

// Stop stops the monitor and returns the peak RSS in bytes.
func (pm *ProcessMonitor) Stop() int64 {
	select {
	case <-pm.stopChan:
		// already closed
	default:
		close(pm.stopChan)
	}
	<-pm.done

	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.peakRSS
}

// readRSS reads the resident set size in bytes from /proc/<pid>/statm.
func readRSS(pid int) int64 {
	if pid <= 0 {
		return 0
	}
	statmPath := fmt.Sprintf("/proc/%d/statm", pid)
	data, err := os.ReadFile(statmPath)
	if err != nil {
		return 0
	}

	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return 0
	}

	// field[1] is resident pages
	pages, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return 0
	}

	pageSize := int64(os.Getpagesize())
	return pages * pageSize
}
