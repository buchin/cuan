package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestProject_ValidationAndNormalization(t *testing.T) {
	if NormalizeProjectType("") != "custom" {
		t.Errorf("expected empty project_type to normalize to custom, got %s", NormalizeProjectType(""))
	}
	if NormalizeProjectType("  BLOG  ") != "blog" {
		t.Errorf("expected '  BLOG  ' to normalize to blog, got %s", NormalizeProjectType("  BLOG  "))
	}
	if NormalizeProjectType("YouTube") != "youtube" {
		t.Errorf("expected 'YouTube' to normalize to youtube, got %s", NormalizeProjectType("YouTube"))
	}

	p := &Project{
		ID:             "proj-creator-001",
		Name:           "My Creator Blog",
		RepoURL:        "https://github.com/creator/blog",
		RepoOwner:      "creator",
		RepoName:       "blog",
		DefaultBranch:  "main",
		LocalPath:      "/data/projects/blog",
		EncryptedToken: "secret-aes-encrypted-pat",
		TokenPreview:   "ghp_****1234",
		CloneStatus:    "synced",
		ProjectType:    "blog",
		Metadata:       json.RawMessage(`{"blueprint":"blog_publisher","niche":"finance"}`),
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	dto := p.ToDTO()
	if dto.ID != p.ID {
		t.Errorf("expected dto ID %s, got %s", p.ID, dto.ID)
	}
	if dto.ProjectType != "blog" {
		t.Errorf("expected dto ProjectType blog, got %s", dto.ProjectType)
	}
	if !dto.HasToken {
		t.Errorf("expected HasToken true")
	}

	// Verify JSON serialization omits EncryptedToken
	jsonBytes, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("failed to marshal project: %v", err)
	}
	if string(jsonBytes) == "" {
		t.Fatalf("empty json output")
	}
	var rawMap map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &rawMap); err != nil {
		t.Fatalf("failed to unmarshal json to map: %v", err)
	}
	if _, exists := rawMap["encrypted_token"]; exists {
		t.Errorf("encrypted_token must not be exported in JSON")
	}
}

func TestMemoryProjectRepository_CRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryProjectRepository()

	// 1. Validation errors
	if err := repo.CreateProject(ctx, nil); err == nil {
		t.Errorf("expected error creating nil project")
	}
	if err := repo.CreateProject(ctx, &Project{Name: "", RepoURL: "https://github.com/test/repo"}); !errors.Is(err, ErrEmptyProjectName) {
		t.Errorf("expected ErrEmptyProjectName, got %v", err)
	}
	if err := repo.CreateProject(ctx, &Project{Name: "Test", RepoURL: ""}); !errors.Is(err, ErrEmptyRepoURL) {
		t.Errorf("expected ErrEmptyRepoURL, got %v", err)
	}

	// 2. Successful creation
	p1 := &Project{
		ID:          "proj-1",
		Name:        "Creator YouTube Channel",
		RepoURL:     "https://github.com/creator/yt",
		RepoOwner:   "creator",
		RepoName:    "yt",
		ProjectType: "youtube",
		Metadata:    json.RawMessage(`{"blueprint":"youtube_channel"}`),
	}
	if err := repo.CreateProject(ctx, p1); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// 3. Duplicate prevention
	dup := &Project{
		Name:      "Duplicate Repo",
		RepoURL:   "https://github.com/creator/yt",
		RepoOwner: "creator",
		RepoName:  "yt",
	}
	if err := repo.CreateProject(ctx, dup); !errors.Is(err, ErrProjectAlreadyExists) {
		t.Errorf("expected ErrProjectAlreadyExists, got %v", err)
	}

	// 4. GetByID
	fetched, err := repo.GetProjectByID(ctx, "proj-1")
	if err != nil {
		t.Fatalf("failed to get project by id: %v", err)
	}
	if fetched.ProjectType != "youtube" {
		t.Errorf("expected project_type youtube, got %s", fetched.ProjectType)
	}

	// 5. GetByRepo (case-insensitive)
	byRepo, err := repo.GetProjectByRepo(ctx, "CREATOR", "YT")
	if err != nil {
		t.Fatalf("failed to get project by repo: %v", err)
	}
	if byRepo.ID != "proj-1" {
		t.Errorf("expected id proj-1, got %s", byRepo.ID)
	}

	// 6. Update
	fetched.Name = "Creator YouTube Studio Pro"
	fetched.ProjectType = "youtube"
	fetched.Metadata = json.RawMessage(`{"blueprint":"youtube_channel","subscribers":50000}`)
	if err := repo.UpdateProject(ctx, fetched); err != nil {
		t.Fatalf("failed to update project: %v", err)
	}

	updated, err := repo.GetProjectByID(ctx, "proj-1")
	if err != nil {
		t.Fatalf("failed to get updated project: %v", err)
	}
	if updated.Name != "Creator YouTube Studio Pro" {
		t.Errorf("expected updated name, got %s", updated.Name)
	}

	// 7. Delete
	if err := repo.DeleteProject(ctx, "proj-1"); err != nil {
		t.Fatalf("failed to delete project: %v", err)
	}
	if _, err := repo.GetProjectByID(ctx, "proj-1"); !errors.Is(err, ErrProjectNotFound) {
		t.Errorf("expected ErrProjectNotFound after delete, got %v", err)
	}
}

func TestMemoryProjectRepository_ListAndFilter(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryProjectRepository()

	projects := []*Project{
		{ID: "p-blog-1", Name: "Blog 1", RepoURL: "https://github.com/test/blog1", ProjectType: "blog"},
		{ID: "p-blog-2", Name: "Blog 2", RepoURL: "https://github.com/test/blog2", ProjectType: "blog"},
		{ID: "p-yt-1", Name: "YT 1", RepoURL: "https://github.com/test/yt1", ProjectType: "youtube"},
		{ID: "p-shop-1", Name: "Shop 1", RepoURL: "https://github.com/test/shop1", ProjectType: "olshop"},
		{ID: "p-custom-1", Name: "Custom 1", RepoURL: "https://github.com/test/custom1", ProjectType: "custom"},
	}

	for _, p := range projects {
		if err := repo.CreateProject(ctx, p); err != nil {
			t.Fatalf("failed to create project %s: %v", p.ID, err)
		}
	}

	all, err := repo.ListProjects(ctx)
	if err != nil {
		t.Fatalf("failed to list projects: %v", err)
	}
	if len(all) != 5 {
		t.Errorf("expected 5 projects, got %d", len(all))
	}

	blogs, err := repo.ListProjectsByType(ctx, "blog")
	if err != nil {
		t.Fatalf("failed to list blog projects: %v", err)
	}
	if len(blogs) != 2 {
		t.Errorf("expected 2 blog projects, got %d", len(blogs))
	}

	yts, err := repo.ListProjectsByType(ctx, "youtube")
	if err != nil {
		t.Fatalf("failed to list youtube projects: %v", err)
	}
	if len(yts) != 1 {
		t.Errorf("expected 1 youtube project, got %d", len(yts))
	}

	shops, err := repo.ListProjectsByType(ctx, "olshop")
	if err != nil {
		t.Fatalf("failed to list olshop projects: %v", err)
	}
	if len(shops) != 1 {
		t.Errorf("expected 1 olshop project, got %d", len(shops))
	}
}

func TestMemoryProjectRepository_ConcurrencyThreadSafety(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryProjectRepository()

	workers := 10
	iterations := 50
	var wg sync.WaitGroup
	wg.Add(workers)

	for w := 0; w < workers; w++ {
		workerID := w
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				projID := fmt.Sprintf("proj-w%d-i%d", workerID, i)
				p := &Project{
					ID:          projID,
					Name:        fmt.Sprintf("Project %s", projID),
					RepoURL:     fmt.Sprintf("https://github.com/worker%d/repo%d", workerID, i),
					RepoOwner:   fmt.Sprintf("worker%d", workerID),
					RepoName:    fmt.Sprintf("repo%d", i),
					ProjectType: "blog",
					Metadata:    json.RawMessage(`{"status":"concurrent"}`),
				}
				if err := repo.CreateProject(ctx, p); err != nil {
					t.Errorf("worker %d failed to create project: %v", workerID, err)
					return
				}

				fetched, err := repo.GetProjectByID(ctx, projID)
				if err != nil || fetched == nil {
					t.Errorf("worker %d failed to get project %s: %v", workerID, projID, err)
					return
				}

				fetched.Name = fetched.Name + " Updated"
				_ = repo.UpdateProject(ctx, fetched)

				_, _ = repo.ListProjectsByType(ctx, "blog")
			}
		}()
	}

	wg.Wait()

	all, err := repo.ListProjects(ctx)
	if err != nil {
		t.Fatalf("failed to list projects after concurrency: %v", err)
	}
	if len(all) != workers*iterations {
		t.Errorf("expected %d projects, got %d", workers*iterations, len(all))
	}
}

func TestRepositories_ProjectIntegration(t *testing.T) {
	repos := NewMemoryRepositories()
	if repos.Projects == nil {
		t.Errorf("expected Projects repository to be non-nil in Repositories")
	}
}
