package creator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
)

var (
	ErrInvalidTemplateID = errors.New("invalid or unknown creator blueprint template ID")
	ErrNilStudioRepo     = errors.New("project studio repository is required")
	ErrNilAgentRepo      = errors.New("agent repository is required")
	ErrEmptyProjectName  = errors.New("project name cannot be empty")
)

var nonAlphaNumRegex = regexp.MustCompile(`[^a-z0-9]+`)

// InstantiateRequest holds the configuration for instantiating a creator project.
type InstantiateRequest struct {
	TemplateID    string          `json:"template_id"`
	ProjectName   string          `json:"project_name"`
	RepoURL       string          `json:"repo_url,omitempty"`
	DefaultBranch string          `json:"default_branch,omitempty"`
	Token         string          `json:"token,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
}

// InstantiateResponse is returned upon successful atomic instantiation of a creator project.
type InstantiateResponse struct {
	Status     string                  `json:"status"`
	TemplateID string                  `json:"template_id"`
	Project    *project.ProjectResponse `json:"project"`
	Room       db.ProjectRoomDTO       `json:"room"`
	Agent      db.AgentDTO             `json:"agent"`
	Tasks      []db.ProjectTaskDTO     `json:"tasks"`
}

// BlueprintEngine manages creator blueprints and atomic project instantiation.
type BlueprintEngine struct {
	mu           sync.RWMutex
	projectStore project.Store
	projectRepo  db.ProjectRepository
	studioRepo   db.ProjectStudioRepository
	agentRepo    db.AgentRepository
}

// Option configures the BlueprintEngine.
type Option func(*BlueprintEngine)

// WithProjectStore sets the project.Store.
func WithProjectStore(s project.Store) Option {
	return func(e *BlueprintEngine) {
		e.projectStore = s
	}
}

// WithProjectRepo sets the db.ProjectRepository.
func WithProjectRepo(r db.ProjectRepository) Option {
	return func(e *BlueprintEngine) {
		e.projectRepo = r
	}
}

// WithStudioRepo sets the db.ProjectStudioRepository.
func WithStudioRepo(r db.ProjectStudioRepository) Option {
	return func(e *BlueprintEngine) {
		e.studioRepo = r
	}
}

// WithAgentRepo sets the db.AgentRepository.
func WithAgentRepo(r db.AgentRepository) Option {
	return func(e *BlueprintEngine) {
		e.agentRepo = r
	}
}

// NewBlueprintEngine constructs a new BlueprintEngine with the supplied options.
func NewBlueprintEngine(opts ...Option) *BlueprintEngine {
	e := &BlueprintEngine{}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// ListTemplates returns all available blueprint templates.
func (e *BlueprintEngine) ListTemplates() []Template {
	return DefaultTemplates()
}

// GetTemplate retrieves a template by ID or alias.
func (e *BlueprintEngine) GetTemplate(id string) (*Template, error) {
	tmpl, ok := LookupTemplate(id)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrInvalidTemplateID, id)
	}
	return tmpl, nil
}

// Instantiate atomically creates a new project, room, agent persona, and pipeline tasks.
func (e *BlueprintEngine) Instantiate(ctx context.Context, req InstantiateRequest) (*InstantiateResponse, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	tmplID := strings.TrimSpace(req.TemplateID)
	if tmplID == "" {
		return nil, fmt.Errorf("%w: template_id is required", ErrInvalidTemplateID)
	}

	tmpl, ok := LookupTemplate(tmplID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrInvalidTemplateID, tmplID)
	}

	if e.studioRepo == nil {
		return nil, ErrNilStudioRepo
	}
	if e.agentRepo == nil {
		return nil, ErrNilAgentRepo
	}
	if e.projectStore == nil && e.projectRepo == nil {
		return nil, errors.New("project repository or store is required")
	}

	projectName := strings.TrimSpace(req.ProjectName)
	if projectName == "" {
		projectName = fmt.Sprintf("%s Project", tmpl.Name)
	}

	// Determine unique project ID
	projectID := fmt.Sprintf("proj_%s", generateRandomHex(6))

	// Resolve repo URL and owner/repo
	repoURL := strings.TrimSpace(req.RepoURL)
	var repoOwner, repoName string

	if repoURL != "" {
		owner, name, err := git.ParseGitHubURL(repoURL)
		if err == nil && owner != "" && name != "" {
			repoOwner = owner
			repoName = name
		} else {
			slug := slugify(projectName)
			repoOwner = "creators"
			repoName = fmt.Sprintf("%s-%s", slug, generateRandomHex(4))
			repoURL = fmt.Sprintf("https://github.com/%s/%s", repoOwner, repoName)
		}
	} else {
		slug := slugify(projectName)
		repoOwner = "creators"
		repoName = fmt.Sprintf("%s-%s", slug, generateRandomHex(4))
		repoURL = fmt.Sprintf("https://github.com/%s/%s", repoOwner, repoName)
	}

	defaultBranch := strings.TrimSpace(req.DefaultBranch)
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	// Prepare metadata
	metaMap := map[string]interface{}{
		"blueprint_id":    tmpl.ID,
		"blueprint_name":  tmpl.Name,
		"category":        tmpl.Category,
		"icon":            tmpl.Icon,
		"pipeline_tasks":  len(tmpl.Tasks),
		"instantiated_at": time.Now().UTC().Format(time.RFC3339),
	}
	if len(req.Metadata) > 0 {
		var userMeta map[string]interface{}
		if err := json.Unmarshal(req.Metadata, &userMeta); err == nil {
			for k, v := range userMeta {
				metaMap[k] = v
			}
		}
	}
	metaBytes, err := json.Marshal(metaMap)
	if err != nil {
		metaBytes = []byte("{}")
	}

	now := time.Now().UTC()
	projEntity := &project.Project{
		ID:            projectID,
		Name:          projectName,
		RepoURL:       repoURL,
		RepoOwner:     repoOwner,
		RepoName:      repoName,
		DefaultBranch: defaultBranch,
		CloneStatus:   "connected",
		ProjectType:   tmpl.ID,
		Metadata:      metaBytes,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// 1. Create Project
	if e.projectStore != nil {
		if err := e.projectStore.CreateProject(ctx, projEntity); err != nil {
			return nil, fmt.Errorf("create project: %w", err)
		}
	}
	if e.projectRepo != nil && (e.projectStore == nil || isMemoryProjectRepo(e.projectRepo)) {
		dbProj := toDBProject(projEntity)
		_ = e.projectRepo.CreateProject(ctx, dbProj)
	}

	// Rollback helper
	rollback := func(createdTasks []string, createdRoomID string) {
		for _, tid := range createdTasks {
			_ = e.studioRepo.DeleteTask(ctx, tid)
		}
		if createdRoomID != "" {
			_ = e.studioRepo.DeleteRoom(ctx, createdRoomID)
		}
		if e.projectStore != nil {
			_ = e.projectStore.DeleteProject(ctx, projectID)
		}
		if e.projectRepo != nil {
			_ = e.projectRepo.DeleteProject(ctx, projectID)
		}
	}

	// 2. Create Collaboration Room
	roomID := db.GenerateUUID()
	room := &db.ProjectRoom{
		ID:        roomID,
		ProjectID: projectID,
		Title:     tmpl.Room.Title,
		Topic:     tmpl.Room.Topic,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := e.studioRepo.CreateRoom(ctx, room); err != nil {
		rollback(nil, "")
		return nil, fmt.Errorf("create blueprint room: %w", err)
	}

	// 3. Resolve or Create Agent Persona
	var targetAgent *db.Agent
	existingAgents, err := e.agentRepo.ListAgents(ctx, db.AgentFilter{Search: tmpl.Agent.Name})
	if err == nil {
		for i := range existingAgents {
			if strings.EqualFold(existingAgents[i].Name, tmpl.Agent.Name) {
				targetAgent = &existingAgents[i]
				break
			}
		}
	}

	if targetAgent == nil {
		newAgent := &db.Agent{
			ID:           db.GenerateUUID(),
			Name:         tmpl.Agent.Name,
			Avatar:       tmpl.Agent.Avatar,
			Role:         tmpl.Agent.Role,
			SystemPrompt: tmpl.Agent.SystemPrompt,
			ModelID:      tmpl.Agent.ModelID,
			Provider:     tmpl.Agent.Provider,
			Status:       db.AgentStatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := e.agentRepo.CreateAgent(ctx, newAgent); err != nil {
			// If creation fails due to duplicate name in concurrent calls, retry fetch
			retryAgents, rErr := e.agentRepo.ListAgents(ctx, db.AgentFilter{Search: tmpl.Agent.Name})
			if rErr == nil && len(retryAgents) > 0 {
				targetAgent = &retryAgents[0]
			} else {
				rollback(nil, roomID)
				return nil, fmt.Errorf("create blueprint agent persona: %w", err)
			}
		} else {
			targetAgent = newAgent
		}
	}

	// 4. Create Pipeline Tasks with DoD Checklist
	createdTaskIDs := make([]string, 0, len(tmpl.Tasks))
	createdTasks := make([]db.ProjectTaskDTO, 0, len(tmpl.Tasks))

	for _, taskTmpl := range tmpl.Tasks {
		taskID := db.GenerateUUID()
		dodCopy := make([]db.DoDItem, len(taskTmpl.DoDChecklist))
		for i, item := range taskTmpl.DoDChecklist {
			dodCopy[i] = db.DoDItem{
				ID:        db.GenerateUUID(),
				Text:      item.Text,
				Completed: item.Completed,
			}
		}

		tagsCopy := make([]string, len(taskTmpl.Tags))
		copy(tagsCopy, taskTmpl.Tags)

		agentIDStr := targetAgent.ID
		roomIDStr := roomID

		taskEntity := &db.ProjectTask{
			ID:              taskID,
			ProjectID:       projectID,
			RoomID:          &roomIDStr,
			Title:           taskTmpl.Title,
			Description:     taskTmpl.Description,
			Status:          "backlog",
			Priority:        taskTmpl.Priority,
			DoDChecklist:    dodCopy,
			Tags:            tagsCopy,
			AssigneeID:      &agentIDStr,
			ExecutionStatus: "idle",
			AgentOutput:     "",
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		if err := e.studioRepo.CreateTask(ctx, taskEntity); err != nil {
			rollback(createdTaskIDs, roomID)
			return nil, fmt.Errorf("create pipeline task '%s': %w", taskTmpl.Title, err)
		}

		createdTaskIDs = append(createdTaskIDs, taskID)
		createdTasks = append(createdTasks, taskEntity.ToDTO())
	}

	// 5. Post Initial Welcome Message to Room
	welcomeContent := fmt.Sprintf(
		"Welcome to **%s**! 🎉\n\nThis project was instantiated from the **%s** blueprint.\n- **Room**: %s (%s)\n- **Pipeline Tasks**: %d structured tasks loaded with full DoD checklists\n- **Assigned Agent**: @%s (%s)\n\nYou can now run tasks with the agent or coordinate here in the studio.",
		projectName,
		tmpl.Name,
		tmpl.Room.Title,
		tmpl.Room.Topic,
		len(createdTasks),
		targetAgent.Name,
		targetAgent.Role,
	)
	welcomeMsg := &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     roomID,
		AuthorType: "system",
		AuthorID:   "system",
		AuthorName: "Cuan Studio",
		Content:    welcomeContent,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	_ = e.studioRepo.CreateMessage(ctx, welcomeMsg)

	return &InstantiateResponse{
		Status:     "success",
		TemplateID: tmpl.ID,
		Project:    projEntity.ToResponse(),
		Room:       room.ToDTO(),
		Agent:      targetAgent.ToDTO(),
		Tasks:      createdTasks,
	}, nil
}

func generateRandomHex(byteCount int) string {
	b := make([]byte, byteCount)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())[:byteCount*2]
	}
	return hex.EncodeToString(b)
}

func slugify(s string) string {
	clean := strings.ToLower(strings.TrimSpace(s))
	clean = nonAlphaNumRegex.ReplaceAllString(clean, "-")
	clean = strings.Trim(clean, "-")
	if clean == "" {
		return "creator-project"
	}
	return clean
}

func toDBProject(p *project.Project) *db.Project {
	if p == nil {
		return nil
	}
	return &db.Project{
		ID:             p.ID,
		Name:           p.Name,
		RepoURL:        p.RepoURL,
		RepoOwner:      p.RepoOwner,
		RepoName:       p.RepoName,
		DefaultBranch:  p.DefaultBranch,
		LocalPath:      p.LocalPath,
		EncryptedToken: p.EncryptedToken,
		TokenPreview:   p.TokenPreview,
		CloneStatus:    p.CloneStatus,
		ProjectType:    p.ProjectType,
		Metadata:       p.Metadata,
		LastFetchedAt:  p.LastFetchedAt,
		LastError:      p.LastError,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

func isMemoryProjectRepo(r db.ProjectRepository) bool {
	_, ok := r.(*db.MemoryProjectRepository)
	return ok
}
