package agent

import (
	"context"
	"testing"
)

func TestCommandCode_VerifyConnection(t *testing.T) {
	prov := NewCommandCodeProvider()
	ctx := context.Background()

	// 1. Subscription mode (empty or "subscription")
	res, err := prov.VerifyConnection(ctx, "")
	if err != nil {
		t.Fatalf("unexpected error for subscription mode: %v", err)
	}
	if !res.Valid || res.PlanHint != "CommandCode subscription" {
		t.Errorf("expected valid subscription probe, got: %+v", res)
	}

	res, err = prov.VerifyConnection(ctx, "subscription")
	if err != nil {
		t.Fatalf("unexpected error for subscription mode string: %v", err)
	}
	if !res.Valid || res.PlanHint != "CommandCode subscription" {
		t.Errorf("expected valid subscription probe, got: %+v", res)
	}

	// 2. BYOK short key
	res, err = prov.VerifyConnection(ctx, "short")
	if err == nil || (res != nil && res.Valid) {
		t.Errorf("expected error for short key, got res=%+v, err=%v", res, err)
	}

	// 3. BYOK valid key (>= 8 chars)
	res, err = prov.VerifyConnection(ctx, "cc_live_1234567890abcdef")
	if err != nil {
		t.Fatalf("unexpected error for valid BYOK key: %v", err)
	}
	if !res.Valid || res.PlanHint != "CommandCode custom provider (BYOK)" {
		t.Errorf("expected valid BYOK probe, got: %+v", res)
	}
}

func TestCommandCode_ListModels(t *testing.T) {
	prov := NewCommandCodeProvider()
	ctx := context.Background()

	models, err := prov.ListModels(ctx, "")
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}
	if len(models) == 0 {
		t.Fatal("expected at least 1 model in catalog")
	}

	foundDefault := false
	foundVision := false
	foundNonVision := false
	for _, m := range models {
		if m.IsDefault {
			foundDefault = true
		}
		if m.SupportsVision {
			foundVision = true
		} else {
			foundNonVision = true
		}
	}

	if !foundDefault {
		t.Error("expected default model to be marked in catalog")
	}
	if !foundVision {
		t.Error("expected at least one vision-capable model in catalog")
	}
	if !foundNonVision {
		t.Error("expected at least one text-only / non-vision model in catalog")
	}
}

func TestCommandCode_VisionDetection(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"commandcode-default", false},
		{"deepseek/deepseek-v4-pro", false},
		{"moonshotai/kimi-k3.1-pro", false},
		{"claude-fable-5-1", false},
		{"qwen/qwen3.8-max", false},
		{"xai/grok-4.5", false},
		{"reflection/beam:free", false},
		{"claude-sonnet-5-5", true},
		{"claude-opus-5-5", true},
		{"deepseek/deepseek-v4-flash-vision-exp", true},
		{"google/gemini-3.8-flash", true},
		{"moonshotai/kimi-k2.7-code", true},
		{"qwen/qwen3.8-omni-flash", true},
	}

	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			actual := IsCommandCodeModelVision(tc.model)
			if actual != tc.expected {
				t.Errorf("IsCommandCodeModelVision(%q) = %v; expected %v", tc.model, actual, tc.expected)
			}

			// Also verify through generic IsModelVisionCapable
			genericActual := IsModelVisionCapable("commandcode", tc.model)
			if genericActual != tc.expected {
				t.Errorf("IsModelVisionCapable('commandcode', %q) = %v; expected %v", tc.model, genericActual, tc.expected)
			}
		})
	}
}
