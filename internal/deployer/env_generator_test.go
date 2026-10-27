package deployer

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
)

func TestGenerateApplicationSecrets(t *testing.T) {
	secrets, err := GenerateApplicationSecrets()
	if err != nil {
		t.Fatalf("unexpected error generating application secrets: %v", err)
	}

	// 1. Laravel APP_KEY: must start with base64: and decode to 32 bytes
	if !strings.HasPrefix(secrets.LaravelAppKey, "base64:") {
		t.Fatalf("expected APP_KEY to start with 'base64:', got: %s", secrets.LaravelAppKey)
	}
	rawB64 := strings.TrimPrefix(secrets.LaravelAppKey, "base64:")
	decodedKey, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil {
		t.Fatalf("failed to decode base64 APP_KEY: %v", err)
	}
	if len(decodedKey) != 32 {
		t.Fatalf("expected 32 raw bytes for APP_KEY, got %d", len(decodedKey))
	}

	// 2. JWT Secret: must decode from 64 hex chars to 32 bytes
	if len(secrets.JWTSecret) != 64 {
		t.Fatalf("expected 64 hex characters for JWTSecret, got %d", len(secrets.JWTSecret))
	}
	decodedJWT, err := hex.DecodeString(secrets.JWTSecret)
	if err != nil {
		t.Fatalf("failed to decode hex JWTSecret: %v", err)
	}
	if len(decodedJWT) != 32 {
		t.Fatalf("expected 32 raw bytes for JWTSecret, got %d", len(decodedJWT))
	}

	// 3. Session Salt: must decode from 32 hex chars to 16 bytes
	if len(secrets.SessionSalt) != 32 {
		t.Fatalf("expected 32 hex characters for SessionSalt, got %d", len(secrets.SessionSalt))
	}

	// 4. DB Password: must be at least 24 chars with sufficient entropy
	if len(secrets.DBPassword) < 24 {
		t.Fatalf("expected at least 24 characters for DBPassword, got %d", len(secrets.DBPassword))
	}
}

func TestAutoMapDatabaseEnv(t *testing.T) {
	tests := []struct {
		name       string
		engine     string
		dbName     string
		dbUser     string
		dbPass     string
		host       string
		port       int
		expected   map[string]string
	}{
		{
			name:   "SQLite mapping",
			engine: "sqlite",
			dbName: "/data/projects/p1/data/database.sqlite",
			dbUser: "",
			dbPass: "",
			host:   "127.0.0.1",
			port:   0,
			expected: map[string]string{
				"DB_CONNECTION": "sqlite",
				"DB_DATABASE":   "/data/projects/p1/data/database.sqlite",
				"DB_HOST":       "127.0.0.1",
				"DB_PORT":       "",
				"DB_USERNAME":   "",
				"DB_PASSWORD":   "",
			},
		},
		{
			name:   "MySQL mapping",
			engine: "mysql",
			dbName: "cuan_prod",
			dbUser: "cuan_user",
			dbPass: "Secret123!",
			host:   "mysql.internal",
			port:   3306,
			expected: map[string]string{
				"DB_CONNECTION": "mysql",
				"DB_DATABASE":   "cuan_prod",
				"DB_HOST":       "mysql.internal",
				"DB_PORT":       "3306",
				"DB_USERNAME":   "cuan_user",
				"DB_PASSWORD":   "Secret123!",
			},
		},
		{
			name:   "Postgres mapping",
			engine: "postgres",
			dbName: "cuan_pg",
			dbUser: "pg_user",
			dbPass: "SuperSecretPass",
			host:   "10.0.0.5",
			port:   5432,
			expected: map[string]string{
				"DB_CONNECTION": "pgsql",
				"DB_DATABASE":   "cuan_pg",
				"DB_HOST":       "10.0.0.5",
				"DB_PORT":       "5432",
				"DB_USERNAME":   "pg_user",
				"DB_PASSWORD":   "SuperSecretPass",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := AutoMapDatabaseEnv(tc.engine, tc.dbName, tc.dbUser, tc.dbPass, tc.host, tc.port)
			for k, v := range tc.expected {
				if res[k] != v {
					t.Errorf("key %s: expected %q, got %q", k, v, res[k])
				}
			}
		})
	}
}

func TestMergeEnv_PreservesStructureAndReplacesPlaceholders(t *testing.T) {
	template := `# Application Config
APP_NAME="Cuan App"
APP_ENV=local
APP_KEY=base64:CHANGE_ME_PLEASE
APP_DEBUG=true

# Database Settings
DB_CONNECTION=mysql
DB_HOST=127.0.0.1
DB_PORT=3306
DB_DATABASE=homestead
DB_USERNAME=homestead
DB_PASSWORD=secret
`

	secrets := map[string]string{
		"APP_KEY":    "base64:QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVowMTIzNDU=",
		"JWT_SECRET": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
	}

	dbVars := map[string]string{
		"DB_CONNECTION": "pgsql",
		"DB_HOST":       "postgres.db.local",
		"DB_PORT":       "5432",
		"DB_DATABASE":   "cuan_isolated_db",
		"DB_USERNAME":   "cuan_pg_user",
		"DB_PASSWORD":   "P@ssw0rdSecure!",
	}

	customOverrides := map[string]string{
		"APP_NAME": "My Production App",
	}

	merged := MergeEnv(template, dbVars, secrets, customOverrides)

	parsed := ParseEnvMap(merged)

	if parsed["APP_NAME"] != "My Production App" {
		t.Errorf("expected APP_NAME to be overridden, got: %s", parsed["APP_NAME"])
	}
	if parsed["APP_KEY"] != secrets["APP_KEY"] {
		t.Errorf("expected APP_KEY to be replaced, got: %s", parsed["APP_KEY"])
	}
	if parsed["DB_CONNECTION"] != "pgsql" {
		t.Errorf("expected DB_CONNECTION=pgsql, got: %s", parsed["DB_CONNECTION"])
	}
	if parsed["DB_HOST"] != "postgres.db.local" {
		t.Errorf("expected DB_HOST=postgres.db.local, got: %s", parsed["DB_HOST"])
	}
	if parsed["DB_PORT"] != "5432" {
		t.Errorf("expected DB_PORT=5432, got: %s", parsed["DB_PORT"])
	}
	if parsed["DB_DATABASE"] != "cuan_isolated_db" {
		t.Errorf("expected DB_DATABASE=cuan_isolated_db, got: %s", parsed["DB_DATABASE"])
	}
	if parsed["DB_USERNAME"] != "cuan_pg_user" {
		t.Errorf("expected DB_USERNAME=cuan_pg_user, got: %s", parsed["DB_USERNAME"])
	}
	if parsed["DB_PASSWORD"] != "P@ssw0rdSecure!" {
		t.Errorf("expected DB_PASSWORD=P@ssw0rdSecure!, got: %s", parsed["DB_PASSWORD"])
	}
	if parsed["JWT_SECRET"] != secrets["JWT_SECRET"] {
		t.Errorf("expected JWT_SECRET to be appended, got: %s", parsed["JWT_SECRET"])
	}

	// Verify comments are preserved
	if !strings.Contains(merged, "# Application Config") || !strings.Contains(merged, "# Database Settings") {
		t.Errorf("expected comments to be preserved in merged env")
	}
}

func TestMaskEnvVars_ZeroCredentialLeakage(t *testing.T) {
	env := map[string]string{
		"APP_NAME":    "Cuan Store",
		"APP_KEY":     "base64:e4b0162bcbe692f89f25d9c28892518e100e4708ff842a84a6018b32eb9e4f50",
		"JWT_SECRET":  "d1c8f1e687e834b9cfcf45800d3d5272a24bc9886a34731c62f27fa00889f41b",
		"DB_PASSWORD": "SuperSecretDatabasePassword123!",
		"DB_USERNAME": "cuan_user",
		"DB_DATABASE": "cuan_db",
		"AUTH_TOKEN":  "bearer_token_xyz",
		"CACHE_TTL":   "3600",
	}

	masked := MaskEnvVars(env)

	// Non-sensitive variables must remain unmasked
	if masked["APP_NAME"] != "Cuan Store" {
		t.Errorf("APP_NAME should not be masked")
	}
	if masked["DB_USERNAME"] != "cuan_user" {
		t.Errorf("DB_USERNAME should not be masked")
	}
	if masked["DB_DATABASE"] != "cuan_db" {
		t.Errorf("DB_DATABASE should not be masked")
	}
	if masked["CACHE_TTL"] != "3600" {
		t.Errorf("CACHE_TTL should not be masked")
	}

	// Sensitive variables MUST be masked
	if masked["APP_KEY"] != "base64:********************" {
		t.Errorf("expected masked APP_KEY, got: %s", masked["APP_KEY"])
	}
	if masked["JWT_SECRET"] != "********" {
		t.Errorf("expected masked JWT_SECRET, got: %s", masked["JWT_SECRET"])
	}
	if masked["DB_PASSWORD"] != "********" {
		t.Errorf("expected masked DB_PASSWORD, got: %s", masked["DB_PASSWORD"])
	}
	if masked["AUTH_TOKEN"] != "********" {
		t.Errorf("expected masked AUTH_TOKEN, got: %s", masked["AUTH_TOKEN"])
	}

	// Test MaskRawEnv
	raw := `
APP_NAME=MyApp
APP_KEY=base64:abc123xyz
DB_PASSWORD=my_plain_pass
`
	maskedRaw := MaskRawEnv(raw)
	if strings.Contains(maskedRaw, "my_plain_pass") {
		t.Errorf("raw env leaked plain password: %s", maskedRaw)
	}
	if strings.Contains(maskedRaw, "abc123xyz") {
		t.Errorf("raw env leaked plain app key: %s", maskedRaw)
	}
	if !strings.Contains(maskedRaw, "DB_PASSWORD=********") {
		t.Errorf("expected DB_PASSWORD=********, got: %s", maskedRaw)
	}
}

func TestScriptDeployer_VaultStorageIntegrity(t *testing.T) {
	secret := "test-vault-secret-key-32-chars-long"

	// Test ScriptDeployment encrypted env
	dep := &db.ScriptDeployment{
		ProjectID: "p_123",
	}
	rawEnv := "APP_KEY=base64:secret\nDB_PASSWORD=secretpass\n"
	if err := dep.EncryptRawEnv(rawEnv, secret); err != nil {
		t.Fatalf("encrypt raw env failed: %v", err)
	}
	if strings.Contains(dep.EnvEncrypted, "secretpass") {
		t.Fatalf("encrypted env contains plaintext password")
	}

	decrypted, err := dep.DecryptRawEnv(secret)
	if err != nil {
		t.Fatalf("decrypt raw env failed: %v", err)
	}
	if decrypted != rawEnv {
		t.Fatalf("decrypted env mismatch: got %q, expected %q", decrypted, rawEnv)
	}

	// Test ProvisionedDatabase encrypted password
	pdb := &db.ProvisionedDatabase{
		ProjectID: "p_123",
		Engine:    "mysql",
		DBName:    "test_db",
	}
	plainPass := "UltraSecretPassword999!"
	if err := pdb.EncryptPassword(plainPass, secret); err != nil {
		t.Fatalf("encrypt password failed: %v", err)
	}
	if strings.Contains(pdb.DBPasswordEncrypted, plainPass) {
		t.Fatalf("encrypted password contains plaintext")
	}

	decryptedPass, err := pdb.DecryptPassword(secret)
	if err != nil {
		t.Fatalf("decrypt password failed: %v", err)
	}
	if decryptedPass != plainPass {
		t.Fatalf("decrypted password mismatch: got %q, expected %q", decryptedPass, plainPass)
	}

	// Verify ToDTO() has zero plain password
	dto := pdb.ToDTO()
	if dto.HasPassword != true {
		t.Fatalf("expected HasPassword to be true")
	}
}
