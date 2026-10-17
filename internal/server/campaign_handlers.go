package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/campaign"
	"github.com/buchin/cuan/internal/db"
)

// handleCreateBatchCampaign creates a new batch campaign.
// Route: POST /api/campaigns
// Aliases: POST /api/v1/campaigns
func (s *Server) handleCreateBatchCampaign(w http.ResponseWriter, r *http.Request) {
	if s.batchCampaignRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "batch campaign repository is unavailable")
		return
	}

	var input db.CreateBatchCampaignInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request json: "+err.Error())
		return
	}

	if err := input.Validate(); err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	entity, err := input.ToEntity()
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.batchCampaignRepo.Create(r.Context(), entity); err != nil {
		if errors.Is(err, db.ErrDuplicateCampaign) {
			respondJSONError(w, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, db.ErrEmptyCampaignTitle) || errors.Is(err, db.ErrInvalidCampaignStatus) || errors.Is(err, db.ErrInvalidUUID) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to create batch campaign: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"status":   "success",
		"campaign": entity.ToDTO(),
	})
}

// handleListBatchCampaigns lists campaigns matching query filters (?status=, ?search=, ?limit=, ?offset=).
// Route: GET /api/campaigns
// Aliases: GET /api/v1/campaigns
func (s *Server) handleListBatchCampaigns(w http.ResponseWriter, r *http.Request) {
	if s.batchCampaignRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "batch campaign repository is unavailable")
		return
	}

	q := r.URL.Query()
	filter := db.BatchCampaignFilter{
		Status: strings.TrimSpace(q.Get("status")),
		Search: strings.TrimSpace(q.Get("search")),
	}

	if limitStr := strings.TrimSpace(q.Get("limit")); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}
	if offsetStr := strings.TrimSpace(q.Get("offset")); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	campaigns, err := s.batchCampaignRepo.List(r.Context(), filter)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to list batch campaigns: "+err.Error())
		return
	}

	dtos := make([]db.BatchCampaignDTO, len(campaigns))
	for i, c := range campaigns {
		dtos[i] = c.ToDTO()
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "success",
		"campaigns": dtos,
		"count":     len(dtos),
	})
}

// handleGetBatchCampaign retrieves a single batch campaign by ID and includes its child subtasks if available.
// Route: GET /api/campaigns/{id}
// Aliases: GET /api/v1/campaigns/{id}
func (s *Server) handleGetBatchCampaign(w http.ResponseWriter, r *http.Request) {
	if s.batchCampaignRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "batch campaign repository is unavailable")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "campaign id is required")
		return
	}
	if !db.IsValidUUID(id) {
		respondJSONError(w, http.StatusBadRequest, "invalid campaign uuid format")
		return
	}

	c, err := s.batchCampaignRepo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrBatchCampaignNotFound) {
			respondJSONError(w, http.StatusNotFound, "batch campaign not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to retrieve batch campaign: "+err.Error())
		return
	}

	resp := map[string]interface{}{
		"status":   "success",
		"campaign": c.ToDTO(),
	}

	// Query child tasks if projectStudioRepo is available
	if s.projectStudioRepo != nil {
		tasks, err := s.projectStudioRepo.ListTasksByBatchID(r.Context(), id)
		if err == nil {
			taskDTOs := make([]db.ProjectTaskDTO, len(tasks))
			for i, t := range tasks {
				taskDTOs[i] = t.ToDTO()
			}
			resp["tasks"] = taskDTOs
			resp["task_count"] = len(taskDTOs)
		}
	}

	respondJSON(w, http.StatusOK, resp)
}

// handleExecuteBatchCampaign executes a batch campaign via the isolated multi-project worker pool.
// Route: POST /api/campaigns/{id}/execute
// Aliases: POST /api/v1/campaigns/{id}/execute
func (s *Server) handleExecuteBatchCampaign(w http.ResponseWriter, r *http.Request) {
	if s.batchCampaignRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "batch campaign repository is unavailable")
		return
	}
	if s.campaignRunner == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "campaign runner engine is unavailable")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "campaign id is required")
		return
	}
	if !db.IsValidUUID(id) {
		respondJSONError(w, http.StatusBadRequest, "invalid campaign uuid format")
		return
	}

	// Verify campaign existence
	c, err := s.batchCampaignRepo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrBatchCampaignNotFound) {
			respondJSONError(w, http.StatusNotFound, "batch campaign not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to verify campaign: "+err.Error())
		return
	}

	// Execute campaign through runner engine
	runResult, err := s.campaignRunner.ExecuteCampaign(r.Context(), id)
	if err != nil {
		if errors.Is(err, campaign.ErrNoTargetProjectsFound) {
			respondJSONError(w, http.StatusBadRequest, "cannot execute campaign: no matching target projects found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "campaign execution failed: "+err.Error())
		return
	}

	// Re-fetch updated campaign entity
	updated, err := s.batchCampaignRepo.GetByID(r.Context(), id)
	if err != nil {
		updated = c
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":           "success",
		"campaign_id":      id,
		"campaign":         updated.ToDTO(),
		"total_tasks":      runResult.TotalTasks,
		"completed_tasks":  runResult.CompletedTasks,
		"failed_tasks":     runResult.FailedTasks,
		"peak_concurrency": runResult.PeakConcurrency,
		"duration_ms":      runResult.DurationMs,
		"results":          runResult.Results,
	})
}
