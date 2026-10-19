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

	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/taskcli"
	"github.com/go-chi/chi/v5"
)

type taskAgentGrant struct {
	TaskID, AttemptID, AgentID string
	Expires                    time.Time
}

func (s *Server) issueTaskAgent(ctx context.Context, taskID, attemptID, agentID string) ([]string, string, string, func(), error) {
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
	s.taskAgentGrants.Store(digest, taskAgentGrant{taskID, attemptID, agentID, time.Now().Add(11 * time.Minute)})
	prompt := "\n\n" + `Task reporting tool (mandatory):
The self-contained command is "$CUAN_TASK_EXECUTABLE" task. Run it through your terminal tool. Use task --help for flags.
1. Run "$CUAN_TASK_EXECUTABLE" task get. Refine the task title/priority, define appropriate acceptance criteria, and publish a short plan using task report --summary "..." --progress "...".
2. Send meaningful progress checkpoints using task report --progress "...". Preserve the original human instructions.
3. Before finishing, send task report --state review --summary "What changed" --complete-note "Actual verification, results, limitations". For a blocker use --state blocked with summary and concrete blocker evidence.
Use --report-file for JSON reports including dod_checklist [{"text":"...","completed":false}], commit_sha, and test_summary. Only mark criteria completed after verifying them. Do not invent evidence. This tool can update only this task during this active attempt; it cannot edit human instructions or approve Done.
A successful process without a complete review report leaves the task Blocked. Review is published only after the process ends. Do not print or copy the private capability file.
`
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
}
