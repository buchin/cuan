package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
)

func TestEvaluateStatus(t *testing.T) {
	tests := []struct {
		name                 string
		prevFailures         int
		result               *ProbeResult
		expectedStatus       string
		expectedFailures     int
	}{
		{
			name:         "Fast 200 OK is Healthy",
			prevFailures: 0,
			result: &ProbeResult{
				StatusCode:     200,
				ResponseTimeMs: 150,
				MatchedKeyword: true,
			},
			expectedStatus:   StatusHealthy,
			expectedFailures: 0,
		},
		{
			name:         "Slow 200 OK (>2s) is Degraded",
			prevFailures: 0,
			result: &ProbeResult{
				StatusCode:     200,
				ResponseTimeMs: 2500,
				MatchedKeyword: true,
			},
			expectedStatus:   StatusDegraded,
			expectedFailures: 0,
		},
		{
			name:         "First 500 error is Degraded (1 failure)",
			prevFailures: 0,
			result: &ProbeResult{
				StatusCode:     500,
				ResponseTimeMs: 50,
				MatchedKeyword: true,
			},
			expectedStatus:   StatusDegraded,
			expectedFailures: 1,
		},
		{
			name:         "Second 500 error is Degraded (2 failures)",
			prevFailures: 1,
			result: &ProbeResult{
				StatusCode:     500,
				ResponseTimeMs: 50,
				MatchedKeyword: true,
			},
			expectedStatus:   StatusDegraded,
			expectedFailures: 2,
		},
		{
			name:         "Third 500 error is Unhealthy (3 failures)",
			prevFailures: 2,
			result: &ProbeResult{
				StatusCode:     500,
				ResponseTimeMs: 50,
				MatchedKeyword: true,
			},
			expectedStatus:   StatusUnhealthy,
			expectedFailures: 3,
		},
		{
			name:         "Timeout / Network Error increases failure count",
			prevFailures: 2,
			result: &ProbeResult{
				StatusCode:     0,
				ResponseTimeMs: 5000,
				Error:          "connection timed out",
				MatchedKeyword: false,
			},
			expectedStatus:   StatusUnhealthy,
			expectedFailures: 3,
		},
		{
			name:         "Keyword mismatch is treated as failure",
			prevFailures: 0,
			result: &ProbeResult{
				StatusCode:     200,
				ResponseTimeMs: 100,
				MatchedKeyword: false,
			},
			expectedStatus:   StatusDegraded,
			expectedFailures: 1,
		},
		{
			name:         "Recovery from Unhealthy to Healthy resets failures",
			prevFailures: 5,
			result: &ProbeResult{
				StatusCode:     200,
				ResponseTimeMs: 120,
				MatchedKeyword: true,
			},
			expectedStatus:   StatusHealthy,
			expectedFailures: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, failures := EvaluateStatus(tt.prevFailures, tt.result)
			if status != tt.expectedStatus {
				t.Errorf("expected status %s, got %s", tt.expectedStatus, status)
			}
			if failures != tt.expectedFailures {
				t.Errorf("expected failures %d, got %d", tt.expectedFailures, failures)
			}
		})
	}
}

func TestRollingHistory_Report(t *testing.T) {
	h := NewRollingHistory(5)

	// Initially empty
	emptyRep := h.Report()
	if emptyRep.UptimePercent != 100.0 {
		t.Errorf("expected 100%% uptime for empty history, got %f", emptyRep.UptimePercent)
	}
	if emptyRep.TotalProbes != 0 {
		t.Errorf("expected 0 total probes, got %d", emptyRep.TotalProbes)
	}

	now := time.Now()
	// Add 3 healthy, 1 degraded, 1 unhealthy
	h.Add(ProbeRecord{Timestamp: now, StatusCode: 200, ResponseTimeMs: 100, Status: StatusHealthy})
	h.Add(ProbeRecord{Timestamp: now, StatusCode: 200, ResponseTimeMs: 200, Status: StatusHealthy})
	h.Add(ProbeRecord{Timestamp: now, StatusCode: 200, ResponseTimeMs: 150, Status: StatusHealthy})
	h.Add(ProbeRecord{Timestamp: now, StatusCode: 200, ResponseTimeMs: 2500, Status: StatusDegraded})
	h.Add(ProbeRecord{Timestamp: now, StatusCode: 500, ResponseTimeMs: 50, Status: StatusUnhealthy})

	rep := h.Report()
	if rep.TotalProbes != 5 {
		t.Errorf("expected 5 total probes, got %d", rep.TotalProbes)
	}
	if rep.SuccessfulProbes != 4 {
		t.Errorf("expected 4 successful probes, got %d", rep.SuccessfulProbes)
	}
	if rep.FailedProbes != 1 {
		t.Errorf("expected 1 failed probe, got %d", rep.FailedProbes)
	}
	// 4/5 * 100 = 80.0%
	if rep.UptimePercent != 80.0 {
		t.Errorf("expected 80.0%% uptime, got %f", rep.UptimePercent)
	}
	// avg latency: (100+200+150+2500+50) / 5 = 600
	if rep.AverageLatencyMs != 600.0 {
		t.Errorf("expected 600ms avg latency, got %f", rep.AverageLatencyMs)
	}

	// Adding one more record should evict the first record
	h.Add(ProbeRecord{Timestamp: now, StatusCode: 200, ResponseTimeMs: 100, Status: StatusHealthy})
	rep2 := h.Report()
	if rep2.TotalProbes != 5 {
		t.Errorf("expected capacity 5, got %d", rep2.TotalProbes)
	}
}

func TestHTTPProber_Probe(t *testing.T) {
	// Mock server that returns 200 with keyword
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","message":"pong"}`))
	}))
	defer ts.Close()

	prober := NewHTTPProber(2 * time.Second)

	// Test successful probe with matching keyword
	res, err := prober.Probe(context.Background(), ts.URL, "pong")
	if err != nil {
		t.Fatalf("probe failed: %v", err)
	}
	if res.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", res.StatusCode)
	}
	if !res.MatchedKeyword {
		t.Errorf("expected keyword 'pong' to match")
	}

	// Test probe with missing keyword
	resKeywordMiss, err := prober.Probe(context.Background(), ts.URL, "nonexistent-keyword")
	if err != nil {
		t.Fatalf("probe failed: %v", err)
	}
	if resKeywordMiss.MatchedKeyword {
		t.Errorf("expected keyword to not match")
	}

	// Test probe with invalid target URL
	resErr, _ := prober.Probe(context.Background(), "http://127.0.0.1:1", "")
	if resErr.Error == "" {
		t.Errorf("expected connection error for unreachable port")
	}
}

func TestHealthPoller_LifecycleAndProbeNow(t *testing.T) {
	repo := db.NewMemoryObservabilityRepository()
	ctx := context.Background()

	var probeCount int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&probeCount, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("cuan-alive"))
	}))
	defer ts.Close()

	poller := NewHealthPoller(repo,
		WithPollInterval(100*time.Millisecond),
		WithProbeTimeout(1*time.Second),
		WithHistoryLimit(10),
	)

	// 1. Ensure check
	hc, err := poller.EnsureHealthCheck(ctx, "dep-1", ts.URL)
	if err != nil {
		t.Fatalf("EnsureHealthCheck failed: %v", err)
	}
	if hc.DeploymentID != "dep-1" {
		t.Errorf("expected dep-1, got %s", hc.DeploymentID)
	}

	// 2. Start poller
	if err := poller.Start(ctx); err != nil {
		t.Fatalf("poller.Start failed: %v", err)
	}
	if !poller.IsRunning() {
		t.Errorf("expected poller to be running")
	}

	// Wait for background poll
	time.Sleep(250 * time.Millisecond)

	// 3. ProbeNow
	updated, rep, err := poller.ProbeNow(ctx, "dep-1")
	if err != nil {
		t.Fatalf("ProbeNow failed: %v", err)
	}
	if updated.Status != StatusHealthy {
		t.Errorf("expected healthy status, got %s", updated.Status)
	}
	if rep == nil || rep.TotalProbes == 0 {
		t.Fatalf("expected non-empty uptime report")
	}

	// 4. Stop poller
	poller.Stop()
	if poller.IsRunning() {
		t.Errorf("expected poller to be stopped")
	}
}
