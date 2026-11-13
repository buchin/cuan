//go:build darwin

package runner

import (
	"golang.org/x/sys/unix"
)

func osMemInfo() (total int64, avail int64) {
	mem, err := unix.SysctlUint64("hw.memsize")
	if err == nil && mem > 0 {
		return int64(mem), int64(mem / 2)
	}
	return 0, 0
}

func osCPULoad() float64 {
	return 0.0
}
