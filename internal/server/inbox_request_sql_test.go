package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Opt-in staging acceptance: distinct OS processes and the actual PostgreSQL
// repository. Only labelled fixture rooms/messages are added; none deleted.
func TestS264PostgresProcessRestart(t *testing.T) {
	dsn := os.Getenv("S264_SQL_DSN")
	if dsn == "" {
		t.Skip("requires explicit isolated staging DSN")
	}
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal("open fixture DB")
	}
	defer database.Close()
	ctx := context.Background()
	if mode := os.Getenv("S264_SQL_PHASE"); mode != "" {
		s, token := setupTestChatServer(t)
		s.cfg.Environment = "staging"
		s.SetProjectStudioRepository(db.NewSQLProjectStudioRepository(database))
		agents := db.NewMemoryAgentRepository()
		agents.CreateAgent(ctx, &db.Agent{ID: "agent_s264_sql_fixture", Name: "@s264-sql-fixture", Harness: "agy", Provider: "agy", Enabled: true, Status: "active"})
		s.SetAgentRepository(agents)
		room := os.Getenv("S264_SQL_ROOM")
		key := os.Getenv("S264_SQL_KEY")
		call := func(method, body string) *httptest.ResponseRecorder {
			r := httptest.NewRequest(method, "/api/v1/chat/conversations/"+room+"/messages", strings.NewReader(body))
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			s.Router().ServeHTTP(w, r)
			return w
		}
		body := `{"message":"SQL FIXTURE acceptance and retry","request_id":"` + key + `"}`
		if mode == "crash" {
			w := call("POST", body)
			if w.Code != 202 {
				t.Fatal(w.Code, w.Body.String())
			}
			end := time.Now().Add(2 * time.Second)
			for {
				b, _ := os.ReadFile(filepath.Join(os.Getenv("HOME"), "counter"))
				if len(b) > 0 {
					break
				}
				if time.Now().After(end) {
					t.Fatal("fixture CLI did not start")
				}
				time.Sleep(5 * time.Millisecond)
			}
			os.Exit(0) // actual service process loss after provider invocation began
		}
		if mode == "submit" {
			w := call("POST", body)
			if w.Code != 202 {
				t.Fatal(w.Code, w.Body.String())
			}
			for i := 0; i < 3; i++ {
				w = call("POST", body)
				if w.Code != 200 && w.Code != 202 {
					t.Fatal(w.Code, w.Body.String())
				}
			}
			end := time.Now().Add(3 * time.Second)
			for {
				w = call("GET", "")
				if strings.Contains(w.Body.String(), `"state":"completed"`) {
					break
				}
				if time.Now().After(end) {
					t.Fatal(w.Body.String())
				}
				time.Sleep(10 * time.Millisecond)
			}
		} else {
			w := call("GET", "")
			if w.Code != 200 {
				t.Fatal(w.Body.String())
			}
			expected := "completed"
			if mode == "recover-crash" {
				expected = "interrupted"
			}
			if !strings.Contains(w.Body.String(), `"state":"`+expected+`"`) {
				t.Fatal(w.Body.String())
			}
			w = call("POST", body)
			if w.Code != 200 || !strings.Contains(w.Body.String(), `"state":"`+expected+`"`) {
				t.Fatal(w.Code, w.Body.String())
			}
		}
		return
	}
	base := t.TempDir()
	bin := filepath.Join(base, "bin")
	os.Mkdir(bin, 0700)
	os.WriteFile(filepath.Join(bin, "agy"), []byte("#!/bin/sh\necho run >> \"$HOME/counter\"\nsleep 0.3\nprintf 'SQL FIXTURE reply'\n"), 0700)
	repo := db.NewSQLProjectStudioRepository(database)
	for _, scenario := range []string{"completed", "crash"} {
		home := filepath.Join(base, scenario)
		os.Mkdir(home, 0700)
		room, key := db.GenerateUUID(), db.GenerateUUID()
		now := time.Now().UTC()
		if err := repo.CreateRoom(ctx, &db.ProjectRoom{ID: room, ProjectID: "default", Title: "S26.4 SQL FIXTURE " + scenario, Topic: "agent:agent_s264_sql_fixture", CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		run := func(phase string) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestS264PostgresProcessRestart$", "-test.v")
			cmd.Env = append(os.Environ(), "S264_SQL_PHASE="+phase, "S264_SQL_ROOM="+room, "S264_SQL_KEY="+key, "HOME="+home, "PATH="+bin+":/usr/bin:/bin")
			out, e := cmd.CombinedOutput()
			if e != nil {
				t.Fatalf("phase %s: %s", phase, out)
			}
		}
		if scenario == "completed" {
			run("submit")
			run("recover")
		} else {
			// Allow the CLI to start then lose its host process before reply commit.
			os.WriteFile(filepath.Join(bin, "agy"), []byte("#!/bin/sh\necho run >> \"$HOME/counter\"\nsleep 1\nprintf 'SQL FIXTURE uncertain reply'\n"), 0700)
			run("crash")
			// Advance only this fixture's durable deadline; no 60-second test sleep.
			if _, err := database.ExecContext(ctx, `UPDATE inbox_requests SET deadline=NOW()-INTERVAL '1 second' WHERE room_id=$1 AND request_id=$2`, room, key); err != nil {
				t.Fatal(err)
			}
			run("recover-crash")
		}
		b, _ := os.ReadFile(filepath.Join(home, "counter"))
		if strings.Count(string(b), "run") != 1 {
			t.Fatalf("%s duplicated CLI: %s", scenario, b)
		}
		msgs, err := repo.ListMessagesByRoomID(ctx, room)
		if err != nil {
			t.Fatal(err)
		}
		want := 2
		if scenario == "crash" {
			want = 1
		}
		if len(msgs) != want {
			t.Fatalf("%s duplicated/missing messages: %d", scenario, len(msgs))
		}
		qs, err := repo.ListInboxRequests(ctx, room)
		if err != nil || len(qs) != 1 || qs[0].RequestID != key {
			t.Fatal("correlation not durable")
		}
		evidence, _ := json.Marshal(map[string]any{"scenario": scenario, "distinctOSProcesses": true, "actualPostgreSQL": true, "requestID": key, "userMessageID": qs[0].MessageID, "state": qs[0].State, "messages": len(msgs), "CLIExecutions": 1, "provider": "FIXTURE; no real provider invocation"})
		t.Log(string(evidence))
	}
}
