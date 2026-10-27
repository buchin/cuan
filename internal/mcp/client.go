package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// TransportType identifies how Cuan talks to the MCP server.
type TransportType string

const (
	TransportHTTP  TransportType = "http"
	TransportSSE   TransportType = "sse"
	TransportStdio TransportType = "stdio"
)

// ServerConfig defines an external MCP server connection (e.g. tebar.app).
type ServerConfig struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	BaseURL     string        `json:"base_url"`
	Transport   TransportType `json:"transport"`
	APIKey      string        `json:"api_key,omitempty"`
	Enabled     bool          `json:"enabled"`
	Timeout     time.Duration `json:"timeout,omitempty"`
}

// ToolDefinition describes a capability exposed by the remote MCP server.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema,omitempty"`
}

// ToolCallRequest is the payload sent to execute a remote tool.
type ToolCallRequest struct {
	ToolName  string                 `json:"tool_name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolCallResponse encapsulates the result returned by the remote MCP server.
type ToolCallResponse struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"is_error"`
}

// ToolContent holds a typed output chunk from a tool execution.
type ToolContent struct {
	Type string `json:"type"` // "text", "image", "resource"
	Text string `json:"text,omitempty"`
}

// Client interacts with an external MCP server according to the Model Context Protocol.
type Client struct {
	cfg        ServerConfig
	httpClient *http.Client
	toolsMu    sync.RWMutex
	tools      []ToolDefinition
}

// NewClient instantiates a client for the designated MCP server.
func NewClient(cfg ServerConfig, client ...*http.Client) *Client {
	hc := &http.Client{Timeout: 30 * time.Second}
	if len(client) > 0 && client[0] != nil {
		hc = client[0]
	}
	if cfg.Timeout > 0 {
		hc.Timeout = cfg.Timeout
	}
	if cfg.Transport == "" {
		cfg.Transport = TransportHTTP
	}
	return &Client{
		cfg:        cfg,
		httpClient: hc,
		tools:      make([]ToolDefinition, 0),
	}
}

// DiscoverTools queries the MCP server's tools endpoint to fetch all registered capabilities.
func (c *Client) DiscoverTools(ctx context.Context) ([]ToolDefinition, error) {
	endpoint := strings.TrimSuffix(c.cfg.BaseURL, "/") + "/mcp/tools"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare tool discovery request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tool discovery failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mcp server returned status %d", resp.StatusCode)
	}

	var payload struct {
		Tools []ToolDefinition `json:"tools"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode mcp tools: %w", err)
	}

	c.toolsMu.Lock()
	c.tools = payload.Tools
	c.toolsMu.Unlock()

	return payload.Tools, nil
}

// ExecuteTool invokes a specific capability on the external MCP server.
func (c *Client) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*ToolCallResponse, error) {
	if toolName == "" {
		return nil, errors.New("tool name is required")
	}

	endpoint := strings.TrimSuffix(c.cfg.BaseURL, "/") + "/mcp/tools/call"
	bodyBytes, err := json.Marshal(map[string]interface{}{
		"name":      toolName,
		"arguments": args,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode tool call arguments: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to prepare tool execution request: %w", err)
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcp tool execution request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mcp server returned error status %d", resp.StatusCode)
	}

	var callResp ToolCallResponse
	if err := json.NewDecoder(resp.Body).Decode(&callResp); err != nil {
		return nil, fmt.Errorf("failed to decode tool execution response: %w", err)
	}

	return &callResp, nil
}

// GetCachedTools returns previously discovered tools without a network roundtrip.
func (c *Client) GetCachedTools() []ToolDefinition {
	c.toolsMu.RLock()
	defer c.toolsMu.RUnlock()
	res := make([]ToolDefinition, len(c.tools))
	copy(res, c.tools)
	return res
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Cuan-MCP-Client/0.1.0")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}
}
