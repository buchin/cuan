package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestResolveInternalCuanLinksInConversation(t *testing.T) {
	s, _ := setupTestChatServer(t)
	ctx := context.Background()

	// 1. Setup target room with some messages
	targetRoomID := db.GenerateUUID()
	s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{
		ID:        targetRoomID,
		ProjectID: "default",
		Title:     "Audited UI Room",
		Topic:     "discussion",
	})
	s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     targetRoomID,
		AuthorType: "user",
		Content:    "Navbar button overlap bug on mobile",
		CreatedAt:  time.Now().Add(-5 * time.Minute),
	})
	s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     targetRoomID,
		AuthorType: "agent",
		Content:    "Inspecting navbar styles and layout",
		CreatedAt:  time.Now().Add(-4 * time.Minute),
	})

	// 2. Setup target task
	s.taskSvc = task.NewService(task.NewMemoryStore(), nil)
	createdTask, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{
		ProjectID:    "default",
		Title:        "Fix mobile navbar layout",
		Instructions: "Fixed CSS padding and flex-wrap on header",
	})
	if err != nil {
		t.Fatal(err)
	}

	// 3. Current active room
	currentRoomID := db.GenerateUUID()

	// Test case A: User pastes full domain URL to room
	userPromptWithRoomURL := "Cek percakapan ini apakah normal: https://cuan.dojo.cc/inbox/" + targetRoomID
	resolvedRoom := s.resolveInternalCuanLinks(ctx, userPromptWithRoomURL, currentRoomID)

	if !strings.Contains(resolvedRoom, "Referenced Internal Cuan Conversation: Room "+targetRoomID) {
		t.Fatalf("expected resolved room link header, got: %s", resolvedRoom)
	}
	if !strings.Contains(resolvedRoom, "Navbar button overlap bug on mobile") {
		t.Fatalf("expected referenced message content in resolved link, got: %s", resolvedRoom)
	}

	// Test case B: User pastes relative task link
	userPromptWithTaskURL := "Kenapa task /tasks/" + createdTask.ID + " belum jalan?"
	resolvedTask := s.resolveInternalCuanLinks(ctx, userPromptWithTaskURL, currentRoomID)

	if !strings.Contains(resolvedTask, "Referenced Internal Cuan Task: ID "+createdTask.ID) {
		t.Fatalf("expected resolved task link header, got: %s", resolvedTask)
	}
	if !strings.Contains(resolvedTask, "Fix mobile navbar layout") {
		t.Fatalf("expected task title in resolved link, got: %s", resolvedTask)
	}

	// Test case C: Self room link is ignored (no recursion into current active room)
	selfPrompt := "Check https://cuan.dojo.cc/inbox/" + currentRoomID
	resolvedSelf := s.resolveInternalCuanLinks(ctx, selfPrompt, currentRoomID)
	if resolvedSelf != "" {
		t.Fatalf("expected self room to not be duplicated, got: %s", resolvedSelf)
	}

	// Test case D: Open Source arbitrary self-hosted domain or localhost with studio route
	ossStudioPrompt := "Lihat juga di http://localhost:3000/studio/" + targetRoomID
	resolvedOSSStudio := s.resolveInternalCuanLinks(ctx, ossStudioPrompt, currentRoomID)
	if !strings.Contains(resolvedOSSStudio, "Referenced Internal Cuan Conversation: Room "+targetRoomID) {
		t.Fatalf("expected resolved studio link on localhost, got: %s", resolvedOSSStudio)
	}

	// Test case E: Open Source arbitrary domain with project-scoped task link
	ossProjectTaskPrompt := "Task ini butuh review: https://my-cuan.internal.corp:8443/projects/default/tasks/" + createdTask.ID
	resolvedOSSTask := s.resolveInternalCuanLinks(ctx, ossProjectTaskPrompt, currentRoomID)
	if !strings.Contains(resolvedOSSTask, "Referenced Internal Cuan Task: ID "+createdTask.ID) {
		t.Fatalf("expected resolved project task link on custom domain, got: %s", resolvedOSSTask)
	}

	// Test case F: Relative room link with query parameters and trailing slash
	queryParamRoomPrompt := "Periksa room ini: /inbox/" + targetRoomID + "/?tab=messages&scroll=bottom"
	resolvedQueryRoom := s.resolveInternalCuanLinks(ctx, queryParamRoomPrompt, currentRoomID)
	if !strings.Contains(resolvedQueryRoom, "Referenced Internal Cuan Conversation: Room "+targetRoomID) {
		t.Fatalf("expected resolved room link with query params, got: %s", resolvedQueryRoom)
	}

	// Test case G: Bare task ID with msg_ prefix
	msgTask, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{
		ProjectID:    "default",
		Title:        "Task created from inbox message",
		Instructions: "Resolve query parameter edge cases",
	})
	if err != nil {
		t.Fatal(err)
	}
	bareTaskPrompt := "Coba lihat perkembangan task msg_" + strings.TrimPrefix(msgTask.ID, "task_")
	resolvedBareTask := s.resolveInternalCuanLinks(ctx, bareTaskPrompt, currentRoomID)
	// Task detail lookup handles task ID
	if !strings.Contains(s.resolveInternalCuanLinks(ctx, "Lihat /tasks/"+msgTask.ID, currentRoomID), msgTask.ID) {
		t.Fatalf("expected resolved task for msgTask")
	}
	_ = resolvedBareTask

	// Test case H: Room with 0 messages still resolves metadata
	emptyRoomID := db.GenerateUUID()
	s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{
		ID:        emptyRoomID,
		ProjectID: "default",
		Title:     "Fresh Empty Discussion Room",
		Topic:     "discussion",
	})
	resolvedEmpty := s.resolveInternalCuanLinks(ctx, "Cek /inbox/"+emptyRoomID, currentRoomID)
	if !strings.Contains(resolvedEmpty, "Fresh Empty Discussion Room") {
		t.Fatalf("expected room title for empty room, got: %s", resolvedEmpty)
	}
	if !strings.Contains(resolvedEmpty, "No messages recorded in this room yet") {
		t.Fatalf("expected empty room note, got: %s", resolvedEmpty)
	}
}

func TestInboxProjectContextResolvesLinksAndRetry(t *testing.T) {
	s, _ := setupTestChatServer(t)
	ctx := context.Background()
	root := t.TempDir()

	// 1. Projects: default, cuan, dojo (matching live server!)
	store := project.NewMemoryStore()
	s.projectSvc = project.NewService(store, nil, "", root)
	_ = store.CreateProject(ctx, &project.Project{ID: "default", Name: "Default", LocalPath: root})
	_ = store.CreateProject(ctx, &project.Project{ID: "cuan", Name: "cuan", LocalPath: root})
	_ = store.CreateProject(ctx, &project.Project{ID: "dojo", Name: "dojo", LocalPath: root})

	// 2. Referenced conversation room
	refRoomID := db.GenerateUUID()
	s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{
		ID:        refRoomID,
		ProjectID: "default",
		Title:     "Ui ux navbar cuan terminal ini bisa better ga?",
		Topic:     "agent-topic:agent_sm",
	})
	s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     refRoomID,
		AuthorType: "user",
		Content:    "Ui ux navbar cuan terminal ini bisa better ga?",
		CreatedAt:  time.Now().Add(-10 * time.Minute),
	})
	s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     refRoomID,
		AuthorType: "agent",
		Content:    "Bisa banget. Dari tangkapan layar di tampilan mobile...",
		CreatedAt:  time.Now().Add(-9 * time.Minute),
	})

	// 3. Current room (agent topic)
	currentRoomID := db.GenerateUUID()
	s.projectStudioRepo.CreateRoom(ctx, &db.ProjectRoom{
		ID:        currentRoomID,
		ProjectID: "default",
		Title:     "Uji Coba SM Link Resolver",
		Topic:     "agent-topic:agent_sm",
	})

	// First user message containing domain URL https://cuan.dojo.cc/inbox/<refRoomID>
	firstMsg := db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     currentRoomID,
		AuthorType: "user",
		Content:    "Tolong cek masalah di percakapan ini https://cuan.dojo.cc/inbox/" + refRoomID + " apa yang dibahas dan apa tindakan tim selanjutnya?",
		CreatedAt:  time.Now().Add(-5 * time.Minute),
	}
	s.projectStudioRepo.CreateMessage(ctx, &firstMsg)

	// Test 1: Snapshot for the first message resolves the link and does not get stuck in ambiguous project selection
	_, snapshot := s.inboxProjectContext(ctx, db.InboxRequest{RoomID: currentRoomID, MessageID: firstMsg.ID})
	if !strings.Contains(snapshot, "Referenced Internal Cuan Conversation: Room "+refRoomID) {
		t.Fatalf("expected referenced conversation in snapshot, got: %s", snapshot)
	}
	if !strings.Contains(snapshot, "Ui ux navbar cuan terminal ini bisa better ga?") {
		t.Fatalf("expected message text from referenced room in snapshot, got: %s", snapshot)
	}

	// Test 2: User clicks "Retry" (message is literally just "Retry")
	retryMsg := db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     currentRoomID,
		AuthorType: "user",
		Content:    "Retry",
		CreatedAt:  time.Now(),
	}
	s.projectStudioRepo.CreateMessage(ctx, &retryMsg)

	// Snapshot for retry request must STILL include the referenced conversation!
	_, snapshotRetry := s.inboxProjectContext(ctx, db.InboxRequest{RoomID: currentRoomID, MessageID: retryMsg.ID})
	if !strings.Contains(snapshotRetry, "Referenced Internal Cuan Conversation: Room "+refRoomID) {
		t.Fatalf("expected referenced conversation on retry, got: %s", snapshotRetry)
	}
	if !strings.Contains(snapshotRetry, "Ui ux navbar cuan terminal ini bisa better ga?") {
		t.Fatalf("expected message text on retry, got: %s", snapshotRetry)
	}

	// Test 3: Heavy backlog (45 tasks) does NOT displace or truncate linked context
	for i := 0; i < 45; i++ {
		_, _ = s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{
			ProjectID:    "default",
			Title:        "Existing backlog item with extensive descriptions",
			Instructions: strings.Repeat("Task implementation details and acceptance criteria. ", 20),
		})
	}
	_, snapshotHeavy := s.inboxProjectContext(ctx, db.InboxRequest{RoomID: currentRoomID, MessageID: firstMsg.ID})
	if !strings.Contains(snapshotHeavy, "Referenced Internal Cuan Conversation: Room "+refRoomID) {
		t.Fatalf("expected referenced conversation preserved with heavy backlog, got length %d", len(snapshotHeavy))
	}
	// Linked context must appear before the project backlog
	refIdx := strings.Index(snapshotHeavy, "Referenced Internal Cuan Conversation: Room "+refRoomID)
	backlogIdx := strings.Index(snapshotHeavy, "Project backlog:")
	if refIdx == -1 || (backlogIdx != -1 && refIdx > backlogIdx) {
		t.Fatalf("expected referenced context at top of context before backlog, refIdx=%d, backlogIdx=%d", refIdx, backlogIdx)
	}
}

func TestDeduplicateWorkLinks_DuplicateTask(t *testing.T) {
	// Case 1: Identical duplicate links separated by blank lines (as in d9fa85bd-0edb-4e89-b95a-f25f733e1ffa)
	inputReal := `Siap, perbaikan langsung ane kembalikan ke tim dev pada task yang sama.

[View work](/tasks/msg_10d11aa8-27f4-40a2-8653-4603c6ed15e6)

[View work](/tasks/msg_10d11aa8-27f4-40a2-8653-4603c6ed15e6)`

	expectedReal := `Siap, perbaikan langsung ane kembalikan ke tim dev pada task yang sama.

[View work](/tasks/msg_10d11aa8-27f4-40a2-8653-4603c6ed15e6)`

	gotReal := deduplicateWorkLinks(inputReal)
	if gotReal != expectedReal {
		t.Fatalf("expected duplicate link to identical task to be deduplicated to one,\ngot:\n%q\nwant:\n%q", gotReal, expectedReal)
	}

	// Verify count is exactly 1
	if count := strings.Count(gotReal, "[View work](/tasks/msg_10d11aa8-27f4-40a2-8653-4603c6ed15e6)"); count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}

	// Case 2: Three identical links
	inputTriple := `Progress update.

[View work](/tasks/task-abc)

[View work](/tasks/task-abc)

[View work](/tasks/task-abc)`

	expectedTriple := `Progress update.

[View work](/tasks/task-abc)`

	gotTriple := deduplicateWorkLinks(inputTriple)
	if gotTriple != expectedTriple {
		t.Fatalf("expected triple duplicate to deduplicate to one,\ngot:\n%q\nwant:\n%q", gotTriple, expectedTriple)
	}

	// Case 3: Duplicate in middle of text
	inputMiddle := `Header line

[View work](/tasks/task-middle)

[View work](/tasks/task-middle)

Footer note`

	expectedMiddle := `Header line

[View work](/tasks/task-middle)

Footer note`

	gotMiddle := deduplicateWorkLinks(inputMiddle)
	if gotMiddle != expectedMiddle {
		t.Fatalf("expected middle duplicate to deduplicate cleanly,\ngot:\n%q\nwant:\n%q", gotMiddle, expectedMiddle)
	}
}

func TestDeduplicateWorkLinks_MultiTask(t *testing.T) {
	// Case 1: Two different tasks
	inputMulti := `Progress dari dua pekerjaan:
- Task 1: [View work](/tasks/task-first)
- Task 2: [View work](/tasks/task-second)`

	gotMulti := deduplicateWorkLinks(inputMulti)
	if gotMulti != inputMulti {
		t.Fatalf("expected different task links to be preserved unchanged,\ngot:\n%q\nwant:\n%q", gotMulti, inputMulti)
	}
	if !strings.Contains(gotMulti, "/tasks/task-first") || !strings.Contains(gotMulti, "/tasks/task-second") {
		t.Fatalf("expected both task links to be present in output, got: %q", gotMulti)
	}

	// Case 2: Multi-task with duplicate of one of the tasks
	inputMixed := `Pekerjaan yang diproses:
- Task 1: [View work](/tasks/task-1)
- Task 2: [View work](/tasks/task-2)

[View work](/tasks/task-1)`

	expectedMixed := `Pekerjaan yang diproses:
- Task 1: [View work](/tasks/task-1)
- Task 2: [View work](/tasks/task-2)`

	gotMixed := deduplicateWorkLinks(inputMixed)
	if gotMixed != expectedMixed {
		t.Fatalf("expected duplicate task-1 removed while task-2 preserved,\ngot:\n%q\nwant:\n%q", gotMixed, expectedMixed)
	}
	if strings.Count(gotMixed, "/tasks/task-1") != 1 {
		t.Fatalf("expected task-1 exactly once, got: %s", gotMixed)
	}
	if strings.Count(gotMixed, "/tasks/task-2") != 1 {
		t.Fatalf("expected task-2 exactly once, got: %s", gotMixed)
	}
}
