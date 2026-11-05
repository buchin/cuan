package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrRoomNotFound           = errors.New("project room not found")
	ErrMessageNotFound        = errors.New("project message not found")
	ErrTaskNotFound           = errors.New("project task not found")
	ErrInvalidUUID            = errors.New("invalid uuid format")
	ErrInvalidAuthorType      = errors.New("invalid author type: must be user, agent, or system")
	ErrInvalidTaskStatus      = errors.New("invalid task status: must be backlog, todo, doing, review, or done")
	ErrInvalidTaskPriority    = errors.New("invalid task priority: must be low, normal, high, or urgent")
	ErrInvalidExecutionStatus = errors.New("invalid execution status: must be idle, running, completed, or failed")
	ErrEmptyRoomTitle         = errors.New("room title cannot be empty")
	ErrEmptyTaskTitle         = errors.New("task title cannot be empty")
	ErrEmptyMessageContent    = errors.New("message content cannot be empty")
	ErrDuplicateRoom          = errors.New("project room already exists")
	ErrDuplicateMessage       = errors.New("project message already exists")
	ErrDuplicateTask          = errors.New("project task already exists")
)

var uuidRegex = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

// IsValidUUID validates standard RFC 4122 UUID string format.
func IsValidUUID(u string) bool {
	return uuidRegex.MatchString(strings.TrimSpace(u))
}

// GenerateUUID produces a cryptographically secure RFC 4122 v4 UUID string.
func GenerateUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		now := time.Now().UnixNano()
		return fmt.Sprintf("%08x-%04x-4%03x-%04x-%012x",
			uint32(now),
			uint16(now>>32),
			uint16(now>>48)&0x0fff,
			0x8000|(uint16(now>>16)&0x3fff),
			uint64(now)&0xffffffffffff)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// NormalizeAuthorType validates and returns lowercase author type.
func NormalizeAuthorType(authorType string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(authorType))
	switch norm {
	case "user", "agent", "system":
		return norm, nil
	default:
		return "", ErrInvalidAuthorType
	}
}

// NormalizeTaskStatus validates and returns lowercase task status.
func NormalizeTaskStatus(status string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(status))
	switch norm {
	case "backlog", "todo", "queued", "doing", "review", "done", "failed", "blocked":
		return norm, nil
	default:
		return "", ErrInvalidTaskStatus
	}
}

// NormalizeTaskPriority validates and returns lowercase task priority.
func NormalizeTaskPriority(priority string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(priority))
	switch norm {
	case "low", "normal", "high", "urgent":
		return norm, nil
	default:
		return "", ErrInvalidTaskPriority
	}
}

// NormalizeTaskExecutionStatus validates and returns lowercase task execution status.
func NormalizeTaskExecutionStatus(status string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(status))
	if norm == "" {
		return "idle", nil
	}
	switch norm {
	case "idle", "running", "completed", "failed":
		return norm, nil
	default:
		return "", ErrInvalidExecutionStatus
	}
}

// ProjectRoom represents a collaboration and discussion room bound to a project.
type ProjectRoom struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Title     string    `json:"title"`
	Topic     string    `json:"topic"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProjectRoomDTO is the public Data Transfer Object for ProjectRoom.
type ProjectRoomDTO struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Title     string    `json:"title"`
	Topic     string    `json:"topic"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ToDTO converts a ProjectRoom to its public DTO.
func (r *ProjectRoom) ToDTO() ProjectRoomDTO {
	return ProjectRoomDTO{
		ID:        r.ID,
		ProjectID: r.ProjectID,
		Title:     r.Title,
		Topic:     r.Topic,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

// ProjectMessage represents an individual chat or agent interaction message within a room.
type ProjectMessage struct {
	ID         string    `json:"id"`
	RoomID     string    `json:"room_id"`
	AuthorType string    `json:"author_type"` // 'user', 'agent', 'system'
	AuthorID   string    `json:"author_id"`
	AuthorName string    `json:"author_name"`
	Content    string    `json:"content"`
	ReplyToID  *string   `json:"reply_to_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ProjectMessageDTO is the public Data Transfer Object for ProjectMessage.
type ProjectMessageDTO struct {
	ID         string    `json:"id"`
	RoomID     string    `json:"room_id"`
	AuthorType string    `json:"author_type"`
	AuthorID   string    `json:"author_id"`
	AuthorName string    `json:"author_name"`
	Content    string    `json:"content"`
	ReplyToID  *string   `json:"reply_to_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ToDTO converts a ProjectMessage to its public DTO.
func (m *ProjectMessage) ToDTO() ProjectMessageDTO {
	return ProjectMessageDTO{
		ID:         m.ID,
		RoomID:     m.RoomID,
		AuthorType: m.AuthorType,
		AuthorID:   m.AuthorID,
		AuthorName: m.AuthorName,
		Content:    m.Content,
		ReplyToID:  m.ReplyToID,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
}

// DoDItem represents a single checklist item in a task's Definition of Done.
type DoDItem struct {
	ID        string `json:"id,omitempty"`
	Text      string `json:"text"`
	Completed bool   `json:"completed"`
}

// ProjectTask represents a structured task with fan-out hierarchy and multi-project batch capabilities.
type ProjectTask struct {
	ID              string     `json:"id"`
	ProjectID       string     `json:"project_id"`
	ProjectIDs      []string   `json:"project_ids,omitempty"`
	RoomID          *string    `json:"room_id,omitempty"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	Status          string     `json:"status"`   // 'backlog', 'doing', 'review', 'done'
	Priority        string     `json:"priority"` // 'low', 'normal', 'high', 'urgent'
	DoDChecklist    []DoDItem  `json:"dod_checklist"`
	AssigneeID      *string    `json:"assignee_id,omitempty"`
	ParentTaskID    *string    `json:"parent_task_id,omitempty"` // For fan-out hierarchy
	BatchID         *string    `json:"batch_id,omitempty"`       // For multi-project batch grouping
	Tags            []string   `json:"tags"`                     // For tag selectors e.g. ["wordpress", "adsense"]
	AgentOutput     string     `json:"agent_output"`             // Generated agent output/deliverables
	ExecutionStatus string     `json:"execution_status"`         // 'idle', 'running', 'completed', 'failed'
	ScheduledAt     *time.Time `json:"scheduled_at,omitempty"`   // Publication schedule
	PublishedAt     *time.Time `json:"published_at,omitempty"`   // Publication timestamp
	PublicationURL  string     `json:"publication_url"`          // URL of published post/article
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ProjectTaskDTO is the public Data Transfer Object for ProjectTask.
type ProjectTaskDTO struct {
	ID              string     `json:"id"`
	ProjectID       string     `json:"project_id"`
	ProjectIDs      []string   `json:"project_ids,omitempty"`
	RoomID          *string    `json:"room_id,omitempty"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	Status          string     `json:"status"`
	Priority        string     `json:"priority"`
	DoDChecklist    []DoDItem  `json:"dod_checklist"`
	AssigneeID      *string    `json:"assignee_id,omitempty"`
	ParentTaskID    *string    `json:"parent_task_id,omitempty"`
	BatchID         *string    `json:"batch_id,omitempty"`
	Tags            []string   `json:"tags"`
	AgentOutput     string     `json:"agent_output"`
	ExecutionStatus string     `json:"execution_status"`
	ScheduledAt     *time.Time `json:"scheduled_at,omitempty"`
	PublishedAt     *time.Time `json:"published_at,omitempty"`
	PublicationURL  string     `json:"publication_url"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// ToDTO converts a ProjectTask to its public DTO.
func (t *ProjectTask) ToDTO() ProjectTaskDTO {
	tagsCopy := make([]string, len(t.Tags))
	copy(tagsCopy, t.Tags)
	checklistCopy := make([]DoDItem, len(t.DoDChecklist))
	copy(checklistCopy, t.DoDChecklist)

	execStatus := t.ExecutionStatus
	if execStatus == "" {
		execStatus = "idle"
	}

	pidsCopy := make([]string, len(t.ProjectIDs))
	copy(pidsCopy, t.ProjectIDs)
	if len(pidsCopy) == 0 && t.ProjectID != "" {
		pidsCopy = []string{t.ProjectID}
	}

	var scheduledAt *time.Time
	if t.ScheduledAt != nil {
		st := *t.ScheduledAt
		scheduledAt = &st
	}
	var publishedAt *time.Time
	if t.PublishedAt != nil {
		pt := *t.PublishedAt
		publishedAt = &pt
	}

	return ProjectTaskDTO{
		ID:              t.ID,
		ProjectID:       t.ProjectID,
		ProjectIDs:      pidsCopy,
		RoomID:          t.RoomID,
		Title:           t.Title,
		Description:     t.Description,
		Status:          t.Status,
		Priority:        t.Priority,
		DoDChecklist:    checklistCopy,
		AssigneeID:      t.AssigneeID,
		ParentTaskID:    t.ParentTaskID,
		BatchID:         t.BatchID,
		Tags:            tagsCopy,
		AgentOutput:     t.AgentOutput,
		ExecutionStatus: execStatus,
		ScheduledAt:     scheduledAt,
		PublishedAt:     publishedAt,
		PublicationURL:  t.PublicationURL,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
}

// ProjectTaskFilter holds criteria for filtering project tasks.
type ProjectTaskFilter struct {
	ProjectID       string
	RoomID          *string
	Status          string
	Priority        string
	ParentTaskID    *string
	BatchID         *string
	Tag             string
	AssigneeID      *string
	ExecutionStatus string
}

// ProjectStudioRepository defines data operations for Project Rooms, Messages, and Tasks.
type ProjectStudioRepository interface {
	// Rooms
	CreateRoom(ctx context.Context, room *ProjectRoom) error
	GetRoomByID(ctx context.Context, id string) (*ProjectRoom, error)
	ListRoomsByProjectID(ctx context.Context, projectID string) ([]ProjectRoom, error)
	ListAllRooms(ctx context.Context) ([]ProjectRoom, error)
	UpdateRoom(ctx context.Context, room *ProjectRoom) error
	DeleteRoom(ctx context.Context, id string) error

	// Messages
	CreateMessage(ctx context.Context, msg *ProjectMessage) error
	GetMessageByID(ctx context.Context, id string) (*ProjectMessage, error)
	ListMessagesByRoomID(ctx context.Context, roomID string) ([]ProjectMessage, error)
	UpdateMessage(ctx context.Context, msg *ProjectMessage) error
	DeleteMessage(ctx context.Context, id string) error

	// Tasks
	CreateTask(ctx context.Context, task *ProjectTask) error
	GetTaskByID(ctx context.Context, id string) (*ProjectTask, error)
	ListTasksByProjectID(ctx context.Context, projectID string) ([]ProjectTask, error)
	ListTasksByRoomID(ctx context.Context, roomID string) ([]ProjectTask, error)
	ListTasksByParentID(ctx context.Context, parentTaskID string) ([]ProjectTask, error)
	ListTasksByBatchID(ctx context.Context, batchID string) ([]ProjectTask, error)
	ListTasksByTag(ctx context.Context, tag string) ([]ProjectTask, error)
	ListTasks(ctx context.Context, filter ProjectTaskFilter) ([]ProjectTask, error)
	UpdateTask(ctx context.Context, task *ProjectTask) error
	UpdateTaskStatus(ctx context.Context, id string, status string) error
	UpdateTaskExecution(ctx context.Context, id string, executionStatus string, agentOutput string) error
	DeleteTask(ctx context.Context, id string) error
}

// SQLProjectStudioRepository implements ProjectStudioRepository using PostgreSQL.
type SQLProjectStudioRepository struct {
	db *sql.DB
}

// NewSQLProjectStudioRepository creates a new SQLProjectStudioRepository instance.
func NewSQLProjectStudioRepository(db *sql.DB) *SQLProjectStudioRepository {
	return &SQLProjectStudioRepository{db: db}
}

// CreateRoom inserts a new project room.
func (r *SQLProjectStudioRepository) CreateRoom(ctx context.Context, room *ProjectRoom) error {
	if room == nil {
		return errors.New("project room cannot be nil")
	}
	if strings.TrimSpace(room.ProjectID) == "" {
		return ErrEmptyProjectID
	}
	if strings.TrimSpace(room.Title) == "" {
		return ErrEmptyRoomTitle
	}
	if strings.TrimSpace(room.ID) == "" {
		room.ID = GenerateUUID()
	} else if !IsValidUUID(room.ID) {
		return fmt.Errorf("%w: room id %s", ErrInvalidUUID, room.ID)
	}

	now := time.Now().UTC()
	if room.CreatedAt.IsZero() {
		room.CreatedAt = now
	}
	room.UpdatedAt = now

	query := `
		INSERT INTO project_rooms (id, project_id, title, topic, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.ExecContext(ctx, query,
		room.ID,
		room.ProjectID,
		room.Title,
		room.Topic,
		room.CreatedAt,
		room.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") ||
			strings.Contains(strings.ToLower(err.Error()), "duplicate") ||
			strings.Contains(err.Error(), "23505") {
			return ErrDuplicateRoom
		}
		return fmt.Errorf("create project room: %w", err)
	}
	return nil
}

// GetRoomByID retrieves a project room by its UUID.
func (r *SQLProjectStudioRepository) GetRoomByID(ctx context.Context, id string) (*ProjectRoom, error) {
	if !IsValidUUID(id) {
		return nil, fmt.Errorf("%w: room id %s", ErrInvalidUUID, id)
	}

	query := `
		SELECT id, project_id, title, topic, created_at, updated_at
		FROM project_rooms
		WHERE id = $1
	`
	var room ProjectRoom
	row := r.db.QueryRowContext(ctx, query, id)
	err := row.Scan(
		&room.ID,
		&room.ProjectID,
		&room.Title,
		&room.Topic,
		&room.CreatedAt,
		&room.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRoomNotFound
		}
		return nil, fmt.Errorf("get project room by id: %w", err)
	}
	return &room, nil
}

// ListRoomsByProjectID lists all rooms belonging to a project ordered by creation time.
func (r *SQLProjectStudioRepository) ListRoomsByProjectID(ctx context.Context, projectID string) ([]ProjectRoom, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, ErrEmptyProjectID
	}

	query := `
		SELECT id, project_id, title, topic, created_at, updated_at
		FROM project_rooms
		WHERE project_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project rooms: %w", err)
	}
	defer rows.Close()

	var list []ProjectRoom
	for rows.Next() {
		var room ProjectRoom
		if err := rows.Scan(
			&room.ID,
			&room.ProjectID,
			&room.Title,
			&room.Topic,
			&room.CreatedAt,
			&room.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan project room: %w", err)
		}
		list = append(list, room)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project rooms: %w", err)
	}
	return list, nil
}

// ListAllRooms lists all project rooms across all projects, ordered by updated_at desc.
func (r *SQLProjectStudioRepository) ListAllRooms(ctx context.Context) ([]ProjectRoom, error) {
	query := `
		SELECT id, project_id, title, topic, created_at, updated_at
		FROM project_rooms
		ORDER BY updated_at DESC, created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list all project rooms: %w", err)
	}
	defer rows.Close()

	var list []ProjectRoom
	for rows.Next() {
		var room ProjectRoom
		if err := rows.Scan(
			&room.ID,
			&room.ProjectID,
			&room.Title,
			&room.Topic,
			&room.CreatedAt,
			&room.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan project room: %w", err)
		}
		list = append(list, room)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate all project rooms: %w", err)
	}
	return list, nil
}

// UpdateRoom updates a room's title and topic.
func (r *SQLProjectStudioRepository) UpdateRoom(ctx context.Context, room *ProjectRoom) error {
	if room == nil {
		return errors.New("project room cannot be nil")
	}
	if !IsValidUUID(room.ID) {
		return fmt.Errorf("%w: room id %s", ErrInvalidUUID, room.ID)
	}
	if strings.TrimSpace(room.Title) == "" {
		return ErrEmptyRoomTitle
	}

	now := time.Now().UTC()
	room.UpdatedAt = now

	projectID := strings.TrimSpace(room.ProjectID)
	if projectID == "" {
		projectID = "default"
	}

	query := `
		UPDATE project_rooms
		SET title = $2, topic = $3, project_id = $4, updated_at = $5
		WHERE id = $1
	`
	res, err := r.db.ExecContext(ctx, query, room.ID, room.Title, room.Topic, projectID, room.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update project room: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update project room rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrRoomNotFound
	}
	return nil
}

// DeleteRoom deletes a room by ID (cascades to messages and sets room_id to null on tasks).
func (r *SQLProjectStudioRepository) DeleteRoom(ctx context.Context, id string) error {
	if !IsValidUUID(id) {
		return fmt.Errorf("%w: room id %s", ErrInvalidUUID, id)
	}

	query := `DELETE FROM project_rooms WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete project room: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete project room rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrRoomNotFound
	}
	return nil
}

// CreateMessage inserts a new message in a room.
func (r *SQLProjectStudioRepository) CreateMessage(ctx context.Context, msg *ProjectMessage) error {
	if msg == nil {
		return errors.New("project message cannot be nil")
	}
	if !IsValidUUID(msg.RoomID) {
		return fmt.Errorf("%w: room id %s", ErrInvalidUUID, msg.RoomID)
	}
	if strings.TrimSpace(msg.Content) == "" {
		return ErrEmptyMessageContent
	}
	normAuthor, err := NormalizeAuthorType(msg.AuthorType)
	if err != nil {
		return err
	}
	msg.AuthorType = normAuthor

	if strings.TrimSpace(msg.ID) == "" {
		msg.ID = GenerateUUID()
	} else if !IsValidUUID(msg.ID) {
		return fmt.Errorf("%w: message id %s", ErrInvalidUUID, msg.ID)
	}

	if msg.ReplyToID != nil && strings.TrimSpace(*msg.ReplyToID) != "" {
		if !IsValidUUID(*msg.ReplyToID) {
			return fmt.Errorf("%w: reply_to_id %s", ErrInvalidUUID, *msg.ReplyToID)
		}
	} else {
		msg.ReplyToID = nil
	}

	now := time.Now().UTC()
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = now
	}
	msg.UpdatedAt = now

	query := `
		INSERT INTO project_messages (
			id, room_id, author_type, author_id, author_name, content, reply_to_id, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err = r.db.ExecContext(ctx, query,
		msg.ID,
		msg.RoomID,
		msg.AuthorType,
		msg.AuthorID,
		msg.AuthorName,
		msg.Content,
		msg.ReplyToID,
		msg.CreatedAt,
		msg.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") ||
			strings.Contains(strings.ToLower(err.Error()), "duplicate") ||
			strings.Contains(err.Error(), "23505") {
			return ErrDuplicateMessage
		}
		return fmt.Errorf("create project message: %w", err)
	}
	return nil
}

// GetMessageByID retrieves a project message by ID.
func (r *SQLProjectStudioRepository) GetMessageByID(ctx context.Context, id string) (*ProjectMessage, error) {
	if !IsValidUUID(id) {
		return nil, fmt.Errorf("%w: message id %s", ErrInvalidUUID, id)
	}

	query := `
		SELECT id, room_id, author_type, author_id, author_name, content, reply_to_id, created_at, updated_at
		FROM project_messages
		WHERE id = $1
	`
	var msg ProjectMessage
	row := r.db.QueryRowContext(ctx, query, id)
	err := row.Scan(
		&msg.ID,
		&msg.RoomID,
		&msg.AuthorType,
		&msg.AuthorID,
		&msg.AuthorName,
		&msg.Content,
		&msg.ReplyToID,
		&msg.CreatedAt,
		&msg.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMessageNotFound
		}
		return nil, fmt.Errorf("get project message by id: %w", err)
	}
	return &msg, nil
}

// ListMessagesByRoomID lists all messages in a room chronologically.
func (r *SQLProjectStudioRepository) ListMessagesByRoomID(ctx context.Context, roomID string) ([]ProjectMessage, error) {
	if !IsValidUUID(roomID) {
		return nil, fmt.Errorf("%w: room id %s", ErrInvalidUUID, roomID)
	}

	query := `
		SELECT id, room_id, author_type, author_id, author_name, content, reply_to_id, created_at, updated_at
		FROM project_messages
		WHERE room_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, roomID)
	if err != nil {
		return nil, fmt.Errorf("list project messages: %w", err)
	}
	defer rows.Close()

	var list []ProjectMessage
	for rows.Next() {
		var msg ProjectMessage
		if err := rows.Scan(
			&msg.ID,
			&msg.RoomID,
			&msg.AuthorType,
			&msg.AuthorID,
			&msg.AuthorName,
			&msg.Content,
			&msg.ReplyToID,
			&msg.CreatedAt,
			&msg.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan project message: %w", err)
		}
		list = append(list, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project messages: %w", err)
	}
	return list, nil
}

// UpdateMessage updates a message's content.
func (r *SQLProjectStudioRepository) UpdateMessage(ctx context.Context, msg *ProjectMessage) error {
	if msg == nil {
		return errors.New("project message cannot be nil")
	}
	if !IsValidUUID(msg.ID) {
		return fmt.Errorf("%w: message id %s", ErrInvalidUUID, msg.ID)
	}
	if strings.TrimSpace(msg.Content) == "" {
		return ErrEmptyMessageContent
	}

	now := time.Now().UTC()
	msg.UpdatedAt = now

	query := `
		UPDATE project_messages
		SET content = $2, updated_at = $3
		WHERE id = $1
	`
	res, err := r.db.ExecContext(ctx, query, msg.ID, msg.Content, msg.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update project message: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update project message rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrMessageNotFound
	}
	return nil
}

// DeleteMessage deletes a message by ID.
func (r *SQLProjectStudioRepository) DeleteMessage(ctx context.Context, id string) error {
	if !IsValidUUID(id) {
		return fmt.Errorf("%w: message id %s", ErrInvalidUUID, id)
	}

	query := `DELETE FROM project_messages WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete project message: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete project message rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrMessageNotFound
	}
	return nil
}

// CreateTask inserts a new project task.
func (r *SQLProjectStudioRepository) CreateTask(ctx context.Context, task *ProjectTask) error {
	if task == nil {
		return errors.New("project task cannot be nil")
	}
	if strings.TrimSpace(task.ProjectID) == "" {
		return ErrEmptyProjectID
	}
	if strings.TrimSpace(task.Title) == "" {
		return ErrEmptyTaskTitle
	}

	normStatus, err := NormalizeTaskStatus(task.Status)
	if err != nil {
		if strings.TrimSpace(task.Status) == "" {
			normStatus = "backlog"
		} else {
			return err
		}
	}
	task.Status = normStatus

	normPriority, err := NormalizeTaskPriority(task.Priority)
	if err != nil {
		if strings.TrimSpace(task.Priority) == "" {
			normPriority = "normal"
		} else {
			return err
		}
	}
	task.Priority = normPriority

	if strings.TrimSpace(task.ID) == "" {
		task.ID = GenerateUUID()
	} else if !IsValidUUID(task.ID) {
		return fmt.Errorf("%w: task id %s", ErrInvalidUUID, task.ID)
	}

	if task.RoomID != nil && strings.TrimSpace(*task.RoomID) != "" {
		if !IsValidUUID(*task.RoomID) {
			return fmt.Errorf("%w: room id %s", ErrInvalidUUID, *task.RoomID)
		}
	} else {
		task.RoomID = nil
	}

	if task.ParentTaskID != nil && strings.TrimSpace(*task.ParentTaskID) != "" {
		if !IsValidUUID(*task.ParentTaskID) {
			return fmt.Errorf("%w: parent_task_id %s", ErrInvalidUUID, *task.ParentTaskID)
		}
	} else {
		task.ParentTaskID = nil
	}

	if task.DoDChecklist == nil {
		task.DoDChecklist = []DoDItem{}
	}
	dodBytes, err := json.Marshal(task.DoDChecklist)
	if err != nil {
		return fmt.Errorf("marshal dod_checklist: %w", err)
	}

	if task.Tags == nil {
		task.Tags = []string{}
	}
	tagsBytes, err := json.Marshal(task.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	normExecution, err := NormalizeTaskExecutionStatus(task.ExecutionStatus)
	if err != nil {
		return err
	}
	task.ExecutionStatus = normExecution

	now := time.Now().UTC()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now

	query := `
		INSERT INTO project_tasks (
			id, project_id, room_id, title, description, status, priority,
			dod_checklist, assignee_id, parent_task_id, batch_id, tags,
			agent_output, execution_status,
			scheduled_at, published_at, publication_url,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
	`
	_, err = r.db.ExecContext(ctx, query,
		task.ID,
		task.ProjectID,
		task.RoomID,
		task.Title,
		task.Description,
		task.Status,
		task.Priority,
		dodBytes,
		task.AssigneeID,
		task.ParentTaskID,
		task.BatchID,
		tagsBytes,
		task.AgentOutput,
		task.ExecutionStatus,
		task.ScheduledAt,
		task.PublishedAt,
		task.PublicationURL,
		task.CreatedAt,
		task.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") ||
			strings.Contains(strings.ToLower(err.Error()), "duplicate") ||
			strings.Contains(err.Error(), "23505") {
			return ErrDuplicateTask
		}
		return fmt.Errorf("create project task: %w", err)
	}
	return nil
}

// GetTaskByID retrieves a project task by its UUID.
func (r *SQLProjectStudioRepository) GetTaskByID(ctx context.Context, id string) (*ProjectTask, error) {
	if !IsValidUUID(id) {
		return nil, fmt.Errorf("%w: task id %s", ErrInvalidUUID, id)
	}

	query := `
		SELECT id, project_id, room_id, title, description, status, priority,
		       dod_checklist, assignee_id, parent_task_id, batch_id, tags,
		       COALESCE(agent_output, ''), COALESCE(execution_status, 'idle'),
		       scheduled_at, published_at, COALESCE(publication_url, ''),
		       created_at, updated_at
		FROM project_tasks
		WHERE id = $1
	`
	var task ProjectTask
	var dodBytes, tagsBytes []byte
	var scheduledAt, publishedAt sql.NullTime

	row := r.db.QueryRowContext(ctx, query, id)
	err := row.Scan(
		&task.ID,
		&task.ProjectID,
		&task.RoomID,
		&task.Title,
		&task.Description,
		&task.Status,
		&task.Priority,
		&dodBytes,
		&task.AssigneeID,
		&task.ParentTaskID,
		&task.BatchID,
		&tagsBytes,
		&task.AgentOutput,
		&task.ExecutionStatus,
		&scheduledAt,
		&publishedAt,
		&task.PublicationURL,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("get project task by id: %w", err)
	}

	if scheduledAt.Valid {
		task.ScheduledAt = &scheduledAt.Time
	}
	if publishedAt.Valid {
		task.PublishedAt = &publishedAt.Time
	}

	task.DoDChecklist = []DoDItem{}
	if len(dodBytes) > 0 {
		_ = json.Unmarshal(dodBytes, &task.DoDChecklist)
	}
	task.Tags = []string{}
	if len(tagsBytes) > 0 {
		_ = json.Unmarshal(tagsBytes, &task.Tags)
	}

	return &task, nil
}

// ListTasksByProjectID retrieves tasks belonging to a project.
func (r *SQLProjectStudioRepository) ListTasksByProjectID(ctx context.Context, projectID string) ([]ProjectTask, error) {
	return r.ListTasks(ctx, ProjectTaskFilter{ProjectID: projectID})
}

// ListTasksByRoomID retrieves tasks linked to a specific room.
func (r *SQLProjectStudioRepository) ListTasksByRoomID(ctx context.Context, roomID string) ([]ProjectTask, error) {
	if !IsValidUUID(roomID) {
		return nil, fmt.Errorf("%w: room id %s", ErrInvalidUUID, roomID)
	}
	return r.ListTasks(ctx, ProjectTaskFilter{RoomID: &roomID})
}

// ListTasksByParentID retrieves child tasks for a parent task (fan-out hierarchy).
func (r *SQLProjectStudioRepository) ListTasksByParentID(ctx context.Context, parentTaskID string) ([]ProjectTask, error) {
	if !IsValidUUID(parentTaskID) {
		return nil, fmt.Errorf("%w: parent_task_id %s", ErrInvalidUUID, parentTaskID)
	}
	return r.ListTasks(ctx, ProjectTaskFilter{ParentTaskID: &parentTaskID})
}

// ListTasksByBatchID retrieves tasks grouped by a batch identifier.
func (r *SQLProjectStudioRepository) ListTasksByBatchID(ctx context.Context, batchID string) ([]ProjectTask, error) {
	if strings.TrimSpace(batchID) == "" {
		return nil, errors.New("batch_id cannot be empty")
	}
	return r.ListTasks(ctx, ProjectTaskFilter{BatchID: &batchID})
}

// ListTasksByTag retrieves tasks containing a specific tag in their tags array.
func (r *SQLProjectStudioRepository) ListTasksByTag(ctx context.Context, tag string) ([]ProjectTask, error) {
	if strings.TrimSpace(tag) == "" {
		return nil, errors.New("tag cannot be empty")
	}
	return r.ListTasks(ctx, ProjectTaskFilter{Tag: tag})
}

// ListTasks retrieves tasks matching the specified filter criteria.
func (r *SQLProjectStudioRepository) ListTasks(ctx context.Context, filter ProjectTaskFilter) ([]ProjectTask, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if strings.TrimSpace(filter.ProjectID) != "" {
		conditions = append(conditions, fmt.Sprintf("project_id = $%d", argIdx))
		args = append(args, filter.ProjectID)
		argIdx++
	}
	if filter.RoomID != nil {
		conditions = append(conditions, fmt.Sprintf("room_id = $%d", argIdx))
		args = append(args, *filter.RoomID)
		argIdx++
	}
	if strings.TrimSpace(filter.Status) != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, strings.ToLower(strings.TrimSpace(filter.Status)))
		argIdx++
	}
	if strings.TrimSpace(filter.Priority) != "" {
		conditions = append(conditions, fmt.Sprintf("priority = $%d", argIdx))
		args = append(args, strings.ToLower(strings.TrimSpace(filter.Priority)))
		argIdx++
	}
	if filter.ParentTaskID != nil {
		conditions = append(conditions, fmt.Sprintf("parent_task_id = $%d", argIdx))
		args = append(args, *filter.ParentTaskID)
		argIdx++
	}
	if filter.BatchID != nil {
		conditions = append(conditions, fmt.Sprintf("batch_id = $%d", argIdx))
		args = append(args, *filter.BatchID)
		argIdx++
	}
	if strings.TrimSpace(filter.Tag) != "" {
		conditions = append(conditions, fmt.Sprintf("tags @> $%d::jsonb", argIdx))
		tagJSON, _ := json.Marshal([]string{strings.TrimSpace(filter.Tag)})
		args = append(args, string(tagJSON))
		argIdx++
	}
	if filter.AssigneeID != nil {
		conditions = append(conditions, fmt.Sprintf("assignee_id = $%d", argIdx))
		args = append(args, *filter.AssigneeID)
		argIdx++
	}
	if strings.TrimSpace(filter.ExecutionStatus) != "" {
		conditions = append(conditions, fmt.Sprintf("execution_status = $%d", argIdx))
		args = append(args, strings.ToLower(strings.TrimSpace(filter.ExecutionStatus)))
		argIdx++
	}

	query := `
		SELECT id, project_id, room_id, title, description, status, priority,
		       dod_checklist, assignee_id, parent_task_id, batch_id, tags,
		       COALESCE(agent_output, ''), COALESCE(execution_status, 'idle'),
		       scheduled_at, published_at, COALESCE(publication_url, ''),
		       created_at, updated_at
		FROM project_tasks
	`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list project tasks: %w", err)
	}
	defer rows.Close()

	var list []ProjectTask
	for rows.Next() {
		var task ProjectTask
		var dodBytes, tagsBytes []byte
		var scheduledAt, publishedAt sql.NullTime

		if err := rows.Scan(
			&task.ID,
			&task.ProjectID,
			&task.RoomID,
			&task.Title,
			&task.Description,
			&task.Status,
			&task.Priority,
			&dodBytes,
			&task.AssigneeID,
			&task.ParentTaskID,
			&task.BatchID,
			&tagsBytes,
			&task.AgentOutput,
			&task.ExecutionStatus,
			&scheduledAt,
			&publishedAt,
			&task.PublicationURL,
			&task.CreatedAt,
			&task.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan project task: %w", err)
		}

		if scheduledAt.Valid {
			task.ScheduledAt = &scheduledAt.Time
		}
		if publishedAt.Valid {
			task.PublishedAt = &publishedAt.Time
		}

		task.DoDChecklist = []DoDItem{}
		if len(dodBytes) > 0 {
			_ = json.Unmarshal(dodBytes, &task.DoDChecklist)
		}
		task.Tags = []string{}
		if len(tagsBytes) > 0 {
			_ = json.Unmarshal(tagsBytes, &task.Tags)
		}

		list = append(list, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project tasks: %w", err)
	}
	return list, nil
}

// UpdateTask updates task details.
func (r *SQLProjectStudioRepository) UpdateTask(ctx context.Context, task *ProjectTask) error {
	if task == nil {
		return errors.New("project task cannot be nil")
	}
	if !IsValidUUID(task.ID) {
		return fmt.Errorf("%w: task id %s", ErrInvalidUUID, task.ID)
	}
	if strings.TrimSpace(task.Title) == "" {
		return ErrEmptyTaskTitle
	}

	normStatus, err := NormalizeTaskStatus(task.Status)
	if err != nil {
		return err
	}
	task.Status = normStatus

	normPriority, err := NormalizeTaskPriority(task.Priority)
	if err != nil {
		return err
	}
	task.Priority = normPriority

	normExecution, err := NormalizeTaskExecutionStatus(task.ExecutionStatus)
	if err != nil {
		return err
	}
	task.ExecutionStatus = normExecution

	if task.RoomID != nil && strings.TrimSpace(*task.RoomID) != "" {
		if !IsValidUUID(*task.RoomID) {
			return fmt.Errorf("%w: room id %s", ErrInvalidUUID, *task.RoomID)
		}
	} else {
		task.RoomID = nil
	}

	if task.ParentTaskID != nil && strings.TrimSpace(*task.ParentTaskID) != "" {
		if !IsValidUUID(*task.ParentTaskID) {
			return fmt.Errorf("%w: parent_task_id %s", ErrInvalidUUID, *task.ParentTaskID)
		}
	} else {
		task.ParentTaskID = nil
	}

	if task.DoDChecklist == nil {
		task.DoDChecklist = []DoDItem{}
	}
	dodBytes, err := json.Marshal(task.DoDChecklist)
	if err != nil {
		return fmt.Errorf("marshal dod_checklist: %w", err)
	}

	if task.Tags == nil {
		task.Tags = []string{}
	}
	tagsBytes, err := json.Marshal(task.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	now := time.Now().UTC()
	task.UpdatedAt = now

	query := `
		UPDATE project_tasks
		SET room_id = $2, title = $3, description = $4, status = $5, priority = $6,
		    dod_checklist = $7, assignee_id = $8, parent_task_id = $9, batch_id = $10,
		    tags = $11, agent_output = $12, execution_status = $13,
		    scheduled_at = $14, published_at = $15, publication_url = $16,
		    updated_at = $17
		WHERE id = $1
	`
	res, err := r.db.ExecContext(ctx, query,
		task.ID,
		task.RoomID,
		task.Title,
		task.Description,
		task.Status,
		task.Priority,
		dodBytes,
		task.AssigneeID,
		task.ParentTaskID,
		task.BatchID,
		tagsBytes,
		task.AgentOutput,
		task.ExecutionStatus,
		task.ScheduledAt,
		task.PublishedAt,
		task.PublicationURL,
		task.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update project task: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update project task rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrTaskNotFound
	}
	return nil
}

// UpdateTaskStatus updates just the task's status.
func (r *SQLProjectStudioRepository) UpdateTaskStatus(ctx context.Context, id string, status string) error {
	if !IsValidUUID(id) {
		return fmt.Errorf("%w: task id %s", ErrInvalidUUID, id)
	}
	normStatus, err := NormalizeTaskStatus(status)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	query := `
		UPDATE project_tasks
		SET status = $2, updated_at = $3
		WHERE id = $1
	`
	res, err := r.db.ExecContext(ctx, query, id, normStatus, now)
	if err != nil {
		return fmt.Errorf("update project task status: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update project task status rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrTaskNotFound
	}
	return nil
}

// UpdateTaskExecution updates execution status and agent output for a task.
func (r *SQLProjectStudioRepository) UpdateTaskExecution(ctx context.Context, id string, executionStatus string, agentOutput string) error {
	if !IsValidUUID(id) {
		return fmt.Errorf("%w: task id %s", ErrInvalidUUID, id)
	}
	normExecution, err := NormalizeTaskExecutionStatus(executionStatus)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	query := `
		UPDATE project_tasks
		SET execution_status = $2, agent_output = $3, updated_at = $4
		WHERE id = $1
	`
	res, err := r.db.ExecContext(ctx, query, id, normExecution, agentOutput, now)
	if err != nil {
		return fmt.Errorf("update project task execution: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update project task execution rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrTaskNotFound
	}
	return nil
}

// DeleteTask deletes a task by ID.
func (r *SQLProjectStudioRepository) DeleteTask(ctx context.Context, id string) error {
	if !IsValidUUID(id) {
		return fmt.Errorf("%w: task id %s", ErrInvalidUUID, id)
	}

	query := `DELETE FROM project_tasks WHERE id = $1`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete project task: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete project task rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrTaskNotFound
	}
	return nil
}

// MemoryProjectStudioRepository provides a thread-safe in-memory implementation of ProjectStudioRepository.
type MemoryProjectStudioRepository struct {
	mu       sync.RWMutex
	rooms    map[string]*ProjectRoom
	messages map[string]*ProjectMessage
	tasks    map[string]*ProjectTask
	requests map[string]InboxRequest
	receipts map[string]MessageReceipt
}

// NewMemoryProjectStudioRepository creates a new thread-safe in-memory ProjectStudioRepository.
func NewMemoryProjectStudioRepository() *MemoryProjectStudioRepository {
	return &MemoryProjectStudioRepository{
		rooms:    make(map[string]*ProjectRoom),
		messages: make(map[string]*ProjectMessage),
		tasks:    make(map[string]*ProjectTask),
		requests: make(map[string]InboxRequest),
	}
}

func cloneRoom(r *ProjectRoom) *ProjectRoom {
	if r == nil {
		return nil
	}
	cp := *r
	return &cp
}

func cloneMessage(m *ProjectMessage) *ProjectMessage {
	if m == nil {
		return nil
	}
	cp := *m
	if m.ReplyToID != nil {
		replyTo := *m.ReplyToID
		cp.ReplyToID = &replyTo
	}
	return &cp
}

func cloneTask(t *ProjectTask) *ProjectTask {
	if t == nil {
		return nil
	}
	cp := *t
	if t.RoomID != nil {
		roomID := *t.RoomID
		cp.RoomID = &roomID
	}
	if t.AssigneeID != nil {
		assigneeID := *t.AssigneeID
		cp.AssigneeID = &assigneeID
	}
	if t.ParentTaskID != nil {
		parentID := *t.ParentTaskID
		cp.ParentTaskID = &parentID
	}
	if t.BatchID != nil {
		batchID := *t.BatchID
		cp.BatchID = &batchID
	}
	if t.DoDChecklist != nil {
		cp.DoDChecklist = make([]DoDItem, len(t.DoDChecklist))
		copy(cp.DoDChecklist, t.DoDChecklist)
	} else {
		cp.DoDChecklist = []DoDItem{}
	}
	if t.Tags != nil {
		cp.Tags = make([]string, len(t.Tags))
		copy(cp.Tags, t.Tags)
	} else {
		cp.Tags = []string{}
	}
	if t.ScheduledAt != nil {
		st := *t.ScheduledAt
		cp.ScheduledAt = &st
	}
	if t.PublishedAt != nil {
		pt := *t.PublishedAt
		cp.PublishedAt = &pt
	}
	return &cp
}

// CreateRoom creates a new room in memory.
func (m *MemoryProjectStudioRepository) CreateRoom(ctx context.Context, room *ProjectRoom) error {
	if room == nil {
		return errors.New("project room cannot be nil")
	}
	if strings.TrimSpace(room.ProjectID) == "" {
		return ErrEmptyProjectID
	}
	if strings.TrimSpace(room.Title) == "" {
		return ErrEmptyRoomTitle
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.TrimSpace(room.ID) == "" {
		room.ID = GenerateUUID()
	} else if !IsValidUUID(room.ID) {
		return fmt.Errorf("%w: room id %s", ErrInvalidUUID, room.ID)
	}

	if _, exists := m.rooms[room.ID]; exists {
		return ErrDuplicateRoom
	}

	now := time.Now().UTC()
	if room.CreatedAt.IsZero() {
		room.CreatedAt = now
	}
	room.UpdatedAt = now

	m.rooms[room.ID] = cloneRoom(room)
	return nil
}

// GetRoomByID retrieves a room by ID from memory.
func (m *MemoryProjectStudioRepository) GetRoomByID(ctx context.Context, id string) (*ProjectRoom, error) {
	if !IsValidUUID(id) {
		return nil, fmt.Errorf("%w: room id %s", ErrInvalidUUID, id)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	room, exists := m.rooms[id]
	if !exists {
		return nil, ErrRoomNotFound
	}
	return cloneRoom(room), nil
}

// ListRoomsByProjectID lists rooms for a project from memory.
func (m *MemoryProjectStudioRepository) ListRoomsByProjectID(ctx context.Context, projectID string) ([]ProjectRoom, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, ErrEmptyProjectID
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []ProjectRoom
	for _, room := range m.rooms {
		if room.ProjectID == projectID {
			list = append(list, *cloneRoom(room))
		}
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})
	return list, nil
}

// ListAllRooms lists all rooms across all projects from memory, ordered by updated_at desc.
func (m *MemoryProjectStudioRepository) ListAllRooms(ctx context.Context) ([]ProjectRoom, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []ProjectRoom
	for _, room := range m.rooms {
		list = append(list, *cloneRoom(room))
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})
	return list, nil
}

// UpdateRoom updates a room in memory.
func (m *MemoryProjectStudioRepository) UpdateRoom(ctx context.Context, room *ProjectRoom) error {
	if room == nil {
		return errors.New("project room cannot be nil")
	}
	if !IsValidUUID(room.ID) {
		return fmt.Errorf("%w: room id %s", ErrInvalidUUID, room.ID)
	}
	if strings.TrimSpace(room.Title) == "" {
		return ErrEmptyRoomTitle
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.rooms[room.ID]
	if !exists {
		return ErrRoomNotFound
	}

	existing.Title = room.Title
	existing.Topic = room.Topic
	if strings.TrimSpace(room.ProjectID) != "" {
		existing.ProjectID = room.ProjectID
	}
	existing.UpdatedAt = time.Now().UTC()
	room.UpdatedAt = existing.UpdatedAt
	return nil
}

// DeleteRoom removes a room and cascades to messages, setting room_id to null on tasks.
func (m *MemoryProjectStudioRepository) DeleteRoom(ctx context.Context, id string) error {
	if !IsValidUUID(id) {
		return fmt.Errorf("%w: room id %s", ErrInvalidUUID, id)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rooms[id]; !exists {
		return ErrRoomNotFound
	}
	delete(m.rooms, id)

	// Cascade delete messages in this room
	for msgID, msg := range m.messages {
		if msg.RoomID == id {
			delete(m.messages, msgID)
		}
	}

	// Set room_id to nil on tasks
	for _, task := range m.tasks {
		if task.RoomID != nil && *task.RoomID == id {
			task.RoomID = nil
			task.UpdatedAt = time.Now().UTC()
		}
	}

	return nil
}

// CreateMessage inserts a message into memory.
func (m *MemoryProjectStudioRepository) CreateMessage(ctx context.Context, msg *ProjectMessage) error {
	if msg == nil {
		return errors.New("project message cannot be nil")
	}
	if !IsValidUUID(msg.RoomID) {
		return fmt.Errorf("%w: room id %s", ErrInvalidUUID, msg.RoomID)
	}
	if strings.TrimSpace(msg.Content) == "" {
		return ErrEmptyMessageContent
	}
	normAuthor, err := NormalizeAuthorType(msg.AuthorType)
	if err != nil {
		return err
	}
	msg.AuthorType = normAuthor

	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.TrimSpace(msg.ID) == "" {
		msg.ID = GenerateUUID()
	} else if !IsValidUUID(msg.ID) {
		return fmt.Errorf("%w: message id %s", ErrInvalidUUID, msg.ID)
	}

	if _, exists := m.messages[msg.ID]; exists {
		return ErrDuplicateMessage
	}

	if msg.ReplyToID != nil && strings.TrimSpace(*msg.ReplyToID) != "" {
		if !IsValidUUID(*msg.ReplyToID) {
			return fmt.Errorf("%w: reply_to_id %s", ErrInvalidUUID, *msg.ReplyToID)
		}
	} else {
		msg.ReplyToID = nil
	}

	now := time.Now().UTC()
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = now
	}
	msg.UpdatedAt = now

	m.messages[msg.ID] = cloneMessage(msg)
	return nil
}

// GetMessageByID retrieves a message by ID from memory.
func (m *MemoryProjectStudioRepository) GetMessageByID(ctx context.Context, id string) (*ProjectMessage, error) {
	if !IsValidUUID(id) {
		return nil, fmt.Errorf("%w: message id %s", ErrInvalidUUID, id)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	msg, exists := m.messages[id]
	if !exists {
		return nil, ErrMessageNotFound
	}
	return cloneMessage(msg), nil
}

// ListMessagesByRoomID lists messages for a room from memory.
func (m *MemoryProjectStudioRepository) ListMessagesByRoomID(ctx context.Context, roomID string) ([]ProjectMessage, error) {
	if !IsValidUUID(roomID) {
		return nil, fmt.Errorf("%w: room id %s", ErrInvalidUUID, roomID)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []ProjectMessage
	for _, msg := range m.messages {
		if msg.RoomID == roomID {
			list = append(list, *cloneMessage(msg))
		}
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})
	return list, nil
}

// UpdateMessage updates a message's content in memory.
func (m *MemoryProjectStudioRepository) UpdateMessage(ctx context.Context, msg *ProjectMessage) error {
	if msg == nil {
		return errors.New("project message cannot be nil")
	}
	if !IsValidUUID(msg.ID) {
		return fmt.Errorf("%w: message id %s", ErrInvalidUUID, msg.ID)
	}
	if strings.TrimSpace(msg.Content) == "" {
		return ErrEmptyMessageContent
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.messages[msg.ID]
	if !exists {
		return ErrMessageNotFound
	}

	existing.Content = msg.Content
	existing.UpdatedAt = time.Now().UTC()
	msg.UpdatedAt = existing.UpdatedAt
	return nil
}

// DeleteMessage deletes a message from memory and unlinks replies.
func (m *MemoryProjectStudioRepository) DeleteMessage(ctx context.Context, id string) error {
	if !IsValidUUID(id) {
		return fmt.Errorf("%w: message id %s", ErrInvalidUUID, id)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.messages[id]; !exists {
		return ErrMessageNotFound
	}
	delete(m.messages, id)

	// Set reply_to_id = nil for replies
	for _, msg := range m.messages {
		if msg.ReplyToID != nil && *msg.ReplyToID == id {
			msg.ReplyToID = nil
			msg.UpdatedAt = time.Now().UTC()
		}
	}
	return nil
}

// CreateTask inserts a task into memory.
func (m *MemoryProjectStudioRepository) CreateTask(ctx context.Context, task *ProjectTask) error {
	if task == nil {
		return errors.New("project task cannot be nil")
	}
	if strings.TrimSpace(task.ProjectID) == "" {
		return ErrEmptyProjectID
	}
	if strings.TrimSpace(task.Title) == "" {
		return ErrEmptyTaskTitle
	}

	normStatus, err := NormalizeTaskStatus(task.Status)
	if err != nil {
		if strings.TrimSpace(task.Status) == "" {
			normStatus = "backlog"
		} else {
			return err
		}
	}
	task.Status = normStatus

	normPriority, err := NormalizeTaskPriority(task.Priority)
	if err != nil {
		if strings.TrimSpace(task.Priority) == "" {
			normPriority = "normal"
		} else {
			return err
		}
	}
	task.Priority = normPriority

	normExecution, err := NormalizeTaskExecutionStatus(task.ExecutionStatus)
	if err != nil {
		return err
	}
	task.ExecutionStatus = normExecution

	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.TrimSpace(task.ID) == "" {
		task.ID = GenerateUUID()
	} else if !IsValidUUID(task.ID) {
		return fmt.Errorf("%w: task id %s", ErrInvalidUUID, task.ID)
	}

	if _, exists := m.tasks[task.ID]; exists {
		return ErrDuplicateTask
	}

	if task.RoomID != nil && strings.TrimSpace(*task.RoomID) != "" {
		if !IsValidUUID(*task.RoomID) {
			return fmt.Errorf("%w: room id %s", ErrInvalidUUID, *task.RoomID)
		}
	} else {
		task.RoomID = nil
	}

	if task.ParentTaskID != nil && strings.TrimSpace(*task.ParentTaskID) != "" {
		if !IsValidUUID(*task.ParentTaskID) {
			return fmt.Errorf("%w: parent_task_id %s", ErrInvalidUUID, *task.ParentTaskID)
		}
	} else {
		task.ParentTaskID = nil
	}

	now := time.Now().UTC()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now

	m.tasks[task.ID] = cloneTask(task)
	return nil
}

// GetTaskByID retrieves a task by ID from memory.
func (m *MemoryProjectStudioRepository) GetTaskByID(ctx context.Context, id string) (*ProjectTask, error) {
	if !IsValidUUID(id) {
		return nil, fmt.Errorf("%w: task id %s", ErrInvalidUUID, id)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, ErrTaskNotFound
	}
	return cloneTask(task), nil
}

// ListTasksByProjectID lists tasks for a project from memory.
func (m *MemoryProjectStudioRepository) ListTasksByProjectID(ctx context.Context, projectID string) ([]ProjectTask, error) {
	return m.ListTasks(ctx, ProjectTaskFilter{ProjectID: projectID})
}

// ListTasksByRoomID lists tasks for a room from memory.
func (m *MemoryProjectStudioRepository) ListTasksByRoomID(ctx context.Context, roomID string) ([]ProjectTask, error) {
	if !IsValidUUID(roomID) {
		return nil, fmt.Errorf("%w: room id %s", ErrInvalidUUID, roomID)
	}
	return m.ListTasks(ctx, ProjectTaskFilter{RoomID: &roomID})
}

// ListTasksByParentID lists tasks for a parent task (fan-out hierarchy) from memory.
func (m *MemoryProjectStudioRepository) ListTasksByParentID(ctx context.Context, parentTaskID string) ([]ProjectTask, error) {
	if !IsValidUUID(parentTaskID) {
		return nil, fmt.Errorf("%w: parent_task_id %s", ErrInvalidUUID, parentTaskID)
	}
	return m.ListTasks(ctx, ProjectTaskFilter{ParentTaskID: &parentTaskID})
}

// ListTasksByBatchID lists tasks for a batch from memory.
func (m *MemoryProjectStudioRepository) ListTasksByBatchID(ctx context.Context, batchID string) ([]ProjectTask, error) {
	if strings.TrimSpace(batchID) == "" {
		return nil, errors.New("batch_id cannot be empty")
	}
	return m.ListTasks(ctx, ProjectTaskFilter{BatchID: &batchID})
}

// ListTasksByTag lists tasks containing a specific tag from memory.
func (m *MemoryProjectStudioRepository) ListTasksByTag(ctx context.Context, tag string) ([]ProjectTask, error) {
	if strings.TrimSpace(tag) == "" {
		return nil, errors.New("tag cannot be empty")
	}
	return m.ListTasks(ctx, ProjectTaskFilter{Tag: tag})
}

// ListTasks lists tasks based on filters from memory.
func (m *MemoryProjectStudioRepository) ListTasks(ctx context.Context, filter ProjectTaskFilter) ([]ProjectTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []ProjectTask
	for _, task := range m.tasks {
		if strings.TrimSpace(filter.ProjectID) != "" && task.ProjectID != filter.ProjectID {
			continue
		}
		if filter.RoomID != nil {
			if task.RoomID == nil || *task.RoomID != *filter.RoomID {
				continue
			}
		}
		if strings.TrimSpace(filter.Status) != "" && strings.ToLower(task.Status) != strings.ToLower(strings.TrimSpace(filter.Status)) {
			continue
		}
		if strings.TrimSpace(filter.Priority) != "" && strings.ToLower(task.Priority) != strings.ToLower(strings.TrimSpace(filter.Priority)) {
			continue
		}
		if filter.ParentTaskID != nil {
			if task.ParentTaskID == nil || *task.ParentTaskID != *filter.ParentTaskID {
				continue
			}
		}
		if filter.BatchID != nil {
			if task.BatchID == nil || *task.BatchID != *filter.BatchID {
				continue
			}
		}
		if strings.TrimSpace(filter.Tag) != "" {
			found := false
			targetTag := strings.ToLower(strings.TrimSpace(filter.Tag))
			for _, t := range task.Tags {
				if strings.ToLower(strings.TrimSpace(t)) == targetTag {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if filter.AssigneeID != nil {
			if task.AssigneeID == nil || *task.AssigneeID != *filter.AssigneeID {
				continue
			}
		}
		if strings.TrimSpace(filter.ExecutionStatus) != "" && !strings.EqualFold(task.ExecutionStatus, strings.TrimSpace(filter.ExecutionStatus)) {
			continue
		}

		list = append(list, *cloneTask(task))
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})
	return list, nil
}

// UpdateTask updates a task in memory.
func (m *MemoryProjectStudioRepository) UpdateTask(ctx context.Context, task *ProjectTask) error {
	if task == nil {
		return errors.New("project task cannot be nil")
	}
	if !IsValidUUID(task.ID) {
		return fmt.Errorf("%w: task id %s", ErrInvalidUUID, task.ID)
	}
	if strings.TrimSpace(task.Title) == "" {
		return ErrEmptyTaskTitle
	}

	normStatus, err := NormalizeTaskStatus(task.Status)
	if err != nil {
		return err
	}
	task.Status = normStatus

	normPriority, err := NormalizeTaskPriority(task.Priority)
	if err != nil {
		return err
	}
	task.Priority = normPriority

	normExecution, err := NormalizeTaskExecutionStatus(task.ExecutionStatus)
	if err != nil {
		return err
	}
	task.ExecutionStatus = normExecution

	if task.RoomID != nil && strings.TrimSpace(*task.RoomID) != "" {
		if !IsValidUUID(*task.RoomID) {
			return fmt.Errorf("%w: room id %s", ErrInvalidUUID, *task.RoomID)
		}
	} else {
		task.RoomID = nil
	}

	if task.ParentTaskID != nil && strings.TrimSpace(*task.ParentTaskID) != "" {
		if !IsValidUUID(*task.ParentTaskID) {
			return fmt.Errorf("%w: parent_task_id %s", ErrInvalidUUID, *task.ParentTaskID)
		}
	} else {
		task.ParentTaskID = nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.tasks[task.ID]
	if !exists {
		return ErrTaskNotFound
	}

	existing.RoomID = task.RoomID
	existing.Title = task.Title
	existing.Description = task.Description
	existing.Status = task.Status
	existing.Priority = task.Priority
	existing.AssigneeID = task.AssigneeID
	existing.ParentTaskID = task.ParentTaskID
	existing.BatchID = task.BatchID
	existing.AgentOutput = task.AgentOutput
	existing.ExecutionStatus = task.ExecutionStatus
	existing.PublicationURL = task.PublicationURL
	if task.ScheduledAt != nil {
		st := *task.ScheduledAt
		existing.ScheduledAt = &st
	} else {
		existing.ScheduledAt = nil
	}
	if task.PublishedAt != nil {
		pt := *task.PublishedAt
		existing.PublishedAt = &pt
	} else {
		existing.PublishedAt = nil
	}

	if task.DoDChecklist != nil {
		existing.DoDChecklist = make([]DoDItem, len(task.DoDChecklist))
		copy(existing.DoDChecklist, task.DoDChecklist)
	} else {
		existing.DoDChecklist = []DoDItem{}
	}

	if task.Tags != nil {
		existing.Tags = make([]string, len(task.Tags))
		copy(existing.Tags, task.Tags)
	} else {
		existing.Tags = []string{}
	}

	existing.UpdatedAt = time.Now().UTC()
	task.UpdatedAt = existing.UpdatedAt
	return nil
}

// UpdateTaskStatus updates a task status in memory.
func (m *MemoryProjectStudioRepository) UpdateTaskStatus(ctx context.Context, id string, status string) error {
	if !IsValidUUID(id) {
		return fmt.Errorf("%w: task id %s", ErrInvalidUUID, id)
	}
	normStatus, err := NormalizeTaskStatus(status)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.tasks[id]
	if !exists {
		return ErrTaskNotFound
	}

	existing.Status = normStatus
	existing.UpdatedAt = time.Now().UTC()
	return nil
}

// UpdateTaskExecution updates execution status and agent output for a task in memory.
func (m *MemoryProjectStudioRepository) UpdateTaskExecution(ctx context.Context, id string, executionStatus string, agentOutput string) error {
	if !IsValidUUID(id) {
		return fmt.Errorf("%w: task id %s", ErrInvalidUUID, id)
	}
	normExecution, err := NormalizeTaskExecutionStatus(executionStatus)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.tasks[id]
	if !exists {
		return ErrTaskNotFound
	}

	existing.ExecutionStatus = normExecution
	existing.AgentOutput = agentOutput
	existing.UpdatedAt = time.Now().UTC()
	return nil
}

// DeleteTask removes a task from memory and sets parent_task_id to null for child tasks.
func (m *MemoryProjectStudioRepository) DeleteTask(ctx context.Context, id string) error {
	if !IsValidUUID(id) {
		return fmt.Errorf("%w: task id %s", ErrInvalidUUID, id)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[id]; !exists {
		return ErrTaskNotFound
	}
	delete(m.tasks, id)

	// Set parent_task_id = nil for children
	for _, child := range m.tasks {
		if child.ParentTaskID != nil && *child.ParentTaskID == id {
			child.ParentTaskID = nil
			child.UpdatedAt = time.Now().UTC()
		}
	}
	return nil
}

// UpdateRoomDisplayTitle changes only a display label, with a compare-and-swap
// guard. It never changes message activity time, topic binding or project.
func (r *SQLProjectStudioRepository) UpdateRoomDisplayTitle(ctx context.Context, id, topic, previous, title string) (bool, error) {
	if !IsValidUUID(id) || strings.TrimSpace(title) == "" {
		return false, ErrEmptyRoomTitle
	}
	result, err := r.db.ExecContext(ctx, "UPDATE project_rooms SET title=$4 WHERE id=$1 AND topic=$2 AND title=$3", id, topic, previous, title)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func (m *MemoryProjectStudioRepository) UpdateRoomDisplayTitle(ctx context.Context, id, topic, previous, title string) (bool, error) {
	if !IsValidUUID(id) || strings.TrimSpace(title) == "" {
		return false, ErrEmptyRoomTitle
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	room := m.rooms[id]
	if room == nil || room.Topic != topic || room.Title != previous {
		return false, nil
	}
	room.Title = title
	return true, nil
}
