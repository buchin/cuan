package task

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestTaskPageLaunchMemoryContract(t *testing.T) {
	taskPageLaunchContract(t, NewMemoryStore(), "launch-project")
}

// A recovery launch exists before a replacement attempt. Lists, counts and detail
// must agree during that interval without rewriting the failed attempt or task.
func taskPageLaunchContract(t *testing.T, store Store, project string) {
	t.Helper()
	ctx := context.Background()
	svc := NewService(store, nil)
	launches := store.(LaunchStore)
	now := time.Now().UTC().Truncate(time.Microsecond)
	for n, c := range []struct{ stored, attempt, launch, want, group string }{
		{"review", "failed", "queued", "queued", "work_active"},
		{"review", "canceled", "claimed", "queued", "work_active"},
		{"review", "succeeded", "stopped", "blocked", "attention"},
		{"review", "succeeded", "cancelled", "canceled", "dibatalkan_review"},
		{"done", "failed", "queued", "done", "completed"},
		{"review", "running", "started", "doing", "work_active"},
	} {
		id := fmt.Sprintf("page_launch_%d", n)
		if err := store.CreateTask(ctx, &Task{ID: id, ProjectID: project, Title: id, Instructions: "Approved scope", Status: c.stored, Priority: "normal", CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		if err := store.CreateAttempt(ctx, &TaskAttempt{ID: id + "_attempt", TaskID: id, AgentName: "agent", RunnerID: "fixture", Status: c.attempt, CreatedAt: now.Add(-time.Minute)}); err != nil {
			t.Fatal(err)
		}
		j, err := launches.SaveLaunch(ctx, TaskLaunch{TaskID: id, ProjectID: project, RequestID: id + "_request", OwnerID: "owner", RoomID: "91000000-0000-4000-8000-000000000001", SourceRequestID: "approved", AgentID: "agent", InstructionsHash: "scope", CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		if c.launch != "queued" {
			claimed, err := launches.ClaimLaunch(ctx, now, "fixture", j.ID)
			if err != nil || claimed == nil {
				t.Fatalf("claim: %v", err)
			}
			if c.launch == "cancelled" {
				if _, err := launches.CancelLaunches(ctx, id, now); err != nil {
					t.Fatal(err)
				}
			} else if c.launch != "claimed" {
				if err := launches.FinishLaunch(ctx, j.ID, claimed.LeaseToken, c.launch, now, ""); err != nil {
					t.Fatal(err)
				}
			}
		}
		for _, filter := range []string{c.want, c.group} {
			page, err := svc.ListTaskPage(ctx, PageOptions{ProjectID: project, Search: id, Status: filter})
			if err != nil {
				t.Fatal(err)
			}
			if page.Total != 1 || len(page.Tasks) != 1 {
				t.Fatalf("%s/%s omitted: %+v", id, filter, page)
			}
			row := page.Tasks[0]
			if row.Launch == nil || row.Launch.State != c.launch {
				t.Fatalf("%s lost launch: %+v", id, row.Launch)
			}
			if row.Status != c.want || row.LatestAttempt.Status != c.attempt {
				t.Fatal("preview status or original attempt differs")
			}
			if page.Counts[c.want] < 1 {
				t.Fatalf("count disagrees: %v", page.Counts)
			}
		}
		detail, err := svc.GetTaskDetail(ctx, id)
		if err != nil || detail.Launch == nil || detail.Launch.State != c.launch || detail.Status != c.stored {
			t.Fatalf("detail differs: %v", err)
		}
	}
}

type pageLaunchCountingStore struct {
	*MemoryStore
	calls, ids int
}

func (s *pageLaunchCountingStore) LatestLaunchInfos(ctx context.Context, ids []string) (map[string]*LaunchInfo, error) {
	s.calls++
	s.ids += len(ids)
	return s.MemoryStore.LatestLaunchInfos(ctx, ids)
}
func TestTaskPageLaunchReadIsBounded(t *testing.T) {
	store := &pageLaunchCountingStore{MemoryStore: NewMemoryStore()}
	ctx := context.Background()
	now := time.Now()
	for i := 0; i < 1000; i++ {
		if err := store.CreateTask(ctx, &Task{ID: fmt.Sprint("bounded_", i), Title: "Work", Status: "todo", CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	page, err := NewService(store, nil).ListTaskPage(ctx, PageOptions{PageSize: 25})
	if err != nil || len(page.Tasks) != 25 || store.calls != 1 || store.ids != 25 {
		t.Fatalf("launch read was not bounded: calls=%d ids=%d err=%v", store.calls, store.ids, err)
	}
}

func TestActiveExecutionLaunchPresentationMemory(t *testing.T) {
	activeExecutionLaunchPresentationContract(t, NewMemoryStore(), "active-launch-project")
}

// Presentation follows the exact active execution, while raw latest-launch
// diagnostics and failed history remain available to recovery policy.
func activeExecutionLaunchPresentationContract(t *testing.T, store Store, project string) {
	t.Helper()
	ctx := context.Background()
	svc := NewService(store, nil)
	now := time.Now().UTC().Truncate(time.Microsecond)
	for _, state := range []string{AttemptStatusQueued, AttemptStatusRunning, AttemptStatusFailed} {
		id := "active_launch_" + state
		item := &Task{ID: id, ProjectID: project, Title: id, Instructions: "Original approved scope", Status: TaskStatusQueued, Priority: "normal", CreatedAt: now, UpdatedAt: now}
		if err := store.CreateTask(ctx, item); err != nil {
			t.Fatal(err)
		}
		save := func(request string, at time.Time) *TaskLaunch {
			t.Helper()
			j, err := svc.SaveLaunch(ctx, TaskLaunch{TaskID: id, ProjectID: project, RequestID: request, OwnerID: "owner", RoomID: "91000000-0000-4000-8000-000000000001", SourceRequestID: "approved", AgentID: "agent", InstructionsHash: LaunchInstructionsHash(item.Instructions), CreatedAt: at, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
			if err != nil {
				t.Fatal(err)
			}
			return j
		}
		original := save(id+"_original", now.Add(-2*time.Second))
		leased, err := svc.ClaimLaunch(ctx, now, "original", original.ID)
		if err != nil || leased == nil {
			t.Fatalf("original claim: %v", err)
		}
		a := &TaskAttempt{ID: attemptRequestID(id, original.RequestID), TaskID: id, AgentName: "agent", RunnerID: "fixture", Status: state, CreatedAt: now}
		if err := store.CreateAttempt(ctx, a); err != nil {
			t.Fatal(err)
		}
		if err := svc.FinishLaunch(ctx, original.ID, leased.LeaseToken, "started", now, ""); err != nil {
			t.Fatal(err)
		}
		duplicate := save(id+"_duplicate", now.Add(-time.Second))
		// A competing queued request must not turn a running execution back
		// into Queued in filters or scope counts.
		if state == AttemptStatusRunning {
			activePage, err := svc.ListTaskPage(ctx, PageOptions{ProjectID: project, Search: id, Status: "doing"})
			if err != nil || activePage.Total != 1 || len(activePage.Tasks) != 1 || activePage.Counts["doing"] < 1 || activePage.Tasks[0].Launch.State != "started" {
				t.Fatalf("queued duplicate overrode running execution: %+v %v", activePage, err)
			}
		}
		lease, err := svc.ClaimLaunch(ctx, now, "duplicate", duplicate.ID)
		if err != nil || lease == nil {
			t.Fatalf("duplicate claim: %v", err)
		}
		diagnostic := "The saved inspection is no longer current. Existing results and work were preserved."
		if err := svc.FinishLaunch(ctx, duplicate.ID, lease.LeaseToken, "stopped", now, diagnostic); err != nil {
			t.Fatal(err)
		}
		want := "started"
		if state == AttemptStatusFailed {
			want = "stopped"
		}
		detail, err := svc.GetTaskDetail(ctx, id)
		if err != nil || detail.Launch == nil || detail.Launch.State != want {
			t.Fatalf("detail %s: %+v %v", state, detail, err)
		}
		if detail.LatestAttempt.ID != a.ID || detail.LatestAttempt.Status != state {
			t.Fatal("presentation changed execution")
		}
		page, err := svc.ListTaskPage(ctx, PageOptions{ProjectID: project, Search: id})
		if err != nil || len(page.Tasks) != 1 || page.Tasks[0].Launch == nil || page.Tasks[0].Launch.State != want {
			t.Fatalf("page %s disagrees: %+v %v", state, page, err)
		}
		raw, err := svc.LatestLaunch(ctx, id)
		if err != nil || raw.ID != duplicate.ID || raw.State != "stopped" || raw.SafeError != diagnostic {
			t.Fatal("historical diagnostic was lost")
		}
		// Release fixture capacity before the next table row.
		a.Status = AttemptStatusFailed
		if err := store.UpdateAttempt(ctx, a); err != nil {
			t.Fatal(err)
		}
		item.Status = TaskStatusBlocked
		if err := store.UpdateTask(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStartupPreservesExactActiveLaunchMemory(t *testing.T) {
	startupActiveLaunchContract(t, NewMemoryStore(), "startup-active-project")
}

func startupActiveLaunchContract(t *testing.T, store Store, project string) {
	t.Helper()
	ctx := context.Background()
	svc := NewService(store, nil)
	now := time.Now().UTC().Truncate(time.Microsecond)
	for n, c := range []struct {
		name, attempt, launch, taskState string
		cancel                           bool
		want                             string
	}{
		{"running started", AttemptStatusRunning, "started", TaskStatusDoing, false, "started"},
		{"queued claimed", AttemptStatusQueued, "claimed", TaskStatusQueued, false, "started"},
		{"repair orphan queued", AttemptStatusQueued, "stopped", TaskStatusQueued, false, "started"},
		{"repair restart diagnostic", AttemptStatusRunning, "stopped", TaskStatusDoing, false, "started"},
		{"cancel requested", AttemptStatusRunning, "started", TaskStatusDoing, true, "stopped"},
		{"failed execution", AttemptStatusFailed, "started", TaskStatusDoing, false, "stopped"},
		{"accepted completion", AttemptStatusRunning, "started", TaskStatusDone, false, "stopped"},
		{"cancelled task", AttemptStatusRunning, "started", "cancelled", false, "stopped"},
	} {
		id := fmt.Sprintf("startup_bound_%d", n)
		item := &Task{ID: id, ProjectID: project, Title: id, Instructions: "Approved scope", ExecutionCategory: ExecutionCategoryReadOnly, Status: TaskStatusQueued, CreatedAt: now}
		if err := store.CreateTask(ctx, item); err != nil {
			t.Fatal(err)
		}
		j, err := svc.SaveLaunch(ctx, TaskLaunch{TaskID: id, ProjectID: project, RequestID: id + "_request", OwnerID: "owner", RoomID: "91000000-0000-4000-8000-000000000001", SourceRequestID: "approved", AgentID: "agent", InstructionsHash: LaunchInstructionsHash(item.Instructions), CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		lease, err := svc.ClaimLaunch(ctx, now, "fixture", j.ID)
		if err != nil || lease == nil {
			t.Fatal("claim", err)
		}
		a := &TaskAttempt{ID: attemptRequestID(id, j.RequestID), TaskID: id, RunnerID: "fixture", AgentName: "agent", Status: c.attempt, CancelRequested: c.cancel, CreatedAt: now}
		if err := store.CreateAttempt(ctx, a); err != nil {
			t.Fatal(err)
		}
		if c.launch != "claimed" {
			diagnostic := "Execution stopped during server restart."
			if c.name == "repair orphan queued" {
				diagnostic = "Execution attempt is no longer active (orphan launch reconciled)."
			}
			if err := svc.FinishLaunch(ctx, j.ID, lease.LeaseToken, c.launch, now, diagnostic); err != nil {
				t.Fatal(err)
			}
		}
		item.Status = c.taskState
		if err := store.UpdateTask(ctx, item); err != nil {
			t.Fatal(err)
		}
		for pass := 0; pass < 2; pass++ {
			if err := svc.ReconcileStartupLaunches(ctx, now); err != nil {
				t.Fatal(err)
			}
			if c.want == "started" {
				if err := svc.ReconcileOrphanLaunches(ctx, now.Add(3*time.Minute)); err != nil {
					t.Fatal(err)
				}
			}
			got, err := svc.GetLaunch(ctx, j.ID)
			if err != nil || got.State != c.want {
				t.Fatalf("%s pass%d: %+v %v", c.name, pass, got, err)
			}
			if c.want == "started" && (got.SafeError != "" || got.LeaseToken != "" || got.LeaseUntil != nil) {
				t.Fatal("active admission retains false error or stale lease")
			}
			original, _ := store.GetAttempt(ctx, a.ID)
			if original.Status != c.attempt || original.CancelRequested != c.cancel {
				t.Fatal("startup changed execution")
			}
		}
		item.Status = TaskStatusBlocked
		a.Status = AttemptStatusFailed
		if err := store.UpdateTask(ctx, item); err != nil {
			t.Fatal(err)
		}
		if err := store.UpdateAttempt(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
}
