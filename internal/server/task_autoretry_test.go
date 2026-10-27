package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
)

func TestTaskAutoRetryQuotaExhausted(t *testing.T) {
	s, token, _, _ := inboxFixture(t)
	owner, _ := s.authSvc.ValidateToken(context.Background(), token)

	// Primary account marked quota limited
	primary, err := agent.CreateProviderAccount(owner.ID, "quota-primary")
	if err != nil {
		t.Fatal(err)
	}
	primaryHome, _ := agent.ProviderAccountHome(owner.ID, primary.ID)
	os.MkdirAll(agent.AntigravityConfigDirPath(primaryHome), 0700)
	os.WriteFile(agent.AntigravityCredentialsFilePath(primaryHome), []byte("fixture-credential"), 0600)
	agent.SetProviderAccountFailover(owner.ID, primary.ID, true)

	// Register agent via connect-cli
	body := `{"name":"@quota-agent","handle":"@quota-agent","provider":"agy","account_id":"` + primary.ID + `"}`
	r := httptest.NewRequest("POST", "/api/v1/agents/connect-cli", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	agentID := "agent_quota_agent"

	// Mark primary as quota limited
	os.WriteFile(filepath.Join(primaryHome, "limited"), nil, 0600)
	agent.RecordProviderAccountQuotaWithDuration(owner.ID, primary.ID, 5*time.Minute, "")

	// Attempt task execution - should return 409 with provider_unavailable (no routes available)
	r = httptest.NewRequest("POST", "/api/v1/tasks/quota_task/run-agent", strings.NewReader(`{"request_id":"`+db.GenerateUUID()+`","agent_id":"`+agentID+`","prompt":"Test task"}`))
	r.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	s.Router().ServeHTTP(w, r)
	if w.Code != 409 {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
	var result struct {
		Code string `json:"code"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.Code != "provider_unavailable" {
		t.Fatalf("expected provider_unavailable code, got %s", result.Code)
	}

	// Verify no CLI was invoked for primary (no backup to fall back to)
	calls, _ := os.ReadFile(filepath.Join(primaryHome, "calls"))
	if len(calls) > 0 {
		t.Fatalf("unexpected CLI invocations: %s", string(calls))
	}
}

func TestTaskAutoRetryIdempotentNoDuplicate(t *testing.T) {
	s, token, _, _ := inboxFixture(t)
	owner, _ := s.authSvc.ValidateToken(context.Background(), token)

	// Primary account marked quota limited
	primary, err := agent.CreateProviderAccount(owner.ID, "idem-primary")
	if err != nil {
		t.Fatal(err)
	}
	primaryHome, _ := agent.ProviderAccountHome(owner.ID, primary.ID)
	os.MkdirAll(agent.AntigravityConfigDirPath(primaryHome), 0700)
	os.WriteFile(agent.AntigravityCredentialsFilePath(primaryHome), []byte("fixture-credential"), 0600)
	agent.SetProviderAccountFailover(owner.ID, primary.ID, true)

	// Register agent via connect-cli
	body := `{"name":"@idem-agent","handle":"@idem-agent","provider":"agy","account_id":"` + primary.ID + `"}`
	r := httptest.NewRequest("POST", "/api/v1/agents/connect-cli", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	agentID := "agent_idem_agent"

	// Mark primary as quota limited
	os.WriteFile(filepath.Join(primaryHome, "limited"), nil, 0600)
	agent.RecordProviderAccountQuotaWithDuration(owner.ID, primary.ID, 5*time.Minute, "")

	// Same request ID sent twice - should not duplicate
	requestID := db.GenerateUUID()
	runBody := `{"request_id":"` + requestID + `","agent_id":"` + agentID + `","prompt":"Idempotent test"}`
	r = httptest.NewRequest("POST", "/api/v1/tasks/idem_task/run-agent", strings.NewReader(runBody))
	r.Header.Set("Authorization", "Bearer "+token)
	w1 := httptest.NewRecorder()
	s.Router().ServeHTTP(w1, r)

	r = httptest.NewRequest("POST", "/api/v1/tasks/idem_task/run-agent", strings.NewReader(runBody))
	r.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	s.Router().ServeHTTP(w2, r)

	codes := map[int]bool{w1.Code: true, w2.Code: true}
	if !codes[409] {
		t.Fatalf("expected 409 for quota exhausted, got %d and %d", w1.Code, w2.Code)
	}

	// Verify no CLI invocations (no backup available)
	calls, _ := os.ReadFile(filepath.Join(primaryHome, "calls"))
	if len(calls) > 0 {
		t.Fatalf("unexpected CLI invocations: %s", string(calls))
	}

	// Verify error contains informative message
	var result struct {
		Error string `json:"error"`
	}
	json.Unmarshal(w1.Body.Bytes(), &result)
	if !strings.Contains(result.Error, "quota") && !strings.Contains(result.Error, "route") {
		t.Fatalf("expected quota/route message, got: %s", result.Error)
	}
}
