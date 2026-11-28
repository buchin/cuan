//go:build !linux

package runner

func descendantCPUTicks(_ int) (uint64, int) {
	return 0, 0
}
