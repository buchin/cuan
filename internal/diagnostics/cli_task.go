package diagnostics

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RunTaskCLI handles the `cuan task` CLI subcommands when executed outside of an isolated runner capability session.
func RunTaskCLI(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		printTaskHelp(out)
		return 0
	}

	subcommand := strings.ToLower(strings.TrimSpace(args[0]))
	extraArgs := args[1:]

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	switch subcommand {
	case "delete":
		return runTaskDelete(ctx, extraArgs, out, errOut)
	case "logs":
		return runTaskLogs(ctx, extraArgs, out, errOut)
	case "cancel", "stop":
		return runTaskCancel(ctx, extraArgs, out, errOut)
	case "list", "ls":
		return runTaskList(ctx, extraArgs, out, errOut)
	case "get", "show", "inspect":
		return runTaskGet(ctx, extraArgs, out, errOut)
	case "update", "set":
		return runTaskUpdate(ctx, extraArgs, out, errOut)
	case "create", "draft", "new":
		return runTaskCreate(ctx, extraArgs, out, errOut)
	default:
		fmt.Fprintf(errOut, "Unknown task command: %s. Run 'cuan task help' for usage.\n", subcommand)
		return 2
	}
}

func runTaskDelete(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(errOut, "Usage: cuan task delete <task-id>")
		return 2
	}
	if err := taskSessionOwnerWriteError(); err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		fmt.Fprintln(errOut, "Configure the Cuan connection first.")
		return 1
	}
	response, err := client.doRequest(ctx, http.MethodDelete, "/api/v1/tasks/"+url.PathEscape(args[0]), nil)
	if err != nil {
		fmt.Fprintln(errOut, "Deletion was not confirmed. Inspect the task before retrying.")
		return 1
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		fmt.Fprintf(errOut, "Deletion was not confirmed (HTTP %d). Active tasks must be stopped first.\n", response.StatusCode)
		return 1
	}
	fmt.Fprintln(out, "Task deleted.")
	return 0
}

func printTaskHelp(out io.Writer) {
	fmt.Fprintln(out, `Cuan Task CLI — Inspect and manage tasks on the Cuan platform board

Usage:
  cuan task list [--project default] [--status doing] [--limit 20] [--json]
  cuan task get <task-id> [--json]
  cuan task logs <task-id> <attempt-id> [--after-id 0] [--limit 100] [--json]
  cuan task cancel <task-id>
  cuan task update <task-id> --status doing [--summary "Plan"] [--complete-note "Evidence"] [--json]
  cuan task create --title "Title" --instructions "Scope" [--priority normal] [--project default] [--json]

Subcommands:
  list      List tasks with optional project and status filters
  get       Inspect a single task and its execution attempts
  update    Update status, summary, and verification evidence
  create    Create a new task draft on the board
  delete    Permanently delete an inactive task (Owner only)

Examples:
  cuan task list --status doing
  cuan task get task_36ecce9be1f5
  cuan task update task_36ecce9be1f5 --status done --complete-note "Verified unit tests passed"
  cuan task create --title "Add OAuth2" --instructions "Integrate GitHub OAuth2 callback"`)
}

func runTaskList(ctx context.Context, args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("task list", flag.ContinueOnError)
	fs.SetOutput(errOut)
	projFlag := fs.String("project", "", "Filter by project ID or name")
	statusFlag := fs.String("status", "", "Filter by status (todo, in_progress, doing, review, done, blocked)")
	limitFlag := fs.Int("limit", 20, "Maximum tasks to retrieve")
	jsonFlag := fs.Bool("json", false, "Output in JSON format")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	res, err := ListTasks(ctx, *projFlag, *statusFlag, *limitFlag)
	if err != nil {
		fmt.Fprintf(errOut, "Error listing tasks: %v\n", err)
		return 1
	}

	if *jsonFlag {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		return 0
	}

	resMap, _ := res.(map[string]any)
	tasks, _ := resMap["tasks"].([]any)
	if tasks == nil {
		if ts, ok := resMap["tasks"].([]TaskSummary); ok {
			for _, t := range ts {
				tasks = append(tasks, t)
			}
		} else if rawList, ok := resMap["tasks"].([]map[string]any); ok {
			for _, t := range rawList {
				tasks = append(tasks, t)
			}
		}
	}

	if *limitFlag > 0 && len(tasks) > *limitFlag {
		tasks = tasks[:*limitFlag]
	}

	fmt.Fprintf(out, "Found %d task(s):\n\n", len(tasks))
	for i, item := range tasks {
		var id, title, status, priority, agent string
		if m, ok := item.(map[string]any); ok {
			id, _ = m["id"].(string)
			title, _ = m["title"].(string)
			status, _ = m["status"].(string)
			priority, _ = m["priority"].(string)
			agent, _ = m["assigned_agent"].(string)
		} else if ts, ok := item.(TaskSummary); ok {
			id = ts.ID
			title = ts.Title
			status = ts.Status
			priority = ts.Priority
		}
		if priority == "" {
			priority = "normal"
		}
		agentStr := ""
		if agent != "" {
			agentStr = fmt.Sprintf(" [%s]", agent)
		}
		fmt.Fprintf(out, "%2d. (%s|%s)%s %s\n    ID: %s\n", i+1, status, priority, agentStr, title, id)
	}
	return 0
}

func runTaskGet(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "Usage: cuan task get <task-id> [--json]")
		return 2
	}

	taskID := args[0]
	fs := flag.NewFlagSet("task get", flag.ContinueOnError)
	fs.SetOutput(errOut)
	jsonFlag := fs.Bool("json", false, "Output in JSON format")

	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	res, err := GetTask(ctx, taskID)
	if err != nil {
		fmt.Fprintf(errOut, "Error retrieving task: %v\n", err)
		return 1
	}

	if *jsonFlag {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		return 0
	}

	resMap, _ := res.(map[string]any)
	if resMap["error"] == "task_not_found" {
		fmt.Fprintf(errOut, "Task not found: %s\n", taskID)
		return 1
	}

	taskData, _ := resMap["task"].(map[string]any)
	if taskData == nil {
		if ts, ok := resMap["task"].(TaskSummary); ok {
			taskData = map[string]any{
				"id":            ts.ID,
				"title":         ts.Title,
				"status":        ts.Status,
				"priority":      ts.Priority,
				"instructions":  ts.Instructions,
				"summary":       ts.Summary,
				"complete_note": ts.CompleteNote,
				"dod_checklist": ts.DoDChecklist,
			}
		} else {
			taskData = resMap
		}
	}

	fmt.Fprintf(out, "Task %s:\n", taskID)
	fmt.Fprintf(out, "  Title:        %v\n", taskData["title"])
	fmt.Fprintf(out, "  Status:       %v\n", taskData["status"])
	fmt.Fprintf(out, "  Priority:     %v\n", taskData["priority"])
	if summary, _ := taskData["summary"].(string); summary != "" {
		fmt.Fprintf(out, "  Summary:      %s\n", summary)
	}
	if note, _ := taskData["complete_note"].(string); note != "" {
		fmt.Fprintf(out, "  CompleteNote: %s\n", note)
	}
	if inst, _ := taskData["instructions"].(string); inst != "" {
		fmt.Fprintf(out, "\nInstructions:\n%s\n", inst)
	}
	return 0
}

func runTaskUpdate(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "Usage: cuan task update <task-id> --status <status> [--summary <summary>] [--complete-note <evidence>] [--json]")
		return 2
	}

	taskID := args[0]
	fs := flag.NewFlagSet("task update", flag.ContinueOnError)
	fs.SetOutput(errOut)
	statusFlag := fs.String("status", "", "Target status (todo, in_progress, review, done, blocked, canceled)")
	summaryFlag := fs.String("summary", "", "Work summary or plan description")
	noteFlag := fs.String("complete-note", "", "Verification notes or blocker explanation")
	jsonFlag := fs.Bool("json", false, "Output in JSON format")

	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if *statusFlag == "" && *summaryFlag == "" && *noteFlag == "" {
		fmt.Fprintln(errOut, "At least one of --status, --summary, or --complete-note must be provided")
		return 2
	}

	res, err := UpdateTaskStatus(ctx, taskID, *statusFlag, *summaryFlag, *noteFlag)
	if err != nil {
		fmt.Fprintf(errOut, "Error updating task: %v\n", err)
		return 1
	}

	if *jsonFlag {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		return 0
	}

	fmt.Fprintf(out, "Task %s successfully updated.\n", taskID)
	if *statusFlag != "" {
		fmt.Fprintf(out, "  Status:        %s\n", *statusFlag)
	}
	if *summaryFlag != "" {
		fmt.Fprintf(out, "  Summary:       %s\n", *summaryFlag)
	}
	if *noteFlag != "" {
		fmt.Fprintf(out, "  Complete Note: %s\n", *noteFlag)
	}
	return 0
}

func runTaskCreate(ctx context.Context, args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("task create", flag.ContinueOnError)
	fs.SetOutput(errOut)
	titleFlag := fs.String("title", "", "Task title")
	instFlag := fs.String("instructions", "", "Task instructions / scope")
	projFlag := fs.String("project", "default", "Project ID or name")
	priorityFlag := fs.String("priority", "normal", "Priority (low, normal, high, urgent)")
	dodFlag := fs.String("dod", "", "Comma-separated Definition of Done checklist items")
	jsonFlag := fs.Bool("json", false, "Output in JSON format")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if *titleFlag == "" {
		fmt.Fprintln(errOut, "--title is required")
		return 2
	}
	if *instFlag == "" {
		*instFlag = *titleFlag
	}

	var dodList []string
	if *dodFlag != "" {
		for _, item := range strings.Split(*dodFlag, ",") {
			item = strings.TrimSpace(item)
			if item != "" {
				dodList = append(dodList, item)
			}
		}
	}

	res, err := CreateTaskDraft(ctx, *projFlag, *titleFlag, *instFlag, *priorityFlag, dodList)
	if err != nil {
		fmt.Fprintf(errOut, "Error creating task draft: %v\n", err)
		return 1
	}

	if *jsonFlag {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		return 0
	}

	resMap, _ := res.(map[string]any)
	fmt.Fprintln(out, "Task draft created successfully:")
	fmt.Fprintf(out, "  ID:       %v\n", resMap["id"])
	fmt.Fprintf(out, "  Title:    %v\n", resMap["title"])
	fmt.Fprintf(out, "  Project:  %v\n", resMap["project_id"])
	fmt.Fprintf(out, "  Status:   %v\n", resMap["status"])
	return 0
}

// Cancellation is an execution request; never report that the worker has already
// stopped until its durable attempt confirms the terminal state.
func runTaskCancel(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintln(errOut, "Usage: cuan task cancel <task-id>")
		return 2
	}
	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		fmt.Fprintln(errOut, "Configure the Cuan connection first.")
		return 1
	}
	response, err := client.doRequest(ctx, http.MethodPost, "/api/v1/tasks/"+url.PathEscape(args[0])+"/cancel", nil)
	if err != nil {
		fmt.Fprintln(errOut, "Could not confirm cancellation. Check task status before retrying.")
		return 1
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		fmt.Fprintf(errOut, "Cancellation was not confirmed (HTTP %d). Check task status.\n", response.StatusCode)
		return 1
	}
	fmt.Fprintln(out, "Cancellation requested. Check the task to confirm the worker stopped.")
	return 0
}
