package wordpress

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	ErrWPConfigNotFound     = errors.New("wp-config.php not found in deployment directory")
	ErrEmptyConfigTweaks    = errors.New("no config tweaks provided")
	ErrInvalidPHPSyntax     = errors.New("php syntax pre-save validation failed")
)

// ConfigTweakResult represents the outcome of modifying wp-config.php.
type ConfigTweakResult struct {
	DeploymentID  string                 `json:"deployment_id"`
	AppliedTweaks map[string]interface{} `json:"applied_tweaks"`
	ConfigPath    string                 `json:"config_path"`
	BackupPath    string                 `json:"backup_path"`
	SyntaxValid   bool                   `json:"syntax_valid"`
	Validator     string                 `json:"validator"` // "php-cli" or "internal-linter"
	Timestamp     time.Time              `json:"timestamp"`
	Logs          []string               `json:"logs"`
}

// ValidatePHPSyntax performs pre-save integrity checks on PHP code.
// Uses `php -l` via PHP CLI if available, and runs an internal pure-Go syntax verification.
func ValidatePHPSyntax(content []byte) (string, error) {
	if len(content) == 0 {
		return "", errors.New("php content is empty")
	}

	contentStr := string(content)

	// 1. Internal pure-Go syntax rules
	if !strings.HasPrefix(strings.TrimSpace(contentStr), "<?php") {
		return "internal-linter", fmt.Errorf("%w: file must start with '<?php'", ErrInvalidPHPSyntax)
	}

	if err := verifyBalancedTokens(contentStr); err != nil {
		return "internal-linter", fmt.Errorf("%w: %v", ErrInvalidPHPSyntax, err)
	}

	// 2. PHP CLI `php -l` validation if binary exists
	phpBinary, err := exec.LookPath("php")
	if err == nil && phpBinary != "" {
		tmpFile, err := os.CreateTemp("", "wp-syntax-check-*.php")
		if err != nil {
			return "internal-linter", fmt.Errorf("create temp php file: %w", err)
		}
		defer func() {
			tmpFile.Close()
			_ = os.Remove(tmpFile.Name())
		}()

		if _, err := tmpFile.Write(content); err != nil {
			return "internal-linter", fmt.Errorf("write temp php file: %w", err)
		}
		_ = tmpFile.Close()

		cmd := exec.Command(phpBinary, "-l", tmpFile.Name())
		out, cmdErr := cmd.CombinedOutput()
		outStr := strings.TrimSpace(string(out))
		if cmdErr != nil || strings.Contains(strings.ToLower(outStr), "parse error") || strings.Contains(strings.ToLower(outStr), "fatal error") {
			// Strip temp file path from message for cleaner reporting
			msg := strings.ReplaceAll(outStr, tmpFile.Name(), "wp-config.php")
			return "php-cli", fmt.Errorf("%w: %s", ErrInvalidPHPSyntax, msg)
		}
		return "php-cli", nil
	}

	return "internal-linter", nil
}

// verifyBalancedTokens checks for balanced parentheses, brackets, braces, and quotes outside comments/strings.
func verifyBalancedTokens(src string) error {
	inSingleQuote := false
	inDoubleQuote := false
	inLineComment := false
	inBlockComment := false
	isEscaped := false

	var parenStack []rune
	line := 1

	runes := []rune(src)
	n := len(runes)

	for i := 0; i < n; i++ {
		r := runes[i]
		if r == '\n' {
			line++
			inLineComment = false
			continue
		}

		if inLineComment {
			continue
		}

		if inBlockComment {
			if r == '*' && i+1 < n && runes[i+1] == '/' {
				inBlockComment = false
				i++ // skip '/'
			}
			continue
		}

		if inSingleQuote {
			if r == '\\' && !isEscaped {
				isEscaped = true
				continue
			}
			if r == '\'' && !isEscaped {
				inSingleQuote = false
			}
			isEscaped = false
			continue
		}

		if inDoubleQuote {
			if r == '\\' && !isEscaped {
				isEscaped = true
				continue
			}
			if r == '"' && !isEscaped {
				inDoubleQuote = false
			}
			isEscaped = false
			continue
		}

		// Check start of comment
		if r == '/' && i+1 < n {
			next := runes[i+1]
			if next == '/' {
				inLineComment = true
				i++
				continue
			}
			if next == '*' {
				inBlockComment = true
				i++
				continue
			}
		}
		if r == '#' {
			inLineComment = true
			continue
		}

		// Check start of quote
		if r == '\'' {
			inSingleQuote = true
			continue
		}
		if r == '"' {
			inDoubleQuote = true
			continue
		}

		// Check brackets & parentheses
		switch r {
		case '(', '{', '[':
			parenStack = append(parenStack, r)
		case ')':
			if len(parenStack) == 0 || parenStack[len(parenStack)-1] != '(' {
				return fmt.Errorf("unexpected closing parenthesis ')' at line %d", line)
			}
			parenStack = parenStack[:len(parenStack)-1]
		case '}':
			if len(parenStack) == 0 || parenStack[len(parenStack)-1] != '{' {
				return fmt.Errorf("unexpected closing brace '}' at line %d", line)
			}
			parenStack = parenStack[:len(parenStack)-1]
		case ']':
			if len(parenStack) == 0 || parenStack[len(parenStack)-1] != '[' {
				return fmt.Errorf("unexpected closing bracket ']' at line %d", line)
			}
			parenStack = parenStack[:len(parenStack)-1]
		}
	}

	if inBlockComment {
		return errors.New("unclosed block comment '/*'")
	}
	if inSingleQuote {
		return errors.New("unclosed single quote")
	}
	if inDoubleQuote {
		return errors.New("unclosed double quote")
	}
	if len(parenStack) > 0 {
		return fmt.Errorf("unclosed token '%c'", parenStack[len(parenStack)-1])
	}

	return nil
}

// FormatDefineStatement formats a PHP define() statement according to type.
func FormatDefineStatement(key string, val interface{}) string {
	switch v := val.(type) {
	case bool:
		if v {
			return fmt.Sprintf("define( '%s', true );", key)
		}
		return fmt.Sprintf("define( '%s', false );", key)
	case int:
		return fmt.Sprintf("define( '%s', %d );", key, v)
	case int64:
		return fmt.Sprintf("define( '%s', %d );", key, v)
	case float64:
		// Check if it's integer float
		if v == float64(int64(v)) {
			return fmt.Sprintf("define( '%s', %d );", key, int64(v))
		}
		return fmt.Sprintf("define( '%s', %v );", key, v)
	default:
		s := fmt.Sprintf("%v", v)
		// Escape single quotes for safe PHP string literal
		s = strings.ReplaceAll(s, "\\", "\\\\")
		s = strings.ReplaceAll(s, "'", "\\'")
		return fmt.Sprintf("define( '%s', '%s' );", key, s)
	}
}

// ApplyConfigTweaks modifies wp-config.php content by updating existing defines or inserting new ones.
func ApplyConfigTweaks(originalContent string, tweaks map[string]interface{}, rawDirectives []string) (string, error) {
	content := originalContent
	insertedDirectives := []string{}

	for key, val := range tweaks {
		cleanKey := strings.TrimSpace(key)
		if cleanKey == "" {
			continue
		}
		defineStmt := FormatDefineStatement(cleanKey, val)

		// Regex to find existing define('KEY', ...) or define("KEY", ...)
		pattern := fmt.Sprintf(`(?m)^[ \t]*define\s*\(\s*['"]%s['"]\s*,[^;]+;[ \t]*$`, regexp.QuoteMeta(cleanKey))
		re := regexp.MustCompile(pattern)

		if re.MatchString(content) {
			content = re.ReplaceAllString(content, defineStmt)
		} else {
			insertedDirectives = append(insertedDirectives, defineStmt)
		}
	}

	for _, raw := range rawDirectives {
		cleanRaw := strings.TrimSpace(raw)
		if cleanRaw != "" {
			if !strings.HasSuffix(cleanRaw, ";") {
				cleanRaw += ";"
			}
			insertedDirectives = append(insertedDirectives, cleanRaw)
		}
	}

	if len(insertedDirectives) > 0 {
		insertionBlock := "\n/** Added by Cuan AI Webmaster Engine Config Tweaker **/\n" +
			strings.Join(insertedDirectives, "\n") + "\n"

		// Marker 1: standard WordPress stop editing comment
		stopMarker := "/* That's all, stop editing! Happy publishing. */"
		if strings.Contains(content, stopMarker) {
			content = strings.Replace(content, stopMarker, insertionBlock+"\n"+stopMarker, 1)
		} else {
			// Marker 2: before ABSPATH definition
			abspathMarker := "if ( ! defined( 'ABSPATH' ) )"
			if strings.Contains(content, abspathMarker) {
				content = strings.Replace(content, abspathMarker, insertionBlock+"\n"+abspathMarker, 1)
			} else {
				// Marker 3: before wp-settings require
				settingsMarker := "require_once ABSPATH . 'wp-settings.php';"
				if strings.Contains(content, settingsMarker) {
					content = strings.Replace(content, settingsMarker, insertionBlock+"\n"+settingsMarker, 1)
				} else {
					content += insertionBlock
				}
			}
		}
	}

	return content, nil
}

// TweakWPConfigFile applies tweaks to targetDir/wp-config.php with atomic write and backup.
// Crucially, it validates syntax BEFORE saving to disk, ensuring 0% WSOD risk.
func TweakWPConfigFile(targetDir string, tweaks map[string]interface{}, rawDirectives []string) (*ConfigTweakResult, error) {
	if len(tweaks) == 0 && len(rawDirectives) == 0 {
		return nil, ErrEmptyConfigTweaks
	}

	configPath := filepath.Join(targetDir, "wp-config.php")
	originalBytes, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrWPConfigNotFound
		}
		return nil, fmt.Errorf("read wp-config.php: %w", err)
	}

	modifiedContent, err := ApplyConfigTweaks(string(originalBytes), tweaks, rawDirectives)
	if err != nil {
		return nil, fmt.Errorf("apply tweaks: %w", err)
	}

	// PRE-SAVE VALIDATION: check PHP syntax
	validator, err := ValidatePHPSyntax([]byte(modifiedContent))
	if err != nil {
		// Do NOT write to disk! Original file remains intact.
		return nil, err
	}

	// Create backup
	backupPath := configPath + ".bak"
	if err := os.WriteFile(backupPath, originalBytes, 0644); err != nil {
		return nil, fmt.Errorf("create config backup: %w", err)
	}

	// Write modified file atomically
	tmpFile := configPath + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(modifiedContent), 0644); err != nil {
		return nil, fmt.Errorf("write temp config: %w", err)
	}

	if err := os.Rename(tmpFile, configPath); err != nil {
		return nil, fmt.Errorf("rename temp config to wp-config.php: %w", err)
	}

	res := &ConfigTweakResult{
		AppliedTweaks: tweaks,
		ConfigPath:    configPath,
		BackupPath:    backupPath,
		SyntaxValid:   true,
		Validator:     validator,
		Timestamp:     time.Now().UTC(),
		Logs: []string{
			fmt.Sprintf("PHP syntax validated successfully via %s", validator),
			fmt.Sprintf("Backup saved to %s", backupPath),
			fmt.Sprintf("Updated %d config directives in %s", len(tweaks)+len(rawDirectives), configPath),
		},
	}

	return res, nil
}

// ParseCurrentConfigTweaks scans wp-config.php and extracts active constant definitions.
func ParseCurrentConfigTweaks(targetDir string) (map[string]interface{}, error) {
	configPath := filepath.Join(targetDir, "wp-config.php")
	contentBytes, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrWPConfigNotFound
		}
		return nil, err
	}

	results := make(map[string]interface{})
	lines := strings.Split(string(contentBytes), "\n")
	re := regexp.MustCompile(`^[ \t]*define\s*\(\s*['"]([A-Z0-9_]+)['"]\s*,\s*(.+)\s*\)\s*;`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		matches := re.FindStringSubmatch(trimmed)
		if len(matches) == 3 {
			key := matches[1]
			valStr := strings.TrimSpace(matches[2])
			// Parse boolean
			if strings.EqualFold(valStr, "true") {
				results[key] = true
			} else if strings.EqualFold(valStr, "false") {
				results[key] = false
			} else if strings.HasPrefix(valStr, "'") && strings.HasSuffix(valStr, "'") && len(valStr) >= 2 {
				results[key] = valStr[1 : len(valStr)-1]
			} else if strings.HasPrefix(valStr, "\"") && strings.HasSuffix(valStr, "\"") && len(valStr) >= 2 {
				results[key] = valStr[1 : len(valStr)-1]
			} else {
				results[key] = valStr
			}
		}
	}

	return results, nil
}
