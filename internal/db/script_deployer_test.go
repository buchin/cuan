package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestScriptDeployer_ValidationAndNormalization(t *testing.T) {
	// Build Status
	bs, err := NormalizeBuildStatus("")
	if err != nil || bs != BuildStatusPending {
		t.Errorf("expected default pending, got %s, err: %v", bs, err)
	}

	bs, err = NormalizeBuildStatus("SUCCESS")
	if err != nil || bs != BuildStatusSuccess {
		t.Errorf("expected success, got %s, err: %v", bs, err)
	}

	_, err = NormalizeBuildStatus("invalid_status")
	if !errors.Is(err, ErrInvalidBuildStatus) {
		t.Errorf("expected ErrInvalidBuildStatus, got %v", err)
	}

	// Health Status
	hs, err := NormalizeHealthStatus("")
	if err != nil || hs != HealthStatusUnknown {
		t.Errorf("expected default unknown, got %s, err: %v", hs, err)
	}

	hs, err = NormalizeHealthStatus("HEALTHY")
	if err != nil || hs != HealthStatusHealthy {
		t.Errorf("expected healthy, got %s, err: %v", hs, err)
	}

	_, err = NormalizeHealthStatus("invalid_health")
	if !errors.Is(err, ErrInvalidHealthStatus) {
		t.Errorf("expected ErrInvalidHealthStatus, got %v", err)
	}

	// DB Engine
	eng, err := NormalizeDBEngine("")
	if err != nil || eng != DBEngineSQLite {
		t.Errorf("expected default sqlite, got %s, err: %v", eng, err)
	}

	eng, err = NormalizeDBEngine("MYSQL")
	if err != nil || eng != DBEngineMySQL {
		t.Errorf("expected mysql, got %s, err: %v", eng, err)
	}

	eng, err = NormalizeDBEngine("postgres")
	if err != nil || eng != DBEnginePostgres {
		t.Errorf("expected postgres, got %s, err: %v", eng, err)
	}

	_, err = NormalizeDBEngine("oracle")
	if !errors.Is(err, ErrInvalidDBEngine) {
		t.Errorf("expected ErrInvalidDBEngine, got %v", err)
	}

	// DB Status
	dbs, err := NormalizeDBStatus("")
	if err != nil || dbs != DBStatusProvisioning {
		t.Errorf("expected default provisioning, got %s, err: %v", dbs, err)
	}

	dbs, err = NormalizeDBStatus("READY")
	if err != nil || dbs != DBStatusReady {
		t.Errorf("expected ready, got %s, err: %v", dbs, err)
	}

	_, err = NormalizeDBStatus("invalid_db_status")
	if !errors.Is(err, ErrInvalidDBStatus) {
		t.Errorf("expected ErrInvalidDBStatus, got %v", err)
	}
}

func TestScriptDeployer_DTOAndSecurity(t *testing.T) {
	serverID := "srv-123"
	dep := &ScriptDeployment{
		ID:           "dep-001",
		ProjectID:    "proj-001",
		ServerID:     &serverID,
		Port:         8085,
		Domain:       "app.example.com",
		EnvEncrypted: "sensitive_encrypted_payload",
		BuildStatus:  "success",
		Health:       "healthy",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	dto := dep.ToDTO()
	if dto.ID != dep.ID || dto.ProjectID != dep.ProjectID || !dto.HasEnv {
		t.Errorf("unexpected DTO values: %+v", dto)
	}
	if dto.Port != 8085 || dto.Domain != "app.example.com" {
		t.Errorf("port or domain mismatch in DTO: %+v", dto)
	}

	// Verify JSON serialization hides EnvEncrypted
	bytes, err := json.Marshal(dep)
	if err != nil {
		t.Fatalf("failed to marshal ScriptDeployment: %v", err)
	}
	var rawMap map[string]interface{}
	if err := json.Unmarshal(bytes, &rawMap); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if _, exists := rawMap["env_encrypted"]; exists {
		t.Errorf("env_encrypted MUST NOT be exposed in JSON output")
	}

	// Test ProvisionedDatabase DTO & security
	pdb := &ProvisionedDatabase{
		ID:                  "pdb-001",
		ProjectID:           "proj-001",
		ServerID:            &serverID,
		Engine:              "mysql",
		DBName:              "cuan_store",
		DBUser:              "cuan_user",
		DBPasswordEncrypted: "super_secret_db_password_enc",
		Status:              "ready",
		SchemaImported:      true,
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}

	pdbDTO := pdb.ToDTO()
	if pdbDTO.ID != pdb.ID || !pdbDTO.HasPassword || !pdbDTO.SchemaImported {
		t.Errorf("unexpected ProvisionedDatabaseDTO values: %+v", pdbDTO)
	}

	pdbBytes, err := json.Marshal(pdb)
	if err != nil {
		t.Fatalf("failed to marshal ProvisionedDatabase: %v", err)
	}
	var pdbRawMap map[string]interface{}
	if err := json.Unmarshal(pdbBytes, &pdbRawMap); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if _, exists := pdbRawMap["db_password_encrypted"]; exists {
		t.Errorf("db_password_encrypted MUST NOT be exposed in JSON output")
	}
}

func TestScriptDeployer_AES256GCMEncryption(t *testing.T) {
	secretKey := "cuan-test-super-secret-vault-key-32b"

	// 1. Test EnvMap Encryption/Decryption
	dep := &ScriptDeployment{
		ID:        "dep-vault-1",
		ProjectID: "proj-vault",
	}

	envSource := map[string]string{
		"APP_NAME":   "Cuan Store",
		"APP_ENV":    "production",
		"APP_KEY":    "base64:randomkey1234567890abcdef=",
		"DB_HOST":    "127.0.0.1",
		"DB_PORT":    "3306",
		"DB_NAME":    "store_prod",
		"DB_PASS":    "P@ssw0rd!Complex",
		"JWT_SECRET": "jwt-token-secret-xyz",
	}

	if err := dep.EncryptEnvMap(envSource, secretKey); err != nil {
		t.Fatalf("EncryptEnvMap failed: %v", err)
	}

	if dep.EnvEncrypted == "" {
		t.Fatalf("expected non-empty encrypted env")
	}

	// Decrypt with correct key
	decryptedMap, err := dep.DecryptEnvMap(secretKey)
	if err != nil {
		t.Fatalf("DecryptEnvMap failed: %v", err)
	}
	for k, v := range envSource {
		if decryptedMap[k] != v {
			t.Errorf("key %s mismatch: expected %s, got %s", k, v, decryptedMap[k])
		}
	}

	// Decrypt with wrong key should fail
	_, err = dep.DecryptEnvMap("wrong-decryption-key-vault")
	if err == nil {
		t.Fatalf("expected decryption failure with wrong key")
	}

	// 2. Test RawEnv Encryption/Decryption
	rawEnv := `APP_NAME="Laravel Script"
APP_ENV=production
APP_KEY=base64:testkey
DB_CONNECTION=mysql
DB_HOST=127.0.0.1
`
	if err := dep.EncryptRawEnv(rawEnv, secretKey); err != nil {
		t.Fatalf("EncryptRawEnv failed: %v", err)
	}

	decryptedRaw, err := dep.DecryptRawEnv(secretKey)
	if err != nil {
		t.Fatalf("DecryptRawEnv failed: %v", err)
	}
	if decryptedRaw != rawEnv {
		t.Errorf("decrypted raw env mismatch: expected %q, got %q", rawEnv, decryptedRaw)
	}

	// 3. Test ProvisionedDatabase Password Encryption
	pdb := &ProvisionedDatabase{
		ID:        "pdb-vault-1",
		ProjectID: "proj-vault",
		Engine:    "mysql",
		DBName:    "prod_db",
	}

	dbPassword := "super_secret_db_pass_!@#123"
	if err := pdb.EncryptPassword(dbPassword, secretKey); err != nil {
		t.Fatalf("EncryptPassword failed: %v", err)
	}

	if pdb.DBPasswordEncrypted == "" {
		t.Fatalf("expected non-empty encrypted password")
	}

	decryptedPass, err := pdb.DecryptPassword(secretKey)
	if err != nil {
		t.Fatalf("DecryptPassword failed: %v", err)
	}
	if decryptedPass != dbPassword {
		t.Errorf("decrypted password mismatch: expected %s, got %s", dbPassword, decryptedPass)
	}

	// Decrypt with wrong key
	_, err = pdb.DecryptPassword("incorrect-secret")
	if err == nil {
		t.Fatalf("expected error decrypting password with wrong secret")
	}
}

func TestMemoryScriptDeploymentRepository_CRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryScriptDeploymentRepository()

	// 1. Validation errors
	if err := repo.CreateDeployment(ctx, nil); err == nil {
		t.Errorf("expected error on nil deployment")
	}
	if err := repo.CreateDeployment(ctx, &ScriptDeployment{ProjectID: ""}); !errors.Is(err, ErrEmptyDeploymentProjectID) {
		t.Errorf("expected ErrEmptyDeploymentProjectID, got %v", err)
	}

	// 2. Create
	serverID := "srv-prod-1"
	dep := &ScriptDeployment{
		ID:          "dep-alpha",
		ProjectID:   "proj-alpha",
		ServerID:    &serverID,
		Port:        8090,
		Domain:      "alpha.example.com",
		BuildStatus: "building",
		Health:      "unknown",
	}
	if err := repo.CreateDeployment(ctx, dep); err != nil {
		t.Fatalf("failed to create deployment: %v", err)
	}

	// 3. GetByID
	got, err := repo.GetDeploymentByID(ctx, "dep-alpha")
	if err != nil {
		t.Fatalf("failed to get deployment by id: %v", err)
	}
	if got.Port != 8090 || got.Domain != "alpha.example.com" {
		t.Errorf("unexpected deployment data: %+v", got)
	}

	// 4. GetByProjectID
	gotProj, err := repo.GetDeploymentByProjectID(ctx, "proj-alpha")
	if err != nil {
		t.Fatalf("failed to get deployment by project id: %v", err)
	}
	if gotProj.ID != "dep-alpha" {
		t.Errorf("expected ID dep-alpha, got %s", gotProj.ID)
	}

	// 5. Update
	got.BuildStatus = "success"
	got.Health = "healthy"
	got.Port = 8091
	if err := repo.UpdateDeployment(ctx, got); err != nil {
		t.Fatalf("failed to update deployment: %v", err)
	}

	updated, err := repo.GetDeploymentByID(ctx, "dep-alpha")
	if err != nil {
		t.Fatalf("failed to get updated deployment: %v", err)
	}
	if updated.BuildStatus != "success" || updated.Health != "healthy" || updated.Port != 8091 {
		t.Errorf("update was not reflected: %+v", updated)
	}

	// 6. ListByProjectID
	dep2 := &ScriptDeployment{
		ID:          "dep-beta",
		ProjectID:   "proj-alpha",
		Port:        8092,
		Domain:      "beta.example.com",
		BuildStatus: "running",
		Health:      "healthy",
	}
	if err := repo.CreateDeployment(ctx, dep2); err != nil {
		t.Fatalf("failed to create second deployment: %v", err)
	}

	list, err := repo.ListDeploymentsByProjectID(ctx, "proj-alpha")
	if err != nil {
		t.Fatalf("failed to list deployments: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 deployments, got %d", len(list))
	}

	// 7. Delete
	if err := repo.DeleteDeployment(ctx, "dep-alpha"); err != nil {
		t.Fatalf("failed to delete deployment: %v", err)
	}

	_, err = repo.GetDeploymentByID(ctx, "dep-alpha")
	if !errors.Is(err, ErrScriptDeploymentNotFound) {
		t.Errorf("expected ErrScriptDeploymentNotFound, got %v", err)
	}
}

func TestMemoryProvisionedDatabaseRepository_CRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryProvisionedDatabaseRepository()

	// 1. Validation errors
	if err := repo.CreateDatabase(ctx, nil); err == nil {
		t.Errorf("expected error on nil database")
	}
	if err := repo.CreateDatabase(ctx, &ProvisionedDatabase{ProjectID: "", DBName: "test"}); !errors.Is(err, ErrEmptyDBProjectID) {
		t.Errorf("expected ErrEmptyDBProjectID, got %v", err)
	}
	if err := repo.CreateDatabase(ctx, &ProvisionedDatabase{ProjectID: "p1", DBName: ""}); !errors.Is(err, ErrEmptyDBName) {
		t.Errorf("expected ErrEmptyDBName, got %v", err)
	}

	// 2. Create
	pdb := &ProvisionedDatabase{
		ID:        "pdb-alpha",
		ProjectID: "proj-alpha",
		Engine:    "postgres",
		DBName:    "alpha_db",
		DBUser:    "alpha_user",
		Status:    "provisioning",
	}
	if err := repo.CreateDatabase(ctx, pdb); err != nil {
		t.Fatalf("failed to create provisioned database: %v", err)
	}

	// 3. GetByID
	got, err := repo.GetDatabaseByID(ctx, "pdb-alpha")
	if err != nil {
		t.Fatalf("failed to get database by id: %v", err)
	}
	if got.DBName != "alpha_db" || got.Engine != "postgres" {
		t.Errorf("unexpected database data: %+v", got)
	}

	// 4. GetByProjectID
	gotProj, err := repo.GetDatabaseByProjectID(ctx, "proj-alpha")
	if err != nil {
		t.Fatalf("failed to get database by project id: %v", err)
	}
	if gotProj.ID != "pdb-alpha" {
		t.Errorf("expected ID pdb-alpha, got %s", gotProj.ID)
	}

	// 5. Update
	got.Status = "ready"
	got.SchemaImported = true
	if err := repo.UpdateDatabase(ctx, got); err != nil {
		t.Fatalf("failed to update database: %v", err)
	}

	updated, err := repo.GetDatabaseByID(ctx, "pdb-alpha")
	if err != nil {
		t.Fatalf("failed to get updated database: %v", err)
	}
	if updated.Status != "ready" || !updated.SchemaImported {
		t.Errorf("database update not reflected: %+v", updated)
	}

	// 6. ListByProjectID
	pdb2 := &ProvisionedDatabase{
		ID:        "pdb-beta",
		ProjectID: "proj-alpha",
		Engine:    "sqlite",
		DBName:    "beta.sqlite",
		Status:    "ready",
	}
	if err := repo.CreateDatabase(ctx, pdb2); err != nil {
		t.Fatalf("failed to create second database: %v", err)
	}

	list, err := repo.ListDatabasesByProjectID(ctx, "proj-alpha")
	if err != nil {
		t.Fatalf("failed to list databases: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 databases, got %d", len(list))
	}

	// 7. Delete
	if err := repo.DeleteDatabase(ctx, "pdb-alpha"); err != nil {
		t.Fatalf("failed to delete database: %v", err)
	}

	_, err = repo.GetDatabaseByID(ctx, "pdb-alpha")
	if !errors.Is(err, ErrProvisionedDBNotFound) {
		t.Errorf("expected ErrProvisionedDBNotFound, got %v", err)
	}
}

func TestScriptDeployerRepositories_Concurrency(t *testing.T) {
	ctx := context.Background()
	depRepo := NewMemoryScriptDeploymentRepository()
	pdbRepo := NewMemoryProvisionedDatabaseRepository()

	var wg sync.WaitGroup
	workers := 25

	// Concurrent ScriptDeployment operations
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := fmt.Sprintf("dep-concur-%d", idx)
			dep := &ScriptDeployment{
				ID:          id,
				ProjectID:   "proj-concur",
				Port:        8000 + idx,
				Domain:      fmt.Sprintf("app-%d.test.com", idx),
				BuildStatus: "pending",
				Health:      "unknown",
			}
			if err := depRepo.CreateDeployment(ctx, dep); err != nil {
				t.Errorf("concurrent create deployment failed: %v", err)
				return
			}
			dep.BuildStatus = "running"
			_ = depRepo.UpdateDeployment(ctx, dep)
			_, _ = depRepo.GetDeploymentByID(ctx, id)
		}(i)
	}

	// Concurrent ProvisionedDatabase operations
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := fmt.Sprintf("pdb-concur-%d", idx)
			pdb := &ProvisionedDatabase{
				ID:        id,
				ProjectID: "proj-concur",
				Engine:    "mysql",
				DBName:    fmt.Sprintf("db_%d", idx),
				Status:    "provisioning",
			}
			if err := pdbRepo.CreateDatabase(ctx, pdb); err != nil {
				t.Errorf("concurrent create database failed: %v", err)
				return
			}
			pdb.Status = "ready"
			_ = pdbRepo.UpdateDatabase(ctx, pdb)
			_, _ = pdbRepo.GetDatabaseByID(ctx, id)
		}(i)
	}

	wg.Wait()

	deps, err := depRepo.ListDeploymentsByProjectID(ctx, "proj-concur")
	if err != nil || len(deps) != workers {
		t.Errorf("expected %d deployments, got %d, err: %v", workers, len(deps), err)
	}

	pdbs, err := pdbRepo.ListDatabasesByProjectID(ctx, "proj-concur")
	if err != nil || len(pdbs) != workers {
		t.Errorf("expected %d databases, got %d, err: %v", workers, len(pdbs), err)
	}
}

func TestRepositories_ScriptDeployerIntegration(t *testing.T) {
	memRepos := NewMemoryRepositories()
	if memRepos.ScriptDeployments == nil {
		t.Errorf("expected non-nil ScriptDeployments in memory repositories")
	}
	if memRepos.ProvisionedDatabases == nil {
		t.Errorf("expected non-nil ProvisionedDatabases in memory repositories")
	}

	sqlRepos := NewSQLRepositories(nil)
	if sqlRepos.ScriptDeployments == nil {
		t.Errorf("expected non-nil ScriptDeployments in SQL repositories")
	}
	if sqlRepos.ProvisionedDatabases == nil {
		t.Errorf("expected non-nil ProvisionedDatabases in SQL repositories")
	}
}
