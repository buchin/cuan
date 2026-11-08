package diagnostics

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/buchin/cuan/internal/taskcli"
)

var taskSessionMCPTools = []mcpToolDefinition{
	{Name: "prepare_current_task_report", Description: "Read the bound current task and prepare review JSON with unchanged original acceptance IDs/text/order and empty evidence. This does not submit a report or accept work. Fill factual observations and leave unmet criteria false before report_current_task. No task IDs or Owner credentials are accepted.", InputSchema: map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}},
	{Name: "get_current_task", Description: "Read the task and attempt bound to this worker session using its private task capability. No task ID or Owner credentials are accepted.", InputSchema: map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}},
	{Name: "report_current_task", Description: "Report progress or verified results for the bound task attempt. Use state progress for checkpoints; final reports need actual summary, verification and unchanged acceptance criteria with evidence. Attach deliverables using workspace-relative artifacts, or session:PATH for files under CUAN_TASK_ARTIFACTS_DIR outside the repository. This is a worker report, not an Owner message or coordinator acceptance.", InputSchema: taskSessionReportSchema()},
	{Name: "push_current_task", Description: "Publish only the bound task branch through its scoped capability when approved scope allows it. Does not prove deployment or complete the task. Never use for no-push work.", InputSchema: map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}},
	{Name: "download_current_task_artifact", Description: "Download a complete delivered file from a prior or current attempt of this bound task into the active attempt's private artifact directory. Rejects unrelated task attachments, path escape, overwrites, partial responses, and excessive size. Transferring an artifact is not acceptance or verification of its content.", InputSchema: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"attachment_id": map[string]any{"type": "string", "description": "Attachment ID of the delivered file to download."},
			"output":        map[string]any{"type": "string", "description": "Optional destination filename inside the attempt artifact directory."},
		},
		"required":             []string{"attachment_id"},
		"additionalProperties": false,
	}},
	{Name: "read_current_task_artifact", Description: "Read the complete text content of a delivered file from a prior or current attempt of this bound task. Rejects unrelated task attachments, grant expiration, and excessive size.", InputSchema: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"attachment_id": map[string]any{"type": "string", "description": "Attachment ID of the delivered file to read."},
		},
		"required":             []string{"attachment_id"},
		"additionalProperties": false,
	}},
}

func taskSessionReportSchema() map[string]any {
	properties := map[string]any{}
	for _, key := range []string{"title", "priority", "summary", "complete_note", "commit_sha", "test_summary", "progress"} {
		properties[key] = map[string]any{"type": "string"}
	}
	properties["state"] = map[string]any{"type": "string", "enum": []string{"progress", "review", "blocked", "done"}}
	properties["result_kind"] = map[string]any{"type": "string", "enum": []string{"answer", "implementation"}}
	properties["artifacts"] = map[string]any{"type": "array", "maxItems": 8, "items": map[string]any{"type": "string"}}
	properties["dod_checklist"] = map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string"}, "text": map[string]any{"type": "string"}, "completed": map[string]any{"type": "boolean"}, "evidence": map[string]any{"type": "string"}}, "additionalProperties": false}}
	return map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
}

// Typed validation prevents arbitrary IDs, endpoints, credentials or instruction
// changes from entering the reporting payload. Server-side acceptance remains
// authoritative; transport success alone never means the task is finished.
type taskSessionReport struct {
	Title        *string  `json:"title"`
	Priority     *string  `json:"priority"`
	Summary      *string  `json:"summary"`
	CompleteNote *string  `json:"complete_note"`
	CommitSHA    *string  `json:"commit_sha"`
	TestSummary  *string  `json:"test_summary"`
	Progress     string   `json:"progress"`
	State        string   `json:"state"`
	ResultKind   string   `json:"result_kind"`
	Artifacts    []string `json:"artifacts"`
	Checklist    []struct {
		ID        string `json:"id"`
		Text      string `json:"text"`
		Completed bool   `json:"completed"`
		Evidence  string `json:"evidence"`
	} `json:"dod_checklist"`
}

func callTaskSessionTool(name string, arguments map[string]any) (any, error) {
	if !boundTaskSession() {
		return nil, errors.New("this tool requires a bound task session; no Owner credential fallback is permitted")
	}
	var args []string
	switch name {
	case "prepare_current_task_report":
		args = []string{"report-template"}
	case "get_current_task":
		args = []string{"get"}
	case "push_current_task":
		args = []string{"push"}
	case "report_current_task":
		if len(arguments) == 0 {
			return nil, errors.New("provide a progress checkpoint or a final report with actual evidence")
		}
		raw, err := json.Marshal(arguments)
		if err != nil || len(raw) > 64000 {
			return nil, errors.New("task report exceeds 64000 bytes or contains invalid JSON")
		}
		var report taskSessionReport
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&report); err != nil {
			return nil, errors.New("invalid task report fields; use report_current_task schema without task IDs, credentials or instruction changes")
		}
		if report.State != "" && report.State != "progress" && report.State != "review" && report.State != "blocked" && report.State != "done" {
			return nil, errors.New("invalid task report state")
		}
		if report.ResultKind != "" && report.ResultKind != "answer" && report.ResultKind != "implementation" {
			return nil, errors.New("invalid task result kind")
		}
		if len(report.Artifacts) > 8 {
			return nil, errors.New("at most eight workspace-relative artifacts may be attached")
		}
		// Reuse the CLI transport, validation and redirect policy. The private file
		// holds only this report, never the session capability, and is always removed.
		file, err := os.CreateTemp("", "cuan-mcp-report-*.json")
		if err != nil {
			return nil, errors.New("could not prepare private task report")
		}
		defer os.Remove(file.Name())
		_, writeErr := file.Write(raw)
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			return nil, errors.New("could not prepare private task report")
		}
		args = []string{"report", "--report-file", file.Name()}
	case "download_current_task_artifact", "download_task_artifact":
		if arguments["task_id"] != nil {
			return nil, errors.New("current-task tools do not accept task IDs; use the bound capability")
		}
		attID, _ := arguments["attachment_id"].(string)
		attID = strings.TrimSpace(attID)
		if attID == "" {
			return nil, errors.New("attachment_id is required")
		}
		args = []string{"artifact", "download", attID}
		if outPath, ok := arguments["output"].(string); ok && strings.TrimSpace(outPath) != "" {
			args = append(args, "--output", strings.TrimSpace(outPath))
		}
	case "read_current_task_artifact", "read_task_artifact":
		if arguments["task_id"] != nil {
			return nil, errors.New("current-task tools do not accept task IDs; use the bound capability")
		}
		attID, _ := arguments["attachment_id"].(string)
		attID = strings.TrimSpace(attID)
		if attID == "" {
			return nil, errors.New("attachment_id is required")
		}
		args = []string{"artifact", "read", attID}
	default:
		return nil, errors.New("unknown task session tool")
	}
	switch name {
	case "report_current_task", "download_current_task_artifact", "download_task_artifact", "read_current_task_artifact", "read_task_artifact":
	default:
		if len(arguments) > 0 {
			return nil, errors.New("current-task tools do not accept task IDs or other arguments")
		}
	}
	var out, errOut bytes.Buffer
	if code := taskcli.Run(args, &out, &errOut); code != 0 {
		return nil, fmt.Errorf("task capability request failed: %s", strings.TrimSpace(errOut.String()))
	}
	var result any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		return nil, errors.New("task capability response was invalid; inspect the same task before retrying")
	}
	return result, nil
}
