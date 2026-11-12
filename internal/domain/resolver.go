package domain

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	cuanDB "github.com/buchin/cuan/internal/db"
)

var (
	ErrDNSLookupFailed    = errors.New("dns lookup failed")
	ErrVerificationFailed = errors.New("domain verification failed")
)

// DNSResolver defines the interface for performing DNS queries.
// It is fully mockable for unit and integration testing without external network dependencies.
type DNSResolver interface {
	LookupCNAME(ctx context.Context, host string) (string, error)
	LookupTXT(ctx context.Context, name string) ([]string, error)
	LookupIP(ctx context.Context, network, host string) ([]net.IP, error)
}

// NetDNSResolver is the production DNSResolver implementation using Go's net package.
type NetDNSResolver struct {
	Resolver *net.Resolver
}

// NewNetDNSResolver creates a NetDNSResolver wrapping net.DefaultResolver or a custom resolver.
func NewNetDNSResolver(r ...*net.Resolver) *NetDNSResolver {
	res := net.DefaultResolver
	if len(r) > 0 && r[0] != nil {
		res = r[0]
	}
	return &NetDNSResolver{Resolver: res}
}

func (r *NetDNSResolver) LookupCNAME(ctx context.Context, host string) (string, error) {
	return r.Resolver.LookupCNAME(ctx, host)
}

func (r *NetDNSResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	return r.Resolver.LookupTXT(ctx, name)
}

func (r *NetDNSResolver) LookupIP(ctx context.Context, network, host string) ([]net.IP, error) {
	return r.Resolver.LookupIP(ctx, network, host)
}

// MockDNSResolver is a thread-safe DNSResolver implementation for testing.
type MockDNSResolver struct {
	mu     sync.RWMutex
	cnames map[string]string
	txts   map[string][]string
	ips    map[string][]net.IP
	errors map[string]error
}

// NewMockDNSResolver instantiates a clean MockDNSResolver.
func NewMockDNSResolver() *MockDNSResolver {
	return &MockDNSResolver{
		cnames: make(map[string]string),
		txts:   make(map[string][]string),
		ips:    make(map[string][]net.IP),
		errors: make(map[string]error),
	}
}

// SetCNAME registers an expected CNAME response for a host.
func (m *MockDNSResolver) SetCNAME(host, target string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cnames[strings.ToLower(strings.TrimSuffix(host, "."))] = target
}

// SetTXT registers expected TXT records for a name.
func (m *MockDNSResolver) SetTXT(name string, records ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.txts[strings.ToLower(strings.TrimSuffix(name, "."))] = records
}

// SetIP registers expected IP addresses for a host.
func (m *MockDNSResolver) SetIP(host string, ips ...net.IP) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ips[strings.ToLower(strings.TrimSuffix(host, "."))] = ips
}

// SetError registers a forced error for any query matching key.
func (m *MockDNSResolver) SetError(key string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[strings.ToLower(strings.TrimSuffix(key, "."))] = err
}

func (m *MockDNSResolver) LookupCNAME(ctx context.Context, host string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normalized := strings.ToLower(strings.TrimSuffix(host, "."))
	if err, exists := m.errors[normalized]; exists && err != nil {
		return "", err
	}
	if target, exists := m.cnames[normalized]; exists {
		return target, nil
	}
	// Return DNS NXDOMAIN / no such host
	return "", &net.DNSError{
		Err:        "no such host",
		Name:       host,
		IsNotFound: true,
	}
}

func (m *MockDNSResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normalized := strings.ToLower(strings.TrimSuffix(name, "."))
	if err, exists := m.errors[normalized]; exists && err != nil {
		return nil, err
	}
	if records, exists := m.txts[normalized]; exists {
		return records, nil
	}
	return nil, &net.DNSError{
		Err:        "no such host",
		Name:       name,
		IsNotFound: true,
	}
}

func (m *MockDNSResolver) LookupIP(ctx context.Context, network, host string) ([]net.IP, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normalized := strings.ToLower(strings.TrimSuffix(host, "."))
	if err, exists := m.errors[normalized]; exists && err != nil {
		return nil, err
	}
	if ips, exists := m.ips[normalized]; exists {
		return ips, nil
	}
	return nil, &net.DNSError{
		Err:        "no such host",
		Name:       host,
		IsNotFound: true,
	}
}

// VerificationResult encapsulates detailed diagnostics from a domain verification attempt.
type VerificationResult struct {
	Verified      bool      `json:"verified"`
	Method        string    `json:"method,omitempty"` // "cname", "txt", "a_record"
	Domain        string    `json:"domain"`
	CNAMETarget   string    `json:"cname_target,omitempty"`
	TXTRecords    []string  `json:"txt_records,omitempty"`
	IPAddresses   []string  `json:"ip_addresses,omitempty"`
	MatchedTarget string    `json:"matched_target,omitempty"`
	Message       string    `json:"message"`
	Error         string    `json:"error,omitempty"`
	CheckedAt     time.Time `json:"checked_at"`
}

// DNSVerifierConfig contains configuration options for the verification engine.
type DNSVerifierConfig struct {
	CanonicalHosts  []string      // e.g. ["cuan.work", "app.cuan.work"]
	TargetIPs       []string      // e.g. ["127.0.0.1", "::1"]
	ChallengePrefix string        // e.g. "_cuan-challenge."
	Timeout         time.Duration // Default per-lookup timeout
}

// VerifierOption configures a DNSVerifier.
type VerifierOption func(*DNSVerifier)

// WithResolver sets the DNSResolver implementation.
func WithResolver(r DNSResolver) VerifierOption {
	return func(v *DNSVerifier) {
		if r != nil {
			v.resolver = r
		}
	}
}

// WithTimeout sets lookup timeout.
func WithTimeout(d time.Duration) VerifierOption {
	return func(v *DNSVerifier) {
		if d > 0 {
			v.timeout = d
		}
	}
}

// WithCanonicalHosts appends or overrides acceptable canonical CNAME targets.
func WithCanonicalHosts(hosts ...string) VerifierOption {
	return func(v *DNSVerifier) {
		for _, h := range hosts {
			cleaned := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(h), "."))
			if cleaned != "" {
				v.canonicalHosts = append(v.canonicalHosts, cleaned)
			}
		}
	}
}

// WithTargetIPs sets target IP addresses for A-record fallback checks.
func WithTargetIPs(ips ...string) VerifierOption {
	return func(v *DNSVerifier) {
		for _, ip := range ips {
			cleaned := strings.TrimSpace(ip)
			if cleaned != "" {
				v.targetIPs = append(v.targetIPs, cleaned)
			}
		}
	}
}

// DNSVerifier evaluates custom domain ownership and routing correctness via DNS queries.
type DNSVerifier struct {
	resolver        DNSResolver
	canonicalHosts  []string
	targetIPs       []string
	challengePrefix string
	timeout         time.Duration
}

// NewDNSVerifier creates a ready-to-use DNSVerifier with default or supplied configuration.
func NewDNSVerifier(cfg DNSVerifierConfig, opts ...VerifierOption) *DNSVerifier {
	v := &DNSVerifier{
		resolver:        NewNetDNSResolver(),
		challengePrefix: "_cuan-challenge.",
		timeout:         5 * time.Second,
	}

	if cfg.Timeout > 0 {
		v.timeout = cfg.Timeout
	}
	if cfg.ChallengePrefix != "" {
		v.challengePrefix = cfg.ChallengePrefix
	}

	for _, h := range cfg.CanonicalHosts {
		cleaned := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(h), "."))
		if cleaned != "" {
			v.canonicalHosts = append(v.canonicalHosts, cleaned)
		}
	}
	// Fallback canonical hosts if none provided
	if len(v.canonicalHosts) == 0 {
		v.canonicalHosts = []string{"cuan.work", "app.cuan.work"}
	}

	for _, ip := range cfg.TargetIPs {
		cleaned := strings.TrimSpace(ip)
		if cleaned != "" {
			v.targetIPs = append(v.targetIPs, cleaned)
		}
	}
	if len(v.targetIPs) == 0 {
		v.targetIPs = []string{"127.0.0.1", "::1"}
	}

	for _, opt := range opts {
		opt(v)
	}

	return v
}

// Resolver returns the active DNSResolver.
func (v *DNSVerifier) Resolver() DNSResolver {
	return v.resolver
}

// SetResolver replaces the active DNSResolver.
func (v *DNSVerifier) SetResolver(r DNSResolver) {
	if r != nil {
		v.resolver = r
	}
}

// CanonicalHosts returns the list of valid canonical host targets.
func (v *DNSVerifier) CanonicalHosts() []string {
	return v.canonicalHosts
}

// TargetIPs returns the configured server IP targets.
func (v *DNSVerifier) TargetIPs() []string {
	return v.targetIPs
}

// isCanonicalTarget checks if the resolved CNAME matches any allowed canonical host.
func (v *DNSVerifier) isCanonicalTarget(cname string) bool {
	cname = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(cname), "."))
	if cname == "" {
		return false
	}

	for _, target := range v.canonicalHosts {
		if cname == target || strings.HasSuffix(cname, "."+target) {
			return true
		}
	}
	return false
}

// isTargetIP checks if the resolved IP matches any configured target IP.
func (v *DNSVerifier) isTargetIP(ipStr string) bool {
	ipStr = strings.TrimSpace(ipStr)
	for _, target := range v.targetIPs {
		if ipStr == target {
			return true
		}
	}
	return false
}

// VerifyDomain performs multi-tiered DNS verification for a custom domain:
// 1. CNAME Resolution: Checks if the domain points to one of the canonical Cuan hostnames.
// 2. TXT Resolution: Checks `_cuan-challenge.<domain>` (or root `<domain>`) for the verification token.
// 3. Fallback A-Record Resolution: Checks if the domain resolves to Cuan server IPs (for CNAME flattening on root domains).
func (v *DNSVerifier) VerifyDomain(ctx context.Context, domainName, verificationToken string) (*VerificationResult, error) {
	checkedAt := time.Now().UTC()
	normalizedDomain := cuanDB.NormalizeDomain(domainName)

	if err := cuanDB.ValidateDomain(normalizedDomain); err != nil {
		return &VerificationResult{
			Verified:  false,
			Domain:    domainName,
			Message:   "invalid domain name format: " + err.Error(),
			Error:     err.Error(),
			CheckedAt: checkedAt,
		}, nil
	}

	res := &VerificationResult{
		Domain:    normalizedDomain,
		CheckedAt: checkedAt,
	}

	queryCtx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	// -------------------------------------------------------------------------
	// 1. CNAME Resolution Check
	// -------------------------------------------------------------------------
	cname, cnameErr := v.resolver.LookupCNAME(queryCtx, normalizedDomain)
	if cnameErr == nil {
		cleanCNAME := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(cname), "."))
		res.CNAMETarget = cleanCNAME

		// Note: net.Resolver returns the query domain itself if no CNAME record exists.
		if cleanCNAME != normalizedDomain && v.isCanonicalTarget(cleanCNAME) {
			res.Verified = true
			res.Method = "cname"
			res.MatchedTarget = cleanCNAME
			res.Message = fmt.Sprintf("CNAME points correctly to canonical host %s", cleanCNAME)
			return res, nil
		}
	}

	// -------------------------------------------------------------------------
	// 2. TXT Challenge Resolution Check
	// -------------------------------------------------------------------------
	token := strings.TrimSpace(verificationToken)
	if token != "" {
		// Look up dedicated challenge subdomain: _cuan-challenge.<domain>
		challengeHost := v.challengePrefix + normalizedDomain
		txtRecords, txtErr := v.resolver.LookupTXT(queryCtx, challengeHost)
		if txtErr == nil && len(txtRecords) > 0 {
			res.TXTRecords = append(res.TXTRecords, txtRecords...)
			for _, rec := range txtRecords {
				cleaned := strings.Trim(strings.TrimSpace(rec), `"`)
				if cleaned == token || strings.Contains(cleaned, token) {
					res.Verified = true
					res.Method = "txt"
					res.MatchedTarget = cleaned
					res.Message = fmt.Sprintf("TXT record at %s verified successfully", challengeHost)
					return res, nil
				}
			}
		}

		// Also check root domain TXT records for fallback verification (e.g. cuan-verification=<token>)
		rootTxtRecords, rootTxtErr := v.resolver.LookupTXT(queryCtx, normalizedDomain)
		if rootTxtErr == nil && len(rootTxtRecords) > 0 {
			res.TXTRecords = append(res.TXTRecords, rootTxtRecords...)
			for _, rec := range rootTxtRecords {
				cleaned := strings.Trim(strings.TrimSpace(rec), `"`)
				if cleaned == token || strings.Contains(cleaned, token) || strings.Contains(cleaned, "cuan-verification="+token) {
					res.Verified = true
					res.Method = "txt"
					res.MatchedTarget = cleaned
					res.Message = fmt.Sprintf("TXT record at %s verified successfully", normalizedDomain)
					return res, nil
				}
			}
		}
	}

	// -------------------------------------------------------------------------
	// 3. Fallback A-Record Resolution Check (CNAME Flattening)
	// -------------------------------------------------------------------------
	ips, ipErr := v.resolver.LookupIP(queryCtx, "ip", normalizedDomain)
	if ipErr == nil && len(ips) > 0 {
		var resolvedIPStrings []string
		for _, ip := range ips {
			ipStr := ip.String()
			resolvedIPStrings = append(resolvedIPStrings, ipStr)
			if v.isTargetIP(ipStr) {
				res.IPAddresses = resolvedIPStrings
				res.Verified = true
				res.Method = "a_record"
				res.MatchedTarget = ipStr
				res.Message = fmt.Sprintf("A record points correctly to Cuan platform IP %s (CNAME flattening fallback)", ipStr)
				return res, nil
			}
		}
		res.IPAddresses = resolvedIPStrings
	}

	// -------------------------------------------------------------------------
	// Failed: Neither CNAME, TXT challenge, nor A-record matched
	// -------------------------------------------------------------------------
	res.Verified = false

	var failureReasons []string
	if res.CNAMETarget != "" && res.CNAMETarget != normalizedDomain {
		failureReasons = append(failureReasons, fmt.Sprintf("CNAME target '%s' is not an authorized canonical host", res.CNAMETarget))
	} else if cnameErr != nil {
		failureReasons = append(failureReasons, "no CNAME record found")
	} else {
		failureReasons = append(failureReasons, "domain resolves to itself without CNAME")
	}

	if len(res.TXTRecords) > 0 {
		failureReasons = append(failureReasons, "TXT records found but verification token did not match")
	} else {
		failureReasons = append(failureReasons, fmt.Sprintf("no TXT record found at %s%s", v.challengePrefix, normalizedDomain))
	}

	if len(res.IPAddresses) > 0 {
		failureReasons = append(failureReasons, fmt.Sprintf("A records %v do not match target platform IPs", res.IPAddresses))
	} else if ipErr != nil {
		failureReasons = append(failureReasons, "no A/AAAA records found")
	}

	res.Message = fmt.Sprintf("DNS verification pending or failed: %s", strings.Join(failureReasons, "; "))
	return res, nil
}
