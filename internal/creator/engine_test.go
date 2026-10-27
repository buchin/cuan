package creator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
)

func setupTestEngine() (*BlueprintEngine, project.Store, db.ProjectStudioRepository, db.AgentRepository) {
	projStore := project.NewMemoryStore()
	studioRepo := db.NewMemoryProjectStudioRepository()
	agentRepo := db.NewMemoryAgentRepository()

	engine := NewBlueprintEngine(
		WithProjectStore(projStore),
		WithStudioRepo(studioRepo),
		WithAgentRepo(agentRepo),
	)

	return engine, projStore, studioRepo, agentRepo
}

func TestCatalogTemplates(t *testing.T) {
	templates := DefaultTemplates()
	if len(templates) != 4 {
		t.Fatalf("expected 4 canonical creator templates, got %d", len(templates))
	}

	expectedIDs := map[string]string{
		BlueprintBlog:      "SEO Specialist",
		BlueprintYouTube:   "YouTube Scriptwriter",
		BlueprintOlshop:    "Conversion Copywriter",
		BlueprintAffiliate: "Affiliate Strategist",
	}

	for _, tmpl := range templates {
		expectedAgent, exists := expectedIDs[tmpl.ID]
		if !exists {
			t.Errorf("unexpected template ID: %s", tmpl.ID)
		}

		if tmpl.Name == "" {
			t.Errorf("[%s] expected non-empty Name", tmpl.ID)
		}
		if tmpl.Description == "" {
			t.Errorf("[%s] expected non-empty Description", tmpl.ID)
		}
		if tmpl.Category == "" {
			t.Errorf("[%s] expected non-empty Category", tmpl.ID)
		}
		if tmpl.Icon == "" {
			t.Errorf("[%s] expected non-empty Icon", tmpl.ID)
		}

		// Room verification
		if tmpl.Room.Title == "" || tmpl.Room.Topic == "" {
			t.Errorf("[%s] expected non-empty Room Title and Topic", tmpl.ID)
		}

		// Agent persona verification
		if tmpl.Agent.Name != expectedAgent {
			t.Errorf("[%s] expected agent name '%s', got '%s'", tmpl.ID, expectedAgent, tmpl.Agent.Name)
		}
		if tmpl.Agent.Role == "" || tmpl.Agent.SystemPrompt == "" {
			t.Errorf("[%s] expected non-empty agent Role and SystemPrompt", tmpl.ID)
		}

		// Task pipeline verification
		if len(tmpl.Tasks) < 3 {
			t.Errorf("[%s] expected at least 3 pipeline tasks, got %d", tmpl.ID, len(tmpl.Tasks))
		}

		for idx, task := range tmpl.Tasks {
			if task.Title == "" {
				t.Errorf("[%s task %d] expected non-empty Title", tmpl.ID, idx)
			}
			if task.Description == "" {
				t.Errorf("[%s task %d] expected non-empty Description", tmpl.ID, idx)
			}
			if task.Priority == "" {
				t.Errorf("[%s task %d] expected non-empty Priority", tmpl.ID, idx)
			}
			if len(task.Tags) == 0 {
				t.Errorf("[%s task %d] expected non-empty Tags", tmpl.ID, idx)
			}
			if len(task.DoDChecklist) < 3 {
				t.Errorf("[%s task %d] expected at least 3 DoD checklist items, got %d", tmpl.ID, idx, len(task.DoDChecklist))
			}
			for dIdx, item := range task.DoDChecklist {
				if item.Text == "" {
					t.Errorf("[%s task %d dod %d] expected non-empty Text", tmpl.ID, idx, dIdx)
				}
				if item.Completed {
					t.Errorf("[%s task %d dod %d] initial DoD item should not be completed", tmpl.ID, idx, dIdx)
				}
			}
		}
	}
}

func TestLookupTemplate_SynonymsAndAliases(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
		found    bool
	}{
		{"blog", BlueprintBlog, true},
		{"blog_publisher", BlueprintBlog, true},
		{"blogger", BlueprintBlog, true},
		{"web-publisher", BlueprintBlog, true},

		{"youtube", BlueprintYouTube, true},
		{"youtube_channel", BlueprintYouTube, true},
		{"video", BlueprintYouTube, true},
		{"yt", BlueprintYouTube, true},

		{"olshop", BlueprintOlshop, true},
		{"online_shop", BlueprintOlshop, true},
		{"digital_product", BlueprintOlshop, true},
		{"ecommerce", BlueprintOlshop, true},
		{"shop", BlueprintOlshop, true},

		{"affiliate", BlueprintAffiliate, true},
		{"affiliate_site", BlueprintAffiliate, true},
		{"affiliate-marketing", BlueprintAffiliate, true},
		{"niche_affiliate", BlueprintAffiliate, true},

		{"unknown_template", "", false},
		{"", "", false},
	}

	for _, tc := range testCases {
		tmpl, ok := LookupTemplate(tc.input)
		if ok != tc.found {
			t.Errorf("LookupTemplate(%q) found=%v, expected %v", tc.input, ok, tc.found)
		}
		if tc.found && tmpl.ID != tc.expected {
			t.Errorf("LookupTemplate(%q) id=%q, expected %q", tc.input, tmpl.ID, tc.expected)
		}
	}
}

func TestInstantiate_AllFourTemplates(t *testing.T) {
	ctx := context.Background()
	engine, projStore, studioRepo, agentRepo := setupTestEngine()

	templateIDs := []string{
		BlueprintBlog,
		BlueprintYouTube,
		BlueprintOlshop,
		BlueprintAffiliate,
	}

	for _, tID := range templateIDs {
		t.Run(tID, func(t *testing.T) {
			req := InstantiateRequest{
				TemplateID:  tID,
				ProjectName: fmt.Sprintf("My Test %s Project", strings.Title(tID)),
			}

			resp, err := engine.Instantiate(ctx, req)
			if err != nil {
				t.Fatalf("unexpected instantiate error: %v", err)
			}

			if resp.Status != "success" {
				t.Errorf("expected status 'success', got %q", resp.Status)
			}
			if resp.TemplateID != tID {
				t.Errorf("expected template_id %q, got %q", tID, resp.TemplateID)
			}

			// 1. Verify Project
			if resp.Project == nil || resp.Project.ID == "" {
				t.Fatal("expected non-nil project with valid ID")
			}
			p, err := projStore.GetProject(ctx, resp.Project.ID)
			if err != nil {
				t.Fatalf("project not found in store: %v", err)
			}
			if p.ProjectType != tID {
				t.Errorf("expected project_type %q, got %q", tID, p.ProjectType)
			}
			if p.Name != req.ProjectName {
				t.Errorf("expected project name %q, got %q", req.ProjectName, p.Name)
			}

			// 2. Verify Room
			if resp.Room.ID == "" {
				t.Fatal("expected non-empty room ID in response")
			}
			room, err := studioRepo.GetRoomByID(ctx, resp.Room.ID)
			if err != nil {
				t.Fatalf("room not found in studio repo: %v", err)
			}
			if room.ProjectID != p.ID {
				t.Errorf("room project_id %q mismatch with project ID %q", room.ProjectID, p.ID)
			}

			// Verify welcome message in room
			messages, err := studioRepo.ListMessagesByRoomID(ctx, room.ID)
			if err != nil {
				t.Fatalf("failed to list room messages: %v", err)
			}
			if len(messages) == 0 {
				t.Errorf("expected initial welcome message in room, got 0")
			} else if !strings.Contains(messages[0].Content, "Welcome to") {
				t.Errorf("expected welcome message, got %q", messages[0].Content)
			}

			// 3. Verify Agent Persona
			if resp.Agent.ID == "" {
				t.Fatal("expected non-empty agent ID in response")
			}
			agent, err := agentRepo.GetAgentByID(ctx, resp.Agent.ID)
			if err != nil {
				t.Fatalf("agent not found in agent repo: %v", err)
			}
			if agent.Status != db.AgentStatusActive {
				t.Errorf("expected active agent status, got %q", agent.Status)
			}

			// 4. Verify Tasks Pipeline
			if len(resp.Tasks) < 3 {
				t.Fatalf("expected at least 3 created tasks, got %d", len(resp.Tasks))
			}
			dbTasks, err := studioRepo.ListTasksByProjectID(ctx, p.ID)
			if err != nil {
				t.Fatalf("failed to list tasks by project ID: %v", err)
			}
			if len(dbTasks) != len(resp.Tasks) {
				t.Errorf("expected %d tasks in studio repo, got %d", len(resp.Tasks), len(dbTasks))
			}

			for _, task := range resp.Tasks {
				if task.ProjectID != p.ID {
					t.Errorf("task project_id %q mismatch", task.ProjectID)
				}
				if task.RoomID == nil || *task.RoomID != room.ID {
					t.Errorf("task room_id %v mismatch with room %q", task.RoomID, room.ID)
				}
				if task.AssigneeID == nil || *task.AssigneeID != agent.ID {
					t.Errorf("task assignee_id %v mismatch with agent %q", task.AssigneeID, agent.ID)
				}
				if len(task.DoDChecklist) < 3 {
					t.Errorf("task %q expected >=3 DoD items, got %d", task.Title, len(task.DoDChecklist))
				}
				if task.ExecutionStatus != "idle" {
					t.Errorf("task execution_status expected 'idle', got %q", task.ExecutionStatus)
				}
			}
		})
	}
}

func TestInstantiate_InvalidTemplate(t *testing.T) {
	ctx := context.Background()
	engine, _, _, _ := setupTestEngine()

	req := InstantiateRequest{
		TemplateID:  "non_existent_blueprint",
		ProjectName: "Broken Project",
	}

	_, err := engine.Instantiate(ctx, req)
	if err == nil {
		t.Fatal("expected error for non-existent blueprint, got nil")
	}
}

func TestInstantiate_ReusesAgentPersona(t *testing.T) {
	ctx := context.Background()
	engine, _, _, agentRepo := setupTestEngine()

	// 1st instantiation
	resp1, err := engine.Instantiate(ctx, InstantiateRequest{
		TemplateID:  BlueprintBlog,
		ProjectName: "Blog Project 1",
	})
	if err != nil {
		t.Fatalf("instantiate 1 failed: %v", err)
	}

	// 2nd instantiation of same template
	resp2, err := engine.Instantiate(ctx, InstantiateRequest{
		TemplateID:  BlueprintBlog,
		ProjectName: "Blog Project 2",
	})
	if err != nil {
		t.Fatalf("instantiate 2 failed: %v", err)
	}

	// Should reuse the same agent persona ID
	if resp1.Agent.ID != resp2.Agent.ID {
		t.Errorf("expected agent persona to be reused (same ID), got %q vs %q", resp1.Agent.ID, resp2.Agent.ID)
	}

	count, _ := agentRepo.CountAgents(ctx, db.AgentFilter{Search: "SEO Specialist"})
	if count != 1 {
		t.Errorf("expected exactly 1 SEO Specialist persona in repo, got %d", count)
	}
}

func TestInstantiate_CustomRepoAndMetadata(t *testing.T) {
	ctx := context.Background()
	engine, projStore, _, _ := setupTestEngine()

	customMeta := map[string]interface{}{
		"target_niche":  "artificial_intelligence",
		"monetization": "adsense_plus_sponsorship",
	}
	metaBytes, _ := json.Marshal(customMeta)

	req := InstantiateRequest{
		TemplateID:    BlueprintBlog,
		ProjectName:   "AI Insights Daily",
		RepoURL:       "https://github.com/myorg/ai-insights",
		DefaultBranch: "develop",
		Metadata:      metaBytes,
	}

	resp, err := engine.Instantiate(ctx, req)
	if err != nil {
		t.Fatalf("instantiate failed: %v", err)
	}

	p, err := projStore.GetProject(ctx, resp.Project.ID)
	if err != nil {
		t.Fatalf("get project failed: %v", err)
	}

	if p.RepoURL != "https://github.com/myorg/ai-insights" {
		t.Errorf("expected repo_url %q, got %q", req.RepoURL, p.RepoURL)
	}
	if p.RepoOwner != "myorg" || p.RepoName != "ai-insights" {
		t.Errorf("expected owner/name myorg/ai-insights, got %s/%s", p.RepoOwner, p.RepoName)
	}
	if p.DefaultBranch != "develop" {
		t.Errorf("expected default branch 'develop', got %q", p.DefaultBranch)
	}

	var storedMeta map[string]interface{}
	if err := json.Unmarshal(p.Metadata, &storedMeta); err != nil {
		t.Fatalf("unmarshal metadata failed: %v", err)
	}
	if storedMeta["target_niche"] != "artificial_intelligence" {
		t.Errorf("custom metadata not found or incorrect: %v", storedMeta)
	}
	if storedMeta["blueprint_id"] != BlueprintBlog {
		t.Errorf("blueprint_id not injected in metadata: %v", storedMeta)
	}
}

func TestInstantiate_Concurrency(t *testing.T) {
	ctx := context.Background()
	engine, projStore, _, _ := setupTestEngine()

	concurrency := 8
	var wg sync.WaitGroup
	errCh := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tmpl := BlueprintBlog
			if idx%4 == 1 {
				tmpl = BlueprintYouTube
			} else if idx%4 == 2 {
				tmpl = BlueprintOlshop
			} else if idx%4 == 3 {
				tmpl = BlueprintAffiliate
			}

			req := InstantiateRequest{
				TemplateID:  tmpl,
				ProjectName: fmt.Sprintf("Concurrent Project %d", idx),
			}
			_, err := engine.Instantiate(ctx, req)
			if err != nil {
				errCh <- fmt.Errorf("worker %d failed: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrency error: %v", err)
	}

	projects, err := projStore.ListProjects(ctx)
	if err != nil {
		t.Fatalf("list projects failed: %v", err)
	}
	if len(projects) != concurrency {
		t.Errorf("expected %d projects created, got %d", concurrency, len(projects))
	}
}

func TestInstantiate_WithProjectRepo(t *testing.T) {
	ctx := context.Background()
	projectRepo := db.NewMemoryProjectRepository()
	studioRepo := db.NewMemoryProjectStudioRepository()
	agentRepo := db.NewMemoryAgentRepository()

	engine := NewBlueprintEngine(
		WithProjectRepo(projectRepo),
		WithStudioRepo(studioRepo),
		WithAgentRepo(agentRepo),
	)

	req := InstantiateRequest{
		TemplateID:  BlueprintYouTube,
		ProjectName: "YouTube Studio Pro",
	}

	resp, err := engine.Instantiate(ctx, req)
	if err != nil {
		t.Fatalf("instantiate with project repo failed: %v", err)
	}

	if resp.Project == nil || resp.Project.ID == "" {
		t.Fatal("expected non-empty project ID")
	}

	p, err := projectRepo.GetProjectByID(ctx, resp.Project.ID)
	if err != nil {
		t.Fatalf("project not found in projectRepo: %v", err)
	}
	if p.ProjectType != BlueprintYouTube {
		t.Errorf("expected project type %s, got %s", BlueprintYouTube, p.ProjectType)
	}
	if p.Name != "YouTube Studio Pro" {
		t.Errorf("expected name 'YouTube Studio Pro', got %s", p.Name)
	}
}
