package server

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
)

func TestServerDTOSanitization_NeverExposesPrivateKey(t *testing.T) {
	now := time.Now().UTC()
	srv := db.Server{
		ID:             "srv_prod_remote_01",
		Name:           "Main BYOS VPS",
		Host:           "203.0.113.50",
		Port:           22,
		Username:       "ubuntu",
		AuthType:       "key",
		EncryptedKey:   "SECRET_VAULT_ENCRYPTED_KEY_AT_REST_BASE64",
		KeyFingerprint: "SHA256:dGhpcy1pcy1hLXRlc3QtZmluZ2VycHJpbnQ=",
		Status:         "online",
		LastPingAt:     &now,
		OSInfo: map[string]interface{}{
			"os":     "Ubuntu 22.04 LTS",
			"cores":  8,
			"ram_mb": 16384,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 1. Check direct serialization of db.Server (used in internal models)
	serverJSONBytes, err := json.Marshal(srv)
	if err != nil {
		t.Fatalf("json.Marshal(srv) failed: %v", err)
	}
	serverJSON := string(serverJSONBytes)

	if strings.Contains(serverJSON, "SECRET_VAULT_ENCRYPTED_KEY_AT_REST_BASE64") {
		t.Fatalf("CRITICAL SECURITY LEAK: EncryptedKey serialized into JSON output: %s", serverJSON)
	}
	if strings.Contains(serverJSON, "encrypted_key") {
		t.Fatalf("CRITICAL SECURITY LEAK: encrypted_key key present in JSON output: %s", serverJSON)
	}

	// 2. Check DTO serialization (used in API responses)
	dto := srv.ToDTO()
	dtoJSONBytes, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("json.Marshal(dto) failed: %v", err)
	}
	dtoJSON := string(dtoJSONBytes)

	if strings.Contains(dtoJSON, "SECRET_VAULT_ENCRYPTED_KEY_AT_REST_BASE64") {
		t.Fatalf("CRITICAL SECURITY LEAK: EncryptedKey serialized into DTO JSON output: %s", dtoJSON)
	}
	if strings.Contains(dtoJSON, "encrypted_key") {
		t.Fatalf("CRITICAL SECURITY LEAK: encrypted_key key present in DTO JSON output: %s", dtoJSON)
	}

	// Verify required public fields are present
	expectedFields := []string{"id", "name", "host", "port", "username", "auth_type", "key_fingerprint", "status"}
	for _, field := range expectedFields {
		if !strings.Contains(dtoJSON, `"`+field+`"`) {
			t.Errorf("expected DTO JSON to include field %q, got: %s", field, dtoJSON)
		}
	}
}
