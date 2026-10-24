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

// VerifyConnection checks that the commandcode binary exists and credentials (if BYOK) are valid.
func (p *CommandCodeProvider) VerifyConnection(ctx context.Context, credentials string) (*ConnectionProbeResult, error) {
	trimmed := strings.TrimSpace(credentials)

	if _, err := exec.LookPath("commandcode"); err != nil {
		found := false
		for _, candidate := range []string{
			"/home/dev/.local/bin/commandcode",
			"/home/ubuntu/.local/bin/commandcode",
			"/usr/local/bin/commandcode",
		} {
			if _, statErr := exec.LookPath(candidate); statErr == nil {
				found = true
				break
			}
		}
		if !found {
			return &ConnectionProbeResult{
				Valid:        false,
				Provider:     "commandcode",
				ErrorMessage: "CommandCode CLI not installed on this server",
				TestedAt:     time.Now().UTC(),
			}, nil
		}
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

func (p *CommandCodeProvider) ListModels(ctx context.Context, credentials string) ([]ModelInfo, error) {
	return []ModelInfo{
		{
			ID:             "commandcode-default",
			Name:           "commandcode-default",
			DisplayName:    "CommandCode Default",
			Description:    "CommandCode default model (Text-only)",
			IsDefault:      true,
			SupportsVision: false,
		},
		{
			ID:             "claude-sonnet-5-5",
			Name:           "claude-sonnet-5-5",
			DisplayName:    "Claude Sonnet 5.5",
			Description:    "Frontier coding & intelligence with vision",
			IsDefault:      false,
			SupportsVision: true,
		},
		{
			ID:             "claude-opus-5-5",
			Name:           "claude-opus-5-5",
			DisplayName:    "Claude Opus 5.5",
			Description:    "Most intelligent Opus for agents with vision",
			IsDefault:      false,
			SupportsVision: true,
		},
		{
			ID:             "claude-fable-5-1",
			Name:           "claude-fable-5-1",
			DisplayName:    "Claude Fable 5.1",
			Description:    "Demanding reasoning & long-horizon agents (Text-only)",
			IsDefault:      false,
			SupportsVision: false,
		},
		{
			ID:             "google/gemini-3.8-flash",
			Name:           "google/gemini-3.8-flash",
			DisplayName:    "Gemini 3.8 Flash",
			Description:    "Newest Gemini Flash, core reasoning with vision",
			IsDefault:      false,
			SupportsVision: true,
		},
		{
			ID:             "deepseek/deepseek-v4-flash-vision-exp",
			Name:           "deepseek/deepseek-v4-flash-vision-exp",
			DisplayName:    "DeepSeek V4 Flash Vision",
			Description:    "Fast hybrid-attention reasoning with vision",
			IsDefault:      false,
			SupportsVision: true,
		},
		{
			ID:             "deepseek/deepseek-v4-pro",
			Name:           "deepseek/deepseek-v4-pro",
			DisplayName:    "DeepSeek V4 Pro",
			Description:    "Hybrid-attention long-context reasoning (Text-only)",
			IsDefault:      false,
			SupportsVision: false,
		},
		{
			ID:             "moonshotai/kimi-k2.7-code",
			Name:           "moonshotai/kimi-k2.7-code",
			DisplayName:    "Kimi K2.7 Code",
			Description:    "Improved long-horizon coding with vision",
			IsDefault:      false,
			SupportsVision: true,
		},
		{
			ID:             "moonshotai/kimi-k3.1-pro",
			Name:           "moonshotai/kimi-k3.1-pro",
			DisplayName:    "Kimi K3.1 Pro",
			Description:    "Frontier reasoning & coding with 1M context (Text-only)",
			IsDefault:      false,
			SupportsVision: false,
		},
		{
			ID:             "qwen/qwen3.8-omni-flash",
			Name:           "qwen/qwen3.8-omni-flash",
			DisplayName:    "Qwen 3.8 Omni Flash",
			Description:    "Omni-modal understanding & multimedia with vision",
			IsDefault:      false,
			SupportsVision: true,
		},
		{
			ID:             "qwen/qwen3.8-max",
			Name:           "qwen/qwen3.8-max",
			DisplayName:    "Qwen 3.8 Max",
			Description:    "Autonomous long-horizon coding (Text-only)",
			IsDefault:      false,
			SupportsVision: false,
		},
		{
			ID:             "xai/grok-4.5",
			Name:           "xai/grok-4.5",
			DisplayName:    "xAI Grok 4.5",
			Description:    "Smartest model for coding & STEM (Text-only)",
			IsDefault:      false,
			SupportsVision: false,
		},
		{
			ID:             "reflection/beam:free",
			Name:           "reflection/beam:free",
			DisplayName:    "Reflection Beam (Free)",
			Description:    "FREE reasoning model for coding (Text-only)",
			IsDefault:      false,
			SupportsVision: false,
		},
	}, nil
}

var _ Provider = (*CommandCodeProvider)(nil)
var _ ContentGenerator = (*CommandCodeProvider)(nil)

// GenerateContent is not implemented for CommandCode; execution uses the CLI binary.
func (p *CommandCodeProvider) GenerateContent(ctx context.Context, apiKey, modelID, systemInstruction, userPrompt string) (string, error) {
	return "", errors.New("CommandCode generation is handled by the CLI binary, not the API provider")
}
