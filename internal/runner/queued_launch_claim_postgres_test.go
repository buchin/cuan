package runner

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/buchin/cuan/internal/task"
	"testing"
	"time"
)

func queuedLaunchClaimPostgresContract(t *testing.T, db *sql.DB) {
	for index, scenario := range []string{"queued reservations", "claimed preparation", "unknown started preparation"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			project := fmt.Sprintf("queue-claim-%d", index)
			if _, err := db.ExecContext(ctx, `INSERT INTO projects(id,name,repo_url) VALUES($1,'Queue fixture','')`, project); err != nil {
				t.Fatal(err)
			}
			defer db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, project)
			var room string
			if err := db.QueryRowContext(ctx, `INSERT INTO project_rooms(project_id,title) VALUES($1,'Queue fixture') RETURNING id::text`, project).Scan(&room); err != nil {
				t.Fatal(err)
			}
			ts, rs := task.NewDBStore(db), NewDBStore(db)
			now := time.Now().UTC()
			for i := 0; i < 7; i++ {
				id := fmt.Sprintf("%s-%d", project, i)
				item := &task.Task{ID: id, ProjectID: project, Title: "Scoped fixture", Instructions: "Implement approved fixture", Summary: "Fixture complete", CompleteNote: "Fixture output verified", Status: task.TaskStatusQueued, Priority: task.PriorityNormal, ExecutionCategory: task.ExecutionCategoryCode, CreatedAt: now, UpdatedAt: now}
				if err := ts.CreateTask(ctx, item); err != nil {
					t.Fatal(err)
				}
				a := &task.TaskAttempt{ID: id + "-attempt", TaskID: id, AgentName: "agy", Status: task.AttemptStatusQueued, AttemptNumber: 1, CreatedAt: now.Add(time.Duration(i) * time.Millisecond)}
				if err := ts.CreateAttempt(ctx, a); err != nil {
					t.Fatal(err)
				}
				if _, err := db.ExecContext(ctx, `INSERT INTO task_launches(id,task_id,project_id,request_id,attempt_id,owner_id,room_id,source_request_id,agent_id,instructions_hash,state,available_at,expires_at,created_at,updated_at) VALUES($1,$2,$3,$1,$4,'fixture',$5,'fixture','agy','fixture','started',$6,$7,$6,$6)`, id+"-launch", id, project, a.ID, room, now, now.Add(time.Hour)); err != nil {
					t.Fatal(err)
				}
			}
			if index > 0 {
				guard := project + "-guard"
				if err := ts.CreateTask(ctx, &task.Task{ID: guard, ProjectID: project, Title: "Preparing", Status: task.TaskStatusQueued, Priority: task.PriorityNormal, ExecutionCategory: task.ExecutionCategoryCode, CreatedAt: now, UpdatedAt: now}); err != nil {
					t.Fatal(err)
				}
				state := "started"
				if index == 1 {
					state = "claimed"
				}
				if _, err := db.ExecContext(ctx, `INSERT INTO task_launches(id,task_id,project_id,request_id,attempt_id,owner_id,room_id,source_request_id,agent_id,instructions_hash,state,lease_until,available_at,expires_at,created_at,updated_at) VALUES($1,$1,$2,$1,'','fixture',$3,'fixture','agy','fixture',$4,$5,$6,$7,$6,$6)`, guard, project, room, state, now.Add(time.Minute), now, now.Add(time.Hour)); err != nil {
					t.Fatal(err)
				}
			}
			type result struct {
				attempt *task.TaskAttempt
				err     error
			}
			results := make(chan result, 2)
			for i := 0; i < 2; i++ {
				id := fmt.Sprintf("%s-runner-%d", project, i)
				if err := rs.RegisterOrUpdateHeartbeat(ctx, &RunnerInfo{ID: id, Name: id, Status: StatusIdle, SupportedAgents: []string{"agy"}}); err != nil {
					t.Fatal(err)
				}
				go func() {
					a, _, err := rs.ClaimNextQueuedAttempt(ctx, id, []string{"agy"}, time.Minute, []string{task.ExecutionCategoryCode})
					results <- result{a, err}
				}()
			}
			count := 0
			var claimed *task.TaskAttempt
			for i := 0; i < 2; i++ {
				r := <-results
				if r.err != nil {
					t.Fatal(r.err)
				}
				if r.attempt != nil {
					claimed = r.attempt
					count++
				}
			}
			want := 0
			if index == 0 {
				want = 1
			}
			if count != want {
				t.Fatalf("claimed %d want %d", count, want)
			}
			if claimed != nil {
				if claimed.TaskID != project+"-0" {
					t.Fatal("oldest queued attempt skipped", claimed.TaskID)
				}
				req := lifecycleResult()
				req.RunnerID = claimed.RunnerID
				if _, err := NewService(rs, ts).Complete(ctx, claimed.ID, req); err != nil {
					t.Fatal(err)
				}
				next, _, err := rs.ClaimNextQueuedAttempt(ctx, claimed.RunnerID, []string{"agy"}, time.Minute, []string{task.ExecutionCategoryCode})
				if err != nil || next == nil || next.TaskID != project+"-1" {
					t.Fatal("queue did not advance after completion", next, err)
				}
			}
		})
	}
}
