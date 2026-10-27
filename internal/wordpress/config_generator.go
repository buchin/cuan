package wordpress

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// SaltChars defines the safe character set for WordPress salt keys.
// Single quotes and backslashes are intentionally excluded to prevent PHP single-quote escaping issues.
const SaltChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#$%&*()-_=+[]{}|;:<>?~"

// GenerateRandomSalt generates a cryptographically secure random salt string of the requested length.
func GenerateRandomSalt(length int) (string, error) {
	if length <= 0 {
		length = 64
	}
	maxIdx := big.NewInt(int64(len(SaltChars)))
	var sb strings.Builder
	sb.Grow(length)

	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, maxIdx)
		if err != nil {
			return "", fmt.Errorf("generate random salt: %w", err)
		}
		sb.WriteByte(SaltChars[n.Int64()])
	}
	return sb.String(), nil
}

// WPConfigParams holds configuration settings for generating wp-config.php.
type WPConfigParams struct {
	DBName         string
	DBUser         string
	DBPassword     string
	DBHost         string
	DBPort         int
	TablePrefix    string
	SiteURL        string
	WPDebug        bool
	AuthKey        string
	SecureAuthKey  string
	LoggedInKey    string
	NonceKey       string
	AuthSalt       string
	SecureAuthSalt string
	LoggedInSalt   string
	NonceSalt      string
}

// PopulateSalts generates 8 random 64-character salts if any are missing.
func (p *WPConfigParams) PopulateSalts() error {
	salts := []*string{
		&p.AuthKey,
		&p.SecureAuthKey,
		&p.LoggedInKey,
		&p.NonceKey,
		&p.AuthSalt,
		&p.SecureAuthSalt,
		&p.LoggedInSalt,
		&p.NonceSalt,
	}

	for _, s := range salts {
		if *s == "" {
			val, err := GenerateRandomSalt(64)
			if err != nil {
				return err
			}
			*s = val
		}
	}
	return nil
}

const wpConfigTemplate = `<?php
/**
 * The base configuration for WordPress
 *
 * Generated automatically by Cuan Turnkey WordPress Cloud Engine (Sprint 13).
 * https://github.com/buchin/cuan
 */

// ** Database settings ** //
define( 'DB_NAME', '{{.DBName}}' );
define( 'DB_USER', '{{.DBUser}}' );
define( 'DB_PASSWORD', '{{.DBPassword}}' );
define( 'DB_HOST', '{{.DBHost}}:{{.DBPort}}' );
define( 'DB_CHARSET', 'utf8mb4' );
define( 'DB_COLLATE', '' );

/**#@+
 * Authentication unique keys and salts.
 */
define( 'AUTH_KEY',         '{{.AuthKey}}' );
define( 'SECURE_AUTH_KEY',  '{{.SecureAuthKey}}' );
define( 'LOGGED_IN_KEY',    '{{.LoggedInKey}}' );
define( 'NONCE_KEY',        '{{.NonceKey}}' );
define( 'AUTH_SALT',        '{{.AuthSalt}}' );
define( 'SECURE_AUTH_SALT', '{{.SecureAuthSalt}}' );
define( 'LOGGED_IN_SALT',   '{{.LoggedInSalt}}' );
define( 'NONCE_SALT',       '{{.NonceSalt}}' );
/**#@-*/

/**
 * WordPress database table prefix.
 */
$table_prefix = '{{.TablePrefix}}';

/**
 * For developers: WordPress debugging mode.
 */
define( 'WP_DEBUG', {{if .WPDebug}}true{{else}}false{{end}} );

/**
 * Reverse Proxy & SSL Automation Detection (Caddy integration)
 */
if ( isset( $_SERVER['HTTP_X_FORWARDED_PROTO'] ) && $_SERVER['HTTP_X_FORWARDED_PROTO'] === 'https' ) {
    $_SERVER['HTTPS'] = 'on';
}

{{if .SiteURL}}
if ( ! defined( 'WP_HOME' ) ) {
    define( 'WP_HOME', '{{.SiteURL}}' );
}
if ( ! defined( 'WP_SITEURL' ) ) {
    define( 'WP_SITEURL', '{{.SiteURL}}' );
}
{{end}}

/* That's all, stop editing! Happy publishing. */

/** Absolute path to the WordPress directory. */
if ( ! defined( 'ABSPATH' ) ) {
    define( 'ABSPATH', __DIR__ . '/' );
}

/** Sets up WordPress vars and included files. */
if ( file_exists( ABSPATH . 'wp-settings.php' ) ) {
    require_once ABSPATH . 'wp-settings.php';
}
`

// GenerateWPConfigContent renders the wp-config.php template with provided parameters.
func GenerateWPConfigContent(params WPConfigParams) (string, error) {
	if err := params.PopulateSalts(); err != nil {
		return "", err
	}
	if params.TablePrefix == "" {
		params.TablePrefix = "wp_"
	}
	if params.DBHost == "" {
		params.DBHost = "127.0.0.1"
	}
	if params.DBPort <= 0 {
		params.DBPort = 3306
	}

	tmpl, err := template.New("wp-config").Parse(wpConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("parse wp-config template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("execute wp-config template: %w", err)
	}

	return buf.String(), nil
}

// WriteWPConfig writes the rendered wp-config.php to the specified target directory.
func WriteWPConfig(targetDir string, params WPConfigParams) error {
	content, err := GenerateWPConfigContent(params)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	filePath := filepath.Join(targetDir, "wp-config.php")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write wp-config.php: %w", err)
	}

	return nil
}
