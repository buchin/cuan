package diagnostics

import (
	"os"
	"testing"
)

// CLI/MCP tests must never import the developer's saved Owner connection or
// discover a production database from ambient environment/.env files. Tests
// needing a connection inject their own client or dedicated fixture database.
func TestMain(m *testing.M) {
	cachedCuanConfig = &CuanAPIConfig{}
	globalCuanClient = &CuanClient{}
	platformDB = nil
	platformDBOnce.Do(func() {})
	os.Exit(m.Run())
}
