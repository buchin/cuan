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

	// 3. Current or system .env
	if cfg.BaseURL == "" || cfg.AuthToken == "" {
		for _, envPath := range []string{".env", "/opt/cuan/.env", "/opt/cuan/current/.env"} {
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
		cfg.BaseURL = "https://cuan.dojo.cc"
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

// IsConfigured returns true if BaseURL and AuthToken are populated.
func (c *CuanClient) IsConfigured() bool {
	return c != nil && c.BaseURL != "" && c.AuthToken != ""
}

func (c *CuanClient) doRequest(ctx context.Context, method, endpoint string, body any) (*http.Response, error) {
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
	if limit <= 0 || limit > 100 {
		limit = 30
	}

	endpoint := fmt.Sprintf("/api/v1/chat/conversations/%s/messages?limit=%d", url.PathEscape(roomID), limit)
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
			"id":              convID,
			"conversation_id": convID,
			"title":           title,
			"agent_id":        agentID,
			"last_message":    lastMsg,
			"updated_at":      updatedAt,
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
	if limit <= 0 || limit > 100 {
		limit = 20
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
