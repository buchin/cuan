package server

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/crypto"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/deployer"
)

// CreateServerPayload represents the JSON request body for registering a BYOS server.
type CreateServerPayload struct {
	Name       string `json:"name"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	AuthType   string `json:"auth_type"`
	PrivateKey string `json:"private_key"`
}

// handleListServers returns all registered BYOS remote servers sanitized via ServerDTO.
// Zero credential leakage is strictly enforced (EncryptedKey is masked and excluded).
func (s *Server) handleListServers(w http.ResponseWriter, r *http.Request) {
	if s.serverRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "server repository is not configured")
		return
	}

	servers, err := s.serverRepo.List(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to list servers: "+err.Error())
		return
	}

	dtos := make([]db.ServerDTO, len(servers))
	for i, srv := range servers {
		dtos[i] = srv.ToDTO()
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"servers": dtos,
		"count":   len(dtos),
	})
}

// handleGetServer retrieves the details of a single remote server by its unique ID.
func (s *Server) handleGetServer(w http.ResponseWriter, r *http.Request) {
	if s.serverRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "server repository is not configured")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "server id is required")
		return
	}

	server, err := s.serverRepo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrServerNotFound) {
			respondJSONError(w, http.StatusNotFound, "server not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get server: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"server": server.ToDTO(),
	})
}

// handleCreateServer registers a new remote BYOS server, encrypting its SSH private key
// with KeyVault AES-256-GCM before persisting to PostgreSQL.
func (s *Server) handleCreateServer(w http.ResponseWriter, r *http.Request) {
	if s.serverRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "server repository is not configured")
		return
	}

	var payload CreateServerPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request json: "+err.Error())
		return
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		respondJSONError(w, http.StatusBadRequest, "name is required")
		return
	}

	host := strings.TrimSpace(payload.Host)
	if host == "" {
		respondJSONError(w, http.StatusBadRequest, "host is required")
		return
	}

	port := payload.Port
	if port <= 0 {
		port = 22
	}

	username := strings.TrimSpace(payload.Username)
	if username == "" {
		username = "ubuntu"
	}

	authType := strings.TrimSpace(payload.AuthType)
	if authType == "" {
		authType = "key"
	}

	privateKey := strings.TrimSpace(payload.PrivateKey)
	if authType == "key" && privateKey == "" {
		respondJSONError(w, http.StatusBadRequest, "private_key is required for key authentication")
		return
	}

	// Validate and compute OpenSSH fingerprint
	var fingerprint string
	if privateKey != "" {
		fp, err := crypto.CalculateSSHFingerprint([]byte(privateKey))
		if err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid private key format: "+err.Error())
			return
		}
		fingerprint = fp
	}

	// Encrypt sensitive private key via KeyVault AES-256-GCM
	vault := s.keyVault
	if vault == nil {
		vault = crypto.DefaultKeyVault()
	}

	var encryptedKey string
	if privateKey != "" {
		enc, err := vault.Encrypt([]byte(privateKey))
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to encrypt credentials: "+err.Error())
			return
		}
		encryptedKey = enc
	}

	// Generate secure unique ID
	idBuf := make([]byte, 8)
	_, _ = rand.Read(idBuf)
	serverID := fmt.Sprintf("srv_%x", idBuf)

	now := time.Now().UTC()
	server := &db.Server{
		ID:             serverID,
		Name:           name,
		Host:           host,
		Port:           port,
		Username:       username,
		AuthType:       authType,
		EncryptedKey:   encryptedKey,
		KeyFingerprint: fingerprint,
		Status:         "pending",
		OSInfo:         make(map[string]interface{}),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.serverRepo.Create(r.Context(), server); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to persist server: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"server": server.ToDTO(),
	})
}

// handleTestServer performs instant connectivity and health probing on a registered server,
// extracting system specifications (OS, CPU, RAM, latency ping) and updating the DB record.
func (s *Server) handleTestServer(w http.ResponseWriter, r *http.Request) {
	if s.serverRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "server repository is not configured")
		return
	}
	if s.sshManager == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "ssh manager is not configured")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "server id is required")
		return
	}

	server, err := s.serverRepo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrServerNotFound) {
			respondJSONError(w, http.StatusNotFound, "server not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to retrieve server: "+err.Error())
		return
	}

	vault := s.keyVault
	if vault == nil {
		vault = crypto.DefaultKeyVault()
	}

	decryptedKey, err := vault.Decrypt(server.EncryptedKey)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to decrypt server credentials: "+err.Error())
		return
	}

	// Probe host via pooled/dialed SSH client with 5s timeout
	specs, probeErr := s.sshManager.Probe(r.Context(), server, decryptedKey)
	if probeErr != nil {
		newStatus := "offline"
		if errors.Is(probeErr, deployer.ErrSSHAuthFailed) {
			newStatus = "error"
		}

		errInfo := map[string]interface{}{
			"error": probeErr.Error(),
		}
		_ = s.serverRepo.UpdateStatus(r.Context(), server.ID, newStatus, server.LastPingAt, errInfo)
		server.Status = newStatus
		server.OSInfo = errInfo

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"status":  newStatus,
			"error":   probeErr.Error(),
			"server":  server.ToDTO(),
		})
		return
	}

	now := time.Now().UTC()
	newStatus := "online"
	osInfo := specs.ToMap()

	if err := s.serverRepo.UpdateStatus(r.Context(), server.ID, newStatus, &now, osInfo); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to update server health record: "+err.Error())
		return
	}

	server.Status = newStatus
	server.LastPingAt = &now
	server.OSInfo = osInfo

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"status":  newStatus,
		"specs":   specs,
		"server":  server.ToDTO(),
	})
}

// handleDeleteServer removes a server from the database and closes any active SSH connection.
func (s *Server) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	if s.serverRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "server repository is not configured")
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		respondJSONError(w, http.StatusBadRequest, "server id is required")
		return
	}

	if s.sshManager != nil {
		_ = s.sshManager.CloseServer(id)
	}

	if err := s.serverRepo.Delete(r.Context(), id); err != nil {
		if errors.Is(err, db.ErrServerNotFound) {
			respondJSONError(w, http.StatusNotFound, "server not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to delete server: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "server deleted",
	})
}
