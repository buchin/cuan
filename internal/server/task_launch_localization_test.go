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

func TestTaskLaunchQueuedNotificationLocalization(t *testing.T) {
	s, token, call, _ := inboxFixture(t)
	ctx := context.Background()
	owner, err := s.authSvc.ValidateToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}

	// Setup tasks and projects stores
	tasksStore := task.NewMemoryStore()
	s.taskSvc = task.NewService(tasksStore, nil)
	projectsStore := project.NewMemoryStore()
	projectsStore.CreateProject(ctx, &project.Project{ID: "test-proj", Name: "Test project"})
	s.projectSvc = project.NewService(projectsStore, nil, "", t.TempDir())

	// Occupy lane with an active running task to trigger runner concurrency lock
	activeTask := &task.Task{
		ID:                "active-task-1",
		ProjectID:         "test-proj",
		Status:            task.TaskStatusDoing,
		ExecutionCategory: task.ExecutionCategoryCode,
	}
	if err := s.taskSvc.Store().CreateTask(ctx, activeTask); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.Store().CreateAttempt(ctx, &task.TaskAttempt{
		ID:        "active-att-1",
		TaskID:    activeTask.ID,
		Status:    task.AttemptStatusRunning,
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	// Create second task that will be queued behind the active task
	queuedTask, err := s.taskSvc.CreateTask(ctx, owner.ID, task.CreateTaskRequest{
		Title:        "Queued Task Behind Active",
		Instructions: "Instructions",
		ProjectID:    "test-proj",
		SourceRoomID: "test-room",
	})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := s.taskSvc.GetTaskDetail(ctx, queuedTask.ID)
	if err != nil {
		t.Fatal(err)
	}

	// --- Scenario 1: Fallback to English when account locale is NOT set ---
	qEnglishFallback := db.InboxRequest{
		OwnerID:   owner.ID,
		AgentID:   "agent_lifecycle",
		RoomID:    "test-room",
		RequestID: db.GenerateUUID(),
		MessageID: db.GenerateUUID(),
	}
	msgEnglishFallback := s.startAuthorizedLaunch(ctx, qEnglishFallback, detail, qEnglishFallback.RequestID, "orig", "revision")
	if !strings.Contains(msgEnglishFallback, "The work is queued waiting for the active project task to complete. I’ll start it automatically and report progress here.") {
		t.Fatalf("expected English fallback message when locale not set, got: %s", msgEnglishFallback)
	}
	if !strings.Contains(msgEnglishFallback, "[View work](/tasks/"+queuedTask.ID+")") {
		t.Fatalf("expected [View work] link, got: %s", msgEnglishFallback)
	}

	// --- Scenario 2: Indonesian notification when User Settings locale is set to Indonesian ("id") ---
	// Update profile to set locale = "id"
	w := call("PATCH", "/api/v1/profile", `{"locale":"id"}`)
	if w.Code != 200 {
		t.Fatalf("PATCH profile failed with code %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"locale":"id"`) {
		t.Fatalf("expected locale:id in PATCH response, got: %s", w.Body.String())
	}

	// Verify /auth/me returns locale = "id"
	me := call("GET", "/api/v1/auth/me", "")
	if !strings.Contains(me.Body.String(), `"locale":"id"`) {
		t.Fatalf("expected locale:id in /auth/me response, got: %s", me.Body.String())
	}

	// Now launch the queued task
	qIndonesian := db.InboxRequest{
		OwnerID:   owner.ID,
		AgentID:   "agent_lifecycle",
		RoomID:    "test-room",
		RequestID: db.GenerateUUID(),
		MessageID: db.GenerateUUID(),
	}
	msgIndonesian := s.startAuthorizedLaunch(ctx, qIndonesian, detail, qIndonesian.RequestID, "orig", "revision")
	expectedIDMsg := "Pekerjaan masuk dalam antrean menunggu tugas yang sedang aktif selesai. Saya akan memulainya secara otomatis dan mengabarkan progresnya di sini."
	if !strings.Contains(msgIndonesian, expectedIDMsg) {
		t.Fatalf("expected Indonesian queue notification, got: %s", msgIndonesian)
	}
	if !strings.Contains(msgIndonesian, "[View work](/tasks/"+queuedTask.ID+")") {
		t.Fatalf("expected [View work] link in Indonesian message, got: %s", msgIndonesian)
	}

	// Verify the message passes isIntermediateRunnerMessage filter (no unread/push spam)
	if !isIntermediateRunnerMessage("Cuan", msgIndonesian) {
		t.Fatalf("isIntermediateRunnerMessage should return true for Indonesian queued message")
	}

	// --- Scenario 3: Explicit English locale ("en") returns English message ---
	wEn := call("PATCH", "/api/v1/profile", `{"locale":"en"}`)
	if wEn.Code != 200 {
		t.Fatalf("PATCH profile locale en failed: %s", wEn.Body.String())
	}
	qEnExplicit := db.InboxRequest{
		OwnerID:   owner.ID,
		AgentID:   "agent_lifecycle",
		RoomID:    "test-room",
		RequestID: db.GenerateUUID(),
		MessageID: db.GenerateUUID(),
	}
	msgEnExplicit := s.startAuthorizedLaunch(ctx, qEnExplicit, detail, qEnExplicit.RequestID, "orig", "revision")
	if !strings.Contains(msgEnExplicit, "The work is queued waiting for the active project task to complete. I’ll start it automatically and report progress here.") {
		t.Fatalf("expected English message for locale en, got: %s", msgEnExplicit)
	}

	// --- Scenario 4: finishQueuedTaskLaunch localization ---
	projectsStore.CreateProject(ctx, &project.Project{ID: "test-proj-2", Name: "Test project 2"})
	queuedTask2, err := s.taskSvc.CreateTask(ctx, owner.ID, task.CreateTaskRequest{
		Title:        "Queued Task 2",
		Instructions: "Instructions",
		ProjectID:    "test-proj-2",
		SourceRoomID: "test-room",
	})
	if err != nil {
		t.Fatal(err)
	}
	detail2, err := s.taskSvc.GetTaskDetail(ctx, queuedTask2.ID)
	if err != nil {
		t.Fatal(err)
	}

	nowLaunch := time.Now().UTC()
	candidateLaunch := task.TaskLaunch{
		TaskID:           queuedTask2.ID,
		ProjectID:        "test-proj-2",
		RequestID:        "req-finish-test",
		OwnerID:          owner.ID,
		RoomID:           "test-room",
		SourceRequestID:  "source-req-1",
		AgentID:          "agent_lifecycle",
		Revision:         "revision",
		InstructionsHash: task.LaunchInstructionsHash(detail2.Instructions),
		CreatedAt:        nowLaunch,
		AvailableAt:      nowLaunch,
		ExpiresAt:        nowLaunch.Add(24 * time.Hour),
	}
	savedLaunch, err := s.taskSvc.SaveLaunch(ctx, candidateLaunch)
	if err != nil {
		t.Fatal(err)
	}
	claimedLaunch, err := s.taskSvc.ClaimLaunch(ctx, time.Now(), "test-lease", savedLaunch.ID)
	if err != nil || claimedLaunch == nil {
		t.Fatalf("claim launch: %v", err)
	}

	// For locale = "en":
	finishEn := s.finishQueuedTaskLaunch(ctx, *claimedLaunch, false, "queued", "active project task running")
	if !strings.Contains(finishEn, "The work is queued waiting for the active project task to complete") {
		t.Fatalf("expected English finishQueuedTaskLaunch, got: %s", finishEn)
	}

	// Switch back to "id" and test finishQueuedTaskLaunch
	call("PATCH", "/api/v1/profile", `{"locale":"id"}`)
	candidateLaunch3 := task.TaskLaunch{
		TaskID:           queuedTask2.ID,
		ProjectID:        "test-proj-2",
		RequestID:        "req-finish-test-id",
		OwnerID:          owner.ID,
		RoomID:           "test-room",
		SourceRequestID:  "source-req-2",
		AgentID:          "agent_lifecycle",
		Revision:         "revision",
		InstructionsHash: task.LaunchInstructionsHash(detail2.Instructions),
		CreatedAt:        nowLaunch,
		AvailableAt:      nowLaunch,
		ExpiresAt:        nowLaunch.Add(24 * time.Hour),
	}
	savedLaunch3, err := s.taskSvc.SaveLaunch(ctx, candidateLaunch3)
	if err != nil {
		t.Fatal(err)
	}
	claimedLaunch3, err := s.taskSvc.ClaimLaunch(ctx, time.Now(), "test-lease-3", savedLaunch3.ID)
	if err != nil || claimedLaunch3 == nil {
		t.Fatalf("claim launch 3: %v", err)
	}
	finishID := s.finishQueuedTaskLaunch(ctx, *claimedLaunch3, false, "queued", "active project task running")
	if !strings.Contains(finishID, expectedIDMsg) {
		t.Fatalf("expected Indonesian finishQueuedTaskLaunch, got: %s", finishID)
	}
}
