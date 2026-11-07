package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/publication"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
	"github.com/go-chi/chi/v5"
)

// Preparation transport accepts identity only. The target/path/environment are
// derived from canonical native evidence and frozen owner project configuration.
func (s *Server) handleRunnerNativeReleasePreparation(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TaskID   string `json:"task_id"`
		RunnerID string `json:"runner_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF || input.TaskID == "" || input.RunnerID == "" {
		respondJSONError(w, 400, "Invalid native preparation identity")
		return
	}
	attemptID := chi.URLParam(r, "id")
	d, _, err := s.authorizedNativeReleaseAttempt(r.Context(), input.TaskID, attemptID, input.RunnerID)
	if err != nil {
		respondJSONError(w, 409, "Native release authorization is no longer active")
		return
	}
	if s.projectSvc == nil {
		respondJSONError(w, 503, "Saved release project unavailable")
		return
	}
	p, err := s.projectSvc.TaskContext(r.Context(), d.ProjectID, d.Instructions)
	if err != nil || p == nil || p.IsArchived {
		respondJSONError(w, 409, "Saved native project context unavailable")
		return
	}
	selectedEnvironment := project.ParseEnvironment(project.SettingsFromMetadata(p.Metadata).Environment)
	sshConfig, configErr := publication.FromEnvironment(selectedEnvironment)
	if configErr != nil {
		respondJSONError(w, 503, "Saved release verification and SSH host trust must be configured before native publication")
		return
	}
	prepared, err := s.ensureNativeReleasePreparation(r.Context(), d.ID, attemptID, input.RunnerID)
	if err != nil {
		respondJSONError(w, 409, "Native release source publication or preparation could not be verified")
		return
	}
	saved, err := task.NativeReleasePreparationForAttempt(r.Context(), s.taskSvc.Store(), &d.Task, attemptID)
	if err != nil || saved == nil {
		respondJSONError(w, 409, "Canonical native preparation unavailable")
		return
	}
	environment := selectedEnvironment
	// Runtime home/cache locations are private to this exact release checkout,
	// regardless of ambient host HOME or an old project execution path.
	root, err := os.OpenRoot(prepared.Path)
	if err != nil {
		respondJSONError(w, 503, "Private native runtime unavailable")
		return
	}
	defer root.Close()
	paths := []string{".git/cuan-native-runtime", ".git/cuan-native-runtime/home", ".git/cuan-native-runtime/cache", ".git/cuan-native-runtime/config", ".git/cuan-native-runtime/data", ".git/cuan-native-runtime/tmp"}
	for _, path := range paths {
		if err := root.Mkdir(path, 0700); err != nil && !os.IsExist(err) {
			respondJSONError(w, 503, "Private native runtime unavailable")
			return
		}
		info, err := root.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
			respondJSONError(w, 503, "Private native runtime changed")
			return
		}
	}
	base := filepath.Join(prepared.Path, ".git", "cuan-native-runtime")
	environment["HOME"] = filepath.Join(base, "home")
	environment["XDG_CACHE_HOME"] = filepath.Join(base, "cache")
	environment["XDG_CONFIG_HOME"] = filepath.Join(base, "config")
	environment["XDG_DATA_HOME"] = filepath.Join(base, "data")
	environment["TMPDIR"] = filepath.Join(base, "tmp")
	environment["GOCACHE"] = filepath.Join(base, "cache", "go")
	environment["GOPATH"] = filepath.Join(base, "data", "go")
	environment["NPM_CONFIG_CACHE"] = filepath.Join(base, "cache", "npm")
	if environment["PATH"] == "" {
		environment["PATH"] = "/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin"
	}
	sshBin, sshErr := prepareNativeReleaseSSH(r.Context(), root, prepared.Path, sshConfig)
	if sshErr != nil {
		respondJSONError(w, 503, "Private SSH runtime and saved host trust could not be prepared")
		return
	}
	environment["PATH"] = sshBin + string(os.PathListSeparator) + environment["PATH"]
	environment["CUAN_DEPLOY_BRANCH"] = prepared.Branch
	environment["CUAN_DEPLOY_EXCLUDE_TASK"] = d.ID
	environment["CUAN_DEPLOY_EXCLUDE_ATTEMPT"] = attemptID
	keys := make([]string, 0, len(environment))
	for key := range environment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	selected := make([]string, 0, len(keys))
	for _, key := range keys {
		selected = append(selected, key+"="+environment[key])
	}
	// Revalidate authorization after preparing runtime directories, before
	// returning any owner execution environment to the authenticated runner.
	fresh, _, err := s.authorizedNativeReleaseAttempt(r.Context(), d.ID, attemptID, input.RunnerID)
	if err != nil || fresh.ProjectID != d.ProjectID || fresh.Instructions != d.Instructions {
		respondJSONError(w, 409, "Native release scope changed during preparation")
		return
	}
	currentProject, projectErr := s.projectSvc.TaskContext(r.Context(), fresh.ProjectID, fresh.Instructions)
	currentPreparation, preparationErr := task.NativeReleasePreparationForAttempt(r.Context(), s.taskSvc.Store(), &fresh.Task, attemptID)
	if projectErr != nil || currentProject == nil || currentProject.IsArchived || currentProject.RepoURL != p.RepoURL || currentProject.LocalPath != p.LocalPath || currentProject.DefaultBranch != p.DefaultBranch || string(currentProject.Metadata) != string(p.Metadata) || preparationErr != nil || currentPreparation == nil || *currentPreparation != *saved {
		respondJSONError(w, 409, "Native release project or preparation changed")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, runner.NativeReleaseExecution{Workspace: prepared, Environment: selected})
}
