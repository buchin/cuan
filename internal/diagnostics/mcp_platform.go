package diagnostics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/agent"
	dbPkg "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/mcp"
	"github.com/buchin/cuan/internal/project"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	platformDB         *sql.DB
	platformDBOnce     sync.Once
	platformDBDisabled bool
)

// DisablePlatformDB prevents fallback to ambient DATABASE_URL or host .env files.
func DisablePlatformDB() {
	platformDBDisabled = true
	platformDB = nil
}

// SetPlatformDB allows injecting a database connection for tests or custom environments.
func SetPlatformDB(db *sql.DB) {
	platformDB = db
}

func getPlatformDB() *sql.DB {
	if platformDBDisabled {
		return nil
	}
	platformDBOnce.Do(func() {
		if platformDB != nil {
			return
		}
		var candidates []string
		if envURL := os.Getenv("DATABASE_URL"); envURL != "" {
			candidates = append(candidates, envURL)
		}
		for _, envPath := range []string{
			"/opt/cuan/.env",
			"/opt/cuan/current/.env",
			".env",
		} {
			if data, err := os.ReadFile(envPath); err == nil {
				for _, line := range strings.Split(string(data), "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "DATABASE_URL=") {
						u := strings.Trim(strings.TrimPrefix(line, "DATABASE_URL="), "\"'")
						if u != "" {
							candidates = append(candidates, u)
						}
						break
					}
				}
			}
		}
		candidates = append(candidates, "postgres://cuan:cuan@localhost:5432/cuan?sslmode=disable")

		for _, candidate := range candidates {
			conn, err := sql.Open("pgx", candidate)
			if err != nil {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			err = conn.PingContext(ctx)
			cancel()
			if err == nil {
				platformDB = conn
				return
			}
			conn.Close()
		}
	})
	return platformDB
}

// ProjectSummary represents a project record for MCP.
type ProjectSummary struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	RepoURL       string `json:"repo_url"`
	DefaultBranch string `json:"default_branch"`
	LocalPath     string `json:"local_path,omitempty"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// ListProjects lists projects managed by Cuan.
func ListProjects(ctx context.Context, limit int) (any, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		projects, err := client.ListProjects(ctx, limit)
		if err != nil {
			return nil, fmt.Errorf("Cuan connection unavailable; project list could not be confirmed")
		}
		return map[string]any{"projects": redactProjectEnvironments(projects), "total": len(projects)}, nil
	}
	db := getPlatformDB()
	if db == nil {
		return nil, fmt.Errorf("Configure a Cuan connection before reading projects")
	}

	rows, err := db.QueryContext(ctx, "SELECT id, name, repo_url, default_branch, COALESCE(local_path, ''), created_at, updated_at FROM projects ORDER BY updated_at DESC LIMIT $1", limit)
	if err != nil {
		return nil, fmt.Errorf("query projects: %w", err)
	}
	defer rows.Close()

	var projects []ProjectSummary
	for rows.Next() {
		var p ProjectSummary
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&p.ID, &p.Name, &p.RepoURL, &p.DefaultBranch, &p.LocalPath, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		p.CreatedAt = createdAt.Format(time.RFC3339)
		p.UpdatedAt = updatedAt.Format(time.RFC3339)
		projects = append(projects, p)
	}
	if projects == nil {
		projects = []ProjectSummary{}
	}

	return map[string]any{
		"projects": projects,
		"total":    len(projects),
	}, nil
}

// GetProject returns detailed project info by ID or Name.
func GetProject(ctx context.Context, projectID string) (any, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		result, err := client.GetProject(ctx, projectID)
		if err != nil {
			return nil, fmt.Errorf("Cuan connection unavailable; project state could not be confirmed")
		}
		redactProjectEnvironment(result)
		return result, nil
	}
	db := getPlatformDB()
	if db == nil {
		return nil, fmt.Errorf("Configure a Cuan connection before reading a project")
	}

	var p ProjectSummary
	var createdAt, updatedAt time.Time
	err := db.QueryRowContext(ctx,
		"SELECT id, name, repo_url, default_branch, COALESCE(local_path, ''), created_at, updated_at FROM projects WHERE id = $1 OR LOWER(name) = LOWER($1) LIMIT 1",
		projectID,
	).Scan(&p.ID, &p.Name, &p.RepoURL, &p.DefaultBranch, &p.LocalPath, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return map[string]any{
			"error":      "project_not_found",
			"project_id": projectID,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}

	p.CreatedAt = createdAt.Format(time.RFC3339)
	p.UpdatedAt = updatedAt.Format(time.RFC3339)

	// Fetch active tasks count
	var activeTasks int
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks WHERE project_id = $1 AND status IN ('backlog', 'doing', 'review')", p.ID).Scan(&activeTasks)

	return map[string]any{
		"project":      p,
		"active_tasks": activeTasks,
	}, nil
}

// TaskSummary represents a task record for MCP.
type TaskSummary struct {
	ID           string   `json:"id"`
	ProjectID    string   `json:"project_id"`
	Title        string   `json:"title"`
	Instructions string   `json:"instructions"`
	Status       string   `json:"status"`
	Priority     string   `json:"priority"`
	Summary      string   `json:"summary,omitempty"`
	CompleteNote string   `json:"complete_note,omitempty"`
	DoDChecklist []string `json:"dod_checklist,omitempty"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

// ListTasksOptions configures filtering, pagination, and verbosity for ListTasksWithOptions.
type ListTasksOptions struct {
	ProjectID string
	Status    string
	Limit     int
	Offset    int
	Full      bool
}

// GetTaskOptions configures verbosity for GetTaskWithOptions.
type GetTaskOptions struct {
	Full bool
}

// ListTasks lists tasks with optional project and status filters using compact defaults.
func ListTasks(ctx context.Context, projectID, status string, limit int) (any, error) {
	return ListTasksWithOptions(ctx, ListTasksOptions{
		ProjectID: projectID,
		Status:    status,
		Limit:     limit,
	})
}

// ListTasksWithOptions lists tasks with optional project, status, pagination, and full-payload options.
func ListTasksWithOptions(ctx context.Context, opts ListTasksOptions) (any, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}
	// A configured Owner endpoint is authoritative. Do not replace a remote
	// failure with local database contents or a fabricated empty task board.
	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		page, err := client.ListTasksPage(ctx, opts.ProjectID, opts.Status, limit, offset)
		if err != nil {
			return nil, fmt.Errorf("Cuan connection unavailable; task list could not be confirmed")
		}
		tasks := page.Tasks
		for i, item := range tasks {
			redactTaskDiagnostic(item)
			if !opts.Full {
				tasks[i] = compactTaskListItem(item)
			}
		}
		res := map[string]any{
			"tasks":       tasks,
			"total":       len(tasks),
			"total_count": page.TotalCount,
			"limit":       limit,
			"offset":      offset,
			"has_more":    page.HasMore,
			"compact":     !opts.Full,
		}
		if page.NextOffset != nil {
			res["next_offset"] = *page.NextOffset
		}
		return res, nil
	}
	db := getPlatformDB()
	if db == nil {
		return nil, fmt.Errorf("Configure a Cuan connection before reading tasks")
	}

	whereClause := " WHERE 1=1"
	args := []any{}
	argIdx := 1

	if opts.ProjectID != "" {
		whereClause += fmt.Sprintf(" AND (project_id = $%d OR project_id IN (SELECT id FROM projects WHERE LOWER(name) = LOWER($%d)))", argIdx, argIdx)
		args = append(args, opts.ProjectID)
		argIdx++
	}
	if opts.Status != "" {
		whereClause += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, opts.Status)
		argIdx++
	}

	var totalCount int
	countQuery := "SELECT COUNT(*) FROM tasks" + whereClause
	if err := db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, fmt.Errorf("count tasks: %w", err)
	}

	query := "SELECT id, project_id, title, instructions, status, priority, summary, complete_note, dod_checklist, created_at, updated_at FROM tasks" + whereClause
	query += fmt.Sprintf(" ORDER BY updated_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	queryArgs := append(append([]any{}, args...), limit, offset)

	rows, err := db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	var fullTasks []TaskSummary
	for rows.Next() {
		var t TaskSummary
		var dodRaw []byte
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Title, &t.Instructions, &t.Status, &t.Priority, &t.Summary, &t.CompleteNote, &dodRaw, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		if len(dodRaw) > 0 {
			var rawList []any
			if json.Unmarshal(dodRaw, &rawList) == nil {
				for _, item := range rawList {
					if str, ok := item.(string); ok {
						t.DoDChecklist = append(t.DoDChecklist, str)
					} else if m, ok := item.(map[string]any); ok {
						if text, ok := m["text"].(string); ok {
							t.DoDChecklist = append(t.DoDChecklist, text)
						}
					}
				}
			}
		}
		t.CreatedAt = createdAt.Format(time.RFC3339)
		redactTaskSummary(&t)
		t.UpdatedAt = updatedAt.Format(time.RFC3339)
		fullTasks = append(fullTasks, t)
	}
	if fullTasks == nil {
		fullTasks = []TaskSummary{}
	}
	if limit > 0 && len(fullTasks) > limit {
		fullTasks = fullTasks[:limit]
	}

	hasMore := offset+len(fullTasks) < totalCount
	res := map[string]any{
		"total":       len(fullTasks),
		"total_count": totalCount,
		"limit":       limit,
		"offset":      offset,
		"has_more":    hasMore,
		"compact":     !opts.Full,
	}
	if hasMore {
		res["next_offset"] = offset + len(fullTasks)
	}
	if opts.Full {
		res["tasks"] = fullTasks
	} else {
		compactTasks := make([]map[string]any, 0, len(fullTasks))
		for _, t := range fullTasks {
			compactTasks = append(compactTasks, compactTaskSummaryItem(t))
		}
		res["tasks"] = compactTasks
	}

	return res, nil
}

// GetTask returns details of a single task using compact defaults.
func GetTask(ctx context.Context, taskID string) (any, error) {
	return GetTaskWithOptions(ctx, taskID, GetTaskOptions{})
}

// GetTaskWithOptions returns details of a single task, compacted by default or full when opts.Full is true.
func GetTaskWithOptions(ctx context.Context, taskID string, opts GetTaskOptions) (any, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}
	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		result, err := client.GetTask(ctx, taskID)
		if err != nil {
			return nil, fmt.Errorf("Cuan connection unavailable; task state could not be confirmed")
		}
		redactTaskDiagnostic(result)
		if !opts.Full {
			return compactTaskDetail(result), nil
		}
		return result, nil
	}
	db := getPlatformDB()
	if db == nil {
		return nil, fmt.Errorf("Configure a Cuan connection before reading a task")
	}

	var t TaskSummary
	var dodRaw []byte
	var createdAt, updatedAt time.Time
	err := db.QueryRowContext(ctx,
		"SELECT id, project_id, title, instructions, status, priority, summary, complete_note, dod_checklist, created_at, updated_at FROM tasks WHERE id = $1 LIMIT 1",
		taskID,
	).Scan(&t.ID, &t.ProjectID, &t.Title, &t.Instructions, &t.Status, &t.Priority, &t.Summary, &t.CompleteNote, &dodRaw, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return map[string]any{
			"error":   "task_not_found",
			"task_id": taskID,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}

	if len(dodRaw) > 0 {
		var rawList []any
		if json.Unmarshal(dodRaw, &rawList) == nil {
			for _, item := range rawList {
				if str, ok := item.(string); ok {
					t.DoDChecklist = append(t.DoDChecklist, str)
				} else if m, ok := item.(map[string]any); ok {
					if text, ok := m["text"].(string); ok {
						t.DoDChecklist = append(t.DoDChecklist, text)
					}
				}
			}
		}
	}
	t.CreatedAt = createdAt.Format(time.RFC3339)
	redactTaskSummary(&t)
	t.UpdatedAt = updatedAt.Format(time.RFC3339)

	// Fetch attempts summary
	rows, _ := db.QueryContext(ctx, "SELECT id, runner_id, agent_name, status, exit_code, peak_rss_bytes, started_at, finished_at, error_message FROM task_attempts WHERE task_id = $1 ORDER BY created_at DESC LIMIT 5", taskID)
	var attempts []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var attID, runnerID, agentName, attStatus string
			var exitCode *int
			var peakRSS int64
			var startedAt, finishedAt *time.Time
			var errMsg *string
			if err := rows.Scan(&attID, &runnerID, &agentName, &attStatus, &exitCode, &peakRSS, &startedAt, &finishedAt, &errMsg); err == nil {
				attMap := map[string]any{
					"id":             attID,
					"runner_id":      runnerID,
					"agent_name":     agentName,
					"status":         attStatus,
					"peak_rss_bytes": peakRSS,
				}
				if exitCode != nil {
					attMap["exit_code"] = *exitCode
				}
				if errMsg != nil {
					attMap["error_message"] = *errMsg
				}
				attempts = append(attempts, attMap)
			}
		}
	}

	res := map[string]any{
		"task":     t,
		"attempts": attempts,
	}
	if !opts.Full {
		return compactTaskDetail(res), nil
	}
	return res, nil
}

const (
	compactListSummaryMaxChars      = 240
	compactDetailInstructionsMax    = 600
	compactDetailSummaryMax         = 500
	compactDetailCompleteNoteMax    = 800
	compactDetailProgressMax        = 500
	compactDetailDiffTestMax        = 500
	compactDetailDoDEvidenceMax     = 200
	compactDetailAttemptErrMax      = 300
	compactDetailActivityMsgMax     = 200
	compactDetailMaxAttempts        = 3
	compactDetailMaxActivities      = 5
)

func truncateDiagnosticText(s string, maxChars int) (string, bool) {
	if maxChars <= 0 || s == "" {
		return s, false
	}
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s, false
	}
	if maxChars < 80 {
		return string(runes[:maxChars]) + "...", true
	}
	const marker = "\n... (truncated) ...\n"
	headLen := (maxChars - len([]rune(marker))) / 2
	tailLen := maxChars - len([]rune(marker)) - headLen
	if headLen < 1 || tailLen < 1 {
		return string(runes[:maxChars]) + "...", true
	}
	return string(runes[:headLen]) + marker + string(runes[len(runes)-tailLen:]), true
}

func compactTaskAttempt(raw map[string]any) (map[string]any, bool) {
	if raw == nil {
		return nil, false
	}
	out := map[string]any{}
	for _, key := range []string{"id", "status", "attempt_number", "runner_id", "agent_name", "exit_code", "started_at", "finished_at"} {
		if val, ok := raw[key]; ok && val != nil && val != "" {
			out[key] = val
		}
	}
	truncated := false
	if errMsg, ok := raw["error_message"].(string); ok && errMsg != "" {
		trimmed, didTrunc := truncateDiagnosticText(errMsg, compactDetailAttemptErrMax)
		out["error_message"] = trimmed
		truncated = didTrunc
	}
	return out, truncated
}

func compactTaskListItem(item map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"id", "project_id", "project_name", "title", "status", "priority", "assigned_agent", "review_state", "created_at", "updated_at"} {
		if val, ok := item[key]; ok && val != nil && val != "" {
			out[key] = val
		}
	}
	if summary, ok := item["summary"].(string); ok && summary != "" {
		out["summary"], _ = truncateDiagnosticText(summary, compactListSummaryMaxChars)
	}
	if la, ok := item["latest_attempt"].(map[string]any); ok && la != nil {
		compactedLA, _ := compactTaskAttempt(la)
		out["latest_attempt"] = compactedLA
	}
	return out
}

func compactTaskSummaryItem(t TaskSummary) map[string]any {
	out := map[string]any{
		"id":         t.ID,
		"project_id": t.ProjectID,
		"title":      t.Title,
		"status":     t.Status,
		"priority":   t.Priority,
		"created_at": t.CreatedAt,
		"updated_at": t.UpdatedAt,
	}
	if t.Summary != "" {
		out["summary"], _ = truncateDiagnosticText(t.Summary, compactListSummaryMaxChars)
	}
	return out
}

func compactTaskDetail(result map[string]any) map[string]any {
	if result == nil {
		return nil
	}
	if _, hasErr := result["error"]; hasErr {
		return result
	}

	// Normalize DB fallback shape {"task": TaskSummary|map, "attempts": ...}
	src := result
	if nestedTask, ok := result["task"]; ok && nestedTask != nil {
		flat := map[string]any{}
		switch t := nestedTask.(type) {
		case TaskSummary:
			flat["id"] = t.ID
			flat["project_id"] = t.ProjectID
			flat["title"] = t.Title
			flat["instructions"] = t.Instructions
			flat["status"] = t.Status
			flat["priority"] = t.Priority
			flat["summary"] = t.Summary
			flat["complete_note"] = t.CompleteNote
			flat["dod_checklist"] = t.DoDChecklist
			flat["created_at"] = t.CreatedAt
			flat["updated_at"] = t.UpdatedAt
		case map[string]any:
			for k, v := range t {
				flat[k] = v
			}
		}
		if attempts, ok := result["attempts"]; ok {
			flat["attempts"] = attempts
		}
		src = flat
	}

	out := map[string]any{
		"compact": true,
	}
	truncated := false

	for _, key := range []string{
		"id", "project_id", "project_ids", "project_name", "title",
		"status", "priority", "execution_category", "assigned_agent",
		"branch_name", "pr_url", "pr_number", "commit_sha",
		"result_kind", "review_state", "review_finding", "review_action",
		"created_at", "updated_at", "completed_at",
	} {
		if val, ok := src[key]; ok && val != nil && val != "" {
			out[key] = val
		}
	}

	textLimits := []struct {
		key string
		max int
	}{
		{"instructions", compactDetailInstructionsMax},
		{"summary", compactDetailSummaryMax},
		{"complete_note", compactDetailCompleteNoteMax},
		{"latest_progress", compactDetailProgressMax},
		{"diff_summary", compactDetailDiffTestMax},
		{"test_summary", compactDetailDiffTestMax},
	}
	for _, spec := range textLimits {
		if val, ok := src[spec.key].(string); ok && val != "" {
			trimmed, didTrunc := truncateDiagnosticText(val, spec.max)
			out[spec.key] = trimmed
			if didTrunc {
				truncated = true
			}
		}
	}

	if la, ok := src["latest_attempt"].(map[string]any); ok && la != nil {
		compactedLA, didTrunc := compactTaskAttempt(la)
		out["latest_attempt"] = compactedLA
		if didTrunc {
			truncated = true
		}
	}

	if dodRaw, ok := src["dod_checklist"]; ok && dodRaw != nil {
		switch items := dodRaw.(type) {
		case []any:
			compactedDoD := make([]any, 0, len(items))
			for _, item := range items {
				if m, ok := item.(map[string]any); ok {
					entry := map[string]any{}
					if id, ok := m["id"]; ok && id != "" {
						entry["id"] = id
					}
					if text, ok := m["text"]; ok {
						entry["text"] = text
					}
					if completed, ok := m["completed"]; ok {
						entry["completed"] = completed
					}
					if ev, ok := m["evidence"].(string); ok && ev != "" {
						trimmedEv, didTrunc := truncateDiagnosticText(ev, compactDetailDoDEvidenceMax)
						entry["evidence"] = trimmedEv
						if didTrunc {
							truncated = true
						}
					}
					compactedDoD = append(compactedDoD, entry)
				} else {
					compactedDoD = append(compactedDoD, item)
				}
			}
			out["dod_checklist"] = compactedDoD
		case []string:
			out["dod_checklist"] = items
		}
	}

	if rawAttempts, ok := src["attempts"]; ok && rawAttempts != nil {
		var attemptList []map[string]any
		switch typed := rawAttempts.(type) {
		case []any:
			for _, a := range typed {
				if m, ok := a.(map[string]any); ok {
					attemptList = append(attemptList, m)
				}
			}
		case []map[string]any:
			attemptList = typed
		}
		if len(attemptList) > 0 {
			if len(attemptList) > compactDetailMaxAttempts {
				out["attempts_total"] = len(attemptList)
				attemptList = attemptList[:compactDetailMaxAttempts]
				truncated = true
			}
			compacted := make([]map[string]any, 0, len(attemptList))
			for _, a := range attemptList {
				ca, didTrunc := compactTaskAttempt(a)
				compacted = append(compacted, ca)
				if didTrunc {
					truncated = true
				}
			}
			out["attempts"] = compacted
		}
	}

	if rawActivities, ok := src["activities"].([]any); ok && len(rawActivities) > 0 {
		if len(rawActivities) > compactDetailMaxActivities {
			out["activities_total"] = len(rawActivities)
			rawActivities = rawActivities[len(rawActivities)-compactDetailMaxActivities:]
			truncated = true
		}
		compacted := make([]map[string]any, 0, len(rawActivities))
		for _, rawAct := range rawActivities {
			if m, ok := rawAct.(map[string]any); ok {
				entry := map[string]any{}
				for _, k := range []string{"id", "activity_type", "author", "old_status", "new_status", "created_at"} {
					if val, exists := m[k]; exists && val != nil && val != "" {
						entry[k] = val
					}
				}
				if msg, ok := m["message"].(string); ok && msg != "" {
					trimmedMsg, didTrunc := truncateDiagnosticText(msg, compactDetailActivityMsgMax)
					entry["message"] = trimmedMsg
					if didTrunc {
						truncated = true
					}
				}
				compacted = append(compacted, entry)
			}
		}
		out["activities"] = compacted
	}

	if truncated {
		out["truncated"] = true
	}
	return out
}

// Diagnostic consumers need approved scope, not the frozen execution environment.
// This operates only on the response; stored context and worker reads stay intact.
func redactTaskDiagnostic(value map[string]any) {
	for key, item := range value {
		value[key] = redactTaskDiagnosticValue(item)
	}
}

func redactTaskDiagnosticValue(value any) any {
	switch item := value.(type) {
	case string:
		return agent.MaskSecrets(project.TaskScopeInstructions(item), nil)
	case map[string]any:
		redactTaskDiagnostic(item)
	case []any:
		for i := range item {
			item[i] = redactTaskDiagnosticValue(item[i])
		}
	}
	return value
}

func redactTaskSummary(t *TaskSummary) {
	t.Instructions = redactTaskDiagnosticValue(t.Instructions).(string)
	t.Summary = agent.MaskSecrets(t.Summary, nil)
	t.CompleteNote = agent.MaskSecrets(t.CompleteNote, nil)
	for i := range t.DoDChecklist {
		t.DoDChecklist[i] = agent.MaskSecrets(t.DoDChecklist[i], nil)
	}
}

// CreateTaskDraft proposes or creates a new task draft for user approval or queueing.
func CreateTaskDraft(ctx context.Context, projectID, title, instructions, priority string, dodChecklist []string) (any, error) {
	return CreateTaskDraftWithRoom(ctx, projectID, title, instructions, priority, "", dodChecklist)
}

// CreateTaskDraftWithRoom proposes or creates a new task draft with an optional source room binding.
func CreateTaskDraftWithRoom(ctx context.Context, projectID, title, instructions, priority, roomID string, dodChecklist []string) (any, error) {
	if err := taskSessionOwnerWriteError(); err != nil {
		return nil, err
	}

	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	instructions = strings.TrimSpace(instructions)
	if instructions == "" {
		return nil, fmt.Errorf("instructions is required")
	}
	if priority == "" {
		priority = "normal"
	}
	projectID = strings.TrimSpace(projectID)
	roomID = strings.TrimSpace(roomID)
	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		created, err := client.CreateTaskWithRoom(ctx, projectID, title, instructions, priority, roomID, dodChecklist)
		if err != nil {
			return nil, err
		}
		return created, nil
	}

	db := getPlatformDB()
	if db == nil || projectID == "" {
		return nil, fmt.Errorf("task was not created: configure the Cuan Owner API or a local database with an explicit project")
	}
	var realProjectID string
	if db != nil {
		var p project.ProjectResponse
		var metadata []byte
		err := db.QueryRowContext(ctx, "SELECT id, name, COALESCE(repo_url,''), COALESCE(default_branch,''), COALESCE(metadata,'{}'::jsonb) FROM projects WHERE id = $1 OR LOWER(name) = LOWER($1) LIMIT 1", projectID).Scan(&p.ID, &p.Name, &p.RepoURL, &p.DefaultBranch, &metadata)
		if err != nil {
			return nil, fmt.Errorf("project guidance is unavailable; draft was not created")
		}
		p.Metadata = metadata
		realProjectID = p.ID
		instructions = project.SnapshotProjectGuidance(instructions, &p)
		if len([]rune(instructions)) > 24000 {
			return nil, fmt.Errorf("instructions and project guidance exceed 24000 characters")
		}
	}
	if realProjectID == "" {
		realProjectID = projectID
	}

	taskID := fmt.Sprintf("task_%d", time.Now().UnixNano())
	dodJSON, _ := json.Marshal(dodChecklist)

	if db != nil && realProjectID != "" {
		if roomID != "" {
			_, err := db.ExecContext(ctx,
				"INSERT INTO tasks (id, project_id, title, instructions, status, priority, dod_checklist, source_room_id, created_at, updated_at) VALUES ($1, $2, $3, $4, 'backlog', $5, $6, $7, NOW(), NOW())",
				taskID, realProjectID, title, instructions, priority, dodJSON, roomID,
			)
			if err != nil {
				return nil, fmt.Errorf("insert task: %w", err)
			}
		} else {
			_, err := db.ExecContext(ctx,
				"INSERT INTO tasks (id, project_id, title, instructions, status, priority, dod_checklist, created_at, updated_at) VALUES ($1, $2, $3, $4, 'backlog', $5, $6, NOW(), NOW())",
				taskID, realProjectID, title, instructions, priority, dodJSON,
			)
			if err != nil {
				return nil, fmt.Errorf("insert task: %w", err)
			}
		}
	}

	return map[string]any{
		"id":            taskID,
		"project_id":    realProjectID,
		"title":         title,
		"instructions":  instructions,
		"priority":      priority,
		"dod_checklist": dodChecklist,
		"status":        "backlog",
		"action":        "task_draft_created",
	}, nil
}

// InboxConversation represents an inbox room / conversation.
type InboxConversation struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	Title       string `json:"title"`
	Topic       string `json:"topic"`
	TaskCount   int    `json:"task_count"`
	ActiveTasks int    `json:"active_tasks"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// ListInboxConversations lists customer chat sessions and project rooms.
func ListInboxConversations(ctx context.Context, projectID string, limit int) (any, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		convs, err := client.ListConversations(ctx, projectID, limit)
		if err != nil {
			return nil, err
		}
		return map[string]any{"conversations": convs, "total": len(convs)}, nil
	}
	db := getPlatformDB()
	if db == nil {
		return map[string]any{
			"conversations": []InboxConversation{},
			"total":         0,
			"status":        "database_offline",
		}, nil
	}

	query := `
		SELECT r.id, r.project_id, r.title, r.topic, r.created_at, r.updated_at,
		       COALESCE(tc.total, 0), COALESCE(tc.active, 0)
		FROM project_rooms r
		LEFT JOIN (
			SELECT source_room_id,
			       COUNT(*) AS total,
			       COUNT(CASE WHEN status NOT IN ('done', 'canceled', 'cancelled') THEN 1 END) AS active
			FROM tasks
			WHERE source_room_id IS NOT NULL
			GROUP BY source_room_id
		) tc ON tc.source_room_id = r.id
		WHERE 1=1`
	args := []any{}
	argIdx := 1
	if projectID != "" {
		query += fmt.Sprintf(" AND (r.project_id = $%d OR r.project_id IN (SELECT id FROM projects WHERE LOWER(name) = LOWER($%d)))", argIdx, argIdx)
		args = append(args, projectID)
		argIdx++
	}
	query += fmt.Sprintf(" ORDER BY r.updated_at DESC LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query inbox rooms: %w", err)
	}
	defer rows.Close()

	var convs []InboxConversation
	for rows.Next() {
		var c InboxConversation
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Title, &c.Topic, &createdAt, &updatedAt, &c.TaskCount, &c.ActiveTasks); err != nil {
			return nil, err
		}
		c.CreatedAt = createdAt.Format(time.RFC3339)
		c.UpdatedAt = updatedAt.Format(time.RFC3339)

		convs = append(convs, c)
	}
	if convs == nil {
		convs = []InboxConversation{}
	}

	return map[string]any{
		"conversations": convs,
		"total":         len(convs),
	}, nil
}

// InboxMessage represents a single message in an inbox room.
type InboxMessage struct {
	ID         string `json:"id"`
	RoomID     string `json:"room_id"`
	AuthorType string `json:"author_type"`
	AuthorName string `json:"author_name"`
	Content    string `json:"content"`
	CreatedAt  string `json:"created_at"`
}

// GetInboxMessages returns conversation messages for a room or visitor session UUID.
func GetInboxMessages(ctx context.Context, roomID string, limit int) (any, error) {
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return nil, fmt.Errorf("room_id is required")
	}
	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		return client.GetConversationSnapshot(ctx, roomID, limit)
	}
	db := getPlatformDB()
	if db == nil {
		return nil, fmt.Errorf("configure the Cuan API to read conversation state")
	}

	dbLimit := limit
	if dbLimit <= 0 {
		dbLimit = 100
	} else if dbLimit > 500 {
		dbLimit = 500
	}

	rows, err := db.QueryContext(ctx,
		"SELECT id, room_id, author_type, author_name, content, created_at FROM (SELECT id, room_id, author_type, author_name, content, created_at FROM project_messages WHERE room_id::text = $1 ORDER BY created_at DESC LIMIT $2) AS sub ORDER BY created_at ASC",
		roomID, dbLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var messages []InboxMessage
	for rows.Next() {
		var m InboxMessage
		var createdAt time.Time
		if err := rows.Scan(&m.ID, &m.RoomID, &m.AuthorType, &m.AuthorName, &m.Content, &createdAt); err != nil {
			return nil, err
		}
		m.CreatedAt = createdAt.Format(time.RFC3339)
		messages = append(messages, m)
	}
	if messages == nil {
		messages = []InboxMessage{}
	}

	return map[string]any{
		"room_id":  roomID,
		"messages": messages,
		"total":    len(messages),
	}, nil
}

// DeleteInboxMessage deletes an inbox or conversation message by its UUID.
func DeleteInboxMessage(ctx context.Context, messageID string) (any, error) {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return nil, fmt.Errorf("message_id is required")
	}
	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		return client.DeleteMessage(ctx, messageID)
	}
	db := getPlatformDB()
	if db == nil {
		return nil, fmt.Errorf("configure the Cuan API to manage conversation messages")
	}

	var roomID string
	err := db.QueryRowContext(ctx, "SELECT room_id::text FROM project_messages WHERE id::text = $1", messageID).Scan(&roomID)
	if err != nil {
		return nil, fmt.Errorf("message not found: %w", err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE project_messages SET reply_to_id = NULL WHERE reply_to_id::text = $1", messageID); err != nil {
		return nil, fmt.Errorf("unlink replies: %w", err)
	}
	res, err := db.ExecContext(ctx, "DELETE FROM project_messages WHERE id::text = $1", messageID)
	if err != nil {
		return nil, fmt.Errorf("delete message: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, fmt.Errorf("message not found")
	}
	return map[string]any{
		"ok":      true,
		"deleted": true,
		"id":      messageID,
		"room_id": roomID,
	}, nil
}

// ContactSummary represents a CRM contact.
type ContactSummary struct {
	ID          string   `json:"id"`
	Product     string   `json:"product"`
	GuestUUID   string   `json:"guest_uuid"`
	SessionUUID string   `json:"session_uuid"`
	Name        string   `json:"name"`
	Email       string   `json:"email"`
	Tags        []string `json:"tags,omitempty"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// ListContacts reads the configured installation, never an ambient host database.
func ListContacts(ctx context.Context, query, product string, limit int) (any, error) {
	return ownerContactList(ctx, query, product, limit)
}

// GetContact resolves a contact or conversation identity through the Owner API.
func GetContact(ctx context.Context, contactID string) (any, error) {
	return ownerContactDetail(ctx, contactID)
}

// ListScheduledJobs returns scheduled background jobs and recurring automations.
func ListScheduledJobs(ctx context.Context) (any, error) {
	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		return nil, fmt.Errorf("Connect Cuan to inspect actual scheduler state")
	}
	jobs, err := client.ScheduledJobs(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{"jobs": jobs, "total": len(jobs)}, nil
}

// GetSystemSettings returns platform telemetry, version, active agents, and environment.
func GetSystemSettings(ctx context.Context) (any, error) {
	version := os.Getenv("CUAN_APP_VERSION")
	if version == "" {
		version = "0.1.0"
	}
	env := os.Getenv("CUAN_ENV")
	if env == "" {
		env = "development"
	}
	baseDomain := os.Getenv("BASE_DOMAIN")
	if baseDomain == "" {
		baseDomain = "localhost"
	}

	db := getPlatformDB()
	dbConnected := false
	var agentCount int
	var projectCount int
	var taskCount int

	if db != nil {
		if err := db.PingContext(ctx); err == nil {
			dbConnected = true
			_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM agents").Scan(&agentCount)
			_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM projects").Scan(&projectCount)
			_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks").Scan(&taskCount)
		}
	}

	return map[string]any{
		"platform": map[string]any{
			"name":        "Cuan Server",
			"version":     version,
			"environment": env,
			"base_domain": baseDomain,
			"license":     "Apache-2.0",
			"repository":  "https://github.com/buchin/cuan",
		},
		"database": map[string]any{
			"connected": dbConnected,
			"engine":    "PostgreSQL",
		},
		"counts": map[string]any{
			"projects": projectCount,
			"tasks":    taskCount,
			"agents":   agentCount,
		},
	}, nil
}

// InspectDatabaseQuery executes a bounded, strictly read-only SQL query against the platform PostgreSQL database.
func InspectDatabaseQuery(ctx context.Context, query string, limit int) (any, error) {
	db := getPlatformDB()
	if db == nil {
		return nil, fmt.Errorf("database connection unavailable")
	}

	clean := strings.TrimSpace(query)
	if clean == "" {
		return nil, fmt.Errorf("empty query")
	}

	// Safety checks: Reject any statements that aren't SELECT, EXPLAIN, WITH (CTE select), or SHOW
	upper := strings.ToUpper(clean)
	for strings.HasPrefix(upper, "--") || strings.HasPrefix(upper, "/*") {
		if strings.HasPrefix(upper, "--") {
			if idx := strings.Index(upper, "\n"); idx != -1 {
				upper = strings.TrimSpace(upper[idx+1:])
				clean = strings.TrimSpace(clean[idx+1:])
			} else {
				return nil, fmt.Errorf("invalid comment-only query")
			}
		} else if strings.HasPrefix(upper, "/*") {
			if idx := strings.Index(upper, "*/"); idx != -1 {
				upper = strings.TrimSpace(upper[idx+2:])
				clean = strings.TrimSpace(clean[idx+2:])
			} else {
				return nil, fmt.Errorf("unclosed comment in query")
			}
		}
	}

	if !strings.HasPrefix(upper, "SELECT") &&
		!strings.HasPrefix(upper, "EXPLAIN") &&
		!strings.HasPrefix(upper, "WITH") &&
		!strings.HasPrefix(upper, "SHOW") {
		return nil, fmt.Errorf("only read-only queries (SELECT, EXPLAIN, WITH, SHOW) are permitted")
	}

	forbidden := []string{
		"INSERT ", "UPDATE ", "DELETE ", "DROP ", "ALTER ", "TRUNCATE ", "GRANT ",
		"REVOKE ", "CREATE ", "REPLACE ", "EXEC ", "EXECUTE ", "CALL ", "COPY ",
		"VACUUM ", "REINDEX ", "LOCK ", "DISCARD ", "DO ",
	}
	for _, word := range forbidden {
		if strings.Contains(upper, word) {
			return nil, fmt.Errorf("query contains forbidden mutating keyword: %s", strings.TrimSpace(word))
		}
	}

	semiIdx := strings.Index(clean, ";")
	if semiIdx != -1 && strings.TrimSpace(clean[semiIdx+1:]) != "" {
		return nil, fmt.Errorf("multiple SQL statements are not permitted")
	}
	clean = strings.TrimSuffix(clean, ";")

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	execQuery := clean
	if strings.HasPrefix(upper, "SELECT") || strings.HasPrefix(upper, "WITH") {
		if !strings.Contains(upper, "LIMIT ") {
			execQuery = fmt.Sprintf("SELECT * FROM (%s) AS _cuan_ro_q LIMIT %d", clean, limit)
		}
	}

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := db.BeginTx(queryCtx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("failed to start read-only transaction: %w", err)
	}
	defer tx.Rollback()

	_, _ = tx.ExecContext(queryCtx, "SET LOCAL statement_timeout = '5000'")

	start := time.Now()
	rows, err := tx.QueryContext(queryCtx, execQuery)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	isMasked := make([]bool, len(cols))
	for i, col := range cols {
		colLower := strings.ToLower(col)
		if strings.Contains(colLower, "password") ||
			strings.Contains(colLower, "secret") ||
			strings.Contains(colLower, "token") ||
			strings.Contains(colLower, "private_key") ||
			strings.Contains(colLower, "access_key") {
			isMasked[i] = true
		}
	}

	var resultRows []map[string]any
	for rows.Next() {
		if len(resultRows) >= limit {
			break
		}
		columnPointers := make([]any, len(cols))
		columnValues := make([]any, len(cols))
		for i := range columnValues {
			columnPointers[i] = &columnValues[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		rowMap := make(map[string]any, len(cols))
		for i, col := range cols {
			if isMasked[i] {
				rowMap[col] = "[REDACTED_SECRET]"
				continue
			}
			val := columnValues[i]
			if b, ok := val.([]byte); ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = val
			}
		}
		resultRows = append(resultRows, rowMap)
	}

	duration := time.Since(start)

	return map[string]any{
		"columns":        cols,
		"rows":           resultRows,
		"row_count":      len(resultRows),
		"execution_time": fmt.Sprintf("%.2fms", float64(duration.Microseconds())/1000.0),
		"limit_applied":  limit,
	}, nil
}

// InspectDatabaseSchema returns tables in the public schema or detailed column information for a table.
func InspectDatabaseSchema(ctx context.Context, tableName string) (any, error) {
	db := getPlatformDB()
	if db == nil {
		return nil, fmt.Errorf("database connection unavailable")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tableName = strings.TrimSpace(tableName)
	if tableName == "" {
		rows, err := db.QueryContext(queryCtx, `
			SELECT table_name,
			       COALESCE((SELECT n_live_tup FROM pg_stat_user_tables WHERE relname = table_name), 0) AS estimated_rows
			FROM information_schema.tables
			WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
			ORDER BY table_name;
		`)
		if err != nil {
			return nil, fmt.Errorf("failed to list tables: %w", err)
		}
		defer rows.Close()

		type TableInfo struct {
			Name          string `json:"table_name"`
			EstimatedRows int64  `json:"estimated_rows"`
		}
		var tables []TableInfo
		for rows.Next() {
			var t TableInfo
			if err := rows.Scan(&t.Name, &t.EstimatedRows); err == nil {
				tables = append(tables, t)
			}
		}
		return map[string]any{
			"schema":       "public",
			"tables_count": len(tables),
			"tables":       tables,
		}, nil
	}

	rows, err := db.QueryContext(queryCtx, `
		SELECT column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position;
	`, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect table columns: %w", err)
	}
	defer rows.Close()

	type ColumnInfo struct {
		Name       string  `json:"column_name"`
		DataType   string  `json:"data_type"`
		IsNullable string  `json:"is_nullable"`
		Default    *string `json:"default,omitempty"`
	}
	var columns []ColumnInfo
	for rows.Next() {
		var c ColumnInfo
		if err := rows.Scan(&c.Name, &c.DataType, &c.IsNullable, &c.Default); err == nil {
			columns = append(columns, c)
		}
	}

	if len(columns) == 0 {
		return nil, fmt.Errorf("table %q not found in public schema", tableName)
	}

	return map[string]any{
		"table_name":   tableName,
		"column_count": len(columns),
		"columns":      columns,
	}, nil
}

// InspectAppLogs retrieves recent application logs via journalctl or agent_logs table.
func InspectAppLogs(ctx context.Context, lines int, filter string) (any, error) {
	if lines <= 0 || lines > 100 {
		lines = 50
	}
	filter = strings.TrimSpace(filter)

	queryCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	cmdArgs := []string{"-u", "cuan", "-u", "cuan-chat@*.service", "-u", "cuan-runner", "-n", fmt.Sprintf("%d", lines), "--no-pager"}
	if filter != "" {
		cmdArgs = append(cmdArgs, "-g", filter)
	}
	cmd := exec.CommandContext(queryCtx, "journalctl", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if err == nil && len(out) > 0 {
		rawLines := strings.Split(strings.TrimSpace(string(out)), "\n")
		return map[string]any{
			"source":     "journalctl",
			"line_count": len(rawLines),
			"lines":      rawLines,
		}, nil
	}

	db := getPlatformDB()
	if db != nil {
		var query string
		var args []any
		if filter != "" {
			query = `SELECT id, stream, line, logged_at 
			         FROM agent_logs 
			         WHERE line ILIKE $1 
			         ORDER BY id DESC LIMIT $2`
			args = []any{"%" + filter + "%", lines}
		} else {
			query = `SELECT id, stream, line, logged_at 
			         FROM agent_logs 
			         ORDER BY id DESC LIMIT $1`
			args = []any{lines}
		}
		rows, qErr := db.QueryContext(queryCtx, query, args...)
		if qErr == nil {
			defer rows.Close()
			type LogEntry struct {
				ID       int64      `json:"id"`
				Stream   string     `json:"stream"`
				Line     string     `json:"line"`
				LoggedAt *time.Time `json:"logged_at,omitempty"`
			}
			var entries []LogEntry
			for rows.Next() {
				var e LogEntry
				if err := rows.Scan(&e.ID, &e.Stream, &e.Line, &e.LoggedAt); err == nil {
					entries = append(entries, e)
				}
			}
			return map[string]any{
				"source":      "agent_logs",
				"entry_count": len(entries),
				"entries":     entries,
			}, nil
		}
	}

	return map[string]any{
		"source":  "none",
		"message": "No recent logs found via journalctl or agent_logs",
	}, nil
}

// ListApps lists external third-party applications in Cuan.
func ListApps(ctx context.Context) (any, error) {
	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		return client.ListApps(ctx)
	}
	reg := mcp.GetDefaultRegistry()
	apps := reg.ListApps()
	return map[string]any{
		"status": "success",
		"apps":   apps,
		"total":  len(apps),
	}, nil
}

// ListSkills lists modular intelligence skills in Cuan.
func ListSkills(ctx context.Context) (any, error) {
	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		return client.ListSkills(ctx)
	}
	reg := mcp.GetDefaultRegistry()
	skills := reg.ListSkills()
	return map[string]any{
		"status": "success",
		"skills": skills,
		"total":  len(skills),
	}, nil
}

// ToggleAgentSkill activates or deactivates a skill for an agent.
func ToggleAgentSkill(ctx context.Context, agentID, skillID string, enabled bool) (any, error) {
	if err := taskSessionOwnerWriteError(); err != nil {
		return nil, err
	}

	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		return client.ToggleAgentSkill(ctx, agentID, skillID, enabled)
	}
	reg := mcp.GetDefaultRegistry()
	if err := reg.ToggleAgentSkill(agentID, skillID, enabled); err != nil {
		return nil, err
	}
	statusStr := "enabled"
	if !enabled {
		statusStr = "disabled"
	}
	return map[string]any{
		"status":   "success",
		"agent_id": agentID,
		"skill_id": skillID,
		"enabled":  enabled,
		"message":  fmt.Sprintf("Skill %s successfully %s for agent %s", skillID, statusStr, agentID),
	}, nil
}

// CreateInboxConversation creates a new discussion room or agent topic conversation.
func CreateInboxConversation(ctx context.Context, agentID, projectID, title string) (any, error) {
	if err := taskSessionOwnerWriteError(); err != nil {
		return nil, err
	}

	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		return client.CreateConversation(ctx, agentID, projectID, title)
	}

	db := getPlatformDB()
	if db == nil {
		return nil, fmt.Errorf("cuan client is not configured and database connection is unavailable")
	}

	agentID = NormalizeAgentAlias(agentID)
	if projectID == "" {
		projectID = "default"
	} else if !strings.EqualFold(projectID, "default") {
		var resolvedProjID string
		if err := db.QueryRowContext(ctx, "SELECT id FROM projects WHERE id = $1 OR LOWER(name) = LOWER($1) OR LOWER(repo_name) = LOWER($1) LIMIT 1", projectID).Scan(&resolvedProjID); err == nil && resolvedProjID != "" {
			projectID = resolvedProjID
		}
	}
	if title == "" {
		title = "New conversation"
	}
	roomID := dbPkg.GenerateUUID()
	now := time.Now().UTC()

	topic := "agent-topic:" + agentID
	_, err := db.ExecContext(ctx, "INSERT INTO project_rooms (id, project_id, title, topic, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $5)",
		roomID, projectID, title, topic, now)
	if err != nil {
		return nil, fmt.Errorf("create room in db: %w", err)
	}

	agentName := ResolveActiveAgentName(ctx, agentID)

	return map[string]any{
		"id":              "topic_" + roomID,
		"room_id":         roomID,
		"conversation_id": roomID,
		"agent_id":        agentID,
		"agent_name":      agentName,
		"project_id":      projectID,
		"title":           title,
		"topic":           topic,
		"status":          "created",
		"created_at":      now.Format(time.RFC3339),
	}, nil
}

// LinkTaskToRoom binds an existing task to an inbox room (source_room_id).
func LinkTaskToRoom(ctx context.Context, taskID, roomID string) (any, error) {
	if err := taskSessionOwnerWriteError(); err != nil {
		return nil, err
	}
	taskID = strings.TrimSpace(taskID)
	roomID = strings.TrimSpace(roomID)
	if taskID == "" || roomID == "" {
		return nil, fmt.Errorf("task_id and room_id are required")
	}
	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		return client.LinkTaskToRoom(ctx, taskID, roomID)
	}
	db := getPlatformDB()
	if db == nil {
		return nil, fmt.Errorf("cuan client is not configured and database connection is unavailable")
	}
	res, err := db.ExecContext(ctx, "UPDATE tasks SET source_room_id = $1, updated_at = $2 WHERE id = $3", roomID, time.Now().UTC(), taskID)
	if err != nil {
		return nil, fmt.Errorf("link task to room in db: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	return map[string]any{
		"status":         "success",
		"task_id":        taskID,
		"source_room_id": roomID,
	}, nil
}

// CreateInboxConversationWithTask atomically creates an inbox room and a linked task.
func CreateInboxConversationWithTask(ctx context.Context, projectID, title, instructions, priority, agentID, initialMessage string, dodList []string) (any, error) {
	if err := taskSessionOwnerWriteError(); err != nil {
		return nil, err
	}
	convRes, err := CreateInboxConversation(ctx, agentID, projectID, title)
	if err != nil {
		return nil, err
	}
	convMap, _ := convRes.(map[string]any)
	roomID, _ := convMap["room_id"].(string)
	if roomID == "" {
		roomID, _ = convMap["conversation_id"].(string)
	}
	if roomID == "" {
		return nil, fmt.Errorf("created conversation did not return a room_id")
	}
	taskRes, err := CreateTaskDraftWithRoom(ctx, projectID, title, instructions, priority, roomID, dodList)
	if err != nil {
		return nil, fmt.Errorf("create linked task: %w", err)
	}
	var msgRes any
	if strings.TrimSpace(initialMessage) != "" {
		msgRes, _ = SendInboxMessage(ctx, roomID, initialMessage, false, 0)
	}
	return map[string]any{
		"status":       "success",
		"conversation": convMap,
		"room_id":      roomID,
		"task":         taskRes,
		"message":      msgRes,
	}, nil
}

// SendInboxMessage sends a user message to an inbox conversation and optionally waits for the agent's reply.
func SendInboxMessage(ctx context.Context, roomID, message string, waitForReply bool, waitSeconds int) (any, error) {
	if err := taskSessionOwnerWriteError(); err != nil {
		return nil, err
	}

	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return nil, fmt.Errorf("room_id is required")
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}

	client := GetCuanClient()
	if client != nil && client.IsConfigured() {
		sentTime := time.Now().UTC()
		sendRes, err := client.SendMessage(ctx, roomID, message)
		if err != nil {
			return nil, err
		}

		if !waitForReply {
			return sendRes, nil
		}

		if waitSeconds <= 0 {
			waitSeconds = 30
		}
		if waitSeconds > 120 {
			waitSeconds = 120
		}

		timeout := time.Duration(waitSeconds) * time.Second
		reply, err := client.WaitForReply(ctx, roomID, sentTime, timeout)
		if err != nil {
			return map[string]any{
				"send_result": sendRes,
				"warning":     fmt.Sprintf("Sent message, but waiting for agent reply timed out after %ds: %v", waitSeconds, err),
				"status":      "sent_waiting_timed_out",
			}, nil
		}

		replyContent, _ := reply["content"].(string)
		if replyContent == "" {
			replyContent, _ = reply["message"].(string)
		}

		return map[string]any{
			"status":     "replied",
			"room_id":    roomID,
			"reply":      replyContent,
			"agent_name": reply["author_name"],
			"agent_id":   reply["author_id"],
			"message_id": reply["uuid"],
			"created_at": reply["created_at"],
		}, nil
	}

	// A database connection is not an authenticated owner identity. Inserting an
	// artificial Owner/user message would also forge a source of authorization.
	return nil, fmt.Errorf("sending an owner message requires an authenticated Cuan API client; worker results must use task report")
}

// UpdateTaskStatus updates status, summary, and verification evidence for a task.
func UpdateTaskStatus(ctx context.Context, taskID, status, summary, completeNote string) (any, error) {
	if err := taskSessionOwnerWriteError(); err != nil {
		return nil, err
	}

	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}
	status = strings.TrimSpace(status)
	if status == "" && summary == "" && completeNote == "" {
		return nil, fmt.Errorf("at least one of status, summary, or complete_note is required")
	}

	client := GetCuanClient()
	if client != nil && client.IsConfigured() {
		return client.UpdateTask(ctx, taskID, status, summary, completeNote)
	}

	db := getPlatformDB()
	if db == nil {
		return nil, fmt.Errorf("cuan client is not configured and database connection is unavailable")
	}

	var err error
	if status == "" {
		_, err = db.ExecContext(ctx,
			"UPDATE tasks SET summary = CASE WHEN $1 <> '' THEN $1 ELSE summary END, complete_note = CASE WHEN $2 <> '' THEN $2 ELSE complete_note END, updated_at = NOW() WHERE id = $3",
			summary, completeNote, taskID)
	} else {
		_, err = db.ExecContext(ctx,
			"UPDATE tasks SET status = $1, summary = CASE WHEN $2 <> '' THEN $2 ELSE summary END, complete_note = CASE WHEN $3 <> '' THEN $3 ELSE complete_note END, updated_at = NOW() WHERE id = $4",
			status, summary, completeNote, taskID)
	}
	if err != nil {
		return nil, fmt.Errorf("update task: %w", err)
	}

	return map[string]any{
		"task_id":       taskID,
		"status":        status,
		"summary":       summary,
		"complete_note": completeNote,
		"updated_at":    time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// CreateProject creates a new project in Cuan.
func CreateProject(ctx context.Context, name, description, repoURL, branch string) (any, error) {
	if err := taskSessionOwnerWriteError(); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		return client.CreateProject(ctx, name, description, repoURL, branch)
	}
	db := getPlatformDB()
	if db == nil {
		return nil, fmt.Errorf("configure Cuan connection or database before creating projects")
	}
	id := dbPkg.GenerateUUID()
	if branch == "" {
		branch = "main"
	}
	_, err := db.ExecContext(ctx, "INSERT INTO projects (id, name, repo_url, default_branch, created_at, updated_at) VALUES ($1, $2, $3, $4, NOW(), NOW())", id, name, repoURL, branch)
	if err != nil {
		return nil, fmt.Errorf("create project in db: %w", err)
	}
	return map[string]any{
		"id":             id,
		"name":           name,
		"repo_url":       repoURL,
		"default_branch": branch,
		"status":         "created",
	}, nil
}

// UpdateProject updates an existing project in Cuan.
func UpdateProject(ctx context.Context, id, name, description, repoURL, branch string) (any, error) {
	if err := taskSessionOwnerWriteError(); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		return client.UpdateProject(ctx, id, name, description, repoURL, branch)
	}
	db := getPlatformDB()
	if db == nil {
		return nil, fmt.Errorf("configure Cuan connection or database before updating projects")
	}
	_, err := db.ExecContext(ctx,
		"UPDATE projects SET name = CASE WHEN $1 <> '' THEN $1 ELSE name END, repo_url = CASE WHEN $2 <> '' THEN $2 ELSE repo_url END, default_branch = CASE WHEN $3 <> '' THEN $3 ELSE default_branch END, updated_at = NOW() WHERE id = $4 OR LOWER(name) = LOWER($4)",
		name, repoURL, branch, id)
	if err != nil {
		return nil, fmt.Errorf("update project: %w", err)
	}
	return map[string]any{"id": id, "status": "updated"}, nil
}

// GetProjectDeploymentConfig reads deployment configuration for a project.
func GetProjectDeploymentConfig(ctx context.Context, projectID string) (any, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		return client.GetProjectDeploymentConfig(ctx, projectID)
	}
	db := getPlatformDB()
	if db == nil {
		return nil, fmt.Errorf("configure Cuan connection or database before reading project deployment config")
	}
	var id, name string
	var metadataRaw []byte
	err := db.QueryRowContext(ctx,
		"SELECT id, name, COALESCE(metadata, '{}'::jsonb) FROM projects WHERE id = $1 OR LOWER(name) = LOWER($1) LIMIT 1",
		projectID,
	).Scan(&id, &name, &metadataRaw)
	if err == sql.ErrNoRows {
		return map[string]any{"error": "project_not_found", "project_id": projectID}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get project deployment config: %w", err)
	}
	var settings project.Settings
	_ = json.Unmarshal(metadataRaw, &settings)
	return map[string]any{
		"project_id":    id,
		"project_name":  name,
		"auto_deploy":   settings.IsAutoDeployEnabled(),
		"deploy_branch": settings.DeployBranch,
		"deploy_script": settings.DeployScript,
	}, nil
}

// UpdateProjectDeploymentConfig updates deployment configuration for a project.
func UpdateProjectDeploymentConfig(ctx context.Context, projectID string, autoDeploy *bool, branch, script string) (any, error) {
	if err := taskSessionOwnerWriteError(); err != nil {
		return nil, err
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		return client.UpdateProjectDeploymentConfig(ctx, projectID, autoDeploy, branch, script)
	}
	db := getPlatformDB()
	if db == nil {
		return nil, fmt.Errorf("configure Cuan connection or database before updating project deployment config")
	}
	var id, name string
	var metadataRaw []byte
	err := db.QueryRowContext(ctx,
		"SELECT id, name, COALESCE(metadata, '{}'::jsonb) FROM projects WHERE id = $1 OR LOWER(name) = LOWER($1) LIMIT 1",
		projectID,
	).Scan(&id, &name, &metadataRaw)
	if err == sql.ErrNoRows {
		return map[string]any{"error": "project_not_found", "project_id": projectID}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("update project deployment config: %w", err)
	}
	var settings project.Settings
	_ = json.Unmarshal(metadataRaw, &settings)
	if autoDeploy != nil {
		settings.AutoDeploy = autoDeploy
	}
	if strings.TrimSpace(branch) != "" {
		settings.DeployBranch = strings.TrimSpace(branch)
	}
	if script != "" {
		settings.DeployScript = strings.TrimSpace(script)
	}
	updatedMetadata, err := json.Marshal(settings)
	if err != nil {
		return nil, fmt.Errorf("marshal updated settings: %w", err)
	}
	_, err = db.ExecContext(ctx, "UPDATE projects SET metadata = $1, updated_at = NOW() WHERE id = $2", updatedMetadata, id)
	if err != nil {
		return nil, fmt.Errorf("update project metadata: %w", err)
	}
	return map[string]any{
		"message":       "Deployment configuration updated successfully",
		"project_id":    id,
		"project_name":  name,
		"auto_deploy":   settings.IsAutoDeployEnabled(),
		"deploy_branch": settings.DeployBranch,
		"deploy_script": settings.DeployScript,
	}, nil
}

// CreateContact creates a new CRM contact in Cuan.
func CreateContact(ctx context.Context, name, email, phone, role, company, notes string) (any, error) {
	if err := taskSessionOwnerWriteError(); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		return nil, fmt.Errorf("Connect Cuan to create CRM contacts")
	}
	return client.CreateContact(ctx, name, email, phone, role, company, notes)
}

// UpdateContact updates an existing CRM contact in Cuan.
func UpdateContact(ctx context.Context, id, name, email, phone, role, company, notes string) (any, error) {
	if err := taskSessionOwnerWriteError(); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("contact_id is required")
	}
	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		return nil, fmt.Errorf("Connect Cuan to update CRM contacts")
	}
	return client.UpdateContact(ctx, id, name, email, phone, role, company, notes)
}

// ListTeamMembers lists team members and agent personas in Cuan.
func ListTeamMembers(ctx context.Context) (any, error) {
	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		members, err := client.ListTeamMembers(ctx)
		if err == nil {
			return map[string]any{"members": members, "total": len(members)}, nil
		}
	}
	db := getPlatformDB()
	if db != nil {
		rows, err := db.QueryContext(ctx, "SELECT id, name, handle, role, status FROM agents ORDER BY name ASC")
		if err == nil {
			defer rows.Close()
			var agents []map[string]any
			for rows.Next() {
				var id, name, handle, role, status string
				if scanErr := rows.Scan(&id, &name, &handle, &role, &status); scanErr == nil {
					agents = append(agents, map[string]any{
						"id":     id,
						"name":   name,
						"handle": handle,
						"role":   role,
						"status": status,
					})
				}
			}
			return map[string]any{"members": agents, "total": len(agents)}, nil
		}
	}
	return map[string]any{"members": []any{}, "total": 0}, nil
}

// VerifyProviderAccount probes and verifies an AI provider account.
func VerifyProviderAccount(ctx context.Context, accountID string) (any, error) {
	if err := taskSessionOwnerWriteError(); err != nil {
		return nil, err
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, fmt.Errorf("account_id is required")
	}
	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		return nil, fmt.Errorf("Connect Cuan to verify AI provider accounts")
	}
	return client.VerifyProviderAccount(ctx, accountID)
}

// UpdateProviderAccount updates an AI provider account's label, models, or fallback policy.
func UpdateProviderAccount(ctx context.Context, accountID, label, modelID, fallbackModelID string, allowFallback *bool) (any, error) {
	if err := taskSessionOwnerWriteError(); err != nil {
		return nil, err
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, fmt.Errorf("account_id is required")
	}
	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		return nil, fmt.Errorf("Connect Cuan to update AI provider accounts")
	}
	return client.UpdateProviderAccount(ctx, accountID, label, modelID, fallbackModelID, allowFallback)
}

// GetBrandMemory retrieves brand guidelines and taste memory in Cuan.
func GetBrandMemory(ctx context.Context) (any, error) {
	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		return client.GetBrandMemory(ctx)
	}
	db := getPlatformDB()
	if db != nil {
		var content string
		err := db.QueryRowContext(ctx, "SELECT content FROM system_settings WHERE key = 'brand_taste' LIMIT 1").Scan(&content)
		if err == nil {
			return map[string]any{"content": content}, nil
		}
	}
	return map[string]any{"content": ""}, nil
}

// UpdateBrandMemory updates brand guidelines and taste memory in Cuan.
func UpdateBrandMemory(ctx context.Context, guidelines string) (any, error) {
	if err := taskSessionOwnerWriteError(); err != nil {
		return nil, err
	}
	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		return client.UpdateBrandMemory(ctx, guidelines)
	}
	db := getPlatformDB()
	if db == nil {
		return nil, fmt.Errorf("Connect Cuan or configure database to update brand memory")
	}
	_, err := db.ExecContext(ctx, "INSERT INTO system_settings (key, content, updated_at) VALUES ('brand_taste', $1, NOW()) ON CONFLICT (key) DO UPDATE SET content = $1, updated_at = NOW()", guidelines)
	if err != nil {
		return nil, fmt.Errorf("update brand memory: %w", err)
	}
	return map[string]any{"status": "updated", "content": guidelines}, nil
}

// RunTask dispatches an execution attempt for a task in Cuan.
func RunTask(ctx context.Context, taskID, agentID string) (any, error) {
	if err := taskSessionOwnerWriteError(); err != nil {
		return nil, err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}
	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		return nil, fmt.Errorf("Connect Cuan to run tasks")
	}
	return client.RunTask(ctx, taskID, agentID)
}
