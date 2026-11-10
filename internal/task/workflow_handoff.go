package task

import (
	"encoding/json"
	"strings"
	"time"
)

const ActivityWorkflowHandoff = "workflow_handoff"
const WorkflowHandoffGrace = 10 * time.Minute

// A short native lease keeps a project workflow's turn across worker -> PM ->
// verifier/publisher handoffs. It is not an execution lease or a completion claim.
func NewWorkflowHandoff(t *Task, a *TaskAttempt, now time.Time) *TaskActivity {
	if !managedCodeWorkflow(t) || a == nil || a.TaskID != t.ID || a.CancelRequested || a.CanceledAt != nil || a.RunnerID == "" || a.StartedAt == nil || a.FinishedAt == nil || (a.Status != AttemptStatusSucceeded && a.Status != AttemptStatusFailed && a.Status != AttemptStatusInterrupted) {
		return nil
	}
	return &TaskActivity{ID: "workflow-handoff:" + a.ID, TaskID: t.ID, AttemptID: &a.ID, Author: a.RunnerID, ActivityType: ActivityWorkflowHandoff, NewStatus: "pending", Message: LaunchInstructionsHash(t.Instructions), CreatedAt: now}
}

func managedCodeWorkflow(t *Task) bool {
	return t != nil && t.SourceRoomID != "" && t.SourceMessageID != "" && strings.HasPrefix(t.Instructions, "Cuan managed delegation v1.\n") && (t.ExecutionCategory == "" || t.ExecutionCategory == ExecutionCategoryCode) && t.Status != TaskStatusDone && t.Status != "cancelled" && t.Status != "canceled"
}

func WorkflowPrecedes(owner, candidate *Task) bool {
	return owner != nil && (candidate == nil || owner.CreatedAt.Before(candidate.CreatedAt) || owner.CreatedAt.Equal(candidate.CreatedAt) && owner.ID < candidate.ID)
}

func WorkflowHandoffPending(t *Task, attempts []*TaskAttempt, events []*TaskActivity, now time.Time) bool {
	if !managedCodeWorkflow(t) {
		return false
	}
	var latest *TaskAttempt
	byID := map[string]*TaskAttempt{}
	for _, a := range attempts {
		if a == nil || a.TaskID != t.ID {
			continue
		}
		byID[a.ID] = a
		if latest == nil || a.CreatedAt.After(latest.CreatedAt) || a.CreatedAt.Equal(latest.CreatedAt) && a.ID > latest.ID {
			latest = a
		}
	}
	if latest == nil || latest.CancelRequested || latest.CanceledAt != nil || (latest.Status == AttemptStatusCanceled || latest.Status == "cancelled") {
		return false
	}
	var selected *TaskActivity
	for _, h := range events {
		if h != nil && h.TaskID == t.ID && h.ActivityType == ActivityWorkflowHandoff && h.Message == LaunchInstructionsHash(t.Instructions) && (selected == nil || h.CreatedAt.After(selected.CreatedAt) || h.CreatedAt.Equal(selected.CreatedAt) && h.ID > selected.ID) {
			selected = h
		}
	}
	for _, h := range events {
		if h != selected {
			continue
		}
		if h == nil || h.TaskID != t.ID || h.AttemptID == nil || h.ActivityType != ActivityWorkflowHandoff || h.ID != "workflow-handoff:"+*h.AttemptID || h.Message != LaunchInstructionsHash(t.Instructions) || h.CreatedAt.After(now) || !h.CreatedAt.Add(WorkflowHandoffGrace).After(now) {
			continue
		}
		a := byID[*h.AttemptID]
		if a == nil || a.RunnerID == "" || a.RunnerID != h.Author || a.CancelRequested || a.CanceledAt != nil || a.FinishedAt == nil || a.Status != AttemptStatusSucceeded && a.Status != AttemptStatusFailed && a.Status != AttemptStatusInterrupted {
			continue
		}
		if h.CreatedAt.Before(a.FinishedAt.Add(-time.Second)) {
			continue
		}
		needsOwner := false
		for _, r := range events {
			if r == nil || r.TaskID != t.ID || r.AttemptID == nil || *r.AttemptID != a.ID || r.Author != "coordinator" || r.ActivityType != "coordinator_review_assessment" || r.CreatedAt.Before(h.CreatedAt) {
				continue
			}
			var record struct {
				Assessment struct {
					Action  string `json:"manager_action"`
					Outcome string `json:"requested_outcome"`
				} `json:"assessment"`
			}
			if json.Unmarshal([]byte(r.Message), &record) == nil && (record.Assessment.Action == "needs_owner" || record.Assessment.Outcome == "needs_input") {
				needsOwner = true
				break
			}
		}
		if !needsOwner {
			return true
		}
	}
	return false
}

func (m *MemoryStore) workflowHandoffLocked(t *Task, now time.Time) bool {
	var attempts []*TaskAttempt
	var events []*TaskActivity
	for _, a := range m.attempts {
		if a.TaskID == t.ID {
			attempts = append(attempts, a)
		}
	}
	for _, e := range m.activities {
		if e.TaskID == t.ID {
			events = append(events, e)
		}
	}
	return WorkflowHandoffPending(t, attempts, events, now)
}

// Aliases are fixed SQL identifiers supplied only by the native stores. No user
// input, revision payload or JSON cast participates in the scheduling predicate.
func WorkflowHandoffSQL(alias string) string {
	return strings.ReplaceAll(`(
 COALESCE(@.source_room_id::text,'')<>'' AND COALESCE(@.source_message_id::text,'')<>''
 AND @.instructions LIKE E'Cuan managed delegation v1.\n%'
 AND COALESCE(@.execution_category,'code')='code' AND @.status NOT IN ('done','cancelled','canceled')
 AND NOT EXISTS(SELECT 1 FROM task_attempts wc WHERE wc.id=(SELECT id FROM task_attempts WHERE task_id=@.id ORDER BY created_at DESC,id DESC LIMIT 1) AND (wc.cancel_requested OR wc.canceled_at IS NOT NULL OR wc.status IN ('canceled','cancelled')))
 AND EXISTS(SELECT 1 FROM task_activities wh JOIN task_attempts wa ON wa.id=wh.attempt_id AND wa.task_id=@.id
 WHERE wh.task_id=@.id AND wh.id='workflow-handoff:'||wa.id AND wh.activity_type='workflow_handoff'
 AND wa.runner_id<>'' AND wh.author=wa.runner_id AND wa.status IN ('succeeded','failed','interrupted')
 AND NOT COALESCE(wa.cancel_requested,false) AND wa.canceled_at IS NULL AND wa.finished_at IS NOT NULL
 AND wh.message=encode(digest(@.instructions,'sha256'),'hex')
 AND wh.id=(SELECT newest.id FROM task_activities newest WHERE newest.task_id=@.id AND newest.activity_type='workflow_handoff' AND newest.message=wh.message ORDER BY newest.created_at DESC,newest.id DESC LIMIT 1)
 AND wh.created_at>=wa.finished_at-INTERVAL '1 second' AND wh.created_at<=NOW() AND wh.created_at>NOW()-INTERVAL '10 minutes'
 AND NOT EXISTS(SELECT 1 FROM task_activities wr WHERE wr.task_id=@.id AND wr.attempt_id=wa.id AND wr.author='coordinator' AND wr.activity_type='coordinator_review_assessment' AND wr.created_at>=wh.created_at AND (wr.message LIKE '%"manager_action":"needs_owner"%' OR wr.message LIKE '%"requested_outcome":"needs_input"%')))
)`, "@", alias)
}

// Keep the older admitted workflow's turn while allowing its own continuation,
// other projects and other execution lanes to proceed.
func WorkflowHandoffBlocksSQL(candidate string) string {
	return "COALESCE(" + candidate + ".execution_category,'code')='code' AND EXISTS(SELECT 1 FROM tasks workflow_owner WHERE workflow_owner.id<>" + candidate + ".id AND (workflow_owner.project_id=" + candidate + ".project_id OR EXISTS(SELECT 1 FROM task_projects wp WHERE wp.task_id=workflow_owner.id AND wp.project_id=" + candidate + ".project_id)) AND (workflow_owner.created_at,workflow_owner.id)<(" + candidate + ".created_at," + candidate + ".id) AND " + WorkflowHandoffSQL("workflow_owner") + ")"
}
