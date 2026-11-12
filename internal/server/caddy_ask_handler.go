package server

import (
	"errors"
	"net/http"
	"strings"

	cuanDB "github.com/buchin/cuan/internal/db"
)

// CaddyAskResponse defines the structured response for /api/caddy/ask gatekeeper checks.
type CaddyAskResponse struct {
	Status  string `json:"status"`
	Domain  string `json:"domain,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// handleCaddyAsk serves as the security gatekeeper endpoint for Caddy On-Demand TLS certificate issuance.
// Route: GET /api/caddy/ask?domain={domain}
// Aliases: GET /api/domains/check-ask?domain={domain}, GET /api/v1/caddy/ask?domain={domain}
//
// Validation & Security Rules:
// 1. Normalizes the domain parameter (trims whitespace, converts to lowercase).
// 2. Rejects empty, missing, or malformed domain strings with HTTP 403 Forbidden.
// 3. Verifies domain registration in DomainRepository via GetCustomDomainByDomain.
// 4. Returns HTTP 200 OK only if the domain is registered and verified (DNSStatus == 'verified'/'active' or SSLStatus == 'active').
// 5. Returns HTTP 403 Forbidden if not found, pending verification, failed, or repository error.
// This strictly prevents ACME CA rate-limit exhaustion and Denial-of-Wallet attacks.
func (s *Server) handleCaddyAsk(w http.ResponseWriter, r *http.Request) {
	rawDomain := r.URL.Query().Get("domain")
	domain := cuanDB.NormalizeDomain(rawDomain)

	// Missing or empty domain query param
	if domain == "" {
		respondJSON(w, http.StatusForbidden, CaddyAskResponse{
			Status: "forbidden",
			Error:  "missing domain parameter",
		})
		return
	}

	// Validate domain name syntax (RFC 1123 / RFC 952 compliance)
	if err := cuanDB.ValidateDomain(domain); err != nil {
		respondJSON(w, http.StatusForbidden, CaddyAskResponse{
			Status: "forbidden",
			Domain: domain,
			Error:  "invalid domain name format: " + err.Error(),
		})
		return
	}

	// Verify base domain authorization (e.g. cuan.work, app.cuan.work, cuan.chat, or configured BaseDomain)
	normalizedDomain := cuanDB.NormalizeDomain(domain)
	if (s.cfg != nil && s.cfg.BaseDomain != "" && strings.EqualFold(normalizedDomain, cuanDB.NormalizeDomain(s.cfg.BaseDomain))) ||
		strings.EqualFold(normalizedDomain, "cuan.work") ||
		strings.EqualFold(normalizedDomain, "app.cuan.work") ||
		strings.EqualFold(normalizedDomain, "cuan.chat") {
		respondJSON(w, http.StatusOK, CaddyAskResponse{
			Status:  "authorized",
			Domain:  domain,
			Message: "base domain is authorized for on-demand TLS certificate issuance",
		})
		return
	}

	// Verify repository readiness
	if s.domainRepo == nil {
		respondJSON(w, http.StatusForbidden, CaddyAskResponse{
			Status: "forbidden",
			Domain: domain,
			Error:  "domain repository is unavailable",
		})
		return
	}

	// Lookup domain in database
	cd, err := s.domainRepo.GetCustomDomainByDomain(r.Context(), domain)
	if err != nil {
		if errors.Is(err, cuanDB.ErrDomainNotFound) {
			respondJSON(w, http.StatusForbidden, CaddyAskResponse{
				Status: "forbidden",
				Domain: domain,
				Error:  "custom domain is not registered",
			})
			return
		}
		respondJSON(w, http.StatusForbidden, CaddyAskResponse{
			Status: "forbidden",
			Domain: domain,
			Error:  "failed to query domain: " + err.Error(),
		})
		return
	}

	// Gatekeeper check: domain must be verified in DNS status or have active SSL
	isVerified := strings.EqualFold(cd.DNSStatus, "verified") ||
		strings.EqualFold(cd.DNSStatus, "active") ||
		strings.EqualFold(cd.SSLStatus, "active")

	if !isVerified {
		respondJSON(w, http.StatusForbidden, CaddyAskResponse{
			Status: "forbidden",
			Domain: domain,
			Error:  "domain ownership is pending verification or unverified",
		})
		return
	}

	// Authorized domain -> allow Caddy On-Demand TLS handshake to proceed
	respondJSON(w, http.StatusOK, CaddyAskResponse{
		Status:  "authorized",
		Domain:  domain,
		Message: "domain is authorized for on-demand TLS certificate issuance",
	})
}
