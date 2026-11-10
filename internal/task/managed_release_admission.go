package task

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

// ManagedReleaseClaim records source-stage ownership, not publication evidence.
type ManagedReleaseClaim struct {
	Version          int    `json:"version"`
	TaskID           string `json:"task_id"`
	ProjectID        string `json:"project_id"`
	RoomID           string `json:"room_id"`
	AttemptID        string `json:"attempt_id"`
	RunnerID         string `json:"runner_id"`
	InstructionsHash string `json:"instructions_hash"`
	Repository       string `json:"repository"`
	TargetBranch     string `json:"target_branch"`
}

// ManagedReleaseAdmission reads only the canonical native record. Current
// project settings or model reports cannot create ownership retroactively.
func ManagedReleaseAdmission(t *Task, att *TaskAttempt, activities []*TaskActivity) (*ManagedReleaseClaim, error) {
	if t == nil || att == nil {
		return nil, ErrInvalidTaskInput
	}
	var receipt *TaskActivity
	for _, a := range activities {
		if a == nil || a.ID != "managed-release-claim:"+att.ID {
			continue
		}
		if receipt != nil {
			return nil, errors.New("duplicate managed release admission")
		}
		receipt = a
	}
	if receipt == nil {
		return nil, nil
	}
	var binding ManagedReleaseClaim
	decoder := json.NewDecoder(strings.NewReader(receipt.Message))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&binding) != nil || decoder.Decode(&struct{}{}) != io.EOF || binding.Version != 1 || binding.TaskID != t.ID || binding.ProjectID != t.ProjectID || binding.RoomID != t.SourceRoomID || binding.AttemptID != att.ID || binding.RunnerID != att.RunnerID || binding.InstructionsHash != LaunchInstructionsHash(t.Instructions) || receipt.TaskID != t.ID || att.TaskID != t.ID || receipt.AttemptID == nil || *receipt.AttemptID != att.ID || receipt.Author != att.RunnerID || receipt.ActivityType != "managed_release_claim" || att.StartedAt == nil || receipt.CreatedAt.Before(*att.StartedAt) || receipt.CreatedAt.After(time.Now().UTC()) || att.CancelRequested || att.CanceledAt != nil || t.ProjectID == "" || t.SourceRoomID == "" || t.ExecutionCategory != ExecutionCategoryCode {
		return nil, errors.New("managed release admission no longer matches execution")
	}
	return &binding, nil
}

// IncompleteManagedSourceTimeout distinguishes a known unfinished local source
// stage from an ordinary execution whose external effects need reconciliation.
// It grants neither a retry nor acceptance: PM review owns the bounded correction.
func IncompleteManagedSourceTimeout(t *Task, att *TaskAttempt, activities []*TaskActivity) (bool, error) {
	if t == nil || t.CommitSHA != "" || !IsExecutionTimeout(att) {
		return false, nil
	}
	admission, err := ManagedReleaseAdmission(t, att, activities)
	return admission != nil, err
}
