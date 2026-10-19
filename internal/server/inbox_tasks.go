package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
	"github.com/go-chi/chi/v5"
)

// Explicit user action: capture work from a message on the canonical task board.
// Creation does not authorize or pretend to start execution.
func (s *Server) handleInboxMessageTask(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	if s.taskSvc == nil || s.projectStudioRepo == nil {
		respondJSONError(w, 503, "Task belum tersedia")
		return
	}
	resolved := s.resolveConversationContext(r.Context(), chi.URLParam(r, "id"), "")
	id := chi.URLParam(r, "messageID")
	if !db.IsValidUUID(id) {
		respondJSONError(w, 400, "ID pesan tidak valid")
		return
	}
	messages, err := s.projectStudioRepo.ListMessagesByRoomID(r.Context(), resolved.RoomID)
	if err != nil {
		respondJSONError(w, 503, "Pesan belum dapat dibaca")
		return
	}
	var source *db.ProjectMessage
	for i := range messages {
		if messages[i].ID == id {
			source = &messages[i]
			break
		}
	}
	if source == nil {
		respondJSONError(w, 404, "Pesan tidak ditemukan dalam percakapan ini")
		return
	}
	var req struct {
		ProjectID    string       `json:"project_id"`
		Title        string       `json:"title"`
		Assignee     string       `json:"assignee,omitempty"`
		DoDChecklist []db.DoDItem `json:"dod_checklist,omitempty"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&req) != nil {
		respondJSONError(w, 400, "Permintaan task tidak valid")
		return
	}
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.Title = strings.TrimSpace(req.Title)
	if req.ProjectID == "" || req.Title == "" || len([]rune(req.Title)) > 200 {
		respondJSONError(w, 400, "Pilih project dan isi judul maksimal 200 karakter")
		return
	}
	if s.projectSvc == nil {
		respondJSONError(w, 503, "Project belum tersedia")
		return
	}
	project, err := s.projectSvc.Get(r.Context(), req.ProjectID)
	if err != nil || project == nil {
		respondJSONError(w, 404, "Project tidak ditemukan")
		return
	}
	assignee := resolved.TargetAgentID
	if strings.TrimSpace(req.Assignee) != "" {
		if s.agentRepo == nil {
			respondJSONError(w, 503, "Daftar agen belum tersedia")
			return
		}
		a, e := s.agentRepo.GetAgentByID(r.Context(), req.Assignee)
		if e != nil || a == nil || !a.Enabled {
			respondJSONError(w, 400, "Pilih agen terdaftar yang aktif")
			return
		}
		assignee = a.ID
	}
	if project.IsArchived {
		respondJSONError(w, 409, "Project diarsipkan. Pilih project aktif.")
		return
	}
	instructions := fmt.Sprintf("%s\n\nSumber: percakapan %s, pesan %s.\nKerjakan sesuai instruksi dan laporkan hasil serta bukti verifikasi; jangan menandai selesai hanya karena proses keluar dengan kode nol.", source.Content, resolved.RoomID, id)
	if len(req.DoDChecklist) > 0 {
		instructions += "\n\nKriteria selesai (belum diverifikasi):"
		for _, item := range req.DoDChecklist {
			if text := strings.TrimSpace(item.Text); text != "" {
				instructions += "\n- " + text
			}
		}
	}
	created, err := s.taskSvc.CreateTaskFromMessage(r.Context(), owner, id, task.CreateTaskRequest{SourceRoomID: resolved.RoomID, ProjectID: req.ProjectID, Title: req.Title, Instructions: instructions, AssignedAgent: assignee})
	if err != nil {
		respondJSONError(w, 503, "Task belum dapat disimpan; coba lagi dari pesan yang sama")
		return
	}
	if created.ProjectID != req.ProjectID {
		respondJSONError(w, 409, "Pesan sudah terhubung ke task di project lain")
		return
	}
	s.publishInbox(resolved.RoomID)
	respondJSON(w, 200, map[string]any{"task": created, "source_message_id": id, "status": created.Status})
}
