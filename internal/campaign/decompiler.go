package campaign

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/db"
)

// DecompileResult holds the output of the fan-out decompiler engine.
type DecompileResult struct {
	CampaignID     string            `json:"campaign_id"`
	BatchID        string            `json:"batch_id"`
	ParentTaskID   *string           `json:"parent_task_id,omitempty"`
	ParentTask     *db.ProjectTask   `json:"parent_task,omitempty"`
	TargetProjects []string          `json:"target_projects"`
	Subtasks       []*db.ProjectTask `json:"subtasks"`
	GeneratedAt    time.Time         `json:"generated_at"`
}

// DecompilerOption configures the DecompilerEngine.
type DecompilerOption func(*DecompilerEngine)

// WithProjectResolver configures a custom ProjectResolver.
func WithProjectResolver(resolver ProjectResolver) DecompilerOption {
	return func(e *DecompilerEngine) {
		e.resolver = resolver
	}
}

// WithProjectStudioRepo configures a ProjectStudioRepository for persisting subtasks.
func WithProjectStudioRepo(repo db.ProjectStudioRepository) DecompilerOption {
	return func(e *DecompilerEngine) {
		e.studioRepo = repo
	}
}

// WithMinDoDItems sets the minimum number of verifiable DoD checklist items (default 3).
func WithMinDoDItems(min int) DecompilerOption {
	return func(e *DecompilerEngine) {
		if min > 0 {
			e.minDoDItems = min
		}
	}
}

// DecompilerEngine transforms high-level parent campaigns into isolated subtasks per target project.
type DecompilerEngine struct {
	resolver    ProjectResolver
	studioRepo  db.ProjectStudioRepository
	minDoDItems int
	mu          sync.RWMutex
}

// NewDecompilerEngine creates a new DecompilerEngine instance with sensible defaults.
func NewDecompilerEngine(opts ...DecompilerOption) *DecompilerEngine {
	e := &DecompilerEngine{
		minDoDItems: 3,
	}

	for _, opt := range opts {
		opt(e)
	}

	if e.resolver == nil {
		e.resolver = NewDefaultProjectResolver(nil, e.studioRepo)
	}

	return e
}

// SetProjectResolver updates the project resolver.
func (e *DecompilerEngine) SetProjectResolver(resolver ProjectResolver) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.resolver = resolver
}

// SetStudioRepo updates the project studio repository.
func (e *DecompilerEngine) SetStudioRepo(repo db.ProjectStudioRepository) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.studioRepo = repo
}

// GenerateDoDChecklist guarantees that the returned checklist has at least minDoDItems (>= 3) verifiable criteria.
func (e *DecompilerEngine) GenerateDoDChecklist(templateItems []db.DoDItem, projectName string) []db.DoDItem {
	e.mu.RLock()
	minItems := e.minDoDItems
	e.mu.RUnlock()

	items := make([]db.DoDItem, 0, len(templateItems)+minItems)
	seen := make(map[string]bool)

	// 1. Add provided template items
	for _, it := range templateItems {
		cleanText := strings.TrimSpace(it.Text)
		if cleanText == "" {
			continue
		}
		// Variable substitution in checklist text
		substituted := strings.ReplaceAll(cleanText, "{{project_name}}", projectName)
		key := strings.ToLower(substituted)
		if !seen[key] {
			itID := it.ID
			if itID == "" {
				itID = db.GenerateUUID()
			}
			items = append(items, db.DoDItem{
				ID:        itID,
				Text:      substituted,
				Completed: it.Completed,
			})
			seen[key] = true
		}
	}

	// 2. Fallback pool of verifiable criteria templates
	defaultVerifiablePool := []string{
		fmt.Sprintf("Target project '%s' environment and dependencies verified", projectName),
		fmt.Sprintf("Campaign payload and script changes applied with valid syntax on '%s'", projectName),
		fmt.Sprintf("Automated targeted validation test passes 100%% without regression on '%s'", projectName),
		fmt.Sprintf("Smoke test and live HTTP readiness check confirmed OK on '%s'", projectName),
	}

	// 3. Fill up to minItems if needed
	for _, fallback := range defaultVerifiablePool {
		if len(items) >= minItems {
			break
		}
		key := strings.ToLower(fallback)
		if !seen[key] {
			items = append(items, db.DoDItem{
				ID:        db.GenerateUUID(),
				Text:      fallback,
				Completed: false,
			})
			seen[key] = true
		}
	}

	return items
}

// Decompile parses the campaign and generates in-memory structured subtasks without persisting them.
func (e *DecompilerEngine) Decompile(ctx context.Context, campaign *db.BatchCampaign) (*DecompileResult, error) {
	if campaign == nil {
		return nil, ErrNilCampaign
	}

	e.mu.RLock()
	resolver := e.resolver
	e.mu.RUnlock()

	if resolver == nil {
		return nil, fmt.Errorf("project resolver is not configured")
	}

	// 1. Parse TargetCriteria
	criteria, err := ParseTargetCriteria(campaign.TargetCriteria)
	if err != nil {
		return nil, fmt.Errorf("invalid target criteria: %w", err)
	}

	// 2. Resolve Target Projects
	projects, err := resolver.ResolveProjects(ctx, *criteria)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve target projects: %w", err)
	}
	if len(projects) == 0 {
		return nil, ErrNoTargetProjectsFound
	}

	// 3. Parse TemplatePayload
	template, err := ParseTemplatePayload(campaign.TemplatePayload)
	if err != nil {
		return nil, fmt.Errorf("invalid template payload: %w", err)
	}

	// 4. Determine Common Batch Attributes
	now := time.Now().UTC()
	batchID := campaign.ID
	if batchID == "" {
		batchID = db.GenerateUUID()
	}

	// Aggregate and deduplicate tags
	tagsMap := make(map[string]bool)
	for _, tg := range criteria.Tags {
		norm := strings.ToLower(strings.TrimSpace(tg))
		if norm != "" {
			tagsMap[norm] = true
		}
	}
	for _, tg := range template.Tags {
		norm := strings.ToLower(strings.TrimSpace(tg))
		if norm != "" {
			tagsMap[norm] = true
		}
	}

	var mergedTags []string
	for tg := range tagsMap {
		mergedTags = append(mergedTags, tg)
	}
	sort.Strings(mergedTags)

	// Status and priority defaults
	priority := "normal"
	if template.Priority != "" {
		if normPrio, err := db.NormalizeTaskPriority(template.Priority); err == nil {
			priority = normPrio
		}
	}

	// 5. Generate subtasks per target project
	targetProjectIDs := make([]string, len(projects))
	subtasks := make([]*db.ProjectTask, 0, len(projects))

	for i, proj := range projects {
		targetProjectIDs[i] = proj.ID

		// Subtask Title
		title := fmt.Sprintf("[%s] %s", proj.Name, campaign.Title)
		if template.TitleTemplate != "" {
			t := strings.ReplaceAll(template.TitleTemplate, "{{project_name}}", proj.Name)
			t = strings.ReplaceAll(t, "{{project_id}}", proj.ID)
			title = strings.TrimSpace(t)
		}

		// Subtask Description
		var descBuilder strings.Builder
		if campaign.Description != "" {
			descBuilder.WriteString(campaign.Description)
			descBuilder.WriteString("\n\n")
		}
		if template.Instructions != "" {
			descBuilder.WriteString("### Instructions\n")
			descBuilder.WriteString(template.Instructions)
			descBuilder.WriteString("\n\n")
		}
		if template.Script != "" {
			descBuilder.WriteString("### Script / Payload\n```\n")
			descBuilder.WriteString(template.Script)
			descBuilder.WriteString("\n```\n")
		}
		desc := strings.TrimSpace(descBuilder.String())
		if desc == "" {
			desc = fmt.Sprintf("Batch campaign subtask for project %s (%s)", proj.Name, proj.ID)
		}

		// Subtask Tags: project tags + campaign tags
		subtaskTagsMap := make(map[string]bool)
		for _, tg := range mergedTags {
			subtaskTagsMap[tg] = true
		}
		for _, tg := range proj.Tags {
			norm := strings.ToLower(strings.TrimSpace(tg))
			if norm != "" {
				subtaskTagsMap[norm] = true
			}
		}
		var subtaskTags []string
		for tg := range subtaskTagsMap {
			subtaskTags = append(subtaskTags, tg)
		}
		sort.Strings(subtaskTags)

		// Guaranteed minimal >= 3 verifiable DoD items
		dodChecklist := e.GenerateDoDChecklist(template.DoDChecklist, proj.Name)

		subtask := &db.ProjectTask{
			ID:           db.GenerateUUID(),
			ProjectID:    proj.ID,
			BatchID:      &batchID,
			ParentTaskID: template.ParentTaskID,
			Title:        title,
			Description:  desc,
			Status:       "backlog",
			Priority:     priority,
			AssigneeID:   template.AssigneeID,
			DoDChecklist: dodChecklist,
			Tags:         subtaskTags,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		subtasks = append(subtasks, subtask)
	}

	return &DecompileResult{
		CampaignID:     campaign.ID,
		BatchID:        batchID,
		ParentTaskID:   template.ParentTaskID,
		TargetProjects: targetProjectIDs,
		Subtasks:       subtasks,
		GeneratedAt:    now,
	}, nil
}

// DecompileAndPersist decompiles the campaign and saves the generated subtasks (and optional parent epic) into db.ProjectStudioRepository.
func (e *DecompilerEngine) DecompileAndPersist(ctx context.Context, campaign *db.BatchCampaign) (*DecompileResult, error) {
	e.mu.RLock()
	studioRepo := e.studioRepo
	e.mu.RUnlock()

	if studioRepo == nil {
		return nil, fmt.Errorf("project studio repository is required for persistence")
	}

	result, err := e.Decompile(ctx, campaign)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	// If no parent_task_id was provided, optionally create a parent task for the campaign
	// anchored to the first target project, so foreign key constraints are fully satisfied.
	if result.ParentTaskID == nil && len(result.Subtasks) > 0 {
		parentDoD := e.GenerateDoDChecklist([]db.DoDItem{
			{Text: fmt.Sprintf("All %d child subtasks generated and tracked under batch %s", len(result.Subtasks), result.BatchID)},
			{Text: "Worker pool execution completed with zero blocking"},
			{Text: "Rollup verification and status confirmed"},
		}, campaign.Title)

		parentTask := &db.ProjectTask{
			ID:           db.GenerateUUID(),
			ProjectID:    result.Subtasks[0].ProjectID,
			BatchID:      &result.BatchID,
			Title:        fmt.Sprintf("[Campaign Epic] %s", campaign.Title),
			Description:  campaign.Description,
			Status:       "doing",
			Priority:     "high",
			DoDChecklist: parentDoD,
			Tags:         []string{"campaign", "epic"},
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if err := studioRepo.CreateTask(ctx, parentTask); err == nil {
			result.ParentTask = parentTask
			result.ParentTaskID = &parentTask.ID
			// Set parent task ID on all generated subtasks
			for _, st := range result.Subtasks {
				st.ParentTaskID = &parentTask.ID
			}
		}
	}

	// Persist each subtask
	for _, st := range result.Subtasks {
		if err := studioRepo.CreateTask(ctx, st); err != nil {
			return nil, fmt.Errorf("failed to persist subtask for project %s: %w", st.ProjectID, err)
		}
	}

	return result, nil
}
