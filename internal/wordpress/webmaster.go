package wordpress

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	cuanDB "github.com/buchin/cuan/internal/db"
)

// WebmasterConfig provides dependencies and directories for the AI Webmaster Engine.
type WebmasterConfig struct {
	DBRepo        cuanDB.WordPressRepository
	BaseDeployDir string
	MockMode      bool
}

// InstallAssetParams defines options for installing a theme or plugin ZIP.
type InstallAssetParams struct {
	DeploymentID  string          `json:"deployment_id"`
	SourceZipPath string          `json:"source_zip_path,omitempty"`
	ZipBytes      []byte          `json:"-"`
	ZipReader     io.ReaderAt     `json:"-"`
	ZipSize       int64           `json:"-"`
	Kind          string          `json:"kind,omitempty"` // "theme", "plugin", or auto-detect
	CustomSlug    string          `json:"custom_slug,omitempty"`
	Activate      bool            `json:"activate"`
}

// InstallAssetResult encapsulates the outcome of asset installation.
type InstallAssetResult struct {
	Asset         *cuanDB.WordPressPluginTheme `json:"asset"`
	Header        AssetHeaderInfo              `json:"header"`
	ExtractedPath string                       `json:"extracted_path"`
	ActionID      string                       `json:"action_id"`
	Activated     bool                         `json:"activated"`
	Logs          []string                     `json:"logs"`
}

// InjectScriptResult encapsulates the outcome of script injection.
type InjectScriptResult struct {
	Script       InjectedScript `json:"script"`
	ActionID     string         `json:"action_id"`
	ScriptsCount int            `json:"scripts_count"`
	MuPluginPath string         `json:"mu_plugin_path"`
	Logs         []string       `json:"logs"`
}

// WebmasterEngine coordinates AI Webmaster operations: ZIP asset installation with ZipSlip mitigation,
// isolated script injection via mu-plugins, and safe wp-config tweaking with PHP pre-save syntax validation.
type WebmasterEngine struct {
	cfg WebmasterConfig
	mu  sync.RWMutex
}

// NewWebmasterEngine initializes a new AI Webmaster Engine.
func NewWebmasterEngine(cfg WebmasterConfig) *WebmasterEngine {
	if cfg.BaseDeployDir == "" {
		cfg.BaseDeployDir = filepath.Join("deployments", "wordpress")
	}
	if cfg.DBRepo == nil {
		cfg.DBRepo = cuanDB.NewMemoryWordPressRepository()
	}

	return &WebmasterEngine{
		cfg: cfg,
	}
}

// TargetDir returns the absolute disk directory for a deployment.
func (e *WebmasterEngine) TargetDir(deploymentID string) string {
	return filepath.Join(e.cfg.BaseDeployDir, deploymentID)
}

// -----------------------------------------------------------------------------
// Asset Manager (Theme & Plugin ZIPs)
// -----------------------------------------------------------------------------

// InstallAsset extracts a theme or plugin ZIP safely to wp-content/themes or wp-content/plugins,
// validates header information, mitigates ZipSlip attacks, and registers in database with audit action.
func (e *WebmasterEngine) InstallAsset(ctx context.Context, params InstallAssetParams) (*InstallAssetResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	depID := strings.TrimSpace(params.DeploymentID)
	if depID == "" {
		return nil, cuanDB.ErrEmptyWordPressDeploymentID
	}

	targetDir := e.TargetDir(depID)
	var logs []string
	log := func(msg string) {
		logs = append(logs, fmt.Sprintf("[%s] %s", time.Now().UTC().Format(time.RFC3339), msg))
	}

	// 1. Prepare Zip Reader
	var zr *zip.Reader
	var err error
	if len(params.ZipBytes) > 0 {
		zr, err = zip.NewReader(bytes.NewReader(params.ZipBytes), int64(len(params.ZipBytes)))
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidAssetZip, err)
		}
	} else if params.ZipReader != nil && params.ZipSize > 0 {
		zr, err = zip.NewReader(params.ZipReader, params.ZipSize)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidAssetZip, err)
		}
	} else if params.SourceZipPath != "" {
		file, err := os.Open(params.SourceZipPath)
		if err != nil {
			return nil, fmt.Errorf("open source zip: %w", err)
		}
		defer file.Close()
		fi, err := file.Stat()
		if err != nil {
			return nil, fmt.Errorf("stat source zip: %w", err)
		}
		zr, err = zip.NewReader(file, fi.Size())
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidAssetZip, err)
		}
	} else {
		return nil, errors.New("no zip archive provided (ZipBytes, ZipReader, or SourceZipPath required)")
	}

	// 2. Inspect ZIP: Detect Header (Theme style.css or Plugin PHP) & Check ZipSlip
	header, rootPrefix, err := InspectAssetZip(zr, params.Kind, params.CustomSlug)
	if err != nil {
		return nil, err
	}
	log(fmt.Sprintf("Detected valid %s: %q (Version: %s, Slug: %s)", header.Kind, header.Name, header.Version, header.Slug))

	// 3. Determine destination directory under wp-content
	var destBaseDir string
	if header.Kind == cuanDB.PluginThemeKindTheme {
		destBaseDir = filepath.Join(targetDir, "wp-content", "themes", header.Slug)
	} else {
		destBaseDir = filepath.Join(targetDir, "wp-content", "plugins", header.Slug)
	}

	log(fmt.Sprintf("Extracting %s files to %s (rootPrefix: %q)", header.Kind, destBaseDir, rootPrefix))

	// 4. Extract archive safely with canonical ZipSlip mitigation
	if err := ExtractAssetZip(zr, destBaseDir, rootPrefix); err != nil {
		return nil, fmt.Errorf("extract asset archive: %w", err)
	}
	log(fmt.Sprintf("Safely unpacked all asset files into %s", destBaseDir))

	// 5. Persist asset record in database
	sourceZipName := ""
	if params.SourceZipPath != "" {
		sourceZipName = filepath.Base(params.SourceZipPath)
	} else {
		sourceZipName = fmt.Sprintf("%s.zip", header.Slug)
	}

	assetRecord := &cuanDB.WordPressPluginTheme{
		ID:           fmt.Sprintf("asset_%s_%d", header.Slug, time.Now().Unix()),
		DeploymentID: depID,
		Kind:         header.Kind,
		Slug:         header.Slug,
		Name:         header.Name,
		Version:      header.Version,
		IsActive:     params.Activate,
		SourceZip:    sourceZipName,
		InstalledAt:  time.Now().UTC(),
	}

	if e.cfg.DBRepo != nil {
		// If activating a theme, optionally deactivate other themes in database
		if params.Activate && header.Kind == cuanDB.PluginThemeKindTheme {
			existingThemes, err := e.cfg.DBRepo.ListPluginsThemesByDeploymentID(ctx, depID, cuanDB.PluginThemeKindTheme)
			if err == nil {
				for _, ex := range existingThemes {
					if ex.IsActive && ex.Slug != header.Slug {
						_ = e.cfg.DBRepo.UpdatePluginThemeStatus(ctx, ex.ID, false)
					}
				}
			}
		}

		if err := e.cfg.DBRepo.CreatePluginTheme(ctx, assetRecord); err != nil {
			return nil, fmt.Errorf("record installed asset in db: %w", err)
		}
		log(fmt.Sprintf("Recorded %s asset %s in database (active: %v)", header.Kind, header.Slug, params.Activate))
	}

	// 6. Record Webmaster Action in audit log
	actionID := fmt.Sprintf("act_asset_%d", time.Now().UnixNano())
	payloadMap := map[string]interface{}{
		"asset_id":       assetRecord.ID,
		"kind":           header.Kind,
		"slug":           header.Slug,
		"name":           header.Name,
		"version":        header.Version,
		"activated":      params.Activate,
		"extracted_path": destBaseDir,
		"source_zip":     sourceZipName,
	}
	payloadJSON, _ := json.Marshal(payloadMap)

	if e.cfg.DBRepo != nil {
		actionRecord := &cuanDB.WordPressWebmasterAction{
			ID:           actionID,
			DeploymentID: depID,
			ActionType:   cuanDB.WebmasterActionInstallAsset,
			Payload:      payloadJSON,
			AppliedAt:    time.Now().UTC(),
		}
		_ = e.cfg.DBRepo.RecordWebmasterAction(ctx, actionRecord)
		log(fmt.Sprintf("Recorded Webmaster Audit Action %s (install_asset)", actionID))
	}

	return &InstallAssetResult{
		Asset:         assetRecord,
		Header:        *header,
		ExtractedPath: destBaseDir,
		ActionID:      actionID,
		Activated:     params.Activate,
		Logs:          logs,
	}, nil
}

// ListAssets returns installed plugins and themes for a deployment.
func (e *WebmasterEngine) ListAssets(ctx context.Context, deploymentID string, kind string) ([]*cuanDB.WordPressPluginTheme, error) {
	if e.cfg.DBRepo == nil {
		return []*cuanDB.WordPressPluginTheme{}, nil
	}
	return e.cfg.DBRepo.ListPluginsThemesByDeploymentID(ctx, deploymentID, kind)
}

// ToggleAssetStatus updates the active state of an installed plugin or theme.
func (e *WebmasterEngine) ToggleAssetStatus(ctx context.Context, assetID string, isActive bool) error {
	if e.cfg.DBRepo == nil {
		return nil
	}
	return e.cfg.DBRepo.UpdatePluginThemeStatus(ctx, assetID, isActive)
}

// DeleteAsset removes an asset from database and deletes its folder.
func (e *WebmasterEngine) DeleteAsset(ctx context.Context, deploymentID string, assetID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cfg.DBRepo == nil {
		return nil
	}

	pt, err := e.cfg.DBRepo.GetPluginThemeByID(ctx, assetID)
	if err != nil {
		return err
	}

	targetDir := e.TargetDir(deploymentID)
	var assetPath string
	if pt.Kind == cuanDB.PluginThemeKindTheme {
		assetPath = filepath.Join(targetDir, "wp-content", "themes", pt.Slug)
	} else {
		assetPath = filepath.Join(targetDir, "wp-content", "plugins", pt.Slug)
	}

	_ = os.RemoveAll(assetPath)
	return e.cfg.DBRepo.DeletePluginTheme(ctx, assetID)
}

// -----------------------------------------------------------------------------
// Safe Script Injector (mu-plugins)
// -----------------------------------------------------------------------------

// InjectScript safely registers a tracking or analytics script in wp-content/mu-plugins/cuan-webmaster.php.
// Scripts are rendered dynamically in wp_head or wp_footer, avoiding theme overwrites and eliminating WSOD risk.
func (e *WebmasterEngine) InjectScript(ctx context.Context, params InjectScriptParams) (*InjectScriptResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	depID := strings.TrimSpace(params.DeploymentID)
	if depID == "" {
		return nil, cuanDB.ErrEmptyWordPressDeploymentID
	}

	content := strings.TrimSpace(params.Content)
	if content == "" {
		return nil, ErrEmptyScriptContent
	}

	position := strings.ToLower(strings.TrimSpace(params.Position))
	if position == "" {
		position = PositionHead
	}
	if position != PositionHead && position != PositionFooter {
		return nil, ErrInvalidScriptPosition
	}

	name := strings.TrimSpace(params.Name)
	if name == "" {
		name = "Custom Script"
	}

	scriptType := strings.ToLower(strings.TrimSpace(params.Type))
	if scriptType == "" {
		scriptType = ScriptTypeCustom
	}

	enabled := true
	if params.Enabled != nil {
		enabled = *params.Enabled
	}

	targetDir := e.TargetDir(depID)

	// Load existing scripts
	existingScripts, err := LoadScriptsFromDisk(targetDir)
	if err != nil {
		return nil, fmt.Errorf("load existing scripts: %w", err)
	}

	scriptID := strings.TrimSpace(params.ID)
	if scriptID == "" {
		scriptID = fmt.Sprintf("script_%d", time.Now().UnixNano())
	}

	now := time.Now().UTC()
	targetScript := InjectedScript{
		ID:        scriptID,
		Name:      name,
		Type:      scriptType,
		Position:  position,
		Content:   content,
		Enabled:   enabled,
		CreatedAt: now,
		UpdatedAt: now,
	}

	found := false
	for i, s := range existingScripts {
		if s.ID == scriptID {
			targetScript.CreatedAt = s.CreatedAt
			existingScripts[i] = targetScript
			found = true
			break
		}
	}
	if !found {
		existingScripts = append(existingScripts, targetScript)
	}

	// Save to disk
	if err := SaveScriptsToDisk(targetDir, existingScripts); err != nil {
		return nil, fmt.Errorf("save scripts to disk: %w", err)
	}

	muPHPPath, _ := GetMuPluginPaths(targetDir)

	// Record Webmaster Action in audit trail
	actionID := fmt.Sprintf("act_script_%d", time.Now().UnixNano())
	payloadMap := map[string]interface{}{
		"script_id": targetScript.ID,
		"name":      targetScript.Name,
		"type":      targetScript.Type,
		"position":  targetScript.Position,
		"enabled":   targetScript.Enabled,
	}
	payloadJSON, _ := json.Marshal(payloadMap)

	if e.cfg.DBRepo != nil {
		actionRecord := &cuanDB.WordPressWebmasterAction{
			ID:           actionID,
			DeploymentID: depID,
			ActionType:   cuanDB.WebmasterActionInjectScript,
			Payload:      payloadJSON,
			AppliedAt:    time.Now().UTC(),
		}
		_ = e.cfg.DBRepo.RecordWebmasterAction(ctx, actionRecord)
	}

	return &InjectScriptResult{
		Script:       targetScript,
		ActionID:     actionID,
		ScriptsCount: len(existingScripts),
		MuPluginPath: muPHPPath,
		Logs: []string{
			fmt.Sprintf("Script %q injected into %s via isolated mu-plugin", targetScript.Name, targetScript.Position),
			fmt.Sprintf("Saved configuration to cuan-webmaster-scripts.json (%d active scripts)", len(existingScripts)),
		},
	}, nil
}

// ListScripts retrieves all injected scripts configured for a WordPress deployment.
func (e *WebmasterEngine) ListScripts(ctx context.Context, deploymentID string) ([]InjectedScript, error) {
	targetDir := e.TargetDir(deploymentID)
	return LoadScriptsFromDisk(targetDir)
}

// DeleteScript removes an injected script from a deployment.
func (e *WebmasterEngine) DeleteScript(ctx context.Context, deploymentID string, scriptID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	targetDir := e.TargetDir(deploymentID)
	existing, err := LoadScriptsFromDisk(targetDir)
	if err != nil {
		return err
	}

	var updated []InjectedScript
	found := false
	for _, s := range existing {
		if s.ID == scriptID {
			found = true
			continue
		}
		updated = append(updated, s)
	}

	if !found {
		return ErrScriptNotFound
	}

	return SaveScriptsToDisk(targetDir, updated)
}

// ToggleScript enables or disables an injected script.
func (e *WebmasterEngine) ToggleScript(ctx context.Context, deploymentID string, scriptID string, enabled bool) (*InjectedScript, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	targetDir := e.TargetDir(deploymentID)
	existing, err := LoadScriptsFromDisk(targetDir)
	if err != nil {
		return nil, err
	}

	var matched *InjectedScript
	for i, s := range existing {
		if s.ID == scriptID {
			existing[i].Enabled = enabled
			existing[i].UpdatedAt = time.Now().UTC()
			matched = &existing[i]
			break
		}
	}

	if matched == nil {
		return nil, ErrScriptNotFound
	}

	if err := SaveScriptsToDisk(targetDir, existing); err != nil {
		return nil, err
	}

	return matched, nil
}

// -----------------------------------------------------------------------------
// Config Tweaker (wp-config.php)
// -----------------------------------------------------------------------------

// TweakConfig modifies wp-config.php settings safely with pre-save PHP syntax verification.
func (e *WebmasterEngine) TweakConfig(ctx context.Context, deploymentID string, tweaks map[string]interface{}, rawDirectives []string) (*ConfigTweakResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	depID := strings.TrimSpace(deploymentID)
	if depID == "" {
		return nil, cuanDB.ErrEmptyWordPressDeploymentID
	}

	targetDir := e.TargetDir(depID)
	res, err := TweakWPConfigFile(targetDir, tweaks, rawDirectives)
	if err != nil {
		return nil, err
	}
	res.DeploymentID = depID

	// Record Webmaster Action in audit log
	actionID := fmt.Sprintf("act_config_%d", time.Now().UnixNano())
	payloadMap := map[string]interface{}{
		"tweaks":         tweaks,
		"raw_directives": rawDirectives,
		"syntax_valid":   res.SyntaxValid,
		"validator":      res.Validator,
	}
	payloadJSON, _ := json.Marshal(payloadMap)

	if e.cfg.DBRepo != nil {
		actionRecord := &cuanDB.WordPressWebmasterAction{
			ID:           actionID,
			DeploymentID: depID,
			ActionType:   cuanDB.WebmasterActionConfigTweak,
			Payload:      payloadJSON,
			AppliedAt:    time.Now().UTC(),
		}
		_ = e.cfg.DBRepo.RecordWebmasterAction(ctx, actionRecord)
	}

	res.Logs = append(res.Logs, fmt.Sprintf("Recorded Webmaster Audit Action %s (config_tweak)", actionID))
	return res, nil
}

// GetConfigTweaks reads active constant definitions from wp-config.php.
func (e *WebmasterEngine) GetConfigTweaks(ctx context.Context, deploymentID string) (map[string]interface{}, error) {
	targetDir := e.TargetDir(deploymentID)
	return ParseCurrentConfigTweaks(targetDir)
}

// -----------------------------------------------------------------------------
// Webmaster Action Audit History
// -----------------------------------------------------------------------------

// ListActions returns all webmaster actions recorded for a WordPress deployment.
func (e *WebmasterEngine) ListActions(ctx context.Context, deploymentID string) ([]*cuanDB.WordPressWebmasterAction, error) {
	if e.cfg.DBRepo == nil {
		return []*cuanDB.WordPressWebmasterAction{}, nil
	}
	return e.cfg.DBRepo.ListWebmasterActionsByDeploymentID(ctx, deploymentID)
}

// GetActionByID retrieves a single webmaster action by ID.
func (e *WebmasterEngine) GetActionByID(ctx context.Context, actionID string) (*cuanDB.WordPressWebmasterAction, error) {
	if e.cfg.DBRepo == nil {
		return nil, cuanDB.ErrWordPressWebmasterActionNotFound
	}
	return e.cfg.DBRepo.GetWebmasterActionByID(ctx, actionID)
}
