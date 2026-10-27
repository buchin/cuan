package diagnostics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

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

// CreateTaskDraft proposes or creates a new task draft for user approval or queueing.
func CreateTaskDraft(ctx context.Context, projectID, title, instructions, priority string, dodChecklist []string) (any, error) {
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
		// Resolve project by ID or Name
		_ = db.QueryRowContext(ctx, "SELECT id FROM projects WHERE id = $1 OR LOWER(name) = LOWER($1) LIMIT 1", projectID).Scan(&realProjectID)
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
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Title     string `json:"title"`
	Topic     string `json:"topic"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ListInboxConversations lists customer chat sessions and project rooms.
func ListInboxConversations(ctx context.Context, projectID string, limit int) (any, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	db := getPlatformDB()
	if db == nil {
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
	db := getPlatformDB()
	if db == nil {
		return map[string]any{
			"messages": []InboxMessage{},
			"total":    0,
			"status":   "database_offline",
		}, nil
	}

	rows, err := db.QueryContext(ctx,
		"SELECT id, room_id, author_type, author_name, content, created_at FROM project_messages WHERE room_id::text = $1 ORDER BY created_at ASC LIMIT $2",
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
