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
	"github.com/buchin/cuan/internal/project"
	cuanPrompt "github.com/buchin/cuan/internal/prompt"
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
	c := taskcli.Capability{API: base + "/api/v1/task-agent/" + taskID, Token: token, TaskID: taskID, AttemptID: attemptID, ArtifactsDir: evidenceDir}
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

Repository access: the runner prepares your bound repository checkout before
starting this session. Inspect that checkout and preserve its existing files;
do not clone a duplicate merely because a standalone gh CLI is not signed in.
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

## Mandatory Task Agent Directives (Product Owner Mandate)
1. Lean, Modular & Lightning Fast Execution (<10 Seconds):
Every refactoring or feature implementation must remain lean and modular. Prevent monolithic files, bloated structures, or dead code. Maintain modular structure so that targeted tests and compilation builds run in under 10 seconds.
2. Synchronous Docs & Landing Page Update for New Capabilities:
Whenever introducing or expanding capabilities or new features, the task agent MUST ensure that technical documentation (docs/, README) and the Cuan Landing Page (web/src/features/landing/LandingPage.tsx) are updated to accurately reflect the latest system state and business value for online store owners.
3. Internal DoD Evaluator Compliance:
Cuan's autonomous DoD Evaluator actively validates that the working tree is clean, targeted tests pass, build has 0 errors, lean standards are upheld, and docs/LP updates are present before a task can transition to done.

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
printing a file:// link or server path does not deliver a file. Example:
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
Done requires actual completed work and verification evidence. For each original
acceptance criterion, supply its unchanged text/id, completed:true and a concrete
"evidence" string (observed result, command output, artifact or live check).
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
	if detail, err := s.taskSvc.GetTaskDetail(ctx, taskID); err == nil && detail != nil && len(detail.DoDChecklist) > 0 {
		var dodBuilder strings.Builder
		dodBuilder.WriteString("\n## Acceptance Criteria (Definition of Done)\nThe following criteria must be fulfilled and verified before reporting completion:\n")
		for i, item := range detail.DoDChecklist {
			dodBuilder.WriteString(fmt.Sprintf("%d. %s\n", i+1, item.Text))
		}
		dodBuilder.WriteString("\n")
		instructions = dodBuilder.String() + instructions
	}
	instructions = workflow.Developer + "\n\n" + cuanPrompt.GetRunnerPromptAdditions() + "\n\n" + instructions
	if techTaste := s.formatTechnicalTastePrompt(ctx); techTaste != "" {
		instructions += "\n\n" + techTaste + "\n"
	}
	for _, part := range contextParts {
		if strings.TrimSpace(part) != "" {
			instructions += "\n" + part + "\n"
		}
	}
	guide := filepath.Join(dir, "session.md")
	if e = os.WriteFile(guide, []byte(instructions), 0600); e != nil {
		cleanup()
		return nil, "", "", none, e
	}
	prompt := "\n\nRead the session guidance at " + guide + " before proceeding."
	return []string{"CUAN_TASK_CAPABILITY_FILE=" + file, "CUAN_TASK_EXECUTABLE=" + executable, "CUAN_TASK_ARTIFACTS_DIR=" + evidenceDir}, prompt, token, cleanup, nil

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
	if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/push") {
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
		commit, err := s.projectSvc.PushTaskBranch(ctx, g.TaskID, latest.ProjectID, latest.Instructions)
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
		respondJSON(w, 200, map[string]any{"commit_sha": commit, "pushed": true, "scope": "bound task branch", "uncommitted_files_saved": false})
		return
	}
	if r.Method == "GET" {
		respondJSON(w, 200, struct {
			*task.TaskDetailResponse
			GitHubConnection map[string]any `json:"github_connection"`
		}{latest, s.boundTaskGitHubConnection(r.Context(), latest)})
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
	if report.State == "" {
		report.State = "progress"
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
		artifacts, e = readTaskArtifacts(g.ArtifactRoot, g.TaskID, g.AttemptID, report.Artifacts, g.EvidenceRoot)
		if e != nil {
			respondJSONError(w, 400, e.Error())
			return
		}
	}
	saveReport := s.taskSvc.ReportAgent
	if len(artifacts) > 0 {
		saveReport = s.taskSvc.StageAgentReport
	}
	updated, e := saveReport(r.Context(), g.TaskID, g.AttemptID, g.AgentID, report.AgentReport)
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
	if e = s.publishTaskArtifacts(r.Context(), latest, artifactOwner, artifacts); e != nil {
		respondJSONError(w, 503, "The result checkpoint is saved, but file delivery was not confirmed. Retry the same report before exiting.")
		return
	}
	if len(artifacts) > 0 {
		updated, e = s.taskSvc.ReportAgent(r.Context(), g.TaskID, g.AttemptID, g.AgentID, report.AgentReport)
		if e != nil {
			code := http.StatusServiceUnavailable
			if errors.Is(e, task.ErrAttemptFinal) || errors.Is(e, task.ErrRequestConflict) {
				code = http.StatusConflict
			}
			respondJSONError(w, code, "Files were delivered, but the final report was not confirmed. Inspect the current task before retrying the same report.")
			return
		}
	}
	respondJSON(w, 200, map[string]any{"task": updated, "report_state": report.State, "workflow": "Result is reconciled after process exit"})
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
