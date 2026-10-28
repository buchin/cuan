package agent

import (
	"testing"
	"time"
)

func TestParseProviderQuotaResetDuration(t *testing.T) {
	tests := []struct {
		input       string
		wantDur     time.Duration
		wantStr     string
		wantSuccess bool
	}{
		{
			input:       "RESOURCE_EXHAUSTED (code 429): Individual quota reached. Please upgrade your subscription to increase your limits. Resets in 40h56m3s.",
			wantDur:     40*time.Hour + 56*time.Minute + 3*time.Second,
			wantStr:     "40h56m3s",
			wantSuccess: true,
		},
		{
			input:       "Resets in 63h.",
			wantDur:     63 * time.Hour,
			wantStr:     "63h",
			wantSuccess: true,
		},
		{
			input:       "Resets in 2d5h.",
			wantDur:     2*24*time.Hour + 5*time.Hour,
			wantStr:     "2d5h",
			wantSuccess: true,
		},
		{
			input:       "Resets in 45m",
			wantDur:     45 * time.Minute,
			wantStr:     "45m",
			wantSuccess: true,
		},
		{
			input:       "Resets in 30s",
			wantDur:     30 * time.Second,
			wantStr:     "30s",
			wantSuccess: true,
		},
		{
			input:       "Provider quota is limited (Resets in 68h12m38s). Your message is saved. Wait for the quota to reset; accounts will not be switched automatically.",
			wantDur:     68*time.Hour + 12*time.Minute + 38*time.Second,
			wantStr:     "68h12m38s",
			wantSuccess: true,
		},
		{
			input:       "Generic error without reset info",
			wantDur:     0,
			wantStr:     "",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			dur, str, ok := ParseProviderQuotaResetDuration(tt.input)
			if ok != tt.wantSuccess {
				t.Fatalf("expected success=%v, got %v", tt.wantSuccess, ok)
			}
			if dur != tt.wantDur {
				t.Fatalf("expected duration=%v, got %v", tt.wantDur, dur)
			}
			if str != tt.wantStr {
				t.Fatalf("expected string=%q, got %q", tt.wantStr, str)
			}
		})
	}
}
