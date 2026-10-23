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

// VerifyConnection checks that the commandcode binary exists and the API key is present.
func (p *CommandCodeProvider) VerifyConnection(ctx context.Context, credentials string) (*ConnectionProbeResult, error) {
	trimmed := strings.TrimSpace(credentials)
	if trimmed == "" {
		return &ConnectionProbeResult{
			Valid:        false,
			Provider:     "commandcode",
			ErrorMessage: "missing CommandCode API key",
			TestedAt:     time.Now().UTC(),
		}, ErrInvalidCredentials
	}

	if _, err := exec.LookPath("commandcode"); err != nil {
		return &ConnectionProbeResult{
			Valid:        false,
			Provider:     "commandcode",
			ErrorMessage: "CommandCode CLI not installed on this server",
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
	if strings.TrimSpace(credentials) == "" {
		return nil, ErrInvalidCredentials
	}
	return []ModelInfo{
		{
			ID:               "commandcode-default",
			Name:             "commandcode-default",
			DisplayName:      "CommandCode Default",
			Description:      "CommandCode default model",
			IsDefault:        true,
		},
	}, nil
}

var _ Provider = (*CommandCodeProvider)(nil)
var _ ContentGenerator = (*CommandCodeProvider)(nil)

// GenerateContent is not implemented for CommandCode; execution uses the CLI binary.
func (p *CommandCodeProvider) GenerateContent(ctx context.Context, apiKey, modelID, systemInstruction, userPrompt string) (string, error) {
	return "", errors.New("CommandCode generation is handled by the CLI binary, not the API provider")
}
