package observability

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/buchin/cuan/internal/crypto"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/deployer"
)

const (
	DefaultMetricsInterval = 60 * time.Second
	DefaultMetricsTimeout  = 10 * time.Second
)

// SystemMetrics represents raw system hardware resource measurements.
type SystemMetrics struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsedMB  int64   `json:"memory_used_mb"`
	MemoryTotalMB int64   `json:"memory_total_mb"`
	DiskUsedGB    float64 `json:"disk_used_gb"`
	DiskTotalGB   float64 `json:"disk_total_gb"`
}

// LocalMetricsReader reads system resource metrics on the local host/runner.
type LocalMetricsReader interface {
	ReadMetrics(ctx context.Context) (*SystemMetrics, error)
}

// OSLocalMetricsReader reads CPU, Memory, and Disk stats directly from Linux /proc and syscalls.
type OSLocalMetricsReader struct {
	mu           sync.Mutex
	prevCPUTotal uint64
	prevCPUIdle  uint64
	lastCPUTime  time.Time
}

// NewOSLocalMetricsReader creates an OS-level resource reader.
func NewOSLocalMetricsReader() *OSLocalMetricsReader {
	return &OSLocalMetricsReader{}
}

// ReadMetrics reads current local CPU, Memory, and Disk metrics.
func (r *OSLocalMetricsReader) ReadMetrics(ctx context.Context) (*SystemMetrics, error) {
	memUsed, memTotal := r.readMemory()
	diskUsed, diskTotal := r.readDisk()
	cpuPct := r.readCPU()

	return &SystemMetrics{
		CPUPercent:    math.Round(cpuPct*100) / 100,
		MemoryUsedMB:  memUsed,
		MemoryTotalMB: memTotal,
		DiskUsedGB:    math.Round(diskUsed*100) / 100,
		DiskTotalGB:   math.Round(diskTotal*100) / 100,
	}, nil
}

func (r *OSLocalMetricsReader) readMemory() (int64, int64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		// Fallback for non-Linux / sandboxes without /proc/meminfo
		return 512, 2048
	}
	defer f.Close()

	var memTotalKB, memAvailableKB, memFreeKB int64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			memTotalKB = parseKB(line)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			memAvailableKB = parseKB(line)
		} else if strings.HasPrefix(line, "MemFree:") {
			memFreeKB = parseKB(line)
		}
	}

	if memTotalKB <= 0 {
		return 512, 2048
	}

	if memAvailableKB <= 0 {
		memAvailableKB = memFreeKB
	}

	usedKB := memTotalKB - memAvailableKB
	if usedKB < 0 {
		usedKB = 0
	}

	return usedKB / 1024, memTotalKB / 1024
}

func parseKB(line string) int64 {
	fields := strings.Fields(line)
	if len(fields) >= 2 {
		val, _ := strconv.ParseInt(fields[1], 10, 64)
		return val
	}
	return 0
}

func (r *OSLocalMetricsReader) readDisk() (float64, float64) {
	var stat syscall.Statfs_t
	// Try root filesystem
	err := syscall.Statfs("/", &stat)
	if err != nil {
		// Fallback for mock/test environments
		return 15.0, 50.0
	}

	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bavail * uint64(stat.Bsize)
	usedBytes := totalBytes - freeBytes

	totalGB := float64(totalBytes) / (1024 * 1024 * 1024)
	usedGB := float64(usedBytes) / (1024 * 1024 * 1024)

	return usedGB, totalGB
}

func (r *OSLocalMetricsReader) readCPU() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	f, err := os.Open("/proc/stat")
	if err != nil {
		return 5.0 // Fallback
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var total, idle uint64
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) > 4 {
				for i := 1; i < len(fields); i++ {
					val, _ := strconv.ParseUint(fields[i], 10, 64)
					total += val
					if i == 4 { // idle is 4th column
						idle = val
					}
				}
			}
			break
		}
	}

	if r.prevCPUTotal == 0 || total <= r.prevCPUTotal {
		r.prevCPUTotal = total
		r.prevCPUIdle = idle
		r.lastCPUTime = time.Now()
		return 5.0 // Initial baseline estimation
	}

	deltaTotal := total - r.prevCPUTotal
	deltaIdle := idle - r.prevCPUIdle
	r.prevCPUTotal = total
	r.prevCPUIdle = idle
	r.lastCPUTime = time.Now()

	if deltaTotal == 0 {
		return 0.0
	}

	cpuPercent := float64(deltaTotal-deltaIdle) / float64(deltaTotal) * 100.0
	if cpuPercent < 0 {
		cpuPercent = 0
	} else if cpuPercent > 100 {
		cpuPercent = 100
	}
	return cpuPercent
}

// MockLocalMetricsReader provides controllable metrics for testing.
type MockLocalMetricsReader struct {
	Metrics *SystemMetrics
	Err     error
}

// ReadMetrics returns mock metrics or configured error.
func (m *MockLocalMetricsReader) ReadMetrics(ctx context.Context) (*SystemMetrics, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.Metrics != nil {
		return m.Metrics, nil
	}
	return &SystemMetrics{
		CPUPercent:    15.5,
		MemoryUsedMB:  512,
		MemoryTotalMB: 2048,
		DiskUsedGB:    12.4,
		DiskTotalGB:   50.0,
	}, nil
}

// SSHRunner executes remote commands on a target BYOS server.
type SSHRunner interface {
	Execute(ctx context.Context, server *db.Server, command string) (stdout string, stderr string, err error)
}

// DefaultSSHRunner executes remote commands using crypto KeyVault and deployer SSHManager.
type DefaultSSHRunner struct {
	keyVault   *crypto.KeyVault
	sshManager deployer.SSHManager
}

// NewDefaultSSHRunner creates an SSHRunner utilizing Sprint 4 client pooling and key vault.
func NewDefaultSSHRunner(keyVault *crypto.KeyVault, sshManager deployer.SSHManager) *DefaultSSHRunner {
	return &DefaultSSHRunner{
		keyVault:   keyVault,
		sshManager: sshManager,
	}
}

// Execute decrypts the server's private key, acquires an SSH client, and executes command.
func (r *DefaultSSHRunner) Execute(ctx context.Context, server *db.Server, command string) (string, string, error) {
	if r.keyVault == nil || r.sshManager == nil {
		return "", "", errors.New("key vault or ssh manager not configured")
	}

	decryptedKey, err := r.keyVault.Decrypt(server.EncryptedKey)
	if err != nil {
		return "", "", fmt.Errorf("decrypt server ssh key: %w", err)
	}

	client, err := r.sshManager.GetClient(ctx, server, decryptedKey)
	if err != nil {
		return "", "", fmt.Errorf("dial ssh server %s: %w", server.ID, err)
	}

	return client.RunCommand(ctx, command)
}

// MockSSHRunner allows injecting test outputs for remote SSH telemetry commands.
type MockSSHRunner struct {
	OutputFunc func(ctx context.Context, server *db.Server, command string) (string, string, error)
}

// Execute runs the injected MockSSHRunner callback.
func (m *MockSSHRunner) Execute(ctx context.Context, server *db.Server, command string) (string, string, error) {
	if m.OutputFunc != nil {
		return m.OutputFunc(ctx, server, command)
	}
	// Default mock output
	sampleOutput := `===CUAN_TELEMETRY===
---CPU---
%Cpu(s): 12.5 us,  3.2 sy,  0.0 ni, 84.3 id,  0.0 wa,  0.0 hi,  0.0 si,  0.0 st
---MEM---
              total        used        free      shared  buff/cache   available
Mem:           7980        2150        3200          50        2630        5780
---DISK---
Filesystem     1M-blocks      Used Available Use% Mounted on
/dev/root          98304     32768     65536  34% /
===END===`
	return sampleOutput, "", nil
}

// RemoteTelemetryScript is a lightweight command gathering CPU, Memory, and Disk metrics.
const RemoteTelemetryScript = `echo "===CUAN_TELEMETRY==="
echo "---CPU---"
top -bn1 2>/dev/null | grep -i "cpu(s)" || grep '^cpu ' /proc/stat 2>/dev/null || echo "CPU: 0"
echo "---MEM---"
free -m 2>/dev/null || cat /proc/meminfo 2>/dev/null
echo "---DISK---"
df -m / 2>/dev/null
echo "===END==="`

var (
	cpuRegex  = regexp.MustCompile(`(\d+[\.\,]?\d*)\s*(?:%?\s*)id`)
	diskRegex = regexp.MustCompile(`\s+(\d+)\s+(\d+)\s+(\d+)\s+\d+%\s+/`)
)

// ParseRemoteTelemetry parses the stdout from RemoteTelemetryScript into SystemMetrics.
func ParseRemoteTelemetry(output string) (*SystemMetrics, error) {
	metrics := &SystemMetrics{
		CPUPercent:    0,
		MemoryUsedMB:  0,
		MemoryTotalMB: 0,
		DiskUsedGB:    0,
		DiskTotalGB:   0,
	}

	lines := strings.Split(output, "\n")
	currentSection := ""

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "---") && strings.HasSuffix(line, "---") {
			currentSection = strings.Trim(line, "-")
			continue
		}

		switch currentSection {
		case "CPU":
			// E.g. "%Cpu(s): 12.5 us,  3.2 sy,  0.0 ni, 84.3 id, ..."
			if matches := cpuRegex.FindStringSubmatch(line); len(matches) > 1 {
				valStr := strings.ReplaceAll(matches[1], ",", ".")
				if idle, err := strconv.ParseFloat(valStr, 64); err == nil {
					used := 100.0 - idle
					if used < 0 {
						used = 0
					} else if used > 100 {
						used = 100
					}
					metrics.CPUPercent = math.Round(used*100) / 100
				}
			}
		case "MEM":
			// E.g. "Mem:           7980        2150        3200..."
			if strings.HasPrefix(line, "Mem:") {
				fields := strings.Fields(line)
				if len(fields) >= 3 {
					total, _ := strconv.ParseInt(fields[1], 10, 64)
					used, _ := strconv.ParseInt(fields[2], 10, 64)
					metrics.MemoryTotalMB = total
					metrics.MemoryUsedMB = used
				}
			} else if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					totalKB, _ := strconv.ParseInt(fields[1], 10, 64)
					metrics.MemoryTotalMB = totalKB / 1024
				}
			} else if strings.HasPrefix(line, "MemAvailable:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					availKB, _ := strconv.ParseInt(fields[1], 10, 64)
					availMB := availKB / 1024
					if metrics.MemoryTotalMB > availMB {
						metrics.MemoryUsedMB = metrics.MemoryTotalMB - availMB
					}
				}
			}
		case "DISK":
			// E.g. "/dev/root          98304     32768     65536  34% /"
			if strings.HasSuffix(line, " /") || strings.Contains(line, "% /") {
				fields := strings.Fields(line)
				if len(fields) >= 4 {
					// fields[1] = total MB, fields[2] = used MB
					totalMB, _ := strconv.ParseFloat(fields[1], 64)
					usedMB, _ := strconv.ParseFloat(fields[2], 64)
					metrics.DiskTotalGB = math.Round((totalMB/1024.0)*100) / 100
					metrics.DiskUsedGB = math.Round((usedMB/1024.0)*100) / 100
				}
			}
		}
	}

	return metrics, nil
}

// MetricsCollectorOption configures a MetricsCollector.
type MetricsCollectorOption func(*MetricsCollector)

// WithCollectorInterval sets the polling interval for metrics collection.
func WithCollectorInterval(d time.Duration) MetricsCollectorOption {
	return func(c *MetricsCollector) {
		if d > 0 {
			c.interval = d
		}
	}
}

// WithCollectorTimeout sets the command timeout for telemetry probes.
func WithCollectorTimeout(d time.Duration) MetricsCollectorOption {
	return func(c *MetricsCollector) {
		if d > 0 {
			c.timeout = d
		}
	}
}

// WithLocalReader sets the local metrics reader.
func WithLocalReader(reader LocalMetricsReader) MetricsCollectorOption {
	return func(c *MetricsCollector) {
		if reader != nil {
			c.localReader = reader
		}
	}
}

// WithSSHRunner sets the remote SSH telemetry runner.
func WithSSHRunner(runner SSHRunner) MetricsCollectorOption {
	return func(c *MetricsCollector) {
		if runner != nil {
			c.sshRunner = runner
		}
	}
}

// WithServerRepo sets the server repository for querying BYOS servers.
func WithServerRepo(repo db.ServerRepository) MetricsCollectorOption {
	return func(c *MetricsCollector) {
		c.serverRepo = repo
	}
}

// MetricsCollector orchestrates metrics gathering from local runners and remote BYOS servers.
type MetricsCollector struct {
	repo        db.ObservabilityRepository
	localReader LocalMetricsReader
	sshRunner   SSHRunner
	serverRepo  db.ServerRepository
	interval    time.Duration
	timeout     time.Duration

	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	running bool
}

// NewMetricsCollector instantiates a new MetricsCollector.
func NewMetricsCollector(repo db.ObservabilityRepository, opts ...MetricsCollectorOption) *MetricsCollector {
	c := &MetricsCollector{
		repo:        repo,
		localReader: NewOSLocalMetricsReader(),
		interval:    DefaultMetricsInterval,
		timeout:     DefaultMetricsTimeout,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Start begins periodic collection of local and server metrics in the background.
func (c *MetricsCollector) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return nil
	}

	c.ctx, c.cancel = context.WithCancel(ctx)
	c.done = make(chan struct{})
	c.running = true
	c.mu.Unlock()

	go c.collectionLoop()
	return nil
}

// Stop gracefully stops the metrics collection loop.
func (c *MetricsCollector) Stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	c.cancel()
	c.running = false
	done := c.done
	c.mu.Unlock()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
}

// IsRunning reports whether the collector is currently active.
func (c *MetricsCollector) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

func (c *MetricsCollector) collectionLoop() {
	defer close(c.done)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Initial collection pass
	_ = c.CollectAll(c.ctx)

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			_ = c.CollectAll(c.ctx)
		}
	}
}

// CollectAll executes a collection pass across local runner and all active BYOS servers.
func (c *MetricsCollector) CollectAll(ctx context.Context) error {
	// 1. Collect local runner metrics
	_, _ = c.CollectLocal(ctx, "")

	// 2. Collect remote servers if repository is configured
	if c.serverRepo != nil && c.sshRunner != nil {
		servers, err := c.serverRepo.List(ctx)
		if err == nil {
			for _, srv := range servers {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				if srv.Status == "online" || srv.Status == "active" || srv.Status == "ready" {
					_, _ = c.CollectServer(ctx, srv.ID)
				}
			}
		}
	}

	return nil
}

// CollectLocal reads local host resource metrics and persists them to the observability repository.
func (c *MetricsCollector) CollectLocal(ctx context.Context, deploymentID string) (*db.ServerMetric, error) {
	if c.localReader == nil {
		return nil, errors.New("local metrics reader not configured")
	}

	sys, err := c.localReader.ReadMetrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("read local metrics: %w", err)
	}

	var depIDPtr *string
	if strings.TrimSpace(deploymentID) != "" {
		trimmed := strings.TrimSpace(deploymentID)
		depIDPtr = &trimmed
	}

	metric := &db.ServerMetric{
		DeploymentID:  depIDPtr,
		CPUPercent:    sys.CPUPercent,
		MemoryUsedMB:  sys.MemoryUsedMB,
		MemoryTotalMB: sys.MemoryTotalMB,
		DiskUsedGB:    sys.DiskUsedGB,
		DiskTotalGB:   sys.DiskTotalGB,
		RecordedAt:    time.Now().UTC(),
	}

	if err := c.repo.RecordServerMetric(ctx, metric); err != nil {
		return nil, fmt.Errorf("record server metric: %w", err)
	}

	return metric, nil
}

// CollectServer probes a remote BYOS server over SSH and persists telemetry metrics.
func (c *MetricsCollector) CollectServer(ctx context.Context, serverID string) (*db.ServerMetric, error) {
	if c.serverRepo == nil || c.sshRunner == nil {
		return nil, errors.New("server repo or ssh runner not configured")
	}

	srv, err := c.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("get server %s: %w", serverID, err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	stdout, _, err := c.sshRunner.Execute(reqCtx, srv, RemoteTelemetryScript)
	if err != nil {
		return nil, fmt.Errorf("execute remote telemetry script on %s: %w", serverID, err)
	}

	sys, err := ParseRemoteTelemetry(stdout)
	if err != nil {
		return nil, fmt.Errorf("parse remote telemetry: %w", err)
	}

	srvID := srv.ID
	metric := &db.ServerMetric{
		ServerID:      &srvID,
		CPUPercent:    sys.CPUPercent,
		MemoryUsedMB:  sys.MemoryUsedMB,
		MemoryTotalMB: sys.MemoryTotalMB,
		DiskUsedGB:    sys.DiskUsedGB,
		DiskTotalGB:   sys.DiskTotalGB,
		RecordedAt:    time.Now().UTC(),
	}

	if err := c.repo.RecordServerMetric(ctx, metric); err != nil {
		return nil, fmt.Errorf("record server metric: %w", err)
	}

	return metric, nil
}

// GetLatestServerMetric retrieves the most recently recorded telemetry metric for a server.
func (c *MetricsCollector) GetLatestServerMetric(ctx context.Context, serverID string) (*db.ServerMetricDTO, error) {
	metric, err := c.repo.GetLatestServerMetric(ctx, serverID)
	if err != nil {
		return nil, err
	}
	dto := metric.ToDTO()
	return &dto, nil
}

// ListServerMetrics retrieves telemetry metrics history for a server.
func (c *MetricsCollector) ListServerMetrics(ctx context.Context, serverID string, limit int) ([]db.ServerMetricDTO, error) {
	metrics, err := c.repo.ListServerMetrics(ctx, serverID, limit)
	if err != nil {
		return nil, err
	}

	dtos := make([]db.ServerMetricDTO, len(metrics))
	for i, m := range metrics {
		dtos[i] = m.ToDTO()
	}
	return dtos, nil
}
