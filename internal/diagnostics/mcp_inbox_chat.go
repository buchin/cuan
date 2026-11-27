package diagnostics

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// InboxChatSession holds per-turn, owner-authenticated and workspace-scoped
// context for running the full Cuan MCP server inside an isolated chat turn.
type InboxChatSession struct {
	OwnerID          string    `json:"owner_id"`
	RoomID           string    `json:"room_id"`
	RequestID        string    `json:"request_id"`
	ProjectID        string    `json:"project_id,omitempty"`
	Workspace        string    `json:"workspace,omitempty"`
	CurrentContactID string    `json:"current_contact_id,omitempty"`
	APIURL           string    `json:"api_url"`
	AuthToken        string    `json:"auth_token"`
	ObservedAt       time.Time `json:"observed_at"`
}

// RunInboxChatMCP runs the full Cuan MCP toolset pinned to a per-turn
// authenticated chat session without inheriting ambient credentials or host DB.
func RunInboxChatMCP(path string, in io.Reader, out, errOut io.Writer) int {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		fmt.Fprintln(errOut, "chat session unavailable")
		return 1
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(errOut, "chat session unavailable")
		return 1
	}
	var session InboxChatSession
	if json.Unmarshal(raw, &session) != nil ||
		strings.TrimSpace(session.OwnerID) == "" ||
		strings.TrimSpace(session.RoomID) == "" ||
		strings.TrimSpace(session.RequestID) == "" ||
		strings.TrimSpace(session.APIURL) == "" ||
		strings.TrimSpace(session.AuthToken) == "" ||
		session.ObservedAt.Before(time.Now().Add(-20*time.Minute)) ||
		session.ObservedAt.After(time.Now().Add(5*time.Second)) {
		fmt.Fprintln(errOut, "chat session unavailable or expired")
		return 1
	}

	// Pin client to this turn's scoped token and disable ambient DB/env fallback.
	DisablePlatformDB()
	for _, key := range []string{"CUAN_AUTH_TOKEN", "CUAN_API_TOKEN", "CUAN_API_URL", "DATABASE_URL"} {
		_ = os.Unsetenv(key)
	}
	SetCuanClientConfig(session.APIURL, session.AuthToken)

	return runMCP(in, out, errOut, func(req jsonRPCRequest, out io.Writer) {
		session.handle(req, out)
	})
}

func (session InboxChatSession) handle(req jsonRPCRequest, out io.Writer) {
	if req.Method != "tools/call" {
		handleMCPRequest(req, out)
		return
	}

	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		sendResponse(out, jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonRPCError{Code: -32602, Message: "Invalid params: " + err.Error()},
		})
		return
	}
	if params.Arguments == nil {
		params.Arguments = map[string]any{}
	}

	modified := false

	// Enforce workspace isolation for local filesystem/git/typecheck tools.
	switch params.Name {
	case "inspect_git_status", "inspect_git_log", "inspect_git_diff", "inspect_git_branches",
		"inspect_typecheck", "inspect_system_health", "inspect_deployment_status":
		rawPath, _ := params.Arguments["path"].(string)
		rawPath = strings.TrimSpace(rawPath)
		if session.Workspace != "" {
			if rawPath == "" {
				params.Arguments["path"] = session.Workspace
				modified = true
			} else if !isPathWithinWorkspace(rawPath, session.Workspace) {
				sendChatToolError(out, req.ID, "Error: path is outside the authorized workspace")
				return
			}
		} else if rawPath != "" && params.Name != "inspect_system_health" {
			sendChatToolError(out, req.ID, "Error: no project workspace is bound to this chat session")
			return
		}
	case "get_contact":
		contactID, _ := params.Arguments["contact_id"].(string)
		if strings.TrimSpace(contactID) == "current" {
			resolved := strings.TrimSpace(session.CurrentContactID)
			if resolved == "" {
				resolved = strings.TrimSpace(session.RoomID)
			}
			params.Arguments["contact_id"] = resolved
			modified = true
		}
	}

	if modified {
		updated, err := json.Marshal(params)
		if err == nil {
			req.Params = updated
		}
	}

	handleMCPRequest(req, out)
}

func sendChatToolError(out io.Writer, id any, message string) {
	sendResponse(out, jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]any{
			"isError": true,
			"content": []map[string]any{
				{"type": "text", "text": message},
			},
		},
	})
}

func isPathWithinWorkspace(candidate, workspace string) bool {
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(absWorkspace); err == nil {
		absWorkspace = resolved
	}
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(absCandidate); err == nil {
		absCandidate = resolved
	}
	rel, err := filepath.Rel(absWorkspace, absCandidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
