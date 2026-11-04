package task

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/buchin/cuan/internal/project"
)

type PageOptions struct {
	ProjectID, Search, Status, Priority, Sort string
	Page, PageSize                            int
	ExcludeTerminals                          bool
}
type TaskPage struct {
	Tasks      []*TaskItemResponse `json:"tasks"`
	Total      int                 `json:"total"`
	ScopeTotal int                 `json:"scope_total"`
	Counts     map[string]int      `json:"counts"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"page_size"`
}
type taskPageSelection struct {
	IDs               []string
	Total, ScopeTotal int
	Counts            map[string]int
}
type taskPageStore interface {
	SelectTaskPage(context.Context, PageOptions) (*taskPageSelection, error)
}

func normalizePage(o PageOptions) PageOptions {
	if o.Page < 1 {
		o.Page = 1
	}
	if o.Page > 1000000 {
		o.Page = 1000000
	}
	if o.PageSize < 1 {
		o.PageSize = 25
	}
	if o.PageSize > 100 {
		o.PageSize = 100
	}
	if o.Status == "all" {
		o.Status = ""
	}
	if o.Priority == "all" {
		o.Priority = ""
	}
	if o.ProjectID == "all" {
		o.ProjectID = ""
	}
	o.Search = strings.TrimSpace(o.Search)
	return o
}
func (s *Service) ListTaskPage(ctx context.Context, o PageOptions) (*TaskPage, error) {
	o = normalizePage(o)
	reader, ok := s.store.(taskPageStore)
	if !ok {
		return nil, fmt.Errorf("paged task storage unavailable")
	}
	selected, err := reader.SelectTaskPage(ctx, o)
	if err != nil {
		return nil, err
	}
	result := &TaskPage{Tasks: []*TaskItemResponse{}, Total: selected.Total, ScopeTotal: selected.ScopeTotal, Counts: selected.Counts, Page: o.Page, PageSize: o.PageSize}
	last := max(1, (result.Total+o.PageSize-1)/o.PageSize)
	if result.Page > last {
		result.Page = last
	}
	if len(selected.IDs) == 0 {
		return result, nil
	}
	items, err := s.ListTasks(ctx, TaskFilter{IDs: selected.IDs})
	if err != nil {
		return nil, err
	}
	launches := map[string]*LaunchInfo{}
	if reader, ok := s.store.(launchInfoReader); ok {
		launches, err = reader.LatestLaunchInfos(ctx, selected.IDs)
		if err != nil {
			return nil, fmt.Errorf("list page launch info: %w", err)
		}
	}
	byID := map[string]*TaskItemResponse{}
	for _, t := range items {
		byID[t.ID] = t
	}
	for _, id := range selected.IDs {
		if item := byID[id]; item != nil {
			item.Launch = launches[id]
			attemptState, launchState := "", ""
			if item.LatestAttempt != nil {
				attemptState = item.LatestAttempt.Status
			}
			if item.Launch != nil {
				launchState = item.Launch.State
			}
			item.Status = EffectiveStatus(item.Status, attemptState, launchState)
			item.Instructions = taskPagePreview(project.TaskScopeInstructions(item.Instructions))
			item.Summary = taskPagePreview(item.Summary)
			item.CompleteNote = taskPagePreview(item.CompleteNote)
			item.DiffSummary = ""
			item.TestSummary = ""
			item.DoDChecklist = nil
			if item.LatestAttempt != nil {
				item.LatestAttempt.LeaseExpiresAt = nil
				item.LatestAttempt.RequestHash = ""
				item.LatestAttempt.ErrorMessage = taskPagePreview(item.LatestAttempt.ErrorMessage)
			}
			result.Tasks = append(result.Tasks, item)
		}
	}
	return result, nil
}

var pageSortSQL = map[string]string{
	"updated": "f.updated_at DESC,f.id ASC", "updated-asc": "f.updated_at ASC,f.id ASC", "created": "f.created_at DESC,f.id ASC",
	"title": "f.title ASC,f.id ASC", "title-desc": "f.title DESC,f.id ASC", "task": "f.id ASC", "id": "f.id ASC", "task-desc": "f.id DESC", "id-desc": "f.id DESC",
	"status": "f.status ASC,f.id ASC", "status-desc": "f.status DESC,f.id ASC",
	"priority":      "CASE f.priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'low' THEN 3 ELSE 2 END ASC,f.updated_at DESC,f.id ASC",
	"priority-desc": "CASE f.priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'low' THEN 3 ELSE 2 END DESC,f.updated_at DESC,f.id ASC",
}

func (s *DBStore) SelectTaskPage(ctx context.Context, o PageOptions) (*taskPageSelection, error) {
	o = normalizePage(o)
	order := pageSortSQL[o.Sort]
	if order == "" {
		order = pageSortSQL["updated"]
	}
	query := `WITH scope AS (
 SELECT t.id,t.title,t.priority,t.created_at,t.updated_at,COALESCE(p.name,'') project_name,
 ` + EffectiveStatusSQL + ` status,COALESCE(a.status,'') attempt_status,COALESCE(k.answered,false) answered
 FROM tasks t LEFT JOIN projects p ON p.id=t.project_id
 LEFT JOIN LATERAL (SELECT id,status FROM task_attempts WHERE task_id=t.id ORDER BY created_at DESC,id DESC LIMIT 1) a ON true
 LEFT JOIN LATERAL (SELECT state FROM task_launches WHERE task_id=t.id ORDER BY created_at DESC,id DESC LIMIT 1) l ON true
 LEFT JOIN LATERAL (
 SELECT kind.new_status='answer' answered FROM
 (SELECT attempt_id,created_at FROM task_activities WHERE task_id=t.id AND activity_type='agent_report' AND new_status IN('review','done','blocked') ORDER BY created_at DESC,id DESC LIMIT 1) report
 JOIN task_activities kind ON kind.task_id=t.id AND kind.attempt_id=report.attempt_id AND kind.created_at=report.created_at
 WHERE report.attempt_id=a.id AND kind.activity_type='agent_result_kind' AND kind.new_status IN('answer','implementation') LIMIT 1
 ) k ON true
 WHERE (NOT $7 OR t.title NOT LIKE 'Terminal ·%') AND ($1='' OR ($1='none' AND COALESCE(t.project_id,'')='' AND NOT EXISTS(SELECT 1 FROM task_projects tp WHERE tp.task_id=t.id)) OR t.project_id=$1 OR EXISTS(SELECT 1 FROM task_projects tp WHERE tp.task_id=t.id AND tp.project_id=$1))
 ), classified AS (
 SELECT *, (status IN ('doing','queued','todo','backlog')) active,
 (status IN ('review','blocked','failed','canceled','cancelled')) board_attention,
 CASE WHEN status='done' THEN 'completed'
 WHEN status IN('canceled','cancelled') THEN 'canceled'
 WHEN status IN('queued','pending') OR (status='doing' AND attempt_status='running') THEN 'active'
 WHEN status='blocked' THEN 'attention'
 WHEN status='review' THEN CASE WHEN answered AND attempt_status IN('succeeded','completed') THEN 'completed' ELSE 'attention' END
 WHEN status='doing' THEN 'attention' ELSE 'active' END work_group FROM scope
 ), filtered AS (
 SELECT * FROM classified WHERE ($2='' OR LOWER(title||' '||id||' '||project_name) LIKE '%'||LOWER($2)||'%' OR EXISTS(SELECT 1 FROM tasks source WHERE source.id=classified.id AND LOWER(source.instructions) LIKE '%'||LOWER($2)||'%'))
 AND ($3='' OR priority=$3) AND ($4='' OR status=$4 OR ($4='active' AND active) OR ($4='completed' AND work_group='completed') OR ($4='work_active' AND work_group='active') OR ($4='attention' AND work_group='attention') OR ($4='selesai' AND status='done') OR ($4 IN ('dibatalkan_review','dibatalkan','review_or_blocked') AND board_attention))
 ), totals AS (SELECT COUNT(*) n FROM filtered), page AS (
 SELECT f.id,ROW_NUMBER() OVER (ORDER BY ` + order + `) ordinal FROM filtered f ORDER BY ` + order + `
 LIMIT $5 OFFSET LEAST(($6::bigint-1)*$5,GREATEST(((SELECT n FROM totals)-1)/$5*$5,0))
 ) SELECT (SELECT n FROM totals),(SELECT COUNT(*) FROM scope),
 COALESCE((SELECT jsonb_object_agg(status,n) FROM(SELECT status,COUNT(*) n FROM scope GROUP BY status) c),'{}'::jsonb)||
 jsonb_build_object('all',(SELECT COUNT(*) FROM scope),'active',(SELECT COUNT(*) FROM classified WHERE active),'work_active',(SELECT COUNT(*) FROM classified WHERE work_group='active'),'completed',(SELECT COUNT(*) FROM classified WHERE work_group='completed'),'attention',(SELECT COUNT(*) FROM classified WHERE work_group='attention'),'review_or_blocked',(SELECT COUNT(*) FROM classified WHERE board_attention)),
 COALESCE((SELECT jsonb_agg(id ORDER BY ordinal) FROM page),'[]'::jsonb)`
	result := &taskPageSelection{}
	var counts, ids []byte
	if err := s.db.QueryRowContext(ctx, query, o.ProjectID, strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(strings.TrimPrefix(o.Search, "#")), o.Priority, o.Status, o.PageSize, o.Page, o.ExcludeTerminals).Scan(&result.Total, &result.ScopeTotal, &counts, &ids); err != nil {
		return nil, fmt.Errorf("select task page: %w", err)
	}
	if err := json.Unmarshal(counts, &result.Counts); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(ids, &result.IDs); err != nil {
		return nil, err
	}
	return result, nil
}
func (m *MemoryStore) SelectTaskPage(ctx context.Context, o PageOptions) (*taskPageSelection, error) {
	o = normalizePage(o)
	filter := TaskFilter{ProjectID: o.ProjectID}
	if o.ProjectID == "none" {
		filter.ProjectID = ""
	}
	tasks, err := m.ListTasks(ctx, filter)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, t := range tasks {
		ids = append(ids, t.ID)
	}
	attempts, err := m.LatestAttempts(ctx, ids)
	if err != nil {
		return nil, err
	}
	classifications, err := m.LatestResultClassifications(ctx, ids)
	if err != nil {
		return nil, err
	}
	launches, err := m.LatestLaunchInfos(ctx, ids)
	if err != nil {
		return nil, err
	}
	statuses := map[string]string{}
	result := &taskPageSelection{IDs: []string{}, Counts: map[string]int{}}
	var rows []*Task
	for _, t := range tasks {
		if o.ExcludeTerminals && strings.HasPrefix(t.Title, "Terminal ·") {
			continue
		}
		if o.ProjectID == "none" && (t.ProjectID != "" || len(t.ProjectIDs) > 0) {
			continue
		}
		result.ScopeTotal++
		status := t.Status
		if status == "failed" {
			status = "blocked"
		}
		attemptStatus := ""
		if a := attempts[t.ID]; a != nil {
			attemptStatus = a.Status
		}
		launchState := ""
		if launch := launches[t.ID]; launch != nil {
			launchState = launch.State
		}
		status = EffectiveStatus(status, attemptStatus, launchState)
		statuses[t.ID] = status
		active := oneOf(status, "doing", "queued", "todo", "backlog")
		attention := oneOf(status, "review", "blocked", "failed", "canceled", "cancelled")
		result.Counts[status]++
		if active {
			result.Counts["active"]++
		}
		if attention {
			result.Counts["review_or_blocked"]++
		}
		answered := resultKindForAttempt(classifications[t.ID], attempts[t.ID]) == "answer"
		workGroup := taskPageWorkGroup(status, attemptStatus, answered)
		switch workGroup {
		case "completed":
			result.Counts["completed"]++
		case "attention":
			result.Counts["attention"]++
		case "active":
			result.Counts["work_active"]++
		}
		if o.Priority != "" && o.Priority != t.Priority {
			continue
		}
		if o.Status != "" && o.Status != status && !(o.Status == "active" && active) && !(o.Status == "completed" && workGroup == "completed") && !(o.Status == "work_active" && workGroup == "active") && !(o.Status == "attention" && workGroup == "attention") && !(o.Status == "selesai" && status == "done") && !(oneOf(o.Status, "dibatalkan_review", "dibatalkan", "review_or_blocked") && attention) {
			continue
		}
		if o.Search != "" && !strings.Contains(strings.ToLower(t.Title+" "+t.ID+" "+t.Instructions), strings.ToLower(strings.TrimPrefix(o.Search, "#"))) {
			continue
		}
		rows = append(rows, t)
	}
	result.Counts["all"] = result.ScopeTotal
	result.Total = len(rows)
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		cmp := 0
		switch o.Sort {
		case "title", "title-desc":
			cmp = strings.Compare(a.Title, b.Title)
		case "task", "id", "task-desc", "id-desc":
			cmp = strings.Compare(a.ID, b.ID)
		case "status", "status-desc":
			as, bs := statuses[a.ID], statuses[b.ID]
			if as == "failed" {
				as = "blocked"
			}
			if bs == "failed" {
				bs = "blocked"
			}
			cmp = strings.Compare(as, bs)
		case "priority", "priority-desc":
			rank := func(p string) int {
				switch p {
				case "urgent":
					return 0
				case "high":
					return 1
				case "low":
					return 3
				}
				return 2
			}
			cmp = rank(a.Priority) - rank(b.Priority)
		case "created":
			if !a.CreatedAt.Equal(b.CreatedAt) {
				return a.CreatedAt.After(b.CreatedAt)
			}
		}
		if cmp != 0 {
			if strings.HasSuffix(o.Sort, "-desc") {
				return cmp > 0
			}
			return cmp < 0
		}
		if o.Sort == "updated-asc" {
			if !a.UpdatedAt.Equal(b.UpdatedAt) {
				return a.UpdatedAt.Before(b.UpdatedAt)
			}
		} else if !oneOf(o.Sort, "title", "title-desc", "task", "id", "task-desc", "id-desc", "status", "status-desc", "created") && !a.UpdatedAt.Equal(b.UpdatedAt) {
			return a.UpdatedAt.After(b.UpdatedAt)
		}
		return a.ID < b.ID
	})
	start := min((o.Page-1)*o.PageSize, max(0, (len(rows)-1)/o.PageSize*o.PageSize))
	for _, t := range rows[start:min(start+o.PageSize, len(rows))] {
		result.IDs = append(result.IDs, t.ID)
	}
	return result, nil
}
func oneOf(value string, options ...string) bool {
	for _, option := range options {
		if value == option {
			return true
		}
	}
	return false
}

func taskPagePreview(text string) string {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if len(runes) > 240 {
		return string(runes[:240]) + "…"
	}
	return text
}

func taskPageWorkGroup(status, attempt string, answered bool) string {
	if status == "done" {
		return "completed"
	}
	if oneOf(status, "canceled", "cancelled") {
		return "canceled"
	}
	if oneOf(status, "queued", "pending") || (status == "doing" && attempt == "running") {
		return "active"
	}
	if status == "blocked" {
		return "attention"
	}
	if status == "review" {
		if answered && oneOf(attempt, "succeeded", "completed") {
			return "completed"
		}
		return "attention"
	}
	if status == "doing" {
		return "attention"
	}
	return "active"
}
