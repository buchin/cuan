package campaign

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/buchin/cuan/internal/db"
)

func TestParseTargetCriteria(t *testing.T) {
	// 1. Empty / null / {}
	c1, err := ParseTargetCriteria(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error on empty: %v", err)
	}
	if !c1.All {
		t.Errorf("expected All=true on empty criteria, got false")
	}

	// 2. Object with project_ids and tags
	c2, err := ParseTargetCriteria(json.RawMessage(`{
		"project_ids": ["proj-1", "proj-2"],
		"tags": ["WordPress", "AdSense"],
		"exclude_ids": ["proj-3"]
	}`))
	if err != nil {
		t.Fatalf("unexpected error on object: %v", err)
	}
	if len(c2.ProjectIDs) != 2 || c2.ProjectIDs[0] != "proj-1" {
		t.Errorf("unexpected ProjectIDs: %+v", c2.ProjectIDs)
	}
	if len(c2.Tags) != 2 || c2.Tags[0] != "wordpress" || c2.Tags[1] != "adsense" {
		t.Errorf("tags should be normalized to lowercase: %+v", c2.Tags)
	}
	if len(c2.ExcludeIDs) != 1 || c2.ExcludeIDs[0] != "proj-3" {
		t.Errorf("unexpected ExcludeIDs: %+v", c2.ExcludeIDs)
	}

	// 3. Array format with tags prefix
	c3, err := ParseTargetCriteria(json.RawMessage(`["proj-10", "tag:WordPress", "tag:adsense", "proj-11"]`))
	if err != nil {
		t.Fatalf("unexpected error on array: %v", err)
	}
	if len(c3.ProjectIDs) != 2 || c3.ProjectIDs[0] != "proj-10" || c3.ProjectIDs[1] != "proj-11" {
		t.Errorf("unexpected ProjectIDs from array: %+v", c3.ProjectIDs)
	}
	if len(c3.Tags) != 2 || c3.Tags[0] != "wordpress" || c3.Tags[1] != "adsense" {
		t.Errorf("unexpected Tags from array: %+v", c3.Tags)
	}
}

func TestParseTemplatePayload(t *testing.T) {
	// 1. Polymorphic DoD checklist as strings
	raw1 := json.RawMessage(`{
		"title_template": "[{{project_name}}] Auto AdSense Injection",
		"instructions": "Place snippet in header.php",
		"script": "<script async src=\"https://ads.js\"></script>",
		"tags": ["WordPress", "AdSense"],
		"priority": "high",
		"dod_checklist": [
			"Check snippet header placement",
			"Verify HTTP 200 on home"
		]
	}`)

	p1, err := ParseTemplatePayload(raw1)
	if err != nil {
		t.Fatalf("ParseTemplatePayload error: %v", err)
	}
	if p1.TitleTemplate != "[{{project_name}}] Auto AdSense Injection" {
		t.Errorf("unexpected title_template: %s", p1.TitleTemplate)
	}
	if p1.Priority != "high" {
		t.Errorf("unexpected priority: %s", p1.Priority)
	}
	if len(p1.Tags) != 2 || p1.Tags[0] != "wordpress" {
		t.Errorf("unexpected tags: %+v", p1.Tags)
	}
	if len(p1.DoDChecklist) != 2 {
		t.Fatalf("expected 2 DoD items, got %d", len(p1.DoDChecklist))
	}
	if p1.DoDChecklist[0].Text != "Check snippet header placement" {
		t.Errorf("unexpected DoD text: %s", p1.DoDChecklist[0].Text)
	}

	// 2. Polymorphic DoD checklist as DoDItem objects
	raw2 := json.RawMessage(`{
		"dod_checklist": [
			{"id": "d1", "text": "Explicit item 1", "completed": true},
			{"text": "Explicit item 2", "completed": false}
		]
	}`)
	p2, err := ParseTemplatePayload(raw2)
	if err != nil {
		t.Fatalf("ParseTemplatePayload error: %v", err)
	}
	if len(p2.DoDChecklist) != 2 {
		t.Fatalf("expected 2 DoD items, got %d", len(p2.DoDChecklist))
	}
	if p2.DoDChecklist[0].ID != "d1" || !p2.DoDChecklist[0].Completed {
		t.Errorf("unexpected DoD item 0: %+v", p2.DoDChecklist[0])
	}
	if p2.DoDChecklist[1].ID == "" {
		t.Errorf("expected auto UUID for item without ID")
	}
}

func TestDecompilerEngine_ProjectResolution(t *testing.T) {
	ctx := context.Background()

	resolver := NewDefaultProjectResolver(nil, nil)
	resolver.AddProject(ProjectInfo{ID: "p1", Name: "Blog WordPress Alpha", Tags: []string{"wordpress"}})
	resolver.AddProject(ProjectInfo{ID: "p2", Name: "E-Commerce Site", Tags: []string{"magento"}})
	resolver.AddProject(ProjectInfo{ID: "p3", Name: "AdSense News Portal", Tags: []string{"wordpress", "adsense"}})
	resolver.AddProject(ProjectInfo{ID: "p4", Name: "Documentation Hub", Tags: []string{"docs"}})

	// 1. Resolve by tag "wordpress"
	resWP, err := resolver.ResolveProjects(ctx, TargetCriteria{Tags: []string{"wordpress"}})
	if err != nil {
		t.Fatalf("ResolveProjects error: %v", err)
	}
	// p1 and p3 match
	if len(resWP) != 2 {
		t.Fatalf("expected 2 wordpress projects, got %d", len(resWP))
	}

	// 2. Resolve by explicit IDs
	resIDs, err := resolver.ResolveProjects(ctx, TargetCriteria{ProjectIDs: []string{"p2", "p4"}})
	if err != nil {
		t.Fatalf("ResolveProjects error: %v", err)
	}
	if len(resIDs) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(resIDs))
	}

	// 3. Resolve by All with Exclude
	resAll, err := resolver.ResolveProjects(ctx, TargetCriteria{All: true, ExcludeIDs: []string{"p2"}})
	if err != nil {
		t.Fatalf("ResolveProjects error: %v", err)
	}
	if len(resAll) != 3 {
		t.Fatalf("expected 3 projects (4 minus 1 excluded), got %d", len(resAll))
	}
}

func TestDecompilerEngine_DoDChecklistGuarantee(t *testing.T) {
	engine := NewDecompilerEngine(WithMinDoDItems(3))

	// Case 1: 0 items provided -> must generate >= 3 verifiable items
	list0 := engine.GenerateDoDChecklist(nil, "Site Alpha")
	if len(list0) < 3 {
		t.Fatalf("expected at least 3 DoD items, got %d", len(list0))
	}
	for _, it := range list0 {
		if it.ID == "" || it.Text == "" {
			t.Errorf("invalid DoD item: %+v", it)
		}
	}

	// Case 2: 1 item provided -> must preserve it and append until >= 3
	custom := []db.DoDItem{{Text: "Custom check on Site Alpha"}}
	list1 := engine.GenerateDoDChecklist(custom, "Site Alpha")
	if len(list1) < 3 {
		t.Fatalf("expected at least 3 DoD items, got %d", len(list1))
	}
	if list1[0].Text != "Custom check on Site Alpha" {
		t.Errorf("custom item was not preserved as first item")
	}

	// Case 3: 4 items provided -> all 4 must be preserved
	fourItems := []db.DoDItem{
		{Text: "Item 1"},
		{Text: "Item 2"},
		{Text: "Item 3"},
		{Text: "Item 4"},
	}
	list4 := engine.GenerateDoDChecklist(fourItems, "Site Alpha")
	if len(list4) != 4 {
		t.Fatalf("expected 4 DoD items, got %d", len(list4))
	}
}

func TestDecompilerEngine_DecompileAndPersist(t *testing.T) {
	ctx := context.Background()
	studioRepo := db.NewMemoryProjectStudioRepository()

	resolver := NewDefaultProjectResolver(nil, studioRepo)
	resolver.AddProject(ProjectInfo{ID: "proj-wp-1", Name: "WP Portal 1", Tags: []string{"wordpress"}})
	resolver.AddProject(ProjectInfo{ID: "proj-wp-2", Name: "WP Portal 2", Tags: []string{"wordpress"}})
	resolver.AddProject(ProjectInfo{ID: "proj-wp-3", Name: "WP Portal 3", Tags: []string{"wordpress"}})

	engine := NewDecompilerEngine(
		WithProjectResolver(resolver),
		WithProjectStudioRepo(studioRepo),
		WithMinDoDItems(3),
	)

	campaign := &db.BatchCampaign{
		ID:          db.GenerateUUID(),
		Title:       "AdSense Mass Rollout",
		Description: "Deploy AdSense script across all WordPress installations",
		Status:      db.CampaignStatusDraft,
		TargetCriteria: json.RawMessage(`{
			"tags": ["wordpress"]
		}`),
		TemplatePayload: json.RawMessage(`{
			"title_template": "[{{project_name}}] AdSense Tag Rollout",
			"instructions": "Inject snippet in header.php",
			"script": "<script async src=\"https://ads.example.com/tag.js\"></script>",
			"tags": ["wordpress", "adsense"],
			"priority": "high",
			"dod_checklist": [
				"Verify snippet in header.php"
			]
		}`),
	}

	result, err := engine.DecompileAndPersist(ctx, campaign)
	if err != nil {
		t.Fatalf("DecompileAndPersist error: %v", err)
	}

	// Verify result
	if result.CampaignID != campaign.ID {
		t.Errorf("expected CampaignID %s, got %s", campaign.ID, result.CampaignID)
	}
	if result.BatchID != campaign.ID {
		t.Errorf("expected BatchID %s, got %s", campaign.ID, result.BatchID)
	}
	if len(result.Subtasks) != 3 {
		t.Fatalf("expected 3 subtasks generated, got %d", len(result.Subtasks))
	}
	if result.ParentTaskID == nil || *result.ParentTaskID == "" {
		t.Fatalf("expected parent_task_id to be populated")
	}

	// Verify each subtask
	for i, st := range result.Subtasks {
		if st.ID == "" || !db.IsValidUUID(st.ID) {
			t.Errorf("subtask %d has invalid UUID: %s", i, st.ID)
		}
		if st.ProjectID != fmt.Sprintf("proj-wp-%d", i+1) {
			t.Errorf("subtask %d unexpected project_id: %s", i, st.ProjectID)
		}
		if st.BatchID == nil || *st.BatchID != campaign.ID {
			t.Errorf("subtask %d batch_id mismatch: %+v", i, st.BatchID)
		}
		if st.ParentTaskID == nil || *st.ParentTaskID != *result.ParentTaskID {
			t.Errorf("subtask %d parent_task_id mismatch: %+v", i, st.ParentTaskID)
		}
		expectedTitle := fmt.Sprintf("[WP Portal %d] AdSense Tag Rollout", i+1)
		if st.Title != expectedTitle {
			t.Errorf("subtask %d unexpected title: %s (expected %s)", i, st.Title, expectedTitle)
		}
		if st.Priority != "high" {
			t.Errorf("subtask %d priority: %s", i, st.Priority)
		}
		if st.Status != "backlog" {
			t.Errorf("subtask %d status: %s", i, st.Status)
		}
		// DoD checklist must have at least 3 items
		if len(st.DoDChecklist) < 3 {
			t.Fatalf("subtask %d DoD items count %d < 3", i, len(st.DoDChecklist))
		}
		if st.DoDChecklist[0].Text != "Verify snippet in header.php" {
			t.Errorf("subtask %d DoD item 0 mismatch: %s", i, st.DoDChecklist[0].Text)
		}
	}

	// Verify persistence in studioRepo
	tasksInBatch, err := studioRepo.ListTasksByBatchID(ctx, campaign.ID)
	if err != nil {
		t.Fatalf("ListTasksByBatchID error: %v", err)
	}
	// 1 parent task + 3 subtasks = 4 tasks
	if len(tasksInBatch) != 4 {
		t.Fatalf("expected 4 tasks in batch, got %d", len(tasksInBatch))
	}

	// Verify parent task query
	children, err := studioRepo.ListTasksByParentID(ctx, *result.ParentTaskID)
	if err != nil {
		t.Fatalf("ListTasksByParentID error: %v", err)
	}
	if len(children) != 3 {
		t.Fatalf("expected 3 child tasks under parent, got %d", len(children))
	}
}

func TestDecompilerEngine_ErrorsAndEdgeCases(t *testing.T) {
	ctx := context.Background()

	// 1. Nil campaign error
	engine := NewDecompilerEngine()
	if _, err := engine.Decompile(ctx, nil); !errors.Is(err, ErrNilCampaign) {
		t.Errorf("expected ErrNilCampaign, got %v", err)
	}

	// 2. No target projects found
	resolver := NewDefaultProjectResolver(nil, nil)
	engineWithResolver := NewDecompilerEngine(WithProjectResolver(resolver))

	campaignNoProjects := &db.BatchCampaign{
		ID:             db.GenerateUUID(),
		Title:          "Orphan Campaign",
		TargetCriteria: json.RawMessage(`{"project_ids": ["non-existent"]}`),
	}
	if _, err := engineWithResolver.Decompile(ctx, campaignNoProjects); !errors.Is(err, ErrNoTargetProjectsFound) {
		t.Errorf("expected ErrNoTargetProjectsFound, got %v", err)
	}
}
