package task

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVerificationIdentityBindsSourceAttemptAndRetainsLegacyHistory(t *testing.T) {
	commit := strings.Repeat("a", 40)
	item := &Task{ID: "task", ProjectID: "project", SourceRoomID: "room", Instructions: "approved scope"}
	proof := VerificationIntent{SourceAttemptID: "new-source", SourceCommit: commit, InstructionsHash: LaunchInstructionsHash(item.Instructions), OwnerID: "owner", RoomID: "room", SourceRequestID: "approval", AssessmentRequestID: "assessment", SourceVerificationHash: strings.Repeat("b", 64)}
	bound := VerificationRequestID(item.ID, commit, proof.SourceAttemptID)
	if bound == VerificationRequestID(item.ID, commit) || bound == VerificationRequestID(item.ID, commit, "old-source") || bound != VerificationRequestID(item.ID, commit, proof.SourceAttemptID) {
		t.Fatal("verification identity collided or lost retry stability")
	}
	for _, source := range []string{"new-source", "old-source"} {
		proof.SourceAttemptID = source
		raw, _ := json.Marshal(proof)
		launch := &TaskLaunch{TaskID: item.ID, ProjectID: item.ProjectID, OwnerID: proof.OwnerID, RoomID: proof.RoomID, SourceRequestID: proof.SourceRequestID, InstructionsHash: proof.InstructionsHash, RequestID: bound, Revision: VerificationIntentMarker + string(raw)}
		_, err := VerificationIntentForLaunch(item, launch)
		if (err == nil) != (source == "new-source") {
			t.Fatal("source attempt was not bound to its execution identity", source, err)
		}
		launch.RequestID = VerificationRequestID(item.ID, commit)
		if _, err := VerificationIntentForLaunch(item, launch); err != nil {
			t.Fatal("legacy history became unreadable", err)
		}
	}
}

func TestVerificationIntentRequiresExactAuthorization(t *testing.T) {
	source := strings.Repeat("a", 40)
	task := &Task{ID: "task", SourceRoomID: "room", ProjectID: "project", Instructions: "approved scope"}
	proof := VerificationIntent{SourceAttemptID: "source", SourceCommit: source, InstructionsHash: LaunchInstructionsHash(task.Instructions), OwnerID: "owner", RoomID: "room", SourceRequestID: "approval", AssessmentRequestID: "assessment", SourceVerificationHash: strings.Repeat("b", 64)}
	launch := &TaskLaunch{TaskID: task.ID, ProjectID: task.ProjectID, RequestID: VerificationRequestID(task.ID, source), OwnerID: proof.OwnerID, RoomID: proof.RoomID, SourceRequestID: proof.SourceRequestID, InstructionsHash: proof.InstructionsHash}
	for _, scenario := range []string{"valid", "valid capacity", "capacity without parent", "capacity with runtime", "unknown recovery", "empty owner", "empty room", "empty approval", "wrong scope", "wrong commit", "wrong request", "duplicate", "runtime only", "parent only", "wrong runtime", "recovery wrong request"} {
		t.Run(scenario, func(t *testing.T) {
			p, j := proof, *launch
			switch scenario {
			case "valid capacity", "capacity with runtime", "unknown recovery":
				p.RecoveryAttemptID = "prior-verifier"
				p.RecoveryCause = "capacity"
				j.RequestID = VerificationRecoveryRequestID(task.ID, source)
				if scenario == "capacity with runtime" {
					p.RuntimeFingerprint = strings.Repeat("a", 64)
				}
				if scenario == "unknown recovery" {
					p.RecoveryCause = "other"
				}
			case "capacity without parent":
				p.RecoveryCause = "capacity"
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
			case "runtime only":
				p.RuntimeFingerprint = strings.Repeat("a", 64)
			case "parent only":
				p.RecoveryAttemptID = "prior-verifier"
			case "wrong runtime":
				p.RecoveryAttemptID = "prior-verifier"
				p.RuntimeFingerprint = strings.Repeat("z", 64)
			case "recovery wrong request":
				p.RecoveryAttemptID = "prior-verifier"
				p.RuntimeFingerprint = strings.Repeat("a", 64)
			case "wrong request":
				j.RequestID = "other"
			}
			raw, _ := json.Marshal(p)
			j.Revision = VerificationIntentMarker + string(raw)
			if scenario == "duplicate" {
				j.Revision += "\n" + j.Revision
			}
			got, err := VerificationIntentForLaunch(task, &j)
			if scenario == "valid" || scenario == "valid capacity" {
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
	}{
		{"runner", "Project verification: test.sh --fast (exit 0)", true},
		{"runner", "Project verification: registered PHPUnit targets (exit 0)\nOK (1 test)", true},
		{"runner", "Project verification: php vendor/bin/phpunit --colors=never (exit 0)\nOK (107 tests, 294 assertions)", true},
		{"runner", "Project verification: php vendor/bin/pest (exit 0)", true},
		{"runner", "Project verification: php artisan test (exit 0)", true},
		{"runner", "Project verification: go test ./... (exit 0)", true},
		{"runner", "Project verification: npm test --if-present (exit 0)", true},
		{"runner", "Project verification: pytest -q (exit 0)", true},
		{"runner", "Project verification: cargo test (exit 0)", true},
		{"runner", "Project verification: documentation change only; no code checks required (exit 0)", true},
		{"agent", "Project verification: test.sh --fast (exit 0)", false},
		{"runner", "No project verifier available", false},
		{"runner", "Project verification: test.sh --fast (exit 1)", false},
		{"runner", "Project verification: php vendor/bin/phpunit --colors=never (exit 1)", false},
		{"runner", "Project verification incomplete: php vendor/bin/phpunit (exit 0)", false},
	} {
		raw, _ := json.Marshal(RunnerVerificationRecord{Summary: c.summary})
		d.Activities = []*TaskActivity{{TaskID: "task", AttemptID: &id, ActivityType: ActivityTypeRunnerVerification, Author: c.author, Message: string(raw)}}
		if got := ProjectChecksPassed(d); got != c.want {
			t.Fatalf("author %s summary %s accepted=%v, want=%v", c.author, c.summary, got, c.want)
		}
	}
}
