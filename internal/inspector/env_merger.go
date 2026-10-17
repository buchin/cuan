package inspector

import (
	"github.com/buchin/cuan/internal/deployer"
)

// Re-export ApplicationSecrets from deployer for inspector package consumers.
type ApplicationSecrets = deployer.ApplicationSecrets

// GenerateApplicationSecrets generates high-entropy random application secrets.
func GenerateApplicationSecrets() (*ApplicationSecrets, error) {
	return deployer.GenerateApplicationSecrets()
}

// GenerateLaravelAppKey creates a Laravel-compatible 32-byte base64-encoded key.
func GenerateLaravelAppKey() (string, error) {
	return deployer.GenerateLaravelAppKey()
}

// GenerateJWTSecret creates a 256-bit hex-encoded secret key.
func GenerateJWTSecret() (string, error) {
	return deployer.GenerateJWTSecret()
}

// GenerateSessionSalt creates a 16-byte hex-encoded salt string.
func GenerateSessionSalt() (string, error) {
	return deployer.GenerateSessionSalt()
}

// GenerateSecurePassword generates a high-entropy password of given length.
func GenerateSecurePassword(length int) (string, error) {
	return deployer.GenerateSecurePassword(length)
}

// AutoMapDatabaseEnv converts provisioned database attributes into standard .env variables.
func AutoMapDatabaseEnv(engine, dbName, dbUser, dbPassword, host string, port int) map[string]string {
	return deployer.AutoMapDatabaseEnv(engine, dbName, dbUser, dbPassword, host, port)
}

// MergeEnv merges template .env content with generated secrets, database variables, and custom overrides.
func MergeEnv(baseTemplate string, dbVars map[string]string, appSecrets map[string]string, customOverrides map[string]string) string {
	return deployer.MergeEnv(baseTemplate, dbVars, appSecrets, customOverrides)
}

// ParseEnvMap parses raw multi-line environment content into key-value map.
func ParseEnvMap(rawEnv string) map[string]string {
	return deployer.ParseEnvMap(rawEnv)
}

// MaskEnvVars returns a copy of env map with all sensitive keys masked.
func MaskEnvVars(env map[string]string) map[string]string {
	return deployer.MaskEnvVars(env)
}

// MaskRawEnv produces a sanitized multi-line string safe for public display and logs.
func MaskRawEnv(rawEnv string) string {
	return deployer.MaskRawEnv(rawEnv)
}
