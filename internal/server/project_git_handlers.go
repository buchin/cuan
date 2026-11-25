package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
)

// ProjectCommitItem represents a single Git commit entry in project activity log.
type ProjectCommitItem struct {
	SHA          string `json:"sha"`
	ShortSHA     string `json:"short_sha"`
	AuthorName   string `json:"author_name"`
	AuthorEmail  string `json:"author_email"`
	Date         string `json:"date"`
	RelativeDate string `json:"relative_date"`
	Message      string `json:"message"`
	TaskID       string `json:"task_id,omitempty"`
	TaskRef      string `json:"task_ref,omitempty"`
}

// ProjectGitLogResponse is the JSON response for GET /api/v1/projects/{id}/git-log.
type ProjectGitLogResponse struct {
	ProjectID string              `json:"project_id"`
	Branch    string              `json:"branch"`
	Commits   []ProjectCommitItem `json:"commits"`
	Total     int                 `json:"total"`
}

// CommitFileStat represents file modification stats in a single commit.
type CommitFileStat struct {
	Path       string `json:"path"`
	Insertions int    `json:"insertions"`
	Deletions  int    `json:"deletions"`
}

// ProjectCommitDetailResponse is the JSON response for GET /api/v1/projects/{id}/commits/{sha}.
type ProjectCommitDetailResponse struct {
	SHA          string           `json:"sha"`
	ShortSHA     string           `json:"short_sha"`
	AuthorName   string           `json:"author_name"`
	AuthorEmail  string           `json:"author_email"`
	Date         string           `json:"date"`
	RelativeDate string           `json:"relative_date"`
	Message      string           `json:"message"`
	Body         string           `json:"body"`
	StatsSummary string           `json:"stats_summary"`
	Files        []CommitFileStat `json:"files"`
	PatchPreview string           `json:"patch_preview,omitempty"`
	TaskID       string           `json:"task_id,omitempty"`
	TaskRef      string           `json:"task_ref,omitempty"`
}

func parseCommitTask(msg string) (string, string) {
	msg = strings.TrimSpace(msg)
	if strings.HasPrefix(msg, "Apply Task ") {
		rest := strings.TrimPrefix(msg, "Apply Task ")
		colon := strings.Index(rest, ":")
		if colon > 0 {
			taskID := strings.TrimSpace(rest[:colon])
			if strings.HasPrefix(taskID, "msg_") || strings.HasPrefix(taskID, "task_") {
				raw := strings.ReplaceAll(strings.ReplaceAll(taskID, "msg_", ""), "task_", "")
				raw = strings.ReplaceAll(raw, "-", "")
				if len(raw) > 6 {
					raw = raw[:6]
				}
				ref := "#" + strings.ToUpper(raw)
				return taskID, ref
			}
		}
	}
	return "", ""
}

func (s *Server) resolveProjectRepoDir(ctx context.Context, id string) (*project.ProjectResponse, string, error) {
	if s.projectSvc == nil {
		return nil, "", errors.New("project service unavailable")
	}
	proj, err := s.projectSvc.Get(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if proj == nil {
		return nil, "", project.ErrProjectNotFound
	}
	workDir := proj.LocalPath
	if wd, e := s.projectSvc.ProjectTerminalWorkspace(ctx, proj.ID, proj); e == nil && wd != "" {
		if fi, err := os.Stat(wd); err == nil && fi.IsDir() {
			workDir = wd
		}
	}
	return proj, workDir, nil
}

// GET /api/v1/projects/{id}/git-log (and /api/projects/{id}/git-log)
func (s *Server) handleGetProjectGitLog(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	proj, workDir, err := s.resolveProjectRepoDir(r.Context(), id)
	if err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			respondJSONError(w, http.StatusNotFound, "Project not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "Failed to load project: "+err.Error())
		return
	}

	defaultBranch := proj.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	// If workspace directory is missing or not a directory, return empty commits gracefully
	if workDir == "" {
		respondJSON(w, http.StatusOK, ProjectGitLogResponse{
			ProjectID: id,
			Branch:    defaultBranch,
			Commits:   []ProjectCommitItem{},
			Total:     0,
		})
		return
	}
	if fi, err := os.Stat(workDir); err != nil || !fi.IsDir() {
		respondJSON(w, http.StatusOK, ProjectGitLogResponse{
			ProjectID: id,
			Branch:    defaultBranch,
			Commits:   []ProjectCommitItem{},
			Total:     0,
		})
		return
	}

	limit := 50
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		if l, err := strconv.Atoi(rawLimit); err == nil && l > 0 {
			limit = l
			if limit > 200 {
				limit = 200
			}
		}
	}
	branch := strings.TrimSpace(r.URL.Query().Get("branch"))

	args := []string{"-C", workDir, "log", fmt.Sprintf("-n%d", limit), "--format=%H|%h|%an|%ae|%cI|%ar|%s"}
	if branch != "" {
		args = append(args, branch)
	}

	cmdCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(out)
		// Empty repo with no commits yet
		if strings.Contains(outStr, "does not have any commits yet") ||
			strings.Contains(outStr, "fatal: your current branch") ||
			strings.Contains(outStr, "unknown revision") {
			respondJSON(w, http.StatusOK, ProjectGitLogResponse{
				ProjectID: id,
				Branch:    defaultBranch,
				Commits:   []ProjectCommitItem{},
				Total:     0,
			})
			return
		}
		respondJSON(w, http.StatusOK, ProjectGitLogResponse{
			ProjectID: id,
			Branch:    defaultBranch,
			Commits:   []ProjectCommitItem{},
			Total:     0,
		})
		return
	}

	curBranch := branch
	if curBranch == "" {
		curOut, _ := exec.CommandContext(cmdCtx, "git", "-C", workDir, "branch", "--show-current").Output()
		curBranch = strings.TrimSpace(string(curOut))
		if curBranch == "" {
			curBranch = defaultBranch
		}
	}

	commits := make([]ProjectCommitItem, 0)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 7)
		if len(parts) >= 7 {
			msg := parts[6]
			taskID, taskRef := parseCommitTask(msg)
			commits = append(commits, ProjectCommitItem{
				SHA:          parts[0],
				ShortSHA:     parts[1],
				AuthorName:   parts[2],
				AuthorEmail:  parts[3],
				Date:         parts[4],
				RelativeDate: parts[5],
				Message:      msg,
				TaskID:       taskID,
				TaskRef:      taskRef,
			})
		}
	}

	respondJSON(w, http.StatusOK, ProjectGitLogResponse{
		ProjectID: id,
		Branch:    curBranch,
		Commits:   commits,
		Total:     len(commits),
	})
}

// GET /api/v1/projects/{id}/commits/{sha} (and /api/projects/{id}/commits/{sha})
func (s *Server) handleGetProjectCommitDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sha := strings.TrimSpace(chi.URLParam(r, "sha"))
	if id == "" || sha == "" {
		respondJSONError(w, http.StatusBadRequest, "project id and commit sha are required")
		return
	}

	// Validate SHA (prevent flag injection or path traversal)
	for _, ch := range sha {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			respondJSONError(w, http.StatusBadRequest, "invalid commit sha format")
			return
		}
	}
	if len(sha) < 4 || len(sha) > 40 {
		respondJSONError(w, http.StatusBadRequest, "invalid commit sha length")
		return
	}

	_, workDir, err := s.resolveProjectRepoDir(r.Context(), id)
	if err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			respondJSONError(w, http.StatusNotFound, "Project not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "Failed to load project: "+err.Error())
		return
	}

	if workDir == "" {
		respondJSONError(w, http.StatusNotFound, "Project workspace unavailable")
		return
	}
	if fi, err := os.Stat(workDir); err != nil || !fi.IsDir() {
		respondJSONError(w, http.StatusNotFound, "Project workspace directory not found")
		return
	}

	cmdCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// 1. Get commit metadata & subject
	headerCmd := exec.CommandContext(cmdCtx, "git", "-C", workDir, "show", "-s", "--format=%H|%h|%an|%ae|%cI|%ar|%s", sha)
	headerOut, err := headerCmd.CombinedOutput()
	if err != nil {
		respondJSONError(w, http.StatusNotFound, "Commit not found in project history")
		return
	}
	headerParts := strings.SplitN(strings.TrimSpace(string(headerOut)), "|", 7)
	if len(headerParts) < 7 {
		respondJSONError(w, http.StatusInternalServerError, "Malformed commit header from git")
		return
	}

	fullSHA := headerParts[0]
	shortSHA := headerParts[1]
	authorName := headerParts[2]
	authorEmail := headerParts[3]
	date := headerParts[4]
	relativeDate := headerParts[5]
	msg := headerParts[6]
	taskID, taskRef := parseCommitTask(msg)

	// 2. Get commit body
	bodyCmd := exec.CommandContext(cmdCtx, "git", "-C", workDir, "show", "-s", "--format=%b", sha)
	bodyOut, _ := bodyCmd.CombinedOutput()
	body := strings.TrimSpace(string(bodyOut))

	// 3. Get stat summary (e.g. "3 files changed, 30 insertions(+), 16 deletions(-)")
	statCmd := exec.CommandContext(cmdCtx, "git", "-C", workDir, "show", "--format=", "--shortstat", sha)
	statOut, _ := statCmd.CombinedOutput()
	statsSummary := strings.TrimSpace(string(statOut))

	// 4. Get numstat for individual files
	numstatCmd := exec.CommandContext(cmdCtx, "git", "-C", workDir, "show", "--format=", "--numstat", sha)
	numstatOut, _ := numstatCmd.CombinedOutput()
	files := make([]CommitFileStat, 0)
	numScanner := bufio.NewScanner(strings.NewReader(string(numstatOut)))
	for numScanner.Scan() {
		line := strings.TrimSpace(numScanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) >= 3 {
			ins, _ := strconv.Atoi(parts[0])
			del, _ := strconv.Atoi(parts[1])
			files = append(files, CommitFileStat{
				Path:       parts[2],
				Insertions: ins,
				Deletions:  del,
			})
		}
	}

	// 5. Get truncated patch preview (up to 300 lines)
	patchCmd := exec.CommandContext(cmdCtx, "git", "-C", workDir, "show", "--format=", sha)
	patchOut, _ := patchCmd.CombinedOutput()
	patchLines := strings.Split(string(patchOut), "\n")
	if len(patchLines) > 300 {
		patchLines = append(patchLines[:300], fmt.Sprintf("\n... [%d lines truncated]", len(patchLines)-300))
	}
	patchPreview := strings.Join(patchLines, "\n")

	respondJSON(w, http.StatusOK, ProjectCommitDetailResponse{
		SHA:          fullSHA,
		ShortSHA:     shortSHA,
		AuthorName:   authorName,
		AuthorEmail:  authorEmail,
		Date:         date,
		RelativeDate: relativeDate,
		Message:      msg,
		Body:         body,
		StatsSummary: statsSummary,
		Files:        files,
		PatchPreview: patchPreview,
		TaskID:       taskID,
		TaskRef:      taskRef,
	})
}

// POST /api/v1/projects/{id}/repo/push
func (s *Server) handlePushProjectRepo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}
	if s.projectSvc == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project service unavailable")
		return
	}

	var body struct {
		Branch string `json:"branch"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 16*1024)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	branch, commit, err := s.projectSvc.PushProjectBranch(r.Context(), id, body.Branch)
	if err != nil {
		switch {
		case errors.Is(err, project.ErrProjectNotFound):
			respondJSONError(w, http.StatusNotFound, "Project not found")
		case errors.Is(err, git.ErrDefaultBranchProtected), errors.Is(err, project.ErrGitHubConnectionRequired):
			respondJSONError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, git.ErrRemoteBranchDiverged):
			respondJSONError(w, http.StatusConflict, err.Error())
		default:
			respondJSONError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"pushed":     true,
		"project_id": id,
		"branch":     branch,
		"commit":     commit,
	})
}
