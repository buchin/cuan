package agent

import (
	"context"
	"testing"
	"time"
)

func TestProviderRegistry_RegistrationAndLookup(t *testing.T) {
	reg := NewProviderRegistry()

	mockP := NewMockProvider()
	reg.Register("mock", mockP)

	p, err := reg.Get("mock")
	if err != nil {
		t.Fatalf("unexpected error getting mock provider: %v", err)
	}

	if p.Type() != "mock" {
		t.Errorf("expected type mock, got %s", p.Type())
	}

	// Case-insensitive lookup
	pUpper, err := reg.Get("MOCK")
	if err != nil {
		t.Fatalf("unexpected error getting MOCK provider: %v", err)
	}
	if pUpper != p {
		t.Errorf("expected identical provider instance")
	}

	// Missing provider
	_, err = reg.Get("non-existent")
	if err == nil {
		t.Errorf("expected error for non-existent provider, got nil")
	}
}

func TestMockProvider_Verification(t *testing.T) {
	mockP := NewMockProvider()
	ctx := context.Background()

	// 1. Success case
	probe, err := mockP.VerifyConnection(ctx, "valid-mock-key")
	if err != nil {
		t.Fatalf("VerifyConnection failed: %v", err)
	}
	if !probe.Valid {
		t.Errorf("expected valid probe")
	}
	if len(probe.AvailableModels) == 0 {
		t.Errorf("expected available models")
	}

	// 2. SetFail case
	mockP.SetFail(true, "quota exceeded")
	probeFail, err := mockP.VerifyConnection(ctx, "valid-mock-key")
	if err == nil {
		t.Errorf("expected error when SetFail is true, got nil")
	}
	if probeFail.Valid {
		t.Errorf("expected invalid probe")
	}
	if probeFail.ErrorMessage != "quota exceeded" {
		t.Errorf("expected error message 'quota exceeded', got %s", probeFail.ErrorMessage)
	}

	// Reset fail
	mockP.SetFail(false, "")

	// 3. Simulated delay with context timeout
	mockP.SimulatedDelay = 100 * time.Millisecond
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()

	_, err = mockP.VerifyConnection(timeoutCtx, "valid-mock-key")
	if err == nil {
		t.Errorf("expected context deadline error, got nil")
	}
}

func TestDefaultRegistry(t *testing.T) {
	reg := DefaultRegistry()
	if reg == nil {
		t.Fatalf("DefaultRegistry returned nil")
	}

	googleP, err := reg.Get("google")
	if err != nil || googleP == nil {
		t.Fatalf("failed to retrieve google provider from DefaultRegistry: %v", err)
	}

	mockP, err := reg.Get("mock")
	if err != nil || mockP == nil {
		t.Fatalf("failed to retrieve mock provider from DefaultRegistry: %v", err)
	}
}
