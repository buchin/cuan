//go:build !linux && !darwin

package diagnostics

func collectSystemLoad(res *SystemHealthResult) {
	// On unsupported platforms, sysinfo metrics are left at their zero values.
}

func inspectMemory(m *MemoryHealth) {
	// On unsupported platforms, memory metrics are left at their zero values.
}
