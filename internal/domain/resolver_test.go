package domain

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestDNSVerifier_CNAMEMatch(t *testing.T) {
	mock := NewMockDNSResolver()
	mock.SetCNAME("app.clientbrand.com", "cuan.app.")

	verifier := NewDNSVerifier(DNSVerifierConfig{
		CanonicalHosts: []string{"cuan.app", "cuan.dojo.cc"},
	}, WithResolver(mock))

	res, err := verifier.VerifyDomain(context.Background(), "app.clientbrand.com", "tok_test123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !res.Verified {
		t.Fatalf("expected domain to be verified via CNAME, got: %s", res.Message)
	}
	if res.Method != "cname" {
		t.Fatalf("expected method cname, got %s", res.Method)
	}
	if res.MatchedTarget != "cuan.app" {
		t.Fatalf("expected matched target cuan.app, got %s", res.MatchedTarget)
	}
}

func TestDNSVerifier_CNAMEMatch_SubdomainBaseDomain(t *testing.T) {
	mock := NewMockDNSResolver()
	mock.SetCNAME("portal.startup.id", "cuan.dojo.cc.")

	verifier := NewDNSVerifier(DNSVerifierConfig{
		CanonicalHosts: []string{"cuan.dojo.cc"},
	}, WithResolver(mock))

	res, err := verifier.VerifyDomain(context.Background(), "portal.startup.id", "tok_test123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !res.Verified || res.Method != "cname" {
		t.Fatalf("expected verified via cname, got: %+v", res)
	}
}

func TestDNSVerifier_TXTMatch(t *testing.T) {
	mock := NewMockDNSResolver()
	// No CNAME configured
	token := "4a8e27c19f563d7e8b0124a9c6f2e8d1"
	mock.SetTXT("_cuan-challenge.mybrand.io", token)

	verifier := NewDNSVerifier(DNSVerifierConfig{}, WithResolver(mock))

	res, err := verifier.VerifyDomain(context.Background(), "mybrand.io", token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !res.Verified {
		t.Fatalf("expected domain to be verified via TXT challenge, got: %s", res.Message)
	}
	if res.Method != "txt" {
		t.Fatalf("expected method txt, got %s", res.Method)
	}
	if res.MatchedTarget != token {
		t.Fatalf("expected matched token %s, got %s", token, res.MatchedTarget)
	}
}

func TestDNSVerifier_TXTMatch_QuotedRecord(t *testing.T) {
	mock := NewMockDNSResolver()
	token := "secret_token_123"
	mock.SetTXT("_cuan-challenge.quoted.com", `"`+token+`"`)

	verifier := NewDNSVerifier(DNSVerifierConfig{}, WithResolver(mock))

	res, err := verifier.VerifyDomain(context.Background(), "quoted.com", token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !res.Verified || res.Method != "txt" {
		t.Fatalf("expected verified via txt, got: %+v", res)
	}
}

func TestDNSVerifier_TXTMatch_RootDomainFallback(t *testing.T) {
	mock := NewMockDNSResolver()
	token := "fallback_token_999"
	// Challenge subdomain returns not found, but root domain has cuan-verification=...
	mock.SetTXT("rootdomain.com", "cuan-verification="+token)

	verifier := NewDNSVerifier(DNSVerifierConfig{}, WithResolver(mock))

	res, err := verifier.VerifyDomain(context.Background(), "rootdomain.com", token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !res.Verified || res.Method != "txt" {
		t.Fatalf("expected verified via root TXT fallback, got: %+v", res)
	}
}

func TestDNSVerifier_ARecordFallback_CNAMEFlattening(t *testing.T) {
	mock := NewMockDNSResolver()
	// Apex domain: Cloudflare CNAME flattening replaces CNAME with A records pointing to platform IP
	mock.SetIP("apexbrand.com", net.ParseIP("127.0.0.1"))

	verifier := NewDNSVerifier(DNSVerifierConfig{
		TargetIPs: []string{"127.0.0.1", "103.150.190.5"},
	}, WithResolver(mock))

	res, err := verifier.VerifyDomain(context.Background(), "apexbrand.com", "tok_unused")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !res.Verified {
		t.Fatalf("expected domain to be verified via A record fallback, got: %s", res.Message)
	}
	if res.Method != "a_record" {
		t.Fatalf("expected method a_record, got %s", res.Method)
	}
	if res.MatchedTarget != "127.0.0.1" {
		t.Fatalf("expected matched target 127.0.0.1, got %s", res.MatchedTarget)
	}
}

func TestDNSVerifier_TokenMismatch(t *testing.T) {
	mock := NewMockDNSResolver()
	mock.SetTXT("_cuan-challenge.wrongtoken.com", "token_abc_123")

	verifier := NewDNSVerifier(DNSVerifierConfig{}, WithResolver(mock))

	res, err := verifier.VerifyDomain(context.Background(), "wrongtoken.com", "expected_token_xyz_789")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Verified {
		t.Fatalf("expected domain verification to fail on token mismatch, but verified was true")
	}
}

func TestDNSVerifier_NXDOMAIN_And_NoRecords(t *testing.T) {
	mock := NewMockDNSResolver()
	// Mock has no records configured -> returns NXDOMAIN

	verifier := NewDNSVerifier(DNSVerifierConfig{}, WithResolver(mock))

	res, err := verifier.VerifyDomain(context.Background(), "nonexistent.example.org", "tok_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Verified {
		t.Fatalf("expected verification to fail for nonexistent domain")
	}
}

func TestDNSVerifier_TimeoutOrNetworkError(t *testing.T) {
	mock := NewMockDNSResolver()
	mock.SetError("timeout.example.org", errors.New("i/o timeout"))
	mock.SetError("_cuan-challenge.timeout.example.org", errors.New("i/o timeout"))

	verifier := NewDNSVerifier(DNSVerifierConfig{}, WithResolver(mock))

	res, err := verifier.VerifyDomain(context.Background(), "timeout.example.org", "tok_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Verified {
		t.Fatalf("expected verification to fail when DNS timed out")
	}
}

func TestDNSVerifier_InvalidDomainFormat(t *testing.T) {
	verifier := NewDNSVerifier(DNSVerifierConfig{})

	invalidDomains := []string{
		"",
		"   ",
		"http://invalid.com",
		"invalid/path",
		"-leadinghyphen.com",
		"singleword",
		"domain:8080",
	}

	for _, d := range invalidDomains {
		res, err := verifier.VerifyDomain(context.Background(), d, "tok_123")
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", d, err)
		}
		if res.Verified {
			t.Fatalf("expected domain %q to fail validation", d)
		}
	}
}

func TestDNSVerifier_OptionsAndAccessors(t *testing.T) {
	mock := NewMockDNSResolver()
	verifier := NewDNSVerifier(DNSVerifierConfig{
		Timeout:         10 * time.Second,
		ChallengePrefix: "_cuan-challenge.",
	},
		WithCanonicalHosts("custom.host.io"),
		WithTargetIPs("192.168.1.1"),
		WithResolver(mock),
		WithTimeout(3*time.Second),
	)

	if verifier.Resolver() != mock {
		t.Fatalf("expected mock resolver")
	}
	if len(verifier.CanonicalHosts()) == 0 {
		t.Fatalf("expected canonical hosts")
	}
	if len(verifier.TargetIPs()) == 0 {
		t.Fatalf("expected target IPs")
	}

	netRes := NewNetDNSResolver()
	verifier.SetResolver(netRes)
	if verifier.Resolver() != netRes {
		t.Fatalf("expected net resolver after SetResolver")
	}
}
