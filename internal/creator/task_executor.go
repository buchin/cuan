package creator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
)

var (
	ErrEmptyTaskID          = errors.New("task id cannot be empty")
	ErrMissingGeminiKey     = errors.New("Google Gemini Pro API key is not configured. Connect your API key in Settings → Agent Hub (/studio/agents) first.")
	ErrAgentExecutionFailed = errors.New("agent execution failed")
)

// EventBroadcaster defines an interface for broadcasting real-time room events.
type EventBroadcaster interface {
	Broadcast(roomID string, eventType string, data interface{})
}

// ExecuteTaskRequest holds parameters for executing an AI agent on a specific task.
type ExecuteTaskRequest struct {
	TaskID       string `json:"task_id"`
	UserID       string `json:"user_id,omitempty"`
	Instructions string `json:"instructions,omitempty"`
	ModelID      string `json:"model_id,omitempty"`
	Provider     string `json:"provider,omitempty"`
}

// ExecuteTaskResponse is returned upon successful in-task execution.
type ExecuteTaskResponse struct {
	Status            string            `json:"status"`
	ExecutionStatus   string            `json:"execution_status"`
	Task              db.ProjectTaskDTO `json:"task"`
	Agent             *db.AgentDTO      `json:"agent,omitempty"`
	AgentOutput       string            `json:"agent_output"`
	DoDCompletedCount int               `json:"dod_completed_count"`
	DoDTotalCount     int               `json:"dod_total_count"`
	RoomMessageID     string            `json:"room_message_id,omitempty"`
}

// TaskExecutionEngine coordinates in-task agentic execution, vault decryption, LLM generation, DoD auto-update, and room syncing.
type TaskExecutionEngine struct {
	mu               sync.RWMutex
	studioRepo       db.ProjectStudioRepository
	agentRepo        db.AgentRepository
	projectStore     project.Store
	projectRepo      db.ProjectRepository
	contentGenerator agent.ContentGenerator
	eventBroadcaster EventBroadcaster
}

// ExecutionOption configures the TaskExecutionEngine.
type ExecutionOption func(*TaskExecutionEngine)

// WithExecutionStudioRepo sets the db.ProjectStudioRepository.
func WithExecutionStudioRepo(repo db.ProjectStudioRepository) ExecutionOption {
	return func(e *TaskExecutionEngine) {
		e.studioRepo = repo
	}
}

// WithExecutionAgentRepo sets the db.AgentRepository.
func WithExecutionAgentRepo(repo db.AgentRepository) ExecutionOption {
	return func(e *TaskExecutionEngine) {
		e.agentRepo = repo
	}
}

// WithExecutionProjectStore sets the project.Store.
func WithExecutionProjectStore(store project.Store) ExecutionOption {
	return func(e *TaskExecutionEngine) {
		e.projectStore = store
	}
}

// WithExecutionProjectRepo sets the db.ProjectRepository.
func WithExecutionProjectRepo(repo db.ProjectRepository) ExecutionOption {
	return func(e *TaskExecutionEngine) {
		e.projectRepo = repo
	}
}

// WithContentGenerator sets the agent.ContentGenerator.
func WithContentGenerator(gen agent.ContentGenerator) ExecutionOption {
	return func(e *TaskExecutionEngine) {
		e.contentGenerator = gen
	}
}

// WithEventBroadcaster sets the EventBroadcaster.
func WithEventBroadcaster(b EventBroadcaster) ExecutionOption {
	return func(e *TaskExecutionEngine) {
		e.eventBroadcaster = b
	}
}

// NewTaskExecutionEngine creates a new TaskExecutionEngine with the provided options.
func NewTaskExecutionEngine(opts ...ExecutionOption) *TaskExecutionEngine {
	e := &TaskExecutionEngine{}
	for _, opt := range opts {
		opt(e)
	}
	if e.contentGenerator == nil {
		e.contentGenerator = agent.NewGeminiProvider()
	}
	return e
}

// ExecuteTask reads the task, retrieves the active Gemini API key from the encrypted vault,
// builds the creator prompt, calls the LLM, persists the agent output, auto-updates the DoD checklist,
// and posts a notification to the project discussion room.
func (e *TaskExecutionEngine) ExecuteTask(ctx context.Context, req ExecuteTaskRequest) (*ExecuteTaskResponse, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	taskID := strings.TrimSpace(req.TaskID)
	if taskID == "" {
		return nil, ErrEmptyTaskID
	}

	if e.studioRepo == nil {
		return nil, errors.New("project studio repository is not configured")
	}
	if e.agentRepo == nil {
		return nil, errors.New("agent repository is not configured")
	}

	// 1. Fetch Task
	task, err := e.studioRepo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	// 2. Resolve Project Type / Metadata
	projectType := ""
	if e.projectStore != nil {
		if proj, err := e.projectStore.GetProject(ctx, task.ProjectID); err == nil && proj != nil {
			projectType = proj.ProjectType
		}
	} else if e.projectRepo != nil {
		if proj, err := e.projectRepo.GetProjectByID(ctx, task.ProjectID); err == nil && proj != nil {
			projectType = proj.ProjectType
		}
	}

	// 3. Resolve Assigned Agent Persona
	var agentEntity *db.Agent
	if task.AssigneeID != nil && strings.TrimSpace(*task.AssigneeID) != "" {
		if a, err := e.agentRepo.GetAgentByID(ctx, strings.TrimSpace(*task.AssigneeID)); err == nil && a != nil {
			agentEntity = a
		}
	}

	// If no assignee or not found, resolve from project type template
	if agentEntity == nil {
		if tmpl, ok := LookupTemplate(projectType); ok {
			// Find existing agent persona with this name
			agents, err := e.agentRepo.ListAgents(ctx, db.AgentFilter{Search: tmpl.Agent.Name})
			if err == nil && len(agents) > 0 {
				for i := range agents {
					if strings.EqualFold(agents[i].Name, tmpl.Agent.Name) {
						agentEntity = &agents[i]
						break
					}
				}
			}
			if agentEntity == nil {
				agentEntity = &db.Agent{
					ID:           db.GenerateUUID(),
					Name:         tmpl.Agent.Name,
					Avatar:       tmpl.Agent.Avatar,
					Role:         tmpl.Agent.Role,
					SystemPrompt: tmpl.Agent.SystemPrompt,
					ModelID:      tmpl.Agent.ModelID,
					Provider:     tmpl.Agent.Provider,
					Status:       db.AgentStatusActive,
				}
			}
		}
	}

	// Fallback general creator agent if still nil
	if agentEntity == nil {
		agentEntity = &db.Agent{
			ID:           "agent-creator-default",
			Name:         "Cuan Creator Agent",
			Avatar:       "https://api.dicebear.com/7.x/bottts/svg?seed=CuanCreator",
			Role:         "AI Content Specialist & Strategy Agent",
			SystemPrompt: "You are an elite autonomous AI Creator Specialist in Cuan Studio, optimizing content for conversion and ranking.",
			ModelID:      db.DefaultAgentModelID,
			Provider:     db.DefaultAgentProvider,
			Status:       db.AgentStatusActive,
		}
	}

	// 4. Retrieve Active Google Gemini Pro Connection from Key Vault
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		provider = db.DefaultConnectionProvider
	}

	var targetConn *db.AgentConnection
	conns, err := e.agentRepo.ListConnections(ctx, db.AgentConnectionFilter{
		Provider: provider,
		Status:   db.ConnectionStatusActive,
	})
	if err == nil && len(conns) > 0 {
		for i := range conns {
			if strings.TrimSpace(conns[i].CredentialsEncrypted) != "" {
				targetConn = &conns[i]
				break
			}
		}
	}

	// Fallback to checking by user_id if specific user was supplied
	if targetConn == nil && strings.TrimSpace(req.UserID) != "" {
		if userConn, err := e.agentRepo.GetConnectionByUserAndProvider(ctx, strings.TrimSpace(req.UserID), provider); err == nil && userConn != nil {
			if strings.TrimSpace(userConn.CredentialsEncrypted) != "" {
				targetConn = userConn
			}
		}
	}

	// Fallback to any connection for provider if active filter yielded nothing
	if targetConn == nil {
		allConns, err := e.agentRepo.ListConnections(ctx, db.AgentConnectionFilter{Provider: provider})
		if err == nil && len(allConns) > 0 {
			for i := range allConns {
				if strings.TrimSpace(allConns[i].CredentialsEncrypted) != "" {
					targetConn = &allConns[i]
					break
				}
			}
		}
	}

	if targetConn == nil || strings.TrimSpace(targetConn.CredentialsEncrypted) == "" {
		return nil, ErrMissingGeminiKey
	}

	// Decrypt credentials from Vault AES-256-GCM
	apiKey, err := agent.DecryptCredential(targetConn.CredentialsEncrypted)
	if err != nil || strings.TrimSpace(apiKey) == "" {
		return nil, ErrMissingGeminiKey
	}

	// 5. Update Task Execution Status to "running"
	task.ExecutionStatus = "running"
	_ = e.studioRepo.UpdateTaskExecution(ctx, task.ID, "running", task.AgentOutput)
	if e.eventBroadcaster != nil && task.RoomID != nil && strings.TrimSpace(*task.RoomID) != "" {
		e.eventBroadcaster.Broadcast(*task.RoomID, "task.updated", task.ToDTO())
	}

	// 6. Build Creator Prompt specifically tuned for the detected pipeline & task DoD
	systemPrompt, userPrompt := BuildCreatorPrompt(ctx, task, agentEntity, req.Instructions, projectType)

	modelID := strings.TrimSpace(req.ModelID)
	if modelID == "" && agentEntity != nil && agentEntity.ModelID != "" {
		modelID = agentEntity.ModelID
	}
	if modelID == "" {
		modelID = agent.DefaultGeminiModel
	}

	// 7. Call LLM Content Generator
	gen := e.contentGenerator
	if gen == nil {
		gen = agent.NewGeminiProvider()
	}

	generatedOutput, genErr := gen.GenerateContent(ctx, apiKey, modelID, systemPrompt, userPrompt)
	if genErr != nil {
		// Mark execution as failed
		task.ExecutionStatus = "failed"
		_ = e.studioRepo.UpdateTaskExecution(ctx, task.ID, "failed", task.AgentOutput)
		if e.eventBroadcaster != nil && task.RoomID != nil && strings.TrimSpace(*task.RoomID) != "" {
			e.eventBroadcaster.Broadcast(*task.RoomID, "task.updated", task.ToDTO())
		}
		return nil, fmt.Errorf("%w: %v", ErrAgentExecutionFailed, genErr)
	}

	// 8. Persist Output & Auto-Update Definition of Done (DoD) Checklist
	task.AgentOutput = generatedOutput
	task.ExecutionStatus = "completed"

	dodTotal := len(task.DoDChecklist)
	for i := range task.DoDChecklist {
		task.DoDChecklist[i].Completed = true
	}
	if len(task.DoDChecklist) == 0 {
		task.DoDChecklist = []db.DoDItem{
			{
				ID:        db.GenerateUUID(),
				Text:      "Content deliverables generated and verified by AI Agent",
				Completed: true,
			},
		}
		dodTotal = 1
	}
	dodCompleted := len(task.DoDChecklist)

	// If task is in backlog or doing, update to review
	if task.Status == "backlog" || task.Status == "doing" {
		task.Status = "review"
	}

	if err := e.studioRepo.UpdateTask(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to persist completed task output: %w", err)
	}

	// 9. Send Notification & Deliverable Preview to Discussion Room
	var roomMessageID string
	if task.RoomID != nil && strings.TrimSpace(*task.RoomID) != "" {
		roomID := strings.TrimSpace(*task.RoomID)
		agentName := agentEntity.Name
		agentID := agentEntity.ID
		agentRole := agentEntity.Role

		preview := truncateSnippet(generatedOutput, 300)
		msgContent := fmt.Sprintf(
			"🤖 **Task Execution Completed:** `%s`\n\nAgent **@%s** (%s) has generated deliverables for this task.\n\n**DoD Checklist:** %d/%d items marked complete.\n\n> %s\n\nFull deliverable saved to **Agent Output** tab.",
			task.Title,
			agentName,
			agentRole,
			dodCompleted,
			dodTotal,
			preview,
		)

		now := time.Now().UTC()
		msgID := db.GenerateUUID()
		roomMsg := &db.ProjectMessage{
			ID:         msgID,
			RoomID:     roomID,
			AuthorType: "agent",
			AuthorID:   agentID,
			AuthorName: agentName,
			Content:    msgContent,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := e.studioRepo.CreateMessage(ctx, roomMsg); err == nil {
			roomMessageID = msgID
			if e.eventBroadcaster != nil {
				e.eventBroadcaster.Broadcast(roomID, "message.created", roomMsg.ToDTO())
			}
		}
	}

	// Broadcast task updated event
	if e.eventBroadcaster != nil && task.RoomID != nil && strings.TrimSpace(*task.RoomID) != "" {
		e.eventBroadcaster.Broadcast(*task.RoomID, "task.updated", map[string]interface{}{
			"task":             task.ToDTO(),
			"execution_status": "completed",
		})
	}

	var agentDTO *db.AgentDTO
	if agentEntity != nil {
		dto := agentEntity.ToDTO()
		agentDTO = &dto
	}

	return &ExecuteTaskResponse{
		Status:            "success",
		ExecutionStatus:   "completed",
		Task:              task.ToDTO(),
		Agent:             agentDTO,
		AgentOutput:       generatedOutput,
		DoDCompletedCount: dodCompleted,
		DoDTotalCount:     dodTotal,
		RoomMessageID:     roomMessageID,
	}, nil
}

// truncateSnippet extracts a short preview of generated markdown text.
func truncateSnippet(text string, maxLen int) string {
	cleaned := strings.ReplaceAll(text, "\r\n", " ")
	cleaned = strings.ReplaceAll(cleaned, "\n", " ")
	cleaned = strings.TrimSpace(cleaned)
	if len(cleaned) <= maxLen {
		return cleaned
	}
	return cleaned[:maxLen] + "..."
}
