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
