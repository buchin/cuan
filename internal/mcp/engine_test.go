package mcp

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSafeExecuteTool_PanicRecovery(t *testing.T) {
	panicHandler := func(ctx context.Context, args map[string]any) (*ToolCallResponse, error) {
		panic("simulated fatal crash in tool handler")
	}

	ctx := context.Background()
	resp, err := SafeExecuteTool(ctx, panicHandler, map[string]any{"data": "test"}, 1*time.Second)

	if err == nil {
		t.Errorf("expected error from panic recovery, got nil")
	}
	if resp == nil {
		t.Fatalf("expected non-nil response")
	}
	if !resp.IsError {
		t.Errorf("expected IsError = true")
	}
	if len(resp.Content) == 0 || !strings.Contains(resp.Content[0].Text, "panic") {
		t.Errorf("expected panic mention in content: %+v", resp)
	}
}

func TestSafeExecuteTool_TimeoutSafety(t *testing.T) {
	slowHandler := func(ctx context.Context, args map[string]any) (*ToolCallResponse, error) {
		select {
		case <-time.After(200 * time.Millisecond):
			return &ToolCallResponse{
				Content: []ToolContent{{Type: "text", Text: "finished"}},
			}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	ctx := context.Background()
	// Bounded to 50ms timeout
	resp, err := SafeExecuteTool(ctx, slowHandler, map[string]any{}, 50*time.Millisecond)

	if err == nil {
		t.Errorf("expected timeout error, got nil")
	}
	if resp == nil || !resp.IsError {
		t.Errorf("expected timeout error response")
	}
	if !strings.Contains(resp.Content[0].Text, "timed out") {
		t.Errorf("expected 'timed out' in response, got: %s", resp.Content[0].Text)
	}
}

func TestSafeExecuteTool_ZeroAmbientCredentialLeak(t *testing.T) {
	// 1. Verify ambient environment variable filtering
	ambientEnv := []string{
		"PATH=/usr/bin:/bin",
		"HOME=/home/user",
		"USER=cuan",
		"AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE",
		"AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		"FURAL_AGENT_TOKEN=fural_sec_994191024",
		"DOJO_VPS_PASSWORD=secret-root-pass",
		"RESEND_KEY=re_123456789",
		"NERD_API_SECRET=nerd_super_secret",
		"TINYFISH_API_KEY=tf_abc123",
	}

	safeEnv := FilterSafeEnvironment(ambientEnv)
	for _, entry := range safeEnv {
		if strings.HasPrefix(entry, "AWS_") ||
			strings.HasPrefix(entry, "FURAL_") ||
			strings.HasPrefix(entry, "DOJO_") ||
			strings.HasPrefix(entry, "RESEND_") ||
			strings.HasPrefix(entry, "NERD_") ||
			strings.HasPrefix(entry, "TINYFISH_") {
			t.Errorf("ambient credential leaked in safe environment: %s", entry)
		}
	}

	if len(safeEnv) != 3 {
		t.Errorf("expected only 3 safe environment variables (PATH, HOME, USER), got %d: %v", len(safeEnv), safeEnv)
	}

	// 2. Verify argument sanitization (strips ambient secret keys)
	argsWithSecrets := map[string]any{
		"query":                 "SELECT * FROM sales",
		"AWS_SECRET_ACCESS_KEY": "leaked_secret_val",
		"FURAL_AGENT_TOKEN":     "leaked_token_val",
		"api_key":               "sk-1234567890abcdef123456",
		"safe_field":            "regular_data",
	}

	cleanArgs := SanitizeArgs(argsWithSecrets)
	if _, exists := cleanArgs["AWS_SECRET_ACCESS_KEY"]; exists {
		t.Errorf("ambient credential key AWS_SECRET_ACCESS_KEY was not stripped from args")
	}
	if _, exists := cleanArgs["FURAL_AGENT_TOKEN"]; exists {
		t.Errorf("ambient credential key FURAL_AGENT_TOKEN was not stripped from args")
	}
	if cleanArgs["safe_field"] != "regular_data" {
		t.Errorf("expected safe_field to be preserved, got %v", cleanArgs["safe_field"])
	}

	// 3. Verify output masking
	outputHandler := func(ctx context.Context, args map[string]any) (*ToolCallResponse, error) {
		return &ToolCallResponse{
			Content: []ToolContent{
				{
					Type: "text",
					Text: "Operation succeeded with Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9 and token=super_secret_token_12345",
				},
			},
		}, nil
	}

	resp, err := SafeExecuteTool(context.Background(), outputHandler, map[string]any{}, 1*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(resp.Content[0].Text, "super_secret_token_12345") {
		t.Errorf("secret token was not masked in output: %s", resp.Content[0].Text)
	}
}
