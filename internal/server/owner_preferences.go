package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/buchin/cuan/internal/auth"
)

var ownerPreferencesMu sync.Mutex

func ownerPreferencesPath(owner string) (string, error) {
	root, err := attachmentRoot(owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(filepath.Dir(root)), "preferences", filepath.Base(root)+".json"), nil
}
func readOwnerPreferences(owner string) (ownerPreferences, error) {
	p := defaultOwnerPreferences()
	path, err := ownerPreferencesPath(owner)
	if err != nil {
		return p, err
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	err = json.Unmarshal(raw, &p)
	return p, err
}
func writeOwnerPreferences(owner string, p ownerPreferences) error {
	path, err := ownerPreferencesPath(owner)
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
	f, err := os.CreateTemp(filepath.Dir(path), ".preferences-*")
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
func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	ownerPreferencesMu.Lock()
	defer ownerPreferencesMu.Unlock()
	p, err := readOwnerPreferences(owner)
	if err != nil {
		respondJSONError(w, 503, "Could not load your profile. Try again.")
		return
	}
	if r.Method == "PATCH" {
		var req struct {
			DisplayName string `json:"display_name"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&req) != nil {
			respondJSONError(w, 400, "Enter a display name up to 80 characters.")
			return
		}
		name := strings.TrimSpace(req.DisplayName)
		if len([]rune(name)) > 80 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
			respondJSONError(w, 400, "Enter a display name up to 80 characters.")
			return
		}
		p.DisplayName = name
		if err = writeOwnerPreferences(owner, p); err != nil {
			respondJSONError(w, 503, "Could not save your display name. Try again.")
			return
		}
	}
	respondJSON(w, 200, map[string]string{"display_name": p.DisplayName})
}
func (s *Server) handleUsername(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"current_password"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil {
		respondJSONError(w, 400, "Enter a username and your current password.")
		return
	}
	err := s.authSvc.UpdateUsername(r.Context(), owner, req.Username, req.Password)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		respondJSONError(w, 403, "Current password is incorrect.")
		return
	}
	if errors.Is(err, auth.ErrInvalidInput) {
		respondJSONError(w, 400, strings.TrimPrefix(err.Error(), "invalid input: "))
		return
	}
	if err != nil {
		respondJSONError(w, 503, "Could not change your username. Try again.")
		return
	}
	respondJSON(w, 200, map[string]string{"username": strings.ToLower(strings.TrimSpace(req.Username))})
}
