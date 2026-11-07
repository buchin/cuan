package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

// Only coordinator code writes this event. Agent report/comment endpoints cannot
// create a phase authorization. Raw task prose is never an authorization record.
type phaseLineage struct {
	Version                int    `json:"version"`
	Phase                  int    `json:"phase"`
	TaskID                 string `json:"task_id"`
	RootTaskID             string `json:"root_task_id"`
	ParentTaskID           string `json:"parent_task_id"`
	ParentAttemptID        string `json:"parent_attempt_id"`
	OwnerID                string `json:"owner_id"`
	RoomID                 string `json:"room_id"`
	ProjectID              string `json:"project_id"`
	RootRequestID          string `json:"root_request_id"`
	RootInstructionsHash   string `json:"root_instructions_hash"`
	ParentInstructionsHash string `json:"parent_instructions_hash"`
	InstructionsHash       string `json:"instructions_hash"`
	ExecutionCategory      string `json:"execution_category"`
	PlanHash               string `json:"plan_hash"`
}

type phaseApproval struct {
	Root    *task.TaskDetailResponse
	Request db.InboxRequest
	Plan    *approvedInboxPlan
	Phase   int
}

type phaseAuthorizationError struct{ reason string }

func (e *phaseAuthorizationError) Error() string { return e.reason }
func phaseDenied(reason string) error            { return &phaseAuthorizationError{reason: reason} }
func phaseLookupError(err error) error {
	if errors.Is(err, task.ErrTaskNotFound) || errors.Is(err, sql.ErrNoRows) {
		return phaseDenied("phase approval record is missing")
	}
	return err
}

func phaseReceiptID(id string) string { return "act_phase_" + delegationMessageID(id) }
func phasePlanHash(plan *approvedInboxPlan) string {
	data, _ := json.Marshal(plan)
	return task.LaunchInstructionsHash(string(data))
}

func phaseTaskInstructions(text string) string {
	if at := strings.Index(text, "\nProject guidance snapshot:"); at >= 0 {
		text = text[:at]
	}
	return strings.TrimSpace(text)
}

func (s *Server) phaseApproval(ctx context.Context, d *task.TaskDetailResponse, owner, room string) (*phaseApproval, error) {
	if d == nil || s.taskSvc == nil || s.projectStudioRepo == nil || d.SourceRoomID != room {
		return nil, phaseDenied("approved phase context unavailable")
	}
	root := d
	var lineage *phaseLineage
	var predecessor *task.TaskDetailResponse
	for _, event := range d.Activities {
		if event == nil || event.ID != phaseReceiptID(d.ID) || event.TaskID != d.ID || event.Author != "cuan-plan" || event.ActivityType != "approved_phase" {
			continue
		}
		var value phaseLineage
		if json.Unmarshal([]byte(event.Message), &value) != nil || value.Version != 1 || value.Phase < 2 || value.TaskID != d.ID || value.OwnerID != owner || value.RoomID != room || value.ProjectID != d.ProjectID || value.ExecutionCategory != d.ExecutionCategory || value.InstructionsHash != task.LaunchInstructionsHash(d.Instructions) || value.RootTaskID == d.ID || value.ParentTaskID == d.ID {
			return nil, phaseDenied("phase authorization changed")
		}
		parent, err := s.taskSvc.GetTaskDetail(ctx, value.ParentTaskID)
		if err != nil {
			return nil, phaseLookupError(err)
		}
		if parent == nil || parent.ProjectID != d.ProjectID || parent.ExecutionCategory != value.ExecutionCategory || parent.SourceRoomID != room || parent.Status != task.TaskStatusDone || parent.LatestAttempt == nil || parent.LatestAttempt.Status != task.AttemptStatusSucceeded || parent.LatestAttempt.CancelRequested || parent.LatestAttempt.CanceledAt != nil || task.LaunchInstructionsHash(parent.Instructions) != value.ParentInstructionsHash {
			return nil, phaseDenied("preceding phase is not accepted")
		}
		if !phasePredecessorAttemptValid(parent, value.ParentAttemptID) {
			return nil, phaseDenied("preceding phase evidence is unavailable")
		}
		root, err = s.taskSvc.GetTaskDetail(ctx, value.RootTaskID)
		if err != nil {
			return nil, phaseLookupError(err)
		}
		if root == nil || root.Status != task.TaskStatusDone || root.LatestAttempt == nil || root.LatestAttempt.Status != task.AttemptStatusSucceeded || root.LatestAttempt.CancelRequested || root.LatestAttempt.CanceledAt != nil || root.ProjectID != d.ProjectID || root.SourceRoomID != room || task.LaunchInstructionsHash(root.Instructions) != value.RootInstructionsHash {
			return nil, phaseDenied("original phase scope changed")
		}
		lineage = &value
		predecessor = parent
		break
	}
	message, err := s.projectStudioRepo.GetMessageByID(ctx, root.SourceMessageID)
	if err != nil {
		return nil, phaseLookupError(err)
	}
	if message == nil || message.AuthorType != "user" || message.RoomID != room {
		return nil, phaseDenied("original user approval unavailable")
	}
	plan := decodeApprovedInboxPlan(message.Content)
	if plan == nil || (plan.Plan.ProjectID != "" && plan.Plan.ProjectID != root.ProjectID) {
		return nil, phaseDenied("approved multi-phase plan unavailable")
	}
	if message.CreatedAt.After(root.CreatedAt) {
		return nil, phaseDenied("approval was created after the task")
	}
	if s.projectSvc != nil {
		bound, e := s.projectSvc.TaskWorkspace(ctx, root.ProjectID, root.Instructions)
		current, ce := s.projectSvc.Get(ctx, root.ProjectID)
		if e != nil || ce != nil || bound == nil || current == nil || bound.RepoURL != current.RepoURL || bound.DefaultBranch != current.DefaultBranch {
			return nil, phaseDenied("approved project repository changed")
		}
	}
	reader, ok := s.projectStudioRepo.(db.InboxRequestRepository)
	if !ok {
		return nil, errors.New("approval storage unavailable")
	}
	requests, err := reader.ListInboxRequests(ctx, room)
	if err != nil {
		return nil, err
	}
	for _, q := range requests {
		if q.OwnerID != owner || q.MessageID != message.ID || q.State == "canceled" || q.State == "cancelled" || q.State == "cancelling" {
			continue
		}
		if lineage != nil && (lineage.RootRequestID != q.RequestID || lineage.PlanHash != phasePlanHash(plan)) {
			return nil, phaseDenied("original approval changed")
		}
		phase := 1
		if match := multiPhaseRegex.FindStringSubmatch(plan.Plan.Title); len(match) > 1 {
			fmt.Sscanf(match[1], "%d", &phase)
		}
		if lineage != nil {
			parentPhase := phase
			if predecessor.ID != root.ID {
				parentPhase = 0
				for _, event := range predecessor.Activities {
					if event == nil || event.ID != phaseReceiptID(predecessor.ID) || event.TaskID != predecessor.ID || event.Author != "cuan-plan" || event.ActivityType != "approved_phase" {
						continue
					}
					var prior phaseLineage
					if json.Unmarshal([]byte(event.Message), &prior) == nil && prior.Version == 1 && prior.TaskID == predecessor.ID && prior.RootTaskID == root.ID && prior.OwnerID == owner && prior.RoomID == room && prior.ProjectID == d.ProjectID && prior.InstructionsHash == task.LaunchInstructionsHash(predecessor.Instructions) && prior.PlanHash == lineage.PlanHash {
						parentPhase = prior.Phase
					}
				}
			}
			if parentPhase < 1 || lineage.Phase != parentPhase+1 {
				return nil, phaseDenied("phase order could not be verified")
			}
			phase = lineage.Phase
			approved := *root
			approved.Title = fmt.Sprintf("Phase %d: predecessor", phase-1)
			approved.Instructions = plan.Plan.Description
			approved.SourceMessageID = ""
			target, exists := s.detectNextMultiPhase(ctx, &approved)
			if !exists || len(d.DoDChecklist) != 1 || d.DoDChecklist[0].Text != target.NextTitle {
				return nil, phaseDenied("approved phase acceptance outcome changed")
			}
		}
		return &phaseApproval{Root: root, Request: q, Plan: plan, Phase: phase}, nil
	}
	return nil, phaseDenied("original user approval is not active")
}

func (s *Server) authorizeApprovedPhase(ctx context.Context, j task.TaskLaunch, d *task.TaskDetailResponse) (bool, error) {
	if d == nil || j.ProjectID != d.ProjectID || j.InstructionsHash != task.LaunchInstructionsHash(d.Instructions) {
		return false, nil
	}
	found := false
	for _, event := range d.Activities {
		if event != nil && event.ID == phaseReceiptID(d.ID) && !event.CreatedAt.After(j.CreatedAt) {
			found = true
		}
	}
	if !found {
		return false, nil
	}
	approval, err := s.phaseApproval(ctx, d, j.OwnerID, j.RoomID)
	if err != nil {
		var denied *phaseAuthorizationError
		if errors.As(err, &denied) {
			return false, nil
		}
		return false, err
	}
	if approval.Request.CreatedAt.After(j.CreatedAt) {
		return false, nil
	}
	return approval.Root.ID != d.ID && approval.Request.RequestID == j.SourceRequestID, nil
}

func (s *Server) savePhaseLineage(ctx context.Context, child, parent *task.TaskDetailResponse, approval *phaseApproval, owner, room string) error {
	if parent.Status != task.TaskStatusDone || parent.LatestAttempt == nil || parent.LatestAttempt.Status != task.AttemptStatusSucceeded || parent.LatestAttempt.CancelRequested || parent.LatestAttempt.CanceledAt != nil {
		return phaseDenied("preceding phase has not been verified")
	}
	value := phaseLineage{ExecutionCategory: child.ExecutionCategory, Version: 1, Phase: approval.Phase + 1, TaskID: child.ID, RootTaskID: approval.Root.ID, ParentTaskID: parent.ID, ParentAttemptID: parent.LatestAttempt.ID, OwnerID: owner, RoomID: room, ProjectID: child.ProjectID, RootRequestID: approval.Request.RequestID, RootInstructionsHash: task.LaunchInstructionsHash(approval.Root.Instructions), ParentInstructionsHash: task.LaunchInstructionsHash(parent.Instructions), InstructionsHash: task.LaunchInstructionsHash(child.Instructions), PlanHash: phasePlanHash(approval.Plan)}
	data, _ := json.Marshal(value)
	for _, event := range child.Activities {
		if event != nil && event.ID == phaseReceiptID(child.ID) {
			if event.Author != "cuan-plan" || event.ActivityType != "approved_phase" {
				return phaseDenied("saved phase authorization differs")
			}
			if event.Message != string(data) {
				// A corrected predecessor changes execution evidence, not the user's plan.
				// Compare every original authorization field and retain its immutable receipt.
				var original phaseLineage
				if json.Unmarshal([]byte(event.Message), &original) != nil {
					return phaseDenied("saved phase authorization differs")
				}
				expected := value
				expected.ParentAttemptID = original.ParentAttemptID
				if expected != original || !phasePredecessorAttemptValid(parent, original.ParentAttemptID) {
					return phaseDenied("saved phase authorization differs")
				}
			}
			return nil
		}
	}
	if err := s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{ID: phaseReceiptID(child.ID), TaskID: child.ID, ActivityType: "approved_phase", Author: "cuan-plan", Message: string(data), CreatedAt: time.Now().UTC()}); err != nil {
		// A concurrent replay may have saved the same immutable receipt first.
		latest, e := s.taskSvc.GetTaskDetail(ctx, child.ID)
		if e == nil {
			for _, event := range latest.Activities {
				if event != nil && event.ID == phaseReceiptID(child.ID) && event.Author == "cuan-plan" && event.ActivityType == "approved_phase" && event.Message == string(data) {
					return nil
				}
			}
		}
		return fmt.Errorf("phase authorization could not be saved: %w", err)
	}
	return nil
}

// A phase receipt records the predecessor accepted when the plan advanced. A
// later accepted correction within identical instructions does not revoke that
// plan. Keep both historical provenance and current dependency checks; never
// accept a failed/cancelled predecessor, missing history or an attempt rewind.
func phasePredecessorAttemptValid(parent *task.TaskDetailResponse, originalID string) bool {
	if parent == nil || parent.LatestAttempt == nil || originalID == "" {
		return false
	}
	if parent.LatestAttempt.ID == originalID {
		return true
	}
	for _, historical := range parent.Attempts {
		if historical != nil && historical.ID == originalID && historical.TaskID == parent.ID && historical.Status == task.AttemptStatusSucceeded && !historical.CancelRequested && historical.CanceledAt == nil && historical.AttemptNumber < parent.LatestAttempt.AttemptNumber {
			return true
		}
	}
	return false
}
