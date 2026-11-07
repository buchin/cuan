package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

type artifactDeliveryRepository interface {
	db.ProjectStudioRepository
	db.InboxRequestRepository
}

type observedArtifactDeliveryRepository struct {
	artifactDeliveryRepository
	beforeDelivery func(context.Context) error
}

func (r observedArtifactDeliveryRepository) CreateMessage(ctx context.Context, m *db.ProjectMessage) error {
	if r.beforeDelivery != nil {
		if err := r.beforeDelivery(ctx); err != nil {
			return err
		}
	}
	return r.artifactDeliveryRepository.CreateMessage(ctx, m)
}

func TestTaskArtifactsCannotEscapeOrExposePrivateFiles(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "private.txt")
	os.WriteFile(outside, []byte("PRIVATE_OUTSIDE"), 0600)
	os.WriteFile(filepath.Join(dir, "report.html"), []byte("<p>Delivered</p>"), 0600)
	os.WriteFile(filepath.Join(dir, ".env"), []byte("PRIVATE_ENV"), 0600)
	os.Symlink(outside, filepath.Join(dir, "escape.txt"))
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for _, path := range []string{"../private.txt", outside, ".env", "nested/.git/config", "secret.pem", "escape.txt"} {
		if _, err := readTaskArtifacts(root, "task", "attempt", []string{path}); err == nil {
			t.Fatal("private/escaped artifact accepted", path)
		}
	}
	files, err := readTaskArtifacts(root, "task", "attempt", []string{"report.html"})
	if err != nil || len(files) != 1 {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "report.html"), []byte("CHANGED_AFTER_SNAPSHOT"), 0600)
	if string(files[0].data) != "<p>Delivered</p>" {
		t.Fatal("snapshot changed with source")
	}
	if _, err := readTaskArtifacts(root, "task", "attempt", []string{"report.html", "report.html"}); err == nil {
		t.Fatal("duplicate path accepted")
	}
	os.WriteFile(filepath.Join(dir, "large.txt"), nil, 0600)
	os.Truncate(filepath.Join(dir, "large.txt"), attachmentLimit+1)
	if _, err := readTaskArtifacts(root, "task", "attempt", []string{"large.txt"}); err == nil {
		t.Fatal("oversize file accepted")
	}
}

func TestTaskArtifactReportDeliversPrivateDownloadInInbox(t *testing.T) {
	for _, session := range []bool{false, true} {
		name := "workspace"
		if session {
			name = "session outside repository"
		}
		t.Run(name, func(t *testing.T) { taskArtifactDeliveryContract(t, session) })
	}
	t.Run("identical workspace and session copies", func(t *testing.T) {
		taskArtifactDeliveryContract(t, false, false, true)
	})
}

func TestTaskArtifactsCoalesceIdenticalNamedCopiesAcrossRoots(t *testing.T) {
	workspace, session := t.TempDir(), t.TempDir()
	for _, dir := range []string{workspace, session} {
		if err := os.WriteFile(filepath.Join(dir, "result.txt"), []byte("verified\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	evidence, err := os.OpenRoot(session)
	if err != nil {
		t.Fatal(err)
	}
	defer evidence.Close()
	files, err := readTaskArtifacts(root, "task", "attempt", []string{"result.txt", "session:result.txt"}, evidence)
	if err != nil || len(files) != 1 || string(files[0].data) != "verified\n" {
		t.Fatalf("duplicate delivery: %v, %#v", err, files)
	}
	// Different requested names or changed bytes are distinct deliverables.
	os.WriteFile(filepath.Join(session, "other.txt"), []byte("verified\n"), 0600)
	os.WriteFile(filepath.Join(session, "result.txt"), []byte("revised\n"), 0600)
	files, err = readTaskArtifacts(root, "task", "attempt", []string{"result.txt", "session:result.txt", "session:other.txt"}, evidence)
	if err != nil || len(files) != 3 {
		t.Fatalf("distinct results lost: %v, %d", err, len(files))
	}
	// A valid copy never excuses an inaccessible second source.
	if _, err := readTaskArtifacts(root, "task", "attempt", []string{"result.txt", "session:missing.txt"}, evidence); err == nil {
		t.Fatal("unavailable source hidden by deduplication")
	}
}
func taskArtifactDeliveryContract(t *testing.T, session bool, native ...bool) {
	taskArtifactDeliveryFormatContract(t, session, "html", native...)
}

func TestTaskArtifactJSONContentReachesIndependentReview(t *testing.T) {
	taskArtifactDeliveryFormatContract(t, false, "json")
}

func taskArtifactDeliveryFormatContract(t *testing.T, session bool, format string, native ...bool) {
	s, token, call, _ := inboxFixture(t)
	ctx := context.Background()
	owner, err := s.authSvc.ValidateToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	now := time.Now().UTC()
	q := db.InboxRequest{OwnerID: owner.ID, AgentID: "agent_lifecycle", RoomID: room.RoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "completed", CreatedAt: now, Deadline: now.Add(time.Hour), PayloadHash: db.GenerateUUID()}
	_, _, err = s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: "Create a downloadable HTML file", CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := s.taskSvc.CreateTaskFromMessage(ctx, owner.ID, q.MessageID, task.CreateTaskRequest{SourceRoomID: q.RoomID, ProjectID: "default", Title: "Deliver HTML", Instructions: "Approved HTML output", AssignedAgent: q.AgentID})
	if err != nil {
		t.Fatal(err)
	}
	runnerID := "local_terminal"
	if len(native) > 0 && native[0] {
		runnerID = "artifact-native"
	}
	a, err := s.taskSvc.CreateAttempt(ctx, saved.ID, owner.ID, task.CreateAttemptRequest{RunnerID: runnerID, AgentName: q.AgentID})
	if err != nil {
		t.Fatal(err)
	}
	doing := task.TaskStatusDoing
	s.taskSvc.UpdateTask(ctx, saved.ID, owner.ID, task.UpdateTaskRequest{Status: &doing})
	s.taskSvc.UpdateAttemptStatus(ctx, a.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning})
	workspace := t.TempDir()
	original := "<!doctype html><title>Offline output</title><p>Works offline</p>" + strings.Repeat("<!-- long report body -->", 400) + "<p>FINAL_VERIFICATION_RESULT</p>"
	filename := "output." + format
	if format == "json" {
		raw, err := json.Marshal(map[string]string{"a_title": "Offline output", "data": strings.Repeat("structured content ", 500), "z_result": "FINAL_VERIFICATION_RESULT"})
		if err != nil {
			t.Fatal(err)
		}
		original = string(raw)
	}
	os.WriteFile(filepath.Join(workspace, filename), []byte(original), 0600)
	env, _, capability, cleanup, err := s.issueTaskAgent(context.WithValue(ctx, taskAgentWorkspaceKey{}, workspace), saved.ID, a.ID, q.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	sourceFile := filepath.Join(workspace, filename)
	if len(native) > 0 && native[0] {
		s.cfg.WorkspacesDir = t.TempDir()
		projects := project.NewMemoryStore()
		if err := projects.CreateProject(ctx, &project.Project{ID: "default", Name: "Fixture", LocalPath: workspace}); err != nil {
			t.Fatal(err)
		}
		s.projectSvc = project.NewService(projects, nil, "fixture", t.TempDir())
		s.runnerSvc = runner.NewService(runner.NewMemoryStore(s.taskSvc.Store()), s.taskSvc.Store())
		if _, err := s.runnerSvc.Heartbeat(ctx, runner.HeartbeatRequest{RunnerID: runnerID, Hostname: "fixture", CurrentAttemptID: a.ID}, time.Minute); err != nil {
			t.Fatal(err)
		}
		worker := filepath.Join(s.cfg.WorkspacesDir, saved.ID)
		if err := os.MkdirAll(filepath.Join(worker, ".git"), 0700); err != nil {
			t.Fatal(err)
		}
		// Both directories contain the same filename, with different content.
		// The delivered attachment must come from the actual worker checkout.
		os.WriteFile(sourceFile, []byte("STALE PROJECT CLONE OUTPUT"), 0600)
		sourceFile = filepath.Join(worker, filename)
		os.WriteFile(sourceFile, []byte(original), 0600)
	}
	paths := `["` + filename + `"]`
	duplicate := len(native) > 1 && native[1]
	if session || duplicate {
		var directory string
		for _, item := range env {
			if strings.HasPrefix(item, "CUAN_TASK_ARTIFACTS_DIR=") {
				directory = strings.TrimPrefix(item, "CUAN_TASK_ARTIFACTS_DIR=")
			}
		}
		if directory == "" || strings.HasPrefix(directory, workspace+string(filepath.Separator)) {
			t.Fatal("missing isolated evidence directory")
		}
		if session {
			os.Remove(sourceFile)
			sourceFile = filepath.Join(directory, filename)
		}
		if err := os.WriteFile(filepath.Join(directory, filename), []byte(original), 0600); err != nil {
			t.Fatal(err)
		}
		paths = `["session:` + filename + `"]`
		if duplicate {
			paths = `["` + filename + `","session:` + filename + `"]`
		}
		// Reopen both roots from the durable encrypted grant, as after restart.
		s.forgetTaskGrant(sha256.Sum256([]byte(capability)))
	}
	var disconnectReport context.CancelFunc
	report := func(paths string, disconnected ...bool) *httptest.ResponseRecorder {
		request := httptest.NewRequest("POST", "/api/v1/task-agent/"+saved.ID, strings.NewReader(`{"state":"done","summary":"HTML ready","complete_note":"Verified offline file","artifacts":`+paths+`}`))
		requestCtx, cancel := context.WithCancel(request.Context())
		defer cancel()
		disconnectReport = cancel
		request = request.WithContext(requestCtx)
		if len(disconnected) > 0 && disconnected[0] {
			cancel()
		}
		request.Header.Set("Authorization", "Bearer "+capability)
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, request)
		return w
	}
	if w := report(`["../private.txt"]`); w.Code != 400 {
		t.Fatal("escaped output accepted", w.Code, w.Body.String())
	}
	report(paths, true)
	beforeDelivery, err := s.taskSvc.GetTaskDetail(ctx, saved.ID)
	if err != nil || beforeDelivery.Summary != "" || beforeDelivery.CompleteNote != "" {
		t.Fatal("disconnected request started writing a report", err)
	}
	deliveryCalls := 0
	originalRepository := s.projectStudioRepo
	s.projectStudioRepo = observedArtifactDeliveryRepository{
		artifactDeliveryRepository: originalRepository.(artifactDeliveryRepository),
		beforeDelivery: func(deliveryCtx context.Context) error {
			ready, err := s.taskSvc.AgentReviewReady(ctx, saved.ID, a.ID)
			if err != nil || ready {
				t.Fatal("runner could stop before file delivery was confirmed", ready, err)
			}
			// The report was accepted. Losing its client connection must not
			// cancel delivery or the final persistence that follows it.
			disconnectReport()
			if deliveryCtx.Err() != nil {
				t.Fatal("accepted report still depends on the client connection")
			}
			if deadline, ok := deliveryCtx.Deadline(); !ok || time.Until(deadline) > 5*time.Second {
				t.Fatal("accepted report has no bounded finalization deadline")
			}
			deliveryCalls++
			if deliveryCalls == 1 {
				return errors.New("fixture: temporary delivery failure")
			}
			return nil
		},
	}
	if w := report(paths); w.Code != 503 {
		t.Fatal("delivery failure was not retryable", w.Code, w.Body.String())
	}
	if ready, err := s.taskSvc.AgentReviewReady(ctx, saved.ID, a.ID); err != nil || ready {
		t.Fatal("failed file delivery became a final report", ready, err)
	}
	if w := report(paths); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if ready, err := s.taskSvc.AgentReviewReady(ctx, saved.ID, a.ID); err != nil || !ready {
		t.Fatal("confirmed file delivery did not finalize the report", ready, err)
	}
	s.projectStudioRepo = originalRepository
	if w := report(paths); w.Code != 200 {
		t.Fatal("identical delivery was not idempotent", w.Code, w.Body.String())
	}
	detail, _ := s.taskSvc.GetTaskDetail(ctx, saved.ID)
	var assessment map[string]any
	if err := json.Unmarshal([]byte(s.inboxReviewData(ctx, detail)), &assessment); err != nil {
		t.Fatal(err)
	}
	delivered, ok := assessment["delivered_files"].([]any)
	if !ok || len(delivered) != 1 {
		t.Fatal("coordinator did not receive real delivery metadata", assessment)
	}
	texts, ok := assessment["delivered_text"].([]any)
	if !ok || len(texts) != 1 {
		t.Fatal("coordinator did not receive delivered report text")
	}
	text := texts[0].(map[string]any)
	excerpt, _ := text["untrusted_excerpt"].(string)
	if !strings.Contains(excerpt, "Offline output") || !strings.Contains(excerpt, "FINAL_VERIFICATION_RESULT") || len(excerpt) > 6000 || text["excerpt_scope"] != "head_and_tail" {
		t.Fatal("actual attachment review omitted final verification or hid truncation")
	}
	messages, err := s.getFormattedMessagesResult(ctx, q.RoomID, "")
	if err != nil {
		t.Fatal(err)
	}
	var attachment conversationAttachment
	for _, m := range messages {
		if len(m.Attachments) > 0 {
			attachment = m.Attachments[0]
			if m.WorkUpdate == nil || m.WorkUpdate.Kind != "files" || m.WorkUpdate.TaskID != saved.ID {
				t.Fatal("native file delivery lost task projection")
			}
		}
	}
	if attachment.Name != filename {
		raw, _ := json.Marshal(messages)
		t.Fatal("download not surfaced in Inbox", string(raw))
	}
	response := call("GET", "/api/v1/tasks/"+saved.ID, "")
	var projected task.TaskDetailResponse
	if err := json.Unmarshal(response.Body.Bytes(), &projected); err != nil || response.Code != 200 {
		t.Fatal("task detail unavailable", response.Code, err)
	}
	foundFiles := false
	for _, event := range projected.Activities {
		if event.ActivityType == "work_files" && event.AttemptID != nil && *event.AttemptID == a.ID && strings.Contains(event.Message, attachment.ID) {
			foundFiles = true
		}
	}
	if !foundFiles {
		t.Fatal("task timeline cannot open the retained delivery")
	}
	os.WriteFile(sourceFile, []byte("REPLACED_SOURCE"), 0600)
	if session {
		entries, _ := os.ReadDir(workspace)
		if len(entries) != 0 {
			t.Fatal("session delivery modified repository")
		}
		cleanup() // Delivered Inbox copies must outlive the private staging directory.
	}
	download := call("GET", attachment.URL, "")
	if download.Code != 200 || download.Body.String() != original || !strings.HasPrefix(download.Header().Get("Content-Disposition"), "attachment;") {
		t.Fatal("immutable HTML download failed", download.Code, download.Header())
	}
	preview := call("GET", attachment.URL+"?preview=1", "")
	if preview.Code != 200 || preview.Body.String() != original || !strings.HasPrefix(preview.Header().Get("Content-Disposition"), "inline;") {
		t.Fatal("inline preview failed", preview.Code, preview.Header())
	}
	unauthorized := httptest.NewRecorder()
	s.Router().ServeHTTP(unauthorized, httptest.NewRequest("GET", attachment.URL, nil))
	if unauthorized.Code != 401 {
		t.Fatal("download did not require sign-in", unauthorized.Code)
	}
	if _, _, err := loadAttachment("another-owner", attachment.ID); err == nil {
		t.Fatal("file crossed owner boundary")
	}
}

func TestSessionArtifactsKeepRootIsolationAndSharedDeliveryLimits(t *testing.T) {
	workspace, stage := t.TempDir(), t.TempDir()
	root, err := os.OpenRoot(workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	evidence, err := os.OpenRoot(stage)
	if err != nil {
		t.Fatal(err)
	}
	defer evidence.Close()
	outside := filepath.Join(t.TempDir(), "private.txt")
	os.WriteFile(outside, []byte("PRIVATE"), 0600)
	os.Symlink(outside, filepath.Join(stage, "escape.txt"))
	for _, path := range []string{"session:../private.txt", "session:" + outside, "session:.env", "session:nested/.git/config", "session:private.pem", "session:escape.txt"} {
		if _, err := readTaskArtifacts(root, "task", "attempt", []string{path}, evidence); err == nil {
			t.Fatal("invalid session file accepted", path)
		}
	}
	os.WriteFile(filepath.Join(stage, "mobile.png"), []byte("snapshot fixture"), 0600)
	if _, err := readTaskArtifacts(root, "task", "attempt", []string{"session:mobile.png"}); err == nil {
		t.Fatal("session path worked without bound session root")
	}
	if _, err := readTaskArtifacts(root, "task", "attempt", []string{"session:mobile.png", "session:mobile.png"}, evidence); err == nil {
		t.Fatal("duplicate session artifact accepted")
	}
	for _, file := range []string{filepath.Join(workspace, "one.txt"), filepath.Join(stage, "two.txt"), filepath.Join(stage, "three.txt")} {
		os.WriteFile(file, nil, 0600)
		os.Truncate(file, 9<<20)
	}
	if _, err := readTaskArtifacts(root, "task", "attempt", []string{"one.txt", "session:two.txt", "session:three.txt"}, evidence); err == nil {
		t.Fatal("mixed roots bypassed aggregate25MB bound")
	}
	if _, err := readTaskArtifacts(root, "task", "attempt", []string{"one.txt", "session:two.txt"}, evidence); err != nil {
		t.Fatal("valid mixed delivery rejected", err)
	}
}
