package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const dependencyLockFingerprintFilename = ".cuan-lock-fingerprint"

// dependencyLockFingerprint returns the hex-encoded SHA-256 digest of the lockfile.
func dependencyLockFingerprint(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

// checkLockfileWarm returns true if the dependency directory (e.g. node_modules or vendor)
// exists as a regular directory and contains a fingerprint matching the expected lock digest.
func checkLockfileWarm(directory, modulesName, expectedFingerprint string) bool {
	if expectedFingerprint == "" {
		return false
	}
	targetDir := filepath.Join(directory, modulesName)
	info, err := os.Lstat(targetDir)
	if err != nil || !info.IsDir() {
		return false
	}
	fingerprintPath := filepath.Join(targetDir, dependencyLockFingerprintFilename)
	data, err := os.ReadFile(fingerprintPath)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == expectedFingerprint
}

// recordLockfileFingerprint records the verified lockfile digest into the dependency directory.
func recordLockfileFingerprint(directory, modulesName, fingerprint string) error {
	if fingerprint == "" {
		return nil
	}
	targetDir := filepath.Join(directory, modulesName)
	if info, err := os.Lstat(targetDir); err != nil || !info.IsDir() {
		return nil
	}
	fingerprintPath := filepath.Join(targetDir, dependencyLockFingerprintFilename)
	return os.WriteFile(fingerprintPath, []byte(fingerprint), 0600)
}

// seedWarmDependencies checks for verified dependencies in the source workspace (such as
// a developer workspace) and seeds them into the target workspace (such as a verifier snapshot)
// only when the target lockfile matches the verified source lockfile fingerprint.
func seedWarmDependencies(source, target string) {
	if source == "" || target == "" || source == target {
		return
	}
	// 1. Seed npm dependencies for known roots (".", "web")
	for _, relative := range []string{".", "web"} {
		targetLockPath := filepath.Join(target, relative, "package-lock.json")
		targetInfo, err := os.Lstat(targetLockPath)
		if err != nil || !targetInfo.Mode().IsRegular() || targetInfo.Size() > 8<<20 {
			continue
		}
		targetRaw, err := os.ReadFile(targetLockPath)
		if err != nil {
			continue
		}
		targetFingerprint := dependencyLockFingerprint(targetRaw)

		targetModules := filepath.Join(target, relative, "node_modules")
		if info, err := os.Lstat(targetModules); err == nil && info.IsDir() {
			continue // Already present in target
		}

		sourceModules := filepath.Join(source, relative, "node_modules")
		sourceInfo, err := os.Lstat(sourceModules)
		if err != nil || !sourceInfo.IsDir() {
			continue
		}

		sourceFingerprintPath := filepath.Join(sourceModules, dependencyLockFingerprintFilename)
		sourceFingerprintData, err := os.ReadFile(sourceFingerprintPath)
		if err != nil || strings.TrimSpace(string(sourceFingerprintData)) != targetFingerprint {
			continue // Lockfile does not match or source is unverified
		}

		_ = seedWarmDirectory(sourceModules, targetModules, targetFingerprint)
	}
}

// seedWarmDirectory copies/links a warm dependency tree into target atomically.
func seedWarmDirectory(src, dst, fingerprint string) error {
	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	tmpDst := filepath.Join(parent, fmt.Sprintf(".cuan-warm-%s-%d", filepath.Base(dst), time.Now().UnixNano()))
	_ = os.RemoveAll(tmpDst)
	if err := linkOrCopyDirectory(src, tmpDst); err != nil {
		_ = os.RemoveAll(tmpDst)
		return err
	}
	if err := os.WriteFile(filepath.Join(tmpDst, dependencyLockFingerprintFilename), []byte(fingerprint), 0600); err != nil {
		_ = os.RemoveAll(tmpDst)
		return err
	}
	_ = os.RemoveAll(dst)
	if err := os.Rename(tmpDst, dst); err != nil {
		_ = os.RemoveAll(tmpDst)
		return err
	}
	return nil
}

// linkOrCopyDirectory copies/links the directory tree from src to dst.
func linkOrCopyDirectory(src, dst string) error {
	srcInfo, err := os.Lstat(src)
	if err != nil || !srcInfo.IsDir() {
		return fmt.Errorf("source must be an available directory")
	}
	if err := os.MkdirAll(dst, 0700); err != nil {
		return err
	}
	return filepath.Walk(src, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, srcPath)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		dstPath := filepath.Join(dst, rel)
		mode := info.Mode()
		if mode.IsDir() {
			return os.MkdirAll(dstPath, 0700)
		}
		if mode&os.ModeSymlink != 0 {
			target, err := os.Readlink(srcPath)
			if err != nil {
				return err
			}
			if filepath.IsAbs(target) {
				return nil
			}
			cleaned := filepath.Clean(filepath.Join(filepath.Dir(srcPath), target))
			if !strings.HasPrefix(cleaned, src) {
				return nil
			}
			_ = os.Remove(dstPath)
			return os.Symlink(target, dstPath)
		}
		if mode.IsRegular() {
			_ = os.Remove(dstPath)
			if err := os.Link(srcPath, dstPath); err == nil {
				return nil
			}
			return copyRegularFile(srcPath, dstPath, mode.Perm())
		}
		return nil
	})
}

func copyRegularFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
