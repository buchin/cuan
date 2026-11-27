package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	mcpProtocolVersion = "2025-03-26"
	mcpMaxListPages    = 50
	mcpMaxErrBodyBytes = 1024
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

type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      *int64      `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type httpStatusError struct {
	StatusCode int
	Body       string
}

func (e *httpStatusError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("mcp server returned status %d: %s", e.StatusCode, e.Body)
	}
	return fmt.Sprintf("mcp server returned status %d", e.StatusCode)
}

// Client interacts with an external MCP server according to the Model Context Protocol.
type Client struct {
	cfg        ServerConfig
	httpClient *http.Client
	toolsMu    sync.RWMutex
	tools      []ToolDefinition

	sessionMu   sync.Mutex
	sessionID   string
	initialized bool
	legacyREST  bool
	nextReqID   atomic.Int64
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

// DiscoverTools queries the MCP server to fetch all registered capabilities.
// It uses standard MCP Streamable HTTP (initialize + notifications/initialized + tools/list)
// and safely falls back to legacy REST (/mcp/tools) only when the configured BaseURL
// does not already point to an explicit /mcp endpoint.
func (c *Client) DiscoverTools(ctx context.Context) ([]ToolDefinition, error) {
	c.sessionMu.Lock()
	useLegacy := c.legacyREST
	c.sessionMu.Unlock()

	if useLegacy {
		return c.discoverToolsLegacy(ctx)
	}

	tools, err := c.discoverToolsStreamable(ctx)
	if err != nil {
		if c.shouldFallbackToLegacy(err) {
			legacyTools, legacyErr := c.discoverToolsLegacy(ctx)
			if legacyErr == nil {
				c.sessionMu.Lock()
				c.legacyREST = true
				c.sessionMu.Unlock()
				return legacyTools, nil
			}
		}
		return nil, err
	}

	return tools, nil
}

// ExecuteTool invokes a specific capability on the external MCP server.
func (c *Client) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*ToolCallResponse, error) {
	if toolName == "" {
		return nil, errors.New("tool name is required")
	}

	c.sessionMu.Lock()
	useLegacy := c.legacyREST
	c.sessionMu.Unlock()

	if useLegacy {
		return c.executeToolLegacy(ctx, toolName, args)
	}

	resp, err := c.executeToolStreamable(ctx, toolName, args)
	if err != nil {
		if c.shouldFallbackToLegacy(err) {
			legacyResp, legacyErr := c.executeToolLegacy(ctx, toolName, args)
			if legacyErr == nil {
				c.sessionMu.Lock()
				c.legacyREST = true
				c.sessionMu.Unlock()
				return legacyResp, nil
			}
		}
		return nil, err
	}

	return resp, nil
}

// GetCachedTools returns previously discovered tools without a network roundtrip.
func (c *Client) GetCachedTools() []ToolDefinition {
	c.toolsMu.RLock()
	defer c.toolsMu.RUnlock()
	res := make([]ToolDefinition, len(c.tools))
	copy(res, c.tools)
	return res
}

func (c *Client) discoverToolsStreamable(ctx context.Context) ([]ToolDefinition, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, err
	}

	var allTools []ToolDefinition
	var cursor string

	for page := 0; page < mcpMaxListPages; page++ {
		var params interface{}
		if cursor != "" {
			params = map[string]string{"cursor": cursor}
		} else {
			params = map[string]interface{}{}
		}

		rpcResp, err := c.callJSONRPC(ctx, "tools/list", params)
		if err != nil {
			return nil, fmt.Errorf("tool discovery failed: %w", err)
		}

		var result struct {
			Tools []struct {
				Name              string                 `json:"name"`
				Description       string                 `json:"description"`
				InputSchemaCamel  map[string]interface{} `json:"inputSchema,omitempty"`
				InputSchemaSnake  map[string]interface{} `json:"input_schema,omitempty"`
			} `json:"tools"`
			NextCursor string `json:"nextCursor,omitempty"`
		}
		if err := json.Unmarshal(rpcResp.Result, &result); err != nil {
			return nil, fmt.Errorf("failed to decode mcp tools: %w", err)
		}

		for _, t := range result.Tools {
			schema := t.InputSchemaCamel
			if schema == nil {
				schema = t.InputSchemaSnake
			}
			allTools = append(allTools, ToolDefinition{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: schema,
			})
		}

		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}

	if allTools == nil {
		allTools = make([]ToolDefinition, 0)
	}

	c.toolsMu.Lock()
	c.tools = allTools
	c.toolsMu.Unlock()

	return allTools, nil
}

func (c *Client) executeToolStreamable(ctx context.Context, toolName string, args map[string]interface{}) (*ToolCallResponse, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, err
	}

	if args == nil {
		args = map[string]interface{}{}
	}

	params := map[string]interface{}{
		"name":      toolName,
		"arguments": args,
	}

	c.sessionMu.Lock()
	hadSession := c.sessionID != ""
	c.sessionMu.Unlock()

	rpcResp, err := c.callJSONRPC(ctx, "tools/call", params)
	if err != nil && hadSession {
		// If an active session expired (HTTP 404), re-initialize once and retry.
		var statusErr *httpStatusError
		if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusNotFound {
			c.resetSession()
			if initErr := c.ensureInitialized(ctx); initErr == nil {
				rpcResp, err = c.callJSONRPC(ctx, "tools/call", params)
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("mcp tool execution request failed: %w", err)
	}

	var rawResult struct {
		Content      []ToolContent `json:"content"`
		IsErrorCamel bool          `json:"isError"`
		IsErrorSnake bool          `json:"is_error"`
	}
	if err := json.Unmarshal(rpcResp.Result, &rawResult); err != nil {
		return nil, fmt.Errorf("failed to decode tool execution response: %w", err)
	}

	return &ToolCallResponse{
		Content: rawResult.Content,
		IsError: rawResult.IsErrorCamel || rawResult.IsErrorSnake,
	}, nil
}

func (c *Client) ensureInitialized(ctx context.Context) error {
	c.sessionMu.Lock()
	if c.initialized {
		c.sessionMu.Unlock()
		return nil
	}
	c.sessionMu.Unlock()

	initParams := map[string]interface{}{
		"protocolVersion": mcpProtocolVersion,
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]string{
			"name":    "cuan-mcp-client",
			"version": "0.1.0",
		},
	}

	if _, err := c.callJSONRPC(ctx, "initialize", initParams); err != nil {
		return fmt.Errorf("mcp initialize failed: %w", err)
	}

	// Send notifications/initialized (best-effort for minimal servers that may return 202/200/204).
	if err := c.sendNotification(ctx, "notifications/initialized", nil); err != nil {
		var statusErr *httpStatusError
		if !errors.As(err, &statusErr) || (statusErr.StatusCode != http.StatusNotFound && statusErr.StatusCode != http.StatusMethodNotAllowed) {
			return fmt.Errorf("mcp initialized notification failed: %w", err)
		}
	}

	c.sessionMu.Lock()
	c.initialized = true
	c.sessionMu.Unlock()
	return nil
}

func (c *Client) resetSession() {
	c.sessionMu.Lock()
	c.sessionID = ""
	c.initialized = false
	c.sessionMu.Unlock()
}

func (c *Client) callJSONRPC(ctx context.Context, method string, params interface{}) (*jsonRPCResponse, error) {
	reqID := c.nextReqID.Add(1)
	payload := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      &reqID,
		Method:  method,
		Params:  params,
	}

	resp, err := c.doJSONRPCRequest(ctx, payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if sid := strings.TrimSpace(resp.Header.Get("Mcp-Session-Id")); sid != "" {
		c.sessionMu.Lock()
		c.sessionID = sid
		c.sessionMu.Unlock()
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodySnippet := readBodySnippet(resp.Body)
		return nil, &httpStatusError{StatusCode: resp.StatusCode, Body: bodySnippet}
	}

	var rpcResp *jsonRPCResponse
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "text/event-stream") {
		rpcResp, err = parseSSEResponse(resp.Body, reqID)
	} else {
		rpcResp, err = parseJSONResponse(resp.Body)
	}
	if err != nil {
		return nil, err
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("mcp rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp, nil
}

func (c *Client) sendNotification(ctx context.Context, method string, params interface{}) error {
	payload := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	resp, err := c.doJSONRPCRequest(ctx, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if sid := strings.TrimSpace(resp.Header.Get("Mcp-Session-Id")); sid != "" {
		c.sessionMu.Lock()
		c.sessionID = sid
		c.sessionMu.Unlock()
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &httpStatusError{StatusCode: resp.StatusCode, Body: readBodySnippet(resp.Body)}
	}
	return nil
}

func (c *Client) doJSONRPCRequest(ctx context.Context, payload jsonRPCRequest) (*http.Response, error) {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode json-rpc request: %w", err)
	}

	endpoint := c.streamableEndpoint()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to prepare mcp request: %w", err)
	}

	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	c.sessionMu.Lock()
	sid := c.sessionID
	c.sessionMu.Unlock()
	if sid != "" {
		req.Header.Set("Mcp-Session-Id", sid)
	}

	return c.httpClient.Do(req)
}

func parseJSONResponse(r io.Reader) (*jsonRPCResponse, error) {
	var rpcResp jsonRPCResponse
	if err := json.NewDecoder(r).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("failed to decode mcp json-rpc response: %w", err)
	}
	return &rpcResp, nil
}

func parseSSEResponse(r io.Reader, expectedID int64) (*jsonRPCResponse, error) {
	scanner := bufio.NewScanner(r)
	// Allow up to 4MB per SSE line for large tool schemas or tool outputs.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var dataLines []string
	var lastResp *jsonRPCResponse

	flushEvent := func() (*jsonRPCResponse, error) {
		if len(dataLines) == 0 {
			return nil, nil
		}
		raw := strings.TrimSpace(strings.Join(dataLines, "\n"))
		dataLines = nil
		if raw == "" || raw == "[DONE]" {
			return nil, nil
		}
		var rpcResp jsonRPCResponse
		if err := json.Unmarshal([]byte(raw), &rpcResp); err != nil {
			return nil, fmt.Errorf("failed to decode mcp sse event: %w", err)
		}
		lastResp = &rpcResp
		if matchesRPCID(rpcResp.ID, expectedID) {
			return &rpcResp, nil
		}
		return nil, nil
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			resp, err := flushEvent()
			if err != nil {
				return nil, err
			}
			if resp != nil {
				return resp, nil
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimPrefix(data, " ")
			dataLines = append(dataLines, data)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read mcp sse stream: %w", err)
	}

	if resp, err := flushEvent(); err != nil {
		return nil, err
	} else if resp != nil {
		return resp, nil
	}

	if lastResp != nil {
		return lastResp, nil
	}
	return nil, errors.New("mcp sse stream ended without json-rpc response")
}

func matchesRPCID(rawID json.RawMessage, expectedID int64) bool {
	trimmed := strings.TrimSpace(string(rawID))
	if trimmed == "" || trimmed == "null" {
		return false
	}
	if idNum, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return idNum == expectedID
	}
	var idStr string
	if err := json.Unmarshal(rawID, &idStr); err == nil {
		return idStr == strconv.FormatInt(expectedID, 10)
	}
	return false
}

func (c *Client) streamableEndpoint() string {
	return strings.TrimRight(strings.TrimSpace(c.cfg.BaseURL), "/")
}

func (c *Client) isExplicitMCPEndpoint() bool {
	endpoint := c.streamableEndpoint()
	if parsed, err := url.Parse(endpoint); err == nil {
		p := strings.TrimRight(parsed.Path, "/")
		return strings.HasSuffix(p, "/mcp")
	}
	return strings.HasSuffix(endpoint, "/mcp")
}

func (c *Client) shouldFallbackToLegacy(err error) bool {
	if c.isExplicitMCPEndpoint() {
		return false
	}
	var statusErr *httpStatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode == http.StatusNotFound || statusErr.StatusCode == http.StatusMethodNotAllowed
	}
	return false
}

func (c *Client) discoverToolsLegacy(ctx context.Context) ([]ToolDefinition, error) {
	endpoint := c.streamableEndpoint() + "/mcp/tools"
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

func (c *Client) executeToolLegacy(ctx context.Context, toolName string, args map[string]interface{}) (*ToolCallResponse, error) {
	endpoint := c.streamableEndpoint() + "/mcp/tools/call"
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

func readBodySnippet(r io.Reader) string {
	data, err := io.ReadAll(io.LimitReader(r, mcpMaxErrBodyBytes))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Cuan-MCP-Client/0.1.0")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}
}
