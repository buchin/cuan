package deployer

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/buchin/cuan/internal/db"
)

// ApplicationSecrets holds newly generated cryptographic secrets.
type ApplicationSecrets struct {
	LaravelAppKey string `json:"laravel_app_key"`
	JWTSecret     string `json:"jwt_secret"`
	SessionSalt   string `json:"session_salt"`
	DBPassword    string `json:"db_password"`
}

// GenerateApplicationSecrets generates high-entropy random application secrets.
func GenerateApplicationSecrets() (*ApplicationSecrets, error) {
	appKey, err := GenerateLaravelAppKey()
	if err != nil {
		return nil, fmt.Errorf("generate laravel app key: %w", err)
	}

	jwtSecret, err := GenerateJWTSecret()
	if err != nil {
		return nil, fmt.Errorf("generate jwt secret: %w", err)
	}

	salt, err := GenerateSessionSalt()
	if err != nil {
		return nil, fmt.Errorf("generate session salt: %w", err)
	}

	dbPass, err := GenerateSecurePassword(24)
	if err != nil {
		return nil, fmt.Errorf("generate db password: %w", err)
	}

	return &ApplicationSecrets{
		LaravelAppKey: appKey,
		JWTSecret:     jwtSecret,
		SessionSalt:   salt,
		DBPassword:    dbPass,
	}, nil
}

// GenerateLaravelAppKey creates a Laravel-compatible 32-byte base64-encoded key.
// Format: "base64:<base64-encoded-32-random-bytes>"
func GenerateLaravelAppKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "base64:" + base64.StdEncoding.EncodeToString(buf), nil
}

// GenerateJWTSecret creates a 256-bit (32 bytes) hex-encoded secret key.
func GenerateJWTSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// GenerateSessionSalt creates a 16-byte (128-bit) hex-encoded salt string.
func GenerateSessionSalt() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// GenerateSecurePassword generates a high-entropy password of given length.
func GenerateSecurePassword(length int) (string, error) {
	if length < 12 {
		length = 24
	}

	const (
		lowerChars  = "abcdefghijklmnopqrstuvwxyz"
		upperChars  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		digitChars  = "0123456789"
		symbolChars = "!@#$%^&*-_=+"
		allChars    = lowerChars + upperChars + digitChars + symbolChars
	)

	// Ensure at least one char from each set
	res := make([]byte, length)
	sets := []string{lowerChars, upperChars, digitChars, symbolChars}
	for i, s := range sets {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(s))))
		if err != nil {
			return "", err
		}
		res[i] = s[idx.Int64()]
	}

	for i := len(sets); i < length; i++ {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(allChars))))
		if err != nil {
			return "", err
		}
		res[i] = allChars[idx.Int64()]
	}

	// Shuffle with Fisher-Yates
	for i := length - 1; i > 0; i-- {
		jBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		j := int(jBig.Int64())
		res[i], res[j] = res[j], res[i]
	}

	return string(res), nil
}

// AutoMapDatabaseEnv converts provisioned database attributes into standard .env variables.
func AutoMapDatabaseEnv(engine, dbName, dbUser, dbPassword, host string, port int) map[string]string {
	normEngine, _ := db.NormalizeDBEngine(engine)
	if host == "" {
		host = "127.0.0.1"
	}

	res := make(map[string]string)
	switch normEngine {
	case db.DBEngineSQLite:
		res["DB_CONNECTION"] = "sqlite"
		res["DB_HOST"] = host
		res["DB_PORT"] = ""
		res["DB_DATABASE"] = dbName
		res["DB_USERNAME"] = ""
		res["DB_PASSWORD"] = ""
	case db.DBEngineMySQL:
		if port <= 0 {
			port = 3306
		}
		res["DB_CONNECTION"] = "mysql"
		res["DB_HOST"] = host
		res["DB_PORT"] = strconv.Itoa(port)
		res["DB_DATABASE"] = dbName
		res["DB_USERNAME"] = dbUser
		res["DB_PASSWORD"] = dbPassword
	case db.DBEnginePostgres:
		if port <= 0 {
			port = 5432
		}
		res["DB_CONNECTION"] = "pgsql"
		res["DB_HOST"] = host
		res["DB_PORT"] = strconv.Itoa(port)
		res["DB_DATABASE"] = dbName
		res["DB_USERNAME"] = dbUser
		res["DB_PASSWORD"] = dbPassword
	}

	return res
}

// MergeEnv merges template .env content with generated secrets, database variables, and custom overrides.
func MergeEnv(baseTemplate string, dbVars map[string]string, appSecrets map[string]string, customOverrides map[string]string) string {
	combined := make(map[string]string)

	for k, v := range appSecrets {
		combined[k] = v
	}
	for k, v := range dbVars {
		combined[k] = v
	}
	for k, v := range customOverrides {
		combined[k] = v
	}

	// Track applied keys so we know which to append at the end
	appliedKeys := make(map[string]bool)

	var sb strings.Builder
	if strings.TrimSpace(baseTemplate) != "" {
		scanner := bufio.NewScanner(strings.NewReader(baseTemplate))
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)

			// Preserve blank lines and comments
			if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
				sb.WriteString(line)
				sb.WriteString("\n")
				continue
			}

			eqIdx := strings.Index(line, "=")
			if eqIdx == -1 {
				sb.WriteString(line)
				sb.WriteString("\n")
				continue
			}

			key := strings.TrimSpace(line[:eqIdx])
			origVal := strings.TrimSpace(line[eqIdx+1:])

			if newVal, exists := combined[key]; exists {
				sb.WriteString(key)
				sb.WriteString("=")
				sb.WriteString(formatEnvVal(newVal))
				sb.WriteString("\n")
				appliedKeys[key] = true
			} else {
				// If key not in combined, keep existing line
				sb.WriteString(key)
				sb.WriteString("=")
				sb.WriteString(origVal)
				sb.WriteString("\n")
				appliedKeys[key] = true
			}
		}
	}

	// Append remaining keys that were not present in the template
	var remainingKeys []string
	for k := range combined {
		if !appliedKeys[k] {
			remainingKeys = append(remainingKeys, k)
		}
	}

	if len(remainingKeys) > 0 {
		sort.Strings(remainingKeys)
		if sb.Len() > 0 && !strings.HasSuffix(sb.String(), "\n\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("# Auto-generated configuration by Cuan Provisioner\n")
		for _, k := range remainingKeys {
			sb.WriteString(k)
			sb.WriteString("=")
			sb.WriteString(formatEnvVal(combined[k]))
			sb.WriteString("\n")
		}
	}

	return strings.TrimRight(sb.String(), "\n") + "\n"
}

// formatEnvVal quotes string if it contains spaces or quotes.
func formatEnvVal(val string) string {
	if strings.ContainsAny(val, " \t\n\"") && !strings.HasPrefix(val, "\"") {
		escaped := strings.ReplaceAll(val, "\"", "\\\"")
		return "\"" + escaped + "\""
	}
	return val
}

// ParseEnvMap parses raw multi-line environment content into key-value map.
func ParseEnvMap(rawEnv string) map[string]string {
	res := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(rawEnv))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		eqIdx := strings.Index(line, "=")
		if eqIdx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:eqIdx])
		val := strings.TrimSpace(line[eqIdx+1:])
		if strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") && len(val) >= 2 {
			val = strings.TrimPrefix(strings.TrimSuffix(val, "\""), "\"")
			val = strings.ReplaceAll(val, "\\\"", "\"")
		} else if strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'") && len(val) >= 2 {
			val = strings.TrimPrefix(strings.TrimSuffix(val, "'"), "'")
		}
		res[key] = val
	}
	return res
}

// IsSensitiveKey returns true if the key likely contains secrets.
func IsSensitiveKey(key string) bool {
	upper := strings.ToUpper(strings.TrimSpace(key))
	if upper == "APP_KEY" ||
		upper == "JWT_SECRET" ||
		upper == "DB_PASSWORD" ||
		strings.Contains(upper, "PASSWORD") ||
		strings.Contains(upper, "SECRET") ||
		strings.Contains(upper, "SALT") ||
		strings.Contains(upper, "TOKEN") ||
		strings.Contains(upper, "PRIVATE_KEY") ||
		strings.Contains(upper, "AUTH_KEY") ||
		strings.Contains(upper, "CREDENTIAL") {
		return true
	}
	return false
}

// MaskValue masks sensitive values to prevent credential leakage.
func MaskValue(key, value string) string {
	if value == "" {
		return ""
	}
	if !IsSensitiveKey(key) {
		return value
	}
	if strings.HasPrefix(value, "base64:") {
		return "base64:********************"
	}
	return "********"
}

// MaskEnvVars returns a copy of env map with all sensitive keys masked.
func MaskEnvVars(env map[string]string) map[string]string {
	masked := make(map[string]string, len(env))
	for k, v := range env {
		masked[k] = MaskValue(k, v)
	}
	return masked
}

// MaskRawEnv produces a sanitized multi-line string safe for public display and logs.
func MaskRawEnv(rawEnv string) string {
	var sb strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(rawEnv))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
			sb.WriteString(line)
			sb.WriteString("\n")
			continue
		}
		eqIdx := strings.Index(line, "=")
		if eqIdx == -1 {
			sb.WriteString(line)
			sb.WriteString("\n")
			continue
		}
		key := strings.TrimSpace(line[:eqIdx])
		val := strings.TrimSpace(line[eqIdx+1:])
		maskedVal := MaskValue(key, val)
		sb.WriteString(key)
		sb.WriteString("=")
		sb.WriteString(maskedVal)
		sb.WriteString("\n")
	}
	return sb.String()
}
