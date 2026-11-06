package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/taskcli"
)

// The subprocess is the real scoped CLI implementation, not an HTTP shortcut.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "task" {
		os.Exit(taskcli.Run(os.Args[2:], os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

func TestPTYNativeHookToDurableReport(t *testing.T) {
	if testing.Short() {
		t.Skip("PTY lifecycle protocol integration")
	}
	root := t.TempDir()
	script := `#!/usr/bin/env python3
import json, os, subprocess, sys, time
roots=[sys.argv[i+1] for i,a in enumerate(sys.argv[:-1]) if a=='--add-dir']
config=json.load(open(os.path.join(roots[-1],'.agents','hooks.json')))
command=config['cuan-final-report']['Stop'][0]['command']
event=json.dumps({'fullyIdle':True,'terminationReason':'NO_TOOL_CALL'})
response=subprocess.run(command,shell=True,input=event,text=True,capture_output=True,check=True)
assert json.loads(response.stdout)['decision']=='continue', response.stdout
# Simulate the harness obeying the injected report-only continuation.
subprocess.run([os.environ['CUAN_TASK_EXECUTABLE'],'task','report','--state','review','--summary','Fixture output','--complete-note','Fixture checks observed'],check=True)
# A second native stop must not start another continuation.
response=subprocess.run(command,shell=True,input=event,text=True,capture_output=True,check=True)
assert json.loads(response.stdout)['decision']=='allow', response.stdout
print('NATIVE_HOOK_REPORT_SAVED',flush=True)
while True: time.sleep(1)
`
	if err := os.WriteFile(filepath.Join(root, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+":"+os.Getenv("PATH"))
	var reported atomic.Bool
	var posts atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/report" {
			w.Write([]byte(`{}`))
			return
		}
		if r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("unscoped report access")
		}
		if r.Method == http.MethodPost {
			var data map[string]any
			if json.NewDecoder(r.Body).Decode(&data) != nil || data["state"] != "review" {
				t.Error("invalid final report")
			}
			posts.Add(1)
			reported.Store(true)
			w.Write([]byte(`{"state":"review"}`))
			return
		}
		state := "progress"
		if reported.Load() {
			state = "review"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"summary": "Fixture output", "complete_note": "Fixture checks observed", "latest_attempt": map[string]any{"id": "attempt-hook", "status": "running"}, "activities": []map[string]string{{"attempt_id": "attempt-hook", "activity_type": "agent_report", "new_status": state}}})
	}))
	defer api.Close()
	capability := &TaskAgentCapability{API: api.URL + "/report", Token: "fixture", TaskID: "task-hook", AttemptID: "attempt-hook"}
	raw, _ := json.Marshal(capability)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r := NewRunner(&config.Config{RunnerID: "fixture", RunnerServerURL: api.URL}, nil, api.Client())
	current := &task.Task{ID: "task-hook"}
	attempt := &task.TaskAttempt{ID: "attempt-hook", TaskID: current.ID}
	claim := ClaimTaskResponse{Task: current, Attempt: attempt, TaskAgent: capability}
	var count int64
	result, err := r.executeWithPTY(context.Background(), claim, current, attempt, "Fixture only", root, map[string]string{"CUAN_TASK_EXECUTABLE": executable, "CUAN_TASK_CAPABILITY_JSON": string(raw)}, 25*time.Second, &count)
	if err != nil {
		t.Fatal(err)
	}
	if !reported.Load() || posts.Load() != 1 || result.Status != "SUCCESS" || result.ExitCode != 0 {
		t.Fatalf("native-hook handoff failed: reports=%d result=%+v", posts.Load(), result)
	}
}
