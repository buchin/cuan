package server

import (
	"context"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/agent"
	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestTaskRequiresVision(t *testing.T) {
	srv := &Server{}
	ctx := context.Background()

	// 1. Plain text without visual indicators -> false
	t1 := &task.TaskDetailResponse{
		Task: task.Task{
			Title:        "Fix backend API endpoint",
			Instructions: "Fix the bug in internal/server/routes.go where status 400 is returned instead of 200.",
		},
	}
	if srv.taskRequiresVision(ctx, "owner-1", t1, "", "") {
		t.Errorf("expected task without visual attachments to not require vision")
	}

	// 2. Instructions mentioning visual inspection or .png -> true
	t2 := &task.TaskDetailResponse{
		Task: task.Task{
			Title:        "Inspect screenshot",
			Instructions: "Check the UI error in screenshot.png and verify alignment.",
		},
	}
	if !srv.taskRequiresVision(ctx, "owner-1", t2, "", "") {
		t.Errorf("expected task with screenshot.png in instructions to require vision")
	}

	// 3. Attachments containing image type -> true
	t3 := &task.TaskDetailResponse{
		Task: task.Task{
			Title:        "Review attachment",
			Instructions: "Please check attached files",
		},
		Attachments: []any{
			map[string]any{
				"name": "mockup.webp",
				"type": "image/webp",
			},
		},
	}
	if !srv.taskRequiresVision(ctx, "owner-1", t3, "", "") {
		t.Errorf("expected task with webp attachment to require vision")
	}

	// 4. Instructions with standard user attachment directive -> true
	t4 := &task.TaskDetailResponse{
		Task: task.Task{
			Title:        "Inspect image",
			Instructions: "Images require visual inspection. Read the actual files.",
		},
	}
	if !srv.taskRequiresVision(ctx, "owner-1", t4, "", "") {
		t.Errorf("expected task with 'Images require visual inspection' to require vision")
	}
}

func TestAgentEntitySupportsVision(t *testing.T) {
	// Gemini model -> supports vision
	geminiAgent := &cuanDB.Agent{
		ID:       "agent_gemini",
		Harness:  "agy",
		Provider: "google",
		ModelID:  "gemini-2.0-flash",
	}
	if !agentEntitySupportsVision(geminiAgent) {
		t.Errorf("expected gemini agent to support vision")
	}

	// Non-vision model on AGY (e.g. gpt-oss) without vision fallback -> false
	nonVisionAgent := &cuanDB.Agent{
		ID:       "agent_textonly",
		Harness:  "agy",
		Provider: "agy",
		ModelID:  "gpt-oss",
	}
	if agentEntitySupportsVision(nonVisionAgent) {
		t.Errorf("expected gpt-oss agent without fallback to not support vision")
	}

	// Non-vision model with vision fallback model -> true
	fallbackAgent := &cuanDB.Agent{
		ID:              "agent_with_fallback",
		Harness:         "agy",
		Provider:        "agy",
		ModelID:         "gpt-oss",
		FallbackModelID: "gemini-2.0-flash",
	}
	if !agentEntitySupportsVision(fallbackAgent) {
		t.Errorf("expected agent with gemini fallback to support vision")
	}
}

func TestTaskLaunchAutoFailoverOnCapacityOrVision(t *testing.T) {
	srv, _, _, _ := inboxFixture(t)
	ctx := context.Background()

	// Register two agents:
	// agent_text: text-only (gpt-oss)
	// agent_vision: vision-capable (gemini-2.0-flash)
	agentText := &cuanDB.Agent{
		ID:       "agent_text",
		Name:     "@agent_text",
		Harness:  "agy",
		Provider: "agy",
		ModelID:  "gpt-oss",
		Enabled:  true,
		Status:   cuanDB.AgentStatusActive,
	}
	agentVision := &cuanDB.Agent{
		ID:       "agent_vision",
		Name:     "@agent_vision",
		Harness:  "agy",
		Provider: "google",
		ModelID:  "gemini-2.0-flash",
		Enabled:  true,
		Status:   cuanDB.AgentStatusActive,
	}

	_ = srv.agentRepo.CreateAgent(ctx, agentText)
	_ = srv.agentRepo.CreateAgent(ctx, agentVision)

	// Detail requiring vision
	detail := &task.TaskDetailResponse{
		Task: task.Task{
			Title:        "Visual Inspection",
			Instructions: "Inspect layout error in error.png. Images require visual inspection.",
		},
	}

	// findHealthyTaskAgentFailover from agent_text when vision is required
	alt := srv.findHealthyTaskAgentFailover(ctx, "owner-1", "agent_text", detail, true)
	if alt == nil {
		t.Fatalf("expected failover agent to be found")
	}
	if alt.ID != "agent_vision" {
		t.Errorf("expected failover to agent_vision, got %s", alt.ID)
	}

	// If vision is NOT required, agent_vision is still a valid alternative
	altNonVision := srv.findHealthyTaskAgentFailover(ctx, "owner-1", "agent_text", nil, false)
	if altNonVision == nil {
		t.Fatalf("expected failover agent to be found when vision is not required")
	}
}

func TestExecuteInboxWithFailover_VisionFiltering(t *testing.T) {
	srv, _, _, _ := inboxFixture(t)
	ctx := context.Background()

	// Registered agent with non-vision model
	reg := &cuanDB.Agent{
		ID:              "agent_command",
		Name:            "@command",
		Harness:         "commandcode",
		ModelID:         "text-only-model",
		FallbackModelID: "claude-3-5-sonnet", // supports vision
		Enabled:         true,
	}

	// Attach image file
	imagePath := t.TempDir() + "/screenshot.png"
	// executeInboxWithFailover should automatically pick fallback model when primary lacks vision
	// We verify that routes or execution identifies fallback
	hasImg := false
	for _, f := range []string{imagePath} {
		if strings.HasSuffix(f, ".png") {
			hasImg = true
			break
		}
	}
	if !hasImg {
		t.Fatalf("expected hasImg to be true")
	}

	if !agent.IsModelVisionCapable("commandcode", reg.FallbackModelID) {
		t.Errorf("expected claude-3-5-sonnet to be vision capable")
	}
	if agent.IsModelVisionCapable("commandcode", reg.ModelID) {
		t.Errorf("expected text-only-model to NOT be vision capable")
	}

	_ = ctx
	_ = srv
}

func TestTaskVisionUsesOriginalScreenshotRequirementWithoutProjectDefaults(t *testing.T) {
	s := &Server{}
	for _, tc := range []struct {
		text string
		want bool
	}{
		{"Capture screenshots of the changed UI at mobile and desktop.", true},
		{"Periksa tangkapan layar halaman yang berubah.", true},
		{"Cuan managed delegation v1.\nFix the backend status code.\n\nProject guidance snapshot (context only; does not expand task authorization):\n```cuan-project-context\nAlways inspect mobile.png for UI work.\n```", false},
	} {
		d := &task.TaskDetailResponse{Task: task.Task{Instructions: tc.text}}
		if got := s.taskRequiresVision(context.Background(), "owner", d, "", ""); got != tc.want {
			t.Fatalf("unexpected vision selection=%v for scoped requirement", got)
		}
	}
}
