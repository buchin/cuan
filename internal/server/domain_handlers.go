package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/deployer"
)

// CreateCustomDomainRequest defines the payload for registering a custom domain to a deployment.
type CreateCustomDomainRequest struct {
	Domain string `json:"domain"`
}

// DNSInstructionDetail describes the DNS record to configure at registrar / DNS provider.
type DNSInstructionDetail struct {
	Type        string `json:"type"`                  // "CNAME", "TXT", "A"
	Name        string `json:"name"`                  // Hostname or Subdomain
	Target      string `json:"target,omitempty"`      // Target value for CNAME / A
	Value       string `json:"value,omitempty"`       // Content value for TXT
	TTL         int    `json:"ttl,omitempty"`         // Suggested TTL in seconds
	Description string `json:"description,omitempty"` // User guidance
}

// CustomDomainInstructions packages actionable DNS setup guidance for users.
type CustomDomainInstructions struct {
	CNAME DNSInstructionDetail `json:"cname"`
	TXT   DNSInstructionDetail `json:"txt"`
	A     DNSInstructionDetail `json:"a"`
}

// buildDNSInstructions generates DNS configuration instructions based on canonical host settings.
func (s *Server) buildDNSInstructions(domainName, verificationToken string) CustomDomainInstructions {
	canonicalHost := "cuan.app"
	if s.cfg != nil && s.cfg.BaseDomain != "" {
		canonicalHost = s.cfg.BaseDomain
	}

	targetIP := "127.0.0.1"
	if s.dnsVerifier != nil && len(s.dnsVerifier.TargetIPs()) > 0 {
		targetIP = s.dnsVerifier.TargetIPs()[0]
	}

	return CustomDomainInstructions{
		CNAME: DNSInstructionDetail{
			Type:        "CNAME",
			Name:        domainName,
			Target:      canonicalHost,
			TTL:         300,
			Description: fmt.Sprintf("Point CNAME record for %s to %s", domainName, canonicalHost),
		},
		TXT: DNSInstructionDetail{
			Type:        "TXT",
			Name:        "_cuan-challenge." + domainName,
			Value:       verificationToken,
			TTL:         300,
			Description: fmt.Sprintf("Set TXT record at _cuan-challenge.%s for domain ownership verification", domainName),
		},
		A: DNSInstructionDetail{
			Type:        "A",
			Name:        domainName,
			Target:      targetIP,
			TTL:         300,
			Description: fmt.Sprintf("Point A record to %s (alternative for apex domain with CNAME flattening)", targetIP),
		},
	}
}

// handleCreateDeploymentDomain registers a new custom domain for an existing deployment.
// Route: POST /api/deployments/{id}/domains
// Aliases: POST /api/v1/deployments/{id}/domains
func (s *Server) handleCreateDeploymentDomain(w http.ResponseWriter, r *http.Request) {
	if s.domainRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "domain repository is unavailable")
		return
	}

	deploymentID := chi.URLParam(r, "id")
	if strings.TrimSpace(deploymentID) == "" {
		respondJSONError(w, http.StatusBadRequest, "deployment id is required")
		return
	}

	// Verify that the deployment actually exists
	if s.deployerDriver != nil {
		dep, err := s.deployerDriver.Get(r.Context(), deploymentID)
		if err != nil {
			if errors.Is(err, deployer.ErrDeploymentNotFound) {
				respondJSONError(w, http.StatusNotFound, "deployment not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to verify deployment: "+err.Error())
			return
		}
		if dep == nil {
			respondJSONError(w, http.StatusNotFound, "deployment not found")
			return
		}
	}

	var req CreateCustomDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid json request body: "+err.Error())
		return
	}

	domainName := cuanDB.NormalizeDomain(req.Domain)
	if err := cuanDB.ValidateDomain(domainName); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid domain name: "+err.Error())
		return
	}

	// Check if domain is already registered
	existing, err := s.domainRepo.GetCustomDomainByDomain(r.Context(), domainName)
	if err == nil && existing != nil {
		respondJSONError(w, http.StatusConflict, fmt.Sprintf("custom domain '%s' is already registered", domainName))
		return
	} else if err != nil && !errors.Is(err, cuanDB.ErrDomainNotFound) {
		respondJSONError(w, http.StatusInternalServerError, "failed to check domain uniqueness: "+err.Error())
		return
	}

	customDomain := &cuanDB.CustomDomain{
		DeploymentID: deploymentID,
		Domain:       domainName,
		DNSStatus:    "pending",
		SSLStatus:    "pending",
	}

	if err := s.domainRepo.CreateCustomDomain(r.Context(), customDomain); err != nil {
		if errors.Is(err, cuanDB.ErrDuplicateDomain) {
			respondJSONError(w, http.StatusConflict, fmt.Sprintf("custom domain '%s' already exists", domainName))
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to register custom domain: "+err.Error())
		return
	}

	instructions := s.buildDNSInstructions(customDomain.Domain, customDomain.VerificationToken)

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"status":       "success",
		"message":      "custom domain registered successfully",
		"domain":       customDomain.ToDTO(),
		"instructions": instructions,
	})
}

// handleListDeploymentDomains retrieves all custom domains associated with a specific deployment.
// Route: GET /api/deployments/{id}/domains
// Aliases: GET /api/v1/deployments/{id}/domains
func (s *Server) handleListDeploymentDomains(w http.ResponseWriter, r *http.Request) {
	if s.domainRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "domain repository is unavailable")
		return
	}

	deploymentID := chi.URLParam(r, "id")
	if strings.TrimSpace(deploymentID) == "" {
		respondJSONError(w, http.StatusBadRequest, "deployment id is required")
		return
	}

	// Verify deployment existence
	if s.deployerDriver != nil {
		dep, err := s.deployerDriver.Get(r.Context(), deploymentID)
		if err != nil {
			if errors.Is(err, deployer.ErrDeploymentNotFound) {
				respondJSONError(w, http.StatusNotFound, "deployment not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to verify deployment: "+err.Error())
			return
		}
		if dep == nil {
			respondJSONError(w, http.StatusNotFound, "deployment not found")
			return
		}
	}

	domains, err := s.domainRepo.ListCustomDomainsByDeploymentID(r.Context(), deploymentID)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to list custom domains: "+err.Error())
		return
	}

	dtos := make([]cuanDB.CustomDomainDTO, 0, len(domains))
	for _, cd := range domains {
		dtos = append(dtos, cd.ToDTO())
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":        "success",
		"deployment_id": deploymentID,
		"domains":       dtos,
		"count":         len(dtos),
	})
}

// handleListDomains retrieves custom domains across the platform, optionally filtered by deployment_id.
// Route: GET /api/domains
// Aliases: GET /api/v1/domains
func (s *Server) handleListDomains(w http.ResponseWriter, r *http.Request) {
	if s.domainRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "domain repository is unavailable")
		return
	}

	deploymentID := r.URL.Query().Get("deployment_id")
	var domains []cuanDB.CustomDomain
	var err error

	if strings.TrimSpace(deploymentID) != "" {
		domains, err = s.domainRepo.ListCustomDomainsByDeploymentID(r.Context(), deploymentID)
	} else {
		domains, err = s.domainRepo.ListAllCustomDomains(r.Context())
	}

	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to query custom domains: "+err.Error())
		return
	}

	dtos := make([]cuanDB.CustomDomainDTO, 0, len(domains))
	for _, cd := range domains {
		dtos = append(dtos, cd.ToDTO())
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"domains": dtos,
		"count":   len(dtos),
	})
}

// handleGetDomain retrieves metadata and instructions for a specific custom domain by its ID.
// Route: GET /api/domains/{id}
// Aliases: GET /api/v1/domains/{id}
func (s *Server) handleGetDomain(w http.ResponseWriter, r *http.Request) {
	if s.domainRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "domain repository is unavailable")
		return
	}

	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		respondJSONError(w, http.StatusBadRequest, "domain id is required")
		return
	}

	cd, err := s.domainRepo.GetCustomDomainByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, cuanDB.ErrDomainNotFound) {
			respondJSONError(w, http.StatusNotFound, "custom domain not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get custom domain: "+err.Error())
		return
	}

	instructions := s.buildDNSInstructions(cd.Domain, cd.VerificationToken)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "success",
		"domain":       cd.ToDTO(),
		"instructions": instructions,
	})
}

// handleVerifyDomain triggers an instant DNS verification probe for the specified domain.
// If valid, updates domain status to 'verified' and registers dynamic Caddy reverse proxy routing.
// Route: POST /api/domains/{id}/verify
// Aliases: POST /api/v1/domains/{id}/verify
func (s *Server) handleVerifyDomain(w http.ResponseWriter, r *http.Request) {
	if s.domainRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "domain repository is unavailable")
		return
	}
	if s.dnsVerifier == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "dns verification engine is not configured")
		return
	}

	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		respondJSONError(w, http.StatusBadRequest, "domain id is required")
		return
	}

	cd, err := s.domainRepo.GetCustomDomainByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, cuanDB.ErrDomainNotFound) {
			respondJSONError(w, http.StatusNotFound, "custom domain not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to query custom domain: "+err.Error())
		return
	}

	// Execute DNS verification probe
	result, verifyErr := s.dnsVerifier.VerifyDomain(r.Context(), cd.Domain, cd.VerificationToken)
	now := time.Now().UTC()

	if verifyErr != nil {
		_ = s.domainRepo.UpdateCustomDomainStatus(r.Context(), cd.ID, "failed", cd.SSLStatus, &now)
		cd.DNSStatus = "failed"
		cd.LastCheckedAt = &now

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":     "failed",
			"verified":   false,
			"dns_status": "failed",
			"domain":     cd.ToDTO(),
			"error":      verifyErr.Error(),
			"message":    "dns query execution error: " + verifyErr.Error(),
		})
		return
	}

	if result.Verified {
		// Update status to 'verified'
		if err := s.domainRepo.UpdateCustomDomainStatus(r.Context(), cd.ID, "verified", cd.SSLStatus, &now); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to update domain status: "+err.Error())
			return
		}
		cd.DNSStatus = "verified"
		cd.LastCheckedAt = &now

		// Dynamic Caddy Route Sync:
		// Map Host/SNI of verified custom domain to deployment target port / host
		var routeRegistered bool
		if s.caddyRouter != nil && s.deployerDriver != nil {
			dep, depErr := s.deployerDriver.Get(r.Context(), cd.DeploymentID)
			if depErr == nil && dep != nil {
				target := deployer.LocalTarget(dep.InternalPort)
				if dep.ServerID != "" && s.serverRepo != nil {
					srv, srvErr := s.serverRepo.GetByID(r.Context(), dep.ServerID)
					if srvErr == nil && srv != nil && srv.Host != "" {
						target = deployer.RemoteTarget(srv.Host, dep.InternalPort)
					}
				}
				if routeErr := s.caddyRouter.AddCustomDomainRoute(r.Context(), cd.Domain, target); routeErr == nil {
					routeRegistered = true
				}
			}
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":           "success",
			"verified":         true,
			"dns_status":       "verified",
			"domain":           cd.ToDTO(),
			"result":           result,
			"route_registered": routeRegistered,
			"message":          result.Message,
		})
		return
	}

	// Verification check failed or propagation pending
	_ = s.domainRepo.UpdateCustomDomainStatus(r.Context(), cd.ID, "failed", cd.SSLStatus, &now)
	cd.DNSStatus = "failed"
	cd.LastCheckedAt = &now

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "failed",
		"verified":   false,
		"dns_status": "failed",
		"domain":     cd.ToDTO(),
		"result":     result,
		"message":    result.Message,
	})
}

// handleDeleteDomain removes a custom domain from the repository and tears down its dynamic Caddy route.
// Route: DELETE /api/domains/{id}
// Aliases: DELETE /api/v1/domains/{id}
func (s *Server) handleDeleteDomain(w http.ResponseWriter, r *http.Request) {
	if s.domainRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "domain repository is unavailable")
		return
	}

	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		respondJSONError(w, http.StatusBadRequest, "domain id is required")
		return
	}

	cd, err := s.domainRepo.GetCustomDomainByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, cuanDB.ErrDomainNotFound) {
			respondJSONError(w, http.StatusNotFound, "custom domain not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to query custom domain: "+err.Error())
		return
	}

	// Teardown dynamic Caddy reverse proxy route
	if s.caddyRouter != nil {
		_ = s.caddyRouter.RemoveCustomDomainRoute(r.Context(), cd.Domain)
	}

	if err := s.domainRepo.DeleteCustomDomain(r.Context(), id); err != nil {
		if errors.Is(err, cuanDB.ErrDomainNotFound) {
			respondJSONError(w, http.StatusNotFound, "custom domain not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to delete custom domain: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "custom domain deleted successfully",
		"domain":  cd.Domain,
	})
}
