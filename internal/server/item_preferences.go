package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/buchin/cuan/internal/db"
)

var itemPreferencesMu sync.Mutex

type itemPreference struct {
	ArchivedAt string `json:"archived_at,omitempty"`
	ReadAt     string `json:"read_at,omitempty"`
	Unread     bool   `json:"unread,omitempty"`
}
type itemPreferences map[string]itemPreference

func itemPreferencesPath(owner string) (string, error) {
	p, err := ownerPreferencesPath(owner)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(p, ".json") + ".items.json", nil
}
func readItemPreferences(owner string) (itemPreferences, error) {
	p, err := itemPreferencesPath(owner)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return itemPreferences{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := itemPreferences{}
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("invalid item preferences")
	}
	return result, nil
}
func writeItemPreferences(owner string, p itemPreferences) error {
	path, err := itemPreferencesPath(owner)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".items-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(raw)
	}
	if err == nil {
		err = f.Sync()
	}
	if e := f.Close(); err == nil {
		err = e
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	return err
}
func (s *Server) handleItemPreferences(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	kind := r.URL.Query().Get("kind")
	if kind != "conversation" && kind != "contact" {
		respondJSONError(w, 400, "Choose conversations or contacts")
		return
	}
	var req struct {
		ID       string     `json:"id"`
		Archived *bool      `json:"archived"`
		Read     *bool      `json:"read"`
		SeenAt   *time.Time `json:"seen_at"`
		All      *bool      `json:"all"`
		IDs      []string   `json:"ids"`
	}
	if r.Method == http.MethodPatch {
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
		d.DisallowUnknownFields()
		if err := d.Decode(&req); err != nil {
			respondJSONError(w, 400, "Choose an item and whether to archive it")
			return
		}
		isAll := req.All != nil && *req.All
		isBatch := len(req.IDs) > 0
		if !isAll && !isBatch {
			if req.ID == "" || len(req.ID) > 200 || strings.IndexFunc(req.ID, unicode.IsControl) >= 0 || (req.Archived == nil && req.Read == nil) || (req.Read != nil && kind != "conversation") {
				respondJSONError(w, 400, "Choose an item and whether to archive it")
				return
			}
		} else {
			if req.Read == nil || kind != "conversation" {
				respondJSONError(w, 400, "Batch update only supports marking conversations as read")
				return
			}
		}
	}
	itemPreferencesMu.Lock()
	defer itemPreferencesMu.Unlock()
	all, err := readItemPreferences(owner)
	if err != nil {
		respondJSONError(w, 503, "Could not load archived items. Try again.")
		return
	}
	if r.Method == http.MethodPatch {
		isAll := req.All != nil && *req.All
		isBatch := len(req.IDs) > 0

		if isAll {
			now := time.Now().UTC()
			nowStr := now.Format(time.RFC3339Nano)
			for k, v := range all {
				if strings.HasPrefix(k, "conversation:") {
					v.Unread = false
					v.ReadAt = nowStr
					all[k] = v
				}
			}
			if s.projectStudioRepo != nil {
				if rooms, err := s.projectStudioRepo.ListAllRooms(r.Context()); err == nil {
					for _, rm := range rooms {
						k := "conversation:" + rm.ID
						v := all[k]
						v.Unread = false
						v.ReadAt = nowStr
						all[k] = v
					}
				}
			}
			if s.agentRepo != nil {
				if agents, err := s.agentRepo.ListAgents(r.Context(), db.AgentFilter{}); err == nil {
					for _, a := range agents {
						k := "conversation:" + a.ID
						v := all[k]
						v.Unread = false
						v.ReadAt = nowStr
						all[k] = v
					}
				}
			}
		} else if isBatch {
			now := time.Now().UTC()
			nowStr := now.Format(time.RFC3339Nano)
			for _, id := range req.IDs {
				if id == "" || len(id) > 200 || strings.IndexFunc(id, unicode.IsControl) >= 0 {
					continue
				}
				key := kind + ":" + id
				value := all[key]
				value.Unread = !*req.Read
				if *req.Read {
					value.ReadAt = nowStr
				}
				all[key] = value
			}
		} else {
			key := kind + ":" + req.ID
			if len(all) >= 20000 {
				if _, exists := all[key]; !exists {
					respondJSONError(w, 409, "Too many saved items. Restore some items first.")
					return
				}
			}
			value := all[key]
			if req.Archived != nil {
				if *req.Archived {
					value.ArchivedAt = time.Now().UTC().Format(time.RFC3339Nano)
				} else {
					value.ArchivedAt = ""
				}
			}
			if req.Read != nil {
				value.Unread = !*req.Read
				if *req.Read {
					seen := time.Now().UTC()
					if req.SeenAt != nil && req.SeenAt.Before(seen) {
						seen = req.SeenAt.UTC()
					}
					previous, _ := time.Parse(time.RFC3339Nano, value.ReadAt)
					if previous.After(seen) {
						seen = previous
					}
					value.ReadAt = seen.Format(time.RFC3339Nano)
				}
			}
			if value.ArchivedAt == "" && value.ReadAt == "" && !value.Unread {
				delete(all, key)
			} else {
				all[key] = value
			}
		}

		if err = writeItemPreferences(owner, all); err != nil {
			respondJSONError(w, 503, "Could not save this change. Try again.")
			return
		}
	}
	items := map[string]itemPreference{}
	for key, value := range all {
		if strings.HasPrefix(key, kind+":") {
			items[strings.TrimPrefix(key, kind+":")] = value
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, 200, map[string]any{"items": items})
}
