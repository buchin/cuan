package diagnostics

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

// RunScheduleCLI handles the `cuan schedule` CLI subcommands.
func RunScheduleCLI(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		printScheduleHelp(out)
		return 0
	}

	subcommand := strings.ToLower(strings.TrimSpace(args[0]))
	extraArgs := args[1:]

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	switch subcommand {
	case "list", "ls":
		return runScheduleList(ctx, extraArgs, out, errOut)
	case "create", "new", "add":
		return runScheduleCreate(ctx, extraArgs, out, errOut)
	case "update", "edit", "set":
		return runScheduleUpdate(ctx, extraArgs, out, errOut)
	case "delete", "rm":
		return runScheduleDelete(ctx, extraArgs, out, errOut)
	case "toggle":
		return runScheduleToggle(ctx, extraArgs, out, errOut)
	case "trigger", "run":
		return runScheduleTrigger(ctx, extraArgs, out, errOut)
	default:
		fmt.Fprintf(errOut, "Unknown schedule subcommand: %s. Run 'cuan schedule help' for usage.\n", subcommand)
		return 2
	}
}

func printScheduleHelp(out io.Writer) {
	fmt.Fprintln(out, `Cuan Schedule CLI — Manage recurring routines and automations

Usage:
  cuan schedule list [--json]
  cuan schedule create --title "..." --prompt "..." [options] [--json]
  cuan schedule update <id> [options] [--json]
  cuan schedule delete <id>
  cuan schedule toggle <id> [--enable|--disable]
  cuan schedule trigger <id>

Subcommands:
  list      List all scheduled routines
  create    Register a new recurring routine
  update    Update title, prompt, timing, or project of an existing routine
  delete    Remove a scheduled routine by ID
  toggle    Enable or disable a scheduled routine
  trigger   Trigger an immediate run of a scheduled routine

Options for create / update:
  --title     Routine title
  --prompt    Instruction prompt for agent execution
  --type      Schedule type: daily, weekly, interval (default: daily)
  --time      Time of day in HH:MM format (default: 08:00)
  --day       Day of week: 0=Sunday..6=Saturday (default: 1)
  --interval  Interval in minutes when type is interval (default: 60)
  --project   Target project ID or name
  --json      Output machine-readable JSON`)
}

func runScheduleList(ctx context.Context, args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("schedule list", flag.ContinueOnError)
	fs.SetOutput(errOut)
	jsonOutput := fs.Bool("json", false, "Output JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		fmt.Fprintln(errOut, "Error: Cuan connection not configured (~/.cuan/config.json).")
		return 1
	}

	tasks, err := client.ListScheduledTasks(ctx)
	if err != nil {
		fmt.Fprintf(errOut, "Error fetching scheduled tasks: %v\n", err)
		return 1
	}

	if *jsonOutput {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(tasks)
		return 0
	}

	if len(tasks) == 0 {
		fmt.Fprintln(out, "No scheduled routines found.")
		return 0
	}

	fmt.Fprintf(out, "Found %d scheduled routine(s):\n\n", len(tasks))
	for i, t := range tasks {
		id, _ := t["id"].(string)
		title, _ := t["title"].(string)
		sType, _ := t["schedule_type"].(string)
		tOfDay, _ := t["time_of_day"].(string)
		enabled, _ := t["enabled"].(bool)
		status, _ := t["last_status"].(string)
		projectID, _ := t["project_id"].(string)
		nextRun, _ := t["next_run_at"].(string)

		timing := sType
		if tOfDay != "" {
			timing += " @ " + tOfDay
		}

		state := "disabled"
		if enabled {
			state = "enabled"
		}
		if status == "" {
			status = "idle"
		}

		fmt.Fprintf(out, " %d. [%s] (%s|%s) %s\n", i+1, timing, state, status, title)
		fmt.Fprintf(out, "    ID: %s", id)
		if projectID != "" {
			fmt.Fprintf(out, " | Project: %s", projectID)
		}
		if nextRun != "" {
			fmt.Fprintf(out, " | Next: %s", nextRun)
		}
		fmt.Fprintln(out)
	}

	return 0
}

func runScheduleCreate(ctx context.Context, args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("schedule create", flag.ContinueOnError)
	fs.SetOutput(errOut)

	title := fs.String("title", "", "Routine title")
	prompt := fs.String("prompt", "", "Instruction prompt for execution")
	sType := fs.String("type", "daily", "Schedule type: daily, weekly, interval")
	tOfDay := fs.String("time", "08:00", "Time of day in HH:MM format")
	dayOfWeek := fs.Int("day", 1, "Day of week: 0=Sun..6=Sat")
	intervalMins := fs.Int("interval", 60, "Interval in minutes")
	projectID := fs.String("project", "default", "Target project ID")
	jsonOutput := fs.Bool("json", false, "Output JSON")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	t := strings.TrimSpace(*title)
	p := strings.TrimSpace(*prompt)
	if t == "" {
		fmt.Fprintln(errOut, "Error: --title is required.")
		return 2
	}
	if p == "" {
		fmt.Fprintln(errOut, "Error: --prompt is required.")
		return 2
	}

	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		fmt.Fprintln(errOut, "Error: Cuan connection not configured (~/.cuan/config.json).")
		return 1
	}

	payload := map[string]any{
		"title":         t,
		"prompt":        p,
		"schedule_type": strings.ToLower(strings.TrimSpace(*sType)),
		"time_of_day":   strings.TrimSpace(*tOfDay),
		"day_of_week":   *dayOfWeek,
		"interval_mins": *intervalMins,
		"project_id":    strings.TrimSpace(*projectID),
	}

	created, err := client.CreateScheduledTask(ctx, payload)
	if err != nil {
		fmt.Fprintf(errOut, "Error creating scheduled task: %v\n", err)
		return 1
	}

	if *jsonOutput {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(created)
		return 0
	}

	id, _ := created["id"].(string)
	fmt.Fprintln(out, "✓ Scheduled routine created successfully:")
	fmt.Fprintf(out, "  ID:            %s\n", id)
	fmt.Fprintf(out, "  Title:         %s\n", t)
	fmt.Fprintf(out, "  Schedule Type: %s (%s)\n", *sType, *tOfDay)
	fmt.Fprintf(out, "  Project:       %s\n", *projectID)
	return 0
}

func runScheduleUpdate(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "Usage: cuan schedule update <id> [options]")
		return 2
	}

	id := strings.TrimSpace(args[0])
	if id == "" || strings.HasPrefix(id, "-") {
		fmt.Fprintln(errOut, "Error: routine ID is required as first argument. Usage: cuan schedule update <id> [options]")
		return 2
	}

	fs := flag.NewFlagSet("schedule update", flag.ContinueOnError)
	fs.SetOutput(errOut)
	title := fs.String("title", "", "Routine title")
	prompt := fs.String("prompt", "", "Instruction prompt for agent execution")
	sType := fs.String("type", "", "Schedule type: daily, weekly, interval")
	tOfDay := fs.String("time", "", "Time of day in HH:MM format (e.g. 03:00)")
	dayOfWeek := fs.Int("day", -1, "Day of week (0=Sunday, 1=Monday... 6=Saturday)")
	intervalMins := fs.Int("interval", 0, "Interval in minutes")
	projectID := fs.String("project", "", "Target project ID or name")
	jsonOutput := fs.Bool("json", false, "Output JSON")

	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		fmt.Fprintln(errOut, "Error: Cuan connection not configured (~/.cuan/config.json).")
		return 1
	}

	payload := make(map[string]any)
	if t := strings.TrimSpace(*title); t != "" {
		payload["title"] = t
	}
	if p := strings.TrimSpace(*prompt); p != "" {
		payload["prompt"] = p
	}
	if st := strings.TrimSpace(*sType); st != "" {
		payload["schedule_type"] = st
	}
	if tod := strings.TrimSpace(*tOfDay); tod != "" {
		payload["time_of_day"] = tod
	}
	if *dayOfWeek >= 0 && *dayOfWeek <= 6 {
		payload["day_of_week"] = *dayOfWeek
	}
	if *intervalMins > 0 {
		payload["interval_mins"] = *intervalMins
	}
	if pID := strings.TrimSpace(*projectID); pID != "" {
		payload["project_id"] = pID
	}

	if len(payload) == 0 {
		fmt.Fprintln(errOut, "Error: at least one update option must be specified (--title, --prompt, --project, --time, etc.).")
		return 2
	}

	updated, err := client.UpdateScheduledTask(ctx, id, payload)
	if err != nil {
		fmt.Fprintf(errOut, "Error updating scheduled task: %v\n", err)
		return 1
	}

	if *jsonOutput {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(updated)
		return 0
	}

	fmt.Fprintf(out, "✓ Scheduled routine %s updated successfully.\n", id)
	return 0
}

func runScheduleDelete(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "Usage: cuan schedule delete <id>")
		return 2
	}

	id := strings.TrimSpace(args[0])
	if id == "" {
		fmt.Fprintln(errOut, "Error: routine ID is required.")
		return 2
	}

	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		fmt.Fprintln(errOut, "Error: Cuan connection not configured (~/.cuan/config.json).")
		return 1
	}

	if err := client.DeleteScheduledTask(ctx, id); err != nil {
		fmt.Fprintf(errOut, "Error deleting scheduled task: %v\n", err)
		return 1
	}

	fmt.Fprintf(out, "✓ Scheduled routine %s deleted.\n", id)
	return 0
}

func runScheduleToggle(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "Usage: cuan schedule toggle <id> [--enable|--disable]")
		return 2
	}

	id := strings.TrimSpace(args[0])
	fs := flag.NewFlagSet("schedule toggle", flag.ContinueOnError)
	fs.SetOutput(errOut)
	enableFlag := fs.Bool("enable", false, "Enable the routine")
	disableFlag := fs.Bool("disable", false, "Disable the routine")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	var enabledPtr *bool
	if *enableFlag && !*disableFlag {
		val := true
		enabledPtr = &val
	} else if *disableFlag && !*enableFlag {
		val := false
		enabledPtr = &val
	}

	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		fmt.Fprintln(errOut, "Error: Cuan connection not configured (~/.cuan/config.json).")
		return 1
	}

	res, err := client.ToggleScheduledTask(ctx, id, enabledPtr)
	if err != nil {
		fmt.Fprintf(errOut, "Error toggling scheduled task: %v\n", err)
		return 1
	}

	status := "toggled"
	if state, ok := res["enabled"].(bool); ok {
		if state {
			status = "enabled"
		} else {
			status = "disabled"
		}
	}
	fmt.Fprintf(out, "✓ Scheduled routine %s is now %s.\n", id, status)
	return 0
}

func runScheduleTrigger(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "Usage: cuan schedule trigger <id>")
		return 2
	}

	id := strings.TrimSpace(args[0])
	if id == "" {
		fmt.Fprintln(errOut, "Error: routine ID is required.")
		return 2
	}

	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		fmt.Fprintln(errOut, "Error: Cuan connection not configured (~/.cuan/config.json).")
		return 1
	}

	if err := client.TriggerScheduledTask(ctx, id); err != nil {
		fmt.Fprintf(errOut, "Error triggering scheduled task: %v\n", err)
		return 1
	}

	fmt.Fprintf(out, "✓ Scheduled routine %s triggered in background.\n", id)
	return 0
}
