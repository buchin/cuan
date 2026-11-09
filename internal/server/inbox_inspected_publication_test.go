package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/task"
)

func TestInspectedUnpublishedSourceReturnsPublicationToDeveloper(t *testing.T) {
	for _, scenario := range []string{"valid", "changed release", "missing optional manager fields", "implementation without manager fields", "already contained", "unknown ancestry", "missing receipt", "missing report", "changed source", "owner prohibition", "wait", "needs owner", "needs input"} {
		t.Run(scenario, func(t *testing.T) {
			s, d, origin, _, binding := failedInspectionSourceFixture(t, false, true)
			ctx := context.Background()
			store := s.taskSvc.Store()
			p, err := s.projectSvc.Store().GetProject(ctx, d.ProjectID)
			if err != nil {
				t.Fatal(err)
			}
			p.RepoURL = "https://github.com/fixture/repo.git"
			p.LocalPath = t.TempDir()
			if err := s.projectSvc.Store().UpdateProject(ctx, p); err != nil {
				t.Fatal(err)
			}
			s.cfg.Environment, s.cfg.SourceRepository, s.cfg.CommitSHA = "production", "fixture/repo", binding.TargetCommit
			if scenario == "changed release" {
				s.cfg.CommitSHA = strings.Repeat("c", 40)
			}
			receipt, _ := json.Marshal(task.LiveInspectionReceipt{Purpose: binding.Purpose, SourceAttemptID: binding.SourceAttemptID, SourceCommit: binding.SourceCommit, TargetCommit: binding.TargetCommit, Repository: binding.Repository, ProjectChecksHash: task.VerificationSummaryHash("Project verification: test.sh --fast (exit 0)")})
			id := d.LatestAttempt.ID
			if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "live-result:" + id, TaskID: d.ID, AttemptID: &id, ActivityType: task.ActivityLiveInspection, Author: d.LatestAttempt.RunnerID, Message: string(receipt), CreatedAt: time.Now()}); err != nil {
				t.Fatal(err)
			}
			d, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
			if err != nil {
				t.Fatal(err)
			}
			bin := t.TempDir()
			script := "#!/bin/sh\ncase \"$*\" in\n *cat-file*) exit 0;;\n *merge-base*) exit 1;;\n *--is-shallow-repository*) echo false;;\n *rev-list*) echo " + s.cfg.CommitSHA + ";;\n *) exit 1;;\nesac\n"
			if scenario == "unknown ancestry" {
				script = "#!/bin/sh\nexit 1\n"
			}
			if scenario == "already contained" {
				script = strings.Replace(script, "*merge-base*) exit 1", "*merge-base*) exit 0", 1)
			}
			if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
			if scenario == "missing receipt" || scenario == "missing report" {
				var events []*task.TaskActivity
				for _, event := range d.Activities {
					if event.AttemptID != nil && *event.AttemptID == id && ((scenario == "missing receipt" && event.ActivityType == task.ActivityLiveInspection) || (scenario == "missing report" && event.ActivityType == "agent_report")) {
						continue
					}
					events = append(events, event)
				}
				d.Activities = events
			}
			if scenario == "changed source" {
				d.CommitSHA = strings.Repeat("d", 40)
			}
			if scenario == "owner prohibition" {
				d.Instructions += "\nDo not deploy."
			}
			decision := inboxReviewEvaluation{Decision: "revise", RequestedOutcome: "unfinished", RemainingWork: "publication", Finding: "Reviewed implementation still requires publication", Explanation: "Source behavior passed; complete original delivery", Revision: "Reconcile the existing code and publish the authorized result through native release", ManagerAction: "continue", ManagerReason: "Source checks passed; original publication remains"}
			if scenario == "wait" {
				decision.ManagerAction = "wait"
			}
			if scenario == "needs owner" {
				decision.ManagerAction = "needs_owner"
			}
			if scenario == "needs input" {
				decision.RequestedOutcome = "needs_input"
			}
			if scenario == "missing optional manager fields" || scenario == "implementation without manager fields" {
				decision.ManagerAction, decision.ManagerReason = "", ""
			}
			if scenario == "implementation without manager fields" {
				decision.RemainingWork = "implementation"
			}
			got := s.requirePublicationReview(ctx, d, decision)
			allowed := scenario == "valid" || scenario == "changed release" || scenario == "missing optional manager fields" || scenario == "implementation without manager fields"
			if (got.RemainingWork == "implementation") != allowed {
				t.Fatalf("publication routing=%+v for %s", got, scenario)
			}
			if !allowed {
				return
			}
			if again := s.requirePublicationReview(ctx, d, got); again != got {
				t.Fatal("publication normalization changed its saved continuation")
			}
			// Exercise the persisted review and actual separate developer launch.
			repo := s.projectStudioRepo.(db.InboxRequestRepository)
			q := origin
			key := managerReviewKey(d)
			q.RequestID, q.MessageID, q.ReplyID = delegationMessageID(key), delegationMessageID(key+":input"), delegationMessageID(key+":reply")
			q.State, q.CreatedAt, q.Deadline = "queued", time.Now(), time.Now().Add(time.Hour)
			if _, _, err := repo.AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "system", AuthorID: "cuan_work_review", Content: inboxOwnershipPolicy + "\nReview [/tasks/" + d.ID + "]", CreatedAt: q.CreatedAt}); err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(decision)
			s.applyInboxReviewEvaluation(ctx, q, string(raw))
			launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID))
			if err != nil || launch == nil || launch.State != "queued" {
				t.Fatal("missing separate integration continuation", err, launch)
			}
			if live, err := task.LiveInspectionForLaunch(&d.Task, launch); err != nil || live != nil {
				t.Fatal("publication reused read-only inspection", err)
			}
			if limits := executionpolicy.FromInstructions(launch.Revision); limits.ReadOnlyRepository || limits.DenyPush || limits.DenyDeploy {
				t.Fatal("historical inspection description restricted the separate developer", limits)
			}
			after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
			if after.Status == task.TaskStatusDone || len(after.Attempts) != len(d.Attempts) {
				t.Fatal("routing accepted work or rewrote attempts")
			}
		})
	}
}
