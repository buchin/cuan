package diagnostics

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      any         `json:"id,omitempty"`
	Result  any         `json:"result,omitempty"`
	Error   *jsonRPCError `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type mcpToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// Registered tools available over MCP
var nativeMCPTools = []mcpToolDefinition{
	{
		Name:        "inspect_git_status",
		Description: "Inspect current git working tree status, staged/unstaged changes, and branch tracking in a strictly read-only manner.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Target repository directory (default: current workspace)",
				},
			},
		},
	},
	{
		Name:        "inspect_git_log",
		Description: "Inspect recent git commit history (bounded to 1-100 commits).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Target repository directory",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Number of commits to retrieve (default: 10, max: 100)",
				},
			},
		},
	},
	{
		Name:        "inspect_git_diff",
		Description: "Inspect summary of uncommitted and staged changes (--stat) without full diff output.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Target repository directory",
				},
			},
		},
	},
	{
		Name:        "inspect_git_branches",
		Description: "Inspect local and remote git branches.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Target repository directory",
				},
			},
		},
	},
	{
		Name:        "inspect_system_health",
		Description: "Inspect host health diagnostics (disk free df, memory free free, load average, uptime).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Target directory or mount point (default: /)",
				},
			},
		},
	},
	{
		Name:        "inspect_typecheck",
		Description: "Run static typechecking (tsc --noEmit or go vet) bounded to 10s timeout.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Target project workspace directory",
				},
			},
		},
	},
}

// RunMCP implements a Stdio MCP (Model Context Protocol) JSON-RPC 2.0 server.
func RunMCP(in io.Reader, out io.Writer, errOut io.Writer) int {
	scanner := bufio.NewScanner(in)
	// Support large JSON-RPC messages up to 1MB
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			resp := jsonRPCResponse{
				JSONRPC: "2.0",
				Error: &jsonRPCError{
					Code:    -32700,
					Message: "Parse error: " + err.Error(),
				},
			}
			sendResponse(out, resp)
			continue
		}

		// Handle notifications (no response if id is nil)
		if req.ID == nil && req.Method == "notifications/initialized" {
			continue
		}

		handleMCPRequest(req, out)
	}

	return 0
}

func handleMCPRequest(req jsonRPCRequest, out io.Writer) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case "initialize":
		resp.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "cuan-inspection-tools",
				"version": "1.0.0",
			},
		}

	case "ping":
		resp.Result = map[string]any{}

	case "tools/list":
		resp.Result = map[string]any{
			"tools": nativeMCPTools,
		}

	case "tools/call":
		var callParams struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &callParams); err != nil {
			resp.Error = &jsonRPCError{Code: -32602, Message: "Invalid params: " + err.Error()}
			break
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		pathVal, _ := callParams.Arguments["path"].(string)
		var toolResult any
		var toolErr error

		switch callParams.Name {
		case "inspect_git_status":
			toolResult, toolErr = InspectGitStatus(ctx, pathVal)
		case "inspect_git_log":
			limit := 10
			if rawLimit, ok := callParams.Arguments["limit"].(float64); ok {
				limit = int(rawLimit)
			}
			toolResult, toolErr = InspectGitLog(ctx, pathVal, limit)
		case "inspect_git_diff":
			toolResult, toolErr = InspectGitDiffSummary(ctx, pathVal)
		case "inspect_git_branches":
			toolResult, toolErr = InspectGitBranches(ctx, pathVal)
		case "inspect_system_health":
			toolResult, toolErr = InspectSystemHealth(ctx, pathVal)
		case "inspect_typecheck":
			toolResult, toolErr = InspectTypecheck(ctx, pathVal)
		default:
			resp.Error = &jsonRPCError{Code: -32601, Message: fmt.Sprintf("Tool not found: %s", callParams.Name)}
			sendResponse(out, resp)
			return
		}

		if toolErr != nil {
			resp.Result = map[string]any{
				"content": []map[string]any{
					{
						"type": "text",
						"text": fmt.Sprintf("Error executing tool %s: %v", callParams.Name, toolErr),
					},
				},
				"isError": true,
			}
		} else {
			textBytes, _ := json.MarshalIndent(toolResult, "", "  ")
			resp.Result = map[string]any{
				"content": []map[string]any{
					{
						"type": "text",
						"text": string(textBytes),
					},
				},
				"isError": false,
			}
		}

	default:
		resp.Error = &jsonRPCError{
			Code:    -32601,
			Message: fmt.Sprintf("Method not found: %s", req.Method),
		}
	}

	sendResponse(out, resp)
}

func sendResponse(out io.Writer, resp jsonRPCResponse) {
	data, err := json.Marshal(resp)
	if err == nil {
		fmt.Fprintf(out, "%s\n", string(data))
	}
}
