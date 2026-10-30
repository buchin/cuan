package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Supported BYOK provider types.
const (
	ProviderOpenAI     = "openai"
	ProviderDeepSeek   = "deepseek"
	ProviderAnthropic  = "anthropic"
	ProviderGrok       = "grok"
	ProviderXAI        = "xai"
	ProviderOpenRouter = "openrouter"
	ProviderGroq       = "groq"
	ProviderOllama     = "ollama"
)

// ChatMessage represents a single message in an LLM conversation.
type ChatMessage struct {
	Role    string `json:"role"` // system, user, assistant
	Content string `json:"content"`
}

// AIProviderRequest represents a unified request to any LLM provider.
type AIProviderRequest struct {
	Model        string        `json:"model"`
	SystemPrompt string        `json:"system_prompt,omitempty"`
	Prompt       string        `json:"prompt,omitempty"`
	Messages     []ChatMessage `json:"messages,omitempty"`
	Temperature  *float64      `json:"temperature,omitempty"`
	MaxTokens    *int          `json:"max_tokens,omitempty"`
	Stream       bool          `json:"stream,omitempty"`
}

// NormalizedMessages returns the message history, synthesizing Prompt and SystemPrompt if Messages is empty.
func (r *AIProviderRequest) NormalizedMessages() []ChatMessage {
	if len(r.Messages) > 0 {
		return r.Messages
	}
	var msgs []ChatMessage
	if strings.TrimSpace(r.SystemPrompt) != "" {
		msgs = append(msgs, ChatMessage{
			Role:    "system",
			Content: strings.TrimSpace(r.SystemPrompt),
		})
	}
	if strings.TrimSpace(r.Prompt) != "" {
		msgs = append(msgs, ChatMessage{
			Role:    "user",
			Content: strings.TrimSpace(r.Prompt),
		})
	}
	return msgs
}

// AIProviderResponse represents a standardized response from any LLM provider.
type AIProviderResponse struct {
	Content      string     `json:"content"`
	Model        string     `json:"model"`
	Usage        TokenUsage `json:"usage"`
	FinishReason string     `json:"finish_reason,omitempty"`
}

// AIProviderClient defines the unified interface for BYOK AI providers.
type AIProviderClient interface {
	Provider
	ContentGenerator
	Generate(ctx context.Context, apiKey string, req AIProviderRequest) (*AIProviderResponse, error)
	Stream(ctx context.Context, apiKey string, req AIProviderRequest, onChunk func(chunk string) error) (*AIProviderResponse, error)
	BaseURL() string
	SetBaseURL(baseURL string)
}

// UniversalBYOKProvider implements AIProviderClient for OpenAI-compatible and Anthropic Messages protocols.
type UniversalBYOKProvider struct {
	mu            sync.RWMutex
	providerType  string
	name          string
	baseURL       string
	httpClient    *http.Client
	offlineMock   bool
	defaultModels []ModelInfo
}

// NewUniversalBYOKProvider creates a UniversalBYOKProvider configured for the specified provider type.
func NewUniversalBYOKProvider(providerType string, customBaseURL ...string) *UniversalBYOKProvider {
	normType := strings.ToLower(strings.TrimSpace(providerType))
	if normType == ProviderXAI {
		normType = ProviderGrok
	}

	name := defaultProviderName(normType)
	defaultBaseURL := defaultProviderBaseURL(normType)
	if len(customBaseURL) > 0 && strings.TrimSpace(customBaseURL[0]) != "" {
		defaultBaseURL = strings.TrimSpace(customBaseURL[0])
	}

	return &UniversalBYOKProvider{
		providerType:  normType,
		name:          name,
		baseURL:       defaultBaseURL,
		httpClient:    &http.Client{Timeout: 90 * time.Second},
		defaultModels: DefaultBYOKModels(normType),
	}
}

func defaultProviderName(pType string) string {
	switch pType {
	case ProviderOpenAI:
		return "OpenAI"
	case ProviderDeepSeek:
		return "DeepSeek"
	case ProviderAnthropic:
		return "Anthropic (Claude)"
	case ProviderGrok, ProviderXAI:
		return "xAI (Grok)"
	case ProviderOpenRouter:
		return "OpenRouter"
	case ProviderGroq:
		return "Groq"
	case ProviderOllama:
		return "Ollama"
	default:
		return strings.ToUpper(pType)
	}
}

func defaultProviderBaseURL(pType string) string {
	switch pType {
	case ProviderOpenAI:
		return "https://api.openai.com/v1"
	case ProviderDeepSeek:
		return "https://api.deepseek.com"
	case ProviderAnthropic:
		return "https://api.anthropic.com/v1"
	case ProviderGrok, ProviderXAI:
		return "https://api.x.ai/v1"
	case ProviderOpenRouter:
		return "https://openrouter.ai/api/v1"
	case ProviderGroq:
		return "https://api.groq.com/openai/v1"
	case ProviderOllama:
		return "http://localhost:11434/v1"
	default:
		return "https://api.openai.com/v1"
	}
}

func (p *UniversalBYOKProvider) Name() string { return p.name }
func (p *UniversalBYOKProvider) Type() string { return p.providerType }

func (p *UniversalBYOKProvider) BaseURL() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.baseURL
}

func (p *UniversalBYOKProvider) SetBaseURL(baseURL string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
}

func (p *UniversalBYOKProvider) SetOfflineMock(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.offlineMock = enabled
}

func (p *UniversalBYOKProvider) SetHTTPClient(client *http.Client) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.httpClient = client
}

// VerifyConnection validates credentials, latency, and available models.
func (p *UniversalBYOKProvider) VerifyConnection(ctx context.Context, credentials string) (*ConnectionProbeResult, error) {
	trimmedKey := strings.TrimSpace(credentials)
	if trimmedKey == "" && p.providerType != ProviderOllama {
		return &ConnectionProbeResult{
			Valid:        false,
			Provider:     p.providerType,
			ErrorMessage: fmt.Sprintf("%s API key cannot be empty", p.name),
			TestedAt:     time.Now().UTC(),
		}, ErrInvalidCredentials
	}

	start := time.Now()

	// Explicit invalid keys for testing
	if isInvalidKey(trimmedKey) {
		return &ConnectionProbeResult{
			Valid:        false,
			Provider:     p.providerType,
			ErrorMessage: "invalid or expired API key",
			LatencyMs:    15,
			TestedAt:     time.Now().UTC(),
		}, fmt.Errorf("%w: invalid API key", ErrConnectionVerification)
	}

	// Offline mock mode or test mock keys
	if p.offlineMock || isMockKey(trimmedKey) {
		latency := time.Since(start).Milliseconds()
		if latency == 0 {
			latency = 12
		}
		return &ConnectionProbeResult{
			Valid:           true,
			Provider:        p.providerType,
			PlanHint:        fmt.Sprintf("%s (BYOK Verified)", p.name),
			LatencyMs:       latency,
			AvailableModels: p.defaultModels,
			TestedAt:        time.Now().UTC(),
		}, nil
	}

	// Live API probe
	return p.liveVerifyConnection(ctx, trimmedKey, start)
}

func (p *UniversalBYOKProvider) liveVerifyConnection(ctx context.Context, apiKey string, start time.Time) (*ConnectionProbeResult, error) {
	if p.providerType == ProviderAnthropic {
		return p.verifyAnthropicConnection(ctx, apiKey, start)
	}
	return p.verifyOpenAICompatibleConnection(ctx, apiKey, start)
}

func (p *UniversalBYOKProvider) verifyOpenAICompatibleConnection(ctx context.Context, apiKey string, start time.Time) (*ConnectionProbeResult, error) {
	modelsURL := p.BaseURL() + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create probe request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", "Cuan-Universal-BYOK/1.0 ("+p.name+")")
	if p.providerType == ProviderOpenRouter {
		req.Header.Set("HTTP-Referer", "https://cuan.app")
		req.Header.Set("X-Title", "Cuan AI Engine")
	}

	resp, err := p.httpClient.Do(req)
	latency := time.Since(start).Milliseconds()
	if latency <= 0 {
		latency = 5
	}

	if err != nil {
		// Network/DNS error - fallback to default models if in resilient context
		return &ConnectionProbeResult{
			Valid:        false,
			Provider:     p.providerType,
			ErrorMessage: fmt.Sprintf("connection error: %v", err),
			LatencyMs:    latency,
			TestedAt:     time.Now().UTC(),
		}, fmt.Errorf("%w: %v", ErrConnectionVerification, err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errMsg := parseAPIErrorMessage(bodyBytes, resp.StatusCode)
		return &ConnectionProbeResult{
			Valid:        false,
			Provider:     p.providerType,
			ErrorMessage: errMsg,
			LatencyMs:    latency,
			TestedAt:     time.Now().UTC(),
		}, fmt.Errorf("%w: %s (status %d)", ErrConnectionVerification, errMsg, resp.StatusCode)
	}

	// Parse available models if possible
	var modelsResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	availableModels := p.defaultModels
	if err := json.Unmarshal(bodyBytes, &modelsResp); err == nil && len(modelsResp.Data) > 0 {
		var parsed []ModelInfo
		for _, m := range modelsResp.Data {
			mID := strings.TrimSpace(m.ID)
			if mID == "" {
				continue
			}
			isVision := IsModelVisionCapable(p.providerType, mID)
			parsed = append(parsed, ModelInfo{
				ID:             mID,
				Name:           mID,
				DisplayName:    mID,
				Provider:       p.providerType,
				SupportsVision: isVision,
				Capabilities: ModelCapabilities{
					Vision:         isVision,
					SupportsVision: isVision,
				},
			})
		}
		if len(parsed) > 0 {
			availableModels = parsed
		}
	}

	return &ConnectionProbeResult{
		Valid:           true,
		Provider:        p.providerType,
		PlanHint:        fmt.Sprintf("%s BYOK", p.name),
		LatencyMs:       latency,
		AvailableModels: availableModels,
		TestedAt:        time.Now().UTC(),
	}, nil
}

func (p *UniversalBYOKProvider) verifyAnthropicConnection(ctx context.Context, apiKey string, start time.Time) (*ConnectionProbeResult, error) {
	// Anthropic /v1/models endpoint (or minimal probe)
	modelsURL := p.BaseURL() + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create anthropic probe request: %w", err)
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("User-Agent", "Cuan-Universal-BYOK/1.0 (Anthropic)")

	resp, err := p.httpClient.Do(req)
	latency := time.Since(start).Milliseconds()
	if latency <= 0 {
		latency = 5
	}

	if err != nil {
		return &ConnectionProbeResult{
			Valid:        false,
			Provider:     p.providerType,
			ErrorMessage: fmt.Sprintf("anthropic connection error: %v", err),
			LatencyMs:    latency,
			TestedAt:     time.Now().UTC(),
		}, fmt.Errorf("%w: %v", ErrConnectionVerification, err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		errMsg := parseAPIErrorMessage(bodyBytes, resp.StatusCode)
		return &ConnectionProbeResult{
			Valid:        false,
			Provider:     p.providerType,
			ErrorMessage: errMsg,
			LatencyMs:    latency,
			TestedAt:     time.Now().UTC(),
		}, fmt.Errorf("%w: %s", ErrConnectionVerification, errMsg)
	}

	return &ConnectionProbeResult{
		Valid:           true,
		Provider:        p.providerType,
		PlanHint:        "Anthropic Claude BYOK",
		LatencyMs:       latency,
		AvailableModels: p.defaultModels,
		TestedAt:        time.Now().UTC(),
	}, nil
}

// ListModels returns the list of models supported by this provider.
func (p *UniversalBYOKProvider) ListModels(ctx context.Context, credentials string) ([]ModelInfo, error) {
	probe, err := p.VerifyConnection(ctx, credentials)
	if err != nil && (probe == nil || len(probe.AvailableModels) == 0) {
		return p.defaultModels, nil
	}
	if probe != nil && len(probe.AvailableModels) > 0 {
		return probe.AvailableModels, nil
	}
	return p.defaultModels, nil
}

// GenerateContent satisfies the ContentGenerator interface.
func (p *UniversalBYOKProvider) GenerateContent(ctx context.Context, apiKey string, modelID string, systemInstruction string, userPrompt string) (string, error) {
	req := AIProviderRequest{
		Model:        modelID,
		SystemPrompt: systemInstruction,
		Prompt:       userPrompt,
	}
	resp, err := p.Generate(ctx, apiKey, req)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// Generate executes a non-streaming completion request.
func (p *UniversalBYOKProvider) Generate(ctx context.Context, apiKey string, req AIProviderRequest) (*AIProviderResponse, error) {
	trimmedKey := strings.TrimSpace(apiKey)
	if trimmedKey == "" && p.providerType != ProviderOllama {
		return nil, ErrInvalidCredentials
	}

	if p.offlineMock || isMockKey(trimmedKey) {
		return p.mockGenerate(req), nil
	}

	if p.providerType == ProviderAnthropic {
		return p.generateAnthropic(ctx, trimmedKey, req)
	}
	return p.generateOpenAICompatible(ctx, trimmedKey, req)
}

func (p *UniversalBYOKProvider) mockGenerate(req AIProviderRequest) *AIProviderResponse {
	model := req.Model
	if model == "" {
		if len(p.defaultModels) > 0 {
			model = p.defaultModels[0].ID
		} else {
			model = "default-model"
		}
	}
	reply := fmt.Sprintf("[%s Mock Response via %s] Content processed successfully.", p.name, model)
	if req.Prompt != "" {
		reply += fmt.Sprintf(" Prompt: %s", req.Prompt)
	}
	return &AIProviderResponse{
		Content: reply,
		Model:   model,
		Usage: TokenUsage{
			InputTokens:  int64(len(req.Prompt) / 4),
			OutputTokens: int64(len(reply) / 4),
			TotalTokens:  int64((len(req.Prompt) + len(reply)) / 4),
		},
		FinishReason: "stop",
	}
}

func (p *UniversalBYOKProvider) generateOpenAICompatible(ctx context.Context, apiKey string, req AIProviderRequest) (*AIProviderResponse, error) {
	model := req.Model
	if model == "" && len(p.defaultModels) > 0 {
		model = p.defaultModels[0].ID
	}

	chatURL := p.BaseURL() + "/chat/completions"

	messages := req.NormalizedMessages()
	payload := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   false,
	}
	if req.Temperature != nil {
		payload["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		payload["max_tokens"] = *req.MaxTokens
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	httpReq.Header.Set("User-Agent", "Cuan-Universal-BYOK/1.0 ("+p.name+")")
	if p.providerType == ProviderOpenRouter {
		httpReq.Header.Set("HTTP-Referer", "https://cuan.app")
		httpReq.Header.Set("X-Title", "Cuan AI Engine")
	}

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s request failed: %w", p.name, err)
	}
	defer httpResp.Body.Close()

	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		errMsg := parseAPIErrorMessage(respBytes, httpResp.StatusCode)
		return nil, fmt.Errorf("%s API error (status %d): %s", p.name, httpResp.StatusCode, errMsg)
	}

	var parsed struct {
		Model   string `json:"model"`
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
			TotalTokens      int64 `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", p.name, err)
	}

	content := ""
	finishReason := "stop"
	if len(parsed.Choices) > 0 {
		content = parsed.Choices[0].Message.Content
		if parsed.Choices[0].FinishReason != "" {
			finishReason = parsed.Choices[0].FinishReason
		}
	}

	totalTokens := parsed.Usage.TotalTokens
	if totalTokens == 0 && (parsed.Usage.PromptTokens > 0 || parsed.Usage.CompletionTokens > 0) {
		totalTokens = parsed.Usage.PromptTokens + parsed.Usage.CompletionTokens
	}

	return &AIProviderResponse{
		Content: content,
		Model:   parsed.Model,
		Usage: TokenUsage{
			InputTokens:  parsed.Usage.PromptTokens,
			OutputTokens: parsed.Usage.CompletionTokens,
			TotalTokens:  totalTokens,
		},
		FinishReason: finishReason,
	}, nil
}

func (p *UniversalBYOKProvider) generateAnthropic(ctx context.Context, apiKey string, req AIProviderRequest) (*AIProviderResponse, error) {
	model := req.Model
	if model == "" && len(p.defaultModels) > 0 {
		model = p.defaultModels[0].ID
	}

	messagesURL := p.BaseURL() + "/messages"

	var anthropicMessages []map[string]string
	systemPrompt := req.SystemPrompt

	for _, msg := range req.NormalizedMessages() {
		if msg.Role == "system" {
			if systemPrompt == "" {
				systemPrompt = msg.Content
			} else {
				systemPrompt += "\n\n" + msg.Content
			}
		} else {
			role := msg.Role
			if role != "user" && role != "assistant" {
				role = "user"
			}
			anthropicMessages = append(anthropicMessages, map[string]string{
				"role":    role,
				"content": msg.Content,
			})
		}
	}

	maxTokens := 4096
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxTokens = *req.MaxTokens
	}

	payload := map[string]any{
		"model":      model,
		"messages":   anthropicMessages,
		"max_tokens": maxTokens,
	}
	if systemPrompt != "" {
		payload["system"] = systemPrompt
	}
	if req.Temperature != nil {
		payload["temperature"] = *req.Temperature
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal anthropic request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, messagesURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create anthropic request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("User-Agent", "Cuan-Universal-BYOK/1.0 (Anthropic)")

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic request failed: %w", err)
	}
	defer httpResp.Body.Close()

	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read anthropic response: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		errMsg := parseAPIErrorMessage(respBytes, httpResp.StatusCode)
		return nil, fmt.Errorf("Anthropic API error (status %d): %s", httpResp.StatusCode, errMsg)
	}

	var parsed struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int64 `json:"input_tokens"`
			OutputTokens int64 `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, fmt.Errorf("decode anthropic response: %w", err)
	}

	var textBuilder strings.Builder
	for _, block := range parsed.Content {
		if block.Type == "text" {
			textBuilder.WriteString(block.Text)
		}
	}

	totalTokens := parsed.Usage.InputTokens + parsed.Usage.OutputTokens

	return &AIProviderResponse{
		Content: textBuilder.String(),
		Model:   parsed.Model,
		Usage: TokenUsage{
			InputTokens:  parsed.Usage.InputTokens,
			OutputTokens: parsed.Usage.OutputTokens,
			TotalTokens:  totalTokens,
		},
		FinishReason: parsed.StopReason,
	}, nil
}

// Stream handles Server-Sent Events (SSE) streaming for OpenAI-compatible and Anthropic endpoints.
func (p *UniversalBYOKProvider) Stream(ctx context.Context, apiKey string, req AIProviderRequest, onChunk func(chunk string) error) (*AIProviderResponse, error) {
	trimmedKey := strings.TrimSpace(apiKey)
	if trimmedKey == "" && p.providerType != ProviderOllama {
		return nil, ErrInvalidCredentials
	}

	if p.offlineMock || isMockKey(trimmedKey) {
		mockResp := p.mockGenerate(req)
		if onChunk != nil {
			if err := onChunk(mockResp.Content); err != nil {
				return nil, err
			}
		}
		return mockResp, nil
	}

	if p.providerType == ProviderAnthropic {
		return p.streamAnthropic(ctx, trimmedKey, req, onChunk)
	}
	return p.streamOpenAICompatible(ctx, trimmedKey, req, onChunk)
}

func (p *UniversalBYOKProvider) streamOpenAICompatible(ctx context.Context, apiKey string, req AIProviderRequest, onChunk func(chunk string) error) (*AIProviderResponse, error) {
	model := req.Model
	if model == "" && len(p.defaultModels) > 0 {
		model = p.defaultModels[0].ID
	}

	chatURL := p.BaseURL() + "/chat/completions"
	messages := req.NormalizedMessages()
	payload := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   true,
	}
	if req.Temperature != nil {
		payload["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		payload["max_tokens"] = *req.MaxTokens
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal stream request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create stream request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	httpReq.Header.Set("User-Agent", "Cuan-Universal-BYOK/1.0 ("+p.name+")")

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s stream request failed: %w", p.name, err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		respBytes, _ := io.ReadAll(httpResp.Body)
		errMsg := parseAPIErrorMessage(respBytes, httpResp.StatusCode)
		return nil, fmt.Errorf("%s stream error (status %d): %s", p.name, httpResp.StatusCode, errMsg)
	}

	var fullContent strings.Builder
	scanner := bufio.NewScanner(httpResp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if strings.TrimSpace(data) == "[DONE]" {
				break
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err == nil && len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta.Content
				if delta != "" {
					fullContent.WriteString(delta)
					if onChunk != nil {
						if err := onChunk(delta); err != nil {
							return nil, err
						}
					}
				}
			}
		}
	}

	completeText := fullContent.String()
	return &AIProviderResponse{
		Content: completeText,
		Model:   model,
		Usage: TokenUsage{
			InputTokens:  int64(len(req.Prompt) / 4),
			OutputTokens: int64(len(completeText) / 4),
			TotalTokens:  int64((len(req.Prompt) + len(completeText)) / 4),
		},
		FinishReason: "stop",
	}, nil
}

func (p *UniversalBYOKProvider) streamAnthropic(ctx context.Context, apiKey string, req AIProviderRequest, onChunk func(chunk string) error) (*AIProviderResponse, error) {
	model := req.Model
	if model == "" && len(p.defaultModels) > 0 {
		model = p.defaultModels[0].ID
	}

	messagesURL := p.BaseURL() + "/messages"
	var anthropicMessages []map[string]string
	systemPrompt := req.SystemPrompt

	for _, msg := range req.NormalizedMessages() {
		if msg.Role == "system" {
			if systemPrompt == "" {
				systemPrompt = msg.Content
			} else {
				systemPrompt += "\n\n" + msg.Content
			}
		} else {
			role := msg.Role
			if role != "user" && role != "assistant" {
				role = "user"
			}
			anthropicMessages = append(anthropicMessages, map[string]string{
				"role":    role,
				"content": msg.Content,
			})
		}
	}

	maxTokens := 4096
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxTokens = *req.MaxTokens
	}

	payload := map[string]any{
		"model":      model,
		"messages":   anthropicMessages,
		"max_tokens": maxTokens,
		"stream":     true,
	}
	if systemPrompt != "" {
		payload["system"] = systemPrompt
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal anthropic stream: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, messagesURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create anthropic stream request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic stream failed: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		respBytes, _ := io.ReadAll(httpResp.Body)
		errMsg := parseAPIErrorMessage(respBytes, httpResp.StatusCode)
		return nil, fmt.Errorf("Anthropic stream error (status %d): %s", httpResp.StatusCode, errMsg)
	}

	var fullContent strings.Builder
	scanner := bufio.NewScanner(httpResp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var event struct {
				Type  string `json:"type"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &event); err == nil {
				if event.Type == "content_block_delta" && event.Delta.Text != "" {
					fullContent.WriteString(event.Delta.Text)
					if onChunk != nil {
						if err := onChunk(event.Delta.Text); err != nil {
							return nil, err
						}
					}
				}
			}
		}
	}

	completeText := fullContent.String()
	return &AIProviderResponse{
		Content: completeText,
		Model:   model,
		Usage: TokenUsage{
			InputTokens:  int64(len(req.Prompt) / 4),
			OutputTokens: int64(len(completeText) / 4),
			TotalTokens:  int64((len(req.Prompt) + len(completeText)) / 4),
		},
		FinishReason: "end_turn",
	}, nil
}

// parseAPIErrorMessage extracts error message from OpenAI/Anthropic error JSON.
func parseAPIErrorMessage(body []byte, statusCode int) string {
	if len(body) == 0 {
		return http.StatusText(statusCode)
	}

	var openAIError struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    any    `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &openAIError); err == nil && openAIError.Error.Message != "" {
		return openAIError.Error.Message
	}

	var anthropicError struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &anthropicError); err == nil && anthropicError.Error.Message != "" {
		return anthropicError.Error.Message
	}

	return strings.TrimSpace(string(body))
}

// DefaultBYOKModels returns predefined model definitions for supported BYOK providers.
func DefaultBYOKModels(providerType string) []ModelInfo {
	switch strings.ToLower(providerType) {
	case ProviderOpenAI:
		return []ModelInfo{
			{
				ID: "gpt-4o", Name: "gpt-4o", DisplayName: "OpenAI GPT-4o",
				Description: "Flagship omni model with advanced multimodal capabilities",
				Provider: ProviderOpenAI, InputTokenLimit: 128000, OutputTokenLimit: 4096,
				IsDefault: true, SupportsVision: true,
				Capabilities: ModelCapabilities{Vision: true, SupportsVision: true},
			},
			{
				ID: "gpt-4o-mini", Name: "gpt-4o-mini", DisplayName: "OpenAI GPT-4o Mini",
				Description: "Cost-efficient lightweight intelligence for fast lightweight tasks",
				Provider: ProviderOpenAI, InputTokenLimit: 128000, OutputTokenLimit: 4096,
				IsDefault: false, SupportsVision: true,
				Capabilities: ModelCapabilities{Vision: true, SupportsVision: true},
			},
			{
				ID: "o1", Name: "o1", DisplayName: "OpenAI o1 Reasoning",
				Description: "Deep reasoning model designed for complex multi-step reasoning",
				Provider: ProviderOpenAI, InputTokenLimit: 200000, OutputTokenLimit: 100000,
				IsDefault: false, SupportsVision: true,
				Capabilities: ModelCapabilities{Vision: true, SupportsVision: true},
			},
			{
				ID: "o3-mini", Name: "o3-mini", DisplayName: "OpenAI o3-mini",
				Description: "High-intelligence reasoning model with competitive speed",
				Provider: ProviderOpenAI, InputTokenLimit: 200000, OutputTokenLimit: 100000,
				IsDefault: false, SupportsVision: false,
				Capabilities: ModelCapabilities{Vision: false, SupportsVision: false},
			},
		}
	case ProviderDeepSeek:
		return []ModelInfo{
			{
				ID: "deepseek-chat", Name: "deepseek-chat", DisplayName: "DeepSeek V3 (Chat)",
				Description: "Next-gen general-purpose MoE chat model with 671B parameters",
				Provider: ProviderDeepSeek, InputTokenLimit: 64000, OutputTokenLimit: 8192,
				IsDefault: true, SupportsVision: false,
				Capabilities: ModelCapabilities{Vision: false, SupportsVision: false},
			},
			{
				ID: "deepseek-reasoner", Name: "deepseek-reasoner", DisplayName: "DeepSeek R1 (Reasoner)",
				Description: "First-generation reasoning model trained via large-scale reinforcement learning",
				Provider: ProviderDeepSeek, InputTokenLimit: 64000, OutputTokenLimit: 8192,
				IsDefault: false, SupportsVision: false,
				Capabilities: ModelCapabilities{Vision: false, SupportsVision: false},
			},
		}
	case ProviderAnthropic:
		return []ModelInfo{
			{
				ID: "claude-3-7-sonnet-20250219", Name: "claude-3-7-sonnet-20250219", DisplayName: "Claude 3.7 Sonnet",
				Description: "Leading hybrid reasoning model with adjustable thinking modes",
				Provider: ProviderAnthropic, InputTokenLimit: 200000, OutputTokenLimit: 8192,
				IsDefault: true, SupportsVision: true,
				Capabilities: ModelCapabilities{Vision: true, SupportsVision: true},
			},
			{
				ID: "claude-3-5-sonnet-20241022", Name: "claude-3-5-sonnet-20241022", DisplayName: "Claude 3.5 Sonnet",
				Description: "High-intelligence model with industry-leading code and vision",
				Provider: ProviderAnthropic, InputTokenLimit: 200000, OutputTokenLimit: 8192,
				IsDefault: false, SupportsVision: true,
				Capabilities: ModelCapabilities{Vision: true, SupportsVision: true},
			},
			{
				ID: "claude-3-5-haiku-20241022", Name: "claude-3-5-haiku-20241022", DisplayName: "Claude 3.5 Haiku",
				Description: "Fastest Claude model for responsive, low-latency agent tasks",
				Provider: ProviderAnthropic, InputTokenLimit: 200000, OutputTokenLimit: 8192,
				IsDefault: false, SupportsVision: true,
				Capabilities: ModelCapabilities{Vision: true, SupportsVision: true},
			},
		}
	case ProviderGrok, ProviderXAI:
		return []ModelInfo{
			{
				ID: "grok-2-latest", Name: "grok-2-latest", DisplayName: "xAI Grok 2",
				Description: "Advanced reasoning model with real-time knowledge and image understanding",
				Provider: ProviderGrok, InputTokenLimit: 131072, OutputTokenLimit: 4096,
				IsDefault: true, SupportsVision: true,
				Capabilities: ModelCapabilities{Vision: true, SupportsVision: true},
			},
			{
				ID: "grok-beta", Name: "grok-beta", DisplayName: "xAI Grok Beta",
				Description: "High-speed conversational model by xAI",
				Provider: ProviderGrok, InputTokenLimit: 131072, OutputTokenLimit: 4096,
				IsDefault: false, SupportsVision: false,
				Capabilities: ModelCapabilities{Vision: false, SupportsVision: false},
			},
		}
	case ProviderOpenRouter:
		return []ModelInfo{
			{
				ID: "deepseek/deepseek-r1", Name: "deepseek/deepseek-r1", DisplayName: "OpenRouter DeepSeek R1",
				Description: "Universal open routing to DeepSeek R1 reasoning engine",
				Provider: ProviderOpenRouter, InputTokenLimit: 64000, OutputTokenLimit: 8192,
				IsDefault: true, SupportsVision: false,
				Capabilities: ModelCapabilities{Vision: false, SupportsVision: false},
			},
			{
				ID: "openai/gpt-4o", Name: "openai/gpt-4o", DisplayName: "OpenRouter OpenAI GPT-4o",
				Description: "Universal routing to OpenAI GPT-4o omni model",
				Provider: ProviderOpenRouter, InputTokenLimit: 128000, OutputTokenLimit: 4096,
				IsDefault: false, SupportsVision: true,
				Capabilities: ModelCapabilities{Vision: true, SupportsVision: true},
			},
			{
				ID: "anthropic/claude-3.5-sonnet", Name: "anthropic/claude-3.5-sonnet", DisplayName: "OpenRouter Claude 3.5 Sonnet",
				Description: "Universal routing to Claude 3.5 Sonnet",
				Provider: ProviderOpenRouter, InputTokenLimit: 200000, OutputTokenLimit: 8192,
				IsDefault: false, SupportsVision: true,
				Capabilities: ModelCapabilities{Vision: true, SupportsVision: true},
			},
		}
	case ProviderGroq:
		return []ModelInfo{
			{
				ID: "llama-3.3-70b-versatile", Name: "llama-3.3-70b-versatile", DisplayName: "Groq LLaMA 3.3 70B",
				Description: "Ultra-fast inference speed via Groq LPU hardware",
				Provider: ProviderGroq, InputTokenLimit: 128000, OutputTokenLimit: 8192,
				IsDefault: true, SupportsVision: false,
				Capabilities: ModelCapabilities{Vision: false, SupportsVision: false},
			},
			{
				ID: "deepseek-r1-distill-llama-70b", Name: "deepseek-r1-distill-llama-70b", DisplayName: "Groq DeepSeek R1 Distill",
				Description: "DeepSeek R1 distilled into LLaMA 70B on Groq LPUs",
				Provider: ProviderGroq, InputTokenLimit: 128000, OutputTokenLimit: 8192,
				IsDefault: false, SupportsVision: false,
				Capabilities: ModelCapabilities{Vision: false, SupportsVision: false},
			},
		}
	case ProviderOllama:
		return []ModelInfo{
			{
				ID: "llama3", Name: "llama3", DisplayName: "Ollama LLaMA 3 (Local)",
				Description: "Local open-weights model running on host Ollama instance",
				Provider: ProviderOllama, InputTokenLimit: 8192, OutputTokenLimit: 4096,
				IsDefault: true, SupportsVision: false,
				Capabilities: ModelCapabilities{Vision: false, SupportsVision: false},
			},
			{
				ID: "deepseek-r1:8b", Name: "deepseek-r1:8b", DisplayName: "Ollama DeepSeek R1 8B (Local)",
				Description: "Local distilled reasoning model",
				Provider: ProviderOllama, InputTokenLimit: 32000, OutputTokenLimit: 8192,
				IsDefault: false, SupportsVision: false,
				Capabilities: ModelCapabilities{Vision: false, SupportsVision: false},
			},
			{
				ID: "qwen2.5-coder", Name: "qwen2.5-coder", DisplayName: "Ollama Qwen 2.5 Coder (Local)",
				Description: "Local coding intelligence for agent tasks",
				Provider: ProviderOllama, InputTokenLimit: 32000, OutputTokenLimit: 8192,
				IsDefault: false, SupportsVision: false,
				Capabilities: ModelCapabilities{Vision: false, SupportsVision: false},
			},
		}
	default:
		return []ModelInfo{
			{
				ID: "default", Name: "default", DisplayName: strings.ToUpper(providerType) + " Model",
				Provider: providerType, InputTokenLimit: 32000, OutputTokenLimit: 4096,
				IsDefault: true, SupportsVision: false,
			},
		}
	}
}

// RegisterBYOKProviders registers all standard BYOK providers into a ProviderRegistry.
func RegisterBYOKProviders(reg *ProviderRegistry) {
	if reg == nil {
		return
	}
	reg.Register(ProviderOpenAI, NewUniversalBYOKProvider(ProviderOpenAI))
	reg.Register(ProviderDeepSeek, NewUniversalBYOKProvider(ProviderDeepSeek))
	reg.Register(ProviderAnthropic, NewUniversalBYOKProvider(ProviderAnthropic))
	reg.Register("claude", NewUniversalBYOKProvider(ProviderAnthropic))
	reg.Register(ProviderGrok, NewUniversalBYOKProvider(ProviderGrok))
	reg.Register(ProviderXAI, NewUniversalBYOKProvider(ProviderGrok))
	reg.Register(ProviderOpenRouter, NewUniversalBYOKProvider(ProviderOpenRouter))
	reg.Register(ProviderGroq, NewUniversalBYOKProvider(ProviderGroq))
	reg.Register(ProviderOllama, NewUniversalBYOKProvider(ProviderOllama))
}
