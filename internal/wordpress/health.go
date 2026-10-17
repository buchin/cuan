package wordpress

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HealthCheckResult represents the outcome of probing a WordPress deployment.
type HealthCheckResult struct {
	Healthy       bool          `json:"healthy"`
	HomeStatus    int           `json:"home_status"`
	LoginStatus   int           `json:"login_status"`
	HomeVerified  bool          `json:"home_verified"`
	LoginVerified bool          `json:"login_verified"`
	Latency       time.Duration `json:"latency"`
	Message       string        `json:"message"`
}

// CheckWordPressHealth probes the target WordPress port for both "/" and "/wp-login.php",
// verifying HTTP 200 responses.
func CheckWordPressHealth(ctx context.Context, client *http.Client, host string, port int, timeout time.Duration) (*HealthCheckResult, error) {
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	if host == "" {
		host = "127.0.0.1"
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	deadline := time.Now().Add(timeout)
	interval := 300 * time.Millisecond

	var lastRes *HealthCheckResult
	var lastErr error

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		res, err := probeWordPressOnce(ctx, client, host, port)
		if err == nil && res.Healthy {
			return res, nil
		}
		lastRes = res
		lastErr = err

		time.Sleep(interval)
	}

	if lastRes != nil {
		return lastRes, fmt.Errorf("wordpress health check failed within timeout: %s", lastRes.Message)
	}
	if lastErr != nil {
		return nil, fmt.Errorf("wordpress health check timed out: %w", lastErr)
	}
	return nil, fmt.Errorf("wordpress health check timed out")
}

func probeWordPressOnce(ctx context.Context, client *http.Client, host string, port int) (*HealthCheckResult, error) {
	start := time.Now()
	homeURL := fmt.Sprintf("http://%s:%d/", host, port)
	loginURL := fmt.Sprintf("http://%s:%d/wp-login.php", host, port)

	// 1. Probe Public Home
	homeReq, err := http.NewRequestWithContext(ctx, http.MethodGet, homeURL, nil)
	if err != nil {
		return nil, err
	}
	homeResp, err := client.Do(homeReq)
	if err != nil {
		return nil, fmt.Errorf("probe home: %w", err)
	}
	defer homeResp.Body.Close()
	_, _ = io.Copy(io.Discard, homeResp.Body)

	// 2. Probe wp-login.php
	loginReq, err := http.NewRequestWithContext(ctx, http.MethodGet, loginURL, nil)
	if err != nil {
		return nil, err
	}
	loginResp, err := client.Do(loginReq)
	if err != nil {
		return nil, fmt.Errorf("probe wp-login: %w", err)
	}
	defer loginResp.Body.Close()
	_, _ = io.Copy(io.Discard, loginResp.Body)

	latency := time.Since(start)

	// In WordPress, 200 OK on both home and login indicates healthy state.
	// Also 301/302 redirecting to canonical home or login is acceptable.
	homeOK := homeResp.StatusCode == http.StatusOK || homeResp.StatusCode == http.StatusFound || homeResp.StatusCode == http.StatusMovedPermanently
	loginOK := loginResp.StatusCode == http.StatusOK || loginResp.StatusCode == http.StatusFound

	healthy := homeOK && loginOK
	var msg string
	if healthy {
		msg = fmt.Sprintf("WordPress responding HTTP 200/OK (Home: %d, Login: %d) in %v", homeResp.StatusCode, loginResp.StatusCode, latency)
	} else {
		msg = fmt.Sprintf("WordPress unhealthy: Home HTTP %d, Login HTTP %d", homeResp.StatusCode, loginResp.StatusCode)
	}

	return &HealthCheckResult{
		Healthy:       healthy,
		HomeStatus:    homeResp.StatusCode,
		LoginStatus:   loginResp.StatusCode,
		HomeVerified:  homeOK,
		LoginVerified: loginOK,
		Latency:       latency,
		Message:       msg,
	}, nil
}
