package runner

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// HostMetrics collects CPU, RAM, and Disk utilization for the runner host.
type HostMetrics struct {
	CPUPercent       float64
	MemoryUsedBytes  int64
	MemoryTotalBytes int64
	DiskUsedBytes    int64
	DiskTotalBytes   int64
}

// CollectHostMetrics reads system metrics from /proc and statfs.
func CollectHostMetrics() HostMetrics {
	var m HostMetrics

	// Memory metrics from /proc/meminfo
	memTotal, memAvail := readMemInfo()
	m.MemoryTotalBytes = memTotal
	m.MemoryUsedBytes = memTotal - memAvail

	// Disk metrics from current working directory
	diskTotal, diskUsed := readDiskInfo(".")
	m.DiskTotalBytes = diskTotal
	m.DiskUsedBytes = diskUsed

	// CPU Load estimate from /proc/loadavg
	m.CPUPercent = readCPULoad()

	return m
}

func readMemInfo() (total int64, avail int64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		val, _ := strconv.ParseInt(fields[1], 10, 64)
		valBytes := val * 1024 // /proc/meminfo values are in kB

		switch key {
		case "MemTotal":
			total = valBytes
		case "MemAvailable":
			avail = valBytes
		}
	}
	return total, avail
}

func readDiskInfo(path string) (total int64, used int64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0
	}
	total = int64(stat.Blocks) * int64(stat.Bsize)
	free := int64(stat.Bfree) * int64(stat.Bsize)
	used = total - free
	return total, used
}

func readCPULoad() float64 {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0.0
	}
	fields := strings.Fields(string(data))
	if len(fields) > 0 {
		load1, err := strconv.ParseFloat(fields[0], 64)
		if err == nil {
			// Convert load to roughly a percentage (e.g. 1.0 on 2-core machine ~ 50%)
			return load1 * 25.0
		}
	}
	return 0.0
}
