package server

// This is an admission floor, not a claim that larger builds will fit. Use
// available bytes (excluding filesystem reserves), not percentage alone.
func taskStorageCritical(resources ServerTelemetry) bool {
	return resources.DiskTotalBytes > 0 && resources.DiskFreeBytes < 2*1024*1024*1024
}
