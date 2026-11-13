//go:build !darwin && !linux

package runner

func osMemInfo() (total int64, avail int64) {
	return 1024 * 1024 * 1024, 512 * 1024 * 1024
}

func osCPULoad() float64 {
	return 0.0
}
