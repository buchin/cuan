package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/taskcli"
	"github.com/buchin/cuan/internal/workflow"
	"github.com/go-chi/chi/v5"
)

type taskAgentGrant struct {
	TaskID, AttemptID, AgentID string
	Expires                    time.Time
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
	cleanup := func() { s.taskAgentGrants.Delete(digest); _ = os.RemoveAll(dir) }
	base := s.taskAgentURL
	if base == "" {
		port := s.cfg.ServerPort
		if port == "" {
			port = "8080"
		}
		base = "http://127.0.0.1:" + port
	}
	c := taskcli.Capability{API: base + "/api/v1/task-agent/" + taskID, Token: token, TaskID: taskID, AttemptID: attemptID}
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
	s.taskAgentGrants.Store(digest, taskAgentGrant{taskID, attemptID, agentID, time.Now().Add(8*time.Hour + 5*time.Minute)})
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
Do not inspect, reverse-engineer, or search the Cuan server binary to understand
this reporting tool. If the tool fails, report its actual error as a blocker.

## Task reporting
The built-in command is "$CUAN_TASK_EXECUTABLE" task.
Run task get only when additional task context is needed. Metadata and checklists
are optional; keep simple requests simple. Use task --help for supported flags.
For repository implementation, commit only your scoped changes. Save the task
branch with "$CUAN_TASK_EXECUTABLE" task push before reporting delivery. This
pushes only the bound branch; it does not commit files, merge or deploy. A failed
push retains local work; report the concrete access/conflict blocker. Never
print credentials or force-push. For substantial work, publish meaningful task report --progress "..." checkpoints.
Print the user-facing answer before the final report.
For completed implementation, finish with:
"$CUAN_TASK_EXECUTABLE" task report --state done --summary "Actual result" --complete-note "Verification and limitations"
For a greeting/question with no file changes, use --state review --result-kind answer.
Use --state review when implementation still needs review.
For implementation, file changes or uncertain classification, use --result-kind implementation (the default).
Done requires actual completed work, verification evidence and all declared criteria completed.
For a greeting/question, put the actual useful answer in --summary, not a meta-description such as "Responded to the message". Explain in --complete-note that no files/tests were needed. Never claim unperformed work. For a blocker use --state blocked
with summary and concrete evidence instead.
A final report closes this session automatically after output drains.
Human instructions are read-only. Never print the private
capability file. Need structured metadata? task --help documents --report-file.
`
	instructions = workflow.Developer + "\n\n" + instructions
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
	return []string{"CUAN_TASK_CAPABILITY_FILE=" + file, "CUAN_TASK_EXECUTABLE=" + executable}, prompt, token, cleanup, nil

}
func (s *Server) handleTaskAgent(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	digest := sha256.Sum256([]byte(raw))
	value, ok := s.taskAgentGrants.Load(digest)
	if !ok || raw == "" {
		respondJSONError(w, 401, "Task session access is unavailable")
		return
	}
	g := value.(taskAgentGrant)
	if time.Now().After(g.Expires) {
		s.taskAgentGrants.Delete(digest)
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
		respondJSON(w, 200, latest)
		return
	}
	var report task.AgentReport
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
	updated, e := s.taskSvc.ReportAgent(r.Context(), g.TaskID, g.AttemptID, g.AgentID, report)
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
	respondJSON(w, 200, map[string]any{"task": updated, "report_state": report.State, "workflow": "Result is reconciled after process exit"})
	if report.State == "review" || report.State == "blocked" || report.State == "done" {
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
