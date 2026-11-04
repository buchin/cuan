package server

import "testing"

func TestTaskStorageAdmissionUsesAvailableBytes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		resources ServerTelemetry
		blocked   bool
	}{
		{"no unreserved space", ServerTelemetry{DiskTotalBytes: 50 << 30, DiskFreeBytes: 0}, true},
		{"below admission floor", ServerTelemetry{DiskTotalBytes: 50 << 30, DiskFreeBytes: (2 << 30) - 1}, true},
		{"at floor", ServerTelemetry{DiskTotalBytes: 50 << 30, DiskFreeBytes: 2 << 30}, false},
		{"high usage but usable space", ServerTelemetry{DiskTotalBytes: 1 << 40, DiskFreeBytes: 4 << 30, DiskUsedPercent: 99.6}, false},
		{"unsupported telemetry is not measured zero space", ServerTelemetry{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if taskStorageCritical(tc.resources) != tc.blocked {
				t.Fatal("incorrect storage admission")
			}
		})
	}
}
