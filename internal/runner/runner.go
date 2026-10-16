package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
)

type Runner struct {
	cfg        *config.Config
	adapter    agent.Adapter
	httpClient *http.Client
	baseURL    string

	mu             sync.Mutex
	currentAttempt *task.TaskAttempt
	cancelAttempt  context.CancelFunc
}

func NewRunner(cfg *config.Config, adapter agent.Adapter, customHTTPClient ...*http.Client) *Runner {
	client := http.DefaultClient
	if len(customHTTPClient) > 0 && customHTTPClient[0] != nil {
		client = customHTTPClient[0]
	}

	baseURL := strings.TrimRight(cfg.RunnerServerURL, "/")

	return &Runner{
		cfg:        cfg,
		adapter:    adapter,
		httpClient: client,
		baseURL:    baseURL,
	}
}

// Start runs the runner daemon: background heartbeat & queue polling loop.
func (r *Runner) Start(ctx context.Context) error {
	version, err := r.adapter.Version()
	if err != nil {
		return fmt.Errorf("check agent adapter: %w", err)
	}

	hostname, _ := os.Hostname()

	log.Printf("[Runner %s] Started on host '%s' with agent '%s' (ver: %s)", r.cfg.RunnerID, hostname, r.adapter.Name(), version)
	log.Printf("[Runner %s] Outbound server URL: %s, poll interval: %v", r.cfg.RunnerID, r.baseURL, r.cfg.RunnerInterval)

	// Immediate initial heartbeat
	r.sendHeartbeat(ctx, hostname)

	// Heartbeat ticker (every 3 seconds)
	heartbeatTicker := time.NewTicker(3 * time.Second)
	defer heartbeatTicker.Stop()

	// Queue poll ticker
	pollTicker := time.NewTicker(r.cfg.RunnerInterval)
	defer pollTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[Runner %s] Shutdown signaled, terminating active work...", r.cfg.RunnerID)
			r.cancelCurrentExecution()
			return nil

		case <-heartbeatTicker.C:
			r.sendHeartbeat(ctx, hostname)

		case <-pollTicker.C:
			r.pollAndExecute(ctx)
		}
	}
}

func (r *Runner) sendHeartbeat(ctx context.Context, hostname string) {
	metrics := CollectHostMetrics()

	r.mu.Lock()
	attemptID := ""
	if r.currentAttempt != nil {
		attemptID = r.currentAttempt.ID
	}
	r.mu.Unlock()

	reqPayload := HeartbeatRequest{
		RunnerID:         r.cfg.RunnerID,
		Hostname:         hostname,
		SupportedAgents:  []string{r.adapter.Name()},
		CurrentAttemptID: attemptID,
		CPUPercent:       metrics.CPUPercent,
		MemoryUsedBytes:  metrics.MemoryUsedBytes,
		MemoryTotalBytes: metrics.MemoryTotalBytes,
		DiskUsedBytes:    metrics.DiskUsedBytes,
		DiskTotalBytes:   metrics.DiskTotalBytes,
	}

	resp, err := r.doRequest(ctx, http.MethodPost, "/api/v1/runner/heartbeat", reqPayload)
	if err != nil {
		log.Printf("[Runner %s] Heartbeat failed: %v", r.cfg.RunnerID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var hbResp HeartbeatResponse
		if err := json.NewDecoder(resp.Body).Decode(&hbResp); err == nil {
			if hbResp.ShouldCancel {
				log.Printf("[Runner %s] Heartbeat received cancellation directive for active attempt %s", r.cfg.RunnerID, attemptID)
				r.cancelCurrentExecution()
			}
		}
	}
}

func (r *Runner) pollAndExecute(ctx context.Context) {
	// CONCURRENCY LIMIT: if currently busy executing, do not claim another task!
	r.mu.Lock()
	if r.currentAttempt != nil {
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()

	claimReq := ClaimTaskRequest{
		RunnerID:        r.cfg.RunnerID,
		SupportedAgents: []string{r.adapter.Name()},
	}

	resp, err := r.doRequest(ctx, http.MethodPost, "/api/v1/runner/claim", claimReq)
	if err != nil {
		log.Printf("[Runner %s] Claim poll failed: %v", r.cfg.RunnerID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var claimResp ClaimTaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&claimResp); err != nil || !claimResp.Claimed || claimResp.Attempt == nil {
		return
	}

	log.Printf("[Runner %s] Claimed task attempt %s (Task: '%s')", r.cfg.RunnerID, claimResp.Attempt.ID, claimResp.Task.Title)

	execCtx, cancelFunc := context.WithCancel(ctx)
	r.mu.Lock()
	r.currentAttempt = claimResp.Attempt
	r.cancelAttempt = cancelFunc
	r.mu.Unlock()

	// Execute attempt in background goroutine so heartbeat ticker continues running
	go func(att *task.TaskAttempt, t *task.Task) {
		defer func() {
			r.mu.Lock()
			r.currentAttempt = nil
			r.cancelAttempt = nil
			r.mu.Unlock()
		}()

		r.runAttempt(execCtx, att, t)
	}(claimResp.Attempt, claimResp.Task)
}

func (r *Runner) runAttempt(ctx context.Context, att *task.TaskAttempt, t *task.Task) {
	startTime := time.Now()

	// Stream initial start log
	r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Runner '%s' memulai eksekusi attempt %s dengan agent '%s'.", r.cfg.RunnerID, att.ID, r.adapter.Name()))

	// Prepare execution request
	prompt := t.Instructions
	if prompt == "" {
		prompt = t.Title
	}

	req := agent.ExecutionRequest{
		TaskID:       t.ID,
		Prompt:       prompt,
		Timeout:      10 * time.Minute,
		OutputFormat: "json",
	}

	result, err := r.adapter.Execute(ctx, req)
	duration := time.Since(startTime)

	var compReq CompleteAttemptRequest
	compReq.RunnerID = r.cfg.RunnerID

	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			compReq.Status = task.AttemptStatusCanceled
			compReq.ExitCode = 130
			compReq.ErrorMessage = "Execution canceled by runner context"
		} else {
			compReq.Status = task.AttemptStatusFailed
			compReq.ExitCode = 1
			compReq.ErrorMessage = err.Error()
		}
	} else if result != nil {
		compReq.PeakRSSBytes = result.PeakRSSBytes

		// Stream stdout & stderr logs (bounded & masked)
		r.streamOutputLogs(ctx, att.ID, result.Stdout, result.Stderr)

		if result.Status == "CANCELED" {
			compReq.Status = task.AttemptStatusCanceled
			compReq.ExitCode = 130
			compReq.ErrorMessage = "Execution canceled"
		} else if result.Status == "TIMEOUT" {
			compReq.Status = task.AttemptStatusFailed
			compReq.ExitCode = 124
			compReq.ErrorMessage = "Execution timed out"
		} else if result.ExitCode != 0 || result.Status == "FAILED" {
			compReq.Status = task.AttemptStatusFailed
			compReq.ExitCode = result.ExitCode
			if compReq.ExitCode == 0 {
				compReq.ExitCode = 1
			}
			compReq.ErrorMessage = result.Error
			if compReq.ErrorMessage == "" {
				compReq.ErrorMessage = "Agent reported execution failure"
			}
		} else {
			compReq.Status = task.AttemptStatusSucceeded
			compReq.ExitCode = 0
		}
	} else {
		compReq.Status = task.AttemptStatusFailed
		compReq.ExitCode = 1
		compReq.ErrorMessage = "No execution result returned from agent adapter"
	}

	r.streamLogLine(ctx, att.ID, "system", fmt.Sprintf("Attempt %s selesai: %s (exit: %d, durasi: %v, peak RSS: %d bytes).",
		att.ID, compReq.Status, compReq.ExitCode, duration.Round(time.Millisecond), compReq.PeakRSSBytes))

	// Send complete notification to server
	urlPath := fmt.Sprintf("/api/v1/runner/attempts/%s/complete", att.ID)
	_, _ = r.doRequest(context.Background(), http.MethodPost, urlPath, compReq)
	log.Printf("[Runner %s] Attempt %s finalized with status '%s' (exit %d)", r.cfg.RunnerID, att.ID, compReq.Status, compReq.ExitCode)
}

func (r *Runner) streamOutputLogs(ctx context.Context, attemptID, stdout, stderr string) {
	var records []task.LogRecord
	now := time.Now().UTC()

	if stdout != "" {
		lines := strings.Split(stdout, "\n")
		for _, l := range lines {
			if strings.TrimSpace(l) != "" {
				records = append(records, task.LogRecord{
					Stream:   "stdout",
					Line:     l,
					LoggedAt: now,
				})
			}
		}
	}

	if stderr != "" {
		lines := strings.Split(stderr, "\n")
		for _, l := range lines {
			if strings.TrimSpace(l) != "" {
				records = append(records, task.LogRecord{
					Stream:   "stderr",
					Line:     l,
					LoggedAt: now,
				})
			}
		}
	}

	// Bound logs to RunnerMaxLogLines
	maxLines := r.cfg.RunnerMaxLogLines
	if maxLines <= 0 {
		maxLines = 5000
	}
	if len(records) > maxLines {
		truncatedMsg := fmt.Sprintf("[LOG TRUNCATED: log output exceeded %d lines]", maxLines)
		records = append(records[:maxLines], task.LogRecord{
			Stream:   "system",
			Line:     truncatedMsg,
			LoggedAt: now,
		})
	}

	if len(records) == 0 {
		return
	}

	// Stream to server
	payload := StreamLogsRequest{
		RunnerID:  r.cfg.RunnerID,
		AttemptID: attemptID,
		Logs:      records,
	}

	urlPath := fmt.Sprintf("/api/v1/runner/attempts/%s/logs", attemptID)
	_, _ = r.doRequest(ctx, http.MethodPost, urlPath, payload)
}

func (r *Runner) streamLogLine(ctx context.Context, attemptID, stream, line string) {
	payload := StreamLogsRequest{
		RunnerID:  r.cfg.RunnerID,
		AttemptID: attemptID,
		Logs: []task.LogRecord{
			{
				Stream:   stream,
				Line:     line,
				LoggedAt: time.Now().UTC(),
			},
		},
	}
	urlPath := fmt.Sprintf("/api/v1/runner/attempts/%s/logs", attemptID)
	_, _ = r.doRequest(ctx, http.MethodPost, urlPath, payload)
}

func (r *Runner) cancelCurrentExecution() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancelAttempt != nil {
		r.cancelAttempt()
	}
}

func (r *Runner) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	reqURL := r.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "CuanRunner/"+r.cfg.RunnerID)
	// Runner authentication via token
	if r.cfg.RunnerToken != "" {
		req.Header.Set("X-Runner-Token", r.cfg.RunnerToken)
		req.Header.Set("Authorization", "Bearer "+r.cfg.RunnerToken)
	}

	return r.httpClient.Do(req)
}
