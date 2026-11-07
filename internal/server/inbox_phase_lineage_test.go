package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/task"
)

func phaseLineageSetup(t *testing.T, databases ...*sql.DB) (*Server, db.InboxRequest, *task.TaskDetailResponse) {
	t.Helper()
	return phaseLineageSetupWithPlan(t, inboxApprovalPlan{Title: "Phase 1: Data", Description: "Phase 1: Data | Phase 2: API | Phase 3: UI", ProjectID: "default"}, databases...)
}

func phaseLineageSetupWithPlan(t *testing.T, plan inboxApprovalPlan, databases ...*sql.DB) (*Server, db.InboxRequest, *task.TaskDetailResponse) {
	t.Helper()
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	if len(databases) > 0 {
		database := databases[0]
		if _, err := database.ExecContext(ctx, `INSERT INTO projects(id,name,repo_url) VALUES('default','Fixture','') ON CONFLICT DO NOTHING`); err != nil {
			t.Fatal(err)
		}
		s.projectStudioRepo = db.NewSQLProjectStudioRepository(database)
		s.taskSvc = task.NewService(task.NewDBStore(database), s.projectSvc)
	}
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "").RoomID
	now := time.Now().UTC()
	q := db.InboxRequest{OwnerID: "owner", AgentID: "agent_lifecycle", RoomID: room, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "completed", CreatedAt: now, Deadline: now.Add(time.Hour)}
	raw, _ := json.Marshal(attachmentMessage{Approval: &approvedInboxPlan{Action: "approve", MessageID: db.GenerateUUID(), Plan: plan}})
	message := &db.ProjectMessage{ID: q.MessageID, RoomID: room, AuthorType: "user", AuthorID: "owner", Content: attachmentEnvelope + string(raw), CreatedAt: now, UpdatedAt: now}
	if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, message); err != nil {
		t.Fatal(err)
	}
	parent, err := s.taskSvc.CreateTaskFromMessage(ctx, "owner", q.MessageID, task.CreateTaskRequest{ProjectID: "default", SourceRoomID: room, Title: "Agent-refined title", Instructions: "Active Phase: 1\nPhase 1: Data | Phase 2: INVENTED SCOPE | Phase 3: UI\nDo not push. Do not deploy.", AssignedAgent: q.AgentID})
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := s.taskSvc.CreateAttempt(ctx, parent.ID, "owner", task.CreateAttemptRequest{RunnerID: "local_terminal", AgentName: q.AgentID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.taskSvc.UpdateAttemptStatus(ctx, attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusSucceeded}); err != nil {
		t.Fatal(err)
	}
	saved, _ := s.taskSvc.Store().GetTask(ctx, parent.ID)
	saved.Status = task.TaskStatusDone
	if err = s.taskSvc.Store().UpdateTask(ctx, saved); err != nil {
		t.Fatal(err)
	}
	// Keep execution queued so this test exercises durable authorization without
	// starting an external provider or relying on an in-process grant.
	if err = s.taskSvc.Store().CreateTask(ctx, &task.Task{ID: "phase-blocker", ProjectID: "default", Title: "Other approved work", Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
		t.Fatal(err)
	}
	detail, err := s.taskSvc.GetTaskDetail(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s, q, detail
}

func phaseLineageFixture(t *testing.T, databases ...*sql.DB) (*Server, db.InboxRequest, *task.TaskDetailResponse, *task.TaskDetailResponse, *task.TaskLaunch) {
	t.Helper()
	s, q, detail := phaseLineageSetup(t, databases...)
	ctx := context.Background()
	parent := &detail.Task
	text := s.enqueueInboxAutoDelegation(ctx, q, detail, "Phase accepted")
	if strings.Contains(text, "without stopping") || !strings.Contains(text, "Phase 2: API") || strings.Contains(text, "INVENTED SCOPE") {
		t.Fatal("untruthful or unapproved phase result", text)
	}
	inputID := delegationMessageID("auto-phase:" + parent.ID + ":2:input")
	child, err := s.taskSvc.GetTaskDetail(ctx, "msg_"+inputID)
	if err != nil {
		t.Fatal(err)
	}
	request := delegationMessageID("auto-phase:" + parent.ID + ":2")
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(child.ID, request))
	if err != nil {
		t.Fatal(err)
	}
	if launch.State != "queued" || launch.SourceRequestID != q.RequestID {
		t.Fatal("phase launch lost original approval", launch.State)
	}
	return s, q, detail, child, launch
}

func TestApprovedPhaseLaunchSurvivesRestartAndReplay(t *testing.T) {
	s, q, parent, child, launch := phaseLineageFixture(t)
	ctx := context.Background()
	if got := executionpolicy.FromInstructions(child.Instructions); !got.DenyPush || !got.DenyDeploy {
		t.Fatal("phase relaxed publication limits", got)
	}
	restarted := &Server{taskSvc: s.taskSvc, projectStudioRepo: s.projectStudioRepo, projectSvc: s.projectSvc}
	_, ok, err := restarted.authorizeSavedTaskLaunch(ctx, *launch, child)
	if err != nil || !ok {
		t.Fatal("durable phase approval was rejected", err)
	}
	_ = s.enqueueInboxAutoDelegation(ctx, q, parent, "Phase accepted")
	rows, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{SourceRoomID: q.RoomID})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatal("phase replay created duplicate tasks", len(rows))
	}
	after, err := s.taskSvc.GetTaskDetail(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Attempts) != 0 {
		t.Fatal("queued phase executed during authorization test")
	}
	if strings.Count(after.Instructions, "Project guidance snapshot:") != 1 {
		t.Fatal("phase nested project snapshots")
	}
}

func TestApprovedPhaseThreeKeepsOriginalApprovalAndParentOrder(t *testing.T) {
	s, q, root, child, _ := phaseLineageFixture(t)
	ctx := context.Background()
	attempt := &task.TaskAttempt{ID: "phase-two-result", TaskID: child.ID, RunnerID: "local_terminal", Status: task.AttemptStatusSucceeded, CreatedAt: time.Now().UTC()}
	if err := s.taskSvc.Store().CreateAttempt(ctx, attempt); err != nil {
		t.Fatal(err)
	}
	row, _ := s.taskSvc.Store().GetTask(ctx, child.ID)
	row.Status = task.TaskStatusDone
	row.Title = "Agent changed this display title to Phase 99"
	if err := s.taskSvc.Store().UpdateTask(ctx, row); err != nil {
		t.Fatal(err)
	}
	s.taskSvc.FinishTaskLaunches(ctx, child.ID, "stopped")
	accepted, err := s.taskSvc.GetTaskDetail(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	text := s.enqueueInboxAutoDelegation(ctx, q, accepted, "Phase accepted")
	if !strings.Contains(text, "Phase 3: UI") || strings.Contains(text, "Phase 100") {
		t.Fatal("display metadata changed the approved phase order", text)
	}
	input := delegationMessageID("auto-phase:" + root.ID + ":3:input")
	third, err := s.taskSvc.GetTaskDetail(ctx, "msg_"+input)
	if err != nil {
		t.Fatal(err)
	}
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(third.ID, delegationMessageID("auto-phase:"+root.ID+":3")))
	if err != nil {
		t.Fatal(err)
	}
	_, authorized, err := s.authorizeSavedTaskLaunch(ctx, *launch, third)
	if err != nil || !authorized || launch.SourceRequestID != q.RequestID {
		t.Fatal("third phase lost original authority", err)
	}
}

func TestApprovedPhaseRejectsChangedAuthorization(t *testing.T) {
	for _, scenario := range []string{"owner", "project", "source request", "child scope", "child outcome removed", "child outcome replaced", "execution lane", "parent cancelled", "receipt author", "receipt step", "plan changed"} {
		t.Run(scenario, func(t *testing.T) {
			s, q, parent, child, launch := phaseLineageFixture(t)
			ctx := context.Background()
			switch scenario {
			case "owner":
				launch.OwnerID = "other"
			case "project":
				launch.ProjectID = "other"
			case "source request":
				launch.SourceRequestID = "other"
			case "child scope":
				child.Instructions += "\nExpand the project"
			case "child outcome removed":
				child.DoDChecklist = nil
			case "child outcome replaced":
				child.DoDChecklist[0].Text = "Unapproved different outcome"
			case "execution lane":
				child.ExecutionCategory = task.ExecutionCategoryReadOnly
			case "parent cancelled":
				row, _ := s.taskSvc.Store().GetTask(ctx, parent.ID)
				row.Status = "cancelled"
				s.taskSvc.Store().UpdateTask(ctx, row)
			case "receipt author", "receipt step":
				for _, event := range child.Activities {
					if event.ID == phaseReceiptID(child.ID) {
						if scenario == "receipt author" {
							event.Author = "agent"
						} else {
							var payload phaseLineage
							json.Unmarshal([]byte(event.Message), &payload)
							payload.Phase = 3
							raw, _ := json.Marshal(payload)
							event.Message = string(raw)
						}
					}
				}
			case "plan changed":
				message, _ := s.projectStudioRepo.GetMessageByID(ctx, q.MessageID)
				message.Content = strings.Replace(message.Content, "Phase 2: API", "Phase 2: Different", 1)
				s.projectStudioRepo.UpdateMessage(ctx, message)
			}
			_, ok, err := s.authorizeSavedTaskLaunch(ctx, *launch, child)
			if ok {
				t.Fatal("changed phase approval accepted")
			}
			if err != nil {
				t.Fatal("invalid scope incorrectly classified as transient storage failure", err)
			}
		})
	}
}

// A transient lookup must remain distinguishable from rejected consent without
// parsing the user-facing explanation. No child may be created on either path.
type phaseLookupFailureRepository struct {
	db.ProjectStudioRepository
	failure error
}

func (r phaseLookupFailureRepository) GetMessageByID(context.Context, string) (*db.ProjectMessage, error) {
	return nil, r.failure
}
func TestApprovedPhaseHandoffReturnsTypedFailure(t *testing.T) {
	s, q, parent, _, _ := phaseLineageFixture(t)
	original := s.projectStudioRepo
	outage := errors.New("temporary storage failure")
	s.projectStudioRepo = phaseLookupFailureRepository{ProjectStudioRepository: original, failure: outage}
	_, err := s.attemptInboxAutoDelegation(context.Background(), q, parent, "Phase accepted")
	if !errors.Is(err, outage) {
		t.Fatalf("lost persistence failure: %v", err)
	}
	var denied *phaseAuthorizationError
	if errors.As(err, &denied) {
		t.Fatal("temporary outage classified as rejected approval")
	}
	s.projectStudioRepo = original
	_, err = s.attemptInboxAutoDelegation(context.Background(), db.InboxRequest{OwnerID: "other", RoomID: q.RoomID}, parent, "Phase accepted")
	if !errors.As(err, &denied) {
		t.Fatalf("changed owner did not return authorization failure: %v", err)
	}
	text, err := s.attemptInboxAutoDelegation(context.Background(), q, parent, "Phase accepted")
	if err != nil || !strings.Contains(text, "Phase 2: API") {
		t.Fatalf("valid replay failed: %v %s", err, text)
	}
}

func TestAutoPhaseRequiresFreshEvidenceForApprovedOutcome(t *testing.T) {
	s, q, parent := phaseLineageSetup(t)
	ctx := context.Background()
	row, err := s.taskSvc.Store().GetTask(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	row.DoDChecklist = []task.DoDItem{{Text: "Prior phase data outcome", Completed: true, Evidence: "Old phase proof"}}
	if err = s.taskSvc.Store().UpdateTask(ctx, row); err != nil {
		t.Fatal(err)
	}
	parent, err = s.taskSvc.GetTaskDetail(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = s.attemptInboxAutoDelegation(ctx, q, parent, "Phase accepted"); err != nil {
			t.Fatal(err)
		}
		input := delegationMessageID("auto-phase:" + parent.ID + ":2:input")
		child, err := s.taskSvc.GetTaskDetail(ctx, "msg_"+input)
		if err != nil {
			t.Fatal(err)
		}
		if len(child.DoDChecklist) != 1 || child.DoDChecklist[0].Text != "API" || child.DoDChecklist[0].Completed || child.DoDChecklist[0].Evidence != "" {
			t.Fatalf("next approved outcome lost or inherited claims: %+v", child.DoDChecklist)
		}
		if len(task.AcceptanceGaps(&child.Task)) == 0 {
			t.Fatal("new phase accepted without current evidence")
		}
	}
}
