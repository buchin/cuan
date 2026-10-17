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

func TestWordPress_ValidationAndNormalization(t *testing.T) {
	// 1. WordPress Deployment Status
	st, err := NormalizeWPStatus("")
	if err != nil || st != WPStatusPending {
		t.Errorf("expected default pending, got %s, err: %v", st, err)
	}

	st, err = NormalizeWPStatus("RUNNING")
	if err != nil || st != WPStatusRunning {
		t.Errorf("expected running, got %s, err: %v", st, err)
	}

	for _, valid := range []string{"pending", "installing", "running", "stopped", "failed"} {
		res, err := NormalizeWPStatus(valid)
		if err != nil || res != valid {
			t.Errorf("expected %s, got %s, err: %v", valid, res, err)
		}
	}

	_, err = NormalizeWPStatus("invalid_status")
	if !errors.Is(err, ErrInvalidWPStatus) {
		t.Errorf("expected ErrInvalidWPStatus, got %v", err)
	}

	// 2. Health Status
	hs, err := NormalizeWPHealthStatus("")
	if err != nil || hs != WPHealthStatusUnknown {
		t.Errorf("expected default unknown, got %s, err: %v", hs, err)
	}

	hs, err = NormalizeWPHealthStatus("HEALTHY")
	if err != nil || hs != WPHealthStatusHealthy {
		t.Errorf("expected healthy, got %s, err: %v", hs, err)
	}

	for _, valid := range []string{"unknown", "healthy", "unhealthy"} {
		res, err := NormalizeWPHealthStatus(valid)
		if err != nil || res != valid {
			t.Errorf("expected %s, got %s, err: %v", valid, res, err)
		}
	}

	_, err = NormalizeWPHealthStatus("broken")
	if !errors.Is(err, ErrInvalidWPHealthStatus) {
		t.Errorf("expected ErrInvalidWPHealthStatus, got %v", err)
	}

	// 3. Plugin / Theme Kind
	k, err := NormalizePluginThemeKind("PLUGIN")
	if err != nil || k != PluginThemeKindPlugin {
		t.Errorf("expected plugin, got %s, err: %v", k, err)
	}

	k, err = NormalizePluginThemeKind("theme")
	if err != nil || k != PluginThemeKindTheme {
		t.Errorf("expected theme, got %s, err: %v", k, err)
	}

	_, err = NormalizePluginThemeKind("widget")
	if !errors.Is(err, ErrInvalidPluginThemeKind) {
		t.Errorf("expected ErrInvalidPluginThemeKind, got %v", err)
	}

	// 4. Webmaster Action Type
	act, err := NormalizeWebmasterActionType("INJECT_SCRIPT")
	if err != nil || act != WebmasterActionInjectScript {
		t.Errorf("expected inject_script, got %s, err: %v", act, err)
	}

	for _, valid := range []string{"inject_script", "config_tweak", "install_asset"} {
		res, err := NormalizeWebmasterActionType(valid)
		if err != nil || res != valid {
			t.Errorf("expected %s, got %s, err: %v", valid, res, err)
		}
	}

	_, err = NormalizeWebmasterActionType("delete_database")
	if !errors.Is(err, ErrInvalidWebmasterActionType) {
		t.Errorf("expected ErrInvalidWebmasterActionType, got %v", err)
	}
}

func TestWordPress_DTOAndSecurity(t *testing.T) {
	dep := &WordPressDeployment{
		ID:                     "wp-dep-001",
		ProjectID:              "proj-001",
		SiteTitle:              "Cuan Creator Blog",
		AdminEmail:             "admin@cuan.co",
		AdminUsername:          "cuanadmin",
		AdminPasswordEncrypted: "enc_aes256gcm_admin_pass_cipher",
		DBName:                 "wp_cuan",
		DBUser:                 "wp_user",
		DBPasswordEncrypted:    "enc_aes256gcm_db_pass_cipher",
		DBHost:                 "127.0.0.1",
		DBPort:                 3306,
		Domain:                 "blog.cuan.co",
		Port:                   8086,
		TablePrefix:            "wp_",
		WPVersion:              "6.7.1",
		Status:                 "running",
		HealthStatus:           "healthy",
		PermalinkStructure:     "/%postname%/",
		CreatedAt:              time.Now().UTC(),
		UpdatedAt:              time.Now().UTC(),
	}

	dto := dep.ToDTO()
	if dto.ID != dep.ID || dto.ProjectID != dep.ProjectID || dto.SiteTitle != dep.SiteTitle {
		t.Errorf("unexpected DTO values: %+v", dto)
	}
	if !dto.HasAdminPassword || !dto.HasDBPassword {
		t.Errorf("expected HasAdminPassword and HasDBPassword to be true")
	}
	if dto.Domain != "blog.cuan.co" || dto.Port != 8086 || dto.Status != "running" || dto.HealthStatus != "healthy" {
		t.Errorf("unexpected DTO status or domain: %+v", dto)
	}

	// Verify JSON serialization hides sensitive encrypted passwords
	bytes, err := json.Marshal(dep)
	if err != nil {
		t.Fatalf("failed to marshal WordPressDeployment: %v", err)
	}
	var rawMap map[string]interface{}
	if err := json.Unmarshal(bytes, &rawMap); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if _, exists := rawMap["admin_password_encrypted"]; exists {
		t.Errorf("admin_password_encrypted MUST NOT be exposed in JSON output")
	}
	if _, exists := rawMap["db_password_encrypted"]; exists {
		t.Errorf("db_password_encrypted MUST NOT be exposed in JSON output")
	}
}

func TestWordPress_AES256GCMEncryption(t *testing.T) {
	secretKey := "cuan-wp-vault-super-secret-key-32b"

	dep := &WordPressDeployment{
		ID:        "wp-dep-vault",
		ProjectID: "proj-vault",
	}

	plainAdminPass := "Sup3r!Secure#WP@Admin!2026"
	plainDBPass := "MySQL!Cuan$DB@Strong#Pass"

	// 1. Admin Password Encryption & Decryption
	err := dep.EncryptAdminPassword(plainAdminPass, secretKey)
	if err != nil {
		t.Fatalf("EncryptAdminPassword failed: %v", err)
	}
	if dep.AdminPasswordEncrypted == "" || dep.AdminPasswordEncrypted == plainAdminPass {
		t.Fatalf("AdminPasswordEncrypted should be non-empty ciphertext, got: %s", dep.AdminPasswordEncrypted)
	}

	decryptedAdmin, err := dep.DecryptAdminPassword(secretKey)
	if err != nil {
		t.Fatalf("DecryptAdminPassword failed: %v", err)
	}
	if decryptedAdmin != plainAdminPass {
		t.Fatalf("expected admin password %s, got %s", plainAdminPass, decryptedAdmin)
	}

	// 2. Database Password Encryption & Decryption
	err = dep.EncryptDBPassword(plainDBPass, secretKey)
	if err != nil {
		t.Fatalf("EncryptDBPassword failed: %v", err)
	}
	if dep.DBPasswordEncrypted == "" || dep.DBPasswordEncrypted == plainDBPass {
		t.Fatalf("DBPasswordEncrypted should be non-empty ciphertext, got: %s", dep.DBPasswordEncrypted)
	}

	decryptedDB, err := dep.DecryptDBPassword(secretKey)
	if err != nil {
		t.Fatalf("DecryptDBPassword failed: %v", err)
	}
	if decryptedDB != plainDBPass {
		t.Fatalf("expected db password %s, got %s", plainDBPass, decryptedDB)
	}

	// 3. Decryption with invalid secret must fail
	_, err = dep.DecryptAdminPassword("wrong-secret-key-1234567890123456")
	if err == nil {
		t.Errorf("expected error when decrypting with wrong secret, got nil")
	}
	_, err = dep.DecryptDBPassword("wrong-secret-key-1234567890123456")
	if err == nil {
		t.Errorf("expected error when decrypting db password with wrong secret, got nil")
	}

	// 4. Empty passwords
	err = dep.EncryptAdminPassword("", secretKey)
	if err != nil || dep.AdminPasswordEncrypted != "" {
		t.Errorf("expected empty ciphertext for empty password, got: %s, err: %v", dep.AdminPasswordEncrypted, err)
	}
	emptyDecAdmin, err := dep.DecryptAdminPassword(secretKey)
	if err != nil || emptyDecAdmin != "" {
		t.Errorf("expected empty string on empty admin pass decrypt, got: %s, err: %v", emptyDecAdmin, err)
	}

	err = dep.EncryptDBPassword("", secretKey)
	if err != nil || dep.DBPasswordEncrypted != "" {
		t.Errorf("expected empty ciphertext for empty db password, got: %s, err: %v", dep.DBPasswordEncrypted, err)
	}
	emptyDecDB, err := dep.DecryptDBPassword(secretKey)
	if err != nil || emptyDecDB != "" {
		t.Errorf("expected empty string on empty db pass decrypt, got: %s, err: %v", emptyDecDB, err)
	}
}

func TestMemoryWordPressRepository_Deployments(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryWordPressRepository()

	// 1. Validation errors
	err := repo.CreateDeployment(ctx, nil)
	if err == nil {
		t.Errorf("expected error on nil deployment")
	}

	err = repo.CreateDeployment(ctx, &WordPressDeployment{})
	if !errors.Is(err, ErrEmptyWordPressProjectID) {
		t.Errorf("expected ErrEmptyWordPressProjectID, got %v", err)
	}

	err = repo.CreateDeployment(ctx, &WordPressDeployment{ProjectID: "proj-1"})
	if !errors.Is(err, ErrEmptyWordPressSiteTitle) {
		t.Errorf("expected ErrEmptyWordPressSiteTitle, got %v", err)
	}

	err = repo.CreateDeployment(ctx, &WordPressDeployment{ProjectID: "proj-1", SiteTitle: "Site"})
	if !errors.Is(err, ErrEmptyWordPressAdminEmail) {
		t.Errorf("expected ErrEmptyWordPressAdminEmail, got %v", err)
	}

	err = repo.CreateDeployment(ctx, &WordPressDeployment{ProjectID: "proj-1", SiteTitle: "Site", AdminEmail: "a@b.com"})
	if !errors.Is(err, ErrEmptyWordPressAdminUsername) {
		t.Errorf("expected ErrEmptyWordPressAdminUsername, got %v", err)
	}

	err = repo.CreateDeployment(ctx, &WordPressDeployment{ProjectID: "proj-1", SiteTitle: "Site", AdminEmail: "a@b.com", AdminUsername: "admin"})
	if !errors.Is(err, ErrEmptyWordPressDomain) {
		t.Errorf("expected ErrEmptyWordPressDomain, got %v", err)
	}

	// 2. Successful creation
	dep := &WordPressDeployment{
		ProjectID:              "proj-001",
		SiteTitle:              "My WordPress Store",
		AdminEmail:             "buchin@dropsugar.com",
		AdminUsername:          "buchin",
		AdminPasswordEncrypted: "enc_admin_pass",
		DBName:                 "wp_store",
		DBUser:                 "wp_user",
		DBPasswordEncrypted:    "enc_db_pass",
		Domain:                 "store.cuan.co",
		Port:                   8087,
		Status:                 WPStatusInstalling,
		HealthStatus:           WPHealthStatusUnknown,
	}

	err = repo.CreateDeployment(ctx, dep)
	if err != nil {
		t.Fatalf("CreateDeployment failed: %v", err)
	}
	if dep.ID == "" {
		t.Fatalf("expected ID to be populated by UUID")
	}
	if dep.DBHost != "127.0.0.1" || dep.DBPort != 3306 || dep.TablePrefix != "wp_" || dep.WPVersion != "latest" {
		t.Errorf("expected defaults applied to deployment: %+v", dep)
	}

	// 3. GetByID
	fetched, err := repo.GetDeploymentByID(ctx, dep.ID)
	if err != nil {
		t.Fatalf("GetDeploymentByID failed: %v", err)
	}
	if fetched.ID != dep.ID || fetched.SiteTitle != "My WordPress Store" {
		t.Errorf("fetched deployment mismatch: %+v", fetched)
	}

	// Non-existent ID
	_, err = repo.GetDeploymentByID(ctx, "non-existent-id")
	if !errors.Is(err, ErrWordPressDeploymentNotFound) {
		t.Errorf("expected ErrWordPressDeploymentNotFound, got %v", err)
	}

	// 4. GetByProjectID
	byProj, err := repo.GetDeploymentByProjectID(ctx, "proj-001")
	if err != nil {
		t.Fatalf("GetDeploymentByProjectID failed: %v", err)
	}
	if byProj.ID != dep.ID {
		t.Errorf("expected deployment id %s, got %s", dep.ID, byProj.ID)
	}

	_, err = repo.GetDeploymentByProjectID(ctx, "non-existent-proj")
	if !errors.Is(err, ErrWordPressDeploymentNotFound) {
		t.Errorf("expected ErrWordPressDeploymentNotFound, got %v", err)
	}

	// 5. Update Status & Health
	err = repo.UpdateDeploymentStatus(ctx, dep.ID, WPStatusRunning)
	if err != nil {
		t.Fatalf("UpdateDeploymentStatus failed: %v", err)
	}
	err = repo.UpdateDeploymentHealth(ctx, dep.ID, WPHealthStatusHealthy)
	if err != nil {
		t.Fatalf("UpdateDeploymentHealth failed: %v", err)
	}

	updated, err := repo.GetDeploymentByID(ctx, dep.ID)
	if err != nil {
		t.Fatalf("GetDeploymentByID failed: %v", err)
	}
	if updated.Status != WPStatusRunning || updated.HealthStatus != WPHealthStatusHealthy {
		t.Errorf("expected running/healthy, got %s/%s", updated.Status, updated.HealthStatus)
	}

	// 6. Full Update
	updated.SiteTitle = "My Renamed WordPress Store"
	err = repo.UpdateDeployment(ctx, updated)
	if err != nil {
		t.Fatalf("UpdateDeployment failed: %v", err)
	}
	refetched, _ := repo.GetDeploymentByID(ctx, dep.ID)
	if refetched.SiteTitle != "My Renamed WordPress Store" {
		t.Errorf("expected renamed site title, got: %s", refetched.SiteTitle)
	}

	// 7. List Deployments
	list, err := repo.ListDeployments(ctx)
	if err != nil || len(list) != 1 {
		t.Errorf("expected 1 deployment in list, got %d, err: %v", len(list), err)
	}

	byProjList, err := repo.ListDeploymentsByProjectID(ctx, "proj-001")
	if err != nil || len(byProjList) != 1 {
		t.Errorf("expected 1 deployment for project, got %d, err: %v", len(byProjList), err)
	}

	// 8. Delete
	err = repo.DeleteDeployment(ctx, dep.ID)
	if err != nil {
		t.Fatalf("DeleteDeployment failed: %v", err)
	}
	_, err = repo.GetDeploymentByID(ctx, dep.ID)
	if !errors.Is(err, ErrWordPressDeploymentNotFound) {
		t.Errorf("expected ErrWordPressDeploymentNotFound after delete, got %v", err)
	}
}

func TestMemoryWordPressRepository_PluginsThemes(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryWordPressRepository()

	dep := &WordPressDeployment{
		ProjectID:     "proj-002",
		SiteTitle:     "Plugin Test Site",
		AdminEmail:    "admin@cuan.co",
		AdminUsername: "admin",
		Domain:        "plugintest.cuan.co",
		Port:          8088,
	}
	if err := repo.CreateDeployment(ctx, dep); err != nil {
		t.Fatalf("create deployment failed: %v", err)
	}

	// 1. Validation errors
	err := repo.CreatePluginTheme(ctx, nil)
	if err == nil {
		t.Errorf("expected error on nil plugin/theme")
	}

	err = repo.CreatePluginTheme(ctx, &WordPressPluginTheme{})
	if !errors.Is(err, ErrEmptyWordPressDeploymentID) {
		t.Errorf("expected ErrEmptyWordPressDeploymentID, got %v", err)
	}

	err = repo.CreatePluginTheme(ctx, &WordPressPluginTheme{DeploymentID: dep.ID})
	if !errors.Is(err, ErrEmptyWordPressPluginSlug) {
		t.Errorf("expected ErrEmptyWordPressPluginSlug, got %v", err)
	}

	err = repo.CreatePluginTheme(ctx, &WordPressPluginTheme{DeploymentID: dep.ID, Slug: "astra"})
	if !errors.Is(err, ErrEmptyWordPressPluginName) {
		t.Errorf("expected ErrEmptyWordPressPluginName, got %v", err)
	}

	err = repo.CreatePluginTheme(ctx, &WordPressPluginTheme{DeploymentID: dep.ID, Slug: "astra", Name: "Astra Theme", Kind: "invalid"})
	if !errors.Is(err, ErrInvalidPluginThemeKind) {
		t.Errorf("expected ErrInvalidPluginThemeKind, got %v", err)
	}

	// 2. Add Theme
	theme := &WordPressPluginTheme{
		DeploymentID: dep.ID,
		Kind:         PluginThemeKindTheme,
		Slug:         "astra",
		Name:         "Astra Multi-purpose Theme",
		Version:      "4.8.2",
		IsActive:     true,
		SourceZip:    "astra.4.8.2.zip",
	}
	if err := repo.CreatePluginTheme(ctx, theme); err != nil {
		t.Fatalf("create theme failed: %v", err)
	}
	if theme.ID == "" {
		t.Errorf("expected theme ID to be generated")
	}

	// 3. Add Plugin
	plugin := &WordPressPluginTheme{
		DeploymentID: dep.ID,
		Kind:         PluginThemeKindPlugin,
		Slug:         "woocommerce",
		Name:         "WooCommerce eCommerce",
		Version:      "9.4.0",
		IsActive:     true,
		SourceZip:    "woocommerce.9.4.0.zip",
	}
	if err := repo.CreatePluginTheme(ctx, plugin); err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}

	// 4. GetByID
	fetchedTheme, err := repo.GetPluginThemeByID(ctx, theme.ID)
	if err != nil || fetchedTheme.Slug != "astra" {
		t.Errorf("expected fetched theme astra, got %+v, err: %v", fetchedTheme, err)
	}

	// 5. List By Deployment
	allAssets, err := repo.ListPluginsThemesByDeploymentID(ctx, dep.ID, "")
	if err != nil || len(allAssets) != 2 {
		t.Errorf("expected 2 assets, got %d, err: %v", len(allAssets), err)
	}

	themesOnly, err := repo.ListPluginsThemesByDeploymentID(ctx, dep.ID, "theme")
	if err != nil || len(themesOnly) != 1 || themesOnly[0].Slug != "astra" {
		t.Errorf("expected 1 theme 'astra', got %d, err: %v", len(themesOnly), err)
	}

	pluginsOnly, err := repo.ListPluginsThemesByDeploymentID(ctx, dep.ID, "plugin")
	if err != nil || len(pluginsOnly) != 1 || pluginsOnly[0].Slug != "woocommerce" {
		t.Errorf("expected 1 plugin 'woocommerce', got %d, err: %v", len(pluginsOnly), err)
	}

	// 6. Update Status (deactivate plugin)
	err = repo.UpdatePluginThemeStatus(ctx, plugin.ID, false)
	if err != nil {
		t.Fatalf("UpdatePluginThemeStatus failed: %v", err)
	}
	updatedPlugin, _ := repo.GetPluginThemeByID(ctx, plugin.ID)
	if updatedPlugin.IsActive {
		t.Errorf("expected plugin to be deactivated")
	}

	// 7. Delete Theme
	err = repo.DeletePluginTheme(ctx, theme.ID)
	if err != nil {
		t.Fatalf("DeletePluginTheme failed: %v", err)
	}
	_, err = repo.GetPluginThemeByID(ctx, theme.ID)
	if !errors.Is(err, ErrWordPressPluginThemeNotFound) {
		t.Errorf("expected ErrWordPressPluginThemeNotFound, got %v", err)
	}

	// 8. Cascade delete on deployment delete
	err = repo.DeleteDeployment(ctx, dep.ID)
	if err != nil {
		t.Fatalf("DeleteDeployment failed: %v", err)
	}
	_, err = repo.GetPluginThemeByID(ctx, plugin.ID)
	if !errors.Is(err, ErrWordPressPluginThemeNotFound) {
		t.Errorf("expected remaining plugin to be cascade-deleted with deployment, got %v", err)
	}
}

func TestMemoryWordPressRepository_WebmasterActions(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryWordPressRepository()

	dep := &WordPressDeployment{
		ProjectID:     "proj-003",
		SiteTitle:     "Webmaster Test Site",
		AdminEmail:    "admin@cuan.co",
		AdminUsername: "admin",
		Domain:        "webmastertest.cuan.co",
		Port:          8089,
	}
	if err := repo.CreateDeployment(ctx, dep); err != nil {
		t.Fatalf("create deployment failed: %v", err)
	}

	// 1. Validation errors
	err := repo.RecordWebmasterAction(ctx, nil)
	if err == nil {
		t.Errorf("expected error on nil action")
	}

	err = repo.RecordWebmasterAction(ctx, &WordPressWebmasterAction{})
	if !errors.Is(err, ErrEmptyWordPressDeploymentID) {
		t.Errorf("expected ErrEmptyWordPressDeploymentID, got %v", err)
	}

	err = repo.RecordWebmasterAction(ctx, &WordPressWebmasterAction{
		DeploymentID: dep.ID,
		ActionType:   "unsupported_action",
	})
	if !errors.Is(err, ErrInvalidWebmasterActionType) {
		t.Errorf("expected ErrInvalidWebmasterActionType, got %v", err)
	}

	// 2. Record Script Injection Action
	actionPayload := map[string]interface{}{
		"target":   "header",
		"tag_name": "google_analytics_ga4",
		"code":     "<script async src='https://www.googletagmanager.com/gtag/js?id=G-TEST'></script>",
	}
	payloadBytes, _ := json.Marshal(actionPayload)

	act1 := &WordPressWebmasterAction{
		DeploymentID: dep.ID,
		ActionType:   WebmasterActionInjectScript,
		Payload:      payloadBytes,
	}
	if err := repo.RecordWebmasterAction(ctx, act1); err != nil {
		t.Fatalf("RecordWebmasterAction failed: %v", err)
	}
	if act1.ID == "" {
		t.Errorf("expected action ID to be generated")
	}

	// 3. Record Config Tweak Action
	act2 := &WordPressWebmasterAction{
		DeploymentID: dep.ID,
		ActionType:   WebmasterActionConfigTweak,
		Payload:      json.RawMessage(`{"constant":"WP_MEMORY_LIMIT","value":"256M"}`),
	}
	if err := repo.RecordWebmasterAction(ctx, act2); err != nil {
		t.Fatalf("RecordWebmasterAction failed: %v", err)
	}

	// 4. GetByID
	fetchedAct, err := repo.GetWebmasterActionByID(ctx, act1.ID)
	if err != nil || fetchedAct.ActionType != WebmasterActionInjectScript {
		t.Errorf("expected inject_script action, got %+v, err: %v", fetchedAct, err)
	}

	// 5. List Actions
	actions, err := repo.ListWebmasterActionsByDeploymentID(ctx, dep.ID)
	if err != nil || len(actions) != 2 {
		t.Errorf("expected 2 actions, got %d, err: %v", len(actions), err)
	}

	// 6. Cascade delete on deployment delete
	err = repo.DeleteDeployment(ctx, dep.ID)
	if err != nil {
		t.Fatalf("DeleteDeployment failed: %v", err)
	}
	_, err = repo.GetWebmasterActionByID(ctx, act1.ID)
	if !errors.Is(err, ErrWordPressWebmasterActionNotFound) {
		t.Errorf("expected action to be cascade-deleted, got %v", err)
	}
}

func TestMemoryWordPressRepository_ConcurrencyThreadSafety(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryWordPressRepository()

	var wg sync.WaitGroup
	numRoutines := 30

	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			depID := fmt.Sprintf("dep-concur-%d", idx)
			dep := &WordPressDeployment{
				ID:            depID,
				ProjectID:     fmt.Sprintf("proj-concur-%d", idx%5),
				SiteTitle:     fmt.Sprintf("Concurrent WP %d", idx),
				AdminEmail:    fmt.Sprintf("admin%d@cuan.co", idx),
				AdminUsername: fmt.Sprintf("admin%d", idx),
				Domain:        fmt.Sprintf("site%d.cuan.co", idx),
				Port:          8100 + idx,
			}
			if err := repo.CreateDeployment(ctx, dep); err != nil {
				t.Errorf("concurrent CreateDeployment failed: %v", err)
			}

			// Read
			_, _ = repo.GetDeploymentByID(ctx, depID)
			_, _ = repo.ListDeployments(ctx)
			_, _ = repo.ListDeploymentsByProjectID(ctx, dep.ProjectID)

			// Update status
			_ = repo.UpdateDeploymentStatus(ctx, depID, WPStatusRunning)
			_ = repo.UpdateDeploymentHealth(ctx, depID, WPHealthStatusHealthy)

			// Add Plugin
			ptID := fmt.Sprintf("pt-concur-%d", idx)
			pt := &WordPressPluginTheme{
				ID:           ptID,
				DeploymentID: depID,
				Kind:         PluginThemeKindPlugin,
				Slug:         fmt.Sprintf("plugin-%d", idx),
				Name:         fmt.Sprintf("Plugin Name %d", idx),
				IsActive:     true,
			}
			_ = repo.CreatePluginTheme(ctx, pt)
			_, _ = repo.GetPluginThemeByID(ctx, ptID)
			_, _ = repo.ListPluginsThemesByDeploymentID(ctx, depID, "")

			// Record Action
			act := &WordPressWebmasterAction{
				DeploymentID: depID,
				ActionType:   WebmasterActionInjectScript,
				Payload:      json.RawMessage(`{"script":"console.log('hi')"}`),
			}
			_ = repo.RecordWebmasterAction(ctx, act)
			_, _ = repo.ListWebmasterActionsByDeploymentID(ctx, depID)
		}(i)
	}

	wg.Wait()

	list, err := repo.ListDeployments(ctx)
	if err != nil {
		t.Fatalf("ListDeployments failed after concurrency: %v", err)
	}
	if len(list) != numRoutines {
		t.Errorf("expected %d deployments after concurrent inserts, got %d", numRoutines, len(list))
	}
}

func TestRepositories_WordPressIntegration(t *testing.T) {
	memRepos := NewMemoryRepositories()
	if memRepos.WordPress == nil {
		t.Fatalf("expected memRepos.WordPress to be initialized")
	}

	ctx := context.Background()
	dep := &WordPressDeployment{
		ProjectID:     "proj-repo-integ",
		SiteTitle:     "Repository Integration Test",
		AdminEmail:    "test@cuan.co",
		AdminUsername: "testadmin",
		Domain:        "integ.cuan.co",
		Port:          8099,
	}
	if err := memRepos.WordPress.CreateDeployment(ctx, dep); err != nil {
		t.Fatalf("memRepos.WordPress.CreateDeployment failed: %v", err)
	}

	fetched, err := memRepos.WordPress.GetDeploymentByID(ctx, dep.ID)
	if err != nil {
		t.Fatalf("memRepos.WordPress.GetDeploymentByID failed: %v", err)
	}
	if fetched.SiteTitle != "Repository Integration Test" {
		t.Errorf("unexpected site title: %s", fetched.SiteTitle)
	}
}
