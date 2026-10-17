package observability

import (
	"context"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
)

func TestParseRemoteTelemetry(t *testing.T) {
	rawOutput := `===CUAN_TELEMETRY===
---CPU---
%Cpu(s): 22.4 us,  5.1 sy,  0.0 ni, 72.5 id,  0.0 wa,  0.0 hi,  0.0 si,  0.0 st
---MEM---
              total        used        free      shared  buff/cache   available
Mem:          15950        4230        8120         120        3600       11600
---DISK---
Filesystem     1M-blocks      Used Available Use% Mounted on
/dev/root         204800     81920    122880  40% /
===END===`

	metrics, err := ParseRemoteTelemetry(rawOutput)
	if err != nil {
		t.Fatalf("ParseRemoteTelemetry failed: %v", err)
	}

	// CPU: 100 - 72.5 = 27.5
	if metrics.CPUPercent != 27.5 {
		t.Errorf("expected CPU 27.5%%, got %f", metrics.CPUPercent)
	}

	// Memory: total 15950 MB, used 4230 MB
	if metrics.MemoryTotalMB != 15950 {
		t.Errorf("expected MemoryTotalMB 15950, got %d", metrics.MemoryTotalMB)
	}
	if metrics.MemoryUsedMB != 4230 {
		t.Errorf("expected MemoryUsedMB 4230, got %d", metrics.MemoryUsedMB)
	}

	// Disk: 204800 MB / 1024 = 200 GB, 81920 MB / 1024 = 80 GB
	if metrics.DiskTotalGB != 200.0 {
		t.Errorf("expected DiskTotalGB 200.0, got %f", metrics.DiskTotalGB)
	}
	if metrics.DiskUsedGB != 80.0 {
		t.Errorf("expected DiskUsedGB 80.0, got %f", metrics.DiskUsedGB)
	}
}

func TestOSLocalMetricsReader_ReadMetrics(t *testing.T) {
	reader := NewOSLocalMetricsReader()
	metrics, err := reader.ReadMetrics(context.Background())
	if err != nil {
		t.Fatalf("OSLocalMetricsReader.ReadMetrics failed: %v", err)
	}

	if metrics == nil {
		t.Fatal("expected non-nil system metrics")
	}

	if metrics.MemoryTotalMB <= 0 {
		t.Errorf("expected positive MemoryTotalMB, got %d", metrics.MemoryTotalMB)
	}
	if metrics.DiskTotalGB <= 0 {
		t.Errorf("expected positive DiskTotalGB, got %f", metrics.DiskTotalGB)
	}
}

func TestMetricsCollector_LocalAndRemote(t *testing.T) {
	repo := db.NewMemoryObservabilityRepository()
	serverRepo := db.NewMemoryServerRepository()
	ctx := context.Background()

	// Register a mock server
	srv := &db.Server{
		ID:           "srv-1",
		Name:         "Production VPS",
		Host:         "192.168.1.10",
		Port:         22,
		Username:     "ubuntu",
		Status:       "active",
		EncryptedKey: "mock-encrypted-key",
	}
	_ = serverRepo.Create(ctx, srv)

	mockReader := &MockLocalMetricsReader{
		Metrics: &SystemMetrics{
			CPUPercent:    18.5,
			MemoryUsedMB:  1024,
			MemoryTotalMB: 4096,
			DiskUsedGB:    25.0,
			DiskTotalGB:   100.0,
		},
	}

	mockSSH := &MockSSHRunner{
		OutputFunc: func(ctx context.Context, server *db.Server, command string) (string, string, error) {
			sampleOutput := `===CUAN_TELEMETRY===
---CPU---
%Cpu(s): 10.0 us,  2.0 sy,  0.0 ni, 88.0 id,  0.0 wa,  0.0 hi,  0.0 si,  0.0 st
---MEM---
Mem:           8192        2048        5120         100        1024        6144
---DISK---
Filesystem     1M-blocks      Used Available Use% Mounted on
/dev/root         102400     40960     61440  40% /
===END===`
			return sampleOutput, "", nil
		},
	}

	collector := NewMetricsCollector(
		repo,
		WithLocalReader(mockReader),
		WithSSHRunner(mockSSH),
		WithServerRepo(serverRepo),
		WithCollectorInterval(100*time.Millisecond),
	)

	// 1. Collect Local Metrics
	localMetric, err := collector.CollectLocal(ctx, "dep-local-1")
	if err != nil {
		t.Fatalf("CollectLocal failed: %v", err)
	}
	if localMetric.CPUPercent != 18.5 {
		t.Errorf("expected local CPU 18.5, got %f", localMetric.CPUPercent)
	}
	if localMetric.MemoryUsedMB != 1024 {
		t.Errorf("expected local memory 1024MB, got %d", localMetric.MemoryUsedMB)
	}

	// 2. Collect Remote Server Metrics
	remoteMetric, err := collector.CollectServer(ctx, "srv-1")
	if err != nil {
		t.Fatalf("CollectServer failed: %v", err)
	}
	if remoteMetric.CPUPercent != 12.0 { // 100 - 88 = 12
		t.Errorf("expected remote CPU 12.0, got %f", remoteMetric.CPUPercent)
	}
	if remoteMetric.MemoryUsedMB != 2048 {
		t.Errorf("expected remote memory 2048MB, got %d", remoteMetric.MemoryUsedMB)
	}
	if remoteMetric.DiskTotalGB != 100.0 { // 102400 / 1024 = 100
		t.Errorf("expected remote disk 100GB, got %f", remoteMetric.DiskTotalGB)
	}

	// 3. Query Latest and List Metrics
	latest, err := collector.GetLatestServerMetric(ctx, "srv-1")
	if err != nil {
		t.Fatalf("GetLatestServerMetric failed: %v", err)
	}
	if latest.CPUPercent != 12.0 {
		t.Errorf("expected latest CPU 12.0, got %f", latest.CPUPercent)
	}

	list, err := collector.ListServerMetrics(ctx, "srv-1", 10)
	if err != nil {
		t.Fatalf("ListServerMetrics failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 metric in list, got %d", len(list))
	}

	// 4. Test Background Lifecycle
	if err := collector.Start(ctx); err != nil {
		t.Fatalf("collector.Start failed: %v", err)
	}
	if !collector.IsRunning() {
		t.Errorf("expected collector to be running")
	}

	time.Sleep(150 * time.Millisecond)

	collector.Stop()
	if collector.IsRunning() {
		t.Errorf("expected collector to be stopped")
	}
}
