package runner

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const StorageInterruptionMessage = "Execution interrupted because server storage was exhausted; saved work is preserved."
const maxProviderDiagnosticRead = 128 * 1024

var providerLogRecord = regexp.MustCompile(`^[EW][0-9]{4} [0-9:.]+ +[0-9]+ [a-zA-Z0-9_]+\.go:[0-9]+\] `)

// Read only this process's private log tail. Do not expose credentials, reasoning
// or arbitrary provider text in task state; retain only a recognized local fault.
func providerStorageExhausted(path, home string) bool {
	if home == "" || !filepath.IsAbs(home) {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	start := info.Size() - maxProviderDiagnosticRead
	if start > 0 {
		if _, err = file.Seek(start, io.SeekStart); err != nil {
			return false
		}
	}
	raw, err := io.ReadAll(io.LimitReader(file, maxProviderDiagnosticRead))
	if err != nil {
		return false
	}
	prefix := filepath.Join(home, ".gemini", "antigravity-cli") + string(filepath.Separator)
	for _, line := range strings.Split(string(raw), "\n") {
		if providerLogRecord.MatchString(line) && strings.Contains(line, prefix) && strings.HasSuffix(strings.TrimSpace(line), "no space left on device") {
			return true
		}
	}
	return false
}
