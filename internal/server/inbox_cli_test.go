package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
)

func TestDecodeInboxAgyResult(t *testing.T) {
	cases := []struct{ name, raw, stderr, want, reason, contains string }{
		{"success", `{"status":"SUCCESS","response":" **Useful answer** ","usage":{"private":"never expose"}}`, "", "**Useful answer**", "success", ""},
		{"empty stdout", "", "429 RESOURCE_EXHAUSTED private-token", "", "empty_output", "Kuota"},
		{"exit zero error", `{"status":"ERROR","response":"partial private text","error":{"message":"429 RESOURCE_EXHAUSTED private-token"}}`, "", "", "provider_failure", "Kuota"},
		{"auth", `{"status":"ERROR","message":"invalid_grant private-token"}`, "", "", "provider_failure", "Login"},
		{"timeout", `{"status":"TIMEOUT","response":"partial private-token"}`, "", "", "provider_timeout", "Batas waktu"},
		{"empty response", `{"status":"SUCCESS","response":" "}`, "", "", "empty_response", "belum menghasilkan"},
		{"missing status", `{"response":"private-token"}`, "", "", "provider_failure", "gagal"},
		{"malformed", "private-token", "", "", "invalid_result", "belum dapat dibaca"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, failure, reason := decodeInboxAgyResult(c.raw, c.stderr)
			if out != c.want || reason != c.reason || !strings.Contains(failure, c.contains) {
				t.Fatalf("got output=%q failure=%q reason=%q", out, failure, reason)
			}
			if strings.Contains(out+failure, "private") {
				t.Fatal("diagnostic leaked")
			}
		})
	}
}
func TestInboxCLIExitZeroFailureAndTrace(t *testing.T) {
	home, workspace, bin := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte("#!/bin/sh\nprintf '%s' '{\"event\":\"result\",\"result\":{\"status\":\"ERROR\",\"message\":\"RESOURCE_EXHAUSTED private-token\"}}'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	s := &Server{cfg: &config.Config{Environment: "staging"}}
	steps := map[string]string{}
	var mu sync.Mutex
	out, failure := s.executeInboxCLI(context.Background(), "fixture", "agy", "diagnostic", home, workspace, func(kind, text string) { mu.Lock(); steps[kind] = text; mu.Unlock() })
	if out != "" || !strings.Contains(failure, "Kuota") {
		t.Fatal("exit0 failure accepted", out, failure)
	}
	if !strings.Contains(steps["provider_result"], "exit 0") || steps["result_validation"] != "Provider melaporkan kegagalan." {
		t.Fatal("missing safe trace", steps)
	}
	for _, value := range steps {
		if strings.Contains(value, "private-token") {
			t.Fatal("trace leaked diagnostic")
		}
	}
}

// Explicit opt-in integration check; never part of an ordinary test run.
func TestLiveInboxAgyDiagnostic(t *testing.T) {
	home := os.Getenv("CUAN_DIAGNOSTIC_PROFILE")
	if home == "" {
		t.Skip("explicit isolated agent profile diagnostic required")
	}
	if !strings.Contains(home, "/.local/share/cuan/accounts/acct_") || !strings.HasSuffix(home, "/home") {
		t.Fatal("dedicated account profile required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := &Server{cfg: &config.Config{Environment: "staging"}}
	steps := map[string]string{}
	var mu sync.Mutex
	out, failure := s.executeInboxCLI(ctx, "pin2", "agy", "Gunakan tool baca README.md di workspace project ini, lalu balas tepat CUAN_DIAGNOSTIC_OK. Jangan mengubah file atau membaca file lain.", home, "/opt/cuan/data/projects/default", func(kind, text string) { mu.Lock(); steps[kind] = text; mu.Unlock() })
	if failure != "" || strings.TrimSpace(out) != "CUAN_DIAGNOSTIC_OK" {
		t.Fatalf("live diagnostic not verified; safe failure=%s", failure)
	}
	if !strings.Contains(steps["provider_result"], "exit 0") || steps["result_validation"] != "Balasan provider tervalidasi." {
		t.Fatal("live trace missing")
	}
	if steps["runtime_tool_started"] == "" || steps["runtime_tool_finished"] == "" {
		t.Fatal("actual tool trace missing")
	}
	t.Log("Actual Cuan executeInboxCLI + bound profile + project CWD: structured reply validated, actual read tool start/finish and exit0/duration trace; no user-message replay or database writes")
}
