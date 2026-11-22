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
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id,omitempty"`
	Result  any           `json:"result,omitempty"`
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
	{Name: "get_github_connection", Description: "Read the saved Cuan GitHub connection through the Owner API or this session's exact task capability. Does not read host gh credentials, expose tokens, change sign-in or prove repository permission. No IDs or credentials are accepted.", InputSchema: map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}},
	{Name: "list_provider_accounts", Description: "Read connected AI account readiness, cooldown and fallback policy through the authenticated Cuan API; no credential contents or native paths.", InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}},
	{Name: "get_provider_account_usage", Description: "Read provider model capacity evidence for an owned account without changing routing policy.", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"account_id": map[string]any{"type": "string"}}, "required": []string{"account_id"}}},
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
		Description: "Run static typechecking (tsc --noEmit or go vet) bounded to 10s timeout. Only status=passed and success=true mean verification passed; not_run and timed_out are not passing checks.",
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
	{
		Name:        "get_project_environment",
		Description: "Read project environment from authenticated production settings. Values are redacted by default; reveal=true explicitly discloses secrets.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{
			"project_id": map[string]any{"type": "string"},
			"reveal":     map[string]any{"type": "boolean", "default": false},
		}, "required": []string{"project_id"}},
	},
	{
		Name:        "update_project_environment",
		Description: "Merge project environment values or unset selected keys using owner-authenticated settings API with conflict protection. Prefer from_env to import named process variables without including secret values in tool arguments. Output always redacted. Existing tasks retain their snapshot.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{
			"project_id": map[string]any{"type": "string"},
			"values":     map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
			"from_env":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"unset":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		}, "required": []string{"project_id"}},
	},
	{
		Name:        "list_projects",
		Description: "List all projects managed by the Cuan platform, including project ID, name, repo URL, default branch, and workspace path.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of projects to return (default: 20)",
				},
			},
		},
	},
	{
		Name:        "get_project",
		Description: "Get details of a specific project by project ID or name, including repo URL, branch, local path, and active tasks count.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"project_id": map[string]any{
					"type":        "string",
					"description": "Project ID or project name (e.g. 'dojo' or UUID)",
				},
			},
			"required": []string{"project_id"},
		},
	},
	{
		Name:        "list_tasks",
		Description: "List tasks across all projects or filtered by project ID, with optional status filter (backlog, doing, review, done, blocked).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"project_id": map[string]any{
					"type":        "string",
					"description": "Filter by project ID or name (optional)",
				},
				"status": map[string]any{
					"type":        "string",
					"description": "Filter by status: backlog, doing, review, done, blocked (optional)",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of tasks (default: 20, max: 100)",
				},
			},
		},
	},
	{
		Name:        "get_task",
		Description: "Get detailed task specification, status, priority, Definition of Done checklist, attempt history, and latest execution logs.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{
					"type":        "string",
					"description": "Task ID",
				},
			},
			"required": []string{"task_id"},
		},
	},
	{
		Name:        "create_task_draft",
		Description: "Propose or create a new task draft with title, instructions/scope, target project ID, priority, and Definition of Done checklist.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"project_id": map[string]any{
					"type":        "string",
					"description": "Target project ID or name",
				},
				"title": map[string]any{
					"type":        "string",
					"description": "Task title",
				},
				"instructions": map[string]any{
					"type":        "string",
					"description": "Task instructions, scope, and execution details",
				},
				"priority": map[string]any{
					"type":        "string",
					"enum":        []string{"low", "normal", "high", "urgent"},
					"description": "Task priority (default: normal)",
				},
				"dod_checklist": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Definition of Done acceptance checklist",
				},
			},
			"required": []string{"project_id", "title", "instructions"},
		},
	},
	{
		Name:        "update_task_status",
		Description: "Update task status and execution notes (e.g. todo, in_progress, review, done, blocked, canceled).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{
					"type":        "string",
					"description": "Task ID",
				},
				"status": map[string]any{
					"type":        "string",
					"enum":        []string{"todo", "in_progress", "doing", "review", "done", "blocked", "canceled", "backlog"},
					"description": "Target status",
				},
				"summary": map[string]any{
					"type":        "string",
					"description": "Summary of work done or planned",
				},
				"complete_note": map[string]any{
					"type":        "string",
					"description": "Verification notes, test evidence, or blocker explanation",
				},
			},
			"required": []string{"task_id", "status"},
		},
	},
	{
		Name:        "list_inbox_conversations",
		Description: "List customer chat conversations and project rooms in Cuan Inbox, including room ID, project ID, title, and topic.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"project_id": map[string]any{
					"type":        "string",
					"description": "Optional project ID filter",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum conversations to return (default: 20)",
				},
			},
		},
	},
	{
		Name:        "get_inbox_messages",
		Description: "Get message history for a specific inbox room or visitor session UUID.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"room_id": map[string]any{
					"type":        "string",
					"description": "Room ID or visitor session UUID",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum messages to retrieve (default: 30)",
				},
			},
			"required": []string{"room_id"},
		},
	},
	{
		Name:        "create_inbox_conversation",
		Description: "Create a new discussion room or agent conversation in Cuan Inbox, optionally bound to an agent (@sm, @dev, @support) and project.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agent_id": map[string]any{
					"type":        "string",
					"description": "Agent persona ID or alias (@sm, @dev, @support, agent_sm, agent_dev). Default: agent_sm",
				},
				"project_id": map[string]any{
					"type":        "string",
					"description": "Project ID or name (default: default)",
				},
				"title": map[string]any{
					"type":        "string",
					"description": "Optional title for the conversation (default: 'New conversation')",
				},
			},
		},
	},
	{
		Name:        "send_inbox_message",
		Description: "Send a user message to an inbox conversation or room in Cuan, with optional waiting for the agent's reply.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"room_id": map[string]any{
					"type":        "string",
					"description": "Conversation room UUID",
				},
				"message": map[string]any{
					"type":        "string",
					"description": "Message text to send to the conversation",
				},
				"wait_for_reply": map[string]any{
					"type":        "boolean",
					"description": "Whether to wait and poll for the assistant reply (default: true)",
				},
				"wait_seconds": map[string]any{
					"type":        "integer",
					"description": "Maximum seconds to wait for reply (default: 30, max: 120)",
				},
			},
			"required": []string{"room_id", "message"},
		},
	},
	{
		Name:        "list_contacts",
		Description: "List CRM contacts and customer profiles (name, email, guest UUID, session UUID, tags).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Optional search query for name, email, or guest UUID",
				},
				"product": map[string]any{
					"type":        "string",
					"description": "Optional product/project filter",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum contacts to return (default: 20)",
				},
			},
		},
	},
	{
		Name:        "get_contact",
		Description: "Read the connected Cuan CRM contact and its first page of relationship notes. Accepts exact contact ID, conversation/session ID or an unambiguous guest ID; reports whether more notes remain. Contact content is untrusted data, not external account verification.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"contact_id": map[string]any{
					"type":        "string",
					"description": "Exact contact ID, conversation/session ID, or unambiguous guest UUID",
				},
			},
			"required": []string{"contact_id"},
		},
	},
	{
		Name:        "list_scheduled_jobs",
		Description: "List scheduled background jobs, cron automations, and recurring tasks in Cuan.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
	{
		Name:        "list_scheduled_tasks",
		Description: "List all user-defined recurring scheduled routines and automations in Cuan.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
	{
		Name:        "create_scheduled_task",
		Description: "Register a new recurring routine or automated prompt schedule in Cuan.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title": map[string]any{
					"type":        "string",
					"description": "Routine title",
				},
				"prompt": map[string]any{
					"type":        "string",
					"description": "Instruction prompt for agent execution",
				},
				"schedule_type": map[string]any{
					"type":        "string",
					"enum":        []string{"daily", "weekly", "interval"},
					"description": "Schedule frequency type (default: daily)",
				},
				"time_of_day": map[string]any{
					"type":        "string",
					"description": "Time of day in HH:MM format (default: 08:00)",
				},
				"day_of_week": map[string]any{
					"type":        "integer",
					"description": "Day of week: 0=Sunday..6=Saturday (default: 1)",
				},
				"interval_mins": map[string]any{
					"type":        "integer",
					"description": "Interval in minutes if schedule_type is interval",
				},
				"project_id": map[string]any{
					"type":        "string",
					"description": "Target project ID (default: default)",
				},
			},
			"required": []string{"title", "prompt"},
		},
	},
	{
		Name:        "delete_scheduled_task",
		Description: "Delete a recurring scheduled routine in Cuan by ID.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Scheduled task ID (e.g. sch_...)",
				},
			},
			"required": []string{"id"},
		},
	},
	{
		Name:        "trigger_scheduled_task",
		Description: "Trigger an immediate run of a recurring scheduled routine in Cuan.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Scheduled task ID to trigger",
				},
			},
			"required": []string{"id"},
		},
	},
	{
		Name:        "get_system_settings",
		Description: "Get platform overview: server version, license, registered agent accounts, telemetry, database connection, and environment.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
	{
		Name:        "inspect_database_query",
		Description: "Execute a strictly read-only SQL query (SELECT, EXPLAIN, WITH) against the Cuan platform PostgreSQL database. All mutations (INSERT, UPDATE, DELETE, DROP, ALTER, TRUNCATE, etc.) are strictly forbidden. Results are bounded to limit (default: 50, max: 100) and sensitive columns (password, secret, token) are automatically masked.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "SQL query string (SELECT, EXPLAIN, WITH)",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum rows to return (default: 50, max: 100)",
				},
			},
			"required": []string{"query"},
		},
	},
	{
		Name:        "inspect_database_schema",
		Description: "Inspect Cuan database schema: list all tables in the public schema with estimated row counts, or inspect column names, data types, nullability, and defaults for a specific table.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"table_name": map[string]any{
					"type":        "string",
					"description": "Optional table name. If omitted, lists all tables in the public schema.",
				},
			},
		},
	},
	{
		Name:        "inspect_app_logs",
		Description: "Inspect recent server and agent application logs (bounded to 10-100 lines) with optional keyword filtering (e.g. 'error', 'failed', 'timeout', 'panic').",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"lines": map[string]any{
					"type":        "integer",
					"description": "Number of log lines to inspect (default: 50, max: 100)",
				},
				"filter": map[string]any{
					"type":        "string",
					"description": "Optional case-insensitive substring filter",
				},
			},
		},
	},
	{
		Name:        "list_apps",
		Description: "List all connected and available external third-party Apps in Cuan ecosystem.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
	{
		Name:        "list_skills",
		Description: "List all modular intelligence skills in Cuan with their enabled status and bound agents.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
	{
		Name:        "toggle_agent_skill",
		Description: "Enable or disable a specific modular skill for a designated agent persona.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agent_id": map[string]any{"type": "string", "description": "Agent ID (e.g. agent_sm, agent_dev, agent_support)"},
				"skill_id": map[string]any{"type": "string", "description": "Skill ID (e.g. vision_receipt_extractor, google_sheets_sync)"},
				"enabled":  map[string]any{"type": "boolean", "description": "True to activate, false to deactivate"},
			},
			"required": []string{"agent_id", "skill_id", "enabled"},
		},
	},
	{
		Name:        "probe_endpoint_health",
		Description: "Probe an HTTP/HTTPS health endpoint (e.g. https://app.cuan.work/healthz or http://localhost:8080/healthz), measuring latency, HTTP status, and parsing version/commit_sha metadata in a strictly read-only manner.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "HTTP or HTTPS target URL to probe (e.g. 'https://app.cuan.work/healthz')",
				},
				"timeout_seconds": map[string]any{
					"type":        "integer",
					"description": "Probe timeout in seconds (default: 5, max: 15)",
				},
			},
			"required": []string{"url"},
		},
	},
	{
		Name:        "browser_fetch",
		Description: "Read text from a public HTTP(S) page using an anonymous, temporary JavaScript browser when plain HTTP is insufficient. No login, custom scripts, shell, browser flags or account cookies. URL is the requested address, not a verified final redirect address. HTTP status is not inferred from rendered text.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{"url": map[string]any{"type": "string"}}, "required": []string{"url"}, "additionalProperties": false},
	},
	{
		Name:        "http_request",
		Description: "Inspect a public HTTP(S) endpoint with anonymous GET or HEAD. Returns real status, selected headers and up to 64 KiB of raw text/JSON without starting a browser. No shell, custom headers, credentials or mutations.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{"url": map[string]any{"type": "string"}, "method": map[string]any{"type": "string", "enum": []string{"GET", "HEAD"}}}, "required": []string{"url"}, "additionalProperties": false},
	},
	{
		Name:        "web_fetch",
		Description: "Fetch and extract clean readable text content and title from a public web page URL (cleans ads, navigation, headers, scripts).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "Public HTTP or HTTPS URL of the web page or article to fetch",
				},
			},
			"required": []string{"url"},
		},
	},
	{
		Name:        "inspect_deployment_status",
		Description: "Inspect live deployment status, release directory metadata, systemd service status, and compare live running commit SHA against local repository HEAD to identify pending unmerged commits.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"project": map[string]any{
					"type":        "string",
					"description": "Target project name (e.g. 'cuan', 'dojo', default: 'cuan')",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Target local repository directory (default: current working directory)",
				},
				"health_url": map[string]any{
					"type":        "string",
					"description": "Optional live health endpoint URL (default: 'https://app.cuan.work/healthz' for cuan)",
				},
			},
		},
	},
}

// RunMCP implements a Stdio MCP (Model Context Protocol) JSON-RPC 2.0 server.
func RunMCP(in io.Reader, out io.Writer, errOut io.Writer) int {
	return runMCP(in, out, errOut, handleMCPRequest)
}

// RunPublicWebMCP exposes only public web retrieval to isolated chat providers.
// Calls are filtered as well as discovery; hidden mutation names stay rejected.
func RunPublicWebMCP(in io.Reader, out io.Writer, errOut io.Writer) int {
	return runMCP(in, out, errOut, handlePublicWebMCPRequest)
}

func handlePublicWebMCPRequest(req jsonRPCRequest, out io.Writer) {
	if req.Method == "tools/list" {
		tools := []mcpToolDefinition{}
		for _, tool := range nativeMCPTools {
			if publicChatTool(tool.Name) {
				tools = append(tools, tool)
			}
		}
		sendResponse(out, jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": tools}})
		return
	}
	if req.Method == "tools/call" {
		var params struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(req.Params, &params) != nil || !publicChatTool(params.Name) {
			sendResponse(out, jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32601, Message: "Tool unavailable in public web chat mode"}})
			return
		}
	}
	handleMCPRequest(req, out)
}

func runMCP(in io.Reader, out io.Writer, errOut io.Writer, handler func(jsonRPCRequest, io.Writer)) int {
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

		// In JSON-RPC 2.0 and MCP, requests without an ID are notifications.
		// The server MUST NOT send any response to a notification.
		if req.ID == nil {
			continue
		}

		handler(req, out)
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
				"name":    "cuan-platform-mcp",
				"version": "1.1.0",
			},
		}

	case "ping":
		resp.Result = map[string]any{}

	case "tools/list":
		tools := nativeMCPTools
		if boundTaskSession() {
			tools = make([]mcpToolDefinition, 0, len(nativeMCPTools))
			for _, tool := range nativeMCPTools {
				switch tool.Name {
				case "send_inbox_message", "create_inbox_conversation", "create_task_draft", "update_task_status", "toggle_agent_skill", "update_project_environment", "create_scheduled_task", "delete_scheduled_task", "trigger_scheduled_task":
					continue
				}
				tools = append(tools, tool)
			}
			tools = append(tools, taskSessionMCPTools...)
		}
		resp.Result = map[string]any{"tools": tools}

	case "resources/list":
		resp.Result = map[string]any{
			"resources": []any{},
		}

	case "prompts/list":
		resp.Result = map[string]any{
			"prompts": []any{},
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
		case "get_current_task", "prepare_current_task_report", "report_current_task", "push_current_task":
			toolResult, toolErr = callTaskSessionTool(callParams.Name, callParams.Arguments)
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
		case "get_project_environment":
			projectID, _ := callParams.Arguments["project_id"].(string)
			reveal, _ := callParams.Arguments["reveal"].(bool)
			toolResult, toolErr = GetCuanClient().GetProjectEnvironment(ctx, projectID, reveal)
		case "update_project_environment":
			toolResult, toolErr = updateEnvironmentMCP(ctx, callParams.Arguments)
		case "list_provider_accounts":
			toolResult, toolErr = GetCuanClient().ListProviderAccounts(ctx)
		case "get_github_connection":
			if len(callParams.Arguments) != 0 {
				toolErr = fmt.Errorf("GitHub connection inspection accepts no IDs or credentials")
			} else {
				toolResult, toolErr = GetCuanClient().GitHubConnection(ctx)
			}
		case "get_provider_account_usage":
			id, _ := callParams.Arguments["account_id"].(string)
			toolResult, toolErr = GetCuanClient().GetProviderAccountUsage(ctx, id)
		case "list_projects":
			limit := 20
			if rawLimit, ok := callParams.Arguments["limit"].(float64); ok {
				limit = int(rawLimit)
			}
			toolResult, toolErr = ListProjects(ctx, limit)
		case "get_project":
			projectID, _ := callParams.Arguments["project_id"].(string)
			toolResult, toolErr = GetProject(ctx, projectID)
		case "list_tasks":
			projectID, _ := callParams.Arguments["project_id"].(string)
			status, _ := callParams.Arguments["status"].(string)
			limit := 20
			if rawLimit, ok := callParams.Arguments["limit"].(float64); ok {
				limit = int(rawLimit)
			}
			toolResult, toolErr = ListTasks(ctx, projectID, status, limit)
		case "get_task":
			taskID, _ := callParams.Arguments["task_id"].(string)
			toolResult, toolErr = GetTask(ctx, taskID)
		case "update_task_status":
			taskID, _ := callParams.Arguments["task_id"].(string)
			status, _ := callParams.Arguments["status"].(string)
			summary, _ := callParams.Arguments["summary"].(string)
			completeNote, _ := callParams.Arguments["complete_note"].(string)
			toolResult, toolErr = UpdateTaskStatus(ctx, taskID, status, summary, completeNote)
		case "create_task_draft":
			projectID, _ := callParams.Arguments["project_id"].(string)
			title, _ := callParams.Arguments["title"].(string)
			instructions, _ := callParams.Arguments["instructions"].(string)
			priority, _ := callParams.Arguments["priority"].(string)
			var dodList []string
			if rawDoD, ok := callParams.Arguments["dod_checklist"].([]any); ok {
				for _, item := range rawDoD {
					if str, ok := item.(string); ok {
						dodList = append(dodList, str)
					}
				}
			}
			toolResult, toolErr = CreateTaskDraft(ctx, projectID, title, instructions, priority, dodList)
		case "list_inbox_conversations":
			projectID, _ := callParams.Arguments["project_id"].(string)
			limit := 20
			if rawLimit, ok := callParams.Arguments["limit"].(float64); ok {
				limit = int(rawLimit)
			}
			toolResult, toolErr = ListInboxConversations(ctx, projectID, limit)
		case "get_inbox_messages":
			roomID, _ := callParams.Arguments["room_id"].(string)
			limit := 30
			if rawLimit, ok := callParams.Arguments["limit"].(float64); ok {
				limit = int(rawLimit)
			}
			toolResult, toolErr = GetInboxMessages(ctx, roomID, limit)
		case "create_inbox_conversation":
			agentID, _ := callParams.Arguments["agent_id"].(string)
			projectID, _ := callParams.Arguments["project_id"].(string)
			title, _ := callParams.Arguments["title"].(string)
			toolResult, toolErr = CreateInboxConversation(ctx, agentID, projectID, title)
		case "send_inbox_message":
			roomID, _ := callParams.Arguments["room_id"].(string)
			message, _ := callParams.Arguments["message"].(string)
			waitForReply := true
			if rawWait, ok := callParams.Arguments["wait_for_reply"].(bool); ok {
				waitForReply = rawWait
			}
			waitSeconds := 30
			if rawSecs, ok := callParams.Arguments["wait_seconds"].(float64); ok {
				waitSeconds = int(rawSecs)
			}
			toolResult, toolErr = SendInboxMessage(ctx, roomID, message, waitForReply, waitSeconds)
		case "list_contacts":
			query, _ := callParams.Arguments["query"].(string)
			product, _ := callParams.Arguments["product"].(string)
			limit := 20
			if rawLimit, ok := callParams.Arguments["limit"].(float64); ok {
				limit = int(rawLimit)
			}
			toolResult, toolErr = ListContacts(ctx, query, product, limit)
		case "get_contact":
			contactID, _ := callParams.Arguments["contact_id"].(string)
			toolResult, toolErr = GetContact(ctx, contactID)
		case "list_scheduled_jobs":
			toolResult, toolErr = ListScheduledJobs(ctx)
		case "list_scheduled_tasks":
			client := GetCuanClient()
			if client == nil || !client.IsConfigured() {
				toolErr = fmt.Errorf("Cuan connection not configured")
			} else {
				toolResult, toolErr = client.ListScheduledTasks(ctx)
			}
		case "create_scheduled_task":
			client := GetCuanClient()
			if client == nil || !client.IsConfigured() {
				toolErr = fmt.Errorf("Cuan connection not configured")
			} else {
				toolResult, toolErr = client.CreateScheduledTask(ctx, callParams.Arguments)
			}
		case "delete_scheduled_task":
			client := GetCuanClient()
			if client == nil || !client.IsConfigured() {
				toolErr = fmt.Errorf("Cuan connection not configured")
			} else {
				id, _ := callParams.Arguments["id"].(string)
				toolErr = client.DeleteScheduledTask(ctx, id)
				if toolErr == nil {
					toolResult = map[string]any{"status": "deleted", "id": id}
				}
			}
		case "trigger_scheduled_task":
			client := GetCuanClient()
			if client == nil || !client.IsConfigured() {
				toolErr = fmt.Errorf("Cuan connection not configured")
			} else {
				id, _ := callParams.Arguments["id"].(string)
				toolErr = client.TriggerScheduledTask(ctx, id)
				if toolErr == nil {
					toolResult = map[string]any{"status": "triggered", "id": id}
				}
			}
		case "get_system_settings":
			toolResult, toolErr = GetSystemSettings(ctx)
		case "inspect_database_query":
			query, _ := callParams.Arguments["query"].(string)
			limit := 50
			if rawLimit, ok := callParams.Arguments["limit"].(float64); ok {
				limit = int(rawLimit)
			}
			toolResult, toolErr = InspectDatabaseQuery(ctx, query, limit)
		case "inspect_database_schema":
			tableName, _ := callParams.Arguments["table_name"].(string)
			toolResult, toolErr = InspectDatabaseSchema(ctx, tableName)
		case "inspect_app_logs":
			lines := 50
			if rawLines, ok := callParams.Arguments["lines"].(float64); ok {
				lines = int(rawLines)
			}
			filter, _ := callParams.Arguments["filter"].(string)
			toolResult, toolErr = InspectAppLogs(ctx, lines, filter)
		case "list_apps":
			toolResult, toolErr = ListApps(ctx)
		case "list_skills":
			toolResult, toolErr = ListSkills(ctx)
		case "toggle_agent_skill":
			agentID, _ := callParams.Arguments["agent_id"].(string)
			skillID, _ := callParams.Arguments["skill_id"].(string)
			enabled, _ := callParams.Arguments["enabled"].(bool)
			toolResult, toolErr = ToggleAgentSkill(ctx, agentID, skillID, enabled)
		case "probe_endpoint_health":
			rawURL, _ := callParams.Arguments["url"].(string)
			timeout := 5
			if rawTimeout, ok := callParams.Arguments["timeout_seconds"].(float64); ok {
				timeout = int(rawTimeout)
			}
			toolResult, toolErr = ProbeEndpointHealth(ctx, rawURL, timeout)
		case "browser_fetch":
			if len(callParams.Arguments) != 1 {
				toolErr = fmt.Errorf("browser_fetch accepts only url")
				break
			}
			rawURL, ok := callParams.Arguments["url"].(string)
			if !ok {
				toolErr = fmt.Errorf("browser_fetch requires a URL string")
				break
			}
			toolResult, toolErr = BrowserFetch(ctx, rawURL)
		case "http_request":
			rawURL, _ := callParams.Arguments["url"].(string)
			method, _ := callParams.Arguments["method"].(string)
			toolResult, toolErr = PublicHTTPRequest(ctx, method, rawURL)
		case "web_fetch":
			rawURL, _ := callParams.Arguments["url"].(string)
			toolResult, toolErr = WebFetch(ctx, rawURL)
		case "inspect_deployment_status":
			project, _ := callParams.Arguments["project"].(string)
			healthURL, _ := callParams.Arguments["health_url"].(string)
			toolResult, toolErr = InspectDeploymentStatus(ctx, project, pathVal, healthURL)
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
				"isError": !diagnosticPassed(toolResult),
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
