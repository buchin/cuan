package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/taskcli"
	"github.com/buchin/cuan/internal/workflow"
	"github.com/go-chi/chi/v5"
)

type taskAgentGrant struct {
	TaskID, AttemptID, AgentID string
	Expires                    time.Time
	ArtifactRoot               *os.Root
	EvidenceRoot               *os.Root
}

func (s *Server) issueTaskAgent(ctx context.Context, taskID, attemptID, agentID string, contextParts ...string) ([]string, string, string, func(), error) {
	none := func() {}
	var b [32]byte
	if _, e := rand.Read(b[:]); e != nil {
		return nil, "", "", none, e
	}
	token := hex.EncodeToString(b[:])
	digest := sha256.Sum256([]byte(token))
	dir, e := os.MkdirTemp("", "cuan-task-agent-")
	if e != nil {
		return nil, "", "", none, e
	}
	var artifactRoot *os.Root
	var evidenceRoot *os.Root
	cleanup := func() {
		s.forgetTaskGrant(digest)
		_ = os.Remove(taskGrantPath(digest))
		if artifactRoot != nil {
			_ = artifactRoot.Close()
		}
		if evidenceRoot != nil {
			_ = evidenceRoot.Close()
		}
		_ = os.RemoveAll(dir)
	}
	_ = agent.EnsureAntigravityMCPConfig("")
	if workspace, ok := ctx.Value(taskAgentWorkspaceKey{}).(string); ok && workspace != "" {
		artifactRoot, e = os.OpenRoot(workspace)
		if e != nil {
			cleanup()
			return nil, "", "", none, e
		}
	}
	evidenceDir := filepath.Join(dir, "artifacts")
	if e = os.Mkdir(evidenceDir, 0700); e != nil {
		cleanup()
		return nil, "", "", none, e
	}
	if evidenceRoot, e = os.OpenRoot(evidenceDir); e != nil {
		cleanup()
		return nil, "", "", none, e
	}
	base := s.taskAgentURL
	if base == "" {
		port := s.cfg.ServerPort
		if port == "" {
			port = "8080"
		}
		base = "http://127.0.0.1:" + port
	}
	c := taskcli.Capability{API: base + "/api/v1/task-agent/" + taskID, Token: token, TaskID: taskID, AttemptID: attemptID, ArtifactsDir: evidenceDir, ReportFile: filepath.Join(dir, "report.json")}
	data, _ := json.Marshal(c)
	file := filepath.Join(dir, "capability.json")
	if e = os.WriteFile(file, data, 0600); e != nil {
		cleanup()
		return nil, "", "", none, e
	}
	executable, e := os.Executable()
	if e != nil {
		cleanup()
		return nil, "", "", none, e
	}
	if e = s.taskSvc.RequireAgentReport(ctx, taskID, attemptID, agentID); e != nil {
		cleanup()
		return nil, "", "", none, e
	}
	grant := taskAgentGrant{TaskID: taskID, AttemptID: attemptID, AgentID: agentID, Expires: time.Now().Add(8*time.Hour + 5*time.Minute), ArtifactRoot: artifactRoot, EvidenceRoot: evidenceRoot}
	workspace, _ := ctx.Value(taskAgentWorkspaceKey{}).(string)
	if e = s.persistTaskGrant(digest, grant, workspace); e != nil {
		cleanup()
		return nil, "", "", none, e
	}
	s.taskAgentGrants.Store(digest, grant)
	instructions := `## Session guidance
Respond to the actual request. A greeting or question is not a request to build
an application, create a repository, inspect deployment, or write tests.
For conversational requests, answer directly and briefly; do not invent coding
work or a project. Use tools only when needed for the request or final reporting.
Do not expand the request into extra deliverables. Refine a title only to clarify
the human request, never to invent scope. Acceptance criteria must derive from
the request rather than justify extra work.
Before the final report, compare the result with the original human instructions:
does it answer the request, stay within scope, and use the correct workspace?
Passing tests alone does not establish that the requested work was delivered.
Include this alignment and actual output location in completion evidence when
relevant; for conversation, no file location or automated tests are required.

For file work, use the explicit workspace below (also WORKSPACE_DIR/PWD), even
if a provider suggests its account-profile scratch directory. Do not create
deliverables in a provider profile or scratch directory. Follow existing
project conventions and stack. If no project stack exists and software is
explicitly requested, default to Go backend and React/Vite/TypeScript frontend.
Environment & tooling awareness: Check available build tools before attempting heavy builds or test runs. If build toolchains (e.g. Go compiler, npm/node modules) are absent or incomplete on this host, do not spend time installing packages or waiting in background loops. Rely on targeted code inspection, syntax verification, and static analysis to implement and verify scoped changes. Deliver clean, verified changes and clearly document what was verified and any environment limitations in the completion report.
Project Runbook & Operational Directives: For operational, administrative, data-verification or status-change tasks on project repositories (e.g. Laravel/Dojo with artisan commands, composer, npm scripts, database migrations, or payments inspection): utilize the project's official CLI / Artisan / runbook commands directly (e.g. php artisan payment:..., php artisan ..., composer ..., or relevant CLI scripts) rather than reading dozens of source files or writing new code from scratch. Run the appropriate command, verify exit status, and report results with concrete evidence.
Do not inspect, reverse-engineer, or search the Cuan server binary to understand
this reporting tool. If the tool fails, report its actual error as a blocker.
Project tests and build scripts must not inherit the live reporting capability.
Run them with CUAN_TASK_CAPABILITY_JSON, CUAN_TASK_CAPABILITY_FILE,
CUAN_TASK_EXECUTABLE and CUAN_TASK_ARTIFACTS_DIR unset in that child command
(for example, env -u CUAN_TASK_CAPABILITY_JSON -u CUAN_TASK_CAPABILITY_FILE
-u CUAN_TASK_EXECUTABLE -u CUAN_TASK_ARTIFACTS_DIR go test ...). Keep these
variables available in your own session for its explicit task reports. Capture
test output in the bound artifact directory using the parent shell; tests must
not report examples into the live task or start another agent execution.

For repository work, the runner prepares your bound repository checkout before
starting this session. Standalone artifact work instead has a private output
workspace: create and inspect only the requested file, without cloning or setting
up a repository, software stack or service. For repository work, inspect the
prepared checkout and preserve its existing files;
do not clone a duplicate merely because a standalone gh CLI is not signed in.
The current task's publication_configuration describes only the owner's registered
production verification setup. not_configured, incomplete or unsupported is a
setup prerequisite, not proof that code or deployment failed. Reconcile the
approved target and existing owner-saved access before repeating implementation.
Never borrow ambient developer credentials or redeploy just to obtain evidence.
Use task get or get_current_task for the current task's github_connection status.
The read-only get_github_connection MCP tool and integration github show command
also use this session's task capability. A connected status is not proof of a
particular repository's permissions. Never search Cuan host configuration,
production databases, provider profiles or credential stores for GitHub tokens,
Owner API credentials or signing keys. Never use sudo/psql or invent an Owner
token to bypass a missing scoped operation. Report the exact scoped-tool error
and preserve your workspace if the necessary operation is unavailable.

## Task reporting
The built-in command is "$CUAN_TASK_EXECUTABLE" task.
Run task get only when additional task context is needed. Metadata and checklists
are optional; keep simple requests simple. Use task --help for supported flags.
Task-specific prohibitions on push or deployment override the generic delivery defaults below. If publication is forbidden, retain and report the local artifact instead.
For repository implementation, commit only your scoped changes. Save the task
branch with "$CUAN_TASK_EXECUTABLE" task push before reporting delivery. This
pushes only the bound branch; it does not commit files, merge or deploy. A failed
push retains local work; report the concrete access/conflict blocker. Never
print credentials or force-push.
Definition of Done & Automated Deployment:
For projects configured with auto_deploy: true, the Definition of Done (DoD) strictly requires
completing implementation, syntax/build verification, executing live deployment, and
conducting live smoke tests. Pushing a task branch or local commit is only an intermediate step;
marking a task as Succeeded/Done based merely on commit/push without live release and verified deployment is strictly forbidden.
Delivery requires verified live production readiness. For substantial work, publish meaningful task report --progress "..." checkpoints.
Deployment Branch Guardrail: For any project, when releasing or deploying changes, all commits must be merged to the project's configured deployment branch (CUAN_DEPLOY_BRANCH or cuan.deployBranch in git config) before executing deployment. Direct deployment from temporary task branches (cuan/task-*) is strictly prohibited.
Use the repository's maintained deployment entry point and its release integrity gates. Never replace it with hand-written release assembly, direct current/previous symlink switches, manually changed schema counts, skipped branch-sync checks or --allow-divergent. A missing Git object or access error is a publication prerequisite to reconcile, not permission to disable the gate. Preserve the local commits and exact error, save a factual checkpoint, and report the remaining publication gap. Do not infer missing code from a compiled binary's strings or symbol names; inspect the actual source, revision and behavior.

## Mandatory Task Agent Directives (Product Owner Mandate)
1. Lean and Modular Implementation:
Keep changes focused, modular and efficient. Use relevant targeted checks and report actual timings; do not change unrelated code to satisfy an invented timing threshold.
2. Project Documentation:
Update relevant documentation and product pages when required by this task's approved scope and repository guidance. Never introduce Cuan-specific files or requirements into another project. A file's existence alone does not establish that documentation is correct or current.
3. Internal DoD Evaluator Compliance:
Cuan's autonomous DoD Evaluator records applicable workspace checks. The coordinator separately reviews the actual original requirements, including documentation, visual quality and publication. Passing workspace checks does not establish every acceptance criterion.

Print the user-facing answer before the final report.
You are the task worker, not the Owner. Never use chat send, send_inbox_message,
create_inbox_conversation or owner task-update tools to deliver your answer or
approve work. Those tools create owner instructions, not worker reports. Publish
progress/results only through the task reporting capability below. When using
MCP, get_current_task, report_current_task and push_current_task use that same
capability without task IDs; use report_current_task for checkpoints/results. The Inbox
coordinator delivers the result with the assistant identity after verification.
When the requested result is a downloadable file, attach each deliverable to the
final report with --artifact "relative/path" (repeatable, up to eight files, 10 MB
per file and 25 MB total). Paths must be relative to WORKSPACE_DIR; do not attach
credentials, hidden files or private keys. Cuan copies the files into private
Inbox attachments so the user can download them without server access. Merely
printing a file:// link or server path does not deliver a file. Examples below show
transport flags; when the task has saved criteria, also supply --report-file with
the complete current dod_checklist as described below. Example:
"$CUAN_TASK_EXECUTABLE" task report --state done --summary "Files ready" --complete-note "Actual verification" --artifact "output.html" --artifact "README.md"
For verification without modifying the repository, write screenshots or reports
under "$CUAN_TASK_ARTIFACTS_DIR". This is a private output directory bound to this
attempt, outside the repository. Attach those files using session-relative paths:
"$CUAN_TASK_EXECUTABLE" task report --state review --summary "Visual checks" --complete-note "Observed findings" --artifact "session:mobile.png" --artifact "session:desktop.png"
The session prefix selects only the bound output directory, never an arbitrary
server path. Do not copy secrets, credentials or hidden files into deliverables.
For completed implementation, finish with:
"$CUAN_TASK_EXECUTABLE" task report --state done --summary "Actual result" --complete-note "Verification and limitations"
For a greeting/question with no file changes, use --state review --result-kind answer.
Use --state review when implementation still needs review.
For implementation, file changes or uncertain classification, use --result-kind implementation (the default).
Use task report-template (or prepare_current_task_report in MCP) to obtain the
original criteria as review JSON with empty evidence, rather than reconstructing
their IDs and text. Fill actual findings before submission. Read task get when
additional scope/context is needed or a report is uncertain/rejected. Both review and done require current
observations for EVERY saved acceptance criterion, preserving its text, ID and
order. Previous attempts' checklist evidence cannot substitute for this report.
Supply completed:true only for verified criteria, with a concrete "evidence"
string (observed result, command output, artifact or live check). For unmet
criteria, use completed:false and a factual gap in a review report; for a blocker
use state blocked. Done requires actual completed work and verification evidence.
If final reporting is rejected, correct and resubmit within this same active
attempt; do not stop or start another task.
Use task report --report-file with JSON containing dod_checklist to send these
structured observations. Do not mark unchecked criteria complete implicitly,
remove criteria, or add revision instructions to the approved checklist.
For long work, publish task report --progress checkpoints before risky or slow
steps and after each verified milestone. On continuation inspect the same task
workspace and checkpoints; reconcile uncertain external effects before repeating.
Retry only unfinished approved work; retain verified artifacts and branch.
For a greeting/question, put the actual useful answer in --summary, not a meta-description such as "Responded to the message". Explain in --complete-note that no files/tests were needed. Never claim unperformed work. For a blocker use --state blocked
with summary and concrete evidence instead.
A final report closes this session automatically after output drains.
Human instructions are read-only. Never print the private
capability file. Need structured metadata? task --help documents --report-file.
`
	detail, detailErr := s.taskSvc.GetTaskDetail(ctx, taskID)
	if detailErr != nil || detail == nil {
		cleanup()
		return nil, "", "", none, errors.New("The original task reporting context is unavailable")
	}
	// Prepare the same blank writable report as the scoped CLI, once at launch.
	// No current or historical completion claim becomes a default observation.
	var currentAttempt any
	if detail.LatestAttempt != nil {
		currentAttempt = map[string]string{"id": detail.LatestAttempt.ID}
	}
	currentJSON, e := json.Marshal(map[string]any{"id": detail.ID, "latest_attempt": currentAttempt, "dod_checklist": detail.DoDChecklist})
	if e != nil {
		cleanup()
		return nil, "", "", none, e
	}
	reportJSON, e := taskcli.PrepareReportTemplate(currentJSON, c)
	if e != nil {
		cleanup()
		return nil, "", "", none, e
	}
	reportFile := c.ReportFile
	if e = os.WriteFile(reportFile, reportJSON, 0600); e != nil {
		cleanup()
		return nil, "", "", none, e
	}
	artifactOnly := executionpolicy.StandaloneArtifactWork(detail.Instructions)
	if artifactOnly {
		instructions = artifactSessionGuidance(detail.DoDChecklist)
	} else {
		if len(detail.DoDChecklist) > 0 {
			var dodBuilder strings.Builder
			dodBuilder.WriteString("\n## Acceptance Criteria (Definition of Done)\nThe following criteria must be fulfilled and verified before reporting completion:\n")
			for i, item := range detail.DoDChecklist {
				dodBuilder.WriteString(fmt.Sprintf("%d. %s\n", i+1, item.Text))
			}
			instructions = dodBuilder.String() + "\n" + instructions
		}
		// Entry points supply the actual repository guidance once. This private
		// guide carries reporting/workflow context, never the API host's AGENTS.
		instructions = workflow.Developer + "\n\n" + instructions
		if techTaste := s.formatTechnicalTastePrompt(ctx); techTaste != "" {
			instructions += "\n\n" + techTaste + "\n"
		}
	}
	for _, part := range contextParts {
		if strings.TrimSpace(part) != "" {
			instructions += "\n" + part + "\n"
		}
	}
	instructions += "\n## Prepared report file\nA fresh report template is already available at CUAN_TASK_REPORT_FILE for this exact task/attempt. It preserves every original criterion with empty evidence and completed=false. Fill that existing JSON with your factual summary, complete_note and observed criterion evidence, then submit:\n\"$CUAN_TASK_EXECUTABLE\" task report --report-file \"$CUAN_TASK_REPORT_FILE\"\nAdd --artifact for each actual deliverable. Do not run help, inspect environment variables, or fetch another template just to rediscover this reporting format. Use scoped task get if actual task context is missing or a report is uncertain/rejected. Template preparation does not submit or accept work.\n"
	guide := filepath.Join(dir, "session.md")
	if e = os.WriteFile(guide, []byte(instructions), 0600); e != nil {
		cleanup()
		return nil, "", "", none, e
	}
	prompt := "\n\nRead the session guidance at " + guide + " before proceeding."
	if artifactOnly {
		prompt = "\n\n" + instructions
	}
	return []string{"CUAN_TASK_CAPABILITY_FILE=" + file, "CUAN_TASK_EXECUTABLE=" + executable, "CUAN_TASK_ARTIFACTS_DIR=" + evidenceDir, "CUAN_TASK_REPORT_FILE=" + reportFile, "CUAN_TASK_CAPABILITY_JSON="}, prompt, token, cleanup, nil

}
func (s *Server) handleTaskAgent(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	digest := sha256.Sum256([]byte(raw))
	value, ok := s.taskAgentGrants.Load(digest)
	// Only a previously issued authenticated record can restore a grant after
	// restart. Unknown tokens still fail closed, regardless of their length.
	if !ok && raw != "" {
		if restored, err := s.loadTaskGrant(digest); err == nil {
			value, ok = restored, true
		}
	}

	if !ok || raw == "" {
		respondJSONError(w, 401, "Task session access is unavailable")
		return
	}
	g := value.(taskAgentGrant)
	if time.Now().After(g.Expires) {
		s.forgetTaskGrant(digest)
		respondJSONError(w, 401, "Task session access expired")
		return
	}
	if chi.URLParam(r, "id") != g.TaskID {
		respondJSONError(w, 403, "This session can access only its bound task")
		return
	}
	latest, e := s.taskSvc.GetTaskDetail(r.Context(), g.TaskID)
	if e != nil || latest.LatestAttempt == nil || latest.LatestAttempt.ID != g.AttemptID || latest.LatestAttempt.Status != task.AttemptStatusRunning {
		respondJSONError(w, 409, "This task attempt is no longer active")
		return
	}
	if r.Method == "POST" && (strings.HasSuffix(r.URL.Path, "/push") || strings.HasSuffix(r.URL.Path, "/publish")) {
		limits, policyErr := s.taskSvc.AttemptPublicationPolicy(r.Context(), g.TaskID, g.AttemptID)
		if policyErr != nil {
			respondJSONError(w, 503, "Task publication limits are unavailable")
			return
		}
		if limits.DenyPush {
			respondJSONError(w, 403, "This task scope forbids pushing. Local work is preserved.")
			return
		}
		if s.projectSvc == nil {
			respondJSONError(w, 503, "Project access is unavailable")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		root, closeRoot, err := s.taskExecutionRoot(ctx, g, latest)
		if err != nil {
			respondJSONError(w, 409, err.Error())
			return
		}
		defer closeRoot()
		var commit string
		var pr *git.PullRequestResult
		branch := latest.BranchName
		publish := strings.HasSuffix(r.URL.Path, "/publish")
		if root != nil && g.ArtifactRoot != nil && root != g.ArtifactRoot {
			if branch == "" {
				branch = "task/" + latest.ID
			}
			commit, pr, err = s.projectSvc.PushNativeTaskBranch(ctx, latest.ID, latest.ProjectID, latest.Instructions, root.Name(), branch, publish, latest.Title)
		} else if publish {
			err = errors.New("The native execution workspace could not be verified. Local work is preserved")
		} else {
			commit, err = s.projectSvc.PushTaskBranch(ctx, g.TaskID, latest.ProjectID, latest.Instructions)
		}
		if err != nil {
			if errors.Is(err, project.ErrGitHubConnectionRequired) {
				if e := s.taskSvc.RecordGitHubPrerequisite(ctx, g.TaskID, g.AttemptID, false); e != nil {
					respondJSONError(w, 503, "Could not retain the GitHub connection requirement. Local work is unchanged.")
					return
				}
				respondJSON(w, 409, map[string]any{"error": err.Error(), "code": "github_connection_required", "connect_url": "/settings/integrations/github?return_to=/tasks/" + g.TaskID})
				return
			}
			respondJSONError(w, 409, err.Error())
			return
		}
		_ = s.taskSvc.RecordGitHubPrerequisite(ctx, g.TaskID, g.AttemptID, true)
		respondJSON(w, 200, map[string]any{"commit_sha": commit, "branch": branch, "pull_request": pr, "pushed": true, "scope": "bound task branch", "uncommitted_files_saved": false})
		return
	}
	if r.Method == "GET" {
		view := *latest
		view.Instructions = project.TaskPromptInstructions(latest.Instructions)
		respondJSON(w, 200, struct {
			*task.TaskDetailResponse
			GitHubConnection         map[string]any `json:"github_connection"`
			PublicationConfiguration string         `json:"publication_configuration"`
		}{&view, s.boundTaskGitHubConnection(r.Context(), latest), s.publicationConfigurationForTask(r.Context(), latest)})
		return
	}
	var report struct {
		task.AgentReport
		Artifacts []string `json:"artifacts,omitempty"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64000))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&report); e != nil {
		respondJSONError(w, 400, "Invalid report. Human instructions, assignment and approval are read-only for agents.")
		return
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		respondJSONError(w, 400, "Send one JSON report per request")
		return
	}
	var artifacts []preparedTaskArtifact
	artifactOwner := ""
	if len(report.Artifacts) > 0 {
		if report.State != "done" && report.State != "review" {
			respondJSONError(w, 400, "Attach files to a final done or review report")
			return
		}
		artifactOwner, e = s.taskArtifactOwner(r.Context(), latest)
		if e != nil {
			respondJSONError(w, 409, e.Error())
			return
		}
		artifactRoot, closeRoot := g.ArtifactRoot, func() {}
		for _, path := range report.Artifacts {
			if !strings.HasPrefix(path, "session:") {
				artifactRoot, closeRoot, e = s.taskExecutionRoot(r.Context(), g, latest)
				if e != nil {
					respondJSONError(w, 409, e.Error())
					return
				}
				break
			}
		}
		defer closeRoot()
		artifacts, e = readTaskArtifacts(artifactRoot, g.TaskID, g.AttemptID, report.Artifacts, g.EvidenceRoot)
		if e != nil {
			respondJSONError(w, 400, e.Error())
			return
		}
	}
	if r.Context().Err() != nil {
		return // No accepted write to finish after an already disconnected request.
	}
	// Once the scoped report and its files are decoded, give persistence a
	// bounded completion window independent of the client's connection. This
	// grants no execution time and never bypasses the active-attempt checks.
	reportCtx, cancelReport := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
	defer cancelReport()
	saveReport := s.taskSvc.ReportAgent
	if len(artifacts) > 0 {
		saveReport = s.taskSvc.StageAgentReport
	}
	updated, e := saveReport(reportCtx, g.TaskID, g.AttemptID, g.AgentID, report.AgentReport)
	if e != nil {
		code := 500
		if errors.Is(e, task.ErrInvalidTaskInput) {
			code = 400
		}
		if errors.Is(e, task.ErrAttemptFinal) || errors.Is(e, task.ErrRequestConflict) {
			code = 409
		}
		respondJSONError(w, code, e.Error())
		return
	}
	if e = s.publishTaskArtifacts(reportCtx, latest, artifactOwner, artifacts); e != nil {
		respondJSONError(w, 503, "The result checkpoint is saved, but file delivery was not confirmed. Retry the same report before exiting.")
		return
	}
	if len(artifacts) > 0 {
		updated, e = s.taskSvc.ReportAgent(reportCtx, g.TaskID, g.AttemptID, g.AgentID, report.AgentReport)
		if e != nil {
			code := http.StatusServiceUnavailable
			if errors.Is(e, task.ErrAttemptFinal) || errors.Is(e, task.ErrRequestConflict) {
				code = http.StatusConflict
			}
			respondJSONError(w, code, "Files were delivered, but the final report was not confirmed. Inspect the current task before retrying the same report.")
			return
		}
	}
	if report.State == "" {
		report.State = "progress"
	}
	workflow := "Result is reconciled after process exit"
	if report.State == "progress" {
		workflow = "Checkpoint saved. Continue the approved task. Submit an explicit review, blocked or done state with summary and completion evidence for the final result."
	}
	// A write receipt confirms identity and state, not a second copy of the
	// approved instructions and evidence. Workers can explicitly GET the task
	// when they need its canonical context again.
	respondJSON(w, 200, map[string]any{
		"task":       map[string]any{"id": updated.ID, "status": updated.Status, "updated_at": updated.UpdatedAt},
		"attempt_id": g.AttemptID, "report_state": report.State, "workflow": workflow,
	})
	if updated != nil {
		if updated.SourceRoomID != "" {
			s.publishInbox(updated.SourceRoomID)
		}
		if s.taskSvc != nil {
			if launch, err := s.taskSvc.LatestLaunch(context.Background(), g.TaskID); err == nil && launch != nil && launch.RoomID != "" && launch.RoomID != updated.SourceRoomID {
				s.publishInbox(launch.RoomID)
			}
		}
	}
	if report.State == "review" || report.State == "blocked" || report.State == "done" {
		// A report is a claim, not acceptance. Publication must not race the
		// runner's process-exit, repository verification and deployment gates.
		// Return the tool response before closing its parent PTY. Never stop a later
		// attempt or convert an independently failed process into successful work.
		acceptedAt := time.Now()
		time.AfterFunc(2*time.Second, func() {
			latest, err := s.taskSvc.GetTaskDetail(context.Background(), g.TaskID)
			if err != nil {
				return
			}
			for _, a := range latest.Activities {
				if a.AttemptID != nil && *a.AttemptID == g.AttemptID && a.ActivityType == "agent_report" && a.CreatedAt.After(acceptedAt) {
					return
				}
			}
			if s.terminalManager != nil {
				s.terminalManager.FinishReportedSession(g.TaskID, g.AttemptID, acceptedAt)
			}
		})
	}

}
