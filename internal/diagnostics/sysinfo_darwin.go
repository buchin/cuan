//go:build darwin

package diagnostics

import (
	"encoding/binary"
	"time"

	"golang.org/x/sys/unix"
)

func collectSystemLoad(res *SystemHealthResult) {
	if b, err := unix.SysctlRaw("vm.loadavg"); err == nil && len(b) >= 16 {
		var scale uint64
		if len(b) >= 24 {
			scale = binary.LittleEndian.Uint64(b[16:24])
		}
		if scale == 0 && len(b) >= 16 {
			scale = uint64(binary.LittleEndian.Uint32(b[12:16]))
		}
		if scale == 0 {
			scale = 2048 // fallback darwin FSCALE
		}
		res.LoadAverages[0] = float64(binary.LittleEndian.Uint32(b[0:4])) / float64(scale)
		res.LoadAverages[1] = float64(binary.LittleEndian.Uint32(b[4:8])) / float64(scale)
		res.LoadAverages[2] = float64(binary.LittleEndian.Uint32(b[8:12])) / float64(scale)
	}
	if tv, err := unix.SysctlTimeval("kern.boottime"); err == nil {
		now := time.Now().Unix()
		if now > int64(tv.Sec) {
			res.UptimeSeconds = uint64(now - int64(tv.Sec))
		}
	}
}

func inspectMemory(m *MemoryHealth) {
	if mem, err := unix.SysctlUint64("hw.memsize"); err == nil && mem > 0 {
		m.TotalBytes = mem
		m.TotalMB = mem / (1024 * 1024)
		m.AvailBytes = mem / 2
		m.AvailableMB = m.AvailBytes / (1024 * 1024)
		if m.TotalBytes > m.AvailBytes {
			m.UsedBytes = m.TotalBytes - m.AvailBytes
			m.UsedMB = m.UsedBytes / (1024 * 1024)
		}
		if m.TotalBytes > 0 {
			m.UsedPercent = float64(m.UsedBytes) / float64(m.TotalBytes) * 100.0
		}
	}
}
