package task

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVerificationIntentRequiresExactAuthorization(t *testing.T) {
	source := strings.Repeat("a", 40)
	task := &Task{ID: "task", SourceRoomID: "room", ProjectID: "project", Instructions: "approved scope"}
	proof := VerificationIntent{SourceAttemptID: "source", SourceCommit: source, InstructionsHash: LaunchInstructionsHash(task.Instructions), OwnerID: "owner", RoomID: "room", SourceRequestID: "approval", AssessmentRequestID: "assessment", SourceVerificationHash: strings.Repeat("b", 64)}
	launch := &TaskLaunch{TaskID: task.ID, ProjectID: task.ProjectID, RequestID: VerificationRequestID(task.ID, source), OwnerID: proof.OwnerID, RoomID: proof.RoomID, SourceRequestID: proof.SourceRequestID, InstructionsHash: proof.InstructionsHash}
	for _, scenario := range []string{"valid", "empty owner", "empty room", "empty approval", "wrong scope", "wrong commit", "wrong request", "duplicate"} {
		t.Run(scenario, func(t *testing.T) {
			p, j := proof, *launch
			switch scenario {
			case "empty owner":
				p.OwnerID = ""
				j.OwnerID = ""
			case "empty room":
				p.RoomID = ""
				j.RoomID = ""
			case "empty approval":
				p.SourceRequestID = ""
				j.SourceRequestID = ""
			case "wrong scope":
				p.InstructionsHash = "other"
			case "wrong commit":
				p.SourceCommit = "short"
			case "wrong request":
				j.RequestID = "other"
			}
			raw, _ := json.Marshal(p)
			j.Revision = VerificationIntentMarker + string(raw)
			if scenario == "duplicate" {
				j.Revision += "\n" + j.Revision
			}
			got, err := VerificationIntentForLaunch(task, &j)
			if scenario == "valid" {
				if err != nil || got == nil {
					t.Fatalf("valid rejected %v", err)
				}
			} else if err == nil {
				t.Fatal("invalid authorization accepted")
			}
		})
	}
}
func TestProjectChecksPassedRejectsUntrustedEvidence(t *testing.T) {
	id := "attempt"
	a := &TaskAttempt{ID: id, TaskID: "task", RunnerID: "runner", Status: AttemptStatusSucceeded}
	d := &TaskDetailResponse{Task: Task{ID: "task"}, LatestAttempt: a}
	for _, c := range []struct {
		author, summary string
		want            bool
	}{{"runner", "Project verification: test.sh --fast (exit 0)", true}, {"agent", "Project verification: test.sh --fast (exit 0)", false}, {"runner", "No project verifier available", false}, {"runner", "Project verification: test.sh --fast (exit 1)", false}} {
		raw, _ := json.Marshal(RunnerVerificationRecord{Summary: c.summary})
		d.Activities = []*TaskActivity{{TaskID: "task", AttemptID: &id, ActivityType: ActivityTypeRunnerVerification, Author: c.author, Message: string(raw)}}
		if got := ProjectChecksPassed(d); got != c.want {
			t.Fatalf("author %s summary %s accepted=%v", c.author, c.summary, got)
		}
	}
}
