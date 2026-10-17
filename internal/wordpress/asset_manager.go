package wordpress

import (
	"archive/zip"
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	cuanDB "github.com/buchin/cuan/internal/db"
)

var (
	ErrInvalidAssetZip       = errors.New("invalid or corrupted zip archive")
	ErrInvalidWordPressAsset = errors.New("not a valid WordPress theme or plugin: missing style.css with Theme Name or PHP plugin header")
)

// AssetHeaderInfo holds detected metadata from a theme style.css or plugin PHP file.
type AssetHeaderInfo struct {
	Kind        string `json:"kind"` // "theme" or "plugin"
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
	Author      string `json:"author,omitempty"`
	AuthorURI   string `json:"author_uri,omitempty"`
	AssetURI    string `json:"asset_uri,omitempty"`
}

// SanitizeSlug converts a name or string into a safe WordPress slug (lowercase alphanumeric and hyphens).
func SanitizeSlug(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var sb strings.Builder
	lastDash := false
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
			lastDash = false
		} else if r == '-' || r == '_' || unicode.IsSpace(r) {
			if !lastDash && sb.Len() > 0 {
				sb.WriteRune('-')
				lastDash = true
			}
		}
	}
	res := strings.Trim(sb.String(), "-")
	if res == "" {
		res = "asset"
	}
	return res
}

// ParseThemeHeader extracts theme metadata from style.css content.
func ParseThemeHeader(content string) (*AssetHeaderInfo, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	info := &AssetHeaderInfo{
		Kind: cuanDB.PluginThemeKindTheme,
	}

	headerMap := map[string]*string{
		"theme name":  &info.Name,
		"theme uri":   &info.AssetURI,
		"description": &info.Description,
		"author":      &info.Author,
		"author uri":  &info.AuthorURI,
		"version":     &info.Version,
	}

	foundThemeName := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "*/") {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.ToLower(strings.TrimSpace(strings.TrimLeft(parts[0], "/* ")))
			val := strings.TrimSpace(parts[1])
			if target, ok := headerMap[key]; ok && *target == "" {
				*target = val
				if key == "theme name" {
					foundThemeName = true
				}
			}
		}
	}

	if !foundThemeName || info.Name == "" {
		return nil, errors.New("style.css does not contain a valid 'Theme Name' header")
	}

	if info.Slug == "" {
		info.Slug = SanitizeSlug(info.Name)
	}
	return info, nil
}

// ParsePluginHeader extracts plugin metadata from a PHP file.
func ParsePluginHeader(content string) (*AssetHeaderInfo, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	info := &AssetHeaderInfo{
		Kind: cuanDB.PluginThemeKindPlugin,
	}

	headerMap := map[string]*string{
		"plugin name": &info.Name,
		"plugin uri":  &info.AssetURI,
		"description": &info.Description,
		"author":      &info.Author,
		"author uri":  &info.AuthorURI,
		"version":     &info.Version,
	}

	foundPluginName := false
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		if lineCount > 100 { // Headers are always in the first 100 lines
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "*/") && foundPluginName {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.ToLower(strings.TrimSpace(strings.TrimLeft(parts[0], "/* ")))
			val := strings.TrimSpace(parts[1])
			if target, ok := headerMap[key]; ok && *target == "" {
				*target = val
				if key == "plugin name" {
					foundPluginName = true
				}
			}
		}
	}

	if !foundPluginName || info.Name == "" {
		return nil, errors.New("php file does not contain a valid 'Plugin Name' header")
	}

	if info.Slug == "" {
		info.Slug = SanitizeSlug(info.Name)
	}
	return info, nil
}

// InspectAssetZip analyzes a zip archive, verifies ZipSlip safety for all entries,
// detects whether it is a theme or plugin, and extracts header info and root directory prefix.
func InspectAssetZip(zipReader *zip.Reader, expectedKind string, customSlug string) (*AssetHeaderInfo, string, error) {
	if zipReader == nil || len(zipReader.File) == 0 {
		return nil, "", ErrInvalidAssetZip
	}

	expectedKind = strings.ToLower(strings.TrimSpace(expectedKind))
	if expectedKind != "" && expectedKind != cuanDB.PluginThemeKindTheme && expectedKind != cuanDB.PluginThemeKindPlugin {
		return nil, "", fmt.Errorf("invalid expected asset kind: %s (must be theme or plugin)", expectedKind)
	}

	// 1. Pre-scan all entry paths for ZipSlip security
	for _, f := range zipReader.File {
		if err := CheckZipSlipEntry(f.Name); err != nil {
			return nil, "", fmt.Errorf("%w: entry %q", ErrZipSlipDetected, f.Name)
		}
	}

	// 2. Identify top-level folder prefix if entire archive is wrapped inside one directory
	rootPrefix := ""
	firstEntryName := zipReader.File[0].Name
	if idx := strings.Index(firstEntryName, "/"); idx != -1 {
		candidatePrefix := firstEntryName[:idx]
		allMatch := true
		for _, f := range zipReader.File {
			clean := strings.TrimPrefix(f.Name, "/")
			if clean == candidatePrefix {
				continue
			}
			if !strings.HasPrefix(clean, candidatePrefix+"/") {
				allMatch = false
				break
			}
		}
		if allMatch && candidatePrefix != "" && candidatePrefix != "." && candidatePrefix != ".." {
			rootPrefix = candidatePrefix
		}
	}

	var themeHeader *AssetHeaderInfo
	var pluginHeader *AssetHeaderInfo

	// Read small buffer helper
	readFileSnippet := func(f *zip.File, maxBytes int64) (string, error) {
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		lr := io.LimitReader(rc, maxBytes)
		bytes, err := io.ReadAll(lr)
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	}

	// 3. Search for style.css (theme) or PHP files with Plugin Name (plugin)
	for _, f := range zipReader.File {
		cleanName := filepath.ToSlash(f.Name)
		baseName := filepath.Base(cleanName)

		// Check for theme style.css
		if strings.EqualFold(baseName, "style.css") {
			// style.css must be at root or right under rootPrefix
			dir := filepath.Dir(cleanName)
			if dir == "." || dir == "" || (rootPrefix != "" && dir == rootPrefix) {
				snippet, err := readFileSnippet(f, 16384)
				if err == nil {
					header, err := ParseThemeHeader(snippet)
					if err == nil && header != nil {
						themeHeader = header
					}
				}
			}
		}

		// Check for plugin header in PHP files
		if strings.HasSuffix(strings.ToLower(baseName), ".php") && pluginHeader == nil {
			dir := filepath.Dir(cleanName)
			if dir == "." || dir == "" || (rootPrefix != "" && dir == rootPrefix) {
				snippet, err := readFileSnippet(f, 16384)
				if err == nil {
					header, err := ParsePluginHeader(snippet)
					if err == nil && header != nil {
						pluginHeader = header
					}
				}
			}
		}
	}

	var selected *AssetHeaderInfo
	if expectedKind == cuanDB.PluginThemeKindTheme {
		if themeHeader == nil {
			return nil, "", fmt.Errorf("%w: requested theme but no valid style.css header found", ErrInvalidWordPressAsset)
		}
		selected = themeHeader
	} else if expectedKind == cuanDB.PluginThemeKindPlugin {
		if pluginHeader == nil {
			return nil, "", fmt.Errorf("%w: requested plugin but no valid PHP plugin header found", ErrInvalidWordPressAsset)
		}
		selected = pluginHeader
	} else {
		// Auto-detect: prefer theme if style.css has Theme Name, else plugin
		if themeHeader != nil {
			selected = themeHeader
		} else if pluginHeader != nil {
			selected = pluginHeader
		} else {
			return nil, "", ErrInvalidWordPressAsset
		}
	}

	// Determine slug
	slug := strings.TrimSpace(customSlug)
	if slug == "" {
		if rootPrefix != "" {
			slug = SanitizeSlug(rootPrefix)
		} else {
			slug = SanitizeSlug(selected.Name)
		}
	} else {
		slug = SanitizeSlug(slug)
	}
	selected.Slug = slug

	return selected, rootPrefix, nil
}

// CheckZipSlipEntry verifies that a zip entry name does not contain traversal paths or absolute roots.
func CheckZipSlipEntry(name string) error {
	if name == "" {
		return nil
	}
	// Check for null bytes
	if strings.ContainsRune(name, 0) {
		return errors.New("contains null byte")
	}
	// Reject paths starting with / or \
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\") {
		return errors.New("absolute path not allowed")
	}
	// Disallow drive letters (e.g., C:)
	if len(name) >= 2 && unicode.IsLetter(rune(name[0])) && name[1] == ':' {
		return errors.New("drive letter not allowed")
	}
	// Reject double-dot traversal sequences in raw string
	if strings.Contains(name, "..") {
		// Look for .. as path segment
		parts := strings.FieldsFunc(name, func(r rune) bool {
			return r == '/' || r == '\\'
		})
		for _, p := range parts {
			if p == ".." {
				return errors.New("contains parent directory reference (..)")
			}
		}
	}
	// Clean check
	clean := filepath.Clean(name)
	if strings.HasPrefix(clean, "..") || strings.HasPrefix(clean, "/..") || strings.HasPrefix(clean, "\\..") {
		return errors.New("clean path escapes root")
	}
	return nil
}

// ExtractAssetZip extracts all files safely to destDir with canonical ZipSlip protection.
// If rootPrefix is non-empty, that leading directory segment is stripped so content is extracted directly.
func ExtractAssetZip(zipReader *zip.Reader, destDir string, rootPrefix string) error {
	cleanDest := filepath.Clean(destDir)
	if err := os.MkdirAll(cleanDest, 0755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	maxFileSize := int64(100 * 1024 * 1024) // 100 MB max per file limit
	totalExtracted := int64(0)
	maxTotalExtracted := int64(500 * 1024 * 1024) // 500 MB max archive total limit

	for _, f := range zipReader.File {
		if err := CheckZipSlipEntry(f.Name); err != nil {
			return fmt.Errorf("%w: %s (%v)", ErrZipSlipDetected, f.Name, err)
		}

		relPath := filepath.ToSlash(f.Name)
		if rootPrefix != "" {
			relPath = strings.TrimPrefix(relPath, rootPrefix)
			relPath = strings.TrimPrefix(relPath, "/")
		}

		if relPath == "" || relPath == "." {
			continue
		}

		targetPath := filepath.Join(cleanDest, filepath.FromSlash(relPath))
		cleanTarget := filepath.Clean(targetPath)

		// Canonical ZipSlip verification against cleanDest
		if !strings.HasPrefix(cleanTarget, cleanDest+string(filepath.Separator)) && cleanTarget != cleanDest {
			return fmt.Errorf("%w: %s resolves to %s outside %s", ErrZipSlipDetected, f.Name, cleanTarget, cleanDest)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(cleanTarget, 0755); err != nil {
				return fmt.Errorf("create directory %s: %w", cleanTarget, err)
			}
			continue
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(cleanTarget), 0755); err != nil {
			return fmt.Errorf("create parent dir for %s: %w", cleanTarget, err)
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open zip entry %s: %w", f.Name, err)
		}

		outFile, err := os.OpenFile(cleanTarget, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode()|0644)
		if err != nil {
			rc.Close()
			return fmt.Errorf("create output file %s: %w", cleanTarget, err)
		}

		lr := io.LimitReader(rc, maxFileSize+1)
		copied, copyErr := io.Copy(outFile, lr)
		outFile.Close()
		rc.Close()

		if copyErr != nil {
			return fmt.Errorf("write output file %s: %w", cleanTarget, copyErr)
		}
		if copied > maxFileSize {
			_ = os.Remove(cleanTarget)
			return fmt.Errorf("zip entry %s exceeds maximum file size limit", f.Name)
		}

		totalExtracted += copied
		if totalExtracted > maxTotalExtracted {
			return errors.New("zip archive exceeds maximum total extracted size limit")
		}
	}

	return nil
}

// CleanSlugRegex validates slug characters.
var CleanSlugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9\-_]{0,62}$`)
