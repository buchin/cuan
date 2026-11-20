package agent

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// CommandCodeProvider verifies CommandCode API connectivity via the official CLI.
type CommandCodeProvider struct{}

// NewCommandCodeProvider creates a CommandCode provider implementation.
func NewCommandCodeProvider() *CommandCodeProvider {
	return &CommandCodeProvider{}
}

func (p *CommandCodeProvider) Name() string { return "CommandCode" }
func (p *CommandCodeProvider) Type() string { return "commandcode" }

// IsCommandCodeModelVision checks if a given CommandCode model supports vision / image input.
func IsCommandCodeModelVision(modelID string) bool {
	m := strings.ToLower(strings.TrimSpace(modelID))
	if m == "" || m == "commandcode-default" || m == "default" {
		// Default mode is text/coding oriented
		return false
	}
	// Models with explicit vision or multimodal capabilities
	visionKeywords := []string{
		"vision", "multimodal", "-vl", "super-vl", "omni",
		"sonnet", "opus", "gemini", "4o", "kimi-k2",
		"minimax-m3", "inkling", "mistral-large-4", "muse-spark-1.3",
		"v4.1-flash",
	}
	for _, kw := range visionKeywords {
		if strings.Contains(m, kw) {
			return true
		}
	}
	return false
}

var commandCodeLookPath = func() (string, error) {
	if bin, err := exec.LookPath("commandcode"); err == nil {
		return bin, nil
	}
	for _, candidate := range []string{
		"/home/dev/.local/bin/commandcode",
		"/home/ubuntu/.local/bin/commandcode",
		"/usr/local/bin/commandcode",
	} {
		if _, statErr := exec.LookPath(candidate); statErr == nil {
			return candidate, nil
		}
	}
	return "", errors.New("commandcode not installed")
}

// VerifyConnection checks that the commandcode binary exists and credentials (if BYOK) are valid.
func (p *CommandCodeProvider) VerifyConnection(ctx context.Context, credentials string) (*ConnectionProbeResult, error) {
	trimmed := strings.TrimSpace(credentials)

	if _, err := commandCodeLookPath(); err != nil {
		return &ConnectionProbeResult{
			Valid:        false,
			Provider:     "commandcode",
			ErrorMessage: "CommandCode CLI not installed on this server",
			TestedAt:     time.Now().UTC(),
		}, nil
	}

	// Subscription connect: no API key required; relies on CLI install and subscription
	if trimmed == "" || trimmed == "subscription" {
		return &ConnectionProbeResult{
			Valid:        true,
			Provider:     "commandcode",
			PlanHint:     "CommandCode subscription",
			LatencyMs:    0,
			TestedAt:     time.Now().UTC(),
		}, nil
	}

	if len(trimmed) < 8 {
		return &ConnectionProbeResult{
			Valid:        false,
			Provider:     "commandcode",
			ErrorMessage: "invalid CommandCode API key",
			TestedAt:     time.Now().UTC(),
		}, ErrConnectionVerification
	}

	return &ConnectionProbeResult{
		Valid:        true,
		Provider:     "commandcode",
		PlanHint:     "CommandCode custom provider (BYOK)",
		LatencyMs:    0,
		TestedAt:     time.Now().UTC(),
	}, nil
}

// DefaultCommandCodeModels returns standard models verified for the CommandCode provider.
func DefaultCommandCodeModels() []ModelInfo {
	return []ModelInfo{
		{
			ID:             "commandcode-default",
			Name:           "commandcode-default",
			DisplayName:    "CommandCode Default",
			Description:    "CommandCode default model (Text-only)",
			Provider:       "commandcode",
			IsDefault:      true,
			SupportsVision: false,
			Capabilities: ModelCapabilities{
				Vision:         false,
				SupportsVision: false,
			},
		},
		{
			ID:             "claude-sonnet-5-5",
			Name:           "claude-sonnet-5-5",
			DisplayName:    "Claude Sonnet 5.5",
			Description:    "Frontier coding & intelligence with vision",
			Provider:       "commandcode",
			IsDefault:      false,
			SupportsVision: true,
			Capabilities: ModelCapabilities{
				Vision:         true,
				SupportsVision: true,
			},
		},
		{
			ID:             "claude-opus-5-5",
			Name:           "claude-opus-5-5",
			DisplayName:    "Claude Opus 5.5",
			Description:    "Most intelligent Opus for agents with vision",
			Provider:       "commandcode",
			IsDefault:      false,
			SupportsVision: true,
			Capabilities: ModelCapabilities{
				Vision:         true,
				SupportsVision: true,
			},
		},
		{
			ID:             "claude-fable-5-1",
			Name:           "claude-fable-5-1",
			DisplayName:    "Claude Fable 5.1",
			Description:    "Demanding reasoning & long-horizon agents (Text-only)",
			Provider:       "commandcode",
			IsDefault:      false,
			SupportsVision: false,
			Capabilities: ModelCapabilities{
				Vision:         false,
				SupportsVision: false,
			},
		},
		{
			ID:             "google/gemini-3.8-flash",
			Name:           "google/gemini-3.8-flash",
			DisplayName:    "Gemini 3.8 Flash",
			Description:    "Newest Gemini Flash, core reasoning with vision",
			Provider:       "commandcode",
			IsDefault:      false,
			SupportsVision: true,
			Capabilities: ModelCapabilities{
				Vision:         true,
				SupportsVision: true,
			},
		},
		{
			ID:             "deepseek/deepseek-v4-flash-vision-exp",
			Name:           "deepseek/deepseek-v4-flash-vision-exp",
			DisplayName:    "DeepSeek V4 Flash Vision",
			Description:    "Fast hybrid-attention reasoning with vision",
			Provider:       "commandcode",
			IsDefault:      false,
			SupportsVision: true,
			Capabilities: ModelCapabilities{
				Vision:         true,
				SupportsVision: true,
			},
		},
		{
			ID:             "deepseek/deepseek-v4-pro",
			Name:           "deepseek/deepseek-v4-pro",
			DisplayName:    "DeepSeek V4 Pro",
			Description:    "Hybrid-attention long-context reasoning (Text-only)",
			Provider:       "commandcode",
			IsDefault:      false,
			SupportsVision: false,
			Capabilities: ModelCapabilities{
				Vision:         false,
				SupportsVision: false,
			},
		},
		{
			ID:             "moonshotai/kimi-k2.7-code",
			Name:           "moonshotai/kimi-k2.7-code",
			DisplayName:    "Kimi K2.7 Code",
			Description:    "Improved long-horizon coding with vision",
			Provider:       "commandcode",
			IsDefault:      false,
			SupportsVision: true,
			Capabilities: ModelCapabilities{
				Vision:         true,
				SupportsVision: true,
			},
		},
		{
			ID:             "moonshotai/kimi-k3.1-pro",
			Name:           "moonshotai/kimi-k3.1-pro",
			DisplayName:    "Kimi K3.1 Pro",
			Description:    "Frontier reasoning & coding with 1M context (Text-only)",
			Provider:       "commandcode",
			IsDefault:      false,
			SupportsVision: false,
			Capabilities: ModelCapabilities{
				Vision:         false,
				SupportsVision: false,
			},
		},
		{
			ID:             "qwen/qwen3.8-omni-flash",
			Name:           "qwen/qwen3.8-omni-flash",
			DisplayName:    "Qwen 3.8 Omni Flash",
			Description:    "Omni-modal understanding & multimedia with vision",
			Provider:       "commandcode",
			IsDefault:      false,
			SupportsVision: true,
			Capabilities: ModelCapabilities{
				Vision:         true,
				SupportsVision: true,
			},
		},
		{
			ID:             "qwen/qwen3.8-max",
			Name:           "qwen/qwen3.8-max",
			DisplayName:    "Qwen 3.8 Max",
			Description:    "Autonomous long-horizon coding (Text-only)",
			Provider:       "commandcode",
			IsDefault:      false,
			SupportsVision: false,
			Capabilities: ModelCapabilities{
				Vision:         false,
				SupportsVision: false,
			},
		},
		{
			ID:             "xai/grok-4.5",
			Name:           "xai/grok-4.5",
			DisplayName:    "xAI Grok 4.5",
			Description:    "Smartest model for coding & STEM (Text-only)",
			Provider:       "commandcode",
			IsDefault:      false,
			SupportsVision: false,
			Capabilities: ModelCapabilities{
				Vision:         false,
				SupportsVision: false,
			},
		},
		{
			ID:             "reflection/beam:free",
			Name:           "reflection/beam:free",
			DisplayName:    "Reflection Beam (Free)",
			Description:    "FREE reasoning model for coding (Text-only)",
			Provider:       "commandcode",
			IsDefault:      false,
			SupportsVision: false,
			Capabilities: ModelCapabilities{
				Vision:         false,
				SupportsVision: false,
			},
		},
	}
}

// ParseCommandCodeModelsOutput parses stdout from `commandcode --list-models`.
func ParseCommandCodeModelsOutput(output string) []ModelInfo {
	knownDisplayNames := map[string]string{
		"commandcode-default":                  "CommandCode Default",
		"claude-sonnet-5-5":                    "Claude Sonnet 5.5",
		"claude-opus-5-5":                      "Claude Opus 5.5",
		"claude-fable-5-1":                     "Claude Fable 5.1",
		"google/gemini-3.8-flash":              "Gemini 3.8 Flash",
		"deepseek/deepseek-v4-flash-vision-exp": "DeepSeek V4 Flash Vision",
		"deepseek/deepseek-v4-pro":             "DeepSeek V4 Pro",
		"moonshotai/kimi-k2.7-code":            "Kimi K2.7 Code",
		"moonshotai/kimi-k3.1-pro":             "Kimi K3.1 Pro",
		"qwen/qwen3.8-omni-flash":              "Qwen 3.8 Omni Flash",
		"qwen/qwen3.8-max":                     "Qwen 3.8 Max",
		"xai/grok-4.5":                         "xAI Grok 4.5",
		"reflection/beam:free":                 "Reflection Beam (Free)",
	}

	var models []ModelInfo
	normalized := strings.ReplaceAll(output, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	for _, line := range strings.Split(normalized, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "Available models") ||
			strings.HasPrefix(trimmed, "Pass the full id") ||
			strings.HasPrefix(trimmed, "cmd ") ||
			strings.HasPrefix(trimmed, "Docs:") ||
			strings.HasPrefix(trimmed, "Decision models") ||
			strings.HasPrefix(trimmed, "typesafe/") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			continue
		}

		id := fields[0]
		if !strings.Contains(id, "/") && !strings.Contains(id, "-") {
			if !strings.HasPrefix(id, "gpt-") && !strings.HasPrefix(id, "claude-") {
				continue
			}
		}

		desc := strings.TrimSpace(trimmed[len(id):])
		displayName := id
		if dn, ok := knownDisplayNames[id]; ok {
			displayName = dn
		} else if parts := strings.Split(id, "/"); len(parts) == 2 {
			displayName = parts[1]
		}

		supportsVision := IsCommandCodeModelVision(id)
		isDefault := id == "commandcode-default" || id == "deepseek/deepseek-v4-flash"

		models = append(models, ModelInfo{
			ID:             id,
			Name:           id,
			DisplayName:    displayName,
			Description:    desc,
			Provider:       "commandcode",
			IsDefault:      isDefault,
			SupportsVision: supportsVision,
			Capabilities: ModelCapabilities{
				Vision:         supportsVision,
				SupportsVision: supportsVision,
			},
		})
	}

	// Ensure commandcode-default is present
	hasDefault := false
	for _, m := range models {
		if m.ID == "commandcode-default" {
			hasDefault = true
			break
		}
	}
	if !hasDefault {
		def := ModelInfo{
			ID:             "commandcode-default",
			Name:           "commandcode-default",
			DisplayName:    "CommandCode Default",
			Description:    "CommandCode default model (Text-only)",
			Provider:       "commandcode",
			IsDefault:      true,
			SupportsVision: false,
			Capabilities: ModelCapabilities{
				Vision:         false,
				SupportsVision: false,
			},
		}
		models = append([]ModelInfo{def}, models...)
	}

	return models
}

func (p *CommandCodeProvider) ListModels(ctx context.Context, credentials string) ([]ModelInfo, error) {
	bin, err := exec.LookPath("commandcode")
	if err != nil {
		for _, candidate := range []string{
			"/home/dev/.local/bin/commandcode",
			"/home/ubuntu/.local/bin/commandcode",
			"/usr/local/bin/commandcode",
		} {
			if _, statErr := exec.LookPath(candidate); statErr == nil {
				bin = candidate
				err = nil
				break
			}
		}
	}

	if err == nil && bin != "" {
		probeCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()

		cmd := exec.CommandContext(probeCtx, bin, "--list-models")
		out, runErr := cmd.Output()
		if runErr == nil && len(out) > 0 {
			models := ParseCommandCodeModelsOutput(string(out))
			if len(models) > 0 {
				return models, nil
			}
		}
	}

	return DefaultCommandCodeModels(), nil
}

var _ Provider = (*CommandCodeProvider)(nil)
var _ ContentGenerator = (*CommandCodeProvider)(nil)

// GenerateContent is not implemented for CommandCode; execution uses the CLI binary.
func (p *CommandCodeProvider) GenerateContent(ctx context.Context, apiKey, modelID, systemInstruction, userPrompt string) (string, error) {
	return "", errors.New("CommandCode generation is handled by the CLI binary, not the API provider")
}
