//go:build !linux

package server

func getSystemResources(checkPath string) ServerTelemetry {
	// Fallback for non-Linux OS (macOS, Windows, BSD in development)
	return ServerTelemetry{
		DiskPath:          checkPath,
		DiskTotalBytes:    100 * 1024 * 1024 * 1024,
		DiskFreeBytes:     50 * 1024 * 1024 * 1024,
		DiskUsedBytes:     50 * 1024 * 1024 * 1024,
		DiskUsedPercent:   50.0,
		MemoryTotalBytes:  16 * 1024 * 1024 * 1024,
		MemoryAvailBytes:  8 * 1024 * 1024 * 1024,
		MemoryUsedBytes:   8 * 1024 * 1024 * 1024,
		MemoryUsedPercent: 50.0,
		UptimeSeconds:     3600,
	}
}
