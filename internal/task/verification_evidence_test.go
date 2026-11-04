package task

import (
	"testing"
	"time"
)

func TestAttemptVerificationDoesNotInheritEarlierTaskSummary(t *testing.T) {
	current, old := "current", "old"
	d := &TaskDetailResponse{Task: Task{ID: "task", TestSummary: "OLD_FAILED_CHECK"}, LatestAttempt: &TaskAttempt{ID: current}}
	d.Activities = []*TaskActivity{{ID: "old", TaskID: d.ID, AttemptID: &old, ActivityType: ActivityTypeRunnerVerification, Message: `{"summary":"OLD_FAILED_CHECK"}`}}
	got := AttemptVerificationEvidence(d)
	if got.Current != "" || got.AttemptID != "" || got.Historical != "OLD_FAILED_CHECK" {
		t.Fatalf("old verification became current: %+v", got)
	}
	d.Activities = append(d.Activities, &TaskActivity{ID: "current", TaskID: d.ID, AttemptID: &current, ActivityType: ActivityTypeRunnerVerification, Message: `{"summary":""}`})
	got = AttemptVerificationEvidence(d)
	if got.Current != "" || got.AttemptID != current || got.Provenance != "current_attempt_verification" || got.Historical != "OLD_FAILED_CHECK" {
		t.Fatalf("empty current check inherited history: %+v", got)
	}
}

func TestAttemptVerificationDeterministicAndTaskBound(t *testing.T) {
	current := "current"
	now := time.Now()
	d := &TaskDetailResponse{Task: Task{ID: "task", TestSummary: "saved"}, LatestAttempt: &TaskAttempt{ID: current}, Activities: []*TaskActivity{
		{ID: "b", TaskID: "task", AttemptID: &current, ActivityType: ActivityTypeRunnerVerification, Message: `{"summary":"LATEST"}`, CreatedAt: now},
		{ID: "a", TaskID: "task", AttemptID: &current, ActivityType: ActivityTypeRunnerVerification, Message: `{"summary":"EARLIER"}`, CreatedAt: now},
		{ID: "foreign", TaskID: "foreign", AttemptID: &current, ActivityType: ActivityTypeRunnerVerification, Message: `{"summary":"WRONG_TASK"}`, CreatedAt: now.Add(time.Hour)},
	}}
	for i := 0; i < len(d.Activities); i++ {
		if got := AttemptVerificationEvidence(d); got.Current != "LATEST" {
			t.Fatalf("ordering or foreign task changed evidence: %+v", got)
		}
		d.Activities = append(d.Activities[1:], d.Activities[0])
	}
	d.Activities = append(d.Activities, &TaskActivity{ID: "invalid", TaskID: d.ID, AttemptID: &current, ActivityType: ActivityTypeRunnerVerification, Message: `{}`, CreatedAt: now.Add(time.Hour)})
	if got := AttemptVerificationEvidence(d); got.Current != "" || got.Provenance != "invalid_current_attempt_verification" {
		t.Fatalf("invalid record inherited old evidence: %+v", got)
	}
}
