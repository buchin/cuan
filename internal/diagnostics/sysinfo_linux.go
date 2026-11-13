//go:build linux

package diagnostics

import "syscall"

func collectSystemLoad(res *SystemHealthResult) {
	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err == nil {
		res.UptimeSeconds = uint64(si.Uptime)
		res.LoadAverages[0] = float64(si.Loads[0]) / 65536.0
		res.LoadAverages[1] = float64(si.Loads[1]) / 65536.0
		res.LoadAverages[2] = float64(si.Loads[2]) / 65536.0
	}
}

func inspectMemory(m *MemoryHealth) {
	inspectMemoryLinux(m)
}
