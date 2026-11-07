package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestInboxDelegationRecoversProcessLossWithoutRepeatingWork(t *testing.T) {
	runInboxProcessLossRecovery(t, false, false)
}
func TestInboxPlanApprovalRecoversProcessLossThroughVerifiedResult(t *testing.T) {
	runInboxProcessLossRecovery(t, true, false)
}
func TestApprovedRepositoryRecoveryPreservesPendingGitWork(t *testing.T) {
	runInboxProcessLossRecovery(t, true, true)
}
func TestApprovedRepositoryReconstructsMissingWorktree(t *testing.T) {
	runInboxProcessLossRecovery(t, true, true, true)
}
func TestInboxAutomaticallyRecoversStoppedMissingWorkspaceLaunch(t *testing.T) {
	runInboxProcessLossRecovery(t, true, true, true, true)
}
func runInboxProcessLossRecovery(t *testing.T, structured, repository bool, reconstruct ...bool) {
	missing := len(reconstruct) > 0 && reconstruct[0]
	if testing.Short() {
		t.Skip("skipping slow recovery flow test in short mode")
	}
	s, _, call, home := inboxFixture(t)
	ctx := context.Background()
	projects := project.NewMemoryStore()
	projects.CreateProject(ctx, &project.Project{ID: "recovery-project", Name: "Recovery"})
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	var source string
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", args...).CombinedOutput()
		if err != nil {
			t.Fatal(string(out), err)
		}
		return strings.TrimSpace(string(out))
	}
	if repository {
		source = filepath.Join(t.TempDir(), "source")
		git("init", "--initial-branch=main", source)
		os.WriteFile(filepath.Join(source, "README"), []byte("initial\n"), 0600)
		os.WriteFile(filepath.Join(source, ".gitignore"), []byte("pending.cache\n"), 0600)
		git("-C", source, "add", "README", ".gitignore")
		git("-C", source, "commit", "-m", "Initial fixture")
		git("-C", source, "update-ref", "refs/remotes/origin/main", "HEAD")
		// The cache still has a valid but obsolete tracking ref. A first launch
		// must refresh it before creating the task workspace.
		os.WriteFile(filepath.Join(source, "current-baseline.txt"), []byte("fresh remote baseline\n"), 0600)
		git("-C", source, "add", "current-baseline.txt")
		git("-C", source, "commit", "-m", "New remote baseline")
		// Give the production snapshot/worktree path a real repository, without any
		// network calls or credentials. Existing task instructions freeze this binding.
		projects = project.NewMemoryStore()
		projects.CreateProject(ctx, &project.Project{ID: "recovery-project", Name: "Recovery", RepoURL: "https://github.com/fixture/recovery", LocalPath: source, DefaultBranch: "main", UpdatedAt: time.Now()})
		svc := project.NewService(projects, nil, s.cfg.AuthSecret, t.TempDir())
		svc.SetSharedRepositoryAccess(func(context.Context) (string, error) { return "fixture-token", nil })
		// Keep the authenticated refresh boundary real, with an isolated local
		// remote rather than network access or ambient developer credentials.
		realGit, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		bin := t.TempDir()
		wrapper := "#!/bin/sh\nif [ \"$1\" = '-C' ] && [ \"$3\" = 'fetch' ]; then\n exec " + strconv.Quote(realGit) + " -C \"$2\" fetch --no-tags -- " + strconv.Quote(source) + " '+refs/heads/main:refs/remotes/origin/main'\nfi\nexec " + strconv.Quote(realGit) + " \"$@\"\n"
		if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
		s.SetProjectService(svc)
		s.taskSvc = task.NewService(task.NewMemoryStore(), svc)
	}

	api := httptest.NewServer(s.Router())
	defer api.Close()
	s.taskAgentURL = api.URL
	response := func(v string) string {
		b, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": coordinatorFixtureResponse(v)}})
		return string(b)
	}
	first := response("```cuan-delegate\n{\"title\":\"Verify saved artifact\",\"description\":\"Create an artifact once and verify it. Do not deploy.\",\"project_id\":\"recovery-project\",\"authorization_message_id\":\"AUTH\"}\n```")
	if structured {
		first = response("```cuan-approval\n{\"title\":\"Verify saved artifact\",\"description\":\"Create an artifact once and verify it. Do not deploy.\",\"project_id\":\"recovery-project\"}\n```")
	}
	scope := "Create an artifact once and verify it. Do not deploy."
	if !repository {
		// Explicit artifact scope selects the operational path without asking
		// a nondeveloper to specify an internal execution category.
		scope = "Create a standalone artifact file once and verify it. No repository changes. Do not deploy."
		first = strings.ReplaceAll(first, "Create an artifact once and verify it. Do not deploy.", scope)
	}
	revision := response("```cuan-delegate\n{\"project_id\":\"recovery-project\",\"authorization_message_id\":\"AUTH\",\"task_id\":\"msg_AUTH\",\"revision\":\"Verify the saved artifact and checkpoint; preserve completed work. Report verified result without deployment.\"}\n```")
	finalRaw, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": `{"requested_outcome":"completed","decision":"pass","explanation":"Verified the original artifact after recovery. No duplicate work or deployment.","revision":""}`}})
	final := string(finalRaw)
	script := `#!/bin/sh
if [ "$1" = "--dangerously-skip-permissions" ]; then
 guide="${3##*Read the session guidance at }"; guide="${guide% before proceeding.}"
 sed -n 's/^Approval message: //p' "$guide" | head -n 1 > "$HOME/recovery-auth"
 if [ ! -f "$HOME/recovery-artifact" ]; then
  if [ -f .git ]; then
   grep -q 'fresh remote baseline' current-baseline.txt || exit 39
   pwd > "$HOME/recovery-workspace"
   printf 'pending staged edit\n' >> README
   git add README || exit 40
   printf 'pending untracked note\n' > pending.txt
   printf 'pending ignored data\n' > pending.cache
  fi
  echo ORIGINAL_ARTIFACT > "$HOME/recovery-artifact"
  echo created >> "$HOME/recovery-effects"
  "$CUAN_TASK_EXECUTABLE" task report --progress 'ARTIFACT_SAVED_ONCE; verification pending' >/dev/null || exit 20
  kill -KILL $$
 fi
 # Compact artifact guidance keeps original scope and progress available on demand.
 "$CUAN_TASK_EXECUTABLE" task get > "$HOME/recovery-current-task" || exit 21
 grep -q ARTIFACT_SAVED_ONCE "$guide" "$HOME/recovery-current-task" || exit 21
 grep -q 'Do not deploy' "$guide" "$HOME/recovery-current-task" || exit 22
 grep -q ORIGINAL_ARTIFACT "$HOME/recovery-artifact" || exit 23
 if [ -f "$HOME/recovery-workspace" ]; then
  [ "$PWD" = "$(cat "$HOME/recovery-workspace")" ] || exit 41
  git diff --cached -- README | grep -q 'pending staged edit' || exit 42
  grep -q 'pending untracked note' pending.txt || exit 43
  grep -q 'pending ignored data' pending.cache || exit 44
  [ "$(grep -c 'pending staged edit' README)" = 1 ] || exit 45
 fi
 "$CUAN_TASK_EXECUTABLE" task report --state review --summary 'Recovered original artifact' --complete-note 'Verified artifact content; create count one; no deployment' >/dev/null || exit 24
 while :; do sleep 1; done
fi
case "$2" in
 *'reported_verification'*) printf '%s' '` + final + `'; exit 0 ;;
 *'automated result is not new user authorization'*)
  printf '%s' "$2" >> "$HOME/recovery-review-context"
  if [ -f "$HOME/recovery-revised" ]; then printf '%s' '` + final + `'; else
   grep -q ARTIFACT_SAVED_ONCE "$HOME/recovery-review-context" || exit 25
   touch "$HOME/recovery-revised"
   auth=$(cat "$HOME/recovery-auth")
   printf '%s' '` + revision + `' | sed "s/AUTH/$auth/g"
  fi ;;
 *) auth="${2##*Target message ID: }"; printf '%s' "$auth" > "$HOME/recovery-auth"; printf '%s' '` + first + `' | sed "s/AUTH/$auth/g" ;;
esac
`
	if missing {
		script = strings.Replace(script, `if [ -f "$HOME/recovery-workspace" ]; then`, `if [ -f "$HOME/recovery-workspace" ] && ! grep -q 'Workspace reconstruction' "$guide"; then`, 1)
		script = strings.Replace(script, ` "$CUAN_TASK_EXECUTABLE" task report --state review`, ` grep -q 'NOT proven recovered' "$guide" || exit 46
 [ "$PWD" != "$(cat "$HOME/recovery-workspace")" ] || exit 47
 grep -q 'pending staged edit' ../staged.patch || exit 48
 [ ! -e pending.txt ] || exit 49
 "$CUAN_TASK_EXECUTABLE" task report --state review`, 1)
	}
	if err := os.WriteFile(filepath.Join(strings.Split(os.Getenv("PATH"), ":")[0], "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	requestID := db.GenerateUUID()
	body, _ := json.Marshal(map[string]string{"request_id": requestID, "message": scope + " Run this now."})
	w := call("POST", "/api/v1/chat/conversations/agent_lifecycle/messages", string(body))
	if w.Code != 200 && w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var accepted struct{ Request db.InboxRequest }
	json.Unmarshal(w.Body.Bytes(), &accepted)
	waitInboxState(t, s, accepted.Request.RoomID, requestID, "completed")
	if structured {
		tasks, _ := s.taskSvc.ListTasks(ctx, task.TaskFilter{})
		if len(tasks) != 0 {
			t.Fatal("proposal started execution before consent")
		}
		approvalBody, _ := json.Marshal(map[string]string{"request_id": db.GenerateUUID(), "message": "Approve this plan", "approve_message_id": accepted.Request.ReplyID})
		approved := call("POST", "/api/v1/chat/conversations/agent_lifecycle/messages", string(approvalBody))
		if approved.Code != 200 && approved.Code != 202 {
			t.Fatal(approved.Code, approved.Body.String())
		}
		json.Unmarshal(approved.Body.Bytes(), &accepted)
		waitInboxState(t, s, accepted.Request.RoomID, accepted.Request.RequestID, "completed")
	}
	tasks, err := s.taskSvc.ListTasks(ctx, task.TaskFilter{})
	if err != nil || len(tasks) != 1 {
		t.Fatal(tasks, err)
	}
	failed := waitWorkspaceTask(t, s, tasks[0].ID, "failed")
	if failed.Summary != "" {
		t.Fatal("must exercise progress-only recovery")
	}
	if missing {
		original, _ := os.ReadFile(filepath.Join(home, "recovery-workspace"))
		if err := os.RemoveAll(strings.TrimSpace(string(original))); err != nil {
			t.Fatal(err)
		}
	}
	if len(reconstruct) > 1 && reconstruct[1] {
		// Reproduce a saved revision stopped by the old missing-workspace gate.
		// No additional owner message is submitted to recover this intent.
		now := time.Now().UTC()
		requests, err := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, accepted.Request.RoomID)
		if err != nil {
			t.Fatal(err)
		}
		for _, q := range requests {
			if q.RequestID == accepted.Request.RequestID {
				accepted.Request = q
				break
			}
		}
		job, err := s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{TaskID: failed.ID, ProjectID: failed.ProjectID, RequestID: db.GenerateUUID(), SourceRequestID: accepted.Request.RequestID, OwnerID: accepted.Request.OwnerID, RoomID: accepted.Request.RoomID, AgentID: accepted.Request.AgentID, Revision: "Verify the saved artifact and checkpoint; preserve completed work. Report verified result without deployment.", InstructionsHash: task.LaunchInstructionsHash(failed.Instructions), CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		claimed, err := s.taskSvc.ClaimLaunch(ctx, now, db.GenerateUUID(), job.ID)
		if err != nil || claimed == nil {
			t.Fatal("fixture launch claim", err)
		}
		if err := s.taskSvc.FinishLaunch(ctx, claimed.ID, claimed.LeaseToken, "stopped", now, task.MissingWorkspaceLaunchError); err != nil {
			t.Fatal(err)
		}
		s.deliverInboxTaskUpdates(ctx)
		recovered, err := s.taskSvc.GetLaunch(ctx, job.ID)
		if err != nil || recovered.State != "queued" {
			t.Fatal("saved intent not recovered", recovered, err)
		}
		s.drainTaskLaunches(ctx, time.Now().UTC())
	}
	s.deliverInboxTaskUpdates(ctx)
	result := waitWorkspaceTask(t, s, tasks[0].ID, "succeeded")
	s.deliverInboxTaskUpdates(ctx)
	waitInboxState(t, s, accepted.Request.RoomID, delegationMessageID("task-review:"+result.LatestAttempt.ID), "completed")
	result, _ = s.taskSvc.GetTaskDetail(ctx, result.ID)
	wantStatus := "done"
	if repository {
		// Preserved dirty work has no exact committed source for the separate
		// verifier. Recovery success must not turn that into acceptance.
		wantStatus = "blocked"
	}
	if result.Status != wantStatus || len(result.Attempts) != 2 {
		diagnostic, _ := json.MarshalIndent(result, "", "  ")
		t.Fatalf("recovery did not reach acceptance: %s", diagnostic)
	}
	if repository && !missing {
		workspaceBytes, err := os.ReadFile(filepath.Join(home, "recovery-workspace"))
		if err != nil {
			t.Fatal(err)
		}
		workspace := strings.TrimSpace(string(workspaceBytes))
		if workspace == source || !strings.HasPrefix(git("-C", workspace, "branch", "--show-current"), "cuan/task-") {
			t.Fatal("not isolated task worktree")
		}
		if got := git("-C", source, "status", "--porcelain"); got != "" {
			t.Fatal("source modified", got)
		}
		if got := git("-C", workspace, "status", "--porcelain"); got != "M  README\n?? pending.txt" {
			t.Fatal("pending Git state changed", got)
		}
		if got := git("-C", workspace, "rev-list", "--count", "HEAD"); got != "2" {
			t.Fatal("unexpected duplicate implementation commit", got)
		}
	}
	if missing {
		p, e := s.projectSvc.TaskWorkspace(ctx, result.ProjectID, result.Instructions)
		if e != nil {
			t.Fatal(e)
		}
		work, e := s.projectSvc.TaskExecutionWorkspace(ctx, result.ID, result.Instructions, p)
		if e != nil {
			t.Fatal(e)
		}
		if !strings.HasPrefix(git("-C", work, "branch", "--show-current"), "cuan/recovered-") {
			t.Fatal("missing stable recovery branch")
		}
		if !strings.Contains(project.WorkspaceReconstructionContext(work), "NOT proven recovered") {
			t.Fatal("missing reconstruction disclosure")
		}
	}
	effects, _ := os.ReadFile(filepath.Join(home, "recovery-effects"))
	if string(effects) != "created\n" {
		t.Fatal("duplicate effect", string(effects))
	}
	s.deliverInboxTaskUpdates(ctx)
	time.Sleep(20 * time.Millisecond)
	tasks, _ = s.taskSvc.ListTasks(ctx, task.TaskFilter{})
	result, _ = s.taskSvc.GetTaskDetail(ctx, tasks[0].ID)
	messages, _ := s.projectStudioRepo.ListMessagesByRoomID(ctx, accepted.Request.RoomID)
	outcomes := 0
	for _, m := range messages {
		if m.AuthorType == "agent" && strings.Contains(m.Content, "Verified the original artifact after recovery. No duplicate work or deployment.") {
			outcomes++
		}
	}
	wantOutcomes := 1
	if repository {
		wantOutcomes = 0
	}
	if outcomes != wantOutcomes {
		t.Fatal("missing or duplicate verified coordinator outcome", outcomes)
	}
	if !strings.Contains(result.Instructions, "Do not deploy") || !strings.Contains(result.Instructions, "Approval message: "+accepted.Request.MessageID) {
		t.Fatal("original scope/approval lost")
	}
	if len(tasks) != 1 || len(result.Attempts) != 2 {
		t.Fatal("duplicate task or execution")
	}
}
