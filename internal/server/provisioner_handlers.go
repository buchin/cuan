package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/deployer"
)

// ProvisionDBRequestBody defines JSON input for POST /api/v1/projects/{id}/provision-db.
type ProvisionDBRequestBody struct {
	Engine       string  `json:"engine"` // sqlite, mysql, postgres
	DBName       string  `json:"db_name,omitempty"`
	DBUser       string  `json:"db_user,omitempty"`
	DBPassword   string  `json:"db_password,omitempty"`
	ServerID     *string `json:"server_id,omitempty"`
	SQLFile      string  `json:"sql_file,omitempty"`
	SQLContent   string  `json:"sql_content,omitempty"`
	ImportSchema *bool   `json:"import_schema,omitempty"`
}

// GenerateEnvRequestBody defines JSON input for POST /api/v1/projects/{id}/generate-env.
type GenerateEnvRequestBody struct {
	DBID       string            `json:"db_id,omitempty"`
	ServerID   *string           `json:"server_id,omitempty"`
	CustomVars map[string]string `json:"custom_vars,omitempty"`
	BaseEnv    string            `json:"base_env,omitempty"`
}

// handleProvisionProjectDatabase provisions an isolated database (SQLite/MySQL/Postgres) and imports SQL schema.
// Routes: POST /api/projects/{id}/provision-db, POST /api/v1/projects/{id}/provision-db
func (s *Server) handleProvisionProjectDatabase(w http.ResponseWriter, r *http.Request) {
	if s.provisionerEngine == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "database provisioner engine is not configured")
		return
	}

	projectID := chi.URLParam(r, "id")
	if strings.TrimSpace(projectID) == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	// Resolve project directory
	projectDir := filepath.Join("data", "projects", projectID)
	if s.projectSvc != nil {
		proj, err := s.projectSvc.Get(r.Context(), projectID)
		if err == nil && proj != nil && proj.LocalPath != "" {
			projectDir = proj.LocalPath
		}
	}

	var reqBody ProvisionDBRequestBody
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
	}

	importSchema := true
	if reqBody.ImportSchema != nil {
		importSchema = *reqBody.ImportSchema
	}

	engine := reqBody.Engine
	if engine == "" {
		engine = db.DBEngineSQLite
	}

	provReq := deployer.ProvisionRequest{
		ProjectID:    projectID,
		ServerID:     reqBody.ServerID,
		Engine:       engine,
		DBName:       reqBody.DBName,
		DBUser:       reqBody.DBUser,
		DBPassword:   reqBody.DBPassword,
		ProjectDir:   projectDir,
		SQLFilePath:  reqBody.SQLFile,
		SQLContent:   reqBody.SQLContent,
		ImportSchema: importSchema,
	}

	result, err := s.provisionerEngine.Provision(r.Context(), provReq)
	if err != nil {
		if errors.Is(err, db.ErrInvalidDBEngine) || errors.Is(err, deployer.ErrInvalidSQLIdentifier) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to provision database: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"status":           "success",
		"database":         result.PublicDTO,
		"tables_count":     result.TablesCount,
		"imported_tables":  result.ImportedTables,
		"total_statements": result.TotalStatements,
		"execution_log":    result.ExecutionLog,
	})
}

// handleGenerateProjectEnv generates application secrets, auto-maps database vars, and saves encrypted .env.
// Routes: POST /api/projects/{id}/generate-env, POST /api/v1/projects/{id}/generate-env
func (s *Server) handleGenerateProjectEnv(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	if strings.TrimSpace(projectID) == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	// Resolve project directory
	projectDir := filepath.Join("data", "projects", projectID)
	if s.projectSvc != nil {
		proj, err := s.projectSvc.Get(r.Context(), projectID)
		if err == nil && proj != nil && proj.LocalPath != "" {
			projectDir = proj.LocalPath
		}
	}

	var reqBody GenerateEnvRequestBody
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
	}

	// 1. Generate high-entropy application secrets
	appSecrets, err := deployer.GenerateApplicationSecrets()
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to generate application secrets: "+err.Error())
		return
	}

	secretsMap := map[string]string{
		"APP_KEY":      appSecrets.LaravelAppKey,
		"JWT_SECRET":   appSecrets.JWTSecret,
		"SESSION_SALT": appSecrets.SessionSalt,
	}

	// 2. Auto-map database variables from provisioned database if available
	dbVars := make(map[string]string)
	vaultSecret := s.cfg.AuthSecret
	if vaultSecret == "" {
		vaultSecret = "cuan-default-database-provisioner-vault-key"
	}

	if s.provisionedDBRepo != nil {
		var pdb *db.ProvisionedDatabase
		var fetchErr error

		if reqBody.DBID != "" {
			pdb, fetchErr = s.provisionedDBRepo.GetDatabaseByID(r.Context(), reqBody.DBID)
		} else {
			pdb, fetchErr = s.provisionedDBRepo.GetDatabaseByProjectID(r.Context(), projectID)
		}

		if fetchErr == nil && pdb != nil {
			decryptedPass, _ := pdb.DecryptPassword(vaultSecret)
			dbVars = deployer.AutoMapDatabaseEnv(pdb.Engine, pdb.DBName, pdb.DBUser, decryptedPass, "127.0.0.1", 0)
		}
	}

	// 3. Resolve base template
	baseTemplate := reqBody.BaseEnv
	if strings.TrimSpace(baseTemplate) == "" {
		candidates := []string{".env.example", ".env.sample", ".env.template", ".env"}
		for _, cand := range candidates {
			candPath := filepath.Join(projectDir, cand)
			if data, err := os.ReadFile(candPath); err == nil && len(data) > 0 {
				baseTemplate = string(data)
				break
			}
		}
	}

	// 4. Merge configuration
	mergedEnv := deployer.MergeEnv(baseTemplate, dbVars, secretsMap, reqBody.CustomVars)

	// Persist .env to disk in project directory if directory exists
	if _, err := os.Stat(projectDir); err == nil {
		_ = os.WriteFile(filepath.Join(projectDir, ".env"), []byte(mergedEnv), 0644)
	}

	// 5. Encrypt and persist to script_deployments repository
	var depDTO db.ScriptDeploymentDTO
	if s.scriptDeployRepo != nil {
		dep, err := s.scriptDeployRepo.GetDeploymentByProjectID(r.Context(), projectID)
		if errors.Is(err, db.ErrScriptDeploymentNotFound) || dep == nil {
			dep = &db.ScriptDeployment{
				ID:          db.GenerateUUID(),
				ProjectID:   projectID,
				ServerID:    reqBody.ServerID,
				BuildStatus: db.BuildStatusPending,
				Health:      db.HealthStatusUnknown,
			}
			if err := dep.EncryptRawEnv(mergedEnv, vaultSecret); err != nil {
				respondJSONError(w, http.StatusInternalServerError, "failed to encrypt environment: "+err.Error())
				return
			}
			if err := s.scriptDeployRepo.CreateDeployment(r.Context(), dep); err != nil {
				respondJSONError(w, http.StatusInternalServerError, "failed to save deployment record: "+err.Error())
				return
			}
		} else {
			if err := dep.EncryptRawEnv(mergedEnv, vaultSecret); err != nil {
				respondJSONError(w, http.StatusInternalServerError, "failed to encrypt environment: "+err.Error())
				return
			}
			if reqBody.ServerID != nil {
				dep.ServerID = reqBody.ServerID
			}
			if err := s.scriptDeployRepo.UpdateDeployment(r.Context(), dep); err != nil {
				respondJSONError(w, http.StatusInternalServerError, "failed to update deployment record: "+err.Error())
				return
			}
		}
		depDTO = dep.ToDTO()
	}

	// 6. Zero Credential Leakage: Mask sensitive values on response
	parsedMap := deployer.ParseEnvMap(mergedEnv)
	maskedMap := deployer.MaskEnvVars(parsedMap)

	var envKeys []string
	for k := range parsedMap {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "success",
		"project_id": projectID,
		"deployment": depDTO,
		"env_keys":   envKeys,
		"masked_env": maskedMap,
	})
}

// handleGetProjectEnv retrieves environment configuration for a project with masked secrets.
// Routes: GET /api/projects/{id}/env, GET /api/v1/projects/{id}/env
func (s *Server) handleGetProjectEnv(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	if strings.TrimSpace(projectID) == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	vaultSecret := s.cfg.AuthSecret
	if vaultSecret == "" {
		vaultSecret = "cuan-default-database-provisioner-vault-key"
	}

	var envMap map[string]string

	// Try reading from script_deployments first
	if s.scriptDeployRepo != nil {
		dep, err := s.scriptDeployRepo.GetDeploymentByProjectID(r.Context(), projectID)
		if err == nil && dep != nil && dep.EnvEncrypted != "" {
			rawEnv, decErr := dep.DecryptRawEnv(vaultSecret)
			if decErr == nil {
				envMap = deployer.ParseEnvMap(rawEnv)
			}
		}
	}

	// Fallback to reading .env file from project directory
	if len(envMap) == 0 {
		projectDir := filepath.Join("data", "projects", projectID)
		if s.projectSvc != nil {
			proj, err := s.projectSvc.Get(r.Context(), projectID)
			if err == nil && proj != nil && proj.LocalPath != "" {
				projectDir = proj.LocalPath
			}
		}
		dotEnvPath := filepath.Join(projectDir, ".env")
		if data, err := os.ReadFile(dotEnvPath); err == nil {
			envMap = deployer.ParseEnvMap(string(data))
		}
	}

	if envMap == nil {
		envMap = make(map[string]string)
	}

	maskedMap := deployer.MaskEnvVars(envMap)
	var envKeys []string
	for k := range envMap {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "success",
		"project_id": projectID,
		"has_env":    len(envMap) > 0,
		"env_keys":   envKeys,
		"masked_env": maskedMap,
	})
}
