package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeCustomConfig(t *testing.T) {
	tempDir := t.TempDir()
	apiKey := "sk-ant-api03-abcdef1234567890abcdef1234567890"

	err := EnsureClaudeCustomConfig(tempDir, apiKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify .claude.json has approved key mask
	claudeJSONPath := filepath.Join(tempDir, ".claude.json")
	data, err := os.ReadFile(claudeJSONPath)
	if err != nil {
		t.Fatalf("could not read .claude.json: %v", err)
	}
	expectedMask := ClaudeCustomAPIKeyMask(apiKey)
	if !strings.Contains(string(data), expectedMask) {
		t.Errorf("expected %s in .claude.json, got %s", expectedMask, string(data))
	}

	// Verify settings.json has disableClaudeAiConnectors
	settingsPath := filepath.Join(tempDir, ".claude", "settings.json")
	sData, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("could not read settings.json: %v", err)
	}
	if !strings.Contains(string(sData), "disableClaudeAiConnectors") {
		t.Errorf("expected disableClaudeAiConnectors in settings.json, got %s", string(sData))
	}

	// Verify env vars
	env := HarnessCustomEnv(HarnessClaude, apiKey, "https://api.anthropic.com", "claude-3-7-sonnet", 200000)
	foundKey := false
	foundModel := false
	for _, e := range env {
		if strings.HasPrefix(e, "ANTHROPIC_API_KEY="+apiKey) {
			foundKey = true
		}
		if strings.HasPrefix(e, "ANTHROPIC_MODEL=claude-3-7-sonnet") {
			foundModel = true
		}
	}
	if !foundKey || !foundModel {
		t.Errorf("missing expected env in %+v", env)
	}
}

func TestCodexCustomConfig(t *testing.T) {
	tempDir := t.TempDir()
	apiKey := "sk-test-codex-key-12345"

	err := EnsureCodexCustomConfig(tempDir, apiKey, "https://my-proxy.com/v1", "o3-mini", 200000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	configPath := filepath.Join(tempDir, ".codex", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("could not read config.toml: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `model_provider = "custom_api"`) || !strings.Contains(content, `base_url = "https://my-proxy.com/v1"`) {
		t.Errorf("unexpected config.toml content: %s", content)
	}

	env := HarnessCustomEnv(HarnessCodex, apiKey, "https://my-proxy.com/v1", "o3-mini", 200000)
	foundKey := false
	for _, e := range env {
		if e == "CUSTOM_API_KEY="+apiKey {
			foundKey = true
		}
	}
	if !foundKey {
		t.Errorf("expected CUSTOM_API_KEY in env: %+v", env)
	}
}

func TestGrokCustomConfig(t *testing.T) {
	tempDir := t.TempDir()
	apiKey := "xai-test-grok-key-12345"

	err := EnsureGrokCustomConfig(tempDir, apiKey, "https://api.x.ai/v1", "grok-2-latest", 131072)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	configPath := filepath.Join(tempDir, ".grok", "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("could not read .grok/config.toml: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `default = "custom"`) || !strings.Contains(content, `model = "grok-2-latest"`) {
		t.Errorf("unexpected grok config.toml: %s", content)
	}

	env := HarnessCustomEnv(HarnessGrok, apiKey, "https://api.x.ai/v1", "grok-2-latest", 131072)
	foundKey := false
	for _, e := range env {
		if e == "CUSTOM_API_KEY="+apiKey {
			foundKey = true
		}
	}
	if !foundKey {
		t.Errorf("expected CUSTOM_API_KEY in grok env: %+v", env)
	}
}
