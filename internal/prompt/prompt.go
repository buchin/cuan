// Package prompt manages automatic system prompt generation and guidance injection
// for Cuan Chat Agents (Scrum Master) and Runner Agents (Development Team).
package prompt

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed agents_guidance.md
var EmbeddedAgentsGuidance string

//go:embed scrum_rules.md
var EmbeddedScrumRules string

// GetAgentsGuidance returns the contents of AGENTS.md.
// It attempts to read AGENTS.md from the provided directory or common repository locations,
// falling back to the embedded copy if the file is not accessible on disk.
func GetAgentsGuidance(dirs ...string) string {
	candidates := make([]string, 0, len(dirs)+4)
	for _, d := range dirs {
		if strings.TrimSpace(d) != "" {
			candidates = append(candidates, filepath.Join(d, "AGENTS.md"))
		}
	}
	candidates = append(candidates,
		"AGENTS.md",
		filepath.Join(".", "AGENTS.md"),
		filepath.Join("..", "AGENTS.md"),
		filepath.Join("..", "..", "AGENTS.md"),
	)

	for _, path := range candidates {
		if data, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(data))) > 0 {
			return strings.TrimSpace(string(data))
		}
	}

	return strings.TrimSpace(EmbeddedAgentsGuidance)
}

// GetScrumRules returns the Agentic Scrum Rules (§1–§13) contract.
func GetScrumRules() string {
	return strings.TrimSpace(EmbeddedScrumRules)
}

// GetChatPromptAdditions formats the combined AGENTS.md guidance and Scrum Rules
// specifically structured for the Chat Agent (acting as Scrum Master).
func GetChatPromptAdditions(dirs ...string) string {
	var sb strings.Builder
	sb.WriteString("=== MANDATORY REPOSITORY GUIDANCE (AGENTS.md) ===\n")
	sb.WriteString(GetAgentsGuidance(dirs...))
	sb.WriteString("\n\n=== AGENTIC SCRUM RULES (§1–§13) ===\n")
	sb.WriteString(GetScrumRules())
	return sb.String()
}

// GetRunnerPromptAdditions formats the combined AGENTS.md guidance and Scrum Rules
// specifically structured for the Runner Agent (acting as Development Team).
func GetRunnerPromptAdditions(dirs ...string) string {
	var sb strings.Builder
	sb.WriteString("\n\n=== MANDATORY REPOSITORY GUIDANCE (AGENTS.md) ===\n")
	sb.WriteString(GetAgentsGuidance(dirs...))
	sb.WriteString("\n\n=== AGENTIC SCRUM RULES FOR DEVELOPMENT TEAM (§1–§13) ===\n")
	sb.WriteString(GetScrumRules())
	return sb.String()
}

// BuildRunnerPromptWithDoD creates the unified task execution prompt for cuan-runner or local runner,
// ensuring AGENTS.md, Definition of Done checklist, and task reporting requirements are explicitly injected.
func BuildRunnerPromptWithDoD(taskTitle, taskInstructions, agentPrompt string, dodChecklist []string, dirs ...string) string {
	var sb strings.Builder

	base := strings.TrimSpace(taskInstructions)
	if base == "" {
		base = strings.TrimSpace(taskTitle)
	}
	sb.WriteString(base)

	if len(dodChecklist) > 0 {
		sb.WriteString("\n\n=== MANDATORY DEFINITION OF DONE & ACCEPTANCE CRITERIA ===\n")
		sb.WriteString("You MUST systematically satisfy, test, and provide concrete verification evidence for every criterion below:\n")
		for _, item := range dodChecklist {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				sb.WriteString(fmt.Sprintf("- [ ] %s\n", trimmed))
			}
		}
		sb.WriteString("\n=== TASK COMPLETION & VERIFICATION REPORTING REQUIREMENT ===\n")
		sb.WriteString("Before concluding, you MUST state your verification evidence (exact test outputs, commit SHA, and verified behavior).\n")
		sb.WriteString("Map your verification evidence directly to the criteria above.\n")
		sb.WriteString("For investigation/audit tasks, provide factual findings, commit SHAs, or diagnostic outputs.\n")
		sb.WriteString("If using the cuan CLI in workspace, report completion using:\n")
		sb.WriteString("  \"$CUAN_TASK_EXECUTABLE\" task report --summary \"<concise summary of work done>\" --complete-note \"<detailed DoD verification evidence>\"\n")
		sb.WriteString("Or provide itemized criteria verification using repeatable flags:\n")
		sb.WriteString("  \"$CUAN_TASK_EXECUTABLE\" task report --summary \"<summary>\" --dod-item \"<original criterion>\" --dod-evidence \"<observed result>\"\n")
	}

	if agentPrompt != "" {
		sb.WriteString("\n\n")
		sb.WriteString(strings.TrimSpace(agentPrompt))
	}

	additions := GetRunnerPromptAdditions(dirs...)
	// Prevent duplicate injection if already present
	if !strings.Contains(sb.String(), "=== MANDATORY REPOSITORY GUIDANCE (AGENTS.md) ===") {
		sb.WriteString(additions)
	}

	return sb.String()
}

// BuildRunnerPrompt calls BuildRunnerPromptWithDoD with no explicit DoD checklist for backward compatibility.
func BuildRunnerPrompt(taskTitle, taskInstructions, agentPrompt string, dirs ...string) string {
	return BuildRunnerPromptWithDoD(taskTitle, taskInstructions, agentPrompt, nil, dirs...)
}

// BuildChatSystemPrompt combines base coordinator instructions with AGENTS.md and Scrum Rules.
func BuildChatSystemPrompt(baseInstruction, schema, coordinatorPolicy, agentName, contextData string, dirs ...string) string {
	var sb strings.Builder
	if strings.TrimSpace(baseInstruction) != "" {
		sb.WriteString(strings.TrimSpace(baseInstruction))
		sb.WriteString("\n\n")
	}

	if strings.TrimSpace(schema) != "" {
		sb.WriteString("Response schema:\n")
		sb.WriteString(strings.TrimSpace(schema))
		sb.WriteString("\n\n")
	}

	if strings.TrimSpace(coordinatorPolicy) != "" {
		sb.WriteString(strings.TrimSpace(coordinatorPolicy))
		sb.WriteString("\n\n")
	}

	chatGuidance := GetChatPromptAdditions(dirs...)
	sb.WriteString(chatGuidance)
	sb.WriteString("\n\n")

	if strings.TrimSpace(agentName) != "" {
		sb.WriteString("Your display name: ")
		sb.WriteString(strings.TrimSpace(agentName))
		sb.WriteString("\n\n")
	}

	if strings.TrimSpace(contextData) != "" {
		sb.WriteString("Context data:\n")
		sb.WriteString(strings.TrimSpace(contextData))
	}

	return sb.String()
}
