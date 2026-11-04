package server

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/task"
)

func TestInboxDeliveryCandidatesPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		ctx := context.Background()
		prefix := "Cuan managed delegation v1.\n"
		_, origin, _ := phaseLineageSetup(t, database)
		if _, err := database.ExecContext(ctx, `INSERT INTO tasks(id,title,instructions,status,source_room_id,summary,complete_note) SELECT 'accepted-'||n,'Accepted result',$1,'done',$2,'Accepted','Verified' FROM generate_series(1,7000) n`, prefix, origin.RoomID); err != nil {
			t.Fatal(err)
		}
		if _, err := database.ExecContext(ctx, `INSERT INTO task_attempts(id,task_id,runner_id,agent_name,status) SELECT 'att-'||id,id,'fixture','agy','succeeded' FROM tasks`); err != nil {
			t.Fatal(err)
		}
		if _, err := database.ExecContext(ctx, `INSERT INTO tasks(id,title,instructions,status,source_room_id,summary,complete_note) VALUES('blocked','Blocked',$1,'blocked',$2,'',''),('cancelled','Canceled',$1,'cancelled',$2,'',''),('manual','Owner result',$1,'done',$2,'Accepted','Verified'),('empty-done','No result',$1,'done',$2,'',''),('unmanaged','Separate','Other instructions','blocked',$2,'','')`, prefix, origin.RoomID); err != nil {
			t.Fatal(err)
		}
		rows, err := task.NewService(task.NewDBStore(database), nil).InboxDeliveryTasks(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		sort.Strings(ids)
		if strings.Join(ids, ",") != "blocked,cancelled,manual" {
			t.Fatal("SQL selected accepted history or omitted delivery", ids)
		}
		var settled int
		if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE status='done' AND id LIKE 'accepted-%'`).Scan(&settled); err != nil || settled != 7000 {
			t.Fatal("delivery query mutated history", settled, err)
		}
	})
}
