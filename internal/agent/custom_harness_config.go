package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	codexCatalogFileName = "codex-model-catalog.json"
)

// ClaudeCustomAPIKeyMask returns the last 20 characters of the key used by Claude Code
// to recognize pre-approved custom API keys in .claude.json.
func ClaudeCustomAPIKeyMask(apiKey string) string {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return ""
	}
	runes := []rune(key)
	if len(runes) <= 20 {
		return key
	}
	return string(runes[len(runes)-20:])
}

// EnsureClaudeCustomConfig pre-approves the custom API key fingerprint in ~/.claude.json
// and ensures connectors are disabled so Claude Code doesn't stall on first run.
func EnsureClaudeCustomConfig(homeDir, apiKey string) error {
	homeDir = strings.TrimSpace(homeDir)
	if homeDir == "" {
		return nil
	}
	mask := ClaudeCustomAPIKeyMask(apiKey)
	if mask != "" {
		globalConfig := filepath.Join(homeDir, ".claude.json")
		if err := approveClaudeCustomKey(globalConfig, mask); err != nil {
			return fmt.Errorf("approve claude custom key: %w", err)
		}
	}

	claudeDir := filepath.Join(homeDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0700); err != nil {
		return err
	}
	settingsPath := filepath.Join(claudeDir, "settings.json")
	return mergeJSONFile(settingsPath, map[string]any{
		"theme":                     "dark",
		"disableClaudeAiConnectors": true,
	})
}

func approveClaudeCustomKey(path, mask string) error {
	_ = os.MkdirAll(filepath.Dir(path), 0700)
	obj := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(raw))) > 0 {
		_ = json.Unmarshal(raw, &obj)
	}

	responses, _ := obj["customApiKeyResponses"].(map[string]any)
	if responses == nil {
		responses = map[string]any{}
	}
	approved, _ := responses["approved"].([]any)
	found := false
	for _, v := range approved {
		if s, ok := v.(string); ok && s == mask {
			found = true
			break
		}
	}
	if !found {
		approved = append(approved, mask)
	}
	responses["approved"] = approved
	obj["customApiKeyResponses"] = responses

	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func mergeJSONFile(path string, fields map[string]any) error {
	_ = os.MkdirAll(filepath.Dir(path), 0700)
	obj := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(raw))) > 0 {
		_ = json.Unmarshal(raw, &obj)
	}
	for k, v := range fields {
		obj[k] = v
	}
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// EnsureCodexCustomConfig materializes ~/.codex/config.toml and codex-model-catalog.json
// pointing Codex CLI to a custom API provider endpoint.
func EnsureCodexCustomConfig(homeDir, apiKey, baseURL, model string, contextWindow int) error {
	homeDir = strings.TrimSpace(homeDir)
	if homeDir == "" {
		return nil
	}
	codexDir := filepath.Join(homeDir, ".codex")
	if err := os.MkdirAll(codexDir, 0700); err != nil {
		return err
	}

	if model == "" {
		model = "gpt-4o"
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if contextWindow <= 0 {
		contextWindow = 128000
	}

	// Write model catalog JSON
	catalogPath := filepath.Join(codexDir, codexCatalogFileName)
	catalogEntry := map[string]any{
		"models": []map[string]any{
			{
				"id":                     model,
				"name":                   model,
				"context_window":         contextWindow,
				"max_output_tokens":      4096,
				"supports_vision":        true,
				"supports_function_call": true,
			},
		},
	}
	if data, err := json.MarshalIndent(catalogEntry, "", "  "); err == nil {
		_ = os.WriteFile(catalogPath, data, 0644)
	}

	configPath := filepath.Join(codexDir, "config.toml")
	tomlContent := fmt.Sprintf(`model = %s
model_provider = "custom_api"
model_context_window = %d
model_catalog_json = "./%s"

[model_providers.custom_api]
name = "Custom API"
base_url = %s
env_key = "CUSTOM_API_KEY"
wire_api = "responses"
`, strconv.Quote(model), contextWindow, codexCatalogFileName, strconv.Quote(baseURL))

	return os.WriteFile(configPath, []byte(tomlContent), 0644)
}

// EnsureGrokCustomConfig materializes ~/.grok/config.toml pointing Grok CLI
// to a custom API provider endpoint.
func EnsureGrokCustomConfig(homeDir, apiKey, baseURL, model string, contextWindow int) error {
	homeDir = strings.TrimSpace(homeDir)
	if homeDir == "" {
		return nil
	}
	if model == "" {
		model = "grok-4.7"
	}
	if baseURL == "" {
		baseURL = "https://api.x.ai/v1"
	}
	if contextWindow <= 0 {
		contextWindow = 131072
	}

	apiBackend := "responses"
	if strings.Contains(strings.ToLower(baseURL), "openai") || strings.Contains(strings.ToLower(baseURL), "chat") {
		apiBackend = "chat_completions"
	}

	tomlContent := fmt.Sprintf(`[models]
default = "custom"

[model."custom"]
model = %s
base_url = %s
name = "Custom API"
env_key = "CUSTOM_API_KEY"
api_backend = %s
context_window = %d
`, strconv.Quote(model), strconv.Quote(baseURL), strconv.Quote(apiBackend), contextWindow)

	// Write to both ~/.grok and <home>/grok for directory compatibility
	dirs := []string{filepath.Join(homeDir, ".grok"), filepath.Join(homeDir, "grok")}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(tomlContent), 0644); err != nil {
			return err
		}
	}
	return nil
}

// PrepareHarnessCustomConfig prepares filesystem files needed by a harness when running in custom API mode.
func PrepareHarnessCustomConfig(harness, homeDir, apiKey, baseURL, modelID string, contextWindow int) error {
	norm := strings.ToLower(strings.TrimSpace(harness))
	switch norm {
	case HarnessClaude:
		return EnsureClaudeCustomConfig(homeDir, apiKey)
	case HarnessCodex:
		return EnsureCodexCustomConfig(homeDir, apiKey, baseURL, modelID, contextWindow)
	case HarnessGrok:
		return EnsureGrokCustomConfig(homeDir, apiKey, baseURL, modelID, contextWindow)
	default:
		return nil
	}
}

// HarnessCustomEnv returns environment variables for running a harness in custom API mode.
func HarnessCustomEnv(harness, apiKey, baseURL, modelID string, contextWindow int) []string {
	norm := strings.ToLower(strings.TrimSpace(harness))
	apiKey = strings.TrimSpace(apiKey)
	baseURL = strings.TrimSpace(baseURL)
	modelID = strings.TrimSpace(modelID)

	switch norm {
	case HarnessClaude:
		base := baseURL
		if base == "" {
			base = "https://api.anthropic.com"
		}
		model := modelID
		if model == "" {
			model = "claude-3-7-sonnet-20250219"
		}
		env := []string{
			"ANTHROPIC_BASE_URL=" + base,
			"ANTHROPIC_API_KEY=" + apiKey,
			"ANTHROPIC_AUTH_TOKEN=" + apiKey,
			"ANTHROPIC_MODEL=" + model,
			"ANTHROPIC_DEFAULT_SONNET_MODEL=" + model,
			"ANTHROPIC_DEFAULT_OPUS_MODEL=" + model,
			"ANTHROPIC_DEFAULT_HAIKU_MODEL=" + model,
			"ENABLE_CLAUDEAI_MCP_SERVERS=false",
		}
		if contextWindow > 0 {
			env = append(env, "CLAUDE_CODE_MAX_CONTEXT_TOKENS="+strconv.Itoa(contextWindow))
		}
		return env

	case HarnessCodex:
		return []string{
			"CUSTOM_API_KEY=" + apiKey,
			"OPENAI_API_KEY=" + apiKey,
		}

	case HarnessGrok:
		return []string{
			"CUSTOM_API_KEY=" + apiKey,
			"XAI_API_KEY=" + apiKey,
		}

	case HarnessAGY, "google", "antigravity":
		env := []string{"GEMINI_API_KEY=" + apiKey}
		if baseURL != "" && baseURL != "https://generativelanguage.googleapis.com" {
			env = append(env, "GOOGLE_GEMINI_BASE_URL="+baseURL)
		}
		return env

	case "commandcode":
		return []string{"COMMAND_CODE_API_KEY=" + apiKey}

	default:
		return nil
	}
}
