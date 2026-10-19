package task

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// AgentReport deliberately cannot change human instructions, assignments or Done.
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
		if len([]rune(*r.Summary)) > 12000 {
			return bad("summary too long")
		}
		t.Summary = strings.TrimSpace(*r.Summary)
	}
	if r.CompleteNote != nil {
		if len([]rune(*r.CompleteNote)) > 16000 {
			return bad("completion evidence too long")
		}
		t.CompleteNote = strings.TrimSpace(*r.CompleteNote)
	}
	if r.Checklist != nil {
		if e = validateChecklist(*r.Checklist); e != nil {
			return nil, e
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
	case "progress", "review", "blocked":
	default:
		return bad("state must be progress, review or blocked")
	}
	if r.State != "progress" && (t.Summary == "" || t.CompleteNote == "") {
		return bad("a result requires summary and completion evidence")
	}
	if r.ResultKind != "" && (r.State == "progress" || (r.ResultKind == "answer" && r.State != "review") || (r.ResultKind != "answer" && r.ResultKind != "implementation")) {
		return bad("result_kind requires a final report and must be answer or implementation")
	}
	t.UpdatedAt = time.Now().UTC()
	if e = s.store.UpdateTask(ctx, t); e != nil {
		return nil, e
	}
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
		if e = s.store.CreateActivity(ctx, &TaskActivity{ID: generateID("act"), TaskID: taskID, AttemptID: &attemptID, ActivityType: "agent_result_kind", Author: author, NewStatus: kind, Message: "Result type reported by agent: " + kind, CreatedAt: stamp}); e != nil {
			return nil, e
		}
	}
	e = s.store.CreateActivity(ctx, &TaskActivity{ID: generateID("act"), TaskID: taskID, AttemptID: &attemptID, ActivityType: "agent_report", Author: author, NewStatus: r.State, Message: message, CreatedAt: stamp})
	if e != nil {
		return nil, e
	}
	return t, nil
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
	return last != nil && last.NewStatus == "review" && strings.TrimSpace(t.Summary) != "" && strings.TrimSpace(t.CompleteNote) != "", nil
}
