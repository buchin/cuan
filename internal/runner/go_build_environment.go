package runner

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

// AI profiles own authentication, not compiler state. Keep Go caches stable
// across an owner's brain failovers without sharing another owner's modules or
// importing the host's Go configuration/credentials. Go verifies/downloads the
// toolchain declared by the repository through its normal toolchain mechanism.
func managedGoBuildEnvironment(root, base, owner string) (map[string]string, error) {
	if base == "" || owner == "" {
		return nil, nil
	}
	hasGo := false
	for _, name := range []string{"go.mod", "go.work"} {
		if info, err := os.Lstat(filepath.Join(root, name)); err == nil && info.Mode().IsRegular() {
			hasGo = true
		}
	}
	if !hasGo {
		return nil, nil
	}
	absolute, err := filepath.Abs(base)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(absolute, 0700); err != nil {
		return nil, fmt.Errorf("prepare managed Go cache: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(owner))
	directory := filepath.Join(canonical, fmt.Sprintf("%x", digest))
	for _, relative := range []string{".", "go", "go/pkg", "go/pkg/mod", "go-build"} {
		path := filepath.Join(directory, relative)
		if err = os.MkdirAll(path, 0700); err != nil {
			return nil, fmt.Errorf("prepare managed Go cache: %w", err)
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || resolved != path {
			return nil, fmt.Errorf("managed Go cache must not use shared directory links")
		}
	}
	return map[string]string{"GOPATH": filepath.Join(directory, "go"), "GOMODCACHE": filepath.Join(directory, "go", "pkg", "mod"), "GOCACHE": filepath.Join(directory, "go-build"), "GOENV": "off", "GOTOOLCHAIN": "auto"}, nil
}
