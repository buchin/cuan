package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestDeployCoordinator_SingleExecution(t *testing.T) {
	c := NewDeployCoordinator()

	var executed atomic.Int32
	var capturedJob DeployJob
	done := make(chan struct{})

	c.SetExecutor(func(ctx context.Context, job DeployJob) error {
		executed.Add(1)
		capturedJob = job
		close(done)
		return nil
	})

	c.Enqueue(context.Background(), DeployJob{
		ProjectID: "proj_test_1",
		TaskID:    "task_1",
		CommitSHA: "sha_111",
	})

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for deploy leader to execute")
	}

	if executed.Load() != 1 {
		t.Fatalf("expected 1 execution, got %d", executed.Load())
	}
	if capturedJob.CommitSHA != "sha_111" {
		t.Fatalf("expected sha_111, got %s", capturedJob.CommitSHA)
	}

	// Coordinator should settle to not busy
	time.Sleep(50 * time.Millisecond)
	if c.IsBusy("proj_test_1") {
		t.Fatal("expected coordinator to be idle after execution")
	}
}

func TestDeployCoordinator_Coalescing(t *testing.T) {
	c := NewDeployCoordinator()

	job1Started := make(chan struct{})
	job1CanFinish := make(chan struct{})
	var executions []DeployJob
	var mu sync.Mutex

	c.SetExecutor(func(ctx context.Context, job DeployJob) error {
		mu.Lock()
		executions = append(executions, job)
		count := len(executions)
		mu.Unlock()

		if count == 1 {
			close(job1Started)
			<-job1CanFinish
		}
		return nil
	})

	// 1. Enqueue first job (becomes leader and pauses)
	c.Enqueue(context.Background(), DeployJob{
		ProjectID: "proj_coalesce",
		TaskID:    "task_1",
		CommitSHA: "sha_1",
	})

	<-job1Started

	if !c.IsBusy("proj_coalesce") {
		t.Fatal("expected coordinator to be busy while job 1 runs")
	}

	// 2. While job 1 is running, enqueue jobs 2, 3, 4
	c.Enqueue(context.Background(), DeployJob{
		ProjectID: "proj_coalesce",
		TaskID:    "task_2",
		CommitSHA: "sha_2",
	})
	c.Enqueue(context.Background(), DeployJob{
		ProjectID: "proj_coalesce",
		TaskID:    "task_3",
		CommitSHA: "sha_3",
	})
	c.Enqueue(context.Background(), DeployJob{
		ProjectID: "proj_coalesce",
		TaskID:    "task_4",
		CommitSHA: "sha_4",
	})

	if count := c.PendingCount("proj_coalesce"); count != 3 {
		t.Fatalf("expected 3 coalesced jobs, got %d", count)
	}

	// 3. Release job 1 so it finishes
	close(job1CanFinish)

	// Wait for coordinator to settle
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should have executed EXACTLY 2 times: job 1, then the coalesced follow-up job 4!
	if len(executions) != 2 {
		t.Fatalf("expected exactly 2 executions (job 1 + coalesced), got %d", len(executions))
	}

	if executions[0].CommitSHA != "sha_1" {
		t.Errorf("job 1 SHA mismatch: %s", executions[0].CommitSHA)
	}
	if executions[1].CommitSHA != "sha_4" {
		t.Errorf("coalesced follow-up should have latest SHA (sha_4), got %s", executions[1].CommitSHA)
	}
	if executions[1].TaskID != "task_4" {
		t.Errorf("coalesced follow-up should have latest TaskID (task_4), got %s", executions[1].TaskID)
	}
	if executions[1].CoalescedCount != 3 {
		t.Errorf("expected CoalescedCount 3, got %d", executions[1].CoalescedCount)
	}
}

func TestDeployCoordinator_ErrorHandlingDoesNotWedge(t *testing.T) {
	c := NewDeployCoordinator()

	var attempts atomic.Int32
	c.SetExecutor(func(ctx context.Context, job DeployJob) error {
		if attempts.Add(1) == 1 {
			return errors.New("simulated transient build failure")
		}
		return nil
	})

	c.Enqueue(context.Background(), DeployJob{
		ProjectID: "proj_err",
		TaskID:    "task_err_1",
	})

	time.Sleep(100 * time.Millisecond)

	// Second job should execute without being wedged
	c.Enqueue(context.Background(), DeployJob{
		ProjectID: "proj_err",
		TaskID:    "task_err_2",
	})

	time.Sleep(100 * time.Millisecond)

	if attempts.Load() != 2 {
		t.Fatalf("expected 2 attempts despite prior error, got %d", attempts.Load())
	}
}

func TestDeployCoordinator_MultipleProjectsAreIndependent(t *testing.T) {
	c := NewDeployCoordinator()

	var executedProjects sync.Map
	var wg sync.WaitGroup
	wg.Add(2)

	c.SetExecutor(func(ctx context.Context, job DeployJob) error {
		executedProjects.Store(job.ProjectID, true)
		wg.Done()
		return nil
	})

	c.Enqueue(context.Background(), DeployJob{ProjectID: "proj_A", TaskID: "task_A"})
	c.Enqueue(context.Background(), DeployJob{ProjectID: "proj_B", TaskID: "task_B"})

	wg.Wait()

	if _, ok := executedProjects.Load("proj_A"); !ok {
		t.Fatal("proj_A did not execute")
	}
	if _, ok := executedProjects.Load("proj_B"); !ok {
		t.Fatal("proj_B did not execute")
	}
}

func TestDeployCoordinatorSuccessAndFailureLifecycle(t *testing.T) {
	s, _ := setupTestChatServer(t)
	ctx := context.Background()

	projects := project.NewMemoryStore()
	p := &project.Project{ID: "proj_lp", Name: "cuan-landing", RepoName: "cuan-landing", DefaultBranch: "main"}
	if err := projects.CreateProject(ctx, p); err != nil {
		t.Fatal(err)
	}
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	depRepo := cuanDB.NewMemoryProjectDeploymentRepository()
	s.projectDeploymentRepo = depRepo

	commitOK := strings.Repeat("a", 40)
	tOK := &task.Task{
		ID:           "task-deploy-ok",
		ProjectID:    p.ID,
		Title:        "Deploy OK",
		Status:       task.TaskStatusReview,
		CommitSHA:    commitOK,
		CompleteNote: "🚀 Perubahan telah berhasil diterapkan (Commit " + commitOK + "). Verifikasi deployment production belum dikonfirmasi.",
	}
	if err := s.taskSvc.Store().CreateTask(ctx, tOK); err != nil {
		t.Fatal(err)
	}
	started := time.Now().Add(-time.Minute)
	attOK := &task.TaskAttempt{ID: "att-ok", TaskID: tOK.ID, RunnerID: "r1", Status: task.AttemptStatusSucceeded, CreatedAt: started, StartedAt: &started}
	if err := s.taskSvc.Store().CreateAttempt(ctx, attOK); err != nil {
		t.Fatal(err)
	}

	s.handleDeployCoordinatorSuccess(ctx, DeployJob{
		ProjectID:  p.ID,
		TaskID:     tOK.ID,
		Branch:     "main",
		CommitSHA:  commitOK,
		EnqueuedAt: started,
	})

	gotOK, err := s.taskSvc.GetTaskDetail(ctx, tOK.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotOK.Status != task.TaskStatusDone {
		t.Fatalf("expected task status done after deploy success, got %s", gotOK.Status)
	}
	if strings.Contains(gotOK.CompleteNote, "Verifikasi deployment production belum dikonfirmasi") {
		t.Fatalf("expected pending verification note to be cleared, got %q", gotOK.CompleteNote)
	}
	if !strings.Contains(gotOK.CompleteNote, "telah diverifikasi aktif pada rilis live") {
		t.Fatalf("expected live verification marker in CompleteNote, got %q", gotOK.CompleteNote)
	}
	if !task.VerifiedDeployment(gotOK.Activities, gotOK.LatestAttempt, commitOK, time.Now().UTC()) {
		t.Fatal("expected deployment_verified activity to be recorded on deploy success")
	}
	depOK, err := depRepo.GetByID(ctx, "dep_"+commitOK[:16])
	if err != nil || depOK == nil || depOK.Status != cuanDB.DeploymentStatusSuccess {
		t.Fatalf("expected success ProjectDeployment record, got %+v (err=%v)", depOK, err)
	}

	commitFail := strings.Repeat("b", 40)
	tFail := &task.Task{
		ID:        "task-deploy-fail",
		ProjectID: p.ID,
		Title:     "Deploy Fail",
		Status:    task.TaskStatusReview,
		CommitSHA: commitFail,
	}
	if err := s.taskSvc.Store().CreateTask(ctx, tFail); err != nil {
		t.Fatal(err)
	}
	s.handleDeployCoordinatorFailure(ctx, DeployJob{
		ProjectID:  p.ID,
		TaskID:     tFail.ID,
		Branch:     "main",
		CommitSHA:  commitFail,
		EnqueuedAt: started,
	}, errors.New("wrangler exit 1"))

	gotFail, err := s.taskSvc.GetTaskDetail(ctx, tFail.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotFail.Status != task.TaskStatusBlocked {
		t.Fatalf("expected task status blocked after deploy failure, got %s", gotFail.Status)
	}
	if !strings.Contains(gotFail.CompleteNote, "Deploy production gagal") {
		t.Fatalf("expected failure note in CompleteNote, got %q", gotFail.CompleteNote)
	}
	depFail, err := depRepo.GetByID(ctx, "dep_"+commitFail[:16])
	if err != nil || depFail == nil || depFail.Status != cuanDB.DeploymentStatusFailed {
		t.Fatalf("expected failed ProjectDeployment record, got %+v (err=%v)", depFail, err)
	}
}

func TestApplyInboxReviewEvaluation_AutoApplyPendingReleaseForDeployScript(t *testing.T) {
	s, _ := setupTestChatServer(t)
	ctx := context.Background()

	projects := project.NewMemoryStore()
	autoDeploy := true
	meta, _ := json.Marshal(map[string]any{
		"cuan_settings": project.Settings{
			AutoDeploy:   &autoDeploy,
			DeployBranch: "main",
			DeployScript: "./scripts/deploy.sh",
		},
	})
	p := &project.Project{ID: "proj_lp_test", Name: "cuan-landing", RepoName: "cuan-landing", DefaultBranch: "main", Metadata: meta}
	if err := projects.CreateProject(ctx, p); err != nil {
		t.Fatal(err)
	}
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())

	room := &cuanDB.ProjectRoom{ID: "11111111-1111-4111-8111-111111111111", ProjectID: p.ID, Title: "Test LP Room"}
	if err := s.projectStudioRepo.CreateRoom(ctx, room); err != nil {
		t.Fatal(err)
	}
	ownerMsg := &cuanDB.ProjectMessage{ID: "22222222-2222-4222-8222-222222222222", RoomID: room.ID, AuthorType: "user", AuthorID: "owner", Content: "Approved: fix LP"}
	if err := s.projectStudioRepo.CreateMessage(ctx, ownerMsg); err != nil {
		t.Fatal(err)
	}

	commit := strings.Repeat("e", 40)
	tk, err := s.taskSvc.CreateTaskFromMessage(ctx, "owner", ownerMsg.ID, task.CreateTaskRequest{
		SourceRoomID: room.ID,
		ProjectID:    p.ID,
		Title:        "Fix LP Hero",
		Instructions: "Cuan managed delegation v1.\nApproval message: " + ownerMsg.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	att, err := s.taskSvc.CreateAttempt(ctx, tk.ID, "owner", task.CreateAttemptRequest{RunnerID: "r1", AgentName: "agent_cuan"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.taskSvc.UpdateAttemptStatus(ctx, att.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusSucceeded}); err != nil {
		t.Fatal(err)
	}
	reviewStatus := task.TaskStatusReview
	branch := "task/" + tk.ID
	prNum := 6
	if _, err := s.taskSvc.UpdateTask(ctx, tk.ID, "runner", task.UpdateTaskRequest{Status: &reviewStatus, CommitSHA: &commit, BranchName: &branch, PRNumber: &prNum}); err != nil {
		t.Fatal(err)
	}

	key := "task-review:" + att.ID
	reqID := delegationMessageID(key)
	inputMsgID := delegationMessageID(key + ":input")
	replyMsgID := delegationMessageID(key + ":reply")
	reviewMsg := &cuanDB.ProjectMessage{
		ID:         inputMsgID,
		RoomID:     room.ID,
		AuthorType: "system",
		AuthorID:   "cuan_work_review",
		Content:    "Review result for [/tasks/" + tk.ID + "]",
	}
	if err := s.projectStudioRepo.CreateMessage(ctx, reviewMsg); err != nil {
		t.Fatal(err)
	}

	q := cuanDB.InboxRequest{
		OwnerID:   "owner",
		AgentID:   "agent_cuan",
		RoomID:    room.ID,
		RequestID: reqID,
		MessageID: inputMsgID,
		ReplyID:   replyMsgID,
		CreatedAt: time.Now().UTC(),
		Deadline:  time.Now().UTC().Add(10 * time.Minute),
	}
	raw := `{"decision":"revise","source_review":"not_checked","requested_outcome":"unfinished","remaining_work":"verification","finding":"Bukti rilis hasil pekerjaan belum lengkap.","explanation":"Bukti rilis hasil pekerjaan belum lengkap.","revision":"Publish to cuan.work"}`
	out := s.applyInboxReviewEvaluation(ctx, q, raw)
	t.Logf("applyInboxReviewEvaluation output: %s", out)
	if strings.Contains(out, "Bukti rilis hasil pekerjaan belum lengkap") {
		t.Fatalf("expected autoApplyAndDeployTask to be attempted, but got: %s", out)
	}
}
