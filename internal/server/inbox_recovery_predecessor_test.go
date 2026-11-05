package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestQueuedAutomaticRecoveryRejectsSupersededCheckpoint(t *testing.T) {
	for _, kind := range []string{"task-interruption-recovery:", "task-storage-recovery:", "task-provider-recovery:"} {
		for _, scenario := range []string{"newer failure", "own running receipt", "own failed receipt"} {
			t.Run(kind+scenario, func(t *testing.T) {
				s, _, _, _ := inboxFixture(t)
				s.inboxStopping.Store(true)
				ctx := context.Background()
				room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
				now := time.Now().UTC()
				q := db.InboxRequest{OwnerID: "owner", RoomID: room.RoomID, AgentID: "agent_lifecycle", RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "queued", PayloadHash: "approved", CreatedAt: now.Add(-time.Hour), Deadline: now.Add(time.Hour)}
				if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: "Approved work", CreatedAt: q.CreatedAt}); err != nil {
					t.Fatal(err)
				}
				item := &task.Task{ID: "source-bound-recovery", ProjectID: "default", SourceRoomID: q.RoomID, SourceMessageID: q.MessageID, AssignedAgent: q.AgentID, Title: "Approved work", Instructions: "Cuan managed delegation v1.\nOriginal approved scope", Status: task.TaskStatusQueued, CreatedAt: now.Add(-time.Hour)}
				if err := s.taskSvc.Store().CreateTask(ctx, item); err != nil {
					t.Fatal(err)
				}
				source := &task.TaskAttempt{ID: "historical-source", TaskID: item.ID, RunnerID: "fixture", AgentName: q.AgentID, Status: task.AttemptStatusFailed, CreatedAt: now.Add(-2 * time.Minute)}
				if err := s.taskSvc.Store().CreateAttempt(ctx, source); err != nil {
					t.Fatal(err)
				}
				j, err := s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{TaskID: item.ID, ProjectID: item.ProjectID, OwnerID: q.OwnerID, RoomID: q.RoomID, RequestID: delegationMessageID(kind + source.ID), SourceRequestID: q.RequestID, AgentID: q.AgentID, InstructionsHash: task.LaunchInstructionsHash(item.Instructions), Revision: "Reconcile the saved work. Do not deploy.", CreatedAt: now.Add(-time.Minute), AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
				if err != nil {
					t.Fatal(err)
				}
				lease, err := s.taskSvc.ClaimLaunch(ctx, now, "fixture", j.ID)
				if err != nil || lease == nil {
					t.Fatal("claim", err)
				}
				if scenario == "newer failure" {
					if err := s.taskSvc.Store().CreateAttempt(ctx, &task.TaskAttempt{ID: "newer-source", TaskID: item.ID, RunnerID: "fixture", AgentName: q.AgentID, Status: task.AttemptStatusFailed, CreatedAt: now.Add(-30 * time.Second)}); err != nil {
						t.Fatal(err)
					}
				} else {
					a, err := s.taskSvc.CreateAttempt(ctx, item.ID, q.OwnerID, task.CreateAttemptRequest{RunnerID: "fixture", AgentName: q.AgentID, RequestID: j.RequestID})
					if err != nil {
						t.Fatal(err)
					}
					a.Status = task.AttemptStatusRunning
					if scenario == "own failed receipt" {
						a.Status = task.AttemptStatusFailed
					}
					if err := s.taskSvc.Store().UpdateAttempt(ctx, a); err != nil {
						t.Fatal(err)
					}
				}
				before, _ := s.taskSvc.GetTaskDetail(ctx, item.ID)
				out := s.executeQueuedTaskLaunch(ctx, *lease, true)
				after, _ := s.taskSvc.GetTaskDetail(ctx, item.ID)
				saved, _ := s.taskSvc.GetLaunch(ctx, j.ID)
				if len(after.Attempts) != len(before.Attempts) || after.LatestAttempt.ID != before.LatestAttempt.ID || after.LatestAttempt.Status != before.LatestAttempt.Status {
					t.Fatal("replayed or rewrote an execution")
				}
				if scenario == "newer failure" {
					if saved.State != "stopped" || saved.SafeError != supersededRecoveryLaunchReason {
						t.Fatalf("obsolete checkpoint accepted: %+v %s", saved, out)
					}
				} else {
					want := "started"
					if scenario == "own failed receipt" {
						want = "stopped"
					}
					if saved.State != want || saved.SafeError != "" || !strings.Contains(out, "recorded execution") {
						t.Fatalf("lost receipt was not reconciled: %+v %s", saved, out)
					}
				}
			})
		}
	}
}
