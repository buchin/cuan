package browserfetch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const Version = "0.2.6"

// Manifest is produced by the native installer. Paths are selected by Cuan,
// not by browser tool inputs or ambient provider/browser profiles.
type Manifest struct {
	Runtime
	Version       string `json:"version"`
	ArchiveSHA256 string `json:"archive_sha256"`
	BinarySHA256  string `json:"binary_sha256"`
}

// LoadManifest checks the selected private manifest and actual installed binary.
// Installation authority comes from the trusted installer and filesystem;
// manifest content is not an authentication token or a new tool permission.
func LoadManifest(path string) (*Manifest, error) {
	invalid := errors.New("verified chat browser runtime is unavailable")
	if !filepath.IsAbs(path) {
		return nil, invalid
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16*1024 || info.Mode().Perm()&0077 != 0 {
		return nil, invalid
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, invalid
	}
	var manifest Manifest
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&manifest) != nil || decoder.Decode(new(any)) != io.EOF || manifest.Version != Version {
		return nil, invalid
	}
	for _, hash := range []string{manifest.ArchiveSHA256, manifest.BinarySHA256} {
		decoded, err := hex.DecodeString(hash)
		if err != nil || len(decoded) != sha256.Size {
			return nil, invalid
		}
	}
	for _, name := range []string{manifest.Binary, manifest.Sandbox, manifest.LockFile} {
		if !filepath.IsAbs(name) {
			return nil, invalid
		}
	}
	lock, err := os.Lstat(manifest.LockFile)
	if err != nil || !lock.Mode().IsRegular() || lock.Mode().Perm()&0077 != 0 {
		return nil, invalid
	}
	if (manifest.Loader == "") != (manifest.LibraryDir == "") {
		return nil, invalid
	}
	if manifest.Loader != "" && (!filepath.IsAbs(manifest.Loader) || !filepath.IsAbs(manifest.LibraryDir)) {
		return nil, invalid
	}
	if manifest.Loader != "" {
		loader, err := os.Stat(manifest.Loader)
		if err != nil || !loader.Mode().IsRegular() || loader.Mode().Perm()&0111 == 0 {
			return nil, invalid
		}
		libraries, err := os.Stat(manifest.LibraryDir)
		if err != nil || !libraries.IsDir() {
			return nil, invalid
		}
	}
	engineInfo, err := os.Lstat(manifest.Binary)
	if err != nil || !engineInfo.Mode().IsRegular() {
		return nil, invalid
	}
	sandbox, err := os.Stat(manifest.Sandbox)
	if err != nil || !sandbox.Mode().IsRegular() || sandbox.Mode().Perm()&0111 == 0 {
		return nil, invalid
	}
	file, err := os.Open(manifest.Binary)
	if err != nil {
		return nil, invalid
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 || info.Size() > 64*1024*1024 {
		return nil, invalid
	}
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(file, 64*1024*1024+1)); err != nil || hex.EncodeToString(h.Sum(nil)) != manifest.BinarySHA256 {
		return nil, invalid
	}
	return &manifest, nil
}
