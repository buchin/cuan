package server

import (
	"context"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/db"
)

// GroundingTaste represents an active taste preference metadata point injected into LLM chat context.
type GroundingTaste struct {
	ID       string `json:"id"`
	Category string `json:"category"` // "brand" or "technical"
	Topic    string `json:"topic,omitempty"`
	Summary  string `json:"summary,omitempty"`
	Preview  string `json:"preview,omitempty"`
	Content  string `json:"content,omitempty"`
}

// SessionGroundingTastesResponse represents the active grounding taste inspection response for a chat session.
type SessionGroundingTastesResponse struct {
	SessionID       string           `json:"session_id"`
	RoomID          string           `json:"room_id,omitempty"`
	BrandTastes     []GroundingTaste `json:"brand_tastes"`
	TechnicalTastes []GroundingTaste `json:"technical_tastes"`
	AllTastes       []GroundingTaste `json:"all_tastes"`
	Total           int              `json:"total"`
}

type tasteGroundingCaptureKey struct{}

// WithTasteGroundingCapture returns a new context configured to capture taste grounding metadata.
func WithTasteGroundingCapture(ctx context.Context, sink *[]GroundingTaste) context.Context {
	return context.WithValue(ctx, tasteGroundingCaptureKey{}, sink)
}

// itemToGroundingTaste converts a TasteItem to a GroundingTaste metadata point.
func itemToGroundingTaste(item TasteItem) GroundingTaste {
	topic := strings.TrimSpace(item.Topic)
	content := strings.TrimSpace(item.Content)
	preview := content
	if utf8.RuneCountInString(preview) > 120 {
		preview = string([]rune(preview)[:117]) + "..."
	}
	return GroundingTaste{
		ID:       item.ID,
		Category: item.Category,
		Topic:    topic,
		Summary:  preview,
		Preview:  preview,
		Content:  content,
	}
}

// FormatBrandTastePromptWithGrounding formats brand taste prompt and returns injected grounding metadata.
func (s *Server) FormatBrandTastePromptWithGrounding(ctx context.Context) (string, []GroundingTaste) {
	var captured []GroundingTaste
	captureCtx := WithTasteGroundingCapture(ctx, &captured)
	prompt := s.formatBrandTastePrompt(captureCtx)
	return prompt, captured
}

// FormatTechnicalTastePromptWithGrounding formats technical taste prompt and returns injected grounding metadata.
func (s *Server) FormatTechnicalTastePromptWithGrounding(ctx context.Context) (string, []GroundingTaste) {
	var captured []GroundingTaste
	captureCtx := WithTasteGroundingCapture(ctx, &captured)
	prompt := s.formatTechnicalTastePrompt(captureCtx)
	return prompt, captured
}

// ActiveBrandGroundingTastes returns active Brand Taste grounding items for the current context owner.
func (s *Server) ActiveBrandGroundingTastes(ctx context.Context) []GroundingTaste {
	ownerID := ""
	if owner := auth.GetOwnerFromContext(ctx); owner != nil {
		ownerID = owner.ID
	}
	return s.ActiveBrandGroundingTastesForOwner(ownerID)
}

// ActiveTechnicalGroundingTastes returns active Technical Taste grounding items for the current context owner.
func (s *Server) ActiveTechnicalGroundingTastes(ctx context.Context) []GroundingTaste {
	ownerID := ""
	if owner := auth.GetOwnerFromContext(ctx); owner != nil {
		ownerID = owner.ID
	}
	return s.ActiveTechnicalGroundingTastesForOwner(ownerID)
}

// ActiveGroundingTastes returns all active grounding items (Brand and Technical) for the current context owner.
func (s *Server) ActiveGroundingTastes(ctx context.Context) []GroundingTaste {
	ownerID := ""
	if owner := auth.GetOwnerFromContext(ctx); owner != nil {
		ownerID = owner.ID
	}
	return s.ActiveGroundingTastesForOwner(ownerID)
}

// ActiveBrandGroundingTastesForOwner returns active Brand Taste grounding items for a specific owner.
func (s *Server) ActiveBrandGroundingTastesForOwner(owner string) []GroundingTaste {
	profile, err := s.getTasteProfile(owner)
	if err != nil || len(profile.BrandItems) == 0 {
		return nil
	}
	tastes := make([]GroundingTaste, 0, len(profile.BrandItems))
	for _, it := range profile.BrandItems {
		tastes = append(tastes, itemToGroundingTaste(it))
	}
	return tastes
}

// ActiveTechnicalGroundingTastesForOwner returns active Technical Taste grounding items for a specific owner.
func (s *Server) ActiveTechnicalGroundingTastesForOwner(owner string) []GroundingTaste {
	profile, err := s.getTasteProfile(owner)
	if err != nil || len(profile.TechnicalItems) == 0 {
		return nil
	}
	tastes := make([]GroundingTaste, 0, len(profile.TechnicalItems))
	for _, it := range profile.TechnicalItems {
		tastes = append(tastes, itemToGroundingTaste(it))
	}
	return tastes
}

// ActiveGroundingTastesForOwner returns all active grounding items (Brand + Technical) for a specific owner.
func (s *Server) ActiveGroundingTastesForOwner(owner string) []GroundingTaste {
	brand := s.ActiveBrandGroundingTastesForOwner(owner)
	tech := s.ActiveTechnicalGroundingTastesForOwner(owner)
	all := make([]GroundingTaste, 0, len(brand)+len(tech))
	all = append(all, brand...)
	all = append(all, tech...)
	return all
}

// RecordMessageGrounding stores the grounding tastes injected into a specific chat message.
func (s *Server) RecordMessageGrounding(messageID string, tastes []GroundingTaste) {
	if strings.TrimSpace(messageID) == "" || len(tastes) == 0 {
		return
	}
	s.messageGrounding.Store(messageID, tastes)
}

// GetMessageGrounding retrieves stored grounding tastes for a specific chat message.
func (s *Server) GetMessageGrounding(messageID string) ([]GroundingTaste, bool) {
	if strings.TrimSpace(messageID) == "" {
		return nil, false
	}
	val, ok := s.messageGrounding.Load(messageID)
	if !ok || val == nil {
		return nil, false
	}
	tastes, ok := val.([]GroundingTaste)
	return tastes, ok
}

// activeBrandGroundingTastesForRoom determines active brand grounding tastes for a given chat room.
func (s *Server) activeBrandGroundingTastesForRoom(ctx context.Context, roomID string) []GroundingTaste {
	ownerID := ""
	if owner := auth.GetOwnerFromContext(ctx); owner != nil {
		ownerID = owner.ID
	}
	if ownerID == "" && s.projectStudioRepo != nil {
		if store, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
			if reqs, err := store.ListInboxRequests(ctx, roomID); err == nil && len(reqs) > 0 && reqs[0].OwnerID != "" {
				ownerID = reqs[0].OwnerID
			}
		}
	}
	if ownerID == "" {
		ownerID = "default"
	}
	return s.ActiveBrandGroundingTastesForOwner(ownerID)
}

// InspectActiveGroundingTastes inspects and returns active grounding tastes for a session Studio chat.
func (s *Server) InspectActiveGroundingTastes(ctx context.Context, sessionID string) (*SessionGroundingTastesResponse, error) {
	ownerID := ""
	if owner := auth.GetOwnerFromContext(ctx); owner != nil {
		ownerID = owner.ID
	}
	if ownerID == "" && sessionID != "" && s.projectStudioRepo != nil {
		if store, ok := s.projectStudioRepo.(db.InboxRequestRepository); ok {
			if reqs, err := store.ListInboxRequests(ctx, sessionID); err == nil && len(reqs) > 0 && reqs[0].OwnerID != "" {
				ownerID = reqs[0].OwnerID
			}
		}
	}
	if ownerID == "" {
		ownerID = "default"
	}

	brandTastes := s.ActiveBrandGroundingTastesForOwner(ownerID)
	techTastes := s.ActiveTechnicalGroundingTastesForOwner(ownerID)
	allTastes := make([]GroundingTaste, 0, len(brandTastes)+len(techTastes))
	allTastes = append(allTastes, brandTastes...)
	allTastes = append(allTastes, techTastes...)

	return &SessionGroundingTastesResponse{
		SessionID:       sessionID,
		RoomID:          sessionID,
		BrandTastes:     brandTastes,
		TechnicalTastes: techTastes,
		AllTastes:       allTastes,
		Total:           len(allTastes),
	}, nil
}

// GetActiveGroundingTastesForSession returns all active grounding tastes for a session Studio chat.
func (s *Server) GetActiveGroundingTastesForSession(ctx context.Context, sessionID string) ([]GroundingTaste, error) {
	resp, err := s.InspectActiveGroundingTastes(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return resp.AllTastes, nil
}

// GetActiveBrandGroundingTastesForSession returns active brand grounding tastes for a session Studio chat.
func (s *Server) GetActiveBrandGroundingTastesForSession(ctx context.Context, sessionID string) ([]GroundingTaste, error) {
	resp, err := s.InspectActiveGroundingTastes(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return resp.BrandTastes, nil
}

// handleGetActiveTasteGrounding handles GET /api/v1/taste/grounding
func (s *Server) handleGetActiveTasteGrounding(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}

	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if sessionID == "" {
		sessionID = strings.TrimSpace(r.URL.Query().Get("room_id"))
	}

	category := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("category")))

	resp, err := s.InspectActiveGroundingTastes(r.Context(), sessionID)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Gagal menginspeksi active taste grounding: "+err.Error())
		return
	}

	if category == "brand" {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"session_id":       resp.SessionID,
			"room_id":          resp.RoomID,
			"grounding_tastes": resp.BrandTastes,
			"total":            len(resp.BrandTastes),
		})
		return
	}
	if category == "technical" {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"session_id":       resp.SessionID,
			"room_id":          resp.RoomID,
			"grounding_tastes": resp.TechnicalTastes,
			"total":            len(resp.TechnicalTastes),
		})
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

// handleGetConversationTasteGrounding handles GET /api/v1/chat/conversations/{id}/grounding
func (s *Server) handleGetConversationTasteGrounding(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}

	convID := strings.TrimSpace(chi.URLParam(r, "id"))
	if convID == "" {
		respondJSONError(w, http.StatusBadRequest, "conversation id is required")
		return
	}

	resp, err := s.InspectActiveGroundingTastes(r.Context(), convID)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "Gagal menginspeksi grounding tastes percakapan: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, resp)
}
