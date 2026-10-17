package db

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestProjectIntegration_ValidationAndNormalization(t *testing.T) {
	if NormalizeIntegrationType("  WordPress  ") != "wordpress" {
		t.Errorf("expected '  WordPress  ' to normalize to 'wordpress', got %s", NormalizeIntegrationType("  WordPress  "))
	}

	status, err := NormalizeIntegrationStatus("")
	if err != nil || status != "active" {
		t.Errorf("expected default active status, got %s, err: %v", status, err)
	}

	status, err = NormalizeIntegrationStatus("INACTIVE")
	if err != nil || status != "inactive" {
		t.Errorf("expected inactive status, got %s, err: %v", status, err)
	}

	_, err = NormalizeIntegrationStatus("invalid_status")
	if !errors.Is(err, ErrInvalidIntegrationStatus) {
		t.Errorf("expected ErrInvalidIntegrationStatus, got %v", err)
	}

	in := &ProjectIntegration{
		ID:              "integ-wp-1",
		ProjectID:       "proj-001",
		IntegrationType: "WordPress",
		ConfigEncrypted: "aes-256-gcm-encrypted-app-password-secret",
		Status:          "active",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	dto := in.ToDTO()
	if dto.ID != in.ID {
		t.Errorf("expected ID %s, got %s", in.ID, dto.ID)
	}
	if dto.IntegrationType != "wordpress" {
		t.Errorf("expected normalized type 'wordpress', got %s", dto.IntegrationType)
	}
	if !dto.HasConfig {
		t.Errorf("expected HasConfig true")
	}

	// Verify JSON serialization omits ConfigEncrypted
	bytes, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("failed to marshal integration: %v", err)
	}
	var rawMap map[string]interface{}
	if err := json.Unmarshal(bytes, &rawMap); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if _, exists := rawMap["config_encrypted"]; exists {
		t.Errorf("config_encrypted must not be exposed in JSON output")
	}
}

func TestMemoryProjectIntegrationRepository_CRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryProjectIntegrationRepository()

	// 1. Validation errors
	if err := repo.CreateIntegration(ctx, nil); err == nil {
		t.Errorf("expected error on nil integration")
	}
	if err := repo.CreateIntegration(ctx, &ProjectIntegration{ProjectID: "", IntegrationType: "wordpress"}); !errors.Is(err, ErrEmptyIntegrationProjectID) {
		t.Errorf("expected ErrEmptyIntegrationProjectID, got %v", err)
	}
	if err := repo.CreateIntegration(ctx, &ProjectIntegration{ProjectID: "p1", IntegrationType: ""}); !errors.Is(err, ErrEmptyIntegrationType) {
		t.Errorf("expected ErrEmptyIntegrationType, got %v", err)
	}

	// 2. Successful creation
	in := &ProjectIntegration{
		ID:              "integ-wp-test",
		ProjectID:       "proj-alpha",
		IntegrationType: "WordPress",
		ConfigEncrypted: "encrypted-payload-sample",
		Status:          "active",
	}
	if err := repo.CreateIntegration(ctx, in); err != nil {
		t.Fatalf("failed to create integration: %v", err)
	}

	// 3. Duplicate prevention for same project + integration type
	dup := &ProjectIntegration{
		ProjectID:       "proj-alpha",
		IntegrationType: "wordpress",
	}
	if err := repo.CreateIntegration(ctx, dup); !errors.Is(err, ErrIntegrationAlreadyExists) {
		t.Errorf("expected ErrIntegrationAlreadyExists, got %v", err)
	}

	// 4. GetByID
	fetched, err := repo.GetIntegrationByID(ctx, "integ-wp-test")
	if err != nil {
		t.Fatalf("failed to get integration by id: %v", err)
	}
	if fetched.IntegrationType != "wordpress" {
		t.Errorf("expected normalized type wordpress, got %s", fetched.IntegrationType)
	}

	// 5. GetByProjectAndType
	byType, err := repo.GetIntegrationByProjectAndType(ctx, "proj-alpha", "WORDPRESS")
	if err != nil {
		t.Fatalf("failed to get integration by project and type: %v", err)
	}
	if byType.ID != "integ-wp-test" {
		t.Errorf("expected id integ-wp-test, got %s", byType.ID)
	}

	// 6. ListIntegrationsByProjectID
	list, err := repo.ListIntegrationsByProjectID(ctx, "proj-alpha")
	if err != nil {
		t.Fatalf("failed to list integrations: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 integration, got %d", len(list))
	}

	// 7. Update
	fetched.Status = "inactive"
	fetched.ConfigEncrypted = "updated-encrypted-config"
	if err := repo.UpdateIntegration(ctx, fetched); err != nil {
		t.Fatalf("failed to update integration: %v", err)
	}

	updated, err := repo.GetIntegrationByID(ctx, "integ-wp-test")
	if err != nil {
		t.Fatalf("failed to get updated integration: %v", err)
	}
	if updated.Status != "inactive" {
		t.Errorf("expected status inactive, got %s", updated.Status)
	}

	// 8. Delete
	if err := repo.DeleteIntegration(ctx, "integ-wp-test"); err != nil {
		t.Fatalf("failed to delete integration: %v", err)
	}
	if _, err := repo.GetIntegrationByID(ctx, "integ-wp-test"); !errors.Is(err, ErrIntegrationNotFound) {
		t.Errorf("expected ErrIntegrationNotFound, got %v", err)
	}
}

func TestProjectIntegration_AES256GCMApplicationPassword(t *testing.T) {
	password := "user_app_pass_abcd_efgh_1234_5678"
	secret := "secret-encryption-key-for-integrations-32b"

	encrypted, err := EncryptApplicationPassword(password, secret)
	if err != nil {
		t.Fatalf("EncryptApplicationPassword failed: %v", err)
	}
	if encrypted == password {
		t.Fatalf("ciphertext must not match plaintext password")
	}

	decrypted, err := DecryptApplicationPassword(encrypted, secret)
	if err != nil {
		t.Fatalf("DecryptApplicationPassword failed: %v", err)
	}
	if decrypted != password {
		t.Errorf("decrypted password mismatch: got %q, want %q", decrypted, password)
	}
}

func TestRepositories_ProjectIntegrationIntegration(t *testing.T) {
	memRepos := NewMemoryRepositories()
	if memRepos.ProjectIntegrations == nil {
		t.Errorf("expected ProjectIntegrations repository to be non-nil in MemoryRepositories")
	}

	sqlRepos := NewSQLRepositories(nil)
	if sqlRepos.ProjectIntegrations == nil {
		t.Errorf("expected ProjectIntegrations repository to be non-nil in SQLRepositories")
	}
}
