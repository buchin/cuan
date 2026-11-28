//go:build linux

package runner

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// descendantCPUTicks returns the total user+system CPU clock ticks across all
// living descendant processes of rootPID, along with the count of descendants.
// Used by the progress watchdog to distinguish an agent waiting on an active
// long-running build/test command from a genuinely stalled agent.
func descendantCPUTicks(rootPID int) (uint64, int) {
	if rootPID <= 0 {
		return 0, 0
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, 0
	}

	type procStat struct {
		ppid int
		cpu  uint64
	}
	stats := make(map[int]procStat, len(entries))
	children := make(map[int][]int)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "stat"))
		if err != nil {
			continue
		}
		line := string(raw)
		rparen := strings.LastIndexByte(line, ')')
		if rparen < 0 || rparen+2 >= len(line) {
			continue
		}
		fields := strings.Fields(line[rparen+2:])
		// fields[0]=state, fields[1]=ppid, fields[11]=utime, fields[12]=stime
		if len(fields) < 13 {
			continue
		}
		ppid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		utime, _ := strconv.ParseUint(fields[11], 10, 64)
		stime, _ := strconv.ParseUint(fields[12], 10, 64)
		stats[pid] = procStat{ppid: ppid, cpu: utime + stime}
		children[ppid] = append(children[ppid], pid)
	}

	queue := append([]int(nil), children[rootPID]...)
	visited := make(map[int]bool)
	var totalCPU uint64
	count := 0
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		if visited[pid] {
			continue
		}
		visited[pid] = true
		if st, ok := stats[pid]; ok {
			totalCPU += st.cpu
			count++
		}
		queue = append(queue, children[pid]...)
	}
	return totalCPU, count
}
