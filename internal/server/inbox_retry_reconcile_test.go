package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestInboxReconciliationRejectsToolsAndUnscopedActions(t *testing.T) {
	for _, scenario := range []string{"answer", "tool", "delegation", "memory", "reconcile"} {
		t.Run(scenario, func(t *testing.T) {
			bin, home := t.TempDir(), t.TempDir()
			reply := coordinatorJSON("Current evidence checked; previous actions were not replayed.", "", nil)
			if scenario == "delegation" {
				reply = coordinatorJSON("Starting duplicate work", "delegation", map[string]string{"title": "Duplicate", "description": "Repeat previous action", "project_id": "default", "authorization_message_id": "source"})
			}
			if scenario == "memory" {
				reply = coordinatorJSON("Saving again", "memory", map[string]string{"content": "Unexpected mutation"})
			}
			if scenario == "reconcile" {
				reply = coordinatorJSON("I will verify current state before continuing the approved work.", "delegation", inboxDelegation{Reconcile: true, Title: "Verify approved work", Description: "Check current state and finish only missing original work.", ProjectID: "default", AuthorizationID: "source"})
			}
			result, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": reply}})
			prefix := ""
			if scenario == "tool" {
				prefix = `{"event":"step_update","step_update":{"step_index":1,"step_type":"tool","state":"DONE","tool_name":"run_command"}}` + "\n"
			}
			script := "#!/bin/sh\nfor arg in \"$@\"; do [ \"$arg\" != \"--add-dir\" ] || exit 25; done\ncat <<'RESULT'\n" + prefix + string(result) + "\nRESULT\n"
			if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			installInboxSandboxStub(t, bin)
			t.Setenv("PATH", bin+":/usr/bin:/bin")
			s := &Server{cfg: &config.Config{Environment: "staging"}}
			ctx := context.WithValue(context.Background(), inboxRetryReconcileKey{}, true)
			attachment := filepath.Join(t.TempDir(), "receipt.jpg")
			if err := os.WriteFile(attachment, []byte("private receipt fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			out, failure := s.executeInboxCLI(ctx, "@cuan", "agy", "Saved facts", home, "", nil, attachment)
			if scenario == "answer" || scenario == "reconcile" {
				if out == "" || failure != "" {
					t.Fatal("fresh answer rejected", failure)
				}
			} else if failure == "" || out != "" {
				t.Fatal("uncertain activity replay was accepted", scenario)
			}
		})
	}
}

func TestInboxDirectRetryDeadlineIsIndependentOfFormatRepair(t *testing.T) {
	t.Setenv("INBOX_CLI_DIRECT_RECOVERY_TIMEOUT", "")
	t.Setenv("INBOX_DIRECT_RECOVERY_TIMEOUT", "")
	parent, stop := context.WithTimeout(context.Background(), 6*time.Minute)
	defer stop()
	ctx, cancel := inboxDirectReplyContext(parent)
	defer cancel()
	deadline, _ := ctx.Deadline()
	if left := time.Until(deadline); left < 124*time.Second || left > 125*time.Second {
		t.Fatal("fresh retry inherited formatter deadline", left)
	}
	short, shortCancel := context.WithTimeout(parent, time.Second)
	defer shortCancel()
	bounded, boundedCancel := inboxDirectReplyContext(short)
	defer boundedCancel()
	a, _ := short.Deadline()
	b, _ := bounded.Deadline()
	if a != b {
		t.Fatal("retry renewed the original deadline")
	}
}

func TestInboxReconciliationContextExcludesUnrelatedProjectBacklog(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	s.inboxStopping.Store(true)
	ctx := context.Background()
	roomID := s.resolveConversationContext(ctx, "agent_lifecycle", "").RoomID
	room, _ := s.projectStudioRepo.GetRoomByID(ctx, roomID)
	room.ProjectID = "bound-project"
	if err := s.projectStudioRepo.UpdateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}
	projects := project.NewMemoryStore()
	if err := projects.CreateProject(ctx, &project.Project{ID: "bound-project", Name: "Project"}); err != nil {
		t.Fatal(err)
	}
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	now := time.Now().UTC()
	q := db.InboxRequest{OwnerID: "owner", AgentID: "agent_lifecycle", RoomID: roomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), PayloadHash: "scope", State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: roomID, AuthorType: "user", Content: "ORIGINAL_APPROVED_REQUEST", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.Store().CreateTask(ctx, &task.Task{ID: "unrelated", ProjectID: "bound-project", SourceRoomID: "other-room", Title: "UNRELATED_PROJECT_TASK", Status: task.TaskStatusDoing, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	packet, err := s.inboxContext(ctx, q, true)
	if err != nil || strings.Contains(packet, "UNRELATED_PROJECT_TASK") || strings.Contains(packet, "Contact memory") || !strings.Contains(packet, "ORIGINAL_APPROVED_REQUEST") || !strings.Contains(packet, "An empty list means no task is recorded here") {
		t.Fatal("reconciliation mixed unrelated work", err, packet)
	}
}

func TestInboxReconciliationDelegatesOnceWithOriginalBinding(t *testing.T) {
	reconciliationDelegationContract(t, nil)
}

func TestInboxReconciliationDelegationPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) { reconciliationDelegationContract(t, database) })
}

func reconciliationDelegationContract(t *testing.T, database *sql.DB) {
	s, token, _, _ := inboxFixture(t)
	s.inboxStopping.Store(true)
	ctx := context.Background()
	if database != nil {
		if _, err := database.ExecContext(ctx, `INSERT INTO projects(id,name,repo_url) VALUES('default','Fixture','') ON CONFLICT DO NOTHING`); err != nil {
			t.Fatal(err)
		}
		s.projectStudioRepo = db.NewSQLProjectStudioRepository(database)
		s.taskSvc = task.NewService(task.NewDBStore(database), nil)
	}
	owner, err := s.authSvc.ValidateToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "").RoomID
	now := time.Now().UTC()
	source := db.InboxRequest{OwnerID: owner.ID, AgentID: "agent_lifecycle", RoomID: room, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), PayloadHash: "source", State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
	store := s.projectStudioRepo.(db.InboxLifecycleRepository)
	if _, _, err := store.AcceptInboxRequest(ctx, source, &db.ProjectMessage{ID: source.MessageID, RoomID: room, AuthorType: "user", Content: "Perform the approved verification and finish only the remaining work.", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	source.State = "interrupted"
	if err := store.FinishInboxRequest(ctx, source, nil); err != nil {
		t.Fatal(err)
	}
	retry, _, err := s.projectStudioRepo.(db.InboxRetryRepository).RetryInboxRequest(ctx, room, source.RequestID, source.OwnerID)
	if err != nil || !db.InboxRetryNeedsReconciliation(retry) {
		t.Fatal(err)
	}
	// Keep execution queued behind unrelated work so this test checks the
	// durable handoff without starting a native task process.
	if err := s.taskSvc.Store().CreateTask(ctx, &task.Task{ID: "occupied-migration", ProjectID: "default", Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode, Title: "Apply database schema migration"}); err != nil {
		t.Fatal(err)
	}
	d := inboxDelegation{Reconcile: true, ExecutionCategory: task.ExecutionCategoryOps, Title: "Verify original work", Description: "Check actual current state and finish only the remaining approved work.\n- [ ] Verified evidence", ProjectID: "default", AuthorizationID: source.MessageID}
	raw, _ := json.Marshal(d)
	content := "I will check current state first.\n```cuan-delegate\n" + string(raw) + "\n```"
	out := s.applyInboxDelegation(ctx, retry, content)
	if strings.Contains(out, "could not") || !strings.Contains(out, "View work") {
		t.Fatal("reconciliation did not hand off", out)
	}
	created, err := s.taskSvc.GetTaskDetail(ctx, "msg_"+source.MessageID)
	if err != nil || created.SourceMessageID != source.MessageID || created.SourceRoomID != room || created.ExecutionCategory != task.ExecutionCategoryOps || !strings.Contains(created.Instructions, inboxReconciliationExecutionContract) {
		t.Fatal("lost original scope or reconcile-first contract", err)
	}
	if created.LatestAttempt != nil || created.Launch == nil || created.Launch.State != "queued" {
		t.Fatalf("unexpected native execution: attempt=%v launch=%v", created.LatestAttempt != nil, created.Launch)
	}
	launch, err := s.taskSvc.LatestLaunch(ctx, created.ID)
	if err != nil || !strings.Contains(launch.Revision, inboxReconciliationExecutionContract) {
		t.Fatal("launch lost reconciliation precondition")
	}
	_ = s.applyInboxDelegation(ctx, retry, content)
	items, _ := s.taskSvc.ListTasks(ctx, task.TaskFilter{SourceRoomID: room})
	if len(items) != 1 {
		t.Fatal("retry created replacement tasks", len(items))
	}
	other := source
	other.RequestID, other.MessageID, other.ReplyID = db.GenerateUUID(), db.GenerateUUID(), db.GenerateUUID()
	other.CreatedAt = now.Add(-time.Minute)
	other.State = "queued"
	if _, _, err := store.AcceptInboxRequest(ctx, other, &db.ProjectMessage{ID: other.MessageID, RoomID: room, AuthorType: "user", Content: "An unrelated approved scope.", CreatedAt: other.CreatedAt}); err != nil {
		t.Fatal(err)
	}
	other.State = "completed"
	if err := store.FinishInboxRequest(ctx, other, nil); err != nil {
		t.Fatal(err)
	}
	otherTask := &task.Task{ID: "msg_" + other.MessageID, ProjectID: "default", SourceRoomID: room, SourceMessageID: other.MessageID, Title: "Other approved work", Status: task.TaskStatusBlocked, Instructions: "Original other scope"}
	if err := s.taskSvc.Store().CreateTask(ctx, otherTask); err != nil {
		t.Fatal(err)
	}
	for _, taskID := range []string{"", otherTask.ID} {
		wrong := d
		wrong.TaskID = taskID
		wrong.AuthorizationID = other.MessageID
		wrong.Revision = "Continue unrelated work"
		body, _ := json.Marshal(wrong)
		out := s.applyInboxDelegation(ctx, retry, "```cuan-delegate\n"+string(body)+"\n```")
		if !strings.Contains(out, "could not start") {
			t.Fatal("retry borrowed another scope", out)
		}
		untouched, _ := s.taskSvc.GetTaskDetail(ctx, otherTask.ID)
		if untouched.LatestAttempt != nil || untouched.Status != task.TaskStatusBlocked || untouched.Instructions != "Original other scope" {
			t.Fatal("unrelated task mutated")
		}
	}
	for _, status := range []string{task.TaskStatusDone, "cancelled", task.TaskStatusDoing} {
		item, _ := s.taskSvc.Store().GetTask(ctx, created.ID)
		item.Status = status
		if err := s.taskSvc.Store().UpdateTask(ctx, item); err != nil {
			t.Fatal(err)
		}
		_ = s.applyInboxDelegation(ctx, retry, content)
		after, _ := s.taskSvc.GetTaskDetail(ctx, created.ID)
		if after.Status != status || after.LatestAttempt != nil {
			t.Fatal("retry changed terminal/active work", status)
		}
	}
}

func TestInboxReconciliationRejectsTopicMutation(t *testing.T) {
	raw := `{"reply":"Saved state checked.","topic_title":"Changed topic","action":null}`
	if output, err := decodeInboxCoordinatorResponseForMode(raw, true, true); err == nil || output != "" {
		t.Fatal("retry changed the conversation topic")
	}
}
