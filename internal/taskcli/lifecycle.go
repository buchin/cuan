package taskcli

import (
	"bytes"
	"encoding/json"
	"github.com/buchin/cuan/internal/task"
	"io"
	"os"
	"path/filepath"
	"time"
)

// runStopHook adapts AGY's native stop event to Cuan's report protocol. It never
// submits a result, accepts work, or writes to the interactive terminal.
func runStopHook(input io.Reader, out io.Writer) int {
	result := struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}{Decision: "allow"}
	defer func() { _ = json.NewEncoder(out).Encode(result) }()
	var event struct {
		FullyIdle         bool   `json:"fullyIdle"`
		TerminationReason string `json:"terminationReason"`
		Error             string `json:"error"`
	}
	if json.NewDecoder(io.LimitReader(input, 64*1024)).Decode(&event) != nil || !event.FullyIdle || event.Error != "" {
		return 0
	}
	if event.TerminationReason != "NO_TOOL_CALL" && event.TerminationReason != "TERMINAL_STEP_TYPE" {
		return 0
	}
	deadline, err := time.Parse(time.RFC3339Nano, os.Getenv("CUAN_TASK_REPORT_DEADLINE"))
	if err != nil || time.Until(deadline) < 15*time.Second {
		return 0
	}
	root := os.Getenv("CUAN_TASK_LIFECYCLE_DIR")
	if !filepath.IsAbs(root) {
		return 0
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return 0
	}
	// Use only the existing scoped CLI read. Failure or ambiguous transport never
	// authorizes a continuation, and no Owner credential fallback is possible.
	var snapshot bytes.Buffer
	if Run([]string{"get"}, &snapshot, io.Discard) != 0 {
		return 0
	}
	var data task.TaskDetailResponse
	if json.Unmarshal(snapshot.Bytes(), &data) != nil || data.LatestAttempt == nil || data.LatestAttempt.ID == "" || data.LatestAttempt.Status != "running" || data.LatestAttempt.CancelRequested {
		return 0
	}
	if data.FinalAgentReportState(data.LatestAttempt.ID) != "" {
		return 0
	}
	if time.Until(deadline) < 15*time.Second {
		return 0
	}
	// The private runner-created directory is unique per execution. O_EXCL makes
	// simultaneous/repeated native stop callbacks share one continuation budget.
	marker, err := os.OpenFile(filepath.Join(root, "report-continuation"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return 0
	}
	_ = marker.Close()
	result.Decision = "continue"
	result.Reason = "Finish reporting only. Read the current bound task and save an explicit final task report (review or blocked) with the actual result, verification evidence and unresolved gaps using report_current_task or the scoped task report CLI. Do not investigate, edit, deploy, retry external operations, or begin additional work. Do not invent missing evidence or claim acceptance. Preserve the original deadline and cancellation. A prose reply is not a saved task report."
	return 0
}
