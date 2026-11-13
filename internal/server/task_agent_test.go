package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/diagnostics"
	cuanPrompt "github.com/buchin/cuan/internal/prompt"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/taskcli"
	"github.com/buchin/cuan/internal/terminal"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "task" {
		os.Exit(taskcli.Run(os.Args[2:], os.Stdout, os.Stderr))
	}
	// A suite may run inside a real worker. Fixture subprocesses must only use
	// capabilities deliberately issued by their tests, never the live parent.
	for _, key := range []string{"CUAN_TASK_CAPABILITY_JSON", "CUAN_TASK_CAPABILITY_FILE", "CUAN_TASK_EXECUTABLE", "CUAN_TASK_ARTIFACTS_DIR"} {
		_ = os.Unsetenv(key)
	}
	os.Exit(m.Run())
}

const fixtureReviewReport = "\n\"$CUAN_TASK_EXECUTABLE\" task report --summary 'Fixture implementation complete' --dod-item 'Verify the result' --dod-evidence 'Fixture output and behavior observed' --complete-note 'Verified fixture output and behavior' --state review\n"

func TestTaskAgentReportReturnsCompactReceipt(t *testing.T) {
	s, _, _, _ := inboxFixture(t)
	ctx := context.Background()
	instructions := strings.Repeat("Original approved task context. ", 600)
	created, err := s.taskSvc.CreateTask(ctx, "owner", task.CreateTaskRequest{Title: "Receipt fixture", Instructions: instructions})
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.taskSvc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "agent_lifecycle"})
	if err != nil {
		t.Fatal(err)
	}
	doing := task.TaskStatusDoing
	s.taskSvc.UpdateTask(ctx, created.ID, "owner", task.UpdateTaskRequest{Status: &doing})
	s.taskSvc.UpdateAttemptStatus(ctx, a.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning})
	_, _, token, cleanup, err := s.issueTaskAgent(ctx, created.ID, a.ID, "agent_lifecycle")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	summary, note := strings.Repeat("Observed work. ", 300), strings.Repeat("Retained verification. ", 300)
	for _, state := range []string{"progress", "review"} {
		payload, _ := json.Marshal(map[string]string{"state": state, "summary": summary, "complete_note": note})
		request := httptest.NewRequest("POST", "/api/v1/task-agent/"+created.ID, bytes.NewReader(payload))
		request.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, request)
		if w.Code != 200 || w.Body.Len() > 1024 {
			t.Fatal("report echoed task context instead of a compact receipt", w.Code, w.Body.Len())
		}
		var receipt struct {
			Task      struct{ ID, Status string } `json:"task"`
			AttemptID string                      `json:"attempt_id"`
			State     string                      `json:"report_state"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &receipt); err != nil || receipt.Task.ID != created.ID || receipt.Task.Status != task.TaskStatusDoing || receipt.AttemptID != a.ID || receipt.State != state {
			t.Fatal("receipt lost saved report identity or implied acceptance", err)
		}
		for _, forbidden := range []string{"instructions", "dod_checklist", "complete_note", "Observed work", "Retained verification"} {
			if strings.Contains(w.Body.String(), forbidden) {
				t.Fatal("receipt repeated task evidence", forbidden)
			}
		}
	}
	request := httptest.NewRequest("GET", "/api/v1/task-agent/"+created.ID, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, request)
	var saved task.TaskDetailResponse
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil || w.Code != 200 || saved.Instructions != created.Instructions || saved.Summary != strings.TrimSpace(summary) || saved.CompleteNote != strings.TrimSpace(note) {
		t.Fatal("compact reporting lost canonical task evidence", err)
	}
}

func TestRunnerPromptFinalReportCommandReachesReview(t *testing.T) {
	var parentRequests atomic.Int32
	parentAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parentRequests.Add(1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer parentAPI.Close()
	parentCapability, _ := json.Marshal(taskcli.Capability{API: parentAPI.URL, Token: "fixture-parent-only", TaskID: "parent-task"})
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", string(parentCapability))
	s, _, call, _ := inboxFixture(t)
	api := httptest.NewServer(s.Router())
	defer api.Close()
	s.taskAgentURL = api.URL
	created := workspaceTask(t, call)
	ctx := context.Background()
	a, err := s.taskSvc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "agent_lifecycle"})
	if err != nil {
		t.Fatal(err)
	}
	doing := task.TaskStatusDoing
	s.taskSvc.UpdateTask(ctx, created.ID, "owner", task.UpdateTaskRequest{Status: &doing})
	s.taskSvc.UpdateAttemptStatus(ctx, a.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning})
	env, _, _, cleanup, err := s.issueTaskAgent(ctx, created.ID, a.ID, "agent_lifecycle")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	prompt := cuanPrompt.BuildRunnerPromptWithDoD("Fixture audit", "Verify the fixture", "", []string{"Verify the result"})
	for _, line := range strings.Split(prompt, "\n") {
		command := strings.TrimSpace(line)
		if !strings.HasPrefix(command, `"$CUAN_TASK_EXECUTABLE" task report `) || !strings.Contains(command, "--dod-item") {
			continue
		}
		command = strings.ReplaceAll(command, "<summary>", "Fixture audit result")
		command = strings.ReplaceAll(command, "<original criterion>", "Verify the result")
		command = strings.ReplaceAll(command, "<observed result>", "Fixture output matched its expected value")
		command = strings.ReplaceAll(command, "<detailed DoD verification evidence>", "Fixture output matched its expected value")
		cmd := exec.Command("sh", "-c", command)
		cmd.Env = append(os.Environ(), env...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("documented reporting command failed: %v: %s", err, output)
		}
		ready, err := s.taskSvc.AgentReviewReady(ctx, created.ID, a.ID)
		if err != nil || !ready {
			t.Fatal("documented final-report command only saved a checkpoint", ready, err)
		}
		if parentRequests.Load() != 0 {
			t.Fatal("fixture subprocess used its parent's reporting capability")
		}
		return
	}
	t.Fatal("runner prompt did not supply a reporting command")
}

func prepareTaskReporter(t *testing.T, s *Server) {
	t.Helper()
	api := httptest.NewServer(s.Router())
	s.taskAgentURL = api.URL
	t.Cleanup(api.Close)
	bin := strings.Split(os.Getenv("PATH"), ":")[0]
	file := filepath.Join(bin, "agy")
	data, e := os.ReadFile(file)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(file, append(data, []byte(fixtureReviewReport)...), 0700); e != nil {
		t.Fatal(e)
	}
}
func TestTaskAgentScopedReportAndReviewGate(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	created := workspaceTask(t, call)
	ctx := context.Background()
	a, e := s.taskSvc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "agent_lifecycle"})
	if e != nil {
		t.Fatal(e)
	}
	doing := task.TaskStatusDoing
	s.taskSvc.UpdateTask(ctx, created.ID, "owner", task.UpdateTaskRequest{Status: &doing})
	s.taskSvc.UpdateAttemptStatus(ctx, a.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning})
	env, visiblePrompt, token, cleanup, e := s.issueTaskAgent(ctx, created.ID, a.ID, "agent_lifecycle")
	if e != nil {
		t.Fatal(e)
	}
	defer cleanup()
	guidePath := filepath.Join(filepath.Dir(strings.TrimPrefix(env[0], "CUAN_TASK_CAPABILITY_FILE=")), "session.md")
	guideInfo, err := os.Stat(guidePath)
	if err != nil || guideInfo.Mode().Perm() != 0600 {
		t.Fatal("session guidance must be private", err)
	}
	guide, err := os.ReadFile(guidePath)
	if err != nil || !strings.Contains(string(guide), "A greeting or question is not a request to build") || !strings.Contains(string(guide), "Do not inspect, reverse-engineer") || !strings.Contains(string(guide), "Never use chat send, send_inbox_message") {
		t.Fatal("missing proportional task guidance", err)
	}
	if len(visiblePrompt) > 220 || strings.Contains(visiblePrompt, "Task reporting tool") || strings.Contains(string(guide), token) {
		t.Fatal("internal reporting instructions or credential leaked into prompt")
	}
	var cap taskcli.Capability
	raw, _ := os.ReadFile(strings.TrimPrefix(env[0], "CUAN_TASK_CAPABILITY_FILE="))
	json.Unmarshal(raw, &cap)
	invoke := func(method, id, body, credential string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/v1/task-agent/"+id, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+credential)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	if w := invoke("GET", created.ID, "", token); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := invoke("GET", "different_task", "", token); w.Code != 403 {
		t.Fatal("cross-task access", w.Code)
	}
	if w := invoke("GET", created.ID, "", "owner-token"); w.Code != 401 {
		t.Fatal("unscoped credential", w.Code)
	}
	for _, body := range []string{`{"instructions":"overwritten"}`, `{"status":"done"}`, `{"state":"done"}`, `{"state":"review"}`, `{"summary":"Result ready","complete_note":"Observed fixture output"}`} {
		if w := invoke("POST", created.ID, body, token); w.Code != 400 {
			t.Fatal("unsafe or incomplete report accepted", body, w.Code)
		}
	}
	w := invoke("POST", created.ID, `{"title":"Refined by agent","summary":"Implemented fixture","progress":"Checking output"}`, token)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var checkpointResponse struct {
		State    string `json:"report_state"`
		Workflow string `json:"workflow"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &checkpointResponse); err != nil || checkpointResponse.State != "progress" || !strings.Contains(checkpointResponse.Workflow, "Continue the approved task") {
		t.Fatal("metadata checkpoint response implied completion", err)
	}
	w = invoke("POST", created.ID, `{"state":"review","summary":"Implemented fixture","complete_note":"Verified actual output","dod_checklist":[{"text":"Verify the result","completed":false,"evidence":"Fixture output still lacks expected behavior"}]}`, token)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	detail, _ := s.taskSvc.GetTaskDetail(ctx, created.ID)
	if detail.Status != "doing" || detail.Instructions != created.Instructions {
		t.Fatal("report approved active work or changed human instructions")
	}
	zero := 0
	s.taskSvc.UpdateAttemptStatus(ctx, a.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusSucceeded, ExitCode: &zero})
	if e = s.taskSvc.ReconcileLocalTask(ctx, created.ID, a.ID); e != nil {
		t.Fatal(e)
	}
	detail, _ = s.taskSvc.GetTaskDetail(ctx, created.ID)
	if detail.Status != "review" || detail.Summary != "Implemented fixture" {
		t.Fatal(detail.Status)
	}
	if w = invoke("POST", created.ID, `{"progress":"late"}`, token); w.Code != 409 {
		t.Fatal("late report accepted", w.Code)
	}
	cleanup()
	if _, err := os.Stat(guidePath); !os.IsNotExist(err) {
		t.Fatal("session guidance was not cleaned")
	}
	if w = invoke("GET", created.ID, "", token); w.Code != 401 {
		t.Fatal("revoked access", w.Code)
	}
}
func TestTaskAgentExitWithoutReportIsBlocked(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	api := httptest.NewServer(s.Router())
	defer api.Close()
	s.taskAgentURL = api.URL
	c := workspaceTask(t, call)
	w := call("POST", "/api/v1/tasks/"+c.ID+"/run-agent", `{"agent_id":"agent_lifecycle","request_id":"90000000-0000-4000-8000-000000000001","runner_id":"local_terminal"}`)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code, w.Body.String())
	}
	d := waitWorkspaceTask(t, s, c.ID, "failed")
	if d.Status != "blocked" || !strings.Contains(d.LatestAttempt.ErrorMessage, "No review-ready result") {
		t.Fatal("process exit claimed a result", d.Status, d.LatestAttempt.ErrorMessage)
	}
}

func TestTaskAgentFailureAndBlockerDoNotBecomeReview(t *testing.T) {
	for _, c := range []struct {
		name, state string
		exit        int
		attempt     string
	}{{"failed_after_report", "review", 7, "failed"}, {"reported_blocker", "blocked", 0, "failed"}} {
		t.Run(c.name, func(t *testing.T) {
			s, _, call, _ := inboxFixture(t)
			api := httptest.NewServer(s.Router())
			defer api.Close()
			s.taskAgentURL = api.URL
			script := "#!/bin/sh\n\"$CUAN_TASK_EXECUTABLE\" task report --summary 'Actual fixture result' --dod-item 'Verify the result' --dod-evidence 'Concrete fixture output observed' --complete-note 'Concrete fixture evidence' --state " + c.state + "\nexit " + fmt.Sprint(c.exit) + "\n"
			if e := os.WriteFile(filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy"), []byte(script), 0700); e != nil {
				t.Fatal(e)
			}
			created := workspaceTask(t, call)
			w := call("POST", "/api/v1/tasks/"+created.ID+"/run-agent", `{"agent_id":"agent_lifecycle","request_id":"90000000-0000-4000-8000-000000000002","runner_id":"local_terminal"}`)
			if w.Code != 200 {
				t.Fatal(w.Code, w.Body.String())
			}
			d := waitWorkspaceTask(t, s, created.ID, c.attempt)
			if d.Status != "blocked" || d.CompleteNote == "" {
				t.Fatal("unsuccessful work became review", d.Status)
			}
		})
	}
}

// A real PTY fixture, not a provider mock response: it stays interactive, accepts
// a second turn, uses the actual capability CLI, and waits for report finalization.
func TestTaskAgentInteractiveTurnAndReportedFinish(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow interactive PTY agent test in short mode")
	}
	for _, state := range []string{"review", "blocked", "done"} {
		t.Run(state, func(t *testing.T) {
			s, _, call, _ := inboxFixture(t)
			api := httptest.NewServer(s.Router())
			defer api.Close()
			s.taskAgentURL = api.URL
			kind := "implementation"
			if state == "review" {
				kind = "answer"
			}
			script := `#!/bin/sh
[ "$1" = "--dangerously-skip-permissions" ] || exit 21
[ "$2" = "--prompt-interactive" ] || exit 22
[ "$4" = "--add-dir" ] && [ "$5" = "$PWD" ] || exit 26
guide="${3##*Read the session guidance at }"
guide="${guide%% before proceeding*}"
[ -r "$guide" ] || exit 27
grep -F "$PWD" "$guide" >/dev/null || exit 28
"$CUAN_TASK_EXECUTABLE" task --help | grep -F 'dod_checklist' >/dev/null || exit 29
printf 'READY_FOR_FOLLOWUP\n'
read -r followup
[ "$followup" = "second turn" ] || exit 23
"$CUAN_TASK_EXECUTABLE" task get >/dev/null || exit 24
printf '%s' '{"state":"progress","dod_checklist":[{"text":"Verify the result","completed":true,"evidence":"Actual PTY input and task tool observed in fixture"}]}' > fixture-report.json
"$CUAN_TASK_EXECUTABLE" task report --report-file fixture-report.json >/dev/null || exit 30
"$CUAN_TASK_EXECUTABLE" task report --state ` + state + ` --result-kind ` + kind + ` --summary 'Two interactive turns completed' --dod-item 'Verify the result' --dod-evidence 'Actual PTY input and scoped task tool observed' --complete-note 'Verified actual PTY input and task tool' || exit 25
printf 'FINAL_OUTPUT_DRAINED\n'
while :; do sleep 1; done
`
			if err := os.WriteFile(filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			created := workspaceTask(t, call)
			path := "/api/v1/tasks/" + created.ID + "/run-agent"
			w := call("POST", path, `{"agent_id":"agent_lifecycle","request_id":"91000000-0000-4000-8000-000000000001","runner_id":"local_terminal"}`)
			if w.Code != 200 {
				t.Fatal(w.Code, w.Body.String())
			}
			sess, ok := s.terminalManager.GetSession(created.ID)
			if !ok {
				t.Fatal("interactive session missing")
			}
			// A detached viewer must not own the process lifetime.
			sub, _, err := sess.Subscribe("fixture-viewer", terminal.ModeInteractive)
			if err != nil {
				t.Fatal(err)
			}
			sess.Unsubscribe(sub.ID)
			if sess.IsClosed() {
				t.Fatal("detaching stopped the agent")
			}
			if _, err := sess.PTY().Write([]byte("second turn\n")); err != nil {
				t.Fatal(err)
			}
			result := waitWorkspaceTask(t, s, created.ID, "succeeded")
			if result.Status != state || result.Summary != "Two interactive turns completed" {
				t.Fatal(result.Status, result.Summary)
			}
			history := s.terminalManager.GetTaskHistory(created.ID)
			if len(history) != 1 || history[0].EndReason != "agent_report" {
				t.Fatal("missing explicit reported finish", history)
			}
			logs, err := s.taskSvc.GetAttemptLogs(context.Background(), result.LatestAttempt.ID, 0, 250)
			if err != nil {
				t.Fatal(err)
			}
			var output strings.Builder
			for _, line := range logs {
				output.WriteString(line.Line)
			}
			if !strings.Contains(output.String(), "FINAL_OUTPUT_DRAINED") {
				t.Fatal("final output not retained")
			}
		})
	}
}

func TestTaskAgentInteractiveFinalizationRespectsProgressAndCancel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow interactive PTY agent cancel test in short mode")
	}
	for _, cancel := range []bool{false, true} {
		t.Run(fmt.Sprint("cancel_", cancel), func(t *testing.T) {
			s, _, call, _ := inboxFixture(t)
			api := httptest.NewServer(s.Router())
			defer api.Close()
			s.taskAgentURL = api.URL
			script := `#!/bin/sh
"$CUAN_TASK_EXECUTABLE" task report --state review --summary 'Initial result' --dod-item 'Verify the result' --dod-evidence 'Initial fixture result observed' --complete-note 'Initial evidence' >/dev/null || exit 31
"$CUAN_TASK_EXECUTABLE" task report --state progress --progress 'Following new instructions' >/dev/null || exit 32
read -r next
"$CUAN_TASK_EXECUTABLE" task report --state review --summary 'Final result' --dod-item 'Verify the result' --dod-evidence 'Follow-up fixture result observed' --complete-note 'Verified follow-up' >/dev/null || exit 33
while :; do sleep 1; done
`
			if err := os.WriteFile(filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			c := workspaceTask(t, call)
			w := call("POST", "/api/v1/tasks/"+c.ID+"/run-agent", `{"agent_id":"agent_lifecycle","request_id":"92000000-0000-4000-8000-000000000001","runner_id":"local_terminal"}`)
			if w.Code != 200 {
				t.Fatal(w.Code)
			}
			// Wait until the later progress checkpoint is persisted.
			deadline := time.Now().Add(time.Second)
			for {
				d, _ := s.taskSvc.GetTaskDetail(context.Background(), c.ID)
				found := false
				for _, a := range d.Activities {
					if a.Message == "Following new instructions" {
						found = true
					}
				}
				if found {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("progress not reported")
				}
				time.Sleep(10 * time.Millisecond)
			}
			time.Sleep(2200 * time.Millisecond)
			sess, ok := s.terminalManager.GetSession(c.ID)
			if !ok || sess.IsClosed() {
				t.Fatal("older final report interrupted a later turn")
			}
			if cancel {
				w = call("POST", "/api/v1/tasks/"+c.ID+"/cancel", "")
				if w.Code != 200 {
					t.Fatal(w.Code)
				}
				d, _ := s.taskSvc.GetTaskDetail(context.Background(), c.ID)
				if d.LatestAttempt.Status != task.AttemptStatusCanceled || d.Status != task.TaskStatusBlocked {
					t.Fatal("cancel was overwritten")
				}
			} else {
				if _, err := sess.PTY().Write([]byte("finish\n")); err != nil {
					t.Fatal(err)
				}
				d := waitWorkspaceTask(t, s, c.ID, task.AttemptStatusSucceeded)
				if d.Status != task.TaskStatusReview || d.Summary != "Final result" {
					t.Fatal(d.Status)
				}
			}
		})
	}
}

func TestTaskAgentUnknownCapabilityCannotRecoverByLength(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	created := workspaceTask(t, call)
	ctx := context.Background()
	a, err := s.taskSvc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "agent_lifecycle"})
	if err != nil {
		t.Fatal(err)
	}
	doing := task.TaskStatusDoing
	s.taskSvc.UpdateTask(ctx, created.ID, "owner", task.UpdateTaskRequest{Status: &doing})
	s.taskSvc.UpdateAttemptStatus(ctx, a.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/task-agent/"+created.ID, nil)
	req.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 64))
	out := httptest.NewRecorder()
	s.Router().ServeHTTP(out, req)
	if out.Code != http.StatusUnauthorized {
		t.Fatalf("unknown capability was reconstructed: %d", out.Code)
	}
}

func TestTaskAgentPushHonorsTaskPublicationLimit(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	created := workspaceTask(t, call)
	ctx := context.Background()
	instructions := "Create one local guide. Jangan push atau deploy."
	if _, err := s.taskSvc.UpdateTask(ctx, created.ID, "owner", task.UpdateTaskRequest{Instructions: &instructions}); err != nil {
		t.Fatal(err)
	}
	a, err := s.taskSvc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "agent_lifecycle"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.taskSvc.UpdateAttemptStatus(ctx, a.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning}); err != nil {
		t.Fatal(err)
	}
	_, _, token, cleanup, err := s.issueTaskAgent(ctx, created.ID, a.ID, "agent_lifecycle")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	r := httptest.NewRequest("POST", "/api/v1/task-agent/"+created.ID+"/push", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Router().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestTaskMCPReportingRetainsCoordinatorAcceptance(t *testing.T) {
	s, _, call, _ := inboxFixture(t)
	created := workspaceTask(t, call)
	ctx := context.Background()
	attempt, err := s.taskSvc.CreateAttempt(ctx, created.ID, "owner", task.CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "agent_lifecycle"})
	if err != nil {
		t.Fatal(err)
	}
	doing := task.TaskStatusDoing
	if _, err := s.taskSvc.UpdateTask(ctx, created.ID, "owner", task.UpdateTaskRequest{Status: &doing}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.taskSvc.UpdateAttemptStatus(ctx, attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning}); err != nil {
		t.Fatal(err)
	}
	api := httptest.NewServer(s.Router())
	defer api.Close()
	s.taskAgentURL = api.URL
	env, _, _, cleanup, err := s.issueTaskAgent(ctx, created.ID, attempt.ID, "agent_lifecycle")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", strings.TrimPrefix(env[0], "CUAN_TASK_CAPABILITY_FILE="))
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")
	invoke := func(name string, arguments map[string]any, wantError bool) {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": arguments}})
		var out, errOut bytes.Buffer
		if diagnostics.RunMCP(strings.NewReader(string(raw)+"\n"), &out, &errOut) != 0 {
			t.Fatal(errOut.String())
		}
		var response struct {
			Result struct {
				IsError bool `json:"isError"`
			} `json:"result"`
		}
		if err := json.Unmarshal(out.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Result.IsError != wantError {
			t.Fatalf("unexpected tool outcome for %s: %s", name, out.String())
		}
	}
	invoke("get_current_task", map[string]any{}, false)
	invoke("report_current_task", map[string]any{"state": "progress", "progress": "Checking actual result"}, false)
	invoke("report_current_task", map[string]any{"state": "done"}, true)
	invoke("report_current_task", map[string]any{"state": "review", "summary": "Fixture result", "complete_note": "Observed local output", "dod_checklist": []map[string]any{{"text": "Verify the result", "completed": true, "evidence": "Fixture local output observed"}}}, false)
	detail, err := s.taskSvc.GetTaskDetail(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Status != task.TaskStatusDoing || detail.Instructions != created.Instructions {
		t.Fatal("MCP bypassed acceptance or scope")
	}
	found := false
	for _, activity := range detail.Activities {
		if activity.ActivityType == "agent_report" && activity.AttemptID != nil && *activity.AttemptID == attempt.ID && activity.NewStatus == "review" {
			found = true
		}
	}
	if !found {
		t.Fatal("report not attributed to worker attempt")
	}
	zero := 0
	if _, err := s.taskSvc.FinishReportedAttempt(ctx, attempt.ID, zero); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.ReconcileLocalTask(ctx, created.ID, attempt.ID); err != nil {
		t.Fatal(err)
	}
	detail, err = s.taskSvc.GetTaskDetail(ctx, created.ID)
	if err != nil || detail.Status != task.TaskStatusReview {
		t.Fatal("report did not reach review", err)
	}
	invoke("report_current_task", map[string]any{"progress": "late"}, true)
}

func TestTaskAgentCompletionOutlivesCanceledStartRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("interactive task completion fixture")
	}
	s, token, call, _ := inboxFixture(t)
	api := httptest.NewServer(s.Router())
	defer api.Close()
	s.taskAgentURL = api.URL
	script := `#!/bin/sh
read -r finish
"$CUAN_TASK_EXECUTABLE" task report --state review --result-kind answer --summary 'Recorded after HTTP cancellation' --dod-item 'Verify the result' --dod-evidence 'Scoped report completed after original HTTP request ended' --complete-note 'The original start request ended before the final report' >/dev/null || exit 21
printf 'FINAL_AFTER_HTTP_CANCEL\n'
while :; do sleep 1; done
`
	if err := os.WriteFile(filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	item := workspaceTask(t, call)
	start, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest("POST", "/api/v1/tasks/"+item.ID+"/run-agent", strings.NewReader(`{"agent_id":"agent_lifecycle","request_id":"98000000-0000-4000-8000-000000000001","runner_id":"local_terminal"}`)).WithContext(start)
	req.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	s.Router().ServeHTTP(response, req)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	cancel()
	session, ok := s.terminalManager.GetSession(item.ID)
	if !ok {
		t.Fatal("missing accepted task session")
	}
	if _, err := session.PTY().Write([]byte("finish\n")); err != nil {
		t.Fatal(err)
	}
	result := waitWorkspaceTask(t, s, item.ID, task.AttemptStatusSucceeded)
	if result.Status != task.TaskStatusReview || result.Summary != "Recorded after HTTP cancellation" {
		t.Fatal("lost completion after request ended", result.Status)
	}
	histories := s.terminalManager.GetTaskHistory(item.ID)
	if len(histories) != 1 || histories[0].EndReason != "agent_report" {
		t.Fatal("final report did not have an explicit completion receipt")
	}
}
