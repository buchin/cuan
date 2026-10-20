package server

import (
	"context"
	"encoding/json"
	"github.com/buchin/cuan/internal/git"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

var githubConnectionMu sync.Mutex

func githubConnectionPath(owner string) (string, error) {
	path, err := ownerPreferencesPath(owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "github", filepath.Base(path)), nil
}
func (s *Server) handleGitHubConnection(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	path, err := githubConnectionPath(owner)
	if err != nil {
		respondJSONError(w, 503, "GitHub connection storage is unavailable.")
		return
	}
	githubConnectionMu.Lock()
	defer githubConnectionMu.Unlock()
	switch r.Method {
	case http.MethodPut:
		var request struct {
			Token string `json:"token"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&request) != nil || len(request.Token) > 4096 {
			respondJSONError(w, 400, "Enter a valid GitHub access token.")
			return
		}
		account, err := git.NewClient().Account(r.Context(), request.Token)
		if err != nil {
			respondJSONError(w, 400, "Could not connect to GitHub. Check the token and try again.")
			return
		}
		if git.SaveConnection(path, s.cfg.AuthSecret, request.Token, account) != nil {
			respondJSONError(w, 503, "Could not save GitHub access. Try again.")
			return
		}
		respondJSON(w, 200, map[string]any{"connected": true, "account": account})
	case http.MethodDelete:
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			respondJSONError(w, 503, "Could not disconnect GitHub. Try again.")
			return
		}
		respondJSON(w, 200, map[string]any{"connected": false})
	default:
		account, _, err := git.LoadConnection(path, s.cfg.AuthSecret)
		if os.IsNotExist(err) {
			respondJSON(w, 200, map[string]any{"connected": false})
			return
		}
		if err != nil {
			respondJSONError(w, 503, "Saved GitHub access is unavailable. Reconnect GitHub.")
			return
		}
		respondJSON(w, 200, map[string]any{"connected": true, "account": account})
	}
}
func (s *Server) handleGitHubRepositories(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	page := 1
	if raw := r.URL.Query().Get("page"); raw != "" {
		var err error
		page, err = strconv.Atoi(raw)
		if err != nil || page < 1 || page > 1000 {
			respondJSONError(w, 400, "Invalid repository page.")
			return
		}
	}
	path, err := githubConnectionPath(owner)
	if err != nil {
		respondJSONError(w, 503, "GitHub connection storage is unavailable.")
		return
	}
	githubConnectionMu.Lock()
	_, token, err := git.LoadConnection(path, s.cfg.AuthSecret)
	githubConnectionMu.Unlock()
	if err != nil {
		respondJSONError(w, 409, "Connect GitHub in Settings first.")
		return
	}
	repos, err := git.NewClient().AccountRepositories(r.Context(), token, page)
	if err != nil {
		respondJSONError(w, 502, "Could not load repositories. Check your GitHub connection and try again.")
		return
	}
	respondJSON(w, 200, map[string]any{"repositories": repos, "page": page, "has_more": len(repos) == 50})
}

func (s *Server) configureSharedGitHubAccess() {
	if s.projectSvc == nil {
		return
	}
	s.projectSvc.SetSharedRepositoryAccess(func(ctx context.Context) (string, error) {
		owner, err := s.authSvc.OwnerID(ctx)
		if err != nil {
			return "", err
		}
		path, err := githubConnectionPath(owner)
		if err != nil {
			return "", err
		}
		githubConnectionMu.Lock()
		defer githubConnectionMu.Unlock()
		_, token, err := git.LoadConnection(path, s.cfg.AuthSecret)
		if os.IsNotExist(err) {
			return "", nil
		}
		return token, err
	})
}
