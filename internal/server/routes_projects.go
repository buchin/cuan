package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
)

// registerProjectRoutes mounts all project CRUD and configuration endpoints on the protected router.
func (s *Server) registerProjectRoutes(pr chi.Router) {
	// Projects endpoints (T3)
	pr.Get("/projects", s.handleListProjects)
	pr.Post("/projects", s.handleConnectProject)
	pr.Get("/projects/{id}", s.handleGetProject)
	pr.Patch("/projects/{id}/settings", s.handleProjectSettings)
	pr.Post("/projects/{id}/sync", s.handleSyncProject)
	pr.Delete("/projects/{id}", s.handleDeleteProject)
	pr.Get("/projects/{id}/branches", s.handleGetProjectBranches)
	pr.Patch("/projects/{id}/modules", s.handleUpdateProjectModules)
	pr.Put("/projects/{id}/modules", s.handleUpdateProjectModules)
	pr.Patch("/projects/{id}/archive", s.handleArchiveProject)
	pr.Post("/projects/{id}/archive", s.handleArchiveProject)
	pr.Patch("/projects/{id}/unarchive", s.handleUnarchiveProject)
	pr.Post("/projects/{id}/unarchive", s.handleUnarchiveProject)
	pr.Patch("/projects/{id}/restore", s.handleUnarchiveProject)
	pr.Post("/projects/{id}/restore", s.handleUnarchiveProject)
}

func (s *Server) isProjectActive(ctx context.Context, projectID string, excludeTaskID string) bool {
	if projectID == "" {
		return false
	}
	if s.terminalManager != nil {
		if active, err := s.terminalManager.ProjectExecutionActive(projectID); err == nil && active {
			return true
		}
	}
	if s.taskSvc != nil {
		if active, err := s.taskSvc.IsProjectActive(ctx, projectID, excludeTaskID); err == nil && active {
			return true
		}
	}
	return false
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.projectSvc.List(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list projects: %v", err))
		return
	}
	if projects == nil {
		projects = []*project.ProjectResponse{}
	}

	archivedParam := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("archived")))
	includeArchivedParam := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("include_archived")))
	statusParam := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))

	var filtered []*project.ProjectResponse
	for _, p := range projects {
		if archivedParam == "true" || statusParam == "archived" {
			if p.IsArchived {
				filtered = append(filtered, p)
			}
		} else if archivedParam == "all" || includeArchivedParam == "true" || statusParam == "all" {
			filtered = append(filtered, p)
		} else {
			// default: only active (is_archived == false)
			if !p.IsArchived {
				filtered = append(filtered, p)
			}
		}
	}
	if filtered == nil {
		filtered = []*project.ProjectResponse{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"projects": filtered,
		"total":    len(filtered),
	})
}

func (s *Server) handleArchiveProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	res, err := s.projectSvc.Archive(r.Context(), id)
	if err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			respondJSONError(w, http.StatusNotFound, "Project not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to archive project: %v", err))
		return
	}

	if s.projectRepo != nil {
		_ = s.projectRepo.ArchiveProject(r.Context(), id)
	}

	respondJSON(w, http.StatusOK, res)
}

func (s *Server) handleUnarchiveProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	res, err := s.projectSvc.Unarchive(r.Context(), id)
	if err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			respondJSONError(w, http.StatusNotFound, "Project not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to unarchive project: %v", err))
		return
	}

	if s.projectRepo != nil {
		_ = s.projectRepo.UnarchiveProject(r.Context(), id)
	}

	respondJSON(w, http.StatusOK, res)
}

func (s *Server) handleConnectProject(w http.ResponseWriter, r *http.Request) {
	var req project.ConnectProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.RepoURL) == "" {
		name := strings.TrimSpace(req.Name)
		if name == "" {
			respondJSONError(w, http.StatusBadRequest, "project name is required")
			return
		}
		id := "proj_" + strings.ReplaceAll(cuanDB.GenerateUUID(), "-", "")
		res, err := s.projectSvc.CreateManagedWorkspace(r.Context(), id, name)
		if err != nil {
			if errors.Is(err, project.ErrProjectAlreadyExists) {
				respondJSONError(w, http.StatusConflict, "A project with this name already exists.")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "Could not create project workspace: "+err.Error())
			return
		}
		respondJSON(w, http.StatusCreated, res)
		return
	}

	res, err := s.projectSvc.Connect(r.Context(), req)
	if err != nil {
		if errors.Is(err, git.ErrInvalidRepoURL) {
			respondJSONError(w, http.StatusBadRequest, "Invalid GitHub repository URL. Use 'https://github.com/owner/repo' or 'owner/repo'.")
			return
		}
		if errors.Is(err, git.ErrRepoNotFound) {
			respondJSONError(w, http.StatusNotFound, "Repository not found. Check its name and make sure the GitHub account connected in Settings > Integrations has access.")
			return
		}
		if errors.Is(err, git.ErrAccessDenied) {
			respondJSONError(w, http.StatusForbidden, "GitHub could not grant access to this repository. Reconnect GitHub in Settings > Integrations with an account that has access, then try again.")
			return
		}
		if errors.Is(err, project.ErrProjectAlreadyExists) {
			respondJSONError(w, http.StatusConflict, "This GitHub repository is already connected to a Cuan project.")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Could not connect repository: %v", err))
		return
	}

	respondJSON(w, http.StatusCreated, res)
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := s.projectSvc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			respondJSONError(w, http.StatusNotFound, "Project not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get project: %v", err))
		return
	}
	respondJSON(w, http.StatusOK, p)
}

func (s *Server) handleSyncProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	res, err := s.projectSvc.Sync(r.Context(), id)
	if err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			respondJSONError(w, http.StatusNotFound, "Project not found")
			return
		}
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":   fmt.Sprintf("Sync failed: %v", err),
			"project": res,
		})
		return
	}
	respondJSON(w, http.StatusOK, res)
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := s.projectSvc.Delete(r.Context(), id)
	if err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			respondJSONError(w, http.StatusNotFound, "Project not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete project: %v", err))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Project removed from Cuan. The remote GitHub repository is preserved.",
		"id":      id,
	})
}

func (s *Server) handleGetProjectBranches(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	branches, defaultBranch, err := s.projectSvc.GetBranches(r.Context(), id)
	if err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			respondJSONError(w, http.StatusNotFound, "Project not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get branches: %v", err))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"branches":       branches,
		"default_branch": defaultBranch,
	})
}

func (s *Server) handleUpdateProjectModules(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	var payload struct {
		EnabledModules []string `json:"enabled_modules"`
		Modules        []string `json:"modules"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	mods := payload.EnabledModules
	if len(mods) == 0 && len(payload.Modules) > 0 {
		mods = payload.Modules
	}

	res, err := s.projectSvc.UpdateModules(r.Context(), id, mods)
	if err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			respondJSONError(w, http.StatusNotFound, "Project not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update project modules: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, res)
}
