package server

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/buchin/cuan/internal/inspector"
)

// handleCreateProjectFromZip processes multipart zip upload and instantiates project, room, agent, and setup tasks.
// Routes: POST /api/projects/from-zip, POST /api/v1/projects/from-zip
func (s *Server) handleCreateProjectFromZip(w http.ResponseWriter, r *http.Request) {
	if s.inspectorEngine == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "inspector engine is not configured")
		return
	}

	// Limit request body to 151MB (150MB zip + 1MB multipart headers)
	r.Body = http.MaxBytesReader(w, r.Body, 151*1024*1024)
	if err := r.ParseMultipartForm(32 * 1024 * 1024); err != nil {
		respondJSONError(w, http.StatusBadRequest, "failed to parse multipart form or payload too large: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, "file multipart field is required: "+err.Error())
		return
	}
	defer file.Close()

	projectName := strings.TrimSpace(r.FormValue("name"))
	if projectName == "" {
		projectName = strings.TrimSpace(r.FormValue("project_name"))
	}

	// Spool to a temporary file
	tmpFile, err := os.CreateTemp("", "cuan-upload-*.zip")
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to create temp file: "+err.Error())
		return
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	if _, err := io.Copy(tmpFile, file); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to spool uploaded archive: "+err.Error())
		return
	}
	_ = tmpFile.Close()

	req := inspector.CreateFromZipRequest{
		ProjectName: projectName,
		ArchiveName: header.Filename,
		TempZipPath: tmpPath,
	}

	resp, err := s.inspectorEngine.CreateProjectFromZip(r.Context(), req)
	if err != nil {
		if errors.Is(err, inspector.ErrZipSlip) {
			respondJSONError(w, http.StatusBadRequest, "zip slip detected: archive contains invalid relative path")
			return
		}
		if errors.Is(err, inspector.ErrQuotaExceeded) {
			respondJSONError(w, http.StatusRequestEntityTooLarge, "extraction quota exceeded: archive size exceeds 150MB limit")
			return
		}
		if errors.Is(err, inspector.ErrEmptyArchive) || errors.Is(err, inspector.ErrInvalidZip) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to create project from zip: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, resp)
}

// handleInspectZip inspects an uploaded ZIP in an isolated sandbox and returns metadata without saving a project.
// Routes: POST /api/inspector/inspect-zip, POST /api/v1/inspector/inspect-zip
func (s *Server) handleInspectZip(w http.ResponseWriter, r *http.Request) {
	if s.inspectorEngine == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "inspector engine is not configured")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 151*1024*1024)
	if err := r.ParseMultipartForm(32 * 1024 * 1024); err != nil {
		respondJSONError(w, http.StatusBadRequest, "failed to parse multipart form or payload too large: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, "file multipart field is required: "+err.Error())
		return
	}
	defer file.Close()

	// Spool to a temporary file
	tmpFile, err := os.CreateTemp("", "cuan-inspect-spool-*.zip")
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to create temp file: "+err.Error())
		return
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	written, err := io.Copy(tmpFile, file)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to spool uploaded archive: "+err.Error())
		return
	}
	_ = tmpFile.Close()

	f, err := os.Open(tmpPath)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to open spooled archive: "+err.Error())
		return
	}
	defer f.Close()

	inspection, err := s.inspectorEngine.InspectZip(f, written, header.Filename)
	if err != nil {
		if errors.Is(err, inspector.ErrZipSlip) {
			respondJSONError(w, http.StatusBadRequest, "zip slip detected: archive contains invalid relative path")
			return
		}
		if errors.Is(err, inspector.ErrQuotaExceeded) {
			respondJSONError(w, http.StatusRequestEntityTooLarge, "extraction quota exceeded: archive size exceeds 150MB limit")
			return
		}
		if errors.Is(err, inspector.ErrEmptyArchive) || errors.Is(err, inspector.ErrInvalidZip) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to inspect zip: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":              "success",
		"inspection":          inspection,
		"recommended_modules": inspection.RecommendedModules,
		"recommended_tasks":   inspection.RecommendedTasks,
	})
}
