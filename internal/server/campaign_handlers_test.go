package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/campaign"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/db"
)

func setupTestCampaignServer() (*Server, *db.MemoryBatchCampaignRepository, *db.MemoryProjectStudioRepository, *campaign.DefaultProjectResolver) {
	cfg := &config.Config{
		AuthSecret: "test-auth-secret-32-bytes-minimum!",
	}
	s := NewServer(cfg, nil)

	batchRepo := db.NewMemoryBatchCampaignRepository()
	studioRepo := db.NewMemoryProjectStudioRepository()

	resolver := campaign.NewDefaultProjectResolver(nil, studioRepo)
	resolver.AddProject(campaign.ProjectInfo{ID: "proj-wp-1", Name: "WordPress Alpha", Tags: []string{"wordpress"}})
	resolver.AddProject(campaign.ProjectInfo{ID: "proj-wp-2", Name: "WordPress Beta", Tags: []string{"wordpress"}})
	resolver.AddProject(campaign.ProjectInfo{ID: "proj-ads-1", Name: "AdSense Gamma", Tags: []string{"adsense"}})

	decompiler := campaign.NewDecompilerEngine(
		campaign.WithProjectResolver(resolver),
		campaign.WithProjectStudioRepo(studioRepo),
		campaign.WithMinDoDItems(3),
	)

	aggregator := campaign.NewRollupAggregator(
		campaign.WithAggregatorStudioRepo(studioRepo),
		campaign.WithAggregatorBatchRepo(batchRepo),
	)

	runner := campaign.NewMultiProjectRunner(
		campaign.WithMaxWorkers(5),
		campaign.WithRunnerBatchRepo(batchRepo),
		campaign.WithRunnerStudioRepo(studioRepo),
		campaign.WithDecompiler(decompiler),
		campaign.WithRunnerAggregator(aggregator),
	)

	s.SetBatchCampaignRepository(batchRepo)
	s.SetProjectStudioRepository(studioRepo)
	s.SetCampaignDecompiler(decompiler)
	s.SetCampaignRunner(runner)
	s.SetCampaignAggregator(aggregator)

	return s, batchRepo, studioRepo, resolver
}

func TestServer_CreateBatchCampaign(t *testing.T) {
	s, _, _, _ := setupTestCampaignServer()

	// 1. Successful creation
	payload := db.CreateBatchCampaignInput{
		Title:           "AdSense Code Mass Rollout",
		Description:     "Rollout new AdSense tracking script to 10 WordPress projects",
		TemplatePayload: json.RawMessage(`{"script": "<script src=\"ads.js\"></script>"}`),
		TargetCriteria:  json.RawMessage(`{"tags": ["wordpress"]}`),
		Status:          "draft",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/campaigns", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.Router().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected HTTP 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Status   string              `json:"status"`
		Campaign db.BatchCampaignDTO `json:"campaign"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "success" {
		t.Errorf("expected status 'success', got %s", resp.Status)
	}
	if resp.Campaign.Title != "AdSense Code Mass Rollout" {
		t.Errorf("expected title %s, got %s", payload.Title, resp.Campaign.Title)
	}
	if resp.Campaign.ID == "" || !db.IsValidUUID(resp.Campaign.ID) {
		t.Errorf("expected valid UUID, got %s", resp.Campaign.ID)
	}

	// 2. Empty title validation failure
	emptyTitlePayload := db.CreateBatchCampaignInput{
		Title: "   ",
	}
	bEmpty, _ := json.Marshal(emptyTitlePayload)
	req2 := httptest.NewRequest(http.MethodPost, "/api/campaigns", bytes.NewReader(bEmpty))
	w2 := httptest.NewRecorder()
	s.Router().ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Errorf("expected HTTP 400 Bad Request for empty title, got %d", w2.Code)
	}

	// 3. Invalid JSON payload
	req3 := httptest.NewRequest(http.MethodPost, "/api/campaigns", bytes.NewReader([]byte("{invalid-json")))
	w3 := httptest.NewRecorder()
	s.Router().ServeHTTP(w3, req3)
	if w3.Code != http.StatusBadRequest {
		t.Errorf("expected HTTP 400 Bad Request for invalid json, got %d", w3.Code)
	}

	// 4. Invalid campaign status
	badStatusPayload := db.CreateBatchCampaignInput{
		Title:  "Bad Status Campaign",
		Status: "invalid_status_xyz",
	}
	bBadStatus, _ := json.Marshal(badStatusPayload)
	req4 := httptest.NewRequest(http.MethodPost, "/api/campaigns", bytes.NewReader(bBadStatus))
	w4 := httptest.NewRecorder()
	s.Router().ServeHTTP(w4, req4)
	if w4.Code != http.StatusBadRequest {
		t.Errorf("expected HTTP 400 Bad Request for invalid status, got %d", w4.Code)
	}
}

func TestServer_ListBatchCampaigns(t *testing.T) {
	s, batchRepo, _, _ := setupTestCampaignServer()
	ctx := context.Background()

	c1 := &db.BatchCampaign{
		Title:       "WordPress Security Patch",
		Description: "Apply critical patch",
		Status:      db.CampaignStatusDraft,
	}
	c2 := &db.BatchCampaign{
		Title:       "AdSense Analytics Injection",
		Description: "Inject tracking code",
		Status:      db.CampaignStatusRunning,
	}
	c3 := &db.BatchCampaign{
		Title:       "SEO Header Optimization",
		Description: "Update meta tags",
		Status:      db.CampaignStatusCompleted,
	}
	_ = batchRepo.Create(ctx, c1)
	_ = batchRepo.Create(ctx, c2)
	_ = batchRepo.Create(ctx, c3)

	// 1. List all
	req := httptest.NewRequest(http.MethodGet, "/api/campaigns", nil)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK, got %d", w.Code)
	}

	var resp struct {
		Status    string                `json:"status"`
		Campaigns []db.BatchCampaignDTO `json:"campaigns"`
		Count     int                   `json:"count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Count != 3 || len(resp.Campaigns) != 3 {
		t.Errorf("expected count 3, got %d", resp.Count)
	}

	// 2. Filter by status: running
	req2 := httptest.NewRequest(http.MethodGet, "/api/campaigns?status=running", nil)
	w2 := httptest.NewRecorder()
	s.Router().ServeHTTP(w2, req2)

	var respRunning struct {
		Campaigns []db.BatchCampaignDTO `json:"campaigns"`
		Count     int                   `json:"count"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &respRunning)
	if respRunning.Count != 1 || respRunning.Campaigns[0].Title != c2.Title {
		t.Errorf("expected 1 running campaign (%s), got %d", c2.Title, respRunning.Count)
	}

	// 3. Search by keyword
	req3 := httptest.NewRequest(http.MethodGet, "/api/campaigns?search=Security", nil)
	w3 := httptest.NewRecorder()
	s.Router().ServeHTTP(w3, req3)

	var respSearch struct {
		Campaigns []db.BatchCampaignDTO `json:"campaigns"`
		Count     int                   `json:"count"`
	}
	_ = json.Unmarshal(w3.Body.Bytes(), &respSearch)
	if respSearch.Count != 1 || respSearch.Campaigns[0].Title != c1.Title {
		t.Errorf("expected 1 search result, got %d", respSearch.Count)
	}
}

func TestServer_GetBatchCampaign(t *testing.T) {
	s, batchRepo, studioRepo, _ := setupTestCampaignServer()
	ctx := context.Background()

	campaignID := db.GenerateUUID()
	c := &db.BatchCampaign{
		ID:          campaignID,
		Title:       "Test Get Campaign",
		Description: "Check detail retrieval",
		Status:      db.CampaignStatusDraft,
	}
	_ = batchRepo.Create(ctx, c)

	// Add child task under batch_id
	taskID := db.GenerateUUID()
	_ = studioRepo.CreateTask(ctx, &db.ProjectTask{
		ID:        taskID,
		ProjectID: "proj-wp-1",
		BatchID:   &campaignID,
		Title:     "Child task in batch",
		Status:    "backlog",
	})

	// 1. Successful get
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/campaigns/%s", campaignID), nil)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Status    string                `json:"status"`
		Campaign  db.BatchCampaignDTO   `json:"campaign"`
		Tasks     []db.ProjectTaskDTO   `json:"tasks"`
		TaskCount int                   `json:"task_count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Campaign.ID != campaignID {
		t.Errorf("expected campaign ID %s, got %s", campaignID, resp.Campaign.ID)
	}
	if resp.TaskCount != 1 || len(resp.Tasks) != 1 {
		t.Errorf("expected 1 child task, got %d", resp.TaskCount)
	}

	// 2. 404 Not Found
	nonExistent := db.GenerateUUID()
	req2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/campaigns/%s", nonExistent), nil)
	w2 := httptest.NewRecorder()
	s.Router().ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Errorf("expected HTTP 404, got %d", w2.Code)
	}

	// 3. 400 Bad UUID
	req3 := httptest.NewRequest(http.MethodGet, "/api/campaigns/not-a-uuid", nil)
	w3 := httptest.NewRecorder()
	s.Router().ServeHTTP(w3, req3)
	if w3.Code != http.StatusBadRequest {
		t.Errorf("expected HTTP 400, got %d", w3.Code)
	}
}

func TestServer_ExecuteBatchCampaign(t *testing.T) {
	s, batchRepo, studioRepo, _ := setupTestCampaignServer()
	ctx := context.Background()

	campaignID := db.GenerateUUID()
	c := &db.BatchCampaign{
		ID:          campaignID,
		Title:       "Execute Campaign Test",
		Description: "Rollout to WordPress targets",
		Status:      db.CampaignStatusDraft,
		TargetCriteria: json.RawMessage(`{
			"tags": ["wordpress"]
		}`),
		TemplatePayload: json.RawMessage(`{
			"instructions": "Inject AdSense tag into header",
			"script": "<script async src=\"https://ads.js\"></script>",
			"dod_checklist": [
				"Verify header injection"
			]
		}`),
	}
	if err := batchRepo.Create(ctx, c); err != nil {
		t.Fatalf("failed to create campaign: %v", err)
	}

	// Execute campaign via POST /api/campaigns/{id}/execute
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/campaigns/%s/execute", campaignID), nil)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Status          string                        `json:"status"`
		CampaignID      string                        `json:"campaign_id"`
		Campaign        db.BatchCampaignDTO           `json:"campaign"`
		TotalTasks      int                           `json:"total_tasks"`
		CompletedTasks  int                           `json:"completed_tasks"`
		FailedTasks     int                           `json:"failed_tasks"`
		PeakConcurrency int                           `json:"peak_concurrency"`
		Results         []*campaign.TaskExecutionResult `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "success" {
		t.Errorf("expected status 'success', got %s", resp.Status)
	}
	if resp.CampaignID != campaignID {
		t.Errorf("expected campaign ID %s, got %s", campaignID, resp.CampaignID)
	}
	if resp.TotalTasks != 2 { // proj-wp-1 and proj-wp-2 match "wordpress"
		t.Errorf("expected 2 total tasks, got %d", resp.TotalTasks)
	}
	if resp.CompletedTasks != 2 {
		t.Errorf("expected 2 completed tasks, got %d", resp.CompletedTasks)
	}
	if resp.FailedTasks != 0 {
		t.Errorf("expected 0 failed tasks, got %d", resp.FailedTasks)
	}
	if resp.Campaign.Status != db.CampaignStatusCompleted {
		t.Errorf("expected campaign status completed, got %s", resp.Campaign.Status)
	}

	// Verify child tasks in studioRepo
	tasksInBatch, err := studioRepo.ListTasksByBatchID(ctx, campaignID)
	if err != nil {
		t.Fatalf("ListTasksByBatchID error: %v", err)
	}
	// 1 parent epic + 2 child tasks = 3
	if len(tasksInBatch) != 3 {
		t.Fatalf("expected 3 tasks in batch, got %d", len(tasksInBatch))
	}
	for _, task := range tasksInBatch {
		if task.ParentTaskID != nil {
			if task.Status != "done" {
				t.Errorf("child task %s status expected 'done', got %s", task.ID, task.Status)
			}
			if len(task.DoDChecklist) < 3 {
				t.Errorf("child task %s DoD items count %d < 3", task.ID, len(task.DoDChecklist))
			}
		}
	}

	// 404 test on execute
	nonExistent := db.GenerateUUID()
	req2 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/campaigns/%s/execute", nonExistent), nil)
	w2 := httptest.NewRecorder()
	s.Router().ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Errorf("expected HTTP 404 on execute non-existent, got %d", w2.Code)
	}
}

func TestServer_CampaignRoutes_V1_Aliases(t *testing.T) {
	s, batchRepo, _, _ := setupTestCampaignServer()
	ctx := context.Background()

	campaignID := db.GenerateUUID()
	c := &db.BatchCampaign{
		ID:          campaignID,
		Title:       "V1 Route Alias Test",
		Description: "Check /api/v1/campaigns routes",
		Status:      db.CampaignStatusDraft,
		TargetCriteria: json.RawMessage(`{
			"tags": ["wordpress"]
		}`),
		TemplatePayload: json.RawMessage(`{
			"script": "alert(1);"
		}`),
	}
	_ = batchRepo.Create(ctx, c)

	// 1. GET /api/v1/campaigns
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/campaigns", nil)
	w1 := httptest.NewRecorder()
	s.Router().ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 on /api/v1/campaigns, got %d", w1.Code)
	}

	// 2. GET /api/v1/campaigns/{id}
	req2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/campaigns/%s", campaignID), nil)
	w2 := httptest.NewRecorder()
	s.Router().ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 on /api/v1/campaigns/{id}, got %d", w2.Code)
	}

	// 3. POST /api/v1/campaigns/{id}/execute
	req3 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/campaigns/%s/execute", campaignID), nil)
	w3 := httptest.NewRecorder()
	s.Router().ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 on /api/v1/campaigns/{id}/execute, got %d", w3.Code)
	}

	// 4. POST /api/v1/campaigns
	newPayload := db.CreateBatchCampaignInput{
		Title: "V1 Create Test",
	}
	bNew, _ := json.Marshal(newPayload)
	req4 := httptest.NewRequest(http.MethodPost, "/api/v1/campaigns", bytes.NewReader(bNew))
	w4 := httptest.NewRecorder()
	s.Router().ServeHTTP(w4, req4)
	if w4.Code != http.StatusCreated {
		t.Errorf("expected HTTP 201 on /api/v1/campaigns, got %d", w4.Code)
	}
}

func TestServer_GetCampaignRollup(t *testing.T) {
	s, batchRepo, studioRepo, _ := setupTestCampaignServer()
	ctx := context.Background()

	campaignID := db.GenerateUUID()
	c := &db.BatchCampaign{
		ID:          campaignID,
		Title:       "Rollup Test Campaign",
		Description: "Testing rollup metrics retrieval",
		Status:      db.CampaignStatusDraft,
	}
	_ = batchRepo.Create(ctx, c)

	// Add 3 subtasks to studioRepo
	for i := 0; i < 3; i++ {
		status := "backlog"
		if i == 0 {
			status = "done"
		} else if i == 1 {
			status = "doing"
		}
		_ = studioRepo.CreateTask(ctx, &db.ProjectTask{
			ID:        db.GenerateUUID(),
			ProjectID: db.GenerateUUID(),
			BatchID:   &campaignID,
			Title:     fmt.Sprintf("Subtask %d", i),
			Status:    status,
		})
	}

	// 1. GET /api/campaigns/{id}/rollup
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/campaigns/%s/rollup", campaignID), nil)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Status     string                  `json:"status"`
		CampaignID string                  `json:"campaign_id"`
		Rollup     *campaign.RollupMetrics `json:"rollup"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode rollup response: %v", err)
	}

	if resp.Status != "success" || resp.CampaignID != campaignID {
		t.Errorf("unexpected status %q or campaign_id %q", resp.Status, resp.CampaignID)
	}
	if resp.Rollup.TotalTasks != 3 || resp.Rollup.TotalTarget != 3 {
		t.Errorf("expected 3 total targets, got %d", resp.Rollup.TotalTasks)
	}
	if resp.Rollup.CompletedTasks != 1 {
		t.Errorf("expected 1 completed, got %d", resp.Rollup.CompletedTasks)
	}
	if resp.Rollup.RunningTasks != 1 {
		t.Errorf("expected 1 running, got %d", resp.Rollup.RunningTasks)
	}
	if resp.Rollup.PendingTasks != 1 {
		t.Errorf("expected 1 pending, got %d", resp.Rollup.PendingTasks)
	}
	if resp.Rollup.SuccessPercentage < 33.3 || resp.Rollup.SuccessPercentage > 33.4 {
		t.Errorf("expected ~33.33%% success, got %f", resp.Rollup.SuccessPercentage)
	}

	// 2. GET /api/v1/campaigns/{id}/rollup alias
	reqV1 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/campaigns/%s/rollup", campaignID), nil)
	wV1 := httptest.NewRecorder()
	s.Router().ServeHTTP(wV1, reqV1)
	if wV1.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 on /api/v1/campaigns/{id}/rollup, got %d", wV1.Code)
	}

	// 3. 404 for non-existent campaign
	nonExistent := db.GenerateUUID()
	req404 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/campaigns/%s/rollup", nonExistent), nil)
	w404 := httptest.NewRecorder()
	s.Router().ServeHTTP(w404, req404)
	if w404.Code != http.StatusNotFound {
		t.Errorf("expected HTTP 404 for non-existent campaign rollup, got %d", w404.Code)
	}

	// 4. 400 for bad UUID
	req400 := httptest.NewRequest(http.MethodGet, "/api/campaigns/bad-uuid/rollup", nil)
	w400 := httptest.NewRecorder()
	s.Router().ServeHTTP(w400, req400)
	if w400.Code != http.StatusBadRequest {
		t.Errorf("expected HTTP 400 for bad uuid, got %d", w400.Code)
	}
}

func TestServer_CampaignEventsSSE(t *testing.T) {
	s, batchRepo, _, _ := setupTestCampaignServer()
	ctx := context.Background()

	campaignID := db.GenerateUUID()
	c := &db.BatchCampaign{
		ID:          campaignID,
		Title:       "SSE Test Campaign",
		Description: "Verifying live SSE events and heartbeat",
		Status:      db.CampaignStatusDraft,
	}
	_ = batchRepo.Create(ctx, c)

	// Set short heartbeat interval for swift test verification
	s.CampaignAggregator().SetHeartbeatInterval(30 * time.Millisecond)

	ts := httptest.NewServer(s.Router())
	defer ts.Close()

	// 1. Check SSE connection, headers, handshake, heartbeat, and events
	clientCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(clientCtx, http.MethodGet, ts.URL+fmt.Sprintf("/api/campaigns/%s/events", campaignID), nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("failed to execute SSE request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected HTTP 200 for SSE stream, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("expected Content-Type text/event-stream, got %s", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Fatalf("expected Cache-Control no-cache, got %s", cc)
	}

	// Read lines concurrently
	linesChan := make(chan string, 100)
	errChan := make(chan error, 1)

	go func() {
		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF && !strings.Contains(err.Error(), "canceled") {
					errChan <- err
				}
				return
			}
			linesChan <- strings.TrimRight(line, "\r\n")
		}
	}()

	// Broadcast manual events via aggregator to test real-time SSE stream delivery
	time.Sleep(20 * time.Millisecond)
	s.CampaignAggregator().Broadcast(campaignID, campaign.EventTaskStatusChanged, campaign.TaskStatusChangedData{
		CampaignID: campaignID,
		TaskID:     "task-sse-1",
		ProjectID:  "proj-wp-1",
		Title:      "SSE Task 1",
		OldStatus:  "backlog",
		NewStatus:  "doing",
		Timestamp:  time.Now().UTC(),
	})

	s.CampaignAggregator().Broadcast(campaignID, campaign.EventCampaignCompleted, campaign.CampaignCompletedData{
		CampaignID: campaignID,
		Status:     "completed",
		Rollup: &campaign.RollupMetrics{
			CampaignID:        campaignID,
			TotalTasks:        1,
			CompletedTasks:    1,
			SuccessPercentage: 100.0,
			Status:            "completed",
		},
		Timestamp: time.Now().UTC(),
	})

	// Wait up to 500ms to collect events including heartbeat
	var collectedLines []string
	timeout := time.After(400 * time.Millisecond)

collectSSE:
	for {
		select {
		case line := <-linesChan:
			collectedLines = append(collectedLines, line)
		case <-timeout:
			break collectSSE
		}
	}

	joined := strings.Join(collectedLines, "\n")

	// Verify handshake
	if !strings.Contains(joined, "event: connected") {
		t.Errorf("expected 'event: connected' in SSE output, got:\n%s", joined)
	}
	// Verify heartbeat ping
	if !strings.Contains(joined, ": ping") {
		t.Errorf("expected heartbeat ': ping' in SSE output, got:\n%s", joined)
	}
	// Verify task.status_changed
	if !strings.Contains(joined, "event: task.status_changed") {
		t.Errorf("expected 'event: task.status_changed' in SSE output, got:\n%s", joined)
	}
	// Verify campaign.completed
	if !strings.Contains(joined, "event: campaign.completed") {
		t.Errorf("expected 'event: campaign.completed' in SSE output, got:\n%s", joined)
	}

	// 2. Disconnect client and verify graceful cleanup (zero memory leaks)
	cancel()
	_ = resp.Body.Close()

	// Wait for server goroutine to detect client disconnect
	time.Sleep(50 * time.Millisecond)
	if count := s.CampaignAggregator().SubscriberCount(campaignID); count != 0 {
		t.Errorf("expected subscriber count 0 after client disconnect, got %d", count)
	}

	// 3. 404 on non-existent campaign
	nonExistent := db.GenerateUUID()
	req404, _ := http.NewRequest(http.MethodGet, ts.URL+fmt.Sprintf("/api/campaigns/%s/events", nonExistent), nil)
	resp404, err := ts.Client().Do(req404)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp404.Body.Close()
	if resp404.StatusCode != http.StatusNotFound {
		t.Errorf("expected HTTP 404 for non-existent campaign events, got %d", resp404.StatusCode)
	}

	// 4. 400 on bad UUID
	req400, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/campaigns/bad-uuid/events", nil)
	resp400, err := ts.Client().Do(req400)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp400.Body.Close()
	if resp400.StatusCode != http.StatusBadRequest {
		t.Errorf("expected HTTP 400 for bad uuid events, got %d", resp400.StatusCode)
	}
}

func TestServer_Integrated_ExecuteAndLiveSSEStream(t *testing.T) {
	s, batchRepo, _, _ := setupTestCampaignServer()
	ctx := context.Background()

	campaignID := db.GenerateUUID()
	c := &db.BatchCampaign{
		ID:          campaignID,
		Title:       "Live SSE Stream Drill",
		Description: "Execute campaign with live SSE stream client attached",
		Status:      db.CampaignStatusDraft,
		TargetCriteria: json.RawMessage(`{
			"tags": ["wordpress"]
		}`),
		TemplatePayload: json.RawMessage(`{
			"instructions": "Inject live banner script",
			"script": "<script src=\"banner.js\"></script>",
			"dod_checklist": ["Banner rendered", "No error"]
		}`),
	}
	_ = batchRepo.Create(ctx, c)

	s.CampaignAggregator().SetHeartbeatInterval(50 * time.Millisecond)

	ts := httptest.NewServer(s.Router())
	defer ts.Close()

	// Connect SSE client
	clientCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sseReq, err := http.NewRequestWithContext(clientCtx, http.MethodGet, ts.URL+fmt.Sprintf("/api/v1/campaigns/%s/events", campaignID), nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	sseResp, err := ts.Client().Do(sseReq)
	if err != nil {
		t.Fatalf("failed to connect SSE: %v", err)
	}
	defer sseResp.Body.Close()

	if sseResp.StatusCode != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", sseResp.StatusCode)
	}

	var eventsMu sync.Mutex
	var collectedEvents []string
	go func() {
		reader := bufio.NewReader(sseResp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			tLine := strings.TrimRight(line, "\r\n")
			if strings.HasPrefix(tLine, "event: ") {
				eventsMu.Lock()
				collectedEvents = append(collectedEvents, tLine[7:])
				eventsMu.Unlock()
			}
		}
	}()

	// Wait 20ms for subscription setup, then execute campaign via POST /api/campaigns/{id}/execute
	time.Sleep(20 * time.Millisecond)
	execReq, _ := http.NewRequest(http.MethodPost, ts.URL+fmt.Sprintf("/api/campaigns/%s/execute", campaignID), nil)
	execResp, err := ts.Client().Do(execReq)
	if err != nil {
		t.Fatalf("failed to execute campaign: %v", err)
	}
	defer execResp.Body.Close()

	if execResp.StatusCode != http.StatusOK {
		t.Fatalf("expected HTTP 200 on execute, got %d", execResp.StatusCode)
	}

	// Allow SSE reader to catch all live execution events
	time.Sleep(100 * time.Millisecond)

	eventsMu.Lock()
	evCopy := append([]string{}, collectedEvents...)
	eventsMu.Unlock()

	// Verify required events were received:
	// connected, campaign.progress, task.status_changed, campaign.completed
	hasConnected := false
	hasProgress := false
	hasStatusChanged := false
	hasCompleted := false

	for _, ev := range evCopy {
		switch ev {
		case "connected":
			hasConnected = true
		case campaign.EventCampaignProgress:
			hasProgress = true
		case campaign.EventTaskStatusChanged:
			hasStatusChanged = true
		case campaign.EventCampaignCompleted:
			hasCompleted = true
		}
	}

	if !hasConnected {
		t.Errorf("missing 'connected' event in %v", evCopy)
	}
	if !hasProgress {
		t.Errorf("missing 'campaign.progress' event in %v", evCopy)
	}
	if !hasStatusChanged {
		t.Errorf("missing 'task.status_changed' event in %v", evCopy)
	}
	if !hasCompleted {
		t.Errorf("missing 'campaign.completed' event in %v", evCopy)
	}

	// Disconnect client and verify zero memory leak
	cancel()
	_ = sseResp.Body.Close()
	time.Sleep(30 * time.Millisecond)

	if count := s.CampaignAggregator().SubscriberCount(campaignID); count != 0 {
		t.Errorf("expected 0 subscribers after SSE disconnect, got %d", count)
	}
}

