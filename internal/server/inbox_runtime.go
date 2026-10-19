package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
)

const inboxRuntimeLineLimit = 1024 * 1024
const inboxRuntimeByteLimit = 16 * 1024 * 1024

var inboxRuntimeID = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

// Consume NDJSON incrementally. Arguments, output, text deltas and reasoning are
// intentionally absent from the decoded event model and never become trace text.
type inboxRuntimeStream struct {
	pending                                 []byte
	result                                  json.RawMessage
	bytes, events, traceCount, toolFailures int
	invalid, overflow, truncated            bool
	seen                                    map[string]bool
	step                                    func(string, string)
}

func (s *inboxRuntimeStream) emit(kind, text string) {
	if s.step == nil {
		return
	}
	if s.traceCount >= 64 {
		if !s.truncated {
			s.truncated = true
			s.step("runtime_trace_limited", "Batas detail aktivitas tercapai; hasil akhir tetap diperiksa.")
		}
		return
	}
	s.traceCount++
	s.step(kind, text)
}
func (s *inboxRuntimeStream) Write(p []byte) (int, error) {
	n := len(p)
	s.bytes += n
	if s.overflow {
		return n, nil
	}
	if s.bytes > inboxRuntimeByteLimit {
		s.overflow = true
		s.pending = nil
		return n, nil
	}
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		part := p
		if i >= 0 {
			part = p[:i]
		}
		if len(s.pending)+len(part) > inboxRuntimeLineLimit {
			s.overflow = true
			s.pending = nil
			return n, nil
		}
		s.pending = append(s.pending, part...)
		if i < 0 {
			break
		}
		s.consume(s.pending)
		if s.overflow {
			s.pending = nil
			return n, nil
		}
		s.pending = nil
		p = p[i+1:]
	}
	return n, nil
}
func (s *inboxRuntimeStream) consume(line []byte) {
	if len(bytes.TrimSpace(line)) == 0 {
		return
	}
	if len(s.result) > 0 {
		s.invalid = true
		return
	}
	s.events++
	if s.events > 10000 {
		s.overflow = true
		return
	}
	var e struct {
		Event          string          `json:"event"`
		ConversationID string          `json:"conversation_id"`
		Result         json.RawMessage `json:"result"`
		Update         struct {
			Index    *int     `json:"step_index"`
			State    string   `json:"state"`
			Type     string   `json:"step_type"`
			Tool     string   `json:"tool_name"`
			Duration *float64 `json:"duration_seconds"`
		} `json:"step_update"`
	}
	if json.Unmarshal(line, &e) != nil {
		s.invalid = true
		return
	}
	switch e.Event {
	case "init":
		s.emit("runtime_init", "Sesi runtime dimulai.")
		if inboxRuntimeID.MatchString(e.ConversationID) {
			s.emit("runtime_session", "ID sesi runtime: "+e.ConversationID)
		}
	case "result":
		if len(s.result) > 0 || len(e.Result) == 0 || string(e.Result) == "null" {
			s.invalid = true
			return
		}
		s.result = append([]byte(nil), e.Result...)
		var final struct {
			DeniedActions []json.RawMessage `json:"denied_actions"`
		}
		if json.Unmarshal(e.Result, &final) == nil && len(final.DeniedActions) > 0 {
			s.emit("runtime_permission_denied", "Izin tool ditolak oleh runtime. Aktivitas berakhir bukan berarti perintah berhasil dijalankan.")
		}
	case "step_update":
		u := e.Update
		if u.Index == nil || *u.Index < 0 || *u.Index > 1000000 {
			return
		}
		if u.Type != "tool" && u.Type != "agent_response" {
			return
		}
		if u.State != "ACTIVE" && u.State != "DONE" && u.State != "ERROR" {
			return
		}
		key := fmt.Sprintf("%s:%d:%s", u.Type, *u.Index, u.State)
		if s.seen == nil {
			s.seen = map[string]bool{}
		}
		if s.seen[key] {
			return
		}
		if len(s.seen) < 256 {
			s.seen[key] = true
		}
		duration := ""
		if u.Duration != nil && !math.IsNaN(*u.Duration) && !math.IsInf(*u.Duration, 0) && *u.Duration >= 0 && *u.Duration <= 86400 {
			duration = fmt.Sprintf(" (%.1f dtk)", *u.Duration)
		}
		if u.Type == "tool" {
			names := map[string]string{"view_file": "Baca file", "view_file_outline": "Struktur file", "view_code_item": "Baca kode", "list_dir": "Daftar direktori", "grep_search": "Cari teks", "find_by_name": "Cari file", "run_command": "Perintah terminal"}
			name := names[u.Tool]
			if name == "" {
				name = "Tool"
			}
			kind, label := "runtime_tool_started", "dimulai"
			if u.State == "DONE" {
				kind, label = "runtime_tool_finished", "aktivitas berakhir"
			}
			if u.State == "ERROR" {
				kind, label = "runtime_tool_failed", "gagal"
				s.toolFailures++
			}
			s.emit(kind, fmt.Sprintf("Tool #%d · %s %s%s.", *u.Index, name, label, duration))
		} else {
			kind, label := "runtime_provider_started", "Provider menyiapkan respons."
			if u.State == "DONE" {
				kind, label = "runtime_provider_finished", "Tahap respons provider selesai"+duration+"."
			}
			if u.State == "ERROR" {
				kind, label = "runtime_provider_failed", "Tahap respons provider gagal"+duration+"."
			}
			s.emit(kind, label)
		}
	}
}
func (s *inboxRuntimeStream) finish() (string, string, string) {
	if len(s.pending) > 0 {
		s.consume(s.pending)
		s.pending = nil
	}
	if s.overflow {
		return "", "Aktivitas provider melewati batas ukuran. Pesan tersimpan; permintaan tidak dijalankan ulang.", "runtime_overflow"
	}
	if s.invalid {
		return "", "Format event runtime tidak valid. Pesan tersimpan; permintaan tidak dijalankan ulang.", "runtime_invalid"
	}
	if len(s.result) == 0 {
		return "", "Proses berhenti sebelum hasil akhir diterima. Pesan tersimpan; hasil belum pasti dan permintaan tidak dijalankan ulang.", "runtime_missing_result"
	}
	return strings.TrimSpace(string(s.result)), "", ""
}
