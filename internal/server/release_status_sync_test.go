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

func TestInboxLinkedTasksProjectsReleaseStatus(t *testing.T) {
	ctx := context.Background()
	taskStore := task.NewMemoryStore()
	taskSvc := task.NewService(taskStore, nil)
	projStore := project.NewMemoryStore()
	proj := &project.Project{
		ID:   "proj_cuan",
		Name: "Cuan App",
	}
	if err := projStore.CreateProject(ctx, proj); err != nil {
		t.Fatal(err)
	}
	projSvc := project.NewService(projStore, nil, "", t.TempDir())
	depRepo := db.NewMemoryProjectDeploymentRepository()
	studioRepo := db.NewMemoryProjectStudioRepository()
	roomID := "room_release_sync"
	now := time.Now().UTC()

	commitFailed := "1111111111111111111111111111111111111111"
	commitBuilding := "2222222222222222222222222222222222222222"
	commitSuccess := "3333333333333333333333333333333333333333"

	for _, dep := range []*db.ProjectDeployment{
		{ID: "dep_fail", ProjectID: proj.ID, CommitSHA: commitFailed, ShortSHA: commitFailed[:7], Branch: "dev", Status: db.DeploymentStatusFailed, StartedAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "dep_build", ProjectID: proj.ID, CommitSHA: commitBuilding, ShortSHA: commitBuilding[:7], Branch: "dev", Status: db.DeploymentStatusBuilding, StartedAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "dep_ok", ProjectID: proj.ID, CommitSHA: commitSuccess, ShortSHA: commitSuccess[:7], Branch: "dev", Status: db.DeploymentStatusSuccess, StartedAt: now, CreatedAt: now, UpdatedAt: now},
	} {
		if err := depRepo.Create(ctx, dep); err != nil {
			t.Fatal(err)
		}
	}

	for _, item := range []*task.Task{
		{ID: "t_fail", ProjectID: proj.ID, SourceRoomID: roomID, Title: "Failed release task", Status: task.TaskStatusDone, ExecutionCategory: task.ExecutionCategoryCode, CommitSHA: commitFailed, CreatedAt: now, UpdatedAt: now},
		{ID: "t_build", ProjectID: proj.ID, SourceRoomID: roomID, Title: "Building release task", Status: task.TaskStatusDone, ExecutionCategory: task.ExecutionCategoryCode, CommitSHA: commitBuilding, CreatedAt: now, UpdatedAt: now},
		{ID: "t_live", ProjectID: proj.ID, SourceRoomID: roomID, Title: "Live release task", Status: task.TaskStatusDone, ExecutionCategory: task.ExecutionCategoryCode, CommitSHA: commitSuccess, CreatedAt: now, UpdatedAt: now},
	} {
		if err := taskStore.CreateTask(ctx, item); err != nil {
			t.Fatal(err)
		}
	}

	srv := &Server{
		taskSvc:               taskSvc,
		projectSvc:            projSvc,
		projectDeploymentRepo: depRepo,
		projectStudioRepo:     studioRepo,
	}

	items, err := srv.inboxLinkedTasks(ctx, roomID, nil)
	if err != nil {
		t.Fatalf("inboxLinkedTasks error: %v", err)
	}
	got := map[string]string{}
	for _, it := range items {
		got[it.ID] = it.ReleaseStatus
	}
	if got["t_fail"] != "failed" {
		t.Fatalf("expected t_fail release_status=failed, got %q", got["t_fail"])
	}
	if got["t_build"] != "deploying" {
		t.Fatalf("expected t_build release_status=deploying, got %q", got["t_build"])
	}
	if got["t_live"] != "live" {
		t.Fatalf("expected t_live release_status=live, got %q", got["t_live"])
	}
}

func TestReconcileVerifiedPublicationTasksPostsCompletionAfterFailureNotice(t *testing.T) {
	ctx := context.Background()
	taskStore := task.NewMemoryStore()
	taskSvc := task.NewService(taskStore, nil)
	studioRepo := db.NewMemoryProjectStudioRepository()

	roomID := db.GenerateUUID()
	if err := studioRepo.CreateRoom(ctx, &db.ProjectRoom{
		ID:        roomID,
		ProjectID: "proj_test",
		Title:     "Saat task masih berjalan:",
	}); err != nil {
		t.Fatal(err)
	}

	taskID := "msg_reconcile_chat_sync"
	summary := "Indikator status di gelembung chat sudah terverifikasi berdenyut saat task berjalan."
	now := time.Now().UTC()

	// Prior hanging failure message in the room referencing /tasks/<id> and containing summary
	if err := studioRepo.CreateMessage(ctx, &db.ProjectMessage{
		ID:         db.GenerateUUID(),
		RoomID:     roomID,
		AuthorType: "agent",
		AuthorID:   "agent_cuan",
		AuthorName: "@cuan",
		Content:    summary + " tetapi pembaruan ini belum masuk ke aplikasi yang sedang berjalan.\n\n[View work](/tasks/" + taskID + ")",
		CreatedAt:  now.Add(-5 * time.Minute),
		UpdatedAt:  now.Add(-5 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	// Task has empty SourceRoomID initially and is in review with verified live commit
	if err := taskStore.CreateTask(ctx, &task.Task{
		ID:                taskID,
		ProjectID:         "proj_test",
		Title:             "Saat task masih berjalan:",
		Status:            task.TaskStatusReview,
		ExecutionCategory: task.ExecutionCategoryCode,
		Summary:           summary,
		CompleteNote:      "Verifikasi independen PASS.",
		DoDChecklist:      []task.DoDItem{{ID: "d1", Text: "Berdenyut", Completed: true}},
		CreatedAt:         now.Add(-6 * time.Minute),
		UpdatedAt:         now,
	}); err != nil {
		t.Fatal(err)
	}

	srv := &Server{
		taskSvc:           taskSvc,
		projectStudioRepo: studioRepo,
	}
	srv.reconcileVerifiedPublicationTasks(ctx)

	updated, err := taskStore.GetTask(ctx, taskID)
	if err != nil || updated.Status != task.TaskStatusDone {
		t.Fatalf("expected task reconciled to done, got err=%v status=%v", err, updated)
	}
	msgs, err := studioRepo.ListMessagesByRoomID(ctx, roomID)
	if err != nil {
		t.Fatal(err)
	}
	foundCompletion := false
	for _, m := range msgs {
		if strings.Contains(m.Content, "telah selesai dan diverifikasi") && strings.Contains(m.Content, "/tasks/"+taskID) {
			foundCompletion = true
			break
		}
	}
	if !foundCompletion {
		t.Fatalf("expected completion message to be posted to room %s after reconciliation, got %d messages", roomID, len(msgs))
	}
}
