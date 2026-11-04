package task

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestRecentAttemptLogRetention(t *testing.T) {
	for _, backend := range []string{"memory", "sql"} {
		t.Run(backend, func(t *testing.T) {
			var store Store = NewMemoryStore()
			if backend == "sql" {
				database, err := sql.Open("sqlite", ":memory:")
				if err != nil {
					t.Fatal(err)
				}
				defer database.Close()
				database.SetMaxOpenConns(1)
				_, err = database.Exec(`CREATE TABLE agent_logs (id INTEGER PRIMARY KEY AUTOINCREMENT, attempt_id TEXT, stream TEXT, line TEXT, logged_at TIMESTAMP)`)
				if err != nil {
					t.Fatal(err)
				}
				store = NewDBStore(database)
			}
			ctx := context.Background()
			appendLines := func(id string, lines ...string) {
				t.Helper()
				logs := make([]LogRecord, len(lines))
				for i, line := range lines {
					logs[i] = LogRecord{Line: line, Stream: "stdout", LoggedAt: time.Now().UTC()}
				}
				if err := store.AppendLogs(ctx, id, logs, 1000); err != nil {
					t.Fatal(err)
				}
			}
			appendLines("other", "preserve other attempt")
			for batch := 0; batch < 12; batch++ {
				lines := make([]string, 100)
				for i := range lines {
					lines[i] = fmt.Sprintf("progress %d", batch*100+i)
				}
				appendLines("run", lines...)
			}
			appendLines("run", "Individual quota reached")
			logs, err := store.GetLogs(ctx, "run", 0, 1000)
			if err != nil || len(logs) != 1000 {
				t.Fatalf("retention count: %d %v", len(logs), err)
			}
			if logs[0].Line != "progress 201" || logs[len(logs)-1].Line != "Individual quota reached" {
				t.Fatal("latest diagnostic was lost")
			}
			tail, err := store.(RecentLogStore).GetRecentLogs(ctx, "run", 50)
			if err != nil || len(tail) != 50 || tail[0].Line != "progress 1151" || tail[49].Line != "Individual quota reached" {
				t.Fatal("recent read returned the beginning of retained history", err)
			}
			absent, err := store.(RecentLogStore).GetRecentLogs(ctx, "missing", 50)
			if err != nil || len(absent) != 0 {
				t.Fatal("missing attempt returned another execution's output", err)
			}
			cursor := logs[len(logs)-1].ID
			appendLines("run", "continued after cursor")
			next, err := store.GetLogs(ctx, "run", cursor, 1000)
			if err != nil || len(next) != 1 || next[0].Line != "continued after cursor" {
				t.Fatal("incremental cursor broke", err)
			}
			for i := 0; i < 40; i++ {
				appendLines("run", strings.Repeat("界", 2000))
			}
			appendLines("run", "late failure after byte limit")
			logs, err = store.GetLogs(ctx, "run", 0, 1000)
			if err != nil {
				t.Fatal(err)
			}
			size := 0
			for _, r := range logs {
				size += len(r.Line) + 1
			}
			if size > maxRetainedLogBytes || logs[len(logs)-1].Line != "late failure after byte limit" {
				t.Fatal("byte budget or latest diagnostic", size)
			}
			appendLines("run", strings.Repeat("x", maxRetainedLogBytes+1))
			logs, err = store.GetLogs(ctx, "run", 0, 1000)
			if err != nil || logs[len(logs)-1].Line != "[Output line too long; not saved]" {
				t.Fatal("oversized line retained", err)
			}
			other, err := store.GetLogs(ctx, "other", 0, 1000)
			if err != nil || len(other) != 1 || other[0].Line != "preserve other attempt" {
				t.Fatal("cross-attempt deletion", err)
			}
		})
	}
}
