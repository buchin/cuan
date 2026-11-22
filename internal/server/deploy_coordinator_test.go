package server

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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
