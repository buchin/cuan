package server

import (
	"context"
	"strings"
	"testing"
)

func TestServerTelemetry_Summary(t *testing.T) {
	telem := ServerTelemetry{
		DiskTotalBytes:    40 * 1024 * 1024 * 1024,
		DiskFreeBytes:     12 * 1024 * 1024 * 1024,
		DiskUsedBytes:     28 * 1024 * 1024 * 1024,
		DiskUsedPercent:   70.0,
		DiskPath:          "/opt/cuan",
		DiskWarning:       false,
		MemoryTotalBytes:  8 * 1024 * 1024 * 1024,
		MemoryAvailBytes:  3 * 1024 * 1024 * 1024,
		MemoryUsedBytes:   5 * 1024 * 1024 * 1024,
		MemoryUsedPercent: 62.5,
		DatabaseStatus:    "connected",
		DatabaseLatencyMs: 2,
		UptimeSeconds:     90000, // 1d 1h
		LoadAverages:      [3]float64{0.15, 0.25, 0.30},
	}

	summary := telem.Summary()
	if !strings.Contains(summary, "Server Infrastructure & Host Telemetry:") {
		t.Fatalf("expected header in summary, got: %s", summary)
	}
	if !strings.Contains(summary, "- Host Disk: 12.0 GiB free of 40.0 GiB (70.0% used, path: /opt/cuan) [HEALTHY]") {
		t.Fatalf("expected healthy disk line, got: %s", summary)
	}
	if !strings.Contains(summary, "- Host Memory (RAM): 3.0 GiB available of 8.0 GiB (62.5% used)") {
		t.Fatalf("expected memory line, got: %s", summary)
	}
	if !strings.Contains(summary, "- Database: connected (latency: 2ms)") {
		t.Fatalf("expected database line, got: %s", summary)
	}
	if !strings.Contains(summary, "- System Uptime: 1d 1h, Load Average: 0.15, 0.25, 0.30") {
		t.Fatalf("expected uptime/load line, got: %s", summary)
	}
	if strings.Contains(summary, "⚠️ INFRASTRUCTURE WARNING") {
		t.Fatalf("unexpected warning in healthy summary: %s", summary)
	}

	// Now test warning condition
	telem.DiskUsedPercent = 85.5
	telem.DiskFreeBytes = 1 * 1024 * 1024 * 1024 // 1 GiB
	telem.DiskWarning = true

	warnSummary := telem.Summary()
	if !strings.Contains(warnSummary, "[CRITICAL / LOW SPACE]") {
		t.Fatalf("expected critical tag in summary, got: %s", warnSummary)
	}
	if !strings.Contains(warnSummary, "⚠️ INFRASTRUCTURE WARNING: Host disk space is at 85.5% used") {
		t.Fatalf("expected warning banner, got: %s", warnSummary)
	}
}

func TestServerTelemetry_GetSystemResources(t *testing.T) {
	ctx := context.Background()
	s := &Server{}
	telem := s.getServerTelemetry(ctx, "/")

	if telem.DiskTotalBytes == 0 {
		t.Fatalf("expected non-zero disk total bytes, got: %d", telem.DiskTotalBytes)
	}
	if telem.DiskPath == "" {
		t.Fatalf("expected non-empty disk path")
	}

	summary := telem.Summary()
	if !strings.Contains(summary, "Server Infrastructure & Host Telemetry:") {
		t.Fatalf("expected header, got: %s", summary)
	}
}

func TestServerTelemetry_FormatHelpers(t *testing.T) {
	if got := formatBytes(500); got != "500 B" {
		t.Fatalf("expected 500 B, got %s", got)
	}
	if got := formatBytes(1024 * 5); got != "5.0 KiB" {
		t.Fatalf("expected 5.0 KiB, got %s", got)
	}
	if got := formatBytes(1024 * 1024 * 50); got != "50.0 MiB" {
		t.Fatalf("expected 50.0 MiB, got %s", got)
	}
	if got := formatBytes(1024 * 1024 * 1024 * 12); got != "12.0 GiB" {
		t.Fatalf("expected 12.0 GiB, got %s", got)
	}

	if got := formatUptime(45); got != "0m" {
		t.Fatalf("expected 0m, got %s", got)
	}
	if got := formatUptime(120); got != "2m" {
		t.Fatalf("expected 2m, got %s", got)
	}
	if got := formatUptime(7200); got != "2h 0m" {
		t.Fatalf("expected 2h 0m, got %s", got)
	}
	if got := formatUptime(90000); got != "1d 1h" {
		t.Fatalf("expected 1d 1h, got %s", got)
	}
}
