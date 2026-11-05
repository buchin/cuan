package task

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestUnrecordedLocalFailureNeverInventsTermination(t *testing.T) {
	start := time.Now().Add(-time.Hour)
	base := TaskAttempt{RunnerID: "local_terminal", Status: AttemptStatusFailed, CreatedAt: start.Add(-time.Minute), StartedAt: &start}
	for _, scenario := range []string{"valid", "foreign", "no start", "zero start", "no creation", "has exit", "has finish", "known error", "running", "queued", "cancel requested", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			a := base
			switch scenario {
			case "foreign":
				a.RunnerID = "other"
			case "no start":
				a.StartedAt = nil
			case "zero start":
				zero := time.Time{}
				a.StartedAt = &zero
			case "no creation":
				a.CreatedAt = time.Time{}
			case "has exit":
				exit := 124
				a.ExitCode = &exit
			case "has finish":
				a.FinishedAt = &start
			case "known error":
				a.ErrorMessage = "Known failure"
			case "running":
				a.Status = AttemptStatusRunning
			case "queued":
				a.Status = AttemptStatusQueued
			case "cancel requested":
				a.CancelRequested = true
			case "cancelled":
				a.CanceledAt = &start
			}
			if IsUnrecordedLocalFailure(&a) != (scenario == "valid") {
				t.Fatal("invalid legacy terminal failure eligibility")
			}
			if IsExecutionTimeout(&a) || IsOpaqueLocalFailure(&a) {
				t.Fatal("missing metadata became a termination diagnosis")
			}
			if a.Status != base.Status && scenario != "running" && scenario != "queued" {
				t.Fatal("classification rewrote execution state")
			}
		})
	}
}

func TestOpaqueLocalFailurePreservesUnknownCause(t *testing.T) {
	start, end := time.Now().Add(-20*time.Minute), time.Now()
	exit := 124
	base := TaskAttempt{RunnerID: "local_terminal", Status: AttemptStatusFailed, ExitCode: &exit, StartedAt: &start, FinishedAt: &end}
	for _, scenario := range []string{"valid", "foreign runner", "known error", "missing start", "missing end", "invalid interval", "no exit", "running", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			a := base
			switch scenario {
			case "foreign runner":
				a.RunnerID = "remote"
			case "known error":
				a.ErrorMessage = "Unknown external effects"
			case "missing start":
				a.StartedAt = nil
			case "missing end":
				a.FinishedAt = nil
			case "invalid interval":
				a.FinishedAt = &start
			case "no exit":
				a.ExitCode = nil
			case "running":
				a.Status = AttemptStatusRunning
			case "cancelled":
				a.CancelRequested = true
			}
			if IsOpaqueLocalFailure(&a) != (scenario == "valid") {
				t.Fatal("opaque failure eligibility lost terminal provenance")
			}
			if IsExecutionTimeout(&a) {
				t.Fatal("opaque failure was relabelled a native timeout")
			}
		})
	}
}

func TestCurrentOutcomeLaunchPreservesScopeAndSingleRequestIdentity(t *testing.T) {
	now := time.Now().UTC()
	task := &Task{ID: "task", ProjectID: "project", SourceRoomID: "room", Instructions: "Approved original requirements"}
	base := LiveInspectionIntent{Purpose: CurrentOutcomeInspection, SourceAttemptID: "failed-timeout", TargetCommit: strings.Repeat("a", 40), Repository: "buchin/cuan", OwnerID: "owner", RoomID: "room", SourceRequestID: "original", InstructionsHash: LaunchInstructionsHash(task.Instructions), ObservedAt: now}
	for _, name := range []string{"valid", "unknown purpose", "invented source", "inspection chain", "wrong scope", "foreign owner", "foreign room", "future observation", "changed target"} {
		t.Run(name, func(t *testing.T) {
			p := base
			launch := &TaskLaunch{TaskID: task.ID, ProjectID: task.ProjectID, OwnerID: p.OwnerID, RoomID: p.RoomID, SourceRequestID: p.SourceRequestID, InstructionsHash: p.InstructionsHash, CreatedAt: now, RequestID: p.RequestID(task.ID)}
			switch name {
			case "unknown purpose":
				p.Purpose = "other"
			case "invented source":
				p.SourceCommit = strings.Repeat("b", 40)
			case "inspection chain":
				p.PreviousAttemptID = "prior"
				p.PreviousCommit = strings.Repeat("b", 40)
			case "wrong scope":
				p.InstructionsHash = "different"
			case "foreign owner":
				p.OwnerID = "other"
			case "foreign room":
				p.RoomID = "other"
			case "future observation":
				p.ObservedAt = now.Add(time.Hour)
			case "changed target":
				p.TargetCommit = strings.Repeat("c", 40)
			}
			raw, _ := json.Marshal(p)
			launch.Revision = LiveInspectionMarker + string(raw)
			got, err := LiveInspectionForLaunch(task, launch)
			valid := name == "valid" || name == "changed target"
			if (err == nil && got != nil) != valid {
				t.Fatalf("unexpected binding acceptance: %v", err)
			}
			if valid && (got.SnapshotSourceCommit() != p.TargetCommit || got.RequestID(task.ID) != base.RequestID(task.ID)) {
				t.Fatal("release change reset budget or snapshot used unknown source")
			}
		})
	}
}

func TestCurrentOutcomeCheckFailureRequiresExactLatestFinalImplementationReport(t *testing.T) {
	attempt := "source"
	old := time.Now().Add(-time.Minute)
	report := &TaskActivity{ID: "report", TaskID: "task", AttemptID: &attempt, ActivityType: "agent_report", NewStatus: TaskStatusReview, CreatedAt: old}
	kind := &TaskActivity{ID: "kind", TaskID: "task", AttemptID: &attempt, ActivityType: "agent_result_kind", NewStatus: "implementation", CreatedAt: old}
	for _, scenario := range []string{"valid", "wrong attempt", "progress only", "newer progress", "other kind", "foreign task"} {
		t.Run(scenario, func(t *testing.T) {
			r, k := *report, *kind
			events := []*TaskActivity{&r, &k}
			switch scenario {
			case "wrong attempt":
				other := "other"
				r.AttemptID = &other
			case "progress only":
				r.NewStatus = "progress"
			case "newer progress":
				later := r
				later.ID = "newer"
				later.CreatedAt = time.Now()
				later.NewStatus = "progress"
				events = append(events, &later)
			case "other kind":
				k.NewStatus = "investigation"
			case "foreign task":
				r.TaskID = "foreign"
			}
			if liveImplementationFinalReport(events, "task", attempt) != (scenario == "valid") {
				t.Fatal("historical or unrelated report granted inspection eligibility")
			}
		})
	}
}

func TestUnpublishedLegacyOutcomeKeepsBranchAndExecutionHistory(t *testing.T) {
	now := time.Now().UTC()
	created := now.Add(-time.Hour)
	started := created.Add(time.Second)
	finished := started.Add(time.Second)
	original := "Cuan managed delegation v1.\nImplement and deploy the feature.\nApproval message: approved\nUser authorization: Implement and deploy the feature.\nConversation: 91000000-0000-4000-8000-000000000001\nReport actual progress and verification evidence. Stay within this scope; do not deploy unless authorized."
	for _, scenario := range []string{"valid", "missing audit", "wrong actor", "other attempt", "future audit", "missing report", "progress report", "answer report", "clone only", "no commit", "failed source", "missing exit", "cancelled", "missing start", "bad interval", "published"} {
		t.Run(scenario, func(t *testing.T) {
			exit := 0
			item := &Task{ID: "task", CommitSHA: strings.Repeat("a", 40), Instructions: original, ExecutionCategory: ExecutionCategoryCode}
			a := &TaskAttempt{ID: "source", TaskID: item.ID, Status: AttemptStatusSucceeded, ExitCode: &exit, CreatedAt: created, StartedAt: &started, FinishedAt: &finished}
			audit := &TaskActivity{TaskID: item.ID, AttemptID: &a.ID, ActivityType: ActivityPublicationAcceptanceAudit, Author: "coordinator", NewStatus: "review_required", CreatedAt: now.Add(-time.Minute)}
			report := &TaskActivity{TaskID: item.ID, AttemptID: &a.ID, ActivityType: "agent_report", NewStatus: TaskStatusDone, CreatedAt: finished}
			kind := &TaskActivity{TaskID: item.ID, AttemptID: &a.ID, ActivityType: "agent_result_kind", NewStatus: "implementation", CreatedAt: finished}
			events := []*TaskActivity{audit, report, kind}
			switch scenario {
			case "missing audit":
				events = events[1:]
			case "wrong actor":
				audit.Author = "agent"
			case "other attempt":
				other := "other"
				audit.AttemptID = &other
			case "future audit":
				audit.CreatedAt = now.Add(time.Hour)
			case "missing report":
				events = []*TaskActivity{audit, kind}
			case "progress report":
				report.NewStatus = "progress"
			case "answer report":
				kind.NewStatus = "answer"
			case "clone only":
				item.Instructions = strings.ReplaceAll(original, "Implement and deploy the feature.", "Clone the existing repository.")
			case "no commit":
				item.CommitSHA = ""
			case "failed source":
				a.Status = AttemptStatusFailed
			case "missing exit":
				a.ExitCode = nil
			case "cancelled":
				a.CancelRequested = true
			case "missing start":
				a.StartedAt = nil
			case "bad interval":
				a.FinishedAt = &created
			case "published":
				raw, _ := json.Marshal(DeploymentReceipt{Version: 1, TaskID: item.ID, AttemptID: a.ID, CommitSHA: item.CommitSHA, ReleaseSHA: item.CommitSHA, Environment: "production", Health: "healthy"})
				events = append(events, &TaskActivity{ID: DeploymentReceiptID(item.ID, a.ID, item.CommitSHA), TaskID: item.ID, AttemptID: &a.ID, ActivityType: "deployment_verified", Author: "cuan-deploy", Message: string(raw), CreatedAt: finished})
			}
			if IsUnpublishedLegacyOutcome(item, a, events) != (scenario == "valid") {
				t.Fatal("unsafe unpublished-result eligibility", scenario)
			}
			if a.Status == AttemptStatusSucceeded && scenario != "cancelled" && scenario != "missing exit" && a.ExitCode != nil && *a.ExitCode != 0 {
				t.Fatal("source exit was changed")
			}
			if item.CommitSHA != "" && item.CommitSHA != strings.Repeat("a", 40) {
				t.Fatal("source commit changed")
			}
		})
	}
}
