package deployer

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrZipSlip is returned when an archive entry attempts directory traversal.
	ErrZipSlip = errors.New("zipslip detected: invalid path in archive")

	// ErrEmptyArchive is returned when an archive contains no valid files.
	ErrEmptyArchive = errors.New("empty zip archive")
)

// ExtractZip securely extracts a ZIP archive into destDir with strict anti-ZipSlip sanitization.
// It rejects any entry containing path traversals (".."), absolute paths, or escaping paths.
func ExtractZip(zipReader io.ReaderAt, size int64, destDir string) error {
	if size <= 0 {
		return ErrEmptyArchive
	}

	cleanDest := filepath.Clean(destDir)
	if err := os.MkdirAll(cleanDest, 0755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	r, err := zip.NewReader(zipReader, size)
	if err != nil {
		return fmt.Errorf("open zip reader: %w", err)
	}

	if len(r.File) == 0 {
		return ErrEmptyArchive
	}

	for _, f := range r.File {
		if err := extractFile(f, cleanDest); err != nil {
			return err
		}
	}

	return nil
}

// extractFile validates and extracts an individual zip entry.
func extractFile(f *zip.File, cleanDest string) error {
	rawName := f.Name

	// 1. Strict traversal pattern rejection
	if strings.Contains(rawName, "..") {
		return fmt.Errorf("%w: path contains '..': %s", ErrZipSlip, rawName)
	}

	// 2. Reject absolute paths or paths starting with slash/backslash
	if filepath.IsAbs(rawName) || strings.HasPrefix(rawName, "/") || strings.HasPrefix(rawName, "\\") {
		return fmt.Errorf("%w: absolute path forbidden: %s", ErrZipSlip, rawName)
	}

	// 3. Clean path and verify boundary prefix
	cleanName := filepath.Clean(rawName)
	targetPath := filepath.Join(cleanDest, cleanName)

	rel, err := filepath.Rel(cleanDest, targetPath)
	if err != nil || strings.HasPrefix(rel, "..") || strings.Contains(rel, "..") {
		return fmt.Errorf("%w: entry escapes destination directory: %s", ErrZipSlip, rawName)
	}

	// Handle directory
	if f.FileInfo().IsDir() {
		return os.MkdirAll(targetPath, 0755)
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("create parent directory for %s: %w", rawName, err)
	}

	// Handle regular file
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("open entry %s: %w", rawName, err)
	}
	defer rc.Close()

	// Handle symlink safety: reject symlinks that point outside destination
	if f.Mode()&os.ModeSymlink != 0 {
		symlinkTarget, err := io.ReadAll(rc)
		if err != nil {
			return fmt.Errorf("read symlink target %s: %w", rawName, err)
		}
		targetStr := string(symlinkTarget)
		if strings.Contains(targetStr, "..") || filepath.IsAbs(targetStr) {
			return fmt.Errorf("%w: symlink target escapes destination: %s -> %s", ErrZipSlip, rawName, targetStr)
		}
		cleanSymTarget := filepath.Join(parentDir, filepath.Clean(targetStr))
		symRel, err := filepath.Rel(cleanDest, cleanSymTarget)
		if err != nil || strings.HasPrefix(symRel, "..") {
			return fmt.Errorf("%w: symlink target resolves outside destination: %s -> %s", ErrZipSlip, rawName, targetStr)
		}
		return os.Symlink(targetStr, targetPath)
	}

	outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return fmt.Errorf("create file %s: %w", rawName, err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, rc); err != nil {
		return fmt.Errorf("write file %s: %w", rawName, err)
	}

	return nil
}

// DetectRuntime inspects the extracted project directory and automatically detects
// whether the runtime stack is PHP, Node.js, Static HTML, or Custom.
//
// Rules:
//   - PHP: presence of index.php or wp-config.php.
//   - Node.js: presence of package.json.
//   - Static: presence of index.html.
//   - Custom: default fallback when no specific marker is identified.
func DetectRuntime(dir string) (string, error) {
	appRoot := FindAppRoot(dir)

	// 1. PHP detection: index.php or wp-config.php
	if fileExists(filepath.Join(appRoot, "index.php")) || fileExists(filepath.Join(appRoot, "wp-config.php")) {
		return RuntimePHP, nil
	}

	// 2. Node.js detection: package.json
	if fileExists(filepath.Join(appRoot, "package.json")) {
		return RuntimeNodeJS, nil
	}

	// 3. Static HTML detection: index.html
	if fileExists(filepath.Join(appRoot, "index.html")) {
		return RuntimeStatic, nil
	}

	return RuntimeCustom, nil
}

// FindAppRoot resolves the root directory of an application. If an archive was wrapped
// in a single top-level directory without root marker files, it returns that subfolder.
func FindAppRoot(dir string) string {
	cleanDir := filepath.Clean(dir)

	// Check if marker files already exist at root
	if fileExists(filepath.Join(cleanDir, "index.php")) ||
		fileExists(filepath.Join(cleanDir, "wp-config.php")) ||
		fileExists(filepath.Join(cleanDir, "package.json")) ||
		fileExists(filepath.Join(cleanDir, "index.html")) {
		return cleanDir
	}

	entries, err := os.ReadDir(cleanDir)
	if err != nil {
		return cleanDir
	}

	// If there is exactly one entry and it's a directory, inspect inside
	var subDirs []string
	for _, entry := range entries {
		// Ignore hidden files/dirs like __MACOSX, .git
		if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "__MACOSX" {
			continue
		}
		if entry.IsDir() {
			subDirs = append(subDirs, entry.Name())
		} else {
			// Found a regular file at root, keep root
			return cleanDir
		}
	}

	if len(subDirs) == 1 {
		nestedRoot := filepath.Join(cleanDir, subDirs[0])
		return nestedRoot
	}

	return cleanDir
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
