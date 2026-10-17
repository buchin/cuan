package wordpress

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	cuanDB "github.com/buchin/cuan/internal/db"
)

// Helper to create an in-memory zip archive from filename -> content map.
func createTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write zip entry %s: %v", name, err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}
	return buf.Bytes()
}

func TestParseHeaders_ThemeAndPlugin(t *testing.T) {
	// 1. Theme style.css
	styleCSS := `/*
Theme Name: Astra Pro
Theme URI: https://wpastra.com/
Author: Brainstorm Force
Author URI: https://brainstormforce.com
Description: The most popular, fastest, beautiful and easily customizable WordPress theme.
Version: 4.8.2
Text Domain: astra
*/
body { font-family: sans-serif; }
`
	themeHeader, err := ParseThemeHeader(styleCSS)
	if err != nil {
		t.Fatalf("ParseThemeHeader failed: %v", err)
	}
	if themeHeader.Name != "Astra Pro" {
		t.Errorf("expected Theme Name 'Astra Pro', got %q", themeHeader.Name)
	}
	if themeHeader.Version != "4.8.2" {
		t.Errorf("expected Version '4.8.2', got %q", themeHeader.Version)
	}
	if themeHeader.Slug != "astra-pro" {
		t.Errorf("expected Slug 'astra-pro', got %q", themeHeader.Slug)
	}

	// 2. Plugin PHP
	pluginPHP := `<?php
/**
 * Plugin Name: Cuan Analytics Pro
 * Plugin URI: https://cuan.dojo.cc
 * Description: High-speed analytics tracker for turnkey WordPress cloud.
 * Version: 2.1.0
 * Author: Dropsugar
 * Author URI: https://dropsugar.com
 * License: GPLv2
 */

if (!defined('ABSPATH')) exit;
`
	pluginHeader, err := ParsePluginHeader(pluginPHP)
	if err != nil {
		t.Fatalf("ParsePluginHeader failed: %v", err)
	}
	if pluginHeader.Name != "Cuan Analytics Pro" {
		t.Errorf("expected Plugin Name 'Cuan Analytics Pro', got %q", pluginHeader.Name)
	}
	if pluginHeader.Version != "2.1.0" {
		t.Errorf("expected Version '2.1.0', got %q", pluginHeader.Version)
	}
	if pluginHeader.Slug != "cuan-analytics-pro" {
		t.Errorf("expected Slug 'cuan-analytics-pro', got %q", pluginHeader.Slug)
	}

	// 3. Invalid header detection
	invalidCSS := `body { background: #fff; }`
	if _, err := ParseThemeHeader(invalidCSS); err == nil {
		t.Errorf("expected error parsing theme header without Theme Name, got nil")
	}

	invalidPHP := `<?php echo "hello"; ?>`
	if _, err := ParsePluginHeader(invalidPHP); err == nil {
		t.Errorf("expected error parsing plugin header without Plugin Name, got nil")
	}
}

func TestZipSlip_Mitigation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-zipslip-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbRepo := cuanDB.NewMemoryWordPressRepository()
	engine := NewWebmasterEngine(WebmasterConfig{
		DBRepo:        dbRepo,
		BaseDeployDir: tempDir,
		MockMode:      true,
	})

	depID := "wp_test_zipslip"
	_ = os.MkdirAll(engine.TargetDir(depID), 0755)

	// Create malicious ZIP with path traversal
	maliciousFiles := map[string]string{
		"../../evil.php":          "<?php phpinfo(); ?>",
		"normal.txt":              "innocent file",
		"theme/style.css":         "/*\nTheme Name: Malicious Theme\nVersion: 1.0\n*/",
	}
	zipBytes := createTestZip(t, maliciousFiles)

	_, err = engine.InstallAsset(context.Background(), InstallAssetParams{
		DeploymentID: depID,
		ZipBytes:     zipBytes,
	})

	if err == nil {
		t.Fatalf("expected ZipSlip error on malicious zip extraction, got nil")
	}

	if !strings.Contains(err.Error(), "zip slip") && !strings.Contains(err.Error(), "escapes") {
		t.Errorf("expected error to mention zip slip or escape, got: %v", err)
	}

	// Verify evil file was NOT created outside target dir
	outsideFile := filepath.Join(tempDir, "evil.php")
	if _, statErr := os.Stat(outsideFile); !os.IsNotExist(statErr) {
		t.Errorf("SECURITY FAILURE: evil.php was written outside destination directory!")
	}
}

func TestAssetManager_ThemeAndPluginLifecycle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-asset-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbRepo := cuanDB.NewMemoryWordPressRepository()
	engine := NewWebmasterEngine(WebmasterConfig{
		DBRepo:        dbRepo,
		BaseDeployDir: tempDir,
		MockMode:      true,
	})

	depID := "wp_asset_lifecycle"
	_ = os.MkdirAll(engine.TargetDir(depID), 0755)

	// 1. Install Theme
	themeFiles := map[string]string{
		"generatepress/style.css": `/*
Theme Name: GeneratePress
Theme URI: https://generatepress.com
Author: Tom Usborne
Version: 3.5.1
Description: Fast, lightweight, and responsive WordPress theme.
*/
`,
		"generatepress/index.php": "<?php // GeneratePress index",
		"generatepress/functions.php": "<?php // GeneratePress functions",
	}
	themeZip := createTestZip(t, themeFiles)

	themeRes, err := engine.InstallAsset(context.Background(), InstallAssetParams{
		DeploymentID: depID,
		ZipBytes:     themeZip,
		Activate:     true,
	})
	if err != nil {
		t.Fatalf("failed to install theme: %v", err)
	}
	if themeRes.Header.Slug != "generatepress" {
		t.Errorf("expected theme slug 'generatepress', got %s", themeRes.Header.Slug)
	}
	if !themeRes.Activated {
		t.Errorf("expected theme to be activated")
	}

	// Verify files extracted to wp-content/themes/generatepress
	themeCSSPath := filepath.Join(engine.TargetDir(depID), "wp-content", "themes", "generatepress", "style.css")
	if _, statErr := os.Stat(themeCSSPath); os.IsNotExist(statErr) {
		t.Errorf("expected extracted style.css at %s", themeCSSPath)
	}

	// 2. Install Plugin
	pluginFiles := map[string]string{
		"wp-super-cache/wp-cache.php": `<?php
/**
 * Plugin Name: WP Super Cache
 * Plugin URI: https://wordpress.org/plugins/wp-super-cache/
 * Description: Very fast caching engine for WordPress.
 * Version: 1.12.4
 * Author: Automattic
 */
`,
		"wp-super-cache/readme.txt": "WP Super Cache readme",
	}
	pluginZip := createTestZip(t, pluginFiles)

	pluginRes, err := engine.InstallAsset(context.Background(), InstallAssetParams{
		DeploymentID: depID,
		ZipBytes:     pluginZip,
		Activate:     true,
	})
	if err != nil {
		t.Fatalf("failed to install plugin: %v", err)
	}
	if pluginRes.Header.Slug != "wp-super-cache" {
		t.Errorf("expected plugin slug 'wp-super-cache', got %s", pluginRes.Header.Slug)
	}

	// Verify files extracted to wp-content/plugins/wp-super-cache
	pluginPHPPath := filepath.Join(engine.TargetDir(depID), "wp-content", "plugins", "wp-super-cache", "wp-cache.php")
	if _, statErr := os.Stat(pluginPHPPath); os.IsNotExist(statErr) {
		t.Errorf("expected extracted wp-cache.php at %s", pluginPHPPath)
	}

	// 3. List Assets via Engine
	assets, err := engine.ListAssets(context.Background(), depID, "")
	if err != nil {
		t.Fatalf("failed to list assets: %v", err)
	}
	if len(assets) != 2 {
		t.Errorf("expected 2 assets in database, got %d", len(assets))
	}

	// 4. Toggle Asset status
	pluginRecord := pluginRes.Asset
	err = engine.ToggleAssetStatus(context.Background(), pluginRecord.ID, false)
	if err != nil {
		t.Fatalf("failed to toggle plugin status: %v", err)
	}

	// 5. Delete Asset
	err = engine.DeleteAsset(context.Background(), depID, pluginRecord.ID)
	if err != nil {
		t.Fatalf("failed to delete plugin asset: %v", err)
	}
	if _, statErr := os.Stat(pluginPHPPath); !os.IsNotExist(statErr) {
		t.Errorf("expected plugin folder to be deleted after DeleteAsset")
	}

	// 6. Verify Webmaster Audit Actions
	actions, err := engine.ListActions(context.Background(), depID)
	if err != nil {
		t.Fatalf("failed to list webmaster actions: %v", err)
	}
	if len(actions) < 2 {
		t.Errorf("expected at least 2 webmaster actions recorded, got %d", len(actions))
	}
}

func TestScriptInjector_MuPlugins(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-script-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbRepo := cuanDB.NewMemoryWordPressRepository()
	engine := NewWebmasterEngine(WebmasterConfig{
		DBRepo:        dbRepo,
		BaseDeployDir: tempDir,
		MockMode:      true,
	})

	depID := "wp_script_test"
	_ = os.MkdirAll(engine.TargetDir(depID), 0755)

	// 1. Inject GA4 script in header
	ga4Snippet := BuildGA4Snippet("G-TEST12345")
	res1, err := engine.InjectScript(context.Background(), InjectScriptParams{
		DeploymentID: depID,
		Name:         "Google Analytics 4",
		Type:         ScriptTypeGA4,
		Position:     PositionHead,
		Content:      ga4Snippet,
	})
	if err != nil {
		t.Fatalf("failed to inject GA4 script: %v", err)
	}
	if res1.Script.Position != PositionHead {
		t.Errorf("expected position 'head', got %s", res1.Script.Position)
	}
	if res1.ScriptsCount != 1 {
		t.Errorf("expected 1 script, got %d", res1.ScriptsCount)
	}

	// 2. Inject Meta Pixel in footer
	pixelSnippet := BuildMetaPixelSnippet("1234567890")
	res2, err := engine.InjectScript(context.Background(), InjectScriptParams{
		DeploymentID: depID,
		Name:         "Meta Pixel",
		Type:         ScriptTypeMetaPixel,
		Position:     PositionFooter,
		Content:      pixelSnippet,
	})
	if err != nil {
		t.Fatalf("failed to inject Meta Pixel script: %v", err)
	}
	if res2.ScriptsCount != 2 {
		t.Errorf("expected 2 scripts, got %d", res2.ScriptsCount)
	}

	// Verify wp-content/mu-plugins/cuan-webmaster.php was written
	phpPath, jsonPath := GetMuPluginPaths(engine.TargetDir(depID))
	if _, statErr := os.Stat(phpPath); os.IsNotExist(statErr) {
		t.Errorf("expected mu-plugin PHP file at %s", phpPath)
	}
	if _, statErr := os.Stat(jsonPath); os.IsNotExist(statErr) {
		t.Errorf("expected mu-plugin JSON config at %s", jsonPath)
	}

	// 3. List Scripts
	scripts, err := engine.ListScripts(context.Background(), depID)
	if err != nil {
		t.Fatalf("failed to list scripts: %v", err)
	}
	if len(scripts) != 2 {
		t.Errorf("expected 2 scripts, got %d", len(scripts))
	}

	// 4. Toggle Script
	toggled, err := engine.ToggleScript(context.Background(), depID, res1.Script.ID, false)
	if err != nil {
		t.Fatalf("failed to toggle script: %v", err)
	}
	if toggled.Enabled {
		t.Errorf("expected script enabled=false")
	}

	// 5. Delete Script
	err = engine.DeleteScript(context.Background(), depID, res2.Script.ID)
	if err != nil {
		t.Fatalf("failed to delete script: %v", err)
	}

	remaining, _ := engine.ListScripts(context.Background(), depID)
	if len(remaining) != 1 {
		t.Errorf("expected 1 remaining script, got %d", len(remaining))
	}
}

func TestScriptInjector_RealPHPExecution_HTMLRendering(t *testing.T) {
	// Verify that PHP CLI can execute cuan-webmaster.php and renders hooks into HTML head/footer safely without WSOD
	phpBinary, err := exec.LookPath("php")
	if err != nil || phpBinary == "" {
		t.Skip("php CLI not found on system, skipping real PHP execution test")
	}

	tempDir, err := os.MkdirTemp("", "cuan-php-exec-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbRepo := cuanDB.NewMemoryWordPressRepository()
	engine := NewWebmasterEngine(WebmasterConfig{
		DBRepo:        dbRepo,
		BaseDeployDir: tempDir,
		MockMode:      false,
	})

	depID := "wp_php_render"
	_ = os.MkdirAll(engine.TargetDir(depID), 0755)

	// Inject head script
	_, err = engine.InjectScript(context.Background(), InjectScriptParams{
		DeploymentID: depID,
		Name:         "GTM Head Script",
		Type:         ScriptTypeGTM,
		Position:     PositionHead,
		Content:      `<script>console.log("GTM-HEAD-LOADED");</script>`,
	})
	if err != nil {
		t.Fatalf("failed to inject head script: %v", err)
	}

	// Inject footer script
	_, err = engine.InjectScript(context.Background(), InjectScriptParams{
		DeploymentID: depID,
		Name:         "Footer Chat Widget",
		Type:         ScriptTypeCustom,
		Position:     PositionFooter,
		Content:      `<script>console.log("FOOTER-CHAT-WIDGET");</script>`,
	})
	if err != nil {
		t.Fatalf("failed to inject footer script: %v", err)
	}

	// Create test PHP harness that simulates WordPress core actions (wp_head and wp_footer)
	harnessPHP := `<?php
define('ABSPATH', __DIR__ . '/');

$actions = array();
function add_action($tag, $callback, $priority = 10) {
    global $actions;
    $actions[$tag][] = $callback;
}
function do_action($tag) {
    global $actions;
    if (isset($actions[$tag])) {
        foreach ($actions[$tag] as $cb) {
            call_user_func($cb);
        }
    }
}

// Include our isolated mu-plugin
require_once __DIR__ . '/wp-content/mu-plugins/cuan-webmaster.php';

echo "<!DOCTYPE html><html><head>";
do_action('wp_head');
echo "</head><body><h1>Turnkey WordPress</h1>";
do_action('wp_footer');
echo "</body></html>";
`
	harnessPath := filepath.Join(engine.TargetDir(depID), "index.php")
	if err := os.WriteFile(harnessPath, []byte(harnessPHP), 0644); err != nil {
		t.Fatalf("failed to write harness PHP: %v", err)
	}

	// Run PHP CLI on harness
	cmd := exec.Command(phpBinary, harnessPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("real PHP execution failed: %s (%v)", string(out), err)
	}

	html := string(out)
	if !strings.Contains(html, "GTM-HEAD-LOADED") {
		t.Errorf("expected HTML head to contain injected GTM-HEAD-LOADED, got:\n%s", html)
	}
	if !strings.Contains(html, "FOOTER-CHAT-WIDGET") {
		t.Errorf("expected HTML footer to contain injected FOOTER-CHAT-WIDGET, got:\n%s", html)
	}
	if !strings.Contains(html, "cuan-webmaster: GTM Head Script") {
		t.Errorf("expected HTML comment tag for GTM Head Script, got:\n%s", html)
	}
}

func TestConfigTweaker_SuccessAndBackup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbRepo := cuanDB.NewMemoryWordPressRepository()
	engine := NewWebmasterEngine(WebmasterConfig{
		DBRepo:        dbRepo,
		BaseDeployDir: tempDir,
		MockMode:      true,
	})

	depID := "wp_config_tweak"
	targetDir := engine.TargetDir(depID)

	// Write base wp-config.php
	if err := WriteWPConfig(targetDir, WPConfigParams{
		DBName:      "test_db",
		DBUser:      "test_user",
		DBPassword:  "test_pass",
		TablePrefix: "wp_",
		WPDebug:     false,
	}); err != nil {
		t.Fatalf("failed to write initial wp-config: %v", err)
	}

	// Apply tweaks: WP_DEBUG true, WP_MEMORY_LIMIT 512M, WP_POST_REVISIONS 5
	tweaks := map[string]interface{}{
		"WP_DEBUG":          true,
		"WP_DEBUG_LOG":      true,
		"WP_DEBUG_DISPLAY":  false,
		"WP_MEMORY_LIMIT":   "512M",
		"WP_POST_REVISIONS": 5,
	}

	res, err := engine.TweakConfig(context.Background(), depID, tweaks, nil)
	if err != nil {
		t.Fatalf("TweakConfig failed: %v", err)
	}

	if !res.SyntaxValid {
		t.Errorf("expected SyntaxValid=true")
	}

	// Verify backup file created
	if _, statErr := os.Stat(res.BackupPath); os.IsNotExist(statErr) {
		t.Errorf("expected backup file at %s", res.BackupPath)
	}

	// Verify constants in wp-config.php
	activeTweaks, err := engine.GetConfigTweaks(context.Background(), depID)
	if err != nil {
		t.Fatalf("GetConfigTweaks failed: %v", err)
	}

	if activeTweaks["WP_DEBUG"] != true {
		t.Errorf("expected WP_DEBUG=true, got %v", activeTweaks["WP_DEBUG"])
	}
	if activeTweaks["WP_MEMORY_LIMIT"] != "512M" {
		t.Errorf("expected WP_MEMORY_LIMIT='512M', got %v", activeTweaks["WP_MEMORY_LIMIT"])
	}

	// Verify Webmaster Action in audit log
	actions, err := engine.ListActions(context.Background(), depID)
	if err != nil {
		t.Fatalf("ListActions failed: %v", err)
	}
	if len(actions) == 0 {
		t.Errorf("expected at least 1 webmaster action for config tweak")
	}
	if actions[0].ActionType != cuanDB.WebmasterActionConfigTweak {
		t.Errorf("expected action type 'config_tweak', got %s", actions[0].ActionType)
	}
}

func TestConfigTweaker_PreSaveSyntaxValidation_WSODPrevention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-syntax-wsod-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbRepo := cuanDB.NewMemoryWordPressRepository()
	engine := NewWebmasterEngine(WebmasterConfig{
		DBRepo:        dbRepo,
		BaseDeployDir: tempDir,
		MockMode:      true,
	})

	depID := "wp_syntax_prevent"
	targetDir := engine.TargetDir(depID)

	if err := WriteWPConfig(targetDir, WPConfigParams{
		DBName:      "test_db",
		DBUser:      "test_user",
		DBPassword:  "test_pass",
		TablePrefix: "wp_",
		WPDebug:     false,
	}); err != nil {
		t.Fatalf("failed to write initial wp-config: %v", err)
	}

	configPath := filepath.Join(targetDir, "wp-config.php")
	originalContent, _ := os.ReadFile(configPath)

	// Attempt tweak with intentional PHP syntax error (unbalanced braces or invalid syntax)
	badDirectives := []string{
		"define( 'BROKEN_CONST', 'unclosed string ;",
		"if ( { function broken() }",
	}

	_, err = engine.TweakConfig(context.Background(), depID, nil, badDirectives)
	if err == nil {
		t.Fatalf("expected PHP syntax validation error, got nil!")
	}

	if !strings.Contains(err.Error(), "syntax") && !strings.Contains(err.Error(), "validation") {
		t.Errorf("expected error message to mention syntax validation, got: %v", err)
	}

	// CRITICAL VERIFICATION: wp-config.php MUST remain 100% UNTOUCHED!
	currentContent, _ := os.ReadFile(configPath)
	if !bytes.Equal(originalContent, currentContent) {
		t.Fatalf("SECURITY/STABILITY FAILURE: wp-config.php was modified despite syntax validation failure! WSOD risk detected.")
	}
}
