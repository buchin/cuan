package diagnostics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
)

// CuanAPIConfig holds connection parameters for the Cuan HTTP API.
type CuanAPIConfig struct {
	BaseURL   string `json:"api_url"`
	AuthToken string `json:"auth_token"`
}

var (
	globalCuanClient *CuanClient
	cachedCuanConfig *CuanAPIConfig
)

// GetCuanConfig retrieves Cuan API configuration from env, ~/.cuan/config.json, or .env.
func GetCuanConfig() CuanAPIConfig {
	if cachedCuanConfig != nil {
		return *cachedCuanConfig
	}

	cfg := CuanAPIConfig{}

	// 1. Environment variables
	cfg.BaseURL = os.Getenv("CUAN_API_URL")
	cfg.AuthToken = os.Getenv("CUAN_AUTH_TOKEN")
	if cfg.AuthToken == "" {
		cfg.AuthToken = os.Getenv("CUAN_API_TOKEN")
	}

	// 2. ~/.cuan/config.json or ~/.config/cuan/config.json
	home, _ := os.UserHomeDir()
	configPaths := []string{
		filepath.Join(home, ".cuan", "config.json"),
		filepath.Join(home, ".config", "cuan", "config.json"),
	}

	for _, p := range configPaths {
		if data, err := os.ReadFile(p); err == nil {
			var fileCfg struct {
				BaseURL   string `json:"api_url"`
				AuthToken string `json:"auth_token"`
			}
			if json.Unmarshal(data, &fileCfg) == nil {
				if cfg.BaseURL == "" {
					cfg.BaseURL = fileCfg.BaseURL
				}
				if cfg.AuthToken == "" {
					cfg.AuthToken = fileCfg.AuthToken
				}
			}
			if cfg.BaseURL != "" && cfg.AuthToken != "" {
				break
			}
		}
	}

	// 3. System or local .env
	if cfg.BaseURL == "" || cfg.AuthToken == "" {
		for _, envPath := range []string{"/opt/cuan/.env", "/opt/cuan/current/.env", ".env"} {
			if data, err := os.ReadFile(envPath); err == nil {
				for _, line := range strings.Split(string(data), "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "CUAN_API_URL=") && cfg.BaseURL == "" {
						cfg.BaseURL = strings.Trim(strings.TrimPrefix(line, "CUAN_API_URL="), "\"'")
					}
					if strings.HasPrefix(line, "CUAN_AUTH_TOKEN=") && cfg.AuthToken == "" {
						cfg.AuthToken = strings.Trim(strings.TrimPrefix(line, "CUAN_AUTH_TOKEN="), "\"'")
					}
				}
			}
		}
	}

	// 4. Default fallbacks
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://app.cuan.work"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")

	cachedCuanConfig = &cfg
	return cfg
}

// CuanClient communicates with the Cuan server over HTTP API.
type CuanClient struct {
	BaseURL    string
	AuthToken  string
	HTTPClient *http.Client
}

// GetCuanClient returns the singleton or default Cuan API client.
func GetCuanClient() *CuanClient {
	if globalCuanClient != nil {
		return globalCuanClient
	}
	cfg := GetCuanConfig()
	globalCuanClient = &CuanClient{
		BaseURL:   cfg.BaseURL,
		AuthToken: cfg.AuthToken,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
	return globalCuanClient
}

// NormalizeAgentAlias maps agent shortcuts (@cuan, @sm, @dev, @support) to agent IDs.
func NormalizeAgentAlias(agent string) string {
	a := strings.TrimSpace(strings.ToLower(agent))
	switch a {
	case "@cuan", "cuan", "agent_cuan":
		return "agent_cuan"
	case "@sm", "sm", "agent_sm", "scrum", "scrum_master":
		return "agent_cuan"
	case "@dev", "dev", "agent_dev", "developer":
		return "agent_cuan"
	case "@support", "support", "agent_support", "cs":
		return "agent_cuan"
	case "@team", "team":
		return "agent_cuan"
	case "@pin", "pin", "agent_pin":
		return "agent_pin"
	case "":
		return "agent_cuan"
	default:
		return strings.TrimPrefix(agent, "@")
	}
}

// ResolveActiveAgentName resolves the display handle/name of an agent dynamically
// based on active agent settings (API or database) without hardcoding.
func ResolveActiveAgentName(ctx context.Context, agentID string) string {
	normalized := NormalizeAgentAlias(agentID)
	if normalized == "" {
		normalized = "agent_cuan"
	}

	// API and direct-database modes are separate connection scopes.
	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		return client.resolveAgentName(ctx, agentID)
	}

	// 2. Try querying via database if connected
	if dbConn := getPlatformDB(); dbConn != nil {
		var name string
		_ = dbConn.QueryRowContext(ctx, "SELECT name FROM agents WHERE id = $1", normalized).Scan(&name)
		if strings.TrimSpace(name) != "" {
			return strings.TrimSpace(name)
		}
	}
	return unavailableAgentName(agentID)
}

// A successful conversation creation must not query a different server's
// metadata or fall through to ambient database credentials for its identity.
func (c *CuanClient) resolveAgentName(ctx context.Context, agentID string) string {
	if agentData, err := c.GetAgent(ctx, NormalizeAgentAlias(agentID)); err == nil {
		if ag, ok := agentData["agent"].(map[string]any); ok {
			if name, ok := ag["name"].(string); ok && strings.TrimSpace(name) != "" {
				return strings.TrimSpace(name)
			}
		}
	}
	return unavailableAgentName(agentID)
}

func unavailableAgentName(agentID string) string {
	if strings.TrimSpace(agentID) != "" && NormalizeAgentAlias(agentID) != "agent_cuan" {
		return strings.TrimSpace(agentID)
	}
	return "Assistant"
}

// IsConfigured returns true if BaseURL and AuthToken are populated.
func (c *CuanClient) IsConfigured() bool {
	return c != nil && c.BaseURL != "" && c.AuthToken != ""
}

func (c *CuanClient) doRequest(ctx context.Context, method, endpoint string, body any) (*http.Response, error) {
	if method != http.MethodGet && method != http.MethodHead {
		if err := taskSessionOwnerWriteError(); err != nil {
			return nil, err
		}
	}

	u := c.BaseURL + endpoint

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	}

	return c.HTTPClient.Do(req)
}

// CreateConversation creates a new discussion room bound to an agent and project.
func (c *CuanClient) CreateConversation(ctx context.Context, agentID, projectID, title string) (map[string]any, error) {
	agentID = NormalizeAgentAlias(agentID)
	if projectID == "" {
		projectID = "default"
	}
	requestID := db.GenerateUUID()

	payload := map[string]string{
		"agent_id":   agentID,
		"request_id": requestID,
		"project_id": projectID,
	}
	if title != "" && title != "New conversation" {
		payload["title"] = title
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/chat/conversations", payload)
	if err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("create conversation failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal create conversation response: %w", err)
	}

	// Ensure standard fields are populated
	roomID, _ := res["conversation_id"].(string)
	if roomID == "" {
		roomID = requestID
	}
	res["room_id"] = roomID
	res["agent_id"] = agentID
	res["project_id"] = projectID
	if title != "" && title != "New conversation" {
		res["title"] = title
	}
	if agName, _ := res["agent_name"].(string); strings.TrimSpace(agName) == "" {
		res["agent_name"] = c.resolveAgentName(ctx, agentID)
	}

	return res, nil
}

// GetAgent retrieves agent details by ID.
func (c *CuanClient) GetAgent(ctx context.Context, id string) (map[string]any, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/agents/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, fmt.Errorf("get agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get agent failed (status %d)", resp.StatusCode)
	}

	var res map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("unmarshal get agent response: %w", err)
	}
	return res, nil
}

// SendMessage sends a user message to an inbox conversation room.
func (c *CuanClient) SendMessage(ctx context.Context, roomID, message string) (map[string]any, error) {
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return nil, fmt.Errorf("room_id is required")
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}

	requestID := db.GenerateUUID()
	payload := map[string]string{
		"message":    message,
		"request_id": requestID,
	}

	endpoint := fmt.Sprintf("/api/v1/chat/conversations/%s/messages", url.PathEscape(roomID))
	resp, err := c.doRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("send message: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("send message failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res map[string]any
	_ = json.Unmarshal(bodyBytes, &res)
	if res == nil {
		res = map[string]any{
			"status":  "sent",
			"room_id": roomID,
		}
	}
	return res, nil
}

// WaitForReply polls for the assistant's reply in the specified conversation.
func (c *CuanClient) WaitForReply(ctx context.Context, roomID string, sentTime time.Time, timeout time.Duration) (map[string]any, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	endpoint := fmt.Sprintf("/api/v1/chat/conversations/%s/messages?limit=10", url.PathEscape(roomID))

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("timeout waiting for agent reply after %v", timeout)
			}

			resp, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
			if err != nil {
				continue
			}

			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				continue
			}

			var data struct {
				Messages []map[string]any `json:"messages"`
			}
			if err := json.Unmarshal(bodyBytes, &data); err != nil {
				continue
			}

			// Find assistant message created after sentTime
			for i := len(data.Messages) - 1; i >= 0; i-- {
				msg := data.Messages[i]
				authorType, _ := msg["author_type"].(string)
				if authorType == "agent" || authorType == "assistant" {
					createdAtStr, _ := msg["created_at"].(string)
					msgTime, err := time.Parse(time.RFC3339Nano, createdAtStr)
					if err != nil {
						msgTime, _ = time.Parse(time.RFC3339, createdAtStr)
					}

					// If msg was created around or after sentTime (with 2s clock skew margin)
					if msgTime.IsZero() || msgTime.Add(2*time.Second).After(sentTime) {
						return msg, nil
					}
				}
			}
		}
	}
}

// GetConversationMessages retrieves recent messages from a conversation room.
func (c *CuanClient) GetConversationMessages(ctx context.Context, roomID string, limit int) ([]map[string]any, error) {
	snapshot, err := c.GetConversationSnapshot(ctx, roomID, limit)
	if err != nil {
		return nil, err
	}
	return snapshot["messages"].([]map[string]any), nil
}

func (c *CuanClient) GetConversationSnapshot(ctx context.Context, roomID string, limit int) (map[string]any, error) {
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return nil, fmt.Errorf("room_id is required")
	}

	var endpoint string
	if limit < 0 {
		endpoint = fmt.Sprintf("/api/v1/chat/conversations/%s/messages?limit=all", url.PathEscape(roomID))
	} else {
		if limit == 0 {
			limit = 50
		} else if limit > 500 {
			limit = 500
		}
		endpoint = fmt.Sprintf("/api/v1/chat/conversations/%s/messages?limit=%d", url.PathEscape(roomID), limit)
	}
	resp, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("get conversation messages: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get conversation messages failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var data struct {
		Messages  []map[string]any `json:"messages"`
		Requests  []map[string]any `json:"requests"`
		TaskCount int              `json:"task_count"`
	}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, fmt.Errorf("unmarshal messages: %w", err)
	}

	if data.Messages == nil {
		data.Messages = []map[string]any{}
	}
	return map[string]any{"room_id": roomID, "messages": data.Messages, "total": len(data.Messages), "requests": data.Requests, "task_count": data.TaskCount}, nil
}

// DeleteMessage deletes a message by its UUID via /api/v1/messages/{id}.
func (c *CuanClient) DeleteMessage(ctx context.Context, messageID string) (map[string]any, error) {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return nil, fmt.Errorf("message_id is required")
	}
	endpoint := fmt.Sprintf("/api/v1/messages/%s", url.PathEscape(messageID))
	resp, err := c.doRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("delete message: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("delete message failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]any
	if err := json.Unmarshal(bodyBytes, &result); err != nil || result == nil {
		return map[string]any{"ok": true, "deleted": true, "id": messageID}, nil
	}
	return result, nil
}

// ListConversations returns available inbox conversations and contacts.
func (c *CuanClient) ListConversations(ctx context.Context, projectID string, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/chat/contacts", nil)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list conversations failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var data struct {
		Contacts []map[string]any `json:"contacts"`
	}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, fmt.Errorf("unmarshal contacts: %w", err)
	}

	var res []map[string]any
	for _, contact := range data.Contacts {
		convID, _ := contact["conversation_id"].(string)
		title, _ := contact["name"].(string)
		agentID, _ := contact["agent_id"].(string)
		lastMsg, _ := contact["last_message"].(string)
		updatedAt, _ := contact["updated_at"].(string)

		res = append(res, map[string]any{
			"id":                     convID,
			"conversation_id":        convID,
			"title":                  title,
			"agent_id":               agentID,
			"last_message":           lastMsg,
			"updated_at":             updatedAt,
			"resolved":               contact["resolved"],
			"resolved_at":            contact["resolved_at"],
			"resolution_observed_at": contact["resolution_observed_at"],
			"has_messages":           contact["has_messages"],
		})
		if len(res) >= limit {
			break
		}
	}

	if res == nil {
		res = []map[string]any{}
	}
	return res, nil
}

// ListTasks queries tasks from the Cuan API.
func (c *CuanClient) ListTasks(ctx context.Context, projectID, status string, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	endpoint := fmt.Sprintf("/api/v1/tasks?limit=%d", limit)
	if projectID != "" {
		endpoint += "&project_id=" + url.QueryEscape(projectID)
	}
	if status != "" {
		endpoint += "&status=" + url.QueryEscape(status)
	}

	resp, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list tasks failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var data struct {
		Tasks []map[string]any `json:"tasks"`
	}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, fmt.Errorf("unmarshal tasks: %w", err)
	}

	if data.Tasks == nil {
		data.Tasks = []map[string]any{}
	}
	if limit > 0 && len(data.Tasks) > limit {
		data.Tasks = data.Tasks[:limit]
	}
	return data.Tasks, nil
}

// GetTask returns a specific task by ID.
func (c *CuanClient) GetTask(ctx context.Context, taskID string) (map[string]any, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	endpoint := fmt.Sprintf("/api/v1/tasks/%s", url.PathEscape(taskID))
	resp, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return map[string]any{
			"error":   "task_not_found",
			"task_id": taskID,
		}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get task failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal task: %w", err)
	}
	return res, nil
}

// CreateTask delegates guidance snapshots and task identity to the Owner API.
// An uncertain response must be reconciled before retrying this mutation.
func (c *CuanClient) CreateTask(ctx context.Context, projectID, title, instructions, priority string, checklist []string) (map[string]any, error) {
	return c.CreateTaskWithRoom(ctx, projectID, title, instructions, priority, "", checklist)
}

// CreateTaskWithRoom delegates guidance snapshots, task identity and room binding to the Owner API.
func (c *CuanClient) CreateTaskWithRoom(ctx context.Context, projectID, title, instructions, priority, roomID string, checklist []string) (map[string]any, error) {
	items := make([]map[string]any, 0, len(checklist))
	for _, text := range checklist {
		items = append(items, map[string]any{"text": text, "completed": false})
	}
	payload := map[string]any{"project_id": projectID, "title": title, "instructions": instructions, "priority": priority, "dod_checklist": items}
	if strings.TrimSpace(roomID) != "" {
		payload["source_room_id"] = strings.TrimSpace(roomID)
	}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/tasks", payload)
	if err != nil {
		return nil, fmt.Errorf("task creation could not be confirmed; inspect the task board before retrying")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("task creation was not confirmed (status %d); inspect the task board before retrying", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("task creation returned an unreadable receipt; inspect the task board before retrying")
	}
	if id, ok := result["id"].(string); !ok || strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("task creation returned no task identity; inspect the task board before retrying")
	}
	redactTaskDiagnostic(result)
	return result, nil
}

// UpdateTask updates task status, summary, and verification evidence.
func (c *CuanClient) UpdateTask(ctx context.Context, taskID, status, summary, completeNote string) (map[string]any, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	payload := map[string]any{}
	if status != "" {
		// Normalize status synonyms
		switch strings.ToLower(status) {
		case "doing", "in_progress", "progress":
			payload["status"] = "in_progress"
		case "ready_for_review", "review":
			payload["status"] = "review"
		case "done", "completed":
			payload["status"] = "done"
		case "blocked":
			payload["status"] = "blocked"
		case "todo", "backlog":
			payload["status"] = "backlog"
		case "canceled", "cancelled":
			payload["status"] = "canceled"
		default:
			payload["status"] = status
		}
	}
	if summary != "" {
		payload["summary"] = summary
	}
	if completeNote != "" {
		payload["complete_note"] = completeNote
	}

	endpoint := fmt.Sprintf("/api/v1/tasks/%s", url.PathEscape(taskID))
	resp, err := c.doRequest(ctx, http.MethodPut, endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("update task: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("update task failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal updated task: %w", err)
	}
	return res, nil
}

// ListProjects queries projects from the Cuan API.
func (c *CuanClient) ListProjects(ctx context.Context, limit int) ([]map[string]any, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/projects", nil)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list projects failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var data struct {
		Projects []map[string]any `json:"projects"`
	}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, fmt.Errorf("unmarshal projects: %w", err)
	}

	if data.Projects == nil {
		data.Projects = []map[string]any{}
	}
	return data.Projects, nil
}

// GetProject queries a single project from the Cuan API.
func (c *CuanClient) GetProject(ctx context.Context, projectID string) (map[string]any, error) {
	endpoint := fmt.Sprintf("/api/v1/projects/%s", url.PathEscape(projectID))
	resp, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return map[string]any{
			"error":      "project_not_found",
			"project_id": projectID,
		}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get project failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal project: %w", err)
	}
	return res, nil
}

// ListProjectDeployments queries recent deployments for a project from the Cuan API.
func (c *CuanClient) ListProjectDeployments(ctx context.Context, projectID string, limit int) (map[string]any, error) {
	if limit <= 0 {
		limit = 20
	}
	endpoint := fmt.Sprintf("/api/v1/projects/%s/deployments?limit=%d", url.PathEscape(projectID), limit)
	resp, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("list project deployments: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list project deployments failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal deployments: %w", err)
	}
	return res, nil
}

// GetProjectDeployment queries a single deployment (including build/deploy logs) from the Cuan API.
func (c *CuanClient) GetProjectDeployment(ctx context.Context, projectID, deploymentID string) (map[string]any, error) {
	endpoint := fmt.Sprintf("/api/v1/projects/%s/deployments/%s", url.PathEscape(projectID), url.PathEscape(deploymentID))
	resp, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("get project deployment: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get project deployment failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal deployment detail: %w", err)
	}
	return res, nil
}

// GetProjectTerminalStatus queries the project terminal status, upstream integrity, and worker log tail.
func (c *CuanClient) GetProjectTerminalStatus(ctx context.Context, projectID string) (map[string]any, error) {
	endpoint := fmt.Sprintf("/api/v1/projects/%s/terminal/status", url.PathEscape(projectID))
	resp, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("get project terminal status: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get project terminal status failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal terminal status: %w", err)
	}
	return res, nil
}

// ListScheduledTasks queries user-defined recurring routines from Cuan.
func (c *CuanClient) ListScheduledTasks(ctx context.Context) ([]map[string]any, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/system/scheduler/tasks", nil)
	if err != nil {
		return nil, fmt.Errorf("list scheduled tasks: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list scheduled tasks failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var data struct {
		Tasks []map[string]any `json:"tasks"`
	}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, fmt.Errorf("unmarshal scheduled tasks: %w", err)
	}
	if data.Tasks == nil {
		data.Tasks = []map[string]any{}
	}
	return data.Tasks, nil
}

// CreateScheduledTask registers a new recurring routine in Cuan.
func (c *CuanClient) CreateScheduledTask(ctx context.Context, payload map[string]any) (map[string]any, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/system/scheduler/tasks", payload)
	if err != nil {
		return nil, fmt.Errorf("create scheduled task: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("create scheduled task failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal created scheduled task: %w", err)
	}
	return res, nil
}

// UpdateScheduledTask modifies an existing recurring routine in Cuan by ID.
func (c *CuanClient) UpdateScheduledTask(ctx context.Context, id string, payload map[string]any) (map[string]any, error) {
	endpoint := fmt.Sprintf("/api/v1/system/scheduler/tasks/%s", url.PathEscape(id))
	resp, err := c.doRequest(ctx, http.MethodPut, endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("update scheduled task: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("update scheduled task failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal updated scheduled task: %w", err)
	}
	return res, nil
}

// DeleteScheduledTask removes a recurring routine in Cuan by ID.
func (c *CuanClient) DeleteScheduledTask(ctx context.Context, id string) error {
	endpoint := fmt.Sprintf("/api/v1/system/scheduler/tasks/%s", url.PathEscape(id))
	resp, err := c.doRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("delete scheduled task: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete scheduled task failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

// ToggleScheduledTask enables or disables a recurring routine in Cuan.
func (c *CuanClient) ToggleScheduledTask(ctx context.Context, id string, enabled *bool) (map[string]any, error) {
	endpoint := fmt.Sprintf("/api/v1/system/scheduler/tasks/%s/toggle", url.PathEscape(id))
	var body any
	if enabled != nil {
		body = map[string]any{"enabled": *enabled}
	}
	resp, err := c.doRequest(ctx, http.MethodPatch, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("toggle scheduled task: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("toggle scheduled task failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal toggle result: %w", err)
	}
	return res, nil
}

// TriggerScheduledTask triggers an immediate execution of a scheduled routine in Cuan.
func (c *CuanClient) TriggerScheduledTask(ctx context.Context, id string) error {
	endpoint := fmt.Sprintf("/api/v1/system/scheduler/tasks/%s/trigger", url.PathEscape(id))
	resp, err := c.doRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("trigger scheduled task: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("trigger scheduled task failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

// CreateProject creates a new project in Cuan.
func (c *CuanClient) CreateProject(ctx context.Context, name, description, repoURL, branch string) (map[string]any, error) {
	payload := map[string]any{"name": name}
	if description != "" {
		payload["description"] = description
	}
	if repoURL != "" {
		payload["repo_url"] = repoURL
	}
	if branch != "" {
		payload["default_branch"] = branch
	}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/projects", payload)
	if err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("create project failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}
	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal create project result: %w", err)
	}
	return res, nil
}

// ListApps lists apps from the running Cuan server.
func (c *CuanClient) ListApps(ctx context.Context) (map[string]any, error) {
	return c.getJSONMap(ctx, "/api/v1/apps", "list apps")
}

// ListSkills lists skills from the running Cuan server.
func (c *CuanClient) ListSkills(ctx context.Context) (map[string]any, error) {
	return c.getJSONMap(ctx, "/api/v1/skills", "list skills")
}

// ToggleAgentSkill toggles a skill for an agent via the running Cuan server.
func (c *CuanClient) ToggleAgentSkill(ctx context.Context, agentID, skillID string, enabled bool) (map[string]any, error) {
	endpoint := fmt.Sprintf("/api/v1/agents/%s/skills", url.PathEscape(NormalizeAgentAlias(agentID)))
	payload := map[string]any{
		"skills": []map[string]any{{"skill_id": skillID, "enabled": enabled}},
	}
	resp, err := c.doRequest(ctx, http.MethodPut, endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("toggle agent skill: %w", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("toggle agent skill failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}
	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal toggle agent skill: %w", err)
	}
	return res, nil
}

func (c *CuanClient) getJSONMap(ctx context.Context, endpoint, op string) (map[string]any, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s failed (status %d): %s", op, resp.StatusCode, string(bodyBytes))
	}
	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", op, err)
	}
	return res, nil
}

// UpdateProject updates an existing project in Cuan.
func (c *CuanClient) UpdateProject(ctx context.Context, id, name, description, repoURL, branch string) (map[string]any, error) {
	proj, err := c.GetProject(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get current project: %w", err)
	}
	realID, _ := proj["id"].(string)
	if realID == "" {
		realID = id
	}
	currentName, _ := proj["name"].(string)
	if name == "" {
		name = currentName
	}
	currentRepo, _ := proj["repo_url"].(string)
	if repoURL == "" {
		repoURL = currentRepo
	}
	currentBranch, _ := proj["default_branch"].(string)
	if branch == "" {
		branch = currentBranch
		if branch == "" {
			branch = "main"
		}
	}
	var cuanSettings map[string]any
	if meta, ok := proj["metadata"].(map[string]any); ok {
		if cs, ok := meta["cuan_settings"].(map[string]any); ok {
			cuanSettings = cs
		}
	}
	goal, _ := proj["goal"].(string)
	if goal == "" && cuanSettings != nil {
		goal, _ = cuanSettings["goal"].(string)
	}
	if description != "" {
		goal = description
	}
	instructions, _ := proj["instructions"].(string)
	if instructions == "" && cuanSettings != nil {
		instructions, _ = cuanSettings["instructions"].(string)
	}

	payload := map[string]any{
		"name":           name,
		"repo_url":       repoURL,
		"default_branch": branch,
		"goal":           goal,
		"instructions":   instructions,
	}
	if cuanSettings != nil {
		for _, k := range []string{"deploy_branch", "deploy_script", "auto_deploy", "environment"} {
			if v, ok := cuanSettings[k]; ok {
				payload[k] = v
			}
		}
	}
	if updatedAtStr, ok := proj["updated_at"].(string); ok && updatedAtStr != "" {
		if t, parseErr := time.Parse(time.RFC3339Nano, updatedAtStr); parseErr == nil {
			payload["expected_updated_at"] = t
		} else if t, parseErr := time.Parse(time.RFC3339, updatedAtStr); parseErr == nil {
			payload["expected_updated_at"] = t
		}
	}

	endpoint := fmt.Sprintf("/api/v1/projects/%s/settings", url.PathEscape(realID))
	resp, err := c.doRequest(ctx, http.MethodPatch, endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("update project: %w", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("update project failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}
	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal update project result: %w", err)
	}
	return res, nil
}

// GetProjectDeploymentConfig retrieves deployment configuration for a project.
func (c *CuanClient) GetProjectDeploymentConfig(ctx context.Context, id string) (map[string]any, error) {
	proj, err := c.GetProject(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get current project: %w", err)
	}
	realID, _ := proj["id"].(string)
	if realID == "" {
		realID = id
	}
	endpoint := fmt.Sprintf("/api/v1/projects/%s/deployment-config", url.PathEscape(realID))
	resp, err := c.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("get project deployment config: %w", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get project deployment config failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}
	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal deployment config result: %w", err)
	}
	return res, nil
}

// UpdateProjectDeploymentConfig updates deployment configuration for a project.
func (c *CuanClient) UpdateProjectDeploymentConfig(ctx context.Context, id string, autoDeploy *bool, branch, script string) (map[string]any, error) {
	proj, err := c.GetProject(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get current project: %w", err)
	}
	realID, _ := proj["id"].(string)
	if realID == "" {
		realID = id
	}
	payload := map[string]any{
		"deploy_branch": branch,
		"deploy_script": script,
	}
	if autoDeploy != nil {
		payload["auto_deploy"] = *autoDeploy
	}
	endpoint := fmt.Sprintf("/api/v1/projects/%s/deployment-config", url.PathEscape(realID))
	resp, err := c.doRequest(ctx, http.MethodPut, endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("update project deployment config: %w", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("update project deployment config failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}
	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal update deployment config result: %w", err)
	}
	return res, nil
}

// PushProjectRepo pushes a branch from a project's workspace via the server's saved GitHub connection.
func (c *CuanClient) PushProjectRepo(ctx context.Context, id, branch string, allowDefault bool) (map[string]any, error) {
	payload := map[string]any{}
	if strings.TrimSpace(branch) != "" {
		payload["branch"] = strings.TrimSpace(branch)
	}
	if allowDefault {
		payload["allow_default"] = true
	}
	endpoint := fmt.Sprintf("/api/v1/projects/%s/repo/push", url.PathEscape(id))
	resp, err := c.doRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("push project repo: %w", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("push project repo failed (status %d): %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}
	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal push result: %w", err)
	}
	return res, nil
}

// CreateContact creates a new contact in Cuan.
func (c *CuanClient) CreateContact(ctx context.Context, name, email, phone, role, company, notes string) (map[string]any, error) {
	reqID := db.GenerateUUID()
	payload := map[string]any{
		"request_id": reqID,
		"name":       name,
		"email":      email,
	}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/contacts", payload)
	if err != nil {
		return nil, fmt.Errorf("create contact: %w", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("create contact failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}
	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal create contact result: %w", err)
	}
	if notes != "" {
		contactID, _ := res["id"].(string)
		if contactID == "" {
			contactID = reqID
		}
		notePayload := map[string]any{
			"request_id": db.GenerateUUID(),
			"kind":       "note",
			"body":       notes,
		}
		noteEndpoint := fmt.Sprintf("/api/v1/contacts/%s/memory", url.PathEscape(contactID))
		if noteResp, noteErr := c.doRequest(ctx, http.MethodPost, noteEndpoint, notePayload); noteErr == nil {
			noteResp.Body.Close()
		}
	}
	return res, nil
}

// UpdateContact updates an existing contact in Cuan.
func (c *CuanClient) UpdateContact(ctx context.Context, id, name, email, phone, role, company, notes string) (map[string]any, error) {
	endpoint := fmt.Sprintf("/api/v1/contacts/%s", url.PathEscape(id))
	var tags []string
	if role != "" {
		tags = append(tags, role)
	}
	if company != "" {
		tags = append(tags, company)
	}
	payload := map[string]any{"tags": tags}
	resp, err := c.doRequest(ctx, http.MethodPatch, endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("update contact: %w", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("update contact failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}
	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal update contact result: %w", err)
	}
	if notes != "" {
		notePayload := map[string]any{
			"request_id": db.GenerateUUID(),
			"kind":       "note",
			"body":       notes,
		}
		noteEndpoint := fmt.Sprintf("/api/v1/contacts/%s/memory", url.PathEscape(id))
		if noteResp, noteErr := c.doRequest(ctx, http.MethodPost, noteEndpoint, notePayload); noteErr == nil {
			noteResp.Body.Close()
		}
	}
	return res, nil
}

// ListTeamMembers lists human team members in Cuan.
func (c *CuanClient) ListTeamMembers(ctx context.Context) ([]map[string]any, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/team/members", nil)
	if err != nil {
		return nil, fmt.Errorf("list team members: %w", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list team members failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}
	var wrapper struct {
		Members     []map[string]any `json:"members"`
		Invitations []map[string]any `json:"invitations"`
	}
	if err := json.Unmarshal(bodyBytes, &wrapper); err == nil && (wrapper.Members != nil || wrapper.Invitations != nil) {
		result := make([]map[string]any, 0, len(wrapper.Members)+len(wrapper.Invitations))
		result = append(result, wrapper.Members...)
		for _, inv := range wrapper.Invitations {
			inv["status"] = "invited"
			result = append(result, inv)
		}
		return result, nil
	}
	var members []map[string]any
	if err := json.Unmarshal(bodyBytes, &members); err != nil {
		return nil, fmt.Errorf("unmarshal team members: %w", err)
	}
	return members, nil
}

// UpdateProviderAccount updates AI provider account settings.
func (c *CuanClient) UpdateProviderAccount(ctx context.Context, accountID, label, modelID, fallbackModelID string, allowFallback *bool) (map[string]any, error) {
	endpoint := fmt.Sprintf("/api/v1/provider-accounts/%s", url.PathEscape(accountID))
	payload := map[string]any{}
	if label != "" || modelID != "" {
		// Preserve existing label/model_id because PATCH /provider-accounts/{id}
		// requires non-empty label and overwrites both fields.
		if accounts, listErr := c.ListProviderAccounts(ctx); listErr == nil {
			for _, acct := range accounts {
				if acct != nil && acct.ID == accountID {
					if label == "" {
						label = acct.Label
					}
					if modelID == "" {
						modelID = acct.ModelID
					}
					break
				}
			}
		}
	}
	if label != "" {
		payload["label"] = label
	}
	if modelID != "" {
		payload["model_id"] = modelID
	}
	var res map[string]any
	if len(payload) > 0 {
		resp, err := c.doRequest(ctx, http.MethodPatch, endpoint, payload)
		if err != nil {
			return nil, fmt.Errorf("update provider account: %w", err)
		}
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("update provider account failed (status %d): %s", resp.StatusCode, string(bodyBytes))
		}
		_ = json.Unmarshal(bodyBytes, &res)
	}
	if allowFallback != nil || fallbackModelID != "" {
		fallbackEndpoint := fmt.Sprintf("/api/v1/provider-accounts/%s/model-fallback", url.PathEscape(accountID))
		fbPayload := map[string]any{}
		if allowFallback != nil {
			fbPayload["enabled"] = *allowFallback
		} else {
			fbPayload["enabled"] = true
		}
		if fallbackModelID != "" {
			fbPayload["model_id"] = fallbackModelID
		}
		resp, err := c.doRequest(ctx, http.MethodPatch, fallbackEndpoint, fbPayload)
		if err != nil {
			return nil, fmt.Errorf("update provider account fallback: %w", err)
		}
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("update provider account fallback failed (status %d): %s", resp.StatusCode, string(bodyBytes))
		}
		_ = json.Unmarshal(bodyBytes, &res)
	}
	if res == nil {
		res = map[string]any{"account_id": accountID, "status": "updated"}
	}
	return res, nil
}

// GetBrandMemory reads brand taste guidelines and memory in Cuan.
func (c *CuanClient) GetBrandMemory(ctx context.Context) (map[string]any, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/taste", nil)
	if err != nil {
		return nil, fmt.Errorf("get brand memory: %w", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get brand memory failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}
	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal brand memory: %w", err)
	}
	return res, nil
}

// UpdateBrandMemory updates brand taste guidelines in Cuan.
func (c *CuanClient) UpdateBrandMemory(ctx context.Context, guidelines string) (map[string]any, error) {
	payload := map[string]any{
		"category": "brand",
		"topic":    "guidelines",
		"content":  guidelines,
	}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/taste/items", payload)
	if err != nil {
		return nil, fmt.Errorf("update brand memory: %w", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("update brand memory failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}
	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal update brand memory result: %w", err)
	}
	return res, nil
}

// RunTask dispatches an execution attempt for a task in Cuan.
func (c *CuanClient) RunTask(ctx context.Context, taskID, agentID string) (map[string]any, error) {
	reqID := db.GenerateUUID()
	payload := map[string]any{
		"request_id": reqID,
		"agent_id":   agentID,
	}
	endpoint := fmt.Sprintf("/api/v1/tasks/%s/run-agent", url.PathEscape(taskID))
	resp, err := c.doRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return nil, fmt.Errorf("run task: %w", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("run task failed (status %d): %s", resp.StatusCode, string(bodyBytes))
	}
	var res map[string]any
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal run task result: %w", err)
	}
	return res, nil
}
