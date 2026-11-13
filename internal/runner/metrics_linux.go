//go:build linux

package runner

func osMemInfo() (total int64, avail int64) {
	return 0, 0
}

func osCPULoad() float64 {
	return 0.0
}
