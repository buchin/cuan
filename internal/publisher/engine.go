package publisher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/crypto"
	"github.com/buchin/cuan/internal/db"
)

// EngineOption allows configuring Engine.
type EngineOption func(*Engine)

// WithStudioRepo sets the ProjectStudioRepository on Engine.
func WithStudioRepo(r db.ProjectStudioRepository) EngineOption {
	return func(e *Engine) {
		e.studioRepo = r
	}
}

// WithIntegrationRepo sets the ProjectIntegrationRepository on Engine.
func WithIntegrationRepo(r db.ProjectIntegrationRepository) EngineOption {
	return func(e *Engine) {
		e.integrationRepo = r
	}
}

// WithSecretKey sets the AES-256-GCM encryption secret key.
func WithSecretKey(key string) EngineOption {
	return func(e *Engine) {
		e.secretKey = key
	}
}

// WithClientFactory overrides the WordPress client factory (useful for tests).
func WithClientFactory(factory func(cfg WordPressConfig) (Client, error)) EngineOption {
	return func(e *Engine) {
		e.clientFactory = factory
	}
}

// Engine provides high-level operations for CMS publishing and multi-format exports.
type Engine struct {
	studioRepo      db.ProjectStudioRepository
	integrationRepo db.ProjectIntegrationRepository
	secretKey       string
	clientFactory   func(cfg WordPressConfig) (Client, error)
}

// NewEngine creates a new Engine instance.
func NewEngine(opts ...EngineOption) *Engine {
	e := &Engine{
		secretKey: "cuan-default-publisher-secret-key-32b",
		clientFactory: func(cfg WordPressConfig) (Client, error) {
			return NewWordPressClient(cfg)
		},
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// SetupWordPressIntegration stores or updates encrypted WordPress credentials and optionally verifies them.
func (e *Engine) SetupWordPressIntegration(ctx context.Context, projectID string, cfg WordPressConfig, testConnection bool) (*db.ProjectIntegrationDTO, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, db.ErrEmptyIntegrationProjectID
	}
	if e.integrationRepo == nil {
		return nil, errors.New("project integration repository is not configured")
	}

	if err := cfg.Normalize(); err != nil {
		return nil, err
	}

	// Test connection if requested
	if testConnection {
		client, err := e.clientFactory(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize wordpress client: %w", err)
		}
		if _, err := client.TestConnection(ctx); err != nil {
			return nil, fmt.Errorf("wordpress connection test failed: %w", err)
		}
	}

	// Serialize configuration to JSON
	rawConfigJSON, err := json.Marshal(map[string]string{
		"site_url":             cfg.SiteURL,
		"username":             cfg.Username,
		"application_password": cfg.ApplicationPassword,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal wordpress config: %w", err)
	}

	// Encrypt via AES-256-GCM
	encryptedConfig, err := crypto.EncryptStringAES256GCM(string(rawConfigJSON), e.secretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt wordpress credentials: %w", err)
	}

	// Check if integration already exists
	existing, err := e.integrationRepo.GetIntegrationByProjectAndType(ctx, projectID, "wordpress")
	if err == nil && existing != nil {
		existing.ConfigEncrypted = encryptedConfig
		existing.Status = "active"
		if err := e.integrationRepo.UpdateIntegration(ctx, existing); err != nil {
			return nil, fmt.Errorf("failed to update wordpress integration: %w", err)
		}
		dto := existing.ToDTO()
		return &dto, nil
	}

	// Create new integration
	newIn := &db.ProjectIntegration{
		ID:              db.GenerateUUID(),
		ProjectID:       projectID,
		IntegrationType: "wordpress",
		ConfigEncrypted: encryptedConfig,
		Status:          "active",
	}
	if err := e.integrationRepo.CreateIntegration(ctx, newIn); err != nil {
		return nil, fmt.Errorf("failed to save wordpress integration: %w", err)
	}

	dto := newIn.ToDTO()
	return &dto, nil
}

// GetWordPressIntegration retrieves the public DTO for the project's WordPress integration.
func (e *Engine) GetWordPressIntegration(ctx context.Context, projectID string) (*db.ProjectIntegrationDTO, error) {
	if e.integrationRepo == nil {
		return nil, errors.New("project integration repository is not configured")
	}
	in, err := e.integrationRepo.GetIntegrationByProjectAndType(ctx, projectID, "wordpress")
	if err != nil {
		return nil, err
	}
	dto := in.ToDTO()
	return &dto, nil
}

// PublishTask publishes a task deliverable (agent_output or description) directly to WordPress.
func (e *Engine) PublishTask(ctx context.Context, taskID string, opts PublishTaskOptions) (*PublishResult, error) {
	if strings.TrimSpace(taskID) == "" {
		return nil, errors.New("task id cannot be empty")
	}
	if e.studioRepo == nil {
		return nil, errors.New("project studio repository is not configured")
	}
	if e.integrationRepo == nil {
		return nil, errors.New("project integration repository is not configured")
	}

	// 1. Fetch Task
	task, err := e.studioRepo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}

	// 2. Fetch WordPress Integration for task's project
	in, err := e.integrationRepo.GetIntegrationByProjectAndType(ctx, task.ProjectID, "wordpress")
	if err != nil {
		return nil, fmt.Errorf("%w for project %s", ErrIntegrationNotFound, task.ProjectID)
	}

	if in.ConfigEncrypted == "" {
		return nil, errors.New("wordpress integration has no configured credentials")
	}

	// 3. Decrypt credentials via AES-256-GCM
	decryptedJSON, err := crypto.DecryptStringAES256GCM(in.ConfigEncrypted, e.secretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt wordpress credentials: %w", err)
	}

	var confMap map[string]string
	if err := json.Unmarshal([]byte(decryptedJSON), &confMap); err != nil {
		return nil, fmt.Errorf("invalid decrypted credentials json: %w", err)
	}

	wpCfg := WordPressConfig{
		SiteURL:             confMap["site_url"],
		Username:            confMap["username"],
		ApplicationPassword: confMap["application_password"],
	}
	if err := wpCfg.Normalize(); err != nil {
		return nil, fmt.Errorf("invalid wordpress config: %w", err)
	}

	// 4. Resolve Content
	content := strings.TrimSpace(opts.Content)
	if content == "" {
		content = strings.TrimSpace(task.AgentOutput)
	}
	if content == "" {
		content = strings.TrimSpace(task.Description)
	}
	if content == "" {
		return nil, ErrEmptyTaskContent
	}

	// 5. Resolve Title
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = strings.TrimSpace(task.Title)
	}
	if title == "" {
		title = "Untitled Task Post"
	}

	// 6. Resolve Status
	status := strings.TrimSpace(opts.Status)
	if status == "" {
		status = "draft"
	}

	// 7. Resolve Tags
	tags := opts.Tags
	if len(tags) == 0 {
		tags = task.Tags
	}

	// 8. Publish via WordPress Client
	client, err := e.clientFactory(wpCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create wordpress client: %w", err)
	}

	post, err := client.CreatePost(ctx, CreatePostRequest{
		Title:           title,
		Content:         content,
		Status:          status,
		Tags:            tags,
		ConvertMarkdown: opts.ConvertMarkdown || true,
	})
	if err != nil {
		return nil, err
	}

	// 9. Update Task with publication result
	now := time.Now().UTC()
	task.PublicationURL = post.Link
	task.PublishedAt = &now
	if err := e.studioRepo.UpdateTask(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to update task with publication result: %w", err)
	}

	return &PublishResult{
		Success:        true,
		TaskID:         task.ID,
		PostID:         post.ID,
		Status:         post.Status,
		PublicationURL: post.Link,
		PublishedAt:    task.PublishedAt,
	}, nil
}

// ExportTask formats a task deliverable into Markdown, HTML, or Plain Text.
func (e *Engine) ExportTask(ctx context.Context, taskID, format string) (*ExportResult, error) {
	if strings.TrimSpace(taskID) == "" {
		return nil, errors.New("task id cannot be empty")
	}
	if e.studioRepo == nil {
		return nil, errors.New("project studio repository is not configured")
	}

	task, err := e.studioRepo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}

	content := strings.TrimSpace(task.AgentOutput)
	if content == "" {
		content = strings.TrimSpace(task.Description)
	}

	return FormatContent(task.ID, task.Title, content, format)
}
