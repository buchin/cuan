package db

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestUUIDGenerationAndValidation(t *testing.T) {
	for i := 0; i < 20; i++ {
		u := GenerateUUID()
		if !IsValidUUID(u) {
			t.Fatalf("generated invalid UUID: %s", u)
		}
	}

	invalidUUIDs := []string{
		"",
		"not-a-uuid",
		"12345678-1234-1234-1234",
		"g0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", // non-hex character 'g'
		"12345678-1234-1234-1234-1234567890ab-extra",
	}

	for _, invalid := range invalidUUIDs {
		if IsValidUUID(invalid) {
			t.Errorf("expected %q to be invalid UUID, but passed", invalid)
		}
	}
}

func TestNormalization(t *testing.T) {
	// Author type
	validAuthors := []string{"USER", "agent", " System "}
	for _, a := range validAuthors {
		norm, err := NormalizeAuthorType(a)
		if err != nil {
			t.Errorf("unexpected error for author %s: %v", a, err)
		}
		if norm != "user" && norm != "agent" && norm != "system" {
			t.Errorf("unexpected normalized author: %s", norm)
		}
	}

	if _, err := NormalizeAuthorType("bot"); err != ErrInvalidAuthorType {
		t.Errorf("expected ErrInvalidAuthorType, got %v", err)
	}

	// Task status
	validStatuses := []string{"BACKLOG", "Doing", " review ", "DONE"}
	for _, s := range validStatuses {
		norm, err := NormalizeTaskStatus(s)
		if err != nil {
			t.Errorf("unexpected error for status %s: %v", s, err)
		}
		if norm != "backlog" && norm != "doing" && norm != "review" && norm != "done" {
			t.Errorf("unexpected normalized status: %s", norm)
		}
	}

	if _, err := NormalizeTaskStatus("completed"); err != ErrInvalidTaskStatus {
		t.Errorf("expected ErrInvalidTaskStatus, got %v", err)
	}

	// Task priority
	validPriorities := []string{"LOW", "Normal", " high ", "URGENT"}
	for _, p := range validPriorities {
		norm, err := NormalizeTaskPriority(p)
		if err != nil {
			t.Errorf("unexpected error for priority %s: %v", p, err)
		}
		if norm != "low" && norm != "normal" && norm != "high" && norm != "urgent" {
			t.Errorf("unexpected normalized priority: %s", norm)
		}
	}

	if _, err := NormalizeTaskPriority("critical"); err != ErrInvalidTaskPriority {
		t.Errorf("expected ErrInvalidTaskPriority, got %v", err)
	}
}

func TestDTOConversion(t *testing.T) {
	now := time.Now().UTC()
	room := &ProjectRoom{
		ID:        GenerateUUID(),
		ProjectID: "proj_cuan",
		Title:     "Sprint 7 Discussion",
		Topic:     "Project Studio Architecture",
		CreatedAt: now,
		UpdatedAt: now,
	}
	roomDTO := room.ToDTO()
	if roomDTO.ID != room.ID || roomDTO.ProjectID != room.ProjectID || roomDTO.Title != room.Title {
		t.Errorf("room DTO mismatch")
	}

	msg := &ProjectMessage{
		ID:         GenerateUUID(),
		RoomID:     room.ID,
		AuthorType: "user",
		AuthorID:   "buchin",
		AuthorName: "Buchin",
		Content:    "Let's build Project Studio!",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	msgDTO := msg.ToDTO()
	if msgDTO.ID != msg.ID || msgDTO.Content != msg.Content || msgDTO.AuthorType != "user" {
		t.Errorf("message DTO mismatch")
	}

	parentID := GenerateUUID()
	batchID := "batch-s7-01"
	assigneeID := "dev_01"
	schedTime := now.Add(24 * time.Hour)
	pubTime := now.Add(25 * time.Hour)
	pubURL := "https://myblog.com/2026/10/modular-os"
	task := &ProjectTask{
		ID:             GenerateUUID(),
		ProjectID:      "proj_cuan",
		RoomID:         &room.ID,
		Title:          "Implement DB Models",
		Description:    "Database models for Studio",
		Status:         "doing",
		Priority:       "high",
		DoDChecklist:   []DoDItem{{ID: "1", Text: "Migrations up & down", Completed: true}},
		AssigneeID:     &assigneeID,
		ParentTaskID:   &parentID,
		BatchID:        &batchID,
		Tags:           []string{"database", "postgresql", "sprint7"},
		ScheduledAt:    &schedTime,
		PublishedAt:    &pubTime,
		PublicationURL: pubURL,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	taskDTO := task.ToDTO()
	if taskDTO.ID != task.ID || taskDTO.Title != task.Title || len(taskDTO.Tags) != 3 || len(taskDTO.DoDChecklist) != 1 {
		t.Errorf("task DTO mismatch")
	}
	if taskDTO.PublicationURL != pubURL {
		t.Errorf("expected publication_url %s, got %s", pubURL, taskDTO.PublicationURL)
	}
	if taskDTO.ScheduledAt == nil || *taskDTO.ScheduledAt != schedTime {
		t.Errorf("scheduled_at mismatch in DTO")
	}
	if taskDTO.PublishedAt == nil || *taskDTO.PublishedAt != pubTime {
		t.Errorf("published_at mismatch in DTO")
	}

	// Verify deep copy isolation
	taskDTO.Tags[0] = "mutated"
	if task.Tags[0] == "mutated" {
		t.Errorf("task DTO tags should be a deep copy")
	}
	taskDTO.DoDChecklist[0].Completed = false
	if !task.DoDChecklist[0].Completed {
		t.Errorf("task DTO checklist should be a deep copy")
	}
}

func TestMemoryProjectStudioRepository_Rooms(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryProjectStudioRepository()

	// 1. Validation errors
	if err := repo.CreateRoom(ctx, nil); err == nil {
		t.Errorf("expected error creating nil room")
	}
	if err := repo.CreateRoom(ctx, &ProjectRoom{Title: "Room 1"}); err != ErrEmptyProjectID {
		t.Errorf("expected ErrEmptyProjectID, got %v", err)
	}
	if err := repo.CreateRoom(ctx, &ProjectRoom{ProjectID: "proj_1"}); err != ErrEmptyRoomTitle {
		t.Errorf("expected ErrEmptyRoomTitle, got %v", err)
	}
	if err := repo.CreateRoom(ctx, &ProjectRoom{ProjectID: "proj_1", Title: "Room 1", ID: "invalid-uuid"}); err == nil {
		t.Errorf("expected invalid UUID error")
	}

	// 2. Successful creation
	room1 := &ProjectRoom{
		ProjectID: "proj_alpha",
		Title:     "General Discussion",
		Topic:     "Brainstorming sprint scope",
	}
	if err := repo.CreateRoom(ctx, room1); err != nil {
		t.Fatalf("failed to create room1: %v", err)
	}
	if !IsValidUUID(room1.ID) {
		t.Fatalf("expected auto-generated UUID, got %s", room1.ID)
	}

	// Duplicate ID error
	if err := repo.CreateRoom(ctx, &ProjectRoom{ID: room1.ID, ProjectID: "proj_alpha", Title: "Dup"}); err != ErrDuplicateRoom {
		t.Errorf("expected ErrDuplicateRoom, got %v", err)
	}

	// 3. Get room by ID
	fetched, err := repo.GetRoomByID(ctx, room1.ID)
	if err != nil {
		t.Fatalf("failed to get room by id: %v", err)
	}
	if fetched.Title != room1.Title || fetched.Topic != room1.Topic {
		t.Errorf("fetched room data mismatch: %+v vs %+v", fetched, room1)
	}

	// Non-existent room
	if _, err := repo.GetRoomByID(ctx, GenerateUUID()); err != ErrRoomNotFound {
		t.Errorf("expected ErrRoomNotFound, got %v", err)
	}

	// 4. List rooms by project ID
	room2 := &ProjectRoom{
		ProjectID: "proj_alpha",
		Title:     "Architecture Room",
		Topic:     "Backend API design",
	}
	if err := repo.CreateRoom(ctx, room2); err != nil {
		t.Fatalf("failed to create room2: %v", err)
	}

	roomOther := &ProjectRoom{
		ProjectID: "proj_beta",
		Title:     "Beta Room",
		Topic:     "Unrelated",
	}
	if err := repo.CreateRoom(ctx, roomOther); err != nil {
		t.Fatalf("failed to create roomOther: %v", err)
	}

	roomsAlpha, err := repo.ListRoomsByProjectID(ctx, "proj_alpha")
	if err != nil {
		t.Fatalf("failed to list rooms for proj_alpha: %v", err)
	}
	if len(roomsAlpha) != 2 {
		t.Fatalf("expected 2 rooms for proj_alpha, got %d", len(roomsAlpha))
	}
	if roomsAlpha[0].Title != "General Discussion" || roomsAlpha[1].Title != "Architecture Room" {
		t.Errorf("unexpected rooms ordering or content: %+v", roomsAlpha)
	}

	// 5. Update room
	fetched.Title = "Renamed Discussion"
	fetched.Topic = "Updated topic"
	if err := repo.UpdateRoom(ctx, fetched); err != nil {
		t.Fatalf("failed to update room: %v", err)
	}

	refetched, err := repo.GetRoomByID(ctx, room1.ID)
	if err != nil {
		t.Fatalf("failed to refetch room: %v", err)
	}
	if refetched.Title != "Renamed Discussion" || refetched.Topic != "Updated topic" {
		t.Errorf("updated room mismatch: %+v", refetched)
	}

	// 6. Delete room
	if err := repo.DeleteRoom(ctx, room1.ID); err != nil {
		t.Fatalf("failed to delete room: %v", err)
	}
	if _, err := repo.GetRoomByID(ctx, room1.ID); err != ErrRoomNotFound {
		t.Errorf("expected ErrRoomNotFound after deletion, got %v", err)
	}
	if err := repo.DeleteRoom(ctx, room1.ID); err != ErrRoomNotFound {
		t.Errorf("expected ErrRoomNotFound when deleting non-existent, got %v", err)
	}
}

func TestMemoryProjectStudioRepository_Messages(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryProjectStudioRepository()

	roomID := GenerateUUID()
	_ = repo.CreateRoom(ctx, &ProjectRoom{ID: roomID, ProjectID: "proj_1", Title: "Chat Room"})

	// 1. Validation errors
	if err := repo.CreateMessage(ctx, nil); err == nil {
		t.Errorf("expected error on nil message")
	}
	if err := repo.CreateMessage(ctx, &ProjectMessage{RoomID: "invalid-uuid", Content: "Hello"}); err == nil {
		t.Errorf("expected invalid UUID error on room ID")
	}
	if err := repo.CreateMessage(ctx, &ProjectMessage{RoomID: roomID, Content: ""}); err != ErrEmptyMessageContent {
		t.Errorf("expected ErrEmptyMessageContent, got %v", err)
	}
	if err := repo.CreateMessage(ctx, &ProjectMessage{RoomID: roomID, Content: "Hi", AuthorType: "robot"}); err != ErrInvalidAuthorType {
		t.Errorf("expected ErrInvalidAuthorType, got %v", err)
	}

	// 2. Successful creation
	msg1 := &ProjectMessage{
		RoomID:     roomID,
		AuthorType: "user",
		AuthorID:   "buchin",
		AuthorName: "Buchin",
		Content:    "Hello agent, please review the sprint plan.",
	}
	if err := repo.CreateMessage(ctx, msg1); err != nil {
		t.Fatalf("failed to create msg1: %v", err)
	}
	if !IsValidUUID(msg1.ID) {
		t.Fatalf("expected auto-generated UUID for msg1, got %s", msg1.ID)
	}

	// 3. Reply message
	msgReply := &ProjectMessage{
		RoomID:     roomID,
		AuthorType: "agent",
		AuthorID:   "sm",
		AuthorName: "Scrum Master",
		Content:    "Acknowledged, Sprint 7 scope is ready.",
		ReplyToID:  &msg1.ID,
	}
	if err := repo.CreateMessage(ctx, msgReply); err != nil {
		t.Fatalf("failed to create reply message: %v", err)
	}

	// 4. Get message by ID
	fetched, err := repo.GetMessageByID(ctx, msgReply.ID)
	if err != nil {
		t.Fatalf("failed to get message by id: %v", err)
	}
	if fetched.ReplyToID == nil || *fetched.ReplyToID != msg1.ID {
		t.Errorf("expected ReplyToID %s, got %+v", msg1.ID, fetched.ReplyToID)
	}

	// 5. List messages by room ID
	list, err := repo.ListMessagesByRoomID(ctx, roomID)
	if err != nil {
		t.Fatalf("failed to list messages: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(list))
	}
	if list[0].Content != msg1.Content || list[1].Content != msgReply.Content {
		t.Errorf("unexpected message list order/content: %+v", list)
	}

	// 6. Update message
	fetched.Content = "Updated reply content"
	if err := repo.UpdateMessage(ctx, fetched); err != nil {
		t.Fatalf("failed to update message: %v", err)
	}
	refetched, _ := repo.GetMessageByID(ctx, msgReply.ID)
	if refetched.Content != "Updated reply content" {
		t.Errorf("expected updated message content, got %s", refetched.Content)
	}

	// 7. Delete message with reply decoupling
	if err := repo.DeleteMessage(ctx, msg1.ID); err != nil {
		t.Fatalf("failed to delete msg1: %v", err)
	}
	replyAfterParentDelete, err := repo.GetMessageByID(ctx, msgReply.ID)
	if err != nil {
		t.Fatalf("failed to get reply after parent delete: %v", err)
	}
	if replyAfterParentDelete.ReplyToID != nil {
		t.Errorf("expected reply_to_id to become nil after deleting parent message, got %v", replyAfterParentDelete.ReplyToID)
	}
}

func TestMemoryProjectStudioRepository_Tasks(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryProjectStudioRepository()

	projectID := "proj_cuan"
	roomID := GenerateUUID()
	_ = repo.CreateRoom(ctx, &ProjectRoom{ID: roomID, ProjectID: projectID, Title: "Studio Backlog"})

	// 1. Validation errors
	if err := repo.CreateTask(ctx, nil); err == nil {
		t.Errorf("expected error on nil task")
	}
	if err := repo.CreateTask(ctx, &ProjectTask{Title: "Task 1"}); err != ErrEmptyProjectID {
		t.Errorf("expected ErrEmptyProjectID, got %v", err)
	}
	if err := repo.CreateTask(ctx, &ProjectTask{ProjectID: projectID}); err != ErrEmptyTaskTitle {
		t.Errorf("expected ErrEmptyTaskTitle, got %v", err)
	}
	if err := repo.CreateTask(ctx, &ProjectTask{ProjectID: projectID, Title: "T", Status: "invalid"}); err != ErrInvalidTaskStatus {
		t.Errorf("expected ErrInvalidTaskStatus, got %v", err)
	}
	if err := repo.CreateTask(ctx, &ProjectTask{ProjectID: projectID, Title: "T", Priority: "invalid"}); err != ErrInvalidTaskPriority {
		t.Errorf("expected ErrInvalidTaskPriority, got %v", err)
	}

	// 2. Parent task creation (Root task in fan-out hierarchy)
	batchID := "batch_s7_studio_core"
	parentTask := &ProjectTask{
		ProjectID:   projectID,
		RoomID:      &roomID,
		Title:       "Epic: Project Studio Engine",
		Description: "Parent epic for fan-out tasks",
		Status:      "doing",
		Priority:    "urgent",
		BatchID:     &batchID,
		Tags:        []string{"epic", "studio", "sprint7"},
		DoDChecklist: []DoDItem{
			{ID: "d1", Text: "Data layer done", Completed: true},
			{ID: "d2", Text: "API routes done", Completed: false},
		},
	}
	if err := repo.CreateTask(ctx, parentTask); err != nil {
		t.Fatalf("failed to create parent task: %v", err)
	}
	if !IsValidUUID(parentTask.ID) {
		t.Fatalf("expected auto-generated UUID, got %s", parentTask.ID)
	}

	// 3. Child tasks (Fan-out subtasks)
	subtask1 := &ProjectTask{
		ProjectID:    projectID,
		RoomID:       &roomID,
		ParentTaskID: &parentTask.ID,
		BatchID:      &batchID,
		Title:        "S07.1: DB Migration & Models",
		Description:  "PostgreSQL 000012 schema and repository layer",
		Status:       "done",
		Priority:     "high",
		Tags:         []string{"database", "postgresql", "sprint7", "wordpress"},
	}
	if err := repo.CreateTask(ctx, subtask1); err != nil {
		t.Fatalf("failed to create subtask1: %v", err)
	}

	subtask2 := &ProjectTask{
		ProjectID:    projectID,
		RoomID:       &roomID,
		ParentTaskID: &parentTask.ID,
		BatchID:      &batchID,
		Title:        "S07.2: API Endpoints & Handlers",
		Description:  "REST API for discussions & tasks",
		Status:       "backlog",
		Priority:     "normal",
		Tags:         []string{"api", "backend", "sprint7", "adsense"},
	}
	if err := repo.CreateTask(ctx, subtask2); err != nil {
		t.Fatalf("failed to create subtask2: %v", err)
	}

	unrelatedTask := &ProjectTask{
		ProjectID:   "proj_other",
		Title:       "Unrelated Blog Post",
		Description: "Marketing copy",
		Status:      "backlog",
		Priority:    "low",
		Tags:        []string{"marketing", "wordpress"},
	}
	if err := repo.CreateTask(ctx, unrelatedTask); err != nil {
		t.Fatalf("failed to create unrelated task: %v", err)
	}

	// 4. Test ListTasksByParentID (Fan-out hierarchy verification)
	children, err := repo.ListTasksByParentID(ctx, parentTask.ID)
	if err != nil {
		t.Fatalf("failed to list tasks by parent ID: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("expected 2 child tasks under parent, got %d", len(children))
	}
	for _, child := range children {
		if child.ParentTaskID == nil || *child.ParentTaskID != parentTask.ID {
			t.Errorf("expected child parent_task_id %s, got %+v", parentTask.ID, child.ParentTaskID)
		}
	}

	// 5. Test ListTasksByBatchID (Multi-project batch grouping)
	batchTasks, err := repo.ListTasksByBatchID(ctx, batchID)
	if err != nil {
		t.Fatalf("failed to list tasks by batch ID: %v", err)
	}
	if len(batchTasks) != 3 { // parent + 2 subtasks
		t.Fatalf("expected 3 tasks in batch %s, got %d", batchID, len(batchTasks))
	}

	// 6. Test ListTasksByTag (Tag selector verification: ["wordpress", "adsense"])
	wordpressTasks, err := repo.ListTasksByTag(ctx, "wordpress")
	if err != nil {
		t.Fatalf("failed to list tasks by tag 'wordpress': %v", err)
	}
	if len(wordpressTasks) != 2 { // subtask1 and unrelatedTask
		t.Fatalf("expected 2 tasks tagged 'wordpress', got %d", len(wordpressTasks))
	}

	adsenseTasks, err := repo.ListTasksByTag(ctx, "adsense")
	if err != nil {
		t.Fatalf("failed to list tasks by tag 'adsense': %v", err)
	}
	if len(adsenseTasks) != 1 || adsenseTasks[0].ID != subtask2.ID {
		t.Fatalf("expected subtask2 tagged 'adsense', got %+v", adsenseTasks)
	}

	// 7. Test Combined Filtering
	filtered, err := repo.ListTasks(ctx, ProjectTaskFilter{
		ProjectID: projectID,
		Status:    "done",
	})
	if err != nil {
		t.Fatalf("failed to filter tasks: %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != subtask1.ID {
		t.Fatalf("expected exactly 1 done task in proj_cuan, got %d", len(filtered))
	}

	// 8. Update Task & Status
	if err := repo.UpdateTaskStatus(ctx, subtask2.ID, "doing"); err != nil {
		t.Fatalf("failed to update task status: %v", err)
	}
	refetchedSubtask2, err := repo.GetTaskByID(ctx, subtask2.ID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}
	if refetchedSubtask2.Status != "doing" {
		t.Errorf("expected status 'doing', got %s", refetchedSubtask2.Status)
	}

	// 9. Cascade on Room Deletion: Tasks in room should have RoomID set to nil
	if err := repo.DeleteRoom(ctx, roomID); err != nil {
		t.Fatalf("failed to delete room: %v", err)
	}
	refetchedParent, err := repo.GetTaskByID(ctx, parentTask.ID)
	if err != nil {
		t.Fatalf("failed to refetch parent task: %v", err)
	}
	if refetchedParent.RoomID != nil {
		t.Errorf("expected task room_id to become nil after deleting room, got %v", refetchedParent.RoomID)
	}

	// 10. Delete parent task: Child tasks should have parent_task_id set to nil
	if err := repo.DeleteTask(ctx, parentTask.ID); err != nil {
		t.Fatalf("failed to delete parent task: %v", err)
	}
	refetchedChild, err := repo.GetTaskByID(ctx, subtask1.ID)
	if err != nil {
		t.Fatalf("failed to refetch child task: %v", err)
	}
	if refetchedChild.ParentTaskID != nil {
		t.Errorf("expected child parent_task_id to become nil after parent deletion, got %v", refetchedChild.ParentTaskID)
	}

	// 11. In-task Agent execution status & agent output
	if refetchedChild.ExecutionStatus != "idle" {
		t.Errorf("expected default execution_status 'idle', got %s", refetchedChild.ExecutionStatus)
	}
	if err := repo.UpdateTaskExecution(ctx, subtask1.ID, "completed", "# Article Outline\n1. Intro\n2. Body\n3. Conclusion"); err != nil {
		t.Fatalf("failed to update task execution: %v", err)
	}
	executedChild, err := repo.GetTaskByID(ctx, subtask1.ID)
	if err != nil {
		t.Fatalf("failed to refetch task after execution update: %v", err)
	}
	if executedChild.ExecutionStatus != "completed" {
		t.Errorf("expected execution_status 'completed', got %s", executedChild.ExecutionStatus)
	}
	if !strings.Contains(executedChild.AgentOutput, "# Article Outline") {
		t.Errorf("expected agent output to contain markdown, got %s", executedChild.AgentOutput)
	}

	// Filter by ExecutionStatus
	execTasks, err := repo.ListTasks(ctx, ProjectTaskFilter{
		ProjectID:       projectID,
		ExecutionStatus: "completed",
	})
	if err != nil {
		t.Fatalf("failed to list tasks by execution status: %v", err)
	}
	if len(execTasks) != 1 || execTasks[0].ID != subtask1.ID {
		t.Errorf("expected 1 task with execution_status 'completed', got %d", len(execTasks))
	}

	// 12. Modular publishing fields: scheduled_at, published_at, publication_url
	schedAt := time.Now().UTC().Add(48 * time.Hour)
	pubAt := time.Now().UTC().Add(49 * time.Hour)
	pubURL := "https://mycreatorblog.com/posts/seo-guide"
	executedChild.ScheduledAt = &schedAt
	executedChild.PublishedAt = &pubAt
	executedChild.PublicationURL = pubURL
	if err := repo.UpdateTask(ctx, executedChild); err != nil {
		t.Fatalf("failed to update task with publication fields: %v", err)
	}

	pubTask, err := repo.GetTaskByID(ctx, executedChild.ID)
	if err != nil {
		t.Fatalf("failed to fetch updated publication task: %v", err)
	}
	if pubTask.PublicationURL != pubURL {
		t.Errorf("expected publication_url %s, got %s", pubURL, pubTask.PublicationURL)
	}
	if pubTask.ScheduledAt == nil || *pubTask.ScheduledAt != schedAt {
		t.Errorf("scheduled_at mismatch")
	}
	if pubTask.PublishedAt == nil || *pubTask.PublishedAt != pubAt {
		t.Errorf("published_at mismatch")
	}
}

func TestMemoryProjectStudioRepository_ConcurrencyThreadSafety(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryProjectStudioRepository()

	projectID := "proj_concurrency"
	roomID := GenerateUUID()
	_ = repo.CreateRoom(ctx, &ProjectRoom{ID: roomID, ProjectID: projectID, Title: "Concurrent Room"})

	var wg sync.WaitGroup
	workers := 15
	iterations := 40

	// Concurrent Message Creators
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = repo.CreateMessage(ctx, &ProjectMessage{
					RoomID:     roomID,
					AuthorType: "user",
					AuthorID:   fmt.Sprintf("user_%d", workerID),
					AuthorName: fmt.Sprintf("User %d", workerID),
					Content:    fmt.Sprintf("Concurrent message %d from worker %d", i, workerID),
				})
			}
		}(w)
	}

	// Concurrent Task Creators
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				batch := fmt.Sprintf("batch_%d", workerID%3)
				_ = repo.CreateTask(ctx, &ProjectTask{
					ProjectID:   projectID,
					RoomID:      &roomID,
					Title:       fmt.Sprintf("Concurrent Task %d from worker %d", i, workerID),
					Description: "Stress testing thread safety",
					Status:      "backlog",
					Priority:    "normal",
					BatchID:     &batch,
					Tags:        []string{"concurrent", "test", fmt.Sprintf("tag_%d", i%4)},
				})
			}
		}(w)
	}

	// Concurrent Readers
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_, _ = repo.ListMessagesByRoomID(ctx, roomID)
				_, _ = repo.ListTasksByProjectID(ctx, projectID)
				_, _ = repo.ListTasksByTag(ctx, "concurrent")
				_, _ = repo.ListTasksByBatchID(ctx, "batch_0")
			}
		}(w)
	}

	wg.Wait()

	messages, err := repo.ListMessagesByRoomID(ctx, roomID)
	if err != nil {
		t.Fatalf("failed to list messages after concurrency test: %v", err)
	}
	if len(messages) != workers*iterations {
		t.Errorf("expected %d messages, got %d", workers*iterations, len(messages))
	}

	tasks, err := repo.ListTasksByProjectID(ctx, projectID)
	if err != nil {
		t.Fatalf("failed to list tasks after concurrency test: %v", err)
	}
	if len(tasks) != workers*iterations {
		t.Errorf("expected %d tasks, got %d", workers*iterations, len(tasks))
	}
}

func TestRepositories_Aggregation(t *testing.T) {
	repos := NewMemoryRepositories()
	if repos.Servers == nil {
		t.Errorf("expected Servers repository to be non-nil")
	}
	if repos.Domains == nil {
		t.Errorf("expected Domains repository to be non-nil")
	}
	if repos.Observability == nil {
		t.Errorf("expected Observability repository to be non-nil")
	}
	if repos.ProjectStudio == nil {
		t.Errorf("expected ProjectStudio repository to be non-nil")
	}
	if repos.BatchCampaigns == nil {
		t.Errorf("expected BatchCampaigns repository to be non-nil")
	}
	if repos.Agents == nil {
		t.Errorf("expected Agents repository to be non-nil")
	}
	if repos.Projects == nil {
		t.Errorf("expected Projects repository to be non-nil")
	}
	if repos.ProjectIntegrations == nil {
		t.Errorf("expected ProjectIntegrations repository to be non-nil")
	}
}
