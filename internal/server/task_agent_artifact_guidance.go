package server

import (
	"encoding/json"
	"github.com/buchin/cuan/internal/task"
)

func artifactSessionGuidance(items []task.DoDItem) string {
	type criterion struct {
		ID   string `json:"id,omitempty"`
		Text string `json:"text"`
	}
	original := make([]criterion, 0, len(items))
	for _, item := range items {
		original = append(original, criterion{ID: item.ID, Text: item.Text})
	}
	raw, _ := json.Marshal(original)
	return `## Bound file-worker reporting
The original request supplied with this session is already approved. Use the private output workspace
to create only the requested file and inspect its actual contents. For fully
specified content, execute and verify directly; do not investigate unrelated code
or plan a software project. Do not clone, initialize Git, configure services,
publish, deploy, read account profiles or use Owner tools.

Report the actual result using actual report/output paths:
"$CUAN_TASK_EXECUTABLE" task report --report-file REPORT.json --artifact RELATIVE_PATH
For output under CUAN_TASK_ARTIFACTS_DIR use --artifact session:PATH.
The JSON needs state:"review", a factual summary,
complete_note, and dod_checklist. For every original criterion below preserve its
ID, text and order; add completed and current observed evidence. Leave unmet
criteria false with their actual gap, or report state:"blocked" for a blocker.
A successful report submits the result for independent review, not acceptance.
Do not stop without a confirmed final receipt. If the original request needs
source data or context not supplied, use the task's scoped reads to obtain it;
never invent data or waive requirements. If a response is uncertain or a report
is rejected, read the same current task and reconcile before resubmitting.
Never print the private capability file. An internal path alone is not delivery.

Original acceptance criteria:
` + "```json\n" + string(raw) + "\n```\n"
}
