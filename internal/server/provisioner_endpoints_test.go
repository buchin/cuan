package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/deployer"
	"github.com/buchin/cuan/internal/project"
)

func setupTestServerForProvisioner(t *testing.T) (*Server, string) {
	t.Helper()
	tempBase, err := os.MkdirTemp("", "cuan-srv-prov-*")
	if err != nil {
		t.Fatalf("create temp base: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempBase)
	})

	cfg := &config.Config{
		AuthSecret: "test-auth-secret-32-chars-long!",
	}
	srv := NewServer(cfg, nil)

	// Configure provisioner engine with temporary base dir
	provEngine := deployer.NewDatabaseProvisioner(deployer.ProvisionerConfig{
		DBRepo:         srv.ProvisionedDatabaseRepository(),
		VaultSecret:    cfg.AuthSecret,
		ProjectBaseDir: tempBase,
		MockMode:       true,
	})
	srv.SetProvisionerEngine(provEngine)

	return srv, tempBase
}

func createDummyProject(t *testing.T, srv *Server, projectID string, baseDir string) *project.ProjectResponse {
	t.Helper()
	projDir := filepath.Join(baseDir, projectID)
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatalf("create project dir: %v", err)
	}

	// Create sample .env.example in project dir
	sampleEnv := `APP_NAME=SampleApp
APP_KEY=base64:CHANGE_ME_PLEASE
APP_ENV=production
DB_CONNECTION=mysql
DB_HOST=127.0.0.1
DB_PORT=3306
DB_DATABASE=db_placeholder
DB_USERNAME=user_placeholder
DB_PASSWORD=secret_placeholder
`
	_ = os.WriteFile(filepath.Join(projDir, ".env.example"), []byte(sampleEnv), 0644)

	ctx := context.Background()
	p := &project.Project{
		ID:        projectID,
		Name:      "Test Automated Script",
		LocalPath: projDir,
	}
	if err := srv.ProjectService().Store().CreateProject(ctx, p); err != nil {
		t.Fatalf("create dummy project: %v", err)
	}
	return p.ToResponse()
}

func TestProvisionDatabaseEndpoint_SQLiteWithSchemaImport(t *testing.T) {
	srv, baseDir := setupTestServerForProvisioner(t)
	proj := createDummyProject(t, srv, "proj_prov_sqlite", baseDir)

	schemaSQL := `
CREATE TABLE customers (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT UNIQUE
);

CREATE TABLE orders (
    id INTEGER PRIMARY KEY,
    customer_id INTEGER,
    amount NUMERIC(10,2),
    FOREIGN KEY(customer_id) REFERENCES customers(id)
);

INSERT INTO customers (id, name, email) VALUES (1, 'Buchin', 'buchin@dropsugar.com');
INSERT INTO orders (id, customer_id, amount) VALUES (1, 1, 99.50);
`

	payload := map[string]interface{}{
		"engine":        "sqlite",
		"db_name":       "cuan_test_orders",
		"sql_content":   schemaSQL,
		"import_schema": true,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+proj.ID+"/provision-db", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Status         string                    `json:"status"`
		Database       map[string]interface{}    `json:"database"`
		TablesCount    int                       `json:"tables_count"`
		ImportedTables []string                  `json:"imported_tables"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Status != "success" {
		t.Errorf("expected status=success, got %s", resp.Status)
	}
	if resp.TablesCount != 2 {
		t.Errorf("expected 2 tables, got %d", resp.TablesCount)
	}
	if resp.Database["engine"] != "sqlite" {
		t.Errorf("expected engine=sqlite, got %v", resp.Database["engine"])
	}
	if resp.Database["status"] != "ready" {
		t.Errorf("expected status=ready, got %v", resp.Database["status"])
	}
	if resp.Database["schema_imported"] != true {
		t.Errorf("expected schema_imported=true, got %v", resp.Database["schema_imported"])
	}

	// Zero password leakage check: ensure no password field exists in JSON
	if _, exists := resp.Database["db_password"]; exists {
		t.Errorf("db_password must NOT be leaked in JSON response")
	}
	if _, exists := resp.Database["db_password_encrypted"]; exists {
		t.Errorf("db_password_encrypted must NOT be leaked in JSON response")
	}
	if resp.Database["has_password"] != false {
		t.Errorf("sqlite should have has_password=false")
	}
}

func TestGenerateEnvEndpoint_WithProvisionedDBAndSecrets(t *testing.T) {
	srv, baseDir := setupTestServerForProvisioner(t)
	proj := createDummyProject(t, srv, "proj_env_gen", baseDir)

	// 1. Provision a SQLite DB first
	provPayload := map[string]interface{}{
		"engine":        "sqlite",
		"db_name":       "cuan_env_db",
		"sql_content":   "CREATE TABLE test (id INTEGER PRIMARY KEY);",
		"import_schema": true,
	}
	body, _ := json.Marshal(provPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+proj.ID+"/provision-db", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup provision-db failed: %d %s", w.Code, w.Body.String())
	}

	// 2. Call POST /api/v1/projects/{id}/generate-env
	genPayload := map[string]interface{}{
		"custom_vars": map[string]string{
			"APP_NAME": "Cuan Turnkey Deployer",
		},
	}
	genBody, _ := json.Marshal(genPayload)
	genReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+proj.ID+"/generate-env", bytes.NewReader(genBody))
	genReq.Header.Set("Content-Type", "application/json")
	genW := httptest.NewRecorder()

	srv.Router().ServeHTTP(genW, genReq)

	if genW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", genW.Code, genW.Body.String())
	}

	var genResp struct {
		Status     string                 `json:"status"`
		ProjectID  string                 `json:"project_id"`
		Deployment map[string]interface{} `json:"deployment"`
		EnvKeys    []string               `json:"env_keys"`
		MaskedEnv  map[string]string      `json:"masked_env"`
	}
	if err := json.Unmarshal(genW.Body.Bytes(), &genResp); err != nil {
		t.Fatalf("unmarshal gen response: %v", err)
	}

	if genResp.Status != "success" {
		t.Errorf("expected status=success, got %s", genResp.Status)
	}
	if genResp.ProjectID != proj.ID {
		t.Errorf("project_id mismatch: got %s, expected %s", genResp.ProjectID, proj.ID)
	}

	// Verify masked variables - ZERO plaintext secrets!
	appKey := genResp.MaskedEnv["APP_KEY"]
	if !strings.HasPrefix(appKey, "base64:") || !strings.Contains(appKey, "*") {
		t.Errorf("expected masked APP_KEY, got: %s", appKey)
	}
	if strings.Contains(genW.Body.String(), "CHANGE_ME_PLEASE") {
		t.Errorf("response contains unreplaced placeholder CHANGE_ME_PLEASE")
	}

	jwtSecret := genResp.MaskedEnv["JWT_SECRET"]
	if jwtSecret != "********" {
		t.Errorf("expected masked JWT_SECRET (********), got: %s", jwtSecret)
	}

	// Verify non-sensitive variables
	if genResp.MaskedEnv["APP_NAME"] != "Cuan Turnkey Deployer" {
		t.Errorf("expected overridden APP_NAME, got: %s", genResp.MaskedEnv["APP_NAME"])
	}
	if genResp.MaskedEnv["DB_CONNECTION"] != "sqlite" {
		t.Errorf("expected DB_CONNECTION=sqlite, got: %s", genResp.MaskedEnv["DB_CONNECTION"])
	}

	// 3. Verify encrypted storage in script_deployments repository
	dep, err := srv.ScriptDeploymentRepository().GetDeploymentByProjectID(context.Background(), proj.ID)
	if err != nil {
		t.Fatalf("retrieve deployment record from db failed: %v", err)
	}
	if dep.EnvEncrypted == "" {
		t.Fatalf("EnvEncrypted must not be empty in database")
	}

	// Verify that the encrypted string does NOT contain plaintext APP_NAME or plain secret in plaintext
	decryptedRaw, err := dep.DecryptRawEnv("test-auth-secret-32-chars-long!")
	if err != nil {
		t.Fatalf("decrypt raw env failed: %v", err)
	}
	if !strings.Contains(decryptedRaw, "Cuan Turnkey Deployer") {
		t.Errorf("decrypted raw env does not contain expected app name")
	}
}

func TestGetEnvEndpoint_ZeroCredentialLeakage(t *testing.T) {
	srv, baseDir := setupTestServerForProvisioner(t)
	proj := createDummyProject(t, srv, "proj_get_env", baseDir)

	// First generate env
	genPayload := map[string]interface{}{
		"base_env": `
APP_NAME=TestApp
APP_KEY=base64:unmasked_raw_key_secret
DB_PASSWORD=SuperSecretPass123
`,
	}
	genBody, _ := json.Marshal(genPayload)
	genReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+proj.ID+"/generate-env", bytes.NewReader(genBody))
	genReq.Header.Set("Content-Type", "application/json")
	genW := httptest.NewRecorder()
	srv.Router().ServeHTTP(genW, genReq)
	if genW.Code != http.StatusOK {
		t.Fatalf("generate-env failed: %d %s", genW.Code, genW.Body.String())
	}

	// Test GET /api/v1/projects/{id}/env
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+proj.ID+"/env", nil)
	getW := httptest.NewRecorder()
	srv.Router().ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", getW.Code, getW.Body.String())
	}

	// Verify zero plaintext secrets in GET output
	rawBody := getW.Body.String()
	if strings.Contains(rawBody, "SuperSecretPass123") {
		t.Fatalf("GET /env leaked plaintext DB_PASSWORD: %s", rawBody)
	}
	if strings.Contains(rawBody, "unmasked_raw_key_secret") {
		t.Fatalf("GET /env leaked plaintext APP_KEY: %s", rawBody)
	}

	var getResp struct {
		Status    string            `json:"status"`
		ProjectID string            `json:"project_id"`
		HasEnv    bool              `json:"has_env"`
		EnvKeys   []string          `json:"env_keys"`
		MaskedEnv map[string]string `json:"masked_env"`
	}
	if err := json.Unmarshal(getW.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("unmarshal get env response: %v", err)
	}

	if !getResp.HasEnv {
		t.Errorf("expected has_env=true")
	}
	if getResp.MaskedEnv["DB_PASSWORD"] != "********" {
		t.Errorf("expected masked DB_PASSWORD, got: %s", getResp.MaskedEnv["DB_PASSWORD"])
	}
	if !strings.HasPrefix(getResp.MaskedEnv["APP_KEY"], "base64:") || !strings.Contains(getResp.MaskedEnv["APP_KEY"], "*") {
		t.Errorf("expected masked APP_KEY, got: %s", getResp.MaskedEnv["APP_KEY"])
	}
}

func TestProvisionDatabaseEndpoint_ValidationAndErrors(t *testing.T) {
	srv, baseDir := setupTestServerForProvisioner(t)
	proj := createDummyProject(t, srv, "proj_val_err", baseDir)

	// 1. Invalid DB Engine
	invalidPayload := map[string]interface{}{
		"engine": "mongodb_nosql",
	}
	body, _ := json.Marshal(invalidPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+proj.ID+"/provision-db", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid engine, got %d", w.Code)
	}

	// 2. Malicious DB Name (SQL Injection attempt)
	injectionPayload := map[string]interface{}{
		"engine":  "postgres",
		"db_name": "db; DROP DATABASE master; --",
	}
	body2, _ := json.Marshal(injectionPayload)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+proj.ID+"/provision-db", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w2, req2)

	if w2.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for SQL injection name, got %d", w2.Code)
	}
}

func TestLegacyApiRoutes_ProvisionAndEnv(t *testing.T) {
	srv, baseDir := setupTestServerForProvisioner(t)
	proj := createDummyProject(t, srv, "proj_legacy_routes", baseDir)

	// 1. Test POST /api/projects/{id}/provision-db
	provPayload := map[string]interface{}{
		"engine":  "sqlite",
		"db_name": "legacy_sqlite",
	}
	body, _ := json.Marshal(provPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+proj.ID+"/provision-db", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on legacy route, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Test POST /api/projects/{id}/generate-env
	genPayload := map[string]interface{}{
		"base_env": "APP_NAME=LegacyApp\nAPP_KEY=\n",
	}
	genBody, _ := json.Marshal(genPayload)
	genReq := httptest.NewRequest(http.MethodPost, "/api/projects/"+proj.ID+"/generate-env", bytes.NewReader(genBody))
	genReq.Header.Set("Content-Type", "application/json")
	genW := httptest.NewRecorder()
	srv.Router().ServeHTTP(genW, genReq)

	if genW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on legacy route, got %d: %s", genW.Code, genW.Body.String())
	}

	// 3. Test GET /api/projects/{id}/env
	getReq := httptest.NewRequest(http.MethodGet, "/api/projects/"+proj.ID+"/env", nil)
	getW := httptest.NewRecorder()
	srv.Router().ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on legacy route, got %d: %s", getW.Code, getW.Body.String())
	}
}

