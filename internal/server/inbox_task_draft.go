package server

import (
	"encoding/json"
	"strings"
)

// Stored in the agent's reply so drafts survive SSE reconnects and reloads.
// A draft has no side effects: only the authenticated capture endpoint creates work.
type InboxTaskDraft struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	ProjectID   string `json:"project_id,omitempty"`
}

const inboxTaskDraftInstruction = `Jika pengguna meminta membuat task dari pembahasan, gunakan riwayat percakapan untuk menyusun SATU draft yang mandiri: judul imperatif, deskripsi berisi tujuan, scope, batasan dan kriteria selesai yang sudah dibahas. Jangan membuat file/task atau menjalankan implementasi. Jelaskan singkat bahwa ini draft, belum tersimpan atau dijalankan. Akhiri balasan dengan satu fenced block berlabel cuan-task, berisi JSON valid dengan field title (maksimal 200 karakter), description (maksimal 6000 karakter), project_id (ID project terdaftar dari snapshot atau string kosong bila ambigu). Contoh format:
` + "```cuan-task\n{\"title\":\"Perbaiki navigasi chat\",\"description\":\"Tujuan dan scope berdasarkan pembahasan.\\nKriteria selesai: ...\",\"project_id\":\"\"}\n```" + `
Hanya buat draft jika diminta, bukan pada sapaan/analisis biasa. Bila tujuan/scope belum jelas, tanyakan dulu. Jangan mengklaim task tersimpan; UI akan menampilkan kartu editable dan pengguna harus menekan Buat task. Jika pengguna merevisi draft, buat draft baru lengkap dengan revisinya. Jangan sertakan credential atau reasoning privat.`

func splitInboxTaskDraft(author, content string) (string, *InboxTaskDraft) {
	if author != "agent" {
		return content, nil
	}
	const marker = "```cuan-task\n"
	trimmed := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	start := strings.LastIndex(trimmed, marker)
	if start < 0 || !strings.HasSuffix(trimmed, "```") || strings.Count(trimmed, marker) != 1 {
		return content, nil
	}
	raw := strings.TrimSpace(trimmed[start+len(marker) : len(trimmed)-3])
	if len(raw) > 32000 {
		return content, nil
	}
	var draft InboxTaskDraft
	if json.Unmarshal([]byte(raw), &draft) != nil {
		return content, nil
	}
	draft.Title = strings.TrimSpace(draft.Title)
	draft.Description = strings.TrimSpace(draft.Description)
	draft.ProjectID = strings.TrimSpace(draft.ProjectID)
	if draft.Title == "" || len([]rune(draft.Title)) > 200 || draft.Description == "" || len([]rune(draft.Description)) > 6000 || len(draft.ProjectID) > 200 {
		return content, nil
	}
	return strings.TrimSpace(trimmed[:start]), &draft
}
