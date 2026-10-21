package server

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGitHubCLIInstallChecksumsPrivateCacheAndExactArchiveEntry(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skip("native Linux CLI")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())
	stem := "gh_9.9.9_linux_" + runtime.GOARCH
	name := stem + ".tar.gz"
	expected := []byte("#!/bin/sh\nexit 0\n")
	var buffer bytes.Buffer
	zip := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(zip)
	for path, data := range map[string][]byte{"../../do-not-extract": []byte("not a binary"), stem + "/bin/gh": expected} {
		if err := archive.WriteHeader(&tar.Header{Name: path, Mode: 0755, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		archive.Write(data)
	}
	archive.Close()
	zip.Close()
	hash := sha256.Sum256(buffer.Bytes())
	checksum := hex.EncodeToString(hash[:])
	requests := 0
	corrupt := false
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch r.URL.Path {
		case "/latest":
			w.Write([]byte(`{"tag_name":"v9.9.9"}`))
		case "/v9.9.9/gh_9.9.9_checksums.txt":
			w.Write([]byte(checksum + "  " + name + "\n"))
		case "/v9.9.9/" + name:
			if corrupt {
				w.Write([]byte("tampered"))
			} else {
				w.Write(buffer.Bytes())
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer provider.Close()
	oldAPI, oldBase := githubCLIReleaseAPI, githubCLIReleaseBase
	githubCLIReleaseAPI = provider.URL + "/latest"
	githubCLIReleaseBase = provider.URL + "/"
	defer func() { githubCLIReleaseAPI = oldAPI; githubCLIReleaseBase = oldBase }()
	binary, err := prepareGitHubDeviceBinary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if binary != filepath.Join(home, ".local", "share", "cuan", "bin", "gh") {
		t.Fatal("unexpected binary cache")
	}
	raw, _ := os.ReadFile(binary)
	if !bytes.Equal(raw, expected) {
		t.Fatal("wrong archive entry")
	}
	info, _ := os.Stat(binary)
	if info.Mode().Perm() != 0755 {
		t.Fatal("binary not executable")
	}
	count := requests
	if _, err := prepareGitHubDeviceBinary(context.Background()); err != nil || requests != count {
		t.Fatal("cache ignored", err)
	}
	os.Remove(binary)
	corrupt = true
	if _, err := prepareGitHubDeviceBinary(context.Background()); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatal("corrupt binary accepted", err)
	}
	if _, err := os.Stat(binary); !os.IsNotExist(err) {
		t.Fatal("unverified binary installed")
	}
}
