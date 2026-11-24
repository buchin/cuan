package db

import (
	"context"
	"testing"
	"time"
)

func TestMemoryProjectDeploymentRepository_CRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryProjectDeploymentRepository()

	dep := &ProjectDeployment{
		ID:            "dep-1",
		ProjectID:     "proj-100",
		CommitSHA:     "d381fe0123456789abcdef",
		CommitMessage: "Refactor user profile update function",
		Branch:        "main",
		Status:        DeploymentStatusBuilding,
		TriggerSource: TriggerSourceManual,
		TriggerAuthor: "Joe Dixon",
		StartedAt:     time.Now().Add(-1 * time.Minute),
		Steps: []ProjectDeploymentStep{
			{
				ID:       "step-1",
				Name:     "Source Checkout",
				Status:   StepStatusSuccess,
				Duration: "2s",
				Summary:  "Checked out commit d381fe0",
				Log:      "git checkout main\nHEAD is at d381fe0",
			},
			{
				ID:       "step-2",
				Name:     "Compile & Build",
				Status:   StepStatusBuilding,
				Duration: "12s",
				Summary:  "Building frontend and backend",
				Log:      "npm run build...",
			},
		},
	}

	// 1. Create
	if err := repo.Create(ctx, dep); err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}

	// 2. GetByID
	fetched, err := repo.GetByID(ctx, "dep-1")
	if err != nil {
		t.Fatalf("GetByID() unexpected error: %v", err)
	}
	if fetched.ID != "dep-1" || fetched.ShortSHA != "d381fe0" {
		t.Errorf("GetByID() mismatch, short_sha=%s", fetched.ShortSHA)
	}
	if len(fetched.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(fetched.Steps))
	}

	// 3. Update
	now := time.Now()
	fetched.Status = DeploymentStatusSuccess
	fetched.CompletedAt = &now
	fetched.DurationSeconds = 42
	fetched.Steps[1].Status = StepStatusSuccess
	fetched.Steps[1].Duration = "28s"

	if err := repo.Update(ctx, fetched); err != nil {
		t.Fatalf("Update() unexpected error: %v", err)
	}

	updated, err := repo.GetByID(ctx, "dep-1")
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if updated.Status != DeploymentStatusSuccess || updated.DurationSeconds != 42 {
		t.Errorf("unexpected updated status=%s or duration=%d", updated.Status, updated.DurationSeconds)
	}
	if dur := updated.FormatDuration(); dur != "42s" {
		t.Errorf("expected FormatDuration '42s', got '%s'", dur)
	}

	// 4. Latest Success
	latest, err := repo.GetLatestSuccess(ctx, "proj-100")
	if err != nil {
		t.Fatalf("GetLatestSuccess() error: %v", err)
	}
	if latest.ID != "dep-1" {
		t.Errorf("expected dep-1 as latest, got %s", latest.ID)
	}

	// 5. List
	list, err := repo.ListByProjectID(ctx, "proj-100", 10)
	if err != nil {
		t.Fatalf("ListByProjectID() error: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 item, got %d", len(list))
	}

	// 6. Delete
	if err := repo.Delete(ctx, "dep-1"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	_, err = repo.GetByID(ctx, "dep-1")
	if err != ErrProjectDeploymentNotFound {
		t.Errorf("expected ErrProjectDeploymentNotFound, got %v", err)
	}
}
