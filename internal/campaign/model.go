package campaign

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
)

var (
	ErrCampaignNotFound      = errors.New("batch campaign not found")
	ErrNoTargetProjectsFound = errors.New("no matching target projects found for campaign")
	ErrExecutionFailed       = errors.New("campaign execution encountered failures")
	ErrNilCampaign           = errors.New("campaign cannot be nil")
)

// TargetCriteria defines criteria for selecting projects to be included in a batch rollout.
type TargetCriteria struct {
	ProjectIDs []string `json:"project_ids,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	All        bool     `json:"all,omitempty"`
	ExcludeIDs []string `json:"exclude_ids,omitempty"`
}

// ParseTargetCriteria parses a TargetCriteria struct from a json.RawMessage.
// It flexibly handles JSON objects ({"project_ids": [...], "tags": [...]}),
// JSON arrays of strings (["proj1", "proj2"] or ["tag:wordpress"]),
// and single string/object fallbacks.
func ParseTargetCriteria(raw json.RawMessage) (*TargetCriteria, error) {
	tc := &TargetCriteria{
		ProjectIDs: make([]string, 0),
		Tags:       make([]string, 0),
		ExcludeIDs: make([]string, 0),
	}

	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "{}" || trimmed == "null" {
		tc.All = true
		return tc, nil
	}

	// Try standard object format first
	if strings.HasPrefix(trimmed, "{") {
		var obj struct {
			ProjectIDs     []string `json:"project_ids"`
			TargetProjects []string `json:"target_projects"`
			ProjectID      string   `json:"project_id"`
			Tags           []string `json:"tags"`
			Tag            string   `json:"tag"`
			All            bool     `json:"all"`
			ExcludeIDs     []string `json:"exclude_ids"`
		}
		if err := json.Unmarshal(raw, &obj); err == nil {
			tc.All = obj.All

			// Collect project IDs
			seenIDs := make(map[string]bool)
			for _, id := range obj.ProjectIDs {
				tID := strings.TrimSpace(id)
				if tID != "" && !seenIDs[tID] {
					tc.ProjectIDs = append(tc.ProjectIDs, tID)
					seenIDs[tID] = true
				}
			}
			for _, id := range obj.TargetProjects {
				tID := strings.TrimSpace(id)
				if tID != "" && !seenIDs[tID] {
					tc.ProjectIDs = append(tc.ProjectIDs, tID)
					seenIDs[tID] = true
				}
			}
			if obj.ProjectID != "" {
				tID := strings.TrimSpace(obj.ProjectID)
				if !seenIDs[tID] {
					tc.ProjectIDs = append(tc.ProjectIDs, tID)
					seenIDs[tID] = true
				}
			}

			// Collect tags
			seenTags := make(map[string]bool)
			for _, tg := range obj.Tags {
				normTag := strings.ToLower(strings.TrimSpace(tg))
				if normTag != "" && !seenTags[normTag] {
					tc.Tags = append(tc.Tags, normTag)
					seenTags[normTag] = true
				}
			}
			if obj.Tag != "" {
				normTag := strings.ToLower(strings.TrimSpace(obj.Tag))
				if normTag != "" && !seenTags[normTag] {
					tc.Tags = append(tc.Tags, normTag)
					seenTags[normTag] = true
				}
			}

			// Collect exclude IDs
			for _, id := range obj.ExcludeIDs {
				tID := strings.TrimSpace(id)
				if tID != "" {
					tc.ExcludeIDs = append(tc.ExcludeIDs, tID)
				}
			}

			if len(tc.ProjectIDs) == 0 && len(tc.Tags) == 0 && !tc.All {
				tc.All = true
			}
			return tc, nil
		}
	}

	// Try array format (["proj-1", "tag:wordpress", ...])
	if strings.HasPrefix(trimmed, "[") {
		var list []string
		if err := json.Unmarshal(raw, &list); err == nil {
			seenIDs := make(map[string]bool)
			seenTags := make(map[string]bool)

			for _, item := range list {
				trimmedItem := strings.TrimSpace(item)
				if trimmedItem == "" {
					continue
				}
				if strings.HasPrefix(strings.ToLower(trimmedItem), "tag:") {
					tagVal := strings.ToLower(strings.TrimSpace(trimmedItem[4:]))
					if tagVal != "" && !seenTags[tagVal] {
						tc.Tags = append(tc.Tags, tagVal)
						seenTags[tagVal] = true
					}
				} else {
					if !seenIDs[trimmedItem] {
						tc.ProjectIDs = append(tc.ProjectIDs, trimmedItem)
						seenIDs[trimmedItem] = true
					}
				}
			}
			return tc, nil
		}
	}

	return tc, nil
}

// TemplatePayload defines blueprint configurations and scripts for generated subtasks.
type TemplatePayload struct {
	TitleTemplate string         `json:"title_template,omitempty"`
	Instructions  string         `json:"instructions,omitempty"`
	Script        string         `json:"script,omitempty"`
	Tags          []string       `json:"tags,omitempty"`
	ParentTaskID  *string        `json:"parent_task_id,omitempty"`
	Priority      string         `json:"priority,omitempty"`
	AssigneeID    *string        `json:"assignee_id,omitempty"`
	DoDChecklist  []db.DoDItem   `json:"dod_checklist,omitempty"`
	RawData       map[string]any `json:"raw_data,omitempty"`
}

// ParseTemplatePayload parses a TemplatePayload struct from a json.RawMessage.
// It supports DoDChecklist specified either as []string or []db.DoDItem.
func ParseTemplatePayload(raw json.RawMessage) (*TemplatePayload, error) {
	tp := &TemplatePayload{
		Tags:         make([]string, 0),
		DoDChecklist: make([]db.DoDItem, 0),
		RawData:      make(map[string]any),
	}

	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "{}" || trimmed == "null" {
		return tp, nil
	}

	// Helper struct for polymorphic dod_checklist
	var intermediate struct {
		TitleTemplate string          `json:"title_template"`
		Instructions  string          `json:"instructions"`
		Script        string          `json:"script"`
		Tags          []string        `json:"tags"`
		ParentTaskID  *string         `json:"parent_task_id"`
		Priority      string          `json:"priority"`
		AssigneeID    *string         `json:"assignee_id"`
		DoDChecklist  json.RawMessage `json:"dod_checklist"`
	}

	if err := json.Unmarshal(raw, &intermediate); err != nil {
		// Fallback to unstructured raw data map
		_ = json.Unmarshal(raw, &tp.RawData)
		return tp, nil
	}

	tp.TitleTemplate = intermediate.TitleTemplate
	tp.Instructions = intermediate.Instructions
	tp.Script = intermediate.Script
	tp.ParentTaskID = intermediate.ParentTaskID
	tp.Priority = intermediate.Priority
	tp.AssigneeID = intermediate.AssigneeID

	for _, tg := range intermediate.Tags {
		norm := strings.ToLower(strings.TrimSpace(tg))
		if norm != "" {
			tp.Tags = append(tp.Tags, norm)
		}
	}

	// Also store arbitrary raw data
	_ = json.Unmarshal(raw, &tp.RawData)

	// Parse DoDChecklist
	if len(intermediate.DoDChecklist) > 0 {
		dodTrimmed := strings.TrimSpace(string(intermediate.DoDChecklist))
		if strings.HasPrefix(dodTrimmed, "[") {
			// First try []db.DoDItem
			var items []db.DoDItem
			if err := json.Unmarshal(intermediate.DoDChecklist, &items); err == nil && len(items) > 0 && items[0].Text != "" {
				for _, it := range items {
					if it.ID == "" {
						it.ID = db.GenerateUUID()
					}
					tp.DoDChecklist = append(tp.DoDChecklist, it)
				}
			} else {
				// Fallback to []string
				var stringItems []string
				if err := json.Unmarshal(intermediate.DoDChecklist, &stringItems); err == nil {
					for _, s := range stringItems {
						text := strings.TrimSpace(s)
						if text != "" {
							tp.DoDChecklist = append(tp.DoDChecklist, db.DoDItem{
								ID:        db.GenerateUUID(),
								Text:      text,
								Completed: false,
							})
						}
					}
				}
			}
		}
	}

	return tp, nil
}

// ProjectInfo represents minimal metadata of a project required for targeting and task generation.
type ProjectInfo struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	RepoURL   string   `json:"repo_url,omitempty"`
	RepoOwner string   `json:"repo_owner,omitempty"`
	RepoName  string   `json:"repo_name,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

// ProjectResolver resolves target projects from criteria and database storage.
type ProjectResolver interface {
	ResolveProjects(ctx context.Context, criteria TargetCriteria) ([]ProjectInfo, error)
}

// DefaultProjectResolver implements ProjectResolver using project.Service and db.ProjectStudioRepository.
type DefaultProjectResolver struct {
	projectSvc    *project.Service
	studioRepo    db.ProjectStudioRepository
	extraProjects []ProjectInfo
	mu            sync.RWMutex
}

// NewDefaultProjectResolver creates a DefaultProjectResolver.
func NewDefaultProjectResolver(projectSvc *project.Service, studioRepo db.ProjectStudioRepository) *DefaultProjectResolver {
	return &DefaultProjectResolver{
		projectSvc:    projectSvc,
		studioRepo:    studioRepo,
		extraProjects: make([]ProjectInfo, 0),
	}
}

// AddProject registers an explicit ProjectInfo for in-memory resolution (useful for tests and mocks).
func (r *DefaultProjectResolver) AddProject(p ProjectInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.extraProjects = append(r.extraProjects, p)
}

// ResolveProjects queries projects matching the provided TargetCriteria.
func (r *DefaultProjectResolver) ResolveProjects(ctx context.Context, criteria TargetCriteria) ([]ProjectInfo, error) {
	r.mu.RLock()
	allProjects := make([]ProjectInfo, len(r.extraProjects))
	copy(allProjects, r.extraProjects)
	r.mu.RUnlock()

	// If project.Service is available, fetch projects from it
	if r.projectSvc != nil {
		svcProjects, err := r.projectSvc.List(ctx)
		if err == nil {
			for _, sp := range svcProjects {
				pInfo := ProjectInfo{
					ID:        sp.ID,
					Name:      sp.Name,
					RepoURL:   sp.RepoURL,
					RepoOwner: sp.RepoOwner,
					RepoName:  sp.RepoName,
				}
				allProjects = append(allProjects, pInfo)
			}
		}
	}

	// Filter excluded IDs
	excludedMap := make(map[string]bool)
	for _, id := range criteria.ExcludeIDs {
		excludedMap[id] = true
	}

	// Build explicit ID filter set if specified
	explicitIDsMap := make(map[string]bool)
	for _, id := range criteria.ProjectIDs {
		explicitIDsMap[id] = true
	}

	// If tags are specified, collect project IDs that contain tasks with matching tags from studioRepo
	taggedProjectIDs := make(map[string]bool)
	if r.studioRepo != nil && len(criteria.Tags) > 0 {
		for _, tag := range criteria.Tags {
			tasks, err := r.studioRepo.ListTasksByTag(ctx, tag)
			if err == nil {
				for _, t := range tasks {
					if t.ProjectID != "" {
						taggedProjectIDs[t.ProjectID] = true
					}
				}
			}
		}
	}

	var results []ProjectInfo
	seen := make(map[string]bool)

	for _, p := range allProjects {
		if excludedMap[p.ID] || seen[p.ID] {
			continue
		}

		matched := false

		// 1. Explicit ID match
		if len(explicitIDsMap) > 0 && explicitIDsMap[p.ID] {
			matched = true
		}

		// 2. Tag match
		if !matched && len(criteria.Tags) > 0 {
			// Check project's own tags
			for _, pt := range p.Tags {
				normPT := strings.ToLower(pt)
				for _, ct := range criteria.Tags {
					if normPT == ct {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}

			// Check project name / repo substring
			if !matched {
				nameLower := strings.ToLower(p.Name)
				repoLower := strings.ToLower(p.RepoName)
				for _, ct := range criteria.Tags {
					if strings.Contains(nameLower, ct) || strings.Contains(repoLower, ct) {
						matched = true
						break
					}
				}
			}

			// Check tasks tagged in studioRepo
			if !matched && taggedProjectIDs[p.ID] {
				matched = true
			}
		}

		// 3. Fallback to All if no specific filter was given
		if !matched && criteria.All && len(explicitIDsMap) == 0 && len(criteria.Tags) == 0 {
			matched = true
		}

		if matched {
			results = append(results, p)
			seen[p.ID] = true
		}
	}

	return results, nil
}
