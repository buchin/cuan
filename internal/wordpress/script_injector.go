package wordpress

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrEmptyScriptContent  = errors.New("script content cannot be empty")
	ErrInvalidScriptPosition = errors.New("invalid script position: must be 'head' or 'footer'")
	ErrScriptNotFound      = errors.New("injected script not found")
)

const (
	PositionHead   = "head"
	PositionFooter = "footer"

	ScriptTypeGA4       = "ga4"
	ScriptTypeGTM       = "gtm"
	ScriptTypeMetaPixel = "meta_pixel"
	ScriptTypeCustom    = "custom"
)

// InjectedScript represents an analytics, tracking, or custom script managed by the AI Webmaster.
type InjectedScript struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // ga4, gtm, meta_pixel, custom
	Position  string    `json:"position"` // head, footer
	Content   string    `json:"content"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// InjectScriptParams defines input parameters to add or update an injected script.
type InjectScriptParams struct {
	DeploymentID string `json:"deployment_id"`
	ID           string `json:"id,omitempty"`
	Name         string `json:"name"`
	Type         string `json:"type,omitempty"`
	Position     string `json:"position,omitempty"` // default: head
	Content      string `json:"content"`
	Enabled      *bool  `json:"enabled,omitempty"`  // default: true
}

// CuanWebmasterMuPluginPHP is the content of the isolated mu-plugin that executes scripts safely without WSOD.
const CuanWebmasterMuPluginPHP = `<?php
/**
 * Plugin Name: Cuan AI Webmaster Engine
 * Plugin URI: https://github.com/buchin/cuan
 * Description: Isolated tracking script injector without theme-dependency or WSOD risk.
 * Version: 1.0.0
 * Author: Cuan AI Webmaster
 */

if (!defined('ABSPATH')) {
    exit;
}

if (!function_exists('cuan_webmaster_render_scripts')) {
    function cuan_webmaster_render_scripts($position) {
        $configFile = __DIR__ . '/cuan-webmaster-scripts.json';
        if (!file_exists($configFile)) {
            return;
        }
        $raw = @file_get_contents($configFile);
        if (!$raw) {
            return;
        }
        $scripts = @json_decode($raw, true);
        if (!is_array($scripts)) {
            return;
        }
        foreach ($scripts as $s) {
            $enabled = isset($s['enabled']) ? (bool)$s['enabled'] : true;
            $pos = isset($s['position']) ? strtolower(trim($s['position'])) : 'head';
            if ($enabled && $pos === $position) {
                $name = isset($s['name']) ? htmlspecialchars($s['name'], ENT_QUOTES, 'UTF-8') : 'cuan-script';
                echo "\n<!-- [cuan-webmaster: {$name}] -->\n";
                if (!empty($s['content'])) {
                    echo $s['content'] . "\n";
                }
            }
        }
    }
}

// Hook head scripts early
add_action('wp_head', function() {
    cuan_webmaster_render_scripts('head');
}, 1);

// Hook footer scripts before body close
add_action('wp_footer', function() {
    cuan_webmaster_render_scripts('footer');
}, 99);
`

// scriptMuLock prevents concurrent writes to mu-plugins scripts file
var scriptMuLock sync.Mutex

// EnsureMuPluginFiles guarantees that wp-content/mu-plugins and cuan-webmaster.php exist.
func EnsureMuPluginFiles(targetDir string) error {
	muDir := filepath.Join(targetDir, "wp-content", "mu-plugins")
	if err := os.MkdirAll(muDir, 0755); err != nil {
		return fmt.Errorf("create mu-plugins dir: %w", err)
	}

	pluginPath := filepath.Join(muDir, "cuan-webmaster.php")
	// If missing or different, write file
	existing, err := os.ReadFile(pluginPath)
	if err != nil || string(existing) != CuanWebmasterMuPluginPHP {
		if err := os.WriteFile(pluginPath, []byte(CuanWebmasterMuPluginPHP), 0644); err != nil {
			return fmt.Errorf("write cuan-webmaster.php: %w", err)
		}
	}

	return nil
}

// GetMuPluginPaths returns paths for cuan-webmaster.php and cuan-webmaster-scripts.json.
func GetMuPluginPaths(targetDir string) (phpPath string, jsonPath string) {
	muDir := filepath.Join(targetDir, "wp-content", "mu-plugins")
	return filepath.Join(muDir, "cuan-webmaster.php"), filepath.Join(muDir, "cuan-webmaster-scripts.json")
}

// LoadScriptsFromDisk reads existing injected scripts from cuan-webmaster-scripts.json.
func LoadScriptsFromDisk(targetDir string) ([]InjectedScript, error) {
	scriptMuLock.Lock()
	defer scriptMuLock.Unlock()

	_, jsonPath := GetMuPluginPaths(targetDir)
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []InjectedScript{}, nil
		}
		return nil, fmt.Errorf("read scripts json: %w", err)
	}

	var scripts []InjectedScript
	if err := json.Unmarshal(data, &scripts); err != nil {
		return nil, fmt.Errorf("unmarshal scripts json: %w", err)
	}
	return scripts, nil
}

// SaveScriptsToDisk atomically persists injected scripts to cuan-webmaster-scripts.json.
func SaveScriptsToDisk(targetDir string, scripts []InjectedScript) error {
	scriptMuLock.Lock()
	defer scriptMuLock.Unlock()

	if err := EnsureMuPluginFiles(targetDir); err != nil {
		return err
	}

	_, jsonPath := GetMuPluginPaths(targetDir)
	data, err := json.MarshalIndent(scripts, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal scripts: %w", err)
	}

	tmpFile := jsonPath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("write temp scripts json: %w", err)
	}

	if err := os.Rename(tmpFile, jsonPath); err != nil {
		return fmt.Errorf("rename scripts json: %w", err)
	}

	return nil
}

// BuildGA4Snippet generates standard Google Analytics 4 tracking script.
func BuildGA4Snippet(measurementID string) string {
	mID := strings.TrimSpace(measurementID)
	return fmt.Sprintf(`<!-- Google tag (gtag.js) -->
<script async src="https://www.googletagmanager.com/gtag/js?id=%s"></script>
<script>
  window.dataLayer = window.dataLayer || [];
  function gtag(){dataLayer.push(arguments);}
  gtag('js', new Date());
  gtag('config', '%s');
</script>`, mID, mID)
}

// BuildGTMSnippets generates Google Tag Manager head and body/footer snippets.
func BuildGTMSnippets(containerID string) (headSnippet, bodySnippet string) {
	cID := strings.TrimSpace(containerID)
	headSnippet = fmt.Sprintf(`<!-- Google Tag Manager -->
<script>(function(w,d,s,l,i){w[l]=w[l]||[];w[l].push({'gtm.start':
new Date().getTime(),event:'gtm.js'});var f=d.getElementsByTagName(s)[0],
j=d.createElement(s),dl=l!='dataLayer'?'&l='+l:'';j.async=true;j.src=
'https://www.googletagmanager.com/gtm.js?id='+i+dl;f.parentNode.insertBefore(j,f);
})(window,document,'script','dataLayer','%s');</script>
<!-- End Google Tag Manager -->`, cID)

	bodySnippet = fmt.Sprintf(`<!-- Google Tag Manager (noscript) -->
<noscript><iframe src="https://www.googletagmanager.com/ns.html?id=%s"
height="0" width="0" style="display:none;visibility:hidden"></iframe></noscript>
<!-- End Google Tag Manager (noscript) -->`, cID)
	return headSnippet, bodySnippet
}

// BuildMetaPixelSnippet generates Meta / Facebook Pixel base tracking snippet.
func BuildMetaPixelSnippet(pixelID string) string {
	pID := strings.TrimSpace(pixelID)
	return fmt.Sprintf(`<!-- Meta Pixel Code -->
<script>
!function(f,b,e,v,n,t,s)
{if(f.fbq)return;n=f.fbq=function(){n.callMethod?
n.callMethod.apply(n,arguments):n.queue.push(arguments)};
if(!f._fbq)f._fbq=n;n.push=n;n.loaded=!0;n.version='2.0';
n.queue=[];t=b.createElement(e);t.async=!0;
t.src=v;s=b.getElementsByTagName(e)[0];
s.parentNode.insertBefore(t,s)}(window, document,'script',
'https://connect.facebook.net/en_US/fbevents.js');
fbq('init', '%s');
fbq('track', 'PageView');
</script>
<noscript><img height="1" width="1" style="display:none"
src="https://www.facebook.com/tr?id=%s&ev=PageView&noscript=1"
/></noscript>
<!-- End Meta Pixel Code -->`, pID, pID)
}
