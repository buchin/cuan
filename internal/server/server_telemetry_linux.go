//go:build linux

package server

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func getSystemResources(checkPath string) ServerTelemetry {
	var t ServerTelemetry
	t.ObservedAt = time.Now().UTC()

	// Disk Telemetry
	diskTarget := checkPath
	if diskTarget == "" {
		diskTarget = "/"
	}
	var stat syscall.Statfs_t
	err := syscall.Statfs(diskTarget, &stat)
	if err != nil && diskTarget != "/" {
		diskTarget = "/"
		err = syscall.Statfs(diskTarget, &stat)
	}
	if err == nil {
		t.DiskPath = diskTarget
		t.DiskTotalBytes = stat.Blocks * uint64(stat.Bsize)
		t.DiskFreeBytes = stat.Bavail * uint64(stat.Bsize)
		if t.DiskTotalBytes > t.DiskFreeBytes {
			t.DiskUsedBytes = t.DiskTotalBytes - t.DiskFreeBytes
		}
		if t.DiskTotalBytes > 0 {
			t.DiskUsedPercent = float64(t.DiskUsedBytes) / float64(t.DiskTotalBytes) * 100.0
		}
		// Warn if disk >= 80% used or free space < 2 GiB
		if t.DiskUsedPercent >= 80.0 || t.DiskFreeBytes < (2*1024*1024*1024) {
			t.DiskWarning = true
		}
	}

	// Memory & Load via /proc/meminfo and Sysinfo
	readMemInfo(&t)

	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err == nil {
		t.UptimeSeconds = uint64(si.Uptime)
		// Linux scales si.Loads by 65536
		t.LoadAverages[0] = float64(si.Loads[0]) / 65536.0
		t.LoadAverages[1] = float64(si.Loads[1]) / 65536.0
		t.LoadAverages[2] = float64(si.Loads[2]) / 65536.0

		// Fallback for memory if /proc/meminfo was unreadable
		if t.MemoryTotalBytes == 0 {
			unit := uint64(si.Unit)
			if unit == 0 {
				unit = 1
			}
			t.MemoryTotalBytes = uint64(si.Totalram) * unit
			t.MemoryAvailBytes = uint64(si.Freeram) * unit
			if t.MemoryTotalBytes > t.MemoryAvailBytes {
				t.MemoryUsedBytes = t.MemoryTotalBytes - t.MemoryAvailBytes
			}
			if t.MemoryTotalBytes > 0 {
				t.MemoryUsedPercent = float64(t.MemoryUsedBytes) / float64(t.MemoryTotalBytes) * 100.0
			}
		}
	}

	return t
}

func readMemInfo(t *ServerTelemetry) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return
	}
	defer f.Close()

	var memTotal, memAvailable, memFree, buffers, cached uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSuffix(parts[0], ":")
		val, _ := strconv.ParseUint(parts[1], 10, 64)

		switch key {
		case "MemTotal":
			memTotal = val * 1024
		case "MemAvailable":
			memAvailable = val * 1024
		case "MemFree":
			memFree = val * 1024
		case "Buffers":
			buffers = val * 1024
		case "Cached":
			cached = val * 1024
		}
	}

	if memTotal > 0 {
		t.MemoryTotalBytes = memTotal
		if memAvailable > 0 {
			t.MemoryAvailBytes = memAvailable
		} else {
			// Older kernels without MemAvailable
			t.MemoryAvailBytes = memFree + buffers + cached
		}
		if t.MemoryTotalBytes > t.MemoryAvailBytes {
			t.MemoryUsedBytes = t.MemoryTotalBytes - t.MemoryAvailBytes
		}
		t.MemoryUsedPercent = float64(t.MemoryUsedBytes) / float64(t.MemoryTotalBytes) * 100.0
	}
}
