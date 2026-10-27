package creator

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
)

type mockEventBroadcaster struct {
	mu     sync.Mutex
	events []struct {
		roomID    string
		eventType string
		data      interface{}
	}
}

func (m *mockEventBroadcaster) Broadcast(roomID string, eventType string, data interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, struct {
		roomID    string
		eventType string
		data      interface{}
	}{roomID, eventType, data})
}

func setupTestExecutionEnvironment(t *testing.T) (*TaskExecutionEngine, db.ProjectStudioRepository, db.AgentRepository, *mockEventBroadcaster) {
	t.Helper()

	studioRepo := db.NewMemoryProjectStudioRepository()
	agentRepo := db.NewMemoryAgentRepository()
	broadcaster := &mockEventBroadcaster{}

	engine := NewTaskExecutionEngine(
		WithExecutionStudioRepo(studioRepo),
		WithExecutionAgentRepo(agentRepo),
		WithEventBroadcaster(broadcaster),
	)

	return engine, studioRepo, agentRepo, broadcaster
}

func registerMockGeminiConnection(t *testing.T, agentRepo db.AgentRepository, apiKey string) string {
	t.Helper()
	enc, err := agent.EncryptCredential(apiKey)
	if err != nil {
		t.Fatalf("failed to encrypt test credential: %v", err)
	}

	conn := &db.AgentConnection{
		ID:                   db.GenerateUUID(),
		UserID:               "default",
		Provider:             "google",
		CredentialsEncrypted: enc,
		PlanHint:             "Google AI Studio (BYOK)",
		Status:               db.ConnectionStatusActive,
	}
	if err := agentRepo.CreateConnection(context.Background(), conn); err != nil {
		t.Fatalf("failed to create agent connection: %v", err)
	}
	return conn.ID
}

func TestTaskExecution_MissingGeminiKey(t *testing.T) {
	engine, studioRepo, _, _ := setupTestExecutionEnvironment(t)
	ctx := context.Background()

	// Create room and task
	roomID := db.GenerateUUID()
	_ = studioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: roomID, ProjectID: "proj-1", Title: "SEO Research"})

	task := &db.ProjectTask{
		ID:           db.GenerateUUID(),
		ProjectID:    "proj-1",
		RoomID:       &roomID,
		Title:        "Keyword research",
		Description:  "Identify 5+ LSI keywords",
		Status:       "backlog",
		Priority:     "high",
		DoDChecklist: []db.DoDItem{{Text: "5+ LSI keywords identified", Completed: false}},
	}
	_ = studioRepo.CreateTask(ctx, task)

	// Execute without registered Gemini key
	resp, err := engine.ExecuteTask(ctx, ExecuteTaskRequest{TaskID: task.ID})
	if err == nil {
		t.Fatalf("expected error on missing Gemini key, got nil")
	}
	if !errors.Is(err, ErrMissingGeminiKey) {
		t.Errorf("expected ErrMissingGeminiKey, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Agent Hub") {
		t.Errorf("expected error message to guide user to Agent Hub, got: %s", err.Error())
	}
	if resp != nil {
		t.Errorf("expected nil response on failure, got %+v", resp)
	}
}

func TestTaskExecution_BlogKeywordResearch_Success(t *testing.T) {
	engine, studioRepo, agentRepo, broadcaster := setupTestExecutionEnvironment(t)
	ctx := context.Background()

	// 1. Register Gemini Key in Vault
	registerMockGeminiConnection(t, agentRepo, "mock-gemini-pro-key-vault-test")

	// 2. Create Agent Persona
	agentPersona := &db.Agent{
		ID:           db.GenerateUUID(),
		Name:         "SEO Specialist",
		Role:         "Search Engine Optimization Specialist",
		SystemPrompt: "You are an expert SEO Specialist.",
		ModelID:      "gemini-1.5-pro",
		Provider:     "google",
		Status:       db.AgentStatusActive,
	}
	_ = agentRepo.CreateAgent(ctx, agentPersona)

	// 3. Create Project Room & Task
	roomID := db.GenerateUUID()
	_ = studioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: roomID, ProjectID: "proj-blog", Title: "SEO Research"})

	taskID := db.GenerateUUID()
	task := &db.ProjectTask{
		ID:          taskID,
		ProjectID:   "proj-blog",
		RoomID:      &roomID,
		Title:       "Keyword research",
		Description: "Perform keyword research and SERP analysis for target topic.",
		Status:      "backlog",
		Priority:    "high",
		AssigneeID:  &agentPersona.ID,
		Tags:        []string{"seo", "research", "keywords"},
		DoDChecklist: []db.DoDItem{
			{ID: db.GenerateUUID(), Text: "Identified primary keyword and 5+ secondary LSI keywords", Completed: false},
			{ID: db.GenerateUUID(), Text: "Analyzed top 3 SERP competitor headings", Completed: false},
		},
	}
	_ = studioRepo.CreateTask(ctx, task)

	// 4. Execute Task via Engine
	resp, err := engine.ExecuteTask(ctx, ExecuteTaskRequest{
		TaskID:       taskID,
		Instructions: "Prioritize low KD keywords under 30",
	})
	if err != nil {
		t.Fatalf("unexpected error during task execution: %v", err)
	}

	// 5. Verify Response Structure
	if resp.Status != "success" || resp.ExecutionStatus != "completed" {
		t.Errorf("expected status 'success' and execution_status 'completed', got: %s / %s", resp.Status, resp.ExecutionStatus)
	}
	if resp.DoDCompletedCount != 2 || resp.DoDTotalCount != 2 {
		t.Errorf("expected 2/2 DoD items completed, got %d/%d", resp.DoDCompletedCount, resp.DoDTotalCount)
	}
	if resp.Agent == nil || resp.Agent.Name != "SEO Specialist" {
		t.Errorf("expected agent persona SEO Specialist in response")
	}
	if resp.RoomMessageID == "" {
		t.Errorf("expected RoomMessageID to be returned")
	}

	// 6. Verify Content Deliverable Quality
	if !strings.Contains(resp.AgentOutput, "SEO Keyword Research") {
		t.Errorf("expected agent output to contain 'SEO Keyword Research', got: %s", resp.AgentOutput)
	}
	if !strings.Contains(resp.AgentOutput, "Primary Target Keyword") {
		t.Errorf("expected primary target keyword in output")
	}

	// 7. Verify Persistence in Database
	savedTask, err := studioRepo.GetTaskByID(ctx, taskID)
	if err != nil {
		t.Fatalf("failed to retrieve task from repo: %v", err)
	}
	if savedTask.ExecutionStatus != "completed" {
		t.Errorf("expected saved execution_status 'completed', got: %s", savedTask.ExecutionStatus)
	}
	if savedTask.AgentOutput != resp.AgentOutput {
		t.Errorf("persisted agent_output does not match response")
	}
	if savedTask.Status != "review" {
		t.Errorf("expected task status to move to 'review', got: %s", savedTask.Status)
	}
	for i, dod := range savedTask.DoDChecklist {
		if !dod.Completed {
			t.Errorf("DoD item %d (%s) expected to be completed", i, dod.Text)
		}
	}

	// 8. Verify Discussion Room Message
	messages, err := studioRepo.ListMessagesByRoomID(ctx, roomID)
	if err != nil || len(messages) == 0 {
		t.Fatalf("expected discussion room message to be posted, got count=%d, err=%v", len(messages), err)
	}
	lastMsg := messages[len(messages)-1]
	if lastMsg.AuthorType != "agent" || lastMsg.AuthorName != "SEO Specialist" {
		t.Errorf("expected message from agent SEO Specialist, got authorType=%s, name=%s", lastMsg.AuthorType, lastMsg.AuthorName)
	}
	if !strings.Contains(lastMsg.Content, "Task Execution Completed") {
		t.Errorf("expected message content to confirm task execution, got: %s", lastMsg.Content)
	}

	// 9. Verify Event Broadcasts
	broadcaster.mu.Lock()
	defer broadcaster.mu.Unlock()
	if len(broadcaster.events) == 0 {
		t.Errorf("expected events to be broadcasted to room event broker")
	}
}

func TestTaskExecution_YouTubeScriptwriting_Success(t *testing.T) {
	engine, studioRepo, agentRepo, _ := setupTestExecutionEnvironment(t)
	ctx := context.Background()

	registerMockGeminiConnection(t, agentRepo, "mock-gemini-key")

	agentPersona := &db.Agent{
		ID:           db.GenerateUUID(),
		Name:         "YouTube Scriptwriter",
		Role:         "Audience Retention Expert",
		SystemPrompt: "You are an elite YouTube scriptwriter.",
	}
	_ = agentRepo.CreateAgent(ctx, agentPersona)

	roomID := db.GenerateUUID()
	_ = studioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: roomID, ProjectID: "proj-yt", Title: "Channel Production"})

	taskID := db.GenerateUUID()
	task := &db.ProjectTask{
		ID:          taskID,
		ProjectID:   "proj-yt",
		RoomID:      &roomID,
		Title:       "10-min scriptwriting",
		Description: "Develop full script with visual cues and mid-roll hook.",
		Status:      "doing",
		Priority:    "urgent",
		AssigneeID:  &agentPersona.ID,
		Tags:        []string{"youtube", "script", "production"},
		DoDChecklist: []db.DoDItem{
			{Text: "Full voiceover script exceeding 1,200 words", Completed: false},
			{Text: "Visual B-roll and cues in brackets", Completed: false},
			{Text: "Mid-roll engagement hook placed at 4-minute mark", Completed: false},
		},
	}
	_ = studioRepo.CreateTask(ctx, task)

	resp, err := engine.ExecuteTask(ctx, ExecuteTaskRequest{TaskID: taskID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.ExecutionStatus != "completed" {
		t.Errorf("expected execution_status 'completed', got: %s", resp.ExecutionStatus)
	}
	if !strings.Contains(resp.AgentOutput, "YouTube Full 10-Minute Video Script") {
		t.Errorf("expected YouTube script deliverable, got: %s", resp.AgentOutput)
	}
	if !strings.Contains(resp.AgentOutput, "[VISUAL:") {
		t.Errorf("expected visual bracketed cues in script output")
	}

	saved, _ := studioRepo.GetTaskByID(ctx, taskID)
	for _, item := range saved.DoDChecklist {
		if !item.Completed {
			t.Errorf("expected DoD item %q to be completed", item.Text)
		}
	}
}

func TestTaskExecution_OlshopSalesPage_Success(t *testing.T) {
	engine, studioRepo, agentRepo, _ := setupTestExecutionEnvironment(t)
	ctx := context.Background()

	registerMockGeminiConnection(t, agentRepo, "mock-gemini-key")

	roomID := db.GenerateUUID()
	_ = studioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: roomID, ProjectID: "proj-shop", Title: "Sales Funnel"})

	taskID := db.GenerateUUID()
	task := &db.ProjectTask{
		ID:          taskID,
		ProjectID:   "proj-shop",
		RoomID:      &roomID,
		Title:       "Sales page copy",
		Description: "Write sales page using PAS framework.",
		Status:      "backlog",
		Priority:    "urgent",
		Tags:        []string{"copywriting", "sales-page", "conversion"},
		DoDChecklist: []db.DoDItem{
			{Text: "Attention-grabbing headline and benefit subheadline", Completed: false},
			{Text: "Pain point agitation", Completed: false},
			{Text: "Risk reversal guarantee", Completed: false},
		},
	}
	_ = studioRepo.CreateTask(ctx, task)

	resp, err := engine.ExecuteTask(ctx, ExecuteTaskRequest{TaskID: taskID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(resp.AgentOutput, "Problem - Agitate - Solution") {
		t.Errorf("expected PAS framework in output, got: %s", resp.AgentOutput)
	}
	if !strings.Contains(resp.AgentOutput, "Risk-Free Guarantee") {
		t.Errorf("expected guarantee section in output")
	}
}

func TestTaskExecution_AffiliateComparison_Success(t *testing.T) {
	engine, studioRepo, agentRepo, _ := setupTestExecutionEnvironment(t)
	ctx := context.Background()

	registerMockGeminiConnection(t, agentRepo, "mock-gemini-key")

	roomID := db.GenerateUUID()
	_ = studioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: roomID, ProjectID: "proj-aff", Title: "Niche Affiliate"})

	taskID := db.GenerateUUID()
	task := &db.ProjectTask{
		ID:          taskID,
		ProjectID:   "proj-aff",
		RoomID:      &roomID,
		Title:       "Product comparison matrix",
		Description: "Side-by-side comparison matrix with ratings.",
		Status:      "backlog",
		Priority:    "high",
		Tags:        []string{"affiliate", "comparison", "matrix"},
		DoDChecklist: []db.DoDItem{
			{Text: "Selected 3-5 competing products", Completed: false},
			{Text: "Markdown comparison matrix with winner badge", Completed: false},
		},
	}
	_ = studioRepo.CreateTask(ctx, task)

	resp, err := engine.ExecuteTask(ctx, ExecuteTaskRequest{TaskID: taskID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(resp.AgentOutput, "Comparison Matrix") {
		t.Errorf("expected comparison matrix in output, got: %s", resp.AgentOutput)
	}
	if !strings.Contains(resp.AgentOutput, "BEST OVERALL") {
		t.Errorf("expected winner badge in output")
	}
}

type failingGenerator struct{}

func (f *failingGenerator) GenerateContent(ctx context.Context, apiKey string, modelID string, systemInstruction string, userPrompt string) (string, error) {
	return "", errors.New("simulated LLM provider 500 internal server error")
}

func TestTaskExecution_FailureHandling(t *testing.T) {
	engine, studioRepo, agentRepo, _ := setupTestExecutionEnvironment(t)
	engine.contentGenerator = &failingGenerator{}
	ctx := context.Background()

	registerMockGeminiConnection(t, agentRepo, "mock-gemini-key")

	taskID := db.GenerateUUID()
	task := &db.ProjectTask{
		ID:           taskID,
		ProjectID:    "proj-err",
		Title:        "Failing Task",
		Description:  "This task will fail during generation",
		Status:       "backlog",
		DoDChecklist: []db.DoDItem{{Text: "DoD item", Completed: false}},
	}
	_ = studioRepo.CreateTask(ctx, task)

	resp, err := engine.ExecuteTask(ctx, ExecuteTaskRequest{TaskID: taskID})
	if err == nil {
		t.Fatalf("expected error from failing generator, got nil")
	}
	if !errors.Is(err, ErrAgentExecutionFailed) {
		t.Errorf("expected ErrAgentExecutionFailed, got: %v", err)
	}
	if resp != nil {
		t.Errorf("expected nil response on error, got: %+v", resp)
	}

	// Verify task execution status was updated to failed
	saved, _ := studioRepo.GetTaskByID(ctx, taskID)
	if saved.ExecutionStatus != "failed" {
		t.Errorf("expected execution_status 'failed', got: %s", saved.ExecutionStatus)
	}
	if saved.DoDChecklist[0].Completed {
		t.Errorf("DoD checklist item should remain incomplete on failure")
	}
}

func TestTaskExecution_ZeroCredentialLeakage(t *testing.T) {
	engine, studioRepo, agentRepo, _ := setupTestExecutionEnvironment(t)
	ctx := context.Background()

	secretKey := "mock-AIzaSySecretApiKeyDoNotLeakEver123456"
	registerMockGeminiConnection(t, agentRepo, secretKey)

	taskID := db.GenerateUUID()
	task := &db.ProjectTask{
		ID:          taskID,
		ProjectID:   "proj-sec",
		Title:       "Security Task",
		Description: "Check for zero plaintext leakage",
		Status:      "backlog",
	}
	_ = studioRepo.CreateTask(ctx, task)

	resp, err := engine.ExecuteTask(ctx, ExecuteTaskRequest{TaskID: taskID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify secret API key is NOT present anywhere in output or task
	if strings.Contains(resp.AgentOutput, secretKey) {
		t.Fatalf("CRITICAL SECURITY VIOLATION: plaintext API key leaked into agent output!")
	}
	if strings.Contains(resp.Task.AgentOutput, secretKey) {
		t.Fatalf("CRITICAL SECURITY VIOLATION: plaintext API key leaked into task agent output!")
	}

	saved, _ := studioRepo.GetTaskByID(ctx, taskID)
	if strings.Contains(saved.AgentOutput, secretKey) {
		t.Fatalf("CRITICAL SECURITY VIOLATION: plaintext API key persisted into DB!")
	}
}

func TestTaskExecution_Concurrency(t *testing.T) {
	engine, studioRepo, agentRepo, _ := setupTestExecutionEnvironment(t)
	ctx := context.Background()

	registerMockGeminiConnection(t, agentRepo, "mock-gemini-pro-concurrent-key")

	roomID := db.GenerateUUID()
	_ = studioRepo.CreateRoom(ctx, &db.ProjectRoom{ID: roomID, ProjectID: "proj-conc", Title: "Concurrent Room"})

	const numTasks = 10
	taskIDs := make([]string, numTasks)
	for i := 0; i < numTasks; i++ {
		tID := db.GenerateUUID()
		task := &db.ProjectTask{
			ID:          tID,
			ProjectID:   "proj-conc",
			RoomID:      &roomID,
			Title:       "Content drafting",
			Description: "Concurrent drafting task",
			Status:      "backlog",
			DoDChecklist: []db.DoDItem{
				{Text: "Engaging introduction", Completed: false},
				{Text: "Comprehensive body", Completed: false},
			},
		}
		_ = studioRepo.CreateTask(ctx, task)
		taskIDs[i] = tID
	}

	var wg sync.WaitGroup
	errCh := make(chan error, numTasks)

	for _, id := range taskIDs {
		wg.Add(1)
		go func(targetID string) {
			defer wg.Done()
			_, err := engine.ExecuteTask(ctx, ExecuteTaskRequest{TaskID: targetID})
			if err != nil {
				errCh <- err
			}
		}(id)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent task execution error: %v", err)
	}

	for _, id := range taskIDs {
		saved, err := studioRepo.GetTaskByID(ctx, id)
		if err != nil {
			t.Errorf("failed to get task %s: %v", id, err)
			continue
		}
		if saved.ExecutionStatus != "completed" {
			t.Errorf("expected execution_status 'completed' for task %s, got: %s", id, saved.ExecutionStatus)
		}
		if saved.AgentOutput == "" {
			t.Errorf("expected non-empty agent_output for task %s", id)
		}
		for _, dod := range saved.DoDChecklist {
			if !dod.Completed {
				t.Errorf("expected DoD item %q to be completed in task %s", dod.Text, id)
			}
		}
	}
}

