package creator

import (
	"context"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
)

func TestDetectPipeline(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		description string
		tags        []string
		projectType string
		expected    string
	}{
		{
			name:        "Blog Blueprint by projectType",
			title:       "Custom Task",
			description: "Some task description",
			tags:        []string{"misc"},
			projectType: "blog",
			expected:    PipelineBlog,
		},
		{
			name:        "YouTube Blueprint by projectType alias",
			title:       "Script task",
			description: "",
			tags:        nil,
			projectType: "youtube_channel",
			expected:    PipelineYouTube,
		},
		{
			name:        "Olshop Blueprint by tags",
			title:       "Funnel Step 1",
			description: "Draft sales page copy",
			tags:        []string{"olshop", "conversion"},
			projectType: "",
			expected:    PipelineOlshop,
		},
		{
			name:        "Affiliate Blueprint by title",
			title:       "Product comparison matrix",
			description: "Compare 4 items",
			tags:        nil,
			projectType: "",
			expected:    PipelineAffiliate,
		},
		{
			name:        "General Creator Task",
			title:       "Prepare meeting notes",
			description: "Sync with marketing contractor",
			tags:        []string{"admin"},
			projectType: "custom",
			expected:    PipelineGeneral,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectPipeline(tt.title, tt.description, tt.tags, tt.projectType)
			if got != tt.expected {
				t.Errorf("DetectPipeline() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestBuildCreatorPrompt(t *testing.T) {
	task := &db.ProjectTask{
		ID:          "task-123",
		Title:       "10-min scriptwriting",
		Description: "Develop full script with visual cues",
		Priority:    "urgent",
		Tags:        []string{"youtube", "production"},
		DoDChecklist: []db.DoDItem{
			{Text: "Full voiceover script exceeding 1,200 words", Completed: false},
			{Text: "Visual B-roll and cues in brackets", Completed: false},
		},
	}

	agent := &db.Agent{
		Name:         "YouTube Scriptwriter",
		Role:         "Audience Retention Expert",
		SystemPrompt: "You are an elite YouTube scriptwriter.",
	}

	ctx := context.Background()
	systemPrompt, userPrompt := BuildCreatorPrompt(ctx, task, agent, "Focus on gaming niche", "youtube")

	if !strings.Contains(systemPrompt, "elite YouTube scriptwriter") {
		t.Errorf("expected systemPrompt to contain agent system prompt")
	}
	if !strings.Contains(systemPrompt, "CORE EXECUTION DIRECTIVES") {
		t.Errorf("expected systemPrompt to contain core execution directives")
	}

	if !strings.Contains(userPrompt, "10-MIN SCRIPTWRITING") {
		t.Errorf("expected userPrompt to contain task title")
	}
	if !strings.Contains(userPrompt, "Pipeline Category:** YOUTUBE") {
		t.Errorf("expected userPrompt to contain pipeline category YOUTUBE, got: %s", userPrompt)
	}
	if !strings.Contains(userPrompt, "Full voiceover script exceeding 1,200 words") {
		t.Errorf("expected userPrompt to contain DoD checklist item")
	}
	if !strings.Contains(userPrompt, "Focus on gaming niche") {
		t.Errorf("expected userPrompt to contain extra user instructions")
	}
}
