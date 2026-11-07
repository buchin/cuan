package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestApprovedInitialPhaseScopesOnlyItsOwnOutcome(t *testing.T) {
	initialPhaseScopeContract(t)
}

func TestConversationalDelegationScopesSavedInitialPhase(t *testing.T) {
	initialPhaseScopeContractMode(t, true)
}

func TestApprovedInitialPhasePostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) { initialPhaseScopeContract(t, database) })
}

func initialPhaseScopeContract(t *testing.T, databases ...*sql.DB) {
	initialPhaseScopeContractMode(t, false, databases...)
}

func initialPhaseScopeContractMode(t *testing.T, conversational bool, databases ...*sql.DB) {
	t.Helper()
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	projects := project.NewMemoryStore()
	if err := projects.CreateProject(ctx, &project.Project{ID: "phase-project", Name: "Phase project"}); err != nil {
		t.Fatal(err)
	}
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	if len(databases) > 0 {
		database := databases[0]
		if _, err := database.ExecContext(ctx, `INSERT INTO projects(id,name,repo_url) VALUES('default','Default',''),('phase-project','Phase project','') ON CONFLICT DO NOTHING`); err != nil {
			t.Fatal(err)
		}
		s.projectStudioRepo = db.NewSQLProjectStudioRepository(database)
		s.taskSvc = task.NewService(task.NewDBStore(database), s.projectSvc)
	}
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "").RoomID
	if err := s.taskSvc.Store().CreateTask(ctx, &task.Task{ID: "busy", ProjectID: "phase-project", Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryOps}); err != nil {
		t.Fatal(err)
	}
	first := "Deliver validated source JSON for download without creating the summary"
	plan := inboxApprovalPlan{Title: "Two connected files", ProjectID: "phase-project", Description: "Standalone file work. Do not modify the repository. Do not push or deploy.\nPhase 1: " + first + " | Phase 2: Deliver a checked summary using the accepted source file\nAcceptance:\n- Deliver source JSON.\n- Deliver the derived summary."}
	if conversational {
		plan.Description = strings.Replace(plan.Description, "Standalone file work. Do not modify the repository. Do not push or deploy.", "Pengujian alur kerja mandiri dalam dua task/sesi terpisah. Berkas sebagai lampiran unduhan. Zero repository mutation (tidak mengubah kode, file repositori, atau konfigurasi), zero deployment (tidak membuat project baru, branch, commit, push, PR, deploy, atau mengirim pesan keluar).", 1)
	}
	now := time.Now().UTC()
	q := db.InboxRequest{OwnerID: "owner", AgentID: "agent_lifecycle", RoomID: room, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "completed", CreatedAt: now, Deadline: now.Add(time.Hour)}
	raw, _ := json.Marshal(attachmentMessage{Text: "Approved: Two connected files", Approval: &approvedInboxPlan{Action: "approve", MessageID: db.GenerateUUID(), Plan: plan}})
	content := attachmentEnvelope + string(raw)
	if conversational {
		proposal := db.InboxRequest{OwnerID: q.OwnerID, AgentID: q.AgentID, RoomID: room, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "completed", CreatedAt: now.Add(-time.Minute), Deadline: now.Add(time.Hour)}
		if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, proposal, &db.ProjectMessage{ID: proposal.MessageID, RoomID: room, AuthorID: "owner", AuthorType: "user", Content: "Propose two tasks", CreatedAt: proposal.CreatedAt}); err != nil {
			t.Fatal(err)
		}
		planJSON, _ := json.Marshal(plan)
		if err := s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{ID: proposal.ReplyID, RoomID: room, AuthorID: q.AgentID, AuthorType: "agent", Content: "```cuan-approval\n" + string(planJSON) + "\n```", CreatedAt: now.Add(-time.Second)}); err != nil {
			t.Fatal(err)
		}
		content = "Gas, approve the saved two-phase proposal and run both tasks separately."
	}
	if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: room, AuthorID: "owner", AuthorType: "user", Content: content, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if conversational {
		if !s.inboxProposalDecision(ctx, q) {
			t.Fatal("saved proposal reply did not select tool-free context")
		}
		foreign := q
		foreign.OwnerID = "different-owner"
		if s.inboxProposalDecision(ctx, foreign) {
			t.Fatal("foreign owner selected proposal context")
		}
	}
	delegation, _ := json.Marshal(inboxDelegation{Action: "run", Title: plan.Title, Description: plan.Description, ProjectID: plan.ProjectID, AuthorizationID: q.MessageID})
	if conversational {
		changed, _ := json.Marshal(inboxDelegation{Action: "run", Title: plan.Title, Description: plan.Description + "\nInvented extra work", ProjectID: plan.ProjectID, AuthorizationID: q.MessageID})
		out := s.applyInboxDelegation(ctx, q, "```cuan-delegate\n"+string(changed)+"\n```")
		if !strings.Contains(out, "differs from the approved plan") {
			t.Fatal("rewritten conversational scope admitted", out)
		}
		items, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{SourceRoomID: room})
		if err != nil || len(items) != 0 {
			t.Fatal("task created from rewritten proposal", items, err)
		}
	}
	for i := 0; i < 2; i++ {
		out := s.applyInboxDelegation(ctx, q, "```cuan-delegate\n"+string(delegation)+"\n```")
		d, err := s.taskSvc.GetTaskDetail(ctx, "msg_"+q.MessageID)
		if err != nil {
			t.Fatal(out, err)
		}
		if len(d.DoDChecklist) != 1 || d.DoDChecklist[0].Text != first || d.DoDChecklist[0].Completed || d.DoDChecklist[0].Evidence != "" {
			t.Fatalf("first task requires future outputs or inherited claims: %+v", d.DoDChecklist)
		}
		if !strings.Contains(d.Instructions, "Active Phase: 1") || !strings.Contains(d.Instructions, plan.Description) {
			t.Fatal("active phase or original plan lost", d.Instructions)
		}
		if approval, err := s.phaseApproval(ctx, d, q.OwnerID, room); err != nil || approval.Phase != 1 {
			t.Fatal("native initial approval not bound", approval, err)
		}
		bad := *d
		bad.DoDChecklist = nil
		if _, err := s.phaseApproval(ctx, &bad, q.OwnerID, room); err == nil {
			t.Fatal("initial outcome deletion accepted")
		}
		policy := executionpolicy.FromInstructions(d.Instructions)
		if d.ExecutionCategory != task.ExecutionCategoryOps || !policy.DenyPush || !policy.DenyDeploy {
			t.Fatal("phase lost original execution restrictions", d.ExecutionCategory, policy)
		}
	}
	items, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{SourceRoomID: room})
	if err != nil || len(items) != 1 {
		t.Fatal("initial phase replay duplicated tasks", len(items), err)
	}
}

func TestActivePhaseMarkerTakesPrecedenceOverDisplayTitle(t *testing.T) {
	s := &Server{}
	d := &task.TaskDetailResponse{Task: task.Task{Title: "Phase 99: User renamed display title", Instructions: "Active Phase: 2\nPhase 1: Data | Phase 2: API | Phase 3: Verified UI"}}
	next, ok := s.detectNextMultiPhase(context.Background(), d)
	if !ok || next.CurrentPhase != 2 || next.NextPhase != 3 || next.NextTitle != "Verified UI" {
		t.Fatal("display title controlled phase progress", next, ok)
	}
}

func TestLongApprovedOutcomeSurvivesTaskTitleLimit(t *testing.T) {
	outcome := strings.Repeat("Verifikasi hasil artefak dan identitas sumber yang sama; ", 7)
	s, q, parent := phaseLineageSetupWithPlan(t, inboxApprovalPlan{Title: "Data delivery", ProjectID: "default", Description: "Phase 1: Data | Phase 2: " + outcome})
	ctx := context.Background()
	if _, err := s.attemptInboxAutoDelegation(ctx, q, parent, "Phase accepted"); err != nil {
		t.Fatal(err)
	}
	child, err := s.taskSvc.GetTaskDetail(ctx, "msg_"+delegationMessageID("auto-phase:"+parent.ID+":2:input"))
	expected := strings.Trim(outcome, `"':;., `)
	if err != nil || len([]rune(child.Title)) > 200 || len(child.DoDChecklist) != 1 || child.DoDChecklist[0].Text != expected || !strings.Contains(child.Instructions, expected) {
		t.Fatal("display shortening lost approved outcome", child, err)
	}
	if child.DoDChecklist[0].Completed || child.DoDChecklist[0].Evidence != "" {
		t.Fatal("display shortening invented evidence")
	}
}

func TestReviewUsesValidatedActivePhaseWithoutDroppingFullPlan(t *testing.T) {
	s, origin, parent := phaseLineageSetup(t)
	ctx := context.Background()
	key := "task-review:" + parent.LatestAttempt.ID
	q := origin
	q.RequestID = delegationMessageID(key)
	q.MessageID = delegationMessageID(key + ":input")
	q.ReplyID = delegationMessageID(key + ":reply")
	if err := s.projectStudioRepo.CreateMessage(ctx, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "system", AuthorID: "cuan_work_review", Content: "Review /tasks/" + parent.ID, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	text, ok := s.inboxTaskReviewPrompt(ctx, q)
	var data map[string]json.RawMessage
	if !ok || json.Unmarshal([]byte(text), &data) != nil {
		t.Fatal("missing phase review", text)
	}
	var phase struct {
		Phase   int    `json:"phase"`
		Outcome string `json:"outcome"`
		Total   int    `json:"total_phases"`
		Plan    string `json:"approved_plan"`
	}
	if err := json.Unmarshal(data["approved_phase_context"], &phase); err != nil || phase.Phase != 1 || phase.Outcome != "Data" || phase.Total != 3 {
		t.Fatal("missing validated active boundary", phase, err)
	}
	if !strings.Contains(phase.Plan, "Phase 2: API") || strings.Contains(phase.Plan, "INVENTED SCOPE") {
		t.Fatal("original approved plan dropped or replaced by task prose", phase.Plan)
	}
	unbound := s.inboxReviewData(ctx, parent)
	if json.Unmarshal([]byte(unbound), &data) != nil || string(data["approved_phase_context"]) != "null" {
		t.Fatal("phase inferred from display/prose", unbound)
	}
}
