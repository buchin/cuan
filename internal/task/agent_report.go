package task

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// AgentReport deliberately cannot change human instructions, assignments.
type AgentReport struct {
	Title        *string    `json:"title,omitempty"`
	Priority     *string    `json:"priority,omitempty"`
	Summary      *string    `json:"summary,omitempty"`
	CompleteNote *string    `json:"complete_note,omitempty"`
	Checklist    *[]DoDItem `json:"dod_checklist,omitempty"`
	CommitSHA    *string    `json:"commit_sha,omitempty"`
	TestSummary  *string    `json:"test_summary,omitempty"`
	Progress     string     `json:"progress,omitempty"`
	State        string     `json:"state,omitempty"`
	ResultKind   string     `json:"result_kind,omitempty"`
}

func (s *Service) RequireAgentReport(ctx context.Context, taskID, attemptID, author string) error {
	return s.store.CreateActivity(ctx, &TaskActivity{ID: generateID("act"), TaskID: taskID, AttemptID: &attemptID, ActivityType: "agent_contract", Author: author, NewStatus: "pending", Message: "Agent reporting enabled for this session.", CreatedAt: time.Now().UTC()})
}
func (s *Service) ReportAgent(ctx context.Context, taskID, attemptID, author string, r AgentReport) (*Task, error) {
	return s.reportAgent(ctx, taskID, attemptID, author, r, false)
}

// StageAgentReport validates the complete result but saves it as a checkpoint.
// The transport must confirm artifact delivery before submitting ReportAgent.
// This keeps terminal completion observers from stopping an in-flight delivery.
func (s *Service) StageAgentReport(ctx context.Context, taskID, attemptID, author string, r AgentReport) (*Task, error) {
	return s.reportAgent(ctx, taskID, attemptID, author, r, true)
}

func (s *Service) reportAgent(ctx context.Context, taskID, attemptID, author string, r AgentReport, pendingDelivery bool) (*Task, error) {
	if r.State == "" && r.CompleteNote != nil && strings.TrimSpace(*r.CompleteNote) != "" && strings.TrimSpace(r.Progress) == "" {
		return nil, fmt.Errorf("%w: completion evidence requires an explicit state: review, blocked or done for a final result; progress to save a checkpoint and continue", ErrInvalidTaskInput)
	}
	if r.Title == nil && r.Priority == nil && r.Summary == nil && r.CompleteNote == nil && r.Checklist == nil && r.CommitSHA == nil && r.TestSummary == nil && strings.TrimSpace(r.Progress) == "" && (r.State == "" || r.State == "progress") {
		return nil, fmt.Errorf("%w: send metadata, a progress checkpoint or a result", ErrInvalidTaskInput)
	}
	s.attemptMu.Lock()
	defer s.attemptMu.Unlock()
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	a, e := s.store.GetLatestAttempt(ctx, taskID)
	if e != nil {
		return nil, e
	}
	if a.ID != attemptID || a.Status != AttemptStatusRunning {
		return nil, ErrAttemptFinal
	}
	t, e := s.store.GetTask(ctx, taskID)
	if e != nil {
		return nil, e
	}
	if t.Status != TaskStatusDoing {
		return nil, ErrRequestConflict
	}
	expectedUpdatedAt := t.UpdatedAt
	var activities []*TaskActivity
	bad := func(message string) (*Task, error) { return nil, fmt.Errorf("%w: %s", ErrInvalidTaskInput, message) }
	if r.Title != nil {
		v := strings.TrimSpace(*r.Title)
		if v == "" || len([]rune(v)) > 200 {
			return bad("title must contain1–200 characters")
		}
		t.Title = v
	}
	if r.Priority != nil {
		switch *r.Priority {
		case PriorityLow, PriorityNormal, PriorityHigh, PriorityUrgent:
			t.Priority = *r.Priority
		default:
			return bad("invalid priority")
		}
	}
	if r.Summary != nil {
		if reportTemplatePlaceholder(*r.Summary) {
			return bad("replace the reporting example's placeholder with actual findings")
		}
		if len([]rune(*r.Summary)) > 12000 {
			return bad("summary too long")
		}
		t.Summary = strings.TrimSpace(*r.Summary)
	}
	if r.CompleteNote != nil {
		if reportTemplatePlaceholder(*r.CompleteNote) {
			return bad("replace the reporting example's placeholder with actual verification evidence")
		}
		if len([]rune(*r.CompleteNote)) > 16000 {
			return bad("completion evidence too long")
		}
		t.CompleteNote = strings.TrimSpace(*r.CompleteNote)
	}
	if r.Checklist != nil {
		if e = validateChecklist(*r.Checklist); e != nil {
			return nil, e
		}
		if len(t.DoDChecklist) > 0 {
			if len(*r.Checklist) != len(t.DoDChecklist) {
				return bad("report evidence against the original criteria; do not add or remove approved scope")
			}
			for i, item := range *r.Checklist {
				if item.Text != t.DoDChecklist[i].Text || (t.DoDChecklist[i].ID != "" && item.ID != t.DoDChecklist[i].ID) {
					return bad("preserve the original acceptance criterion text and order")
				}
			}
		}
		t.DoDChecklist = append([]DoDItem{}, (*r.Checklist)...)
	}
	if r.CommitSHA != nil {
		if len(*r.CommitSHA) > 100 {
			return bad("commit reference too long")
		}
		t.CommitSHA = *r.CommitSHA
	}
	if r.TestSummary != nil {
		if len([]rune(*r.TestSummary)) > 12000 {
			return bad("test summary too long")
		}
		t.TestSummary = *r.TestSummary
	}
	if len([]rune(r.Progress)) > 2000 {
		return bad("progress message too long")
	}
	if r.State == "" {
		r.State = "progress"
	}
	switch r.State {
	case "progress", "review", "blocked", "done":
	default:
		return bad("state must be progress, review, blocked or done")
	}
	if r.State != "progress" && (r.Summary == nil || r.CompleteNote == nil || t.Summary == "" || t.CompleteNote == "") {
		return bad("a result requires summary and completion evidence")
	}
	// Answers remain reviewable; implementation completion is an explicit agent decision.
	if r.State == "done" && r.ResultKind == "answer" {
		r.State = "review"
	}
	if r.State == "done" {
		if gaps := AcceptanceGaps(t); len(gaps) > 0 {
			return bad(strings.Join(gaps, " "))
		}
	}
	if r.ResultKind != "" && (r.State == "progress" || (r.ResultKind == "answer" && r.State != "review") || (r.ResultKind != "answer" && r.ResultKind != "implementation")) {
		return bad("result_kind requires a final report and must be answer or implementation")
	}
	if pendingDelivery {
		r.State = "progress"
		r.ResultKind = ""
		r.Progress = "Result saved; output file delivery is pending."
	}
	t.UpdatedAt = time.Now().UTC()
	message := strings.TrimSpace(r.Progress)
	if message == "" {
		message = "Agent report: " + r.State
	}
	if r.State == "progress" && r.Summary != nil {
		message += "\n\n" + t.Summary
	}
	if r.State != "progress" {
		message += "\n\n## Work summary\n" + t.Summary + "\n\n## Verification\n" + t.CompleteNote
	}
	stamp := time.Now().UTC()
	if r.State != "progress" {
		kind := r.ResultKind
		if kind == "" {
			kind = "implementation"
		}
		activities = append(activities, &TaskActivity{ID: generateID("act"), TaskID: taskID, AttemptID: &attemptID, ActivityType: "agent_result_kind", Author: author, NewStatus: kind, Message: "Result type reported by agent: " + kind, CreatedAt: stamp})
	}
	activities = append(activities, &TaskActivity{ID: generateID("act"), TaskID: taskID, AttemptID: &attemptID, ActivityType: "agent_report", Author: author, NewStatus: r.State, Message: message, CreatedAt: stamp})
	completion, ok := s.store.(CompletionStore)
	if !ok {
		return nil, ErrRequestConflict
	}
	if e = completion.CompleteExecution(ctx, a, t, nil, expectedUpdatedAt, a.RunnerID, a.RunnerID != "local_terminal", activities...); e != nil {
		return nil, e
	}

	return t, nil
}

func reportTemplatePlaceholder(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "<summary>", "<concise summary of work done>", "<detailed dod verification evidence>", "<observed result>":
		return true
	}
	return false
}

func (s *Service) AgentReviewReady(ctx context.Context, taskID, attemptID string) (bool, error) {
	t, e := s.store.GetTask(ctx, taskID)
	if e != nil {
		return false, e
	}
	acts, e := s.store.ListActivities(ctx, taskID)
	if e != nil {
		return false, e
	}
	required := false
	var last *TaskActivity
	for _, a := range acts {
		if a.AttemptID == nil || *a.AttemptID != attemptID {
			continue
		}
		if a.ActivityType == "agent_contract" {
			required = true
		}
		if a.ActivityType == "agent_report" && (last == nil || a.CreatedAt.After(last.CreatedAt)) {
			last = a
		}
	}
	if !required {
		return true, nil
	}
	return last != nil && (last.NewStatus == "review" || last.NewStatus == "done") && strings.TrimSpace(t.Summary) != "" && strings.TrimSpace(t.CompleteNote) != "", nil
}
