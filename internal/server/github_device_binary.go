package server

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

var ensureGitHubDeviceBinary = prepareGitHubDeviceBinary
var githubCLIInstallMu sync.Mutex
var githubCLIReleaseAPI = "https://api.github.com/repos/cli/cli/releases/latest"
var githubCLIReleaseBase = "https://github.com/cli/cli/releases/download/"
var githubCLIDownloadHTTP = &http.Client{Timeout: 45 * time.Second}

func githubCLIGet(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Cuan GitHub sign-in")
	response, err := githubCLIDownloadHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, errors.New("GitHub CLI download unavailable")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, errors.New("invalid GitHub CLI download")
	}
	return data, nil
}
func githubCLIBinary(archive []byte, expectedPath string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	reader := tar.NewReader(io.LimitReader(gz, 140<<20))
	for i := 0; i < 1000; i++ {
		header, err := reader.Next()
		if err != nil {
			return nil, errors.New("GitHub CLI binary missing")
		}
		if header.Name != expectedPath {
			continue
		}
		if header.Typeflag != tar.TypeReg || header.Size <= 0 || header.Size > 100<<20 {
			return nil, errors.New("invalid GitHub CLI binary")
		}
		data, err := io.ReadAll(io.LimitReader(reader, header.Size+1))
		if err != nil || int64(len(data)) != header.Size {
			return nil, errors.New("invalid GitHub CLI binary")
		}
		return data, nil
	}
	return nil, errors.New("invalid GitHub CLI archive")
}
func prepareGitHubDeviceBinary(ctx context.Context) (string, error) {
	if binary, err := exec.LookPath("gh"); err == nil {
		return binary, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	binary := filepath.Join(home, ".local", "share", "cuan", "bin", "gh")
	githubCLIInstallMu.Lock()
	defer githubCLIInstallMu.Unlock()
	if info, err := os.Stat(binary); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
		return binary, nil
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return "", errors.New("install GitHub CLI on this platform")
	}
	raw, err := githubCLIGet(ctx, githubCLIReleaseAPI, 1<<20)
	if err != nil {
		return "", err
	}
	var release struct {
		Tag string `json:"tag_name"`
	}
	if json.Unmarshal(raw, &release) != nil || !regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(release.Tag) {
		return "", errors.New("invalid GitHub CLI release")
	}
	version := strings.TrimPrefix(release.Tag, "v")
	stem := "gh_" + version + "_linux_" + runtime.GOARCH
	archiveName := stem + ".tar.gz"
	sums, err := githubCLIGet(ctx, githubCLIReleaseBase+release.Tag+"/gh_"+version+"_checksums.txt", 2<<20)
	if err != nil {
		return "", err
	}
	expected := ""
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == archiveName {
			expected = fields[0]
		}
	}
	if len(expected) != 64 {
		return "", errors.New("GitHub CLI checksum missing")
	}
	archive, err := githubCLIGet(ctx, githubCLIReleaseBase+release.Tag+"/"+archiveName, 40<<20)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(archive)
	if !strings.EqualFold(hex.EncodeToString(hash[:]), expected) {
		return "", errors.New("GitHub CLI checksum mismatch")
	}
	executable, err := githubCLIBinary(archive, stem+"/bin/gh")
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Dir(binary), 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Dir(binary), ".gh-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(executable); err == nil {
		err = f.Chmod(0755)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(f.Name(), binary)
	}
	return binary, err
}
