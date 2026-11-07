package server

import (
	"context"
	"database/sql"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

type claimBeforeEnqueueReturnsStore struct {
	task.Store
	claim func(context.Context) error
}

func (s claimBeforeEnqueueReturnsStore) CreateQueuedAttempt(ctx context.Context, a *task.TaskAttempt, predecessor string, resetReport bool) error {
	store := s.Store.(interface {
		CreateQueuedAttempt(context.Context, *task.TaskAttempt, string, bool) error
	})
	if err := store.CreateQueuedAttempt(ctx, a, predecessor, resetReport); err != nil {
		return err
	}
	return s.claim(ctx)
}

func TestQueuedStartEndpointPreservesClaimBeforeMetadataWrite(t *testing.T) {
	for _, outcome := range []string{"running", "succeeded", "failed"} {
		t.Run(outcome, func(t *testing.T) { queuedStartEndpointContract(t, nil, outcome) })
	}
}

func TestQueuedStartPostgres(t *testing.T) {
	for _, outcome := range []string{"running", "succeeded", "failed"} {
		t.Run(outcome, func(t *testing.T) {
			withPhasePostgres(t, func(database *sql.DB) { queuedStartEndpointContract(t, database, outcome) })
		})
	}
}

func TestQueuedStartInitialization(t *testing.T) {
	queuedStartInitializationContract(t, task.NewMemoryStore())
}

func TestQueuedStartInitializationPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		queuedStartInitializationContract(t, task.NewDBStore(database))
	})
}

func queuedStartInitializationContract(t *testing.T, store task.Store) {
	t.Helper()
	ctx := context.Background()
	for _, mode := range []string{"reset", "preserve", "active", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			item := &task.Task{ID: "initialize-" + mode, Title: "Original scope", Instructions: "Keep original instructions", Status: task.TaskStatusBacklog, Summary: "prior summary", CompleteNote: "prior notes", DoDChecklist: []task.DoDItem{{Text: "Original criterion", Completed: true, Evidence: "prior evidence"}}}
			if mode == "cancelled" {
				item.Status = "cancelled"
			}
			if err := store.CreateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			if mode == "active" {
				if err := store.CreateAttempt(ctx, &task.TaskAttempt{ID: "existing", TaskID: item.ID, Status: task.AttemptStatusQueued, RunnerID: "cuan-runner"}); err != nil {
					t.Fatal(err)
				}
			}
			svc := task.NewService(store, nil)
			attempt, err := svc.CreateAttempt(ctx, item.ID, "owner", task.CreateAttemptRequest{RunnerID: "cuan-runner", AgentName: "selected-agent", InitializeQueuedTask: true, PreserveQueuedReport: mode == "preserve"})
			if mode == "active" || mode == "cancelled" {
				if !errors.Is(err, task.ErrTaskActive) && !errors.Is(err, task.ErrInvalidTaskInput) {
					t.Fatalf("expected admission rejection, got %v", err)
				}
			} else if err != nil || attempt == nil {
				t.Fatalf("queue creation failed: %v", err)
			}
			saved, err := store.GetTask(ctx, item.ID)
			if err != nil {
				t.Fatal(err)
			}
			if saved.Title != item.Title || saved.Instructions != item.Instructions || saved.DoDChecklist[0].Text != "Original criterion" {
				t.Fatal("queue initialization changed original scope")
			}
			if mode == "reset" {
				if saved.Summary != "" || saved.CompleteNote != "" || saved.DoDChecklist[0].Completed {
					t.Fatal("fresh queue retained previous acceptance")
				}
			} else if saved.Summary != "prior summary" || saved.CompleteNote != "prior notes" || !saved.DoDChecklist[0].Completed || saved.DoDChecklist[0].Evidence != "prior evidence" {
				t.Fatal("saved report changed on preservation or failed admission")
			}
			if mode == "active" || mode == "cancelled" {
				if saved.Status != item.Status || saved.AssignedAgent != item.AssignedAgent {
					t.Fatal("failed enqueue partially updated task")
				}
				attempts, err := store.ListAttempts(ctx, item.ID)
				want := 0
				if mode == "active" {
					want = 1
				}
				if err != nil || len(attempts) != want {
					t.Fatal("failed enqueue leaked a new attempt", err)
				}
			} else if saved.Status != task.TaskStatusQueued || saved.AssignedAgent != "selected-agent" {
				t.Fatal("queue metadata missing")
			}
		})
	}
}

func queuedStartEndpointContract(t *testing.T, database *sql.DB, outcome string) {
	t.Helper()
	s, _, call, _ := inboxFixture(t)
	ctx := context.Background()
	var base task.Store = s.taskSvc.Store()
	if database != nil {
		if _, err := database.ExecContext(ctx, `INSERT INTO projects(id,name,repo_url) VALUES('default','Fixture','') ON CONFLICT DO NOTHING`); err != nil {
			t.Fatal(err)
		}
		base = task.NewDBStore(database)
		s.taskSvc = task.NewService(base, s.projectSvc)
		s.runnerSvc = runner.NewService(runner.NewDBStore(database), base)
	}
	item := workspaceTask(t, call)
	s.taskSvc = task.NewService(claimBeforeEnqueueReturnsStore{Store: base, claim: func(ctx context.Context) error {
		queued, err := base.GetTask(ctx, item.ID)
		if err != nil || queued.Status != task.TaskStatusQueued || queued.AssignedAgent != "agent_lifecycle" {
			t.Fatalf("queue exposed before metadata commit: %+v %v", queued, err)
		}
		claimed, err := s.runnerSvc.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "fast-worker", SupportedAgents: []string{"agy", "agent_lifecycle"}}, time.Minute)
		if err != nil {
			return err
		}
		if !claimed.Claimed {
			t.Fatal("fixture did not claim the visible queued attempt")
		}
		if outcome != task.AttemptStatusRunning {
			// Simulate a worker that finishes before the launching request returns.
			// The handler must preserve its exact report and checklist as well.
			a := claimed.Attempt
			a.Status = outcome
			finished := time.Now().UTC()
			a.FinishedAt = &finished
			if err := base.UpdateAttempt(ctx, a); err != nil {
				return err
			}
			item, err := base.GetTask(ctx, item.ID)
			if err != nil {
				return err
			}
			item.Status = task.TaskStatusReview
			item.Summary = "worker-owned report"
			item.DoDChecklist[0].Completed = true
			item.DoDChecklist[0].Evidence = "worker-owned evidence"
			if err := base.UpdateTask(ctx, item); err != nil {
				return err
			}
		}
		return nil
	}}, s.projectSvc)
	response := call("POST", "/api/v1/tasks/"+item.ID+"/run-agent", `{"agent_id":"agent_lifecycle","runner_id":"cuan-runner"}`)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"already_started":true`) {
		t.Fatal("late enqueue rejected claimed work", response.Code, response.Body.String())
	}
	detail, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantStatus := task.TaskStatusDoing
	if outcome != task.AttemptStatusRunning {
		wantStatus = task.TaskStatusReview
		if detail.Summary != "worker-owned report" || !detail.DoDChecklist[0].Completed || detail.DoDChecklist[0].Evidence != "worker-owned evidence" {
			t.Fatal("late enqueue replaced the worker's report or criteria")
		}
	}
	if detail.Status != wantStatus || detail.LatestAttempt == nil || detail.LatestAttempt.Status != outcome || detail.LatestAttempt.RunnerID != "fast-worker" || len(detail.Attempts) != 1 {
		t.Fatalf("late enqueue damaged claimed execution: %+v", detail)
	}
}

func TestQueuedStartAcknowledgesExactClaimWithoutChangingWorkerOutcome(t *testing.T) {
	for _, scenario := range []string{"running", "succeeded", "failed", "queued", "cancelled", "foreign attempt"} {
		t.Run(scenario, func(t *testing.T) {
			s, _, _, _ := inboxFixture(t)
			ctx := context.Background()
			store := s.taskSvc.Store()
			item := &task.Task{ID: "task", Title: "Authorized work", Status: task.TaskStatusDoing, Summary: "preserve", DoDChecklist: []task.DoDItem{{Text: "Original criterion", Completed: true, Evidence: "saved"}}}
			if err := store.CreateTask(ctx, item); err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			a := &task.TaskAttempt{ID: "attempt", TaskID: item.ID, RunnerID: "worker", Status: scenario, CreatedAt: now.Add(-time.Second), StartedAt: &now}
			if scenario == "cancelled" {
				a.Status = task.AttemptStatusRunning
				a.CancelRequested = true
			}
			if scenario == "foreign attempt" {
				a.Status = task.AttemptStatusRunning
				a.ID = "other"
			}
			if scenario == "queued" {
				a.StartedAt = nil
			}
			if err := store.CreateAttempt(ctx, a); err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			got := s.acknowledgeClaimedWorkerAttempt(response, ctx, item.ID, "attempt")
			want := scenario == "running" || scenario == "succeeded" || scenario == "failed"
			if got != want {
				t.Fatal("wrong claim reconciliation", scenario, got)
			}
			if want && (response.Code != 200 || !strings.Contains(response.Body.String(), `"status":"`+scenario+`"`)) {
				t.Fatal("actual attempt outcome lost", response.Body.String())
			}
			current, err := store.GetTask(ctx, item.ID)
			if err != nil {
				t.Fatal(err)
			}
			saved, err := store.GetAttempt(ctx, a.ID)
			if err != nil {
				t.Fatal(err)
			}
			if current.Status != task.TaskStatusDoing || current.Summary != "preserve" || !current.DoDChecklist[0].Completed || saved.Status != a.Status {
				t.Fatal("late enqueue changed worker-owned state")
			}
		})
	}
}
