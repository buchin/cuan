package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	sdk "github.com/buchin/cuan/pkg/extensions"
	"github.com/go-chi/chi/v5"
)

// SetExtensionRegistry is a bootstrap operation, before serving requests. Native
// extensions are trusted application code; runtime package installation is not supported.
func (s *Server) SetExtensionRegistry(registry *sdk.Registry) {
	s.extensions = registry
	if s.taskSvc != nil {
		s.taskSvc.SetExtensionRegistry(registry)
	}
}
func (s *Server) prepareExtension(ctx context.Context, point string, scope sdk.Scope, input, output any) error {
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	if s.extensions != nil {
		raw, err = s.extensions.Prepare(ctx, point, scope, raw)
		if err != nil {
			return err
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(output); err != nil {
		return err
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return sdk.ErrCallback
	}
	return nil
}
func (s *Server) publishExtension(ctx context.Context, kind, resource, project string, at time.Time) {
	if s.extensions == nil {
		return
	}
	event := sdk.Event{ID: delegationMessageID(kind + ":" + resource + ":" + at.Format(time.RFC3339Nano)), Type: kind, Version: 1, Scope: sdk.Scope{ProjectID: project, ResourceID: resource}, At: at}
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), 500*time.Millisecond)
	defer cancel()
	if failures := s.extensions.Publish(bounded, event); len(failures) > 0 {
		log.Printf("extension event delivery incomplete type=%s failures=%d", kind, len(failures))
	}
}
func (s *Server) handleExtensions(w http.ResponseWriter, r *http.Request) {
	if s.accountOwner(w, r) == "" {
		return
	}
	items := []sdk.Status{}
	if s.extensions != nil {
		items = s.extensions.List()
	}
	respondJSON(w, 200, map[string]any{"api_version": sdk.APIVersion, "extensions": items, "installation": "trusted build-time extensions"})
}
func (s *Server) handleExtensionAction(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	var body struct {
		ProjectID  string          `json:"project_id"`
		ResourceID string          `json:"resource_id"`
		Input      json.RawMessage `json:"input"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 70000))
	d.DisallowUnknownFields()
	if d.Decode(&body) != nil || d.Decode(&struct{}{}) != io.EOF || len(body.ResourceID) > 200 {
		respondJSONError(w, 400, "Invalid extension action")
		return
	}
	if s.extensions == nil {
		respondJSONError(w, 404, "Extension unavailable")
		return
	}
	if body.ProjectID != "" {
		if s.projectSvc == nil {
			respondJSONError(w, 503, "Projects unavailable")
			return
		}
		if _, err := s.projectSvc.Get(r.Context(), body.ProjectID); err != nil {
			respondJSONError(w, 404, "Project not found")
			return
		}
	}
	result, err := s.extensions.Action(r.Context(), chi.URLParam(r, "extensionID"), chi.URLParam(r, "action"), sdk.Scope{OwnerID: owner, ProjectID: body.ProjectID, ResourceID: body.ResourceID}, body.Input)
	if err != nil {
		if err == sdk.ErrUnavailable {
			respondJSONError(w, 404, "Extension unavailable")
		} else {
			respondJSONError(w, 422, "The extension could not complete this action")
		}
		return
	}
	respondJSON(w, 200, json.RawMessage(result))
}
