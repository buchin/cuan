package db

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestValidateDomain(t *testing.T) {
	validDomains := []string{
		"example.com",
		"sub.example.com",
		"deep.sub.domain.co.id",
		"my-app-123.org",
		"api.v2.saas.io",
		"startup-ku.tech",
	}

	for _, domain := range validDomains {
		if err := ValidateDomain(domain); err != nil {
			t.Errorf("expected domain '%s' to be valid, got error: %v", domain, err)
		}
	}

	invalidDomains := []struct {
		domain string
		reason string
	}{
		{"", "empty domain"},
		{"   ", "whitespace only"},
		{"http://example.com", "contains http scheme"},
		{"https://sub.domain.com", "contains https scheme"},
		{"example.com/path", "contains path"},
		{"example.com:8080", "contains port"},
		{"user@example.com", "contains user@"},
		{"invalid domain.com", "contains space"},
		{"singleword", "missing TLD"},
		{"-leadinghyphen.com", "leading hyphen in label"},
		{"trailinghyphen-.com", "trailing hyphen in label"},
		{"example..com", "empty label between dots"},
		{strings.Repeat("a", 64) + ".com", "label exceeds 63 characters"},
		{strings.Repeat("a.", 130) + "com", "domain exceeds 253 characters"},
		{"example.123", "purely numeric TLD"},
	}

	for _, tc := range invalidDomains {
		if err := ValidateDomain(tc.domain); err == nil {
			t.Errorf("expected domain '%s' (%s) to fail validation, but got nil", tc.domain, tc.reason)
		} else if !errors.Is(err, ErrInvalidDomainName) {
			t.Errorf("expected ErrInvalidDomainName for '%s', got: %v", tc.domain, err)
		}
	}
}

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  Example.COM  ", "example.com"},
		{"SUB.Domain.Org  ", "sub.domain.org"},
		{"test.io", "test.io"},
	}

	for _, tc := range tests {
		actual := NormalizeDomain(tc.input)
		if actual != tc.expected {
			t.Errorf("NormalizeDomain(%q) = %q, expected %q", tc.input, actual, tc.expected)
		}
	}
}

func TestCustomDomain_DTOAndSanitization(t *testing.T) {
	now := time.Now().UTC()
	token := "abcdef1234567890abcdef1234567890"
	cd := CustomDomain{
		ID:                "dom_test_001",
		DeploymentID:      "dep_app_123",
		Domain:            "app.client.com",
		DNSStatus:         "pending",
		SSLStatus:         "pending",
		VerificationToken: token,
		LastCheckedAt:     &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	// 1. Test standard DTO
	dto := cd.ToDTO()
	if dto.ID != cd.ID || dto.Domain != cd.Domain || dto.VerificationToken != token {
		t.Errorf("ToDTO mismatch: %+v", dto)
	}

	// 2. Test sanitized DTO
	sanitized := cd.ToSanitizedDTO()
	if sanitized.ID != cd.ID || sanitized.Domain != cd.Domain {
		t.Errorf("sanitized DTO ID or Domain mismatch: %+v", sanitized)
	}
	expectedMasked := "abcd...7890"
	if sanitized.VerificationToken != expectedMasked {
		t.Errorf("expected masked token %s, got %s", expectedMasked, sanitized.VerificationToken)
	}

	// 3. JSON serialization roundtrip
	dtoBytes, err := json.Marshal(sanitized)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	dtoJSON := string(dtoBytes)
	if strings.Contains(dtoJSON, token) {
		t.Errorf("sanitized DTO JSON leaked full verification token!")
	}
	if !strings.Contains(dtoJSON, expectedMasked) {
		t.Errorf("sanitized DTO JSON missing masked token")
	}
}

func TestMemoryDomainRepository_CRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryDomainRepository()

	// 1. Validation failure: empty deployment_id
	err := repo.CreateCustomDomain(ctx, &CustomDomain{
		Domain: "valid.com",
	})
	if !errors.Is(err, ErrEmptyDeploymentID) {
		t.Errorf("expected ErrEmptyDeploymentID, got %v", err)
	}

	// 2. Validation failure: invalid domain
	err = repo.CreateCustomDomain(ctx, &CustomDomain{
		DeploymentID: "dep_1",
		Domain:       "http://invalid.com",
	})
	if !errors.Is(err, ErrInvalidDomainName) {
		t.Errorf("expected ErrInvalidDomainName, got %v", err)
	}

	// 3. Create Custom Domain 1
	now := time.Now().UTC()
	cd1 := &CustomDomain{
		ID:           "dom_001",
		DeploymentID: "dep_1",
		Domain:       "App.StartupKu.ID", // uppercase to test normalization
		CreatedAt:    now,
	}
	if err := repo.CreateCustomDomain(ctx, cd1); err != nil {
		t.Fatalf("CreateCustomDomain failed: %v", err)
	}

	// Domain must be normalized to lowercase
	if cd1.Domain != "app.startupku.id" {
		t.Errorf("expected normalized domain 'app.startupku.id', got '%s'", cd1.Domain)
	}
	if cd1.DNSStatus != "pending" || cd1.SSLStatus != "pending" {
		t.Errorf("expected default pending status, got dns=%s, ssl=%s", cd1.DNSStatus, cd1.SSLStatus)
	}
	if cd1.VerificationToken == "" {
		t.Errorf("expected auto-generated verification token")
	}

	// 4. Duplicate Domain Unique Constraint Check
	duplicateDomain := &CustomDomain{
		ID:           "dom_002",
		DeploymentID: "dep_2",
		Domain:       "app.startupku.id", // duplicate
	}
	err = repo.CreateCustomDomain(ctx, duplicateDomain)
	if !errors.Is(err, ErrDuplicateDomain) {
		t.Errorf("expected ErrDuplicateDomain, got %v", err)
	}

	// Duplicate Domain with different case
	duplicateCase := &CustomDomain{
		ID:           "dom_003",
		DeploymentID: "dep_3",
		Domain:       "APP.STARTUPKU.ID",
	}
	err = repo.CreateCustomDomain(ctx, duplicateCase)
	if !errors.Is(err, ErrDuplicateDomain) {
		t.Errorf("expected ErrDuplicateDomain for uppercase variant, got %v", err)
	}

	// 5. GetCustomDomainByID
	fetched, err := repo.GetCustomDomainByID(ctx, "dom_001")
	if err != nil {
		t.Fatalf("GetCustomDomainByID failed: %v", err)
	}
	if fetched.Domain != "app.startupku.id" || fetched.DeploymentID != "dep_1" {
		t.Errorf("fetched domain mismatch: %+v", fetched)
	}

	// Get non-existent ID
	_, err = repo.GetCustomDomainByID(ctx, "dom_non_existent")
	if !errors.Is(err, ErrDomainNotFound) {
		t.Errorf("expected ErrDomainNotFound, got %v", err)
	}

	// 6. GetCustomDomainByDomain (case-insensitive test)
	fetchedByDomain, err := repo.GetCustomDomainByDomain(ctx, "APP.startupku.id")
	if err != nil {
		t.Fatalf("GetCustomDomainByDomain failed: %v", err)
	}
	if fetchedByDomain.ID != "dom_001" {
		t.Errorf("expected dom_001, got %s", fetchedByDomain.ID)
	}

	// Get non-existent domain
	_, err = repo.GetCustomDomainByDomain(ctx, "unknown.org")
	if !errors.Is(err, ErrDomainNotFound) {
		t.Errorf("expected ErrDomainNotFound, got %v", err)
	}

	// 7. Add another domain to same deployment
	cd2 := &CustomDomain{
		DeploymentID: "dep_1",
		Domain:       "api.startupku.id",
	}
	if err := repo.CreateCustomDomain(ctx, cd2); err != nil {
		t.Fatalf("CreateCustomDomain cd2 failed: %v", err)
	}
	if !strings.HasPrefix(cd2.ID, "dom_") {
		t.Errorf("expected auto-generated ID starting with 'dom_', got %s", cd2.ID)
	}

	// 8. Add domain to different deployment
	cd3 := &CustomDomain{
		DeploymentID: "dep_2",
		Domain:       "client.otherapp.com",
	}
	if err := repo.CreateCustomDomain(ctx, cd3); err != nil {
		t.Fatalf("CreateCustomDomain cd3 failed: %v", err)
	}

	// 9. ListCustomDomainsByDeploymentID
	dep1List, err := repo.ListCustomDomainsByDeploymentID(ctx, "dep_1")
	if err != nil {
		t.Fatalf("ListCustomDomainsByDeploymentID failed: %v", err)
	}
	if len(dep1List) != 2 {
		t.Errorf("expected 2 domains for dep_1, got %d", len(dep1List))
	}

	dep2List, err := repo.ListCustomDomainsByDeploymentID(ctx, "dep_2")
	if err != nil {
		t.Fatalf("ListCustomDomainsByDeploymentID failed: %v", err)
	}
	if len(dep2List) != 1 {
		t.Errorf("expected 1 domain for dep_2, got %d", len(dep2List))
	}

	// 10. ListAllCustomDomains
	allDomains, err := repo.ListAllCustomDomains(ctx)
	if err != nil {
		t.Fatalf("ListAllCustomDomains failed: %v", err)
	}
	if len(allDomains) != 3 {
		t.Errorf("expected 3 domains total, got %d", len(allDomains))
	}

	// 11. UpdateCustomDomainStatus
	checkedAt := time.Now().UTC()
	err = repo.UpdateCustomDomainStatus(ctx, "dom_001", "verified", "active", &checkedAt)
	if err != nil {
		t.Fatalf("UpdateCustomDomainStatus failed: %v", err)
	}

	updated, err := repo.GetCustomDomainByID(ctx, "dom_001")
	if err != nil {
		t.Fatalf("GetCustomDomainByID failed: %v", err)
	}
	if updated.DNSStatus != "verified" || updated.SSLStatus != "active" {
		t.Errorf("expected verified/active, got dns=%s, ssl=%s", updated.DNSStatus, updated.SSLStatus)
	}
	if updated.LastCheckedAt == nil || !updated.LastCheckedAt.Equal(checkedAt) {
		t.Errorf("expected LastCheckedAt=%v, got %v", checkedAt, updated.LastCheckedAt)
	}

	// Update non-existent domain status
	err = repo.UpdateCustomDomainStatus(ctx, "dom_missing", "verified", "active", &checkedAt)
	if !errors.Is(err, ErrDomainNotFound) {
		t.Errorf("expected ErrDomainNotFound, got %v", err)
	}

	// 12. DeleteCustomDomain
	err = repo.DeleteCustomDomain(ctx, "dom_001")
	if err != nil {
		t.Fatalf("DeleteCustomDomain failed: %v", err)
	}

	// Verify it's gone
	_, err = repo.GetCustomDomainByID(ctx, "dom_001")
	if !errors.Is(err, ErrDomainNotFound) {
		t.Errorf("expected ErrDomainNotFound after delete, got %v", err)
	}
	_, err = repo.GetCustomDomainByDomain(ctx, "app.startupku.id")
	if !errors.Is(err, ErrDomainNotFound) {
		t.Errorf("expected ErrDomainNotFound after delete by domain, got %v", err)
	}

	// Delete non-existent
	err = repo.DeleteCustomDomain(ctx, "dom_001")
	if !errors.Is(err, ErrDomainNotFound) {
		t.Errorf("expected ErrDomainNotFound on deleting already deleted domain, got %v", err)
	}

	// Check remaining count
	remaining, _ := repo.ListAllCustomDomains(ctx)
	if len(remaining) != 2 {
		t.Errorf("expected 2 remaining domains, got %d", len(remaining))
	}
}
