// Package prompt manages automatic system prompt generation and guidance injection
// for Cuan Chat Agents (Scrum Master) and Runner Agents (Development Team).
package prompt

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/dodeval"
)

//go:embed agents_guidance.md
var EmbeddedAgentsGuidance string

//go:embed scrum_rules.md
var EmbeddedScrumRules string

// GetAgentsGuidance returns the contents of AGENTS.md.
// Explicit repository roots never borrow the API host's rules. Without an
// explicit root, platform callers retain the common-path/embedded fallback.
func GetAgentsGuidance(dirs ...string) string {
	candidates := make([]string, 0, len(dirs)+4)
	explicitRoot := false
	for _, d := range dirs {
		if strings.TrimSpace(d) != "" {
			explicitRoot = true
			candidates = append(candidates, filepath.Join(d, "AGENTS.md"))
		}
	}
	if !explicitRoot {
		candidates = append(candidates,
			"AGENTS.md",
			filepath.Join(".", "AGENTS.md"),
			filepath.Join("..", "AGENTS.md"),
			filepath.Join("..", "..", "AGENTS.md"),
		)
	}

	for _, path := range candidates {
		if data, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(data))) > 0 {
			return strings.TrimSpace(string(data))
		}
	}
	if explicitRoot {
		return "Repository AGENTS.md is not available at the supplied root. Follow the saved project instructions and approved task scope; never substitute another repository's guidance. If required guidance cannot be read, report that context gap."
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
	sb.WriteString("\n\n" + AutonomousOwnership)
	return sb.String()
}

// GetReviewPromptAdditions keeps repository constraints and ownership for a
// supplied-result evaluation. Sprint planning and worker execution procedures
// are not repeated: review's explicit contract and saved scope govern this stage.
func GetReviewPromptAdditions(dirs ...string) string {
	return "=== MANDATORY REPOSITORY GUIDANCE (AGENTS.md) ===\n" + GetAgentsGuidance(dirs...) + "\n\n" + AutonomousOwnership
}

// GetRunnerPromptAdditions formats the combined AGENTS.md guidance and Scrum Rules
// specifically structured for the Runner Agent (acting as Development Team).
func GetRunnerPromptAdditions(dirs ...string) string {
	var sb strings.Builder
	sb.WriteString("\n\n=== MANDATORY REPOSITORY GUIDANCE (AGENTS.md) ===\n")
	sb.WriteString(GetAgentsGuidance(dirs...))
	sb.WriteString("\n\n=== AGENTIC SCRUM RULES FOR DEVELOPMENT TEAM (§1–§13) ===\n")
	sb.WriteString(GetScrumRules())
	sb.WriteString("\n\n" + AutonomousOwnership)
	return sb.String()
}

// BuildRunnerPromptWithDoD creates the unified task execution prompt for cuan-runner or local runner,
// ensuring AGENTS.md, Definition of Done checklist, and task reporting requirements are explicitly injected.
func BuildRunnerPromptWithDoD(taskTitle, taskInstructions, agentPrompt string, dodChecklist []string, dirs ...string) string {
	var sb strings.Builder

	artifactOnly := executionpolicy.StandaloneArtifactWork(taskInstructions)
	base := strings.TrimSpace(executionpolicy.NormalizeManagedInstructions(taskInstructions))
	if artifactOnly {
		base = strings.TrimSpace(project.TaskScopeInstructions(taskInstructions))
		goal, guidance, err := project.TaskReviewGuidance(taskInstructions)
		if err != nil {
			base += "\nSaved project guidance could not be verified. Preserve original scope and report this context gap; do not substitute current settings."
		} else {
			if goal != "" {
				base += "\n\nSaved project goal (context only):\n" + goal
			}
			if guidance != "" {
				base += "\n\nSaved project instructions (context only):\n" + guidance
			}
		}
	}
	if base == "" {
		base = strings.TrimSpace(taskTitle)
	}
	sb.WriteString(base)
	if artifactOnly {
		sb.WriteString("\n\n=== STANDALONE FILE DELIVERY ===\nThis task uses a private output workspace, not a project repository. Create only the requested deliverable, inspect its actual filename and content against the original criteria, and attach it with the task reporting capability. Do not clone repositories, initialize Git, inspect unrelated project source, configure services, install a software stack or deploy. A file path or a promise is not delivery. Use --artifact for the actual workspace file, or --artifact session:PATH for a file under CUAN_TASK_ARTIFACTS_DIR.\n")
	}

	if len(dodChecklist) == 0 {
		dodChecklist = ApprovedScopeRequirements(taskInstructions)
	}
	if len(dodChecklist) > 0 {
		sb.WriteString("\n\n=== MANDATORY DEFINITION OF DONE & ACCEPTANCE CRITERIA ===\n")
		sb.WriteString("You MUST systematically satisfy, test, and provide concrete verification evidence for every criterion below:\n")
		for _, item := range dodChecklist {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				sb.WriteString(fmt.Sprintf("- [ ] %s\n", trimmed))
			}
		}
	}
	if !strings.Contains(sb.String(), "=== TASK COMPLETION & VERIFICATION REPORTING REQUIREMENT ===") {
		sb.WriteString("\n\n=== TASK COMPLETION & VERIFICATION REPORTING REQUIREMENT ===\n")
		sb.WriteString("Before concluding, you MUST state your verification evidence (exact test outputs, commit SHA, and verified behavior).\n")
		sb.WriteString("For each original criterion, name the exact check or artifact and the observed feature/state it proves. Leave criteria without relevant evidence incomplete; passing generic checks or unrelated screenshots do not prove them. An empty checklist does not waive verification or reporting for the original requirements.\n")
		sb.WriteString("For investigation/audit tasks, provide factual findings, commit SHAs, or diagnostic outputs.\n")
		if artifactOnly {
			sb.WriteString("Use the supplied original task and acceptance snapshot. Read \"$CUAN_TASK_EXECUTABLE\" task get when information is missing or after an uncertain/rejected report; do not reread known instructions merely as a startup ritual. ")
		} else {
			sb.WriteString("Use the supplied original criteria or \"$CUAN_TASK_EXECUTABLE\" task report-template to prepare report JSON with original IDs/text/order and empty evidence. Read task get for missing task context or an uncertain/rejected report. ")
		}
		sb.WriteString("If the saved task has a DoD checklist, review/done reports MUST include every original item with unchanged ID, text and order, plus current factual evidence. Use --report-file JSON to preserve IDs. Mark unmet items completed=false and describe the gap; never fabricate evidence. A rejected report leaves this attempt active: correct it and resubmit.\nFor tasks without a saved checklist, report completion using:\n")
		sb.WriteString("  \"$CUAN_TASK_EXECUTABLE\" task report --state review --summary \"<concise summary of work done>\" --complete-note \"<detailed DoD verification evidence>\"\n")
		sb.WriteString("For criteria without IDs, itemized verification can use repeatable flags (include every saved criterion):\n")
		sb.WriteString("  \"$CUAN_TASK_EXECUTABLE\" task report --state review --summary \"<summary>\" --dod-item \"<original criterion>\" --dod-evidence \"<observed result>\"\n")
		sb.WriteString("Final results require an explicit state: review for result review, blocked for a blocker, or done for a completed implementation report. Worker completion requests independent review; it does not accept the task. A metadata update or progress checkpoint does not finish the session.\n")
		sb.WriteString("Save a factual checkpoint before broad investigation, before slow or risky steps, and at least every five minutes during substantial work:\n")
		sb.WriteString("  \"$CUAN_TASK_EXECUTABLE\" task report --state progress --progress \"<observed findings, checks, saved files, uncertainty and next concrete step>\"\n")
		sb.WriteString("A progress report is not completion. Continue the approved task afterward. Keep checkpoints concise and do not report guesses as findings.\n")
	}

	if agentPrompt != "" {
		sb.WriteString("\n\n")
		sb.WriteString(strings.TrimSpace(agentPrompt))
	}

	if artifactOnly {
		// The output workspace has no repository. Do not borrow the server's
		// ambient AGENTS.md or coding/sprint procedures for an unrelated file.
		sb.WriteString("\n\n" + AutonomousOwnership)
		return sb.String()
	}
	additions := GetRunnerPromptAdditions(dirs...)
	// Prevent duplicate injection if already present
	if !strings.Contains(sb.String(), "=== MANDATORY REPOSITORY GUIDANCE (AGENTS.md) ===") {
		sb.WriteString(additions)
	}

	return sb.String()
}

// ExecutionBudget states the runner's actual deadline, not a new model-selected
// time limit or permission to skip required verification.
func ExecutionBudget(deadline, now time.Time) string {
	remaining := deadline.Sub(now)
	if remaining < 0 {
		remaining = 0
	}
	return fmt.Sprintf("\n\n=== EXECUTION TIME BUDGET ===\nExecution deadline: %s. Remaining at dispatch: %s. This is the runner's existing deadline; do not extend it or treat it as task acceptance.\nFor an investigation, establish a reproducible failure or a specific falsifiable hypothesis early. By halfway through the remaining time, save a checkpoint with concrete evidence and the next decisive check. If broad searches add no evidence, narrow or change the hypothesis instead of repeatedly scanning the repository. Preserve existing work and reconcile prior effects.\nReserve the final part of the budget for targeted verification and reporting. Before the deadline, save findings, changed files, actual checks and the exact remaining gap through the task capability. Never replace missing evidence with a completion claim, expand scope, or ask the user to operate the tools.\n", deadline.UTC().Format(time.RFC3339), remaining.Round(time.Second))
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

// ApprovedScopeRequirements reads only the native delegation description. It
// never parses user attachment data, transport metadata, project defaults or
// reviewer feedback into new criteria, and never changes stored task scope.
func ApprovedScopeRequirements(instructions string) []string {
	const prefix = "Cuan managed delegation v1.\n"
	if !strings.HasPrefix(instructions, prefix) {
		return nil
	}
	body := strings.TrimPrefix(instructions, prefix)
	end := strings.Index(body, "\n\nApproval message: ")
	if end < 0 {
		return nil
	}
	body = body[:end]
	if strings.HasPrefix(body, "Approved scope digest: ") {
		if line := strings.IndexByte(body, '\n'); line >= 0 {
			body = body[line+1:]
		} else {
			return nil
		}
	}
	var result []string
	for _, requirement := range dodeval.ParseRequirements(body) {
		if text := strings.TrimSpace(requirement.Text); text != "" {
			result = append(result, text)
		}
		if len(result) == 25 {
			break
		}
	}
	return result
}
