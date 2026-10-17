package db

import (
	"context"
	"testing"
)

func TestMemoryTaskProjectRepository_LinkAndUnlink(t *testing.T) {
	repo := NewMemoryTaskProjectRepository()
	ctx := context.Background()

	taskID := "task_abc123"
	project1 := "proj_1"
	project2 := "proj_2"

	// 1. Link project1
	if err := repo.LinkTaskProject(ctx, taskID, project1); err != nil {
		t.Fatalf("failed to link task to project1: %v", err)
	}

	// 2. Link project2
	if err := repo.LinkTaskProject(ctx, taskID, project2); err != nil {
		t.Fatalf("failed to link task to project2: %v", err)
	}

	// 3. Link duplicate should not error
	if err := repo.LinkTaskProject(ctx, taskID, project1); err != nil {
		t.Fatalf("expected duplicate link to succeed: %v", err)
	}

	// 4. Verify projects by task
	projs, err := repo.GetProjectsByTask(ctx, taskID)
	if err != nil {
		t.Fatalf("GetProjectsByTask failed: %v", err)
	}
	if len(projs) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projs))
	}

	// 5. Verify tasks by project
	tasks1, err := repo.GetTasksByProject(ctx, project1)
	if err != nil {
		t.Fatalf("GetTasksByProject failed: %v", err)
	}
	if len(tasks1) != 1 || tasks1[0] != taskID {
		t.Fatalf("expected [task_abc123], got %v", tasks1)
	}

	count, err := repo.CountTasksByProject(ctx, project1)
	if err != nil {
		t.Fatalf("CountTasksByProject failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}

	// 6. Unlink project1
	if err := repo.UnlinkTaskProject(ctx, taskID, project1); err != nil {
		t.Fatalf("UnlinkTaskProject failed: %v", err)
	}

	countAfter, err := repo.CountTasksByProject(ctx, project1)
	if err != nil {
		t.Fatalf("CountTasksByProject failed: %v", err)
	}
	if countAfter != 0 {
		t.Fatalf("expected count 0 after unlink, got %d", countAfter)
	}

	// 7. SetTaskProjects batch
	if err := repo.SetTaskProjects(ctx, taskID, []string{"proj_a", "proj_b", "proj_c"}); err != nil {
		t.Fatalf("SetTaskProjects failed: %v", err)
	}
	newProjs, err := repo.GetProjectsByTask(ctx, taskID)
	if err != nil {
		t.Fatalf("GetProjectsByTask failed: %v", err)
	}
	if len(newProjs) != 3 {
		t.Fatalf("expected 3 projects after SetTaskProjects, got %d", len(newProjs))
	}
}

func TestMemoryTaskProjectRepository_Validation(t *testing.T) {
	repo := NewMemoryTaskProjectRepository()
	ctx := context.Background()

	if err := repo.LinkTaskProject(ctx, "", "proj_1"); err != ErrTaskProjectInvalidInput {
		t.Fatalf("expected ErrTaskProjectInvalidInput, got %v", err)
	}
	if err := repo.LinkTaskProject(ctx, "task_1", ""); err != ErrTaskProjectInvalidInput {
		t.Fatalf("expected ErrTaskProjectInvalidInput, got %v", err)
	}
	if _, err := repo.GetProjectsByTask(ctx, ""); err != ErrTaskProjectInvalidInput {
		t.Fatalf("expected ErrTaskProjectInvalidInput, got %v", err)
	}
	if _, err := repo.GetTasksByProject(ctx, ""); err != ErrTaskProjectInvalidInput {
		t.Fatalf("expected ErrTaskProjectInvalidInput, got %v", err)
	}
}
