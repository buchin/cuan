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

	dbPkg "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/mcp"
	"github.com/buchin/cuan/internal/project"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	platformDB     *sql.DB
	platformDBOnce sync.Once
)

// SetPlatformDB allows injecting a database connection for tests or custom environments.
func SetPlatformDB(db *sql.DB) {
	platformDB = db
}

func getPlatformDB() *sql.DB {
	platformDBOnce.Do(func() {
		if platformDB != nil {
			return
		}
		dbURL := os.Getenv("DATABASE_URL")
		if dbURL == "" {
			for _, envPath := range []string{
				".env",
				"/opt/cuan/.env",
				"/opt/cuan/current/.env",
			} {
				if data, err := os.ReadFile(envPath); err == nil {
					for _, line := range strings.Split(string(data), "\n") {
						line = strings.TrimSpace(line)
						if strings.HasPrefix(line, "DATABASE_URL=") {
							dbURL = strings.TrimPrefix(line, "DATABASE_URL=")
							dbURL = strings.Trim(dbURL, "\"'")
							break
						}
					}
					if dbURL != "" {
						break
					}
				}
			}
		}
		if dbURL == "" {
			dbURL = "postgres://cuan:cuan@localhost:5432/cuan?sslmode=disable"
		}

		conn, err := sql.Open("pgx", dbURL)
		if err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := conn.PingContext(ctx); err == nil {
			platformDB = conn
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
	db := getPlatformDB()
	if db == nil {
		if client := GetCuanClient(); client != nil && client.IsConfigured() {
			if projects, err := client.ListProjects(ctx, limit); err == nil {
				return map[string]any{
					"projects": redactProjectEnvironments(projects),
					"total":    len(projects),
				}, nil
			}
		}
		return map[string]any{
			"projects": []ProjectSummary{},
			"total":    0,
			"status":   "database_offline",
		}, nil
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
	db := getPlatformDB()
	if db == nil {
		if client := GetCuanClient(); client != nil && client.IsConfigured() {
			if proj, err := client.GetProject(ctx, projectID); err == nil {
				redactProjectEnvironment(proj)
				return proj, nil
			}
		}
		return map[string]any{
			"id":     projectID,
			"status": "database_offline",
		}, nil
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

// ListTasks lists tasks with optional project and status filters.
func ListTasks(ctx context.Context, projectID, status string, limit int) (any, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	db := getPlatformDB()
	if db == nil {
		if client := GetCuanClient(); client != nil && client.IsConfigured() {
			if tasks, err := client.ListTasks(ctx, projectID, status, limit); err == nil {
				for _, task := range tasks {
					redactTaskDiagnostic(task)
				}
				return map[string]any{
					"tasks": tasks,
					"total": len(tasks),
				}, nil
			}
		}
		return map[string]any{
			"tasks":  []TaskSummary{},
			"total":  0,
			"status": "database_offline",
		}, nil
	}

	query := "SELECT id, project_id, title, instructions, status, priority, summary, complete_note, dod_checklist, created_at, updated_at FROM tasks WHERE 1=1"
	args := []any{}
	argIdx := 1

	if projectID != "" {
		query += fmt.Sprintf(" AND (project_id = $%d OR project_id IN (SELECT id FROM projects WHERE LOWER(name) = LOWER($%d)))", argIdx, argIdx)
		args = append(args, projectID)
		argIdx++
	}
	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}
	query += fmt.Sprintf(" ORDER BY updated_at DESC LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []TaskSummary
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
		t.Instructions = project.TaskScopeInstructions(t.Instructions)
		t.UpdatedAt = updatedAt.Format(time.RFC3339)
		tasks = append(tasks, t)
	}
	if tasks == nil {
		tasks = []TaskSummary{}
	}

	return map[string]any{
		"tasks": tasks,
		"total": len(tasks),
	}, nil
}

// GetTask returns details of a single task.
func GetTask(ctx context.Context, taskID string) (any, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}
	db := getPlatformDB()
	if db == nil {
		if client := GetCuanClient(); client != nil && client.IsConfigured() {
			if t, err := client.GetTask(ctx, taskID); err == nil {
				redactTaskDiagnostic(t)
				return t, nil
			}
		}
		return map[string]any{
			"id":     taskID,
			"status": "database_offline",
		}, nil
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
	t.Instructions = project.TaskScopeInstructions(t.Instructions)
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

	return map[string]any{
		"task":     t,
		"attempts": attempts,
	}, nil
}

// Diagnostic consumers need approved scope, not the frozen execution environment.
// This operates only on the response; stored context and worker reads stay intact.
func redactTaskDiagnostic(value map[string]any) {
	for key, item := range value {
		if text, ok := item.(string); ok && key == "instructions" {
			value[key] = project.TaskScopeInstructions(text)
		}
		if nested, ok := item.(map[string]any); ok {
			redactTaskDiagnostic(nested)
		}
	}
}

// CreateTaskDraft proposes or creates a new task draft for user approval or queueing.
func CreateTaskDraft(ctx context.Context, projectID, title, instructions, priority string, dodChecklist []string) (any, error) {
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

	db := getPlatformDB()
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
		_, err := db.ExecContext(ctx,
			"INSERT INTO tasks (id, project_id, title, instructions, status, priority, dod_checklist, created_at, updated_at) VALUES ($1, $2, $3, $4, 'backlog', $5, $6, NOW(), NOW())",
			taskID, realProjectID, title, instructions, priority, dodJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("insert task: %w", err)
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
	db := getPlatformDB()
	if db == nil {
		if client := GetCuanClient(); client != nil && client.IsConfigured() {
			if convs, err := client.ListConversations(ctx, projectID, limit); err == nil {
				return map[string]any{
					"conversations": convs,
					"total":         len(convs),
				}, nil
			}
		}
		return map[string]any{
			"conversations": []InboxConversation{},
			"total":         0,
			"status":        "database_offline",
		}, nil
	}

	query := "SELECT id, project_id, title, topic, created_at, updated_at FROM project_rooms WHERE 1=1"
	args := []any{}
	argIdx := 1
	if projectID != "" {
		query += fmt.Sprintf(" AND (project_id = $%d OR project_id IN (SELECT id FROM projects WHERE LOWER(name) = LOWER($%d)))", argIdx, argIdx)
		args = append(args, projectID)
		argIdx++
	}
	query += fmt.Sprintf(" ORDER BY updated_at DESC LIMIT $%d", argIdx)
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
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Title, &c.Topic, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		c.CreatedAt = createdAt.Format(time.RFC3339)
		c.UpdatedAt = updatedAt.Format(time.RFC3339)

		var tc, at int
		_ = db.QueryRowContext(ctx, "SELECT COUNT(*), COUNT(CASE WHEN status != 'done' THEN 1 END) FROM tasks WHERE room_id = $1", c.ID).Scan(&tc, &at)
		c.TaskCount = tc
		c.ActiveTasks = at

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
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	if client := GetCuanClient(); client != nil && client.IsConfigured() {
		return client.GetConversationSnapshot(ctx, roomID, limit)
	}
	db := getPlatformDB()
	if db == nil {
		return nil, fmt.Errorf("configure the Cuan API to read conversation state")
	}

	rows, err := db.QueryContext(ctx,
		"SELECT id, room_id, author_type, author_name, content, created_at FROM (SELECT id, room_id, author_type, author_name, content, created_at FROM project_messages WHERE room_id::text = $1 ORDER BY created_at DESC LIMIT $2) AS sub ORDER BY created_at ASC",
		roomID, limit,
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

// ListContacts lists CRM contacts and customer profiles.
func ListContacts(ctx context.Context, query, product string, limit int) (any, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	db := getPlatformDB()
	if db == nil {
		return map[string]any{
			"contacts": []ContactSummary{},
			"total":    0,
			"status":   "database_offline",
		}, nil
	}

	sqlQuery := "SELECT id, product, guest_uuid, session_uuid, name, email, tags, created_at, updated_at FROM crm_contacts WHERE 1=1"
	args := []any{}
	argIdx := 1

	if product != "" {
		sqlQuery += fmt.Sprintf(" AND product = $%d", argIdx)
		args = append(args, product)
		argIdx++
	}
	if query != "" {
		sqlQuery += fmt.Sprintf(" AND (name ILIKE $%d OR email ILIKE $%d OR guest_uuid ILIKE $%d)", argIdx, argIdx, argIdx)
		args = append(args, "%"+query+"%")
		argIdx++
	}
	sqlQuery += fmt.Sprintf(" ORDER BY updated_at DESC LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query contacts: %w", err)
	}
	defer rows.Close()

	var contacts []ContactSummary
	for rows.Next() {
		var c ContactSummary
		var tagsRaw []byte
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&c.ID, &c.Product, &c.GuestUUID, &c.SessionUUID, &c.Name, &c.Email, &tagsRaw, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		if len(tagsRaw) > 0 {
			_ = json.Unmarshal(tagsRaw, &c.Tags)
		}
		c.CreatedAt = createdAt.Format(time.RFC3339)
		c.UpdatedAt = updatedAt.Format(time.RFC3339)
		contacts = append(contacts, c)
	}
	if contacts == nil {
		contacts = []ContactSummary{}
	}

	return map[string]any{
		"contacts": contacts,
		"total":    len(contacts),
	}, nil
}

// GetContact returns customer contact details and relationship memory notes.
func GetContact(ctx context.Context, contactID string) (any, error) {
	contactID = strings.TrimSpace(contactID)
	if contactID == "" {
		return nil, fmt.Errorf("contact_id is required")
	}
	db := getPlatformDB()
	if db == nil {
		return map[string]any{
			"id":     contactID,
			"status": "database_offline",
		}, nil
	}

	var c ContactSummary
	var tagsRaw []byte
	var createdAt, updatedAt time.Time
	err := db.QueryRowContext(ctx,
		"SELECT id, product, guest_uuid, session_uuid, name, email, tags, created_at, updated_at FROM crm_contacts WHERE id::text = $1 OR guest_uuid = $1 LIMIT 1",
		contactID,
	).Scan(&c.ID, &c.Product, &c.GuestUUID, &c.SessionUUID, &c.Name, &c.Email, &tagsRaw, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return map[string]any{
			"error":      "contact_not_found",
			"contact_id": contactID,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get contact: %w", err)
	}

	if len(tagsRaw) > 0 {
		_ = json.Unmarshal(tagsRaw, &c.Tags)
	}
	c.CreatedAt = createdAt.Format(time.RFC3339)
	c.UpdatedAt = updatedAt.Format(time.RFC3339)

	// Fetch relationship memory notes
	rows, _ := db.QueryContext(ctx, "SELECT id, kind, body, recorded_by, created_at FROM contact_memory WHERE contact_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC", c.ID)
	var memories []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var memID, kind, body, recordedBy string
			var memCreated time.Time
			if err := rows.Scan(&memID, &kind, &body, &recordedBy, &memCreated); err == nil {
				memories = append(memories, map[string]any{
					"id":          memID,
					"kind":        kind,
					"body":        body,
					"recorded_by": recordedBy,
					"created_at":  memCreated.Format(time.RFC3339),
				})
			}
		}
	}
	if memories == nil {
		memories = []map[string]any{}
	}

	return map[string]any{
		"contact":  c,
		"memories": memories,
	}, nil
}

// ListScheduledJobs returns scheduled background jobs and recurring automations.
func ListScheduledJobs(ctx context.Context) (any, error) {
	jobs := []map[string]any{
		{
			"name":        "inbox_dispatcher",
			"description": "Orchestrates visitor messages, agent assignment and queue failover",
			"interval":    "continuous",
			"status":      "active",
		},
		{
			"name":        "notification_dispatcher",
			"description": "Processes outbound chat notifications, webhooks and alerts",
			"interval":    "continuous",
			"status":      "active",
		},
		{
			"name":        "workspace_retention",
			"description": "Prunes expired task worktrees, temporary mounts and cached diffs",
			"interval":    "hourly",
			"status":      "active",
		},
		{
			"name":        "github_recovery",
			"description": "Reconciles diverged git branches, unpushed commits and repository sync",
			"interval":    "every 10m",
			"status":      "active",
		},
		{
			"name":        "health_poller",
			"description": "Monitors remote worker heartbeats, daemon availability and resource limits",
			"interval":    "every 30s",
			"status":      "active",
		},
		{
			"name":        "dns_poller",
			"description": "Verifies custom domains, TLS certificates and Caddy reverse proxy routing",
			"interval":    "every 5m",
			"status":      "active",
		},
	}

	return map[string]any{
		"jobs":  jobs,
		"total": len(jobs),
	}, nil
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
		baseDomain = "cuan.dojo.cc"
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

	return map[string]any{
		"id":              "topic_" + roomID,
		"room_id":         roomID,
		"conversation_id": roomID,
		"agent_id":        agentID,
		"project_id":      projectID,
		"title":           title,
		"topic":           topic,
		"status":          "created",
		"created_at":      now.Format(time.RFC3339),
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
