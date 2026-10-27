package server

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ServerTelemetry contains host and infrastructure health metrics injected
// into the Scrum Master context snapshot to detect resource blockers early.
type ServerTelemetry struct {
	DiskTotalBytes    uint64     `json:"disk_total_bytes"`
	DiskFreeBytes     uint64     `json:"disk_free_bytes"`
	DiskUsedBytes     uint64     `json:"disk_used_bytes"`
	DiskUsedPercent   float64    `json:"disk_used_percent"`
	DiskPath          string     `json:"disk_path"`
	DiskWarning       bool       `json:"disk_warning"`
	MemoryTotalBytes  uint64     `json:"memory_total_bytes"`
	MemoryAvailBytes  uint64     `json:"memory_avail_bytes"`
	MemoryUsedBytes   uint64     `json:"memory_used_bytes"`
	MemoryUsedPercent float64    `json:"memory_used_percent"`
	DatabaseStatus    string     `json:"database_status"`
	DatabaseLatencyMs int64      `json:"database_latency_ms"`
	UptimeSeconds     uint64     `json:"uptime_seconds"`
	LoadAverages      [3]float64 `json:"load_averages"`
}

func (t ServerTelemetry) Summary() string {
	var b strings.Builder
	b.WriteString("Server Infrastructure & Host Telemetry:\n")

	// Disk
	diskStatus := "HEALTHY"
	if t.DiskWarning {
		diskStatus = "CRITICAL / LOW SPACE"
	}
	path := t.DiskPath
	if path == "" {
		path = "/"
	}
	fmt.Fprintf(&b, "- Host Disk: %s free of %s (%.1f%% used, path: %s) [%s]\n",
		formatBytes(t.DiskFreeBytes), formatBytes(t.DiskTotalBytes), t.DiskUsedPercent, path, diskStatus)

	// Memory
	if t.MemoryTotalBytes > 0 {
		fmt.Fprintf(&b, "- Host Memory (RAM): %s available of %s (%.1f%% used)\n",
			formatBytes(t.MemoryAvailBytes), formatBytes(t.MemoryTotalBytes), t.MemoryUsedPercent)
	}

	// Database
	if t.DatabaseStatus != "" {
		if t.DatabaseLatencyMs > 0 {
			fmt.Fprintf(&b, "- Database: %s (latency: %dms)\n", t.DatabaseStatus, t.DatabaseLatencyMs)
		} else {
			fmt.Fprintf(&b, "- Database: %s\n", t.DatabaseStatus)
		}
	}

	// Uptime & Load
	if t.UptimeSeconds > 0 {
		fmt.Fprintf(&b, "- System Uptime: %s, Load Average: %.2f, %.2f, %.2f\n",
			formatUptime(t.UptimeSeconds), t.LoadAverages[0], t.LoadAverages[1], t.LoadAverages[2])
	}

	// Critical warning banner
	if t.DiskWarning {
		fmt.Fprintf(&b, "⚠️ INFRASTRUCTURE WARNING: Host disk space is at %.1f%% used (%s free). Builds, task runners, and agent workspaces risk unexpected crashes or failure. Flag infrastructure constraints proactively to the user.\n",
			t.DiskUsedPercent, formatBytes(t.DiskFreeBytes))
	}

	return b.String()
}

func (s *Server) getServerTelemetry(ctx context.Context, checkPath string) ServerTelemetry {
	t := getSystemResources(checkPath)

	if s != nil && s.db != nil {
		start := time.Now()
		dbCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := s.db.PingContext(dbCtx); err != nil {
			t.DatabaseStatus = "disconnected: " + err.Error()
		} else {
			t.DatabaseStatus = "connected"
			t.DatabaseLatencyMs = time.Since(start).Milliseconds()
		}
	} else {
		t.DatabaseStatus = "connected"
	}

	return t
}

func formatBytes(bytes uint64) string {
	const (
		kib = 1024
		mib = kib * 1024
		gib = mib * 1024
		tib = gib * 1024
	)

	switch {
	case bytes >= tib:
		return fmt.Sprintf("%.1f TiB", float64(bytes)/float64(tib))
	case bytes >= gib:
		return fmt.Sprintf("%.1f GiB", float64(bytes)/float64(gib))
	case bytes >= mib:
		return fmt.Sprintf("%.1f MiB", float64(bytes)/float64(mib))
	case bytes >= kib:
		return fmt.Sprintf("%.1f KiB", float64(bytes)/float64(kib))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func formatUptime(seconds uint64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
