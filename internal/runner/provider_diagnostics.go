package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
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

// HistoricalProviderStorageEvidence recognizes one provider process started for
// this exact workspace during this attempt. It never searches global profiles.
func HistoricalProviderStorageEvidence(home, workspace string, started, finished time.Time) string {
	if home == "" || !filepath.IsAbs(workspace) || started.IsZero() || !finished.After(started) {
		return ""
	}
	directory := filepath.Join(home, ".gemini", "antigravity-cli", "log")
	entries, err := os.ReadDir(directory)
	if err != nil {
		return ""
	}
	checked := 0
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		stamp, err := time.ParseInLocation("cli-20060102_150405.log", entry.Name(), time.Local)
		if err != nil || stamp.Before(started.Add(-2*time.Second)) || stamp.After(started.Add(30*time.Second)) {
			continue
		}
		checked++
		if checked > 20 {
			return ""
		}
		path := filepath.Join(directory, entry.Name())
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		header, err := io.ReadAll(io.LimitReader(file, 16*1024))
		file.Close()
		if err != nil || !strings.Contains(string(header), "workspaceDirs=["+workspace+" "+workspace+"]") {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().Before(started) || info.ModTime().After(finished.Add(5*time.Second)) {
			continue
		}
		if providerStorageExhausted(path, home) {
			file, err = os.Open(path)
			if err != nil {
				continue
			}
			if info.Size() > maxProviderDiagnosticRead {
				_, err = file.Seek(info.Size()-maxProviderDiagnosticRead, io.SeekStart)
			}
			tail, readErr := io.ReadAll(io.LimitReader(file, maxProviderDiagnosticRead))
			file.Close()
			if err != nil || readErr != nil {
				continue
			}
			digest := sha256.Sum256(append(header, tail...))
			return hex.EncodeToString(digest[:])
		}
	}
	return ""
}
