package inspector

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SafeUnpacker handles isolated streaming unzip operations with strict Zip Slip protection and quota limits.
type SafeUnpacker struct {
	maxUncompressedSize int64
	maxFileSize         int64
	maxFilesCount       int
}

// NewSafeUnpacker constructs a SafeUnpacker with configured limits or defaults.
func NewSafeUnpacker(maxSize int64) *SafeUnpacker {
	if maxSize <= 0 {
		maxSize = DefaultMaxZipSize
	}
	return &SafeUnpacker{
		maxUncompressedSize: maxSize,
		maxFileSize:         maxSize,
		maxFilesCount:       DefaultMaxFilesCount,
	}
}

// SanitizeZipEntryPath verifies that the entry relative path stays within destDir.
// Returns an error (ErrZipSlip) if the path attempts any directory traversal or escapes destDir.
func SanitizeZipEntryPath(destDir, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", ErrZipSlip
	}

	// 1. Normalize all backslashes to forward slashes for uniform cross-platform parsing
	normalized := strings.ReplaceAll(name, "\\", "/")

	// 2. Reject absolute paths or Windows drive letters
	if strings.HasPrefix(normalized, "/") || filepath.IsAbs(normalized) {
		return "", ErrZipSlip
	}
	if len(normalized) >= 2 && normalized[1] == ':' {
		return "", ErrZipSlip
	}

	// 3. Reject any path component explicitly referencing parent directory
	parts := strings.Split(normalized, "/")
	for _, part := range parts {
		if part == ".." {
			return "", ErrZipSlip
		}
	}

	// 4. Clean path using filepath.Clean
	cleanPath := filepath.Clean(normalized)
	if cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) || strings.HasPrefix(cleanPath, "../") {
		return "", ErrZipSlip
	}

	// 5. Join with destination directory
	destDirClean := filepath.Clean(destDir)
	targetPath := filepath.Join(destDirClean, cleanPath)

	// 6. Verify target path is strictly within destDirClean
	rel, err := filepath.Rel(destDirClean, targetPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || strings.HasPrefix(rel, "../") {
		return "", ErrZipSlip
	}

	return targetPath, nil
}

// Unpack extracts a zip archive from an io.ReaderAt into the specified destination directory.
func (u *SafeUnpacker) Unpack(r io.ReaderAt, size int64, destDir string) (*UnpackResult, error) {
	if size <= 0 {
		return nil, ErrEmptyArchive
	}

	zipReader, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidZip, err.Error())
	}

	return u.UnpackZipReader(zipReader, destDir)
}

// UnpackZipReader extracts entries from an existing *zip.Reader with safety checks.
func (u *SafeUnpacker) UnpackZipReader(zipReader *zip.Reader, destDir string) (*UnpackResult, error) {
	if len(zipReader.File) == 0 {
		return nil, ErrEmptyArchive
	}
	if len(zipReader.File) > u.maxFilesCount {
		return nil, ErrMaxFilesExceeded
	}

	destDirClean := filepath.Clean(destDir)
	if err := os.MkdirAll(destDirClean, 0755); err != nil {
		return nil, fmt.Errorf("create destination directory: %w", err)
	}

	var (
		totalBytes int64
		totalFiles int
		totalDirs  int
		extracted  []string
		topDirs    = make(map[string]bool)
	)

	// Clean up partially extracted files on failure
	cleanupOnErr := func() {
		_ = os.RemoveAll(destDirClean)
	}

	for _, file := range zipReader.File {
		// Pre-flight check on header size
		if file.UncompressedSize64 > uint64(u.maxFileSize) {
			cleanupOnErr()
			return nil, ErrQuotaExceeded
		}
		if totalBytes+int64(file.UncompressedSize64) > u.maxUncompressedSize {
			cleanupOnErr()
			return nil, ErrQuotaExceeded
		}

		targetPath, err := SanitizeZipEntryPath(destDirClean, file.Name)
		if err != nil {
			cleanupOnErr()
			return nil, err
		}

		// Disallow symlinks to prevent arbitrary link-traversal attacks
		if file.Mode()&os.ModeSymlink != 0 {
			cleanupOnErr()
			return nil, fmt.Errorf("%w: symlinks in archives are prohibited", ErrZipSlip)
		}

		// Track top-level prefix
		relPath, _ := filepath.Rel(destDirClean, targetPath)
		firstSeg := strings.Split(relPath, string(filepath.Separator))[0]
		if firstSeg != "" {
			topDirs[firstSeg] = true
		}

		if file.FileInfo().IsDir() {
			totalDirs++
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				cleanupOnErr()
				return nil, fmt.Errorf("create dir '%s': %w", targetPath, err)
			}
			continue
		}

		// Ensure parent directory exists
		parentDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			cleanupOnErr()
			return nil, fmt.Errorf("create parent dir for '%s': %w", targetPath, err)
		}

		// Extract file with bounded streaming limit
		rc, err := file.Open()
		if err != nil {
			cleanupOnErr()
			return nil, fmt.Errorf("open zip entry '%s': %w", file.Name, err)
		}

		dstFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode().Perm())
		if err != nil {
			rc.Close()
			cleanupOnErr()
			return nil, fmt.Errorf("create destination file '%s': %w", targetPath, err)
		}

		// Guard against zip bombs with spoofed uncompressed header sizes
		remainingQuota := u.maxUncompressedSize - totalBytes
		limitReader := io.LimitReader(rc, remainingQuota+1)

		written, err := io.Copy(dstFile, limitReader)
		_ = dstFile.Close()
		_ = rc.Close()

		if err != nil {
			cleanupOnErr()
			return nil, fmt.Errorf("extract entry '%s': %w", file.Name, err)
		}

		totalBytes += written
		if totalBytes > u.maxUncompressedSize {
			cleanupOnErr()
			return nil, ErrQuotaExceeded
		}

		totalFiles++
		extracted = append(extracted, relPath)
	}

	var rootPrefix string
	if len(topDirs) == 1 {
		for dir := range topDirs {
			// Check if this single top item is indeed a directory
			candidate := filepath.Join(destDirClean, dir)
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				rootPrefix = dir
			}
		}
	}

	return &UnpackResult{
		DestDir:     destDirClean,
		TotalFiles:  totalFiles,
		TotalDirs:   totalDirs,
		TotalBytes:  totalBytes,
		RootPrefix:  rootPrefix,
		ExtractedAt: time.Now().UTC(),
		Files:       extracted,
	}, nil
}

// UnpackStream reads a stream into a temporary spool file, then safely extracts it.
func (u *SafeUnpacker) UnpackStream(r io.Reader, destDir string) (*UnpackResult, error) {
	tmpFile, err := os.CreateTemp("", "cuan-upload-*.zip")
	if err != nil {
		return nil, fmt.Errorf("create temp spool: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	// Copy stream with size quota
	limitReader := io.LimitReader(r, u.maxUncompressedSize+1)
	written, err := io.Copy(tmpFile, limitReader)
	if err != nil {
		return nil, fmt.Errorf("spool zip stream: %w", err)
	}
	if written > u.maxUncompressedSize {
		return nil, ErrQuotaExceeded
	}

	return u.Unpack(tmpFile, written, destDir)
}
