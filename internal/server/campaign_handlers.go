package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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

	// Include rollup metrics if aggregator is available
	if s.campaignAggregator != nil {
		rollup := s.campaignAggregator.GetRollup(id)
		if rollup == nil && s.projectStudioRepo != nil {
			rollup, _ = s.campaignAggregator.RefreshFromRepository(r.Context(), id)
		}
		if rollup != nil {
			resp["rollup"] = rollup
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

// handleGetCampaignRollup retrieves aggregated rollup metrics for a batch campaign.
// Route: GET /api/campaigns/{id}/rollup
// Aliases: GET /api/v1/campaigns/{id}/rollup
func (s *Server) handleGetCampaignRollup(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "campaign id is required")
		return
	}
	if !db.IsValidUUID(id) {
		respondJSONError(w, http.StatusBadRequest, "invalid campaign uuid format")
		return
	}

	if s.batchCampaignRepo != nil {
		_, err := s.batchCampaignRepo.GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, db.ErrBatchCampaignNotFound) {
				respondJSONError(w, http.StatusNotFound, "batch campaign not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to get batch campaign: "+err.Error())
			return
		}
	}

	if s.campaignAggregator == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "campaign aggregator engine is unavailable")
		return
	}

	rollup := s.campaignAggregator.GetRollup(id)
	if rollup == nil {
		var err error
		rollup, err = s.campaignAggregator.RefreshFromRepository(r.Context(), id)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to aggregate campaign rollup: "+err.Error())
			return
		}
	}

	if rollup == nil {
		rollup = &campaign.RollupMetrics{
			CampaignID: id,
			Status:     "draft",
			UpdatedAt:  time.Now().UTC(),
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "success",
		"campaign_id": id,
		"rollup":      rollup,
	})
}

// handleCampaignEventsSSE streams real-time campaign progress, task status changes,
// and completion events via Server-Sent Events (SSE) with heartbeat and graceful cleanup.
// Route: GET /api/campaigns/{id}/events
// Aliases: GET /api/v1/campaigns/{id}/events
func (s *Server) handleCampaignEventsSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		respondJSONError(w, http.StatusInternalServerError, "server-sent events streaming unsupported")
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

	// Verify campaign existence if repository is configured
	if s.batchCampaignRepo != nil {
		_, err := s.batchCampaignRepo.GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, db.ErrBatchCampaignNotFound) {
				respondJSONError(w, http.StatusNotFound, "batch campaign not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to retrieve batch campaign: "+err.Error())
			return
		}
	}

	// Ensure campaignAggregator is initialized
	if s.campaignAggregator == nil {
		s.campaignAggregator = campaign.NewRollupAggregator(
			campaign.WithAggregatorStudioRepo(s.projectStudioRepo),
			campaign.WithAggregatorBatchRepo(s.batchCampaignRepo),
		)
		if s.campaignRunner != nil {
			s.campaignRunner.AddListener(s.campaignAggregator)
		}
	}

	// Set standard SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Subscribe client channel before sending handshake
	clientChan := s.campaignAggregator.Subscribe(id)
	defer s.campaignAggregator.Unsubscribe(id, clientChan)

	// Emit initial connection handshake event
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\",\"campaign_id\":\"%s\"}\n\n", id)
	flusher.Flush()

	// Emit initial campaign.progress event with current rollup state if available
	initialRollup := s.campaignAggregator.GetRollup(id)
	if initialRollup == nil && s.projectStudioRepo != nil {
		initialRollup, _ = s.campaignAggregator.RefreshFromRepository(r.Context(), id)
	}
	if initialRollup != nil {
		dataBytes, err := json.Marshal(campaign.CampaignProgressData{
			CampaignID: id,
			Rollup:     initialRollup,
			Timestamp:  time.Now().UTC(),
		})
		if err == nil {
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", campaign.EventCampaignProgress, string(dataBytes))
			flusher.Flush()
		}
	}

	// Heartbeat ticker (15s default interval emitting ": ping\n\n")
	ticker := time.NewTicker(s.campaignAggregator.HeartbeatInterval())
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			// Client disconnected gracefully; deferred unsubscribe will clean up channel and map
			return

		case <-ticker.C:
			// Send heartbeat comment as requested (: ping\n\n)
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()

		case ev, ok := <-clientChan:
			if !ok {
				return
			}
			dataBytes, err := json.Marshal(ev.Data)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, string(dataBytes))
			flusher.Flush()
		}
	}
}
