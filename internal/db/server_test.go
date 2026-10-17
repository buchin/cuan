package db

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestServer_JSONSerializationSanitization(t *testing.T) {
	now := time.Now().UTC()
	srv := Server{
		ID:             "srv_test_12345",
		Name:           "Production Hetzner VPS",
		Host:           "192.168.1.100",
		Port:           22,
		Username:       "ubuntu",
		AuthType:       "key",
		EncryptedKey:   "ENC:SUPER_SECRET_CIPHERTEXT_BASE64==",
		KeyFingerprint: "SHA256:47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU",
		Status:         "online",
		LastPingAt:     &now,
		OSInfo: map[string]interface{}{
			"os":     "Ubuntu 24.04 LTS",
			"cores":  4,
			"ram_mb": 8192,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 1. Verify JSON serialization of Server struct itself
	serverBytes, err := json.Marshal(srv)
	if err != nil {
		t.Fatalf("json.Marshal(srv) failed: %v", err)
	}
	serverJSON := string(serverBytes)

	if strings.Contains(serverJSON, "SUPER_SECRET_CIPHERTEXT_BASE64") {
		t.Fatalf("CRITICAL SECURITY FLAW: Server JSON serialization exposed EncryptedKey!")
	}
	if strings.Contains(serverJSON, "encrypted_key") {
		t.Fatalf("CRITICAL SECURITY FLAW: Server JSON serialization contains encrypted_key field key!")
	}

	// Verify other public fields are preserved
	if !strings.Contains(serverJSON, "Production Hetzner VPS") {
		t.Errorf("Server JSON missing name")
	}
	if !strings.Contains(serverJSON, "192.168.1.100") {
		t.Errorf("Server JSON missing host")
	}
	if !strings.Contains(serverJSON, "SHA256:47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU") {
		t.Errorf("Server JSON missing key_fingerprint")
	}

	// 2. Verify ServerDTO serialization
	dto := srv.ToDTO()
	dtoBytes, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("json.Marshal(dto) failed: %v", err)
	}
	dtoJSON := string(dtoBytes)

	if strings.Contains(dtoJSON, "SUPER_SECRET_CIPHERTEXT_BASE64") || strings.Contains(dtoJSON, "encrypted_key") {
		t.Fatalf("CRITICAL SECURITY FLAW: ServerDTO JSON serialization exposed credentials!")
	}
	if !strings.Contains(dtoJSON, "Production Hetzner VPS") {
		t.Errorf("DTO JSON missing name")
	}
}

func TestMemoryServerRepository_CRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryServerRepository()

	now := time.Now().UTC()
	srv := &Server{
		ID:             "srv_test_crud_1",
		Name:           "Node Alpha",
		Host:           "10.0.0.1",
		Port:           22,
		Username:       "ubuntu",
		AuthType:       "key",
		EncryptedKey:   "vault-ciphertext-abc",
		KeyFingerprint: "SHA256:testfingerprint",
		Status:         "pending",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// Create
	if err := repo.Create(ctx, srv); err != nil {
		t.Fatalf("repo.Create failed: %v", err)
	}

	// Duplicate create should error
	if err := repo.Create(ctx, srv); err == nil {
		t.Fatalf("expected error on duplicate create, got nil")
	}

	// GetByID
	fetched, err := repo.GetByID(ctx, "srv_test_crud_1")
	if err != nil {
		t.Fatalf("repo.GetByID failed: %v", err)
	}
	if fetched.Name != "Node Alpha" || fetched.EncryptedKey != "vault-ciphertext-abc" {
		t.Errorf("fetched server mismatch: %+v", fetched)
	}

	// List
	list, err := repo.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("repo.List failed: %v, len=%d", err, len(list))
	}

	// Update
	fetched.Name = "Node Alpha Renamed"
	if err := repo.Update(ctx, fetched); err != nil {
		t.Fatalf("repo.Update failed: %v", err)
	}
	updated, _ := repo.GetByID(ctx, "srv_test_crud_1")
	if updated.Name != "Node Alpha Renamed" {
		t.Errorf("expected updated name, got %s", updated.Name)
	}

	// UpdateStatus
	pingTime := time.Now().UTC()
	osInfo := map[string]interface{}{"distro": "Debian 12"}
	if err := repo.UpdateStatus(ctx, "srv_test_crud_1", "online", &pingTime, osInfo); err != nil {
		t.Fatalf("repo.UpdateStatus failed: %v", err)
	}
	statusUpdated, _ := repo.GetByID(ctx, "srv_test_crud_1")
	if statusUpdated.Status != "online" || statusUpdated.LastPingAt == nil || statusUpdated.OSInfo["distro"] != "Debian 12" {
		t.Errorf("status update failed: %+v", statusUpdated)
	}

	// Delete
	if err := repo.Delete(ctx, "srv_test_crud_1"); err != nil {
		t.Fatalf("repo.Delete failed: %v", err)
	}
	_, err = repo.GetByID(ctx, "srv_test_crud_1")
	if err != ErrServerNotFound {
		t.Errorf("expected ErrServerNotFound after delete, got %v", err)
	}
}
