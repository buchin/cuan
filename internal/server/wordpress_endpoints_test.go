package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/config"
	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/deployer"
	"github.com/buchin/cuan/internal/wordpress"
)

func setupTestServerWithWordPress(t *testing.T) (*Server, string) {
	tempDir, err := os.MkdirTemp("", "cuan-server-wp-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cfg := &config.Config{
		AuthSecret:  "test-secret-key-32-bytes-cuan!",
		DatabaseURL: "",
	}

	srv := NewServer(cfg, nil)

	// Configure mock mode orchestrator for deterministic testing
	wpRepo := cuanDB.NewMemoryWordPressRepository()
	portPool := deployer.NewPortPoolWithRange(9400, 9450)
	portPool.SetCheckSocket(false)

	orch := wordpress.NewOrchestrator(wordpress.Config{
		DBRepo:        wpRepo,
		PortPool:      portPool,
		VaultSecret:   "test-secret-key-32-bytes-cuan!",
		BaseDeployDir: filepath.Join(tempDir, "deployments"),
		CoreCacheDir:  filepath.Join(tempDir, "cache"),
		MockMode:      true,
	})

	srv.SetWordPressRepository(wpRepo)
	srv.SetWordPressOrchestrator(orch)

	webmaster := wordpress.NewWebmasterEngine(wordpress.WebmasterConfig{
		DBRepo:        wpRepo,
		BaseDeployDir: filepath.Join(tempDir, "deployments"),
		MockMode:      true,
	})
	srv.SetWordPressWebmaster(webmaster)

	return srv, tempDir
}

func TestWordPressEndpoints_Validation(t *testing.T) {
	srv, tempDir := setupTestServerWithWordPress(t)
	defer os.RemoveAll(tempDir)

	// Missing site_title
	body := map[string]interface{}{
		"admin_email":    "admin@test.com",
		"admin_username": "admin",
	}
	data, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/projects/p_test/wordpress/deploy", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing site_title, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWordPressEndpoints_FullLifecycle(t *testing.T) {
	srv, tempDir := setupTestServerWithWordPress(t)
	defer os.RemoveAll(tempDir)

	projectID := "p_wp_lifecycle"

	// 1. Deploy
	deployPayload := WordPressDeployRequestBody{
		SiteTitle:          "Turnkey WP Blog",
		AdminEmail:         "founder@wpblog.io",
		AdminUsername:      "wpadmin",
		PermalinkStructure: "/%postname%/",
	}
	data, _ := json.Marshal(deployPayload)

	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/wordpress/deploy", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created on deploy, got %d: %s", w.Code, w.Body.String())
	}

	var deployRes wordpress.DeployResult
	if err := json.NewDecoder(w.Body).Decode(&deployRes); err != nil {
		t.Fatalf("failed to decode deploy response: %v", err)
	}

	if deployRes.Status != cuanDB.WPStatusRunning {
		t.Errorf("expected status 'running', got %s", deployRes.Status)
	}
	if deployRes.HealthStatus != cuanDB.WPHealthStatusHealthy {
		t.Errorf("expected health 'healthy', got %s", deployRes.HealthStatus)
	}
	if deployRes.Port < 9400 || deployRes.Port > 9450 {
		t.Errorf("allocated port %d outside range [9400, 9450]", deployRes.Port)
	}
	if deployRes.AdminPasswordPlain == "" {
		t.Errorf("expected admin_password_plain to be populated in creation result")
	}

	// 2. GET /api/projects/{id}/wordpress (sanitized DTO)
	getReq := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/wordpress", nil)
	getW := httptest.NewRecorder()
	srv.Router().ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on get, got %d: %s", getW.Code, getW.Body.String())
	}

	var dto cuanDB.WordPressDeploymentDTO
	if err := json.NewDecoder(getW.Body).Decode(&dto); err != nil {
		t.Fatalf("failed to decode DTO: %v", err)
	}
	if dto.SiteTitle != "Turnkey WP Blog" {
		t.Errorf("expected site title 'Turnkey WP Blog', got %s", dto.SiteTitle)
	}
	if !dto.HasAdminPassword {
		t.Errorf("expected HasAdminPassword=true")
	}

	// Verify zero password leakage in raw JSON
	rawJSON := getW.Body.String()
	if bytes.Contains([]byte(rawJSON), []byte("admin_password_encrypted")) {
		t.Errorf("raw JSON leaked admin_password_encrypted field")
	}
	if bytes.Contains([]byte(rawJSON), []byte(deployRes.AdminPasswordPlain)) {
		t.Errorf("raw JSON leaked plain admin password")
	}

	// 3. GET /api/projects/{id}/wordpress/health
	healthReq := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/wordpress/health", nil)
	healthW := httptest.NewRecorder()
	srv.Router().ServeHTTP(healthW, healthReq)

	if healthW.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on health check, got %d: %s", healthW.Code, healthW.Body.String())
	}

	// 4. POST /api/projects/{id}/wordpress/restart
	restartReq := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/wordpress/restart", nil)
	restartW := httptest.NewRecorder()
	srv.Router().ServeHTTP(restartW, restartReq)

	if restartW.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on restart, got %d: %s", restartW.Code, restartW.Body.String())
	}

	// 5. POST /api/projects/{id}/wordpress/stop
	stopReq := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/wordpress/stop", nil)
	stopW := httptest.NewRecorder()
	srv.Router().ServeHTTP(stopW, stopReq)

	if stopW.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on stop, got %d: %s", stopW.Code, stopW.Body.String())
	}

	// 6. DELETE /api/projects/{id}/wordpress
	delReq := httptest.NewRequest(http.MethodDelete, "/api/projects/"+projectID+"/wordpress", nil)
	delW := httptest.NewRecorder()
	srv.Router().ServeHTTP(delW, delReq)

	if delW.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on delete, got %d: %s", delW.Code, delW.Body.String())
	}

	// 7. Verify subsequent GET returns 404
	afterGetReq := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/wordpress", nil)
	afterGetW := httptest.NewRecorder()
	srv.Router().ServeHTTP(afterGetW, afterGetReq)

	if afterGetW.Code != http.StatusNotFound {
		t.Errorf("expected 404 after deletion, got %d", afterGetW.Code)
	}
}

func createZipBuffer(t *testing.T, files map[string]string) *bytes.Buffer {
	t.Helper()
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write zip content %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip: %v", err)
	}
	return buf
}

func deployTestWordPressProject(t *testing.T, srv *Server, projectID string) {
	t.Helper()
	deployPayload := WordPressDeployRequestBody{
		SiteTitle:     "Webmaster Blog",
		AdminEmail:    "webmaster@cuan.dojo.cc",
		AdminUsername: "webmaster",
	}
	data, _ := json.Marshal(deployPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/wordpress/deploy", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("failed to deploy wordpress for project %s: %s", projectID, w.Body.String())
	}
}

func TestWordPressEndpoints_WebmasterAssets(t *testing.T) {
	srv, tempDir := setupTestServerWithWordPress(t)
	defer os.RemoveAll(tempDir)

	projectID := "p_wp_assets"
	deployTestWordPressProject(t, srv, projectID)

	// 1. Upload Theme ZIP via multipart/form-data
	themeFiles := map[string]string{
		"generatepress/style.css": "/*\nTheme Name: GeneratePress\nVersion: 3.5.0\n*/",
		"generatepress/index.php": "<?php // index",
	}
	zipBuf := createZipBuffer(t, themeFiles)

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "generatepress.zip")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write(zipBuf.Bytes()); err != nil {
		t.Fatalf("failed to write form file: %v", err)
	}
	_ = writer.WriteField("kind", "theme")
	_ = writer.WriteField("activate", "true")
	writer.Close()

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/wordpress/assets/upload", body)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadW := httptest.NewRecorder()
	srv.Router().ServeHTTP(uploadW, uploadReq)

	if uploadW.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on asset upload, got %d: %s", uploadW.Code, uploadW.Body.String())
	}

	var installRes wordpress.InstallAssetResult
	if err := json.NewDecoder(uploadW.Body).Decode(&installRes); err != nil {
		t.Fatalf("failed to decode upload response: %v", err)
	}
	if installRes.Header.Slug != "generatepress" {
		t.Errorf("expected slug 'generatepress', got %s", installRes.Header.Slug)
	}
	assetID := installRes.Asset.ID

	// 2. List Assets
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID+"/wordpress/assets", nil)
	listW := httptest.NewRecorder()
	srv.Router().ServeHTTP(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on list assets, got %d: %s", listW.Code, listW.Body.String())
	}

	var assets []*cuanDB.WordPressPluginTheme
	if err := json.NewDecoder(listW.Body).Decode(&assets); err != nil {
		t.Fatalf("failed to decode assets list: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}

	// 3. Toggle Status
	statusBody, _ := json.Marshal(WordPressAssetStatusRequestBody{IsActive: false})
	toggleReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/wordpress/assets/"+assetID+"/status", bytes.NewReader(statusBody))
	toggleReq.Header.Set("Content-Type", "application/json")
	toggleW := httptest.NewRecorder()
	srv.Router().ServeHTTP(toggleW, toggleReq)

	if toggleW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on toggle asset status, got %d: %s", toggleW.Code, toggleW.Body.String())
	}

	// 4. Delete Asset
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+projectID+"/wordpress/assets/"+assetID, nil)
	delW := httptest.NewRecorder()
	srv.Router().ServeHTTP(delW, delReq)

	if delW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on delete asset, got %d: %s", delW.Code, delW.Body.String())
	}
}

func TestWordPressEndpoints_WebmasterScripts(t *testing.T) {
	srv, tempDir := setupTestServerWithWordPress(t)
	defer os.RemoveAll(tempDir)

	projectID := "p_wp_scripts"
	deployTestWordPressProject(t, srv, projectID)

	// 1. Inject Script
	enabled := true
	injectBody, _ := json.Marshal(WordPressScriptInjectRequestBody{
		Name:     "Google Analytics 4",
		Type:     "ga4",
		Position: "head",
		Content:  "<script>window.dataLayer=[];</script>",
		Enabled:  &enabled,
	})

	injectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/wordpress/scripts", bytes.NewReader(injectBody))
	injectReq.Header.Set("Content-Type", "application/json")
	injectW := httptest.NewRecorder()
	srv.Router().ServeHTTP(injectW, injectReq)

	if injectW.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on inject script, got %d: %s", injectW.Code, injectW.Body.String())
	}

	var injectRes wordpress.InjectScriptResult
	if err := json.NewDecoder(injectW.Body).Decode(&injectRes); err != nil {
		t.Fatalf("failed to decode inject response: %v", err)
	}
	scriptID := injectRes.Script.ID

	// 2. List Scripts
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID+"/wordpress/scripts", nil)
	listW := httptest.NewRecorder()
	srv.Router().ServeHTTP(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on list scripts, got %d: %s", listW.Code, listW.Body.String())
	}

	var scripts []wordpress.InjectedScript
	if err := json.NewDecoder(listW.Body).Decode(&scripts); err != nil {
		t.Fatalf("failed to decode scripts: %v", err)
	}
	if len(scripts) != 1 {
		t.Fatalf("expected 1 script, got %d", len(scripts))
	}

	// 3. Toggle Script
	toggleBody, _ := json.Marshal(WordPressScriptToggleRequestBody{Enabled: false})
	toggleReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/wordpress/scripts/"+scriptID+"/toggle", bytes.NewReader(toggleBody))
	toggleReq.Header.Set("Content-Type", "application/json")
	toggleW := httptest.NewRecorder()
	srv.Router().ServeHTTP(toggleW, toggleReq)

	if toggleW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on toggle script, got %d: %s", toggleW.Code, toggleW.Body.String())
	}

	// 4. Delete Script
	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+projectID+"/wordpress/scripts/"+scriptID, nil)
	delW := httptest.NewRecorder()
	srv.Router().ServeHTTP(delW, delReq)

	if delW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on delete script, got %d: %s", delW.Code, delW.Body.String())
	}
}

func TestWordPressEndpoints_WebmasterConfig(t *testing.T) {
	srv, tempDir := setupTestServerWithWordPress(t)
	defer os.RemoveAll(tempDir)

	projectID := "p_wp_config"
	deployTestWordPressProject(t, srv, projectID)

	// 1. Tweak Config (Success)
	tweakBody, _ := json.Marshal(WordPressConfigTweakRequestBody{
		Tweaks: map[string]interface{}{
			"WP_DEBUG":        true,
			"WP_MEMORY_LIMIT": "512M",
		},
	})
	tweakReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/wordpress/config", bytes.NewReader(tweakBody))
	tweakReq.Header.Set("Content-Type", "application/json")
	tweakW := httptest.NewRecorder()
	srv.Router().ServeHTTP(tweakW, tweakReq)

	if tweakW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on config tweak, got %d: %s", tweakW.Code, tweakW.Body.String())
	}

	var tweakRes wordpress.ConfigTweakResult
	if err := json.NewDecoder(tweakW.Body).Decode(&tweakRes); err != nil {
		t.Fatalf("failed to decode config tweak response: %v", err)
	}
	if !tweakRes.SyntaxValid {
		t.Errorf("expected SyntaxValid=true")
	}

	// 2. Get Config
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID+"/wordpress/config", nil)
	getW := httptest.NewRecorder()
	srv.Router().ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on get config, got %d: %s", getW.Code, getW.Body.String())
	}

	var activeTweaks map[string]interface{}
	if err := json.NewDecoder(getW.Body).Decode(&activeTweaks); err != nil {
		t.Fatalf("failed to decode config map: %v", err)
	}
	if activeTweaks["WP_DEBUG"] != true {
		t.Errorf("expected WP_DEBUG=true, got %v", activeTweaks["WP_DEBUG"])
	}

	// 3. Tweak Config with broken syntax (Pre-save validation failure -> 400 Bad Request)
	badBody, _ := json.Marshal(WordPressConfigTweakRequestBody{
		RawDirectives: []string{"define( 'BROKEN_PHP', 'unclosed quote ;"},
	})
	badReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/wordpress/config", bytes.NewReader(badBody))
	badReq.Header.Set("Content-Type", "application/json")
	badW := httptest.NewRecorder()
	srv.Router().ServeHTTP(badW, badReq)

	if badW.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on syntax validation failure, got %d: %s", badW.Code, badW.Body.String())
	}
	if !strings.Contains(badW.Body.String(), "syntax") && !strings.Contains(badW.Body.String(), "validation") {
		t.Errorf("expected error message to mention syntax/validation, got: %s", badW.Body.String())
	}
}

func TestWordPressEndpoints_WebmasterActions(t *testing.T) {
	srv, tempDir := setupTestServerWithWordPress(t)
	defer os.RemoveAll(tempDir)

	projectID := "p_wp_actions"
	deployTestWordPressProject(t, srv, projectID)

	// Inject a script to generate an audit action
	enabled := true
	injectBody, _ := json.Marshal(WordPressScriptInjectRequestBody{
		Name:     "Meta Pixel Head",
		Type:     "meta_pixel",
		Position: "head",
		Content:  "<script>fbq('init', '123');</script>",
		Enabled:  &enabled,
	})
	injectReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/wordpress/scripts", bytes.NewReader(injectBody))
	injectReq.Header.Set("Content-Type", "application/json")
	injectW := httptest.NewRecorder()
	srv.Router().ServeHTTP(injectW, injectReq)
	if injectW.Code != http.StatusCreated {
		t.Fatalf("inject script failed: %s", injectW.Body.String())
	}

	// Fetch Actions History
	actionsReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID+"/wordpress/actions", nil)
	actionsW := httptest.NewRecorder()
	srv.Router().ServeHTTP(actionsW, actionsReq)

	if actionsW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on list actions, got %d: %s", actionsW.Code, actionsW.Body.String())
	}

	var actions []*cuanDB.WordPressWebmasterAction
	if err := json.NewDecoder(actionsW.Body).Decode(&actions); err != nil {
		t.Fatalf("failed to decode actions: %v", err)
	}
	if len(actions) == 0 {
		t.Fatalf("expected at least 1 webmaster action recorded")
	}
	if actions[0].ActionType != cuanDB.WebmasterActionInjectScript {
		t.Errorf("expected action_type 'inject_script', got %s", actions[0].ActionType)
	}
}

