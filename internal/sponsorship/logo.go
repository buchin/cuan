package sponsorship

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	MaxLogoSizeBytes = 2 * 1024 * 1024 // 2MB
	MIMETypePNG      = "image/png"
	MIMETypeWebP     = "image/webp"
	MIMETypeSVG      = "image/svg+xml"
)

var (
	ErrFileTooLarge        = errors.New("Logo file exceeds the 2 MB limit")
	ErrEmptyFile           = errors.New("Logo file is empty")
	ErrUnsupportedMimeType = errors.New("Unsupported file type. Only SVG, PNG, and WebP are allowed")
	ErrMaliciousSVG        = errors.New("Upload rejected. Unsafe scripts or an XSS payload were detected")
	ErrInvalidSVG          = errors.New("Invalid SVG file format")
)

var (
	pngMagic  = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	riffMagic = []byte("RIFF")
	webpMagic = []byte("WEBP")

	// Regexes for detecting and sanitizing SVG
	scriptTagRegex   = regexp.MustCompile(`(?is)<\s*script[^>]*>.*?<\s*/\s*script\s*>|<\s*script[^>]*/>|<\s*script[^>]*>`)
	foreignObjRegex  = regexp.MustCompile(`(?is)<\s*foreignObject[^>]*>.*?<\s*/\s*foreignObject\s*>|<\s*foreignObject[^>]*/>|<\s*foreignObject[^>]*>`)
	embedTagsRegex   = regexp.MustCompile(`(?is)<\s*(iframe|embed|object|applet|meta|link|base)[^>]*>.*?<\s*/\s*(iframe|embed|object|applet|meta|link|base)\s*>|<\s*(iframe|embed|object|applet|meta|link|base)[^>]*/>|<\s*(iframe|embed|object|applet|meta|link|base)[^>]*>`)
	onEventAttrRegex = regexp.MustCompile(`(?i)\s+on[a-z]+\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)`)
	jsHrefAttrRegex  = regexp.MustCompile(`(?i)(href|xlink:href|src)\s*=\s*["']\s*(?:javascript|vbscript|data:text/html|data:application/javascript):[^"']*["']`)
	doctypeEntityReg = regexp.MustCompile(`(?is)<!ENTITY[^>]*>|<!DOCTYPE[^>]*\[.*?\]>`)
	jsProtocolSearch = regexp.MustCompile(`(?i)(?:javascript|vbscript)\s*:`)
)

// DetectMimeType detects whether the file is PNG, WebP, or SVG.
func DetectMimeType(data []byte) (string, error) {
	if len(data) == 0 {
		return "", ErrEmptyFile
	}
	if len(data) > MaxLogoSizeBytes {
		return "", ErrFileTooLarge
	}

	// 1. Check PNG magic bytes
	if len(data) >= len(pngMagic) && bytes.Equal(data[:len(pngMagic)], pngMagic) {
		return MIMETypePNG, nil
	}

	// 2. Check WebP magic bytes: starts with RIFF and at offset 8 has WEBP
	if len(data) >= 12 && bytes.Equal(data[:4], riffMagic) && bytes.Equal(data[8:12], webpMagic) {
		return MIMETypeWebP, nil
	}

	// 3. Check SVG (text XML with <svg)
	if isSVGContent(data) {
		return MIMETypeSVG, nil
	}

	return "", ErrUnsupportedMimeType
}

func isSVGContent(data []byte) bool {
	// Look at the first 4KB
	limit := len(data)
	if limit > 4096 {
		limit = 4096
	}
	sample := strings.ToLower(string(data[:limit]))

	// Quick rejection of HTML or PHP
	if strings.Contains(sample, "<html") || strings.Contains(sample, "<?php") {
		return false
	}

	// Check if contains <svg
	if strings.Contains(sample, "<svg") {
		return true
	}

	return false
}

// CheckMaliciousSVG inspects SVG data for XSS payloads (scripts, event handlers, javascript: URIs).
// Returns ErrMaliciousSVG if any dangerous payload is detected.
func CheckMaliciousSVG(data []byte) error {
	str := string(data)

	// Check script tags
	if scriptTagRegex.MatchString(str) {
		return ErrMaliciousSVG
	}

	// Check foreignObject, iframe, embed, object
	if foreignObjRegex.MatchString(str) || embedTagsRegex.MatchString(str) {
		return ErrMaliciousSVG
	}

	// Check event handlers like onload, onerror, onclick
	if onEventAttrRegex.MatchString(str) {
		return ErrMaliciousSVG
	}

	// Check javascript: or vbscript: links
	if jsHrefAttrRegex.MatchString(str) || jsProtocolSearch.MatchString(str) {
		return ErrMaliciousSVG
	}

	// Check external entities
	if doctypeEntityReg.MatchString(str) {
		return ErrMaliciousSVG
	}

	return nil
}

// SanitizeSVG strips all scripts, event handlers, and dangerous attributes/tags from SVG content.
func SanitizeSVG(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrEmptyFile
	}
	if len(data) > MaxLogoSizeBytes {
		return nil, ErrFileTooLarge
	}

	content := string(data)

	// Strip DOCTYPE entity expansions
	content = doctypeEntityReg.ReplaceAllString(content, "")

	// Strip script tags
	content = scriptTagRegex.ReplaceAllString(content, "")

	// Strip foreignObject
	content = foreignObjRegex.ReplaceAllString(content, "")

	// Strip dangerous embedding tags
	content = embedTagsRegex.ReplaceAllString(content, "")

	// Strip on* event handlers
	content = onEventAttrRegex.ReplaceAllString(content, "")

	// Strip javascript: and vbscript: links
	content = jsHrefAttrRegex.ReplaceAllString(content, "")

	// Clean out any lingering raw javascript: URI references
	content = jsProtocolSearch.ReplaceAllString(content, "blocked:")

	sanitized := []byte(strings.TrimSpace(content))
	if !isSVGContent(sanitized) {
		return nil, ErrInvalidSVG
	}

	return sanitized, nil
}

// LogoStorage manages safe filesystem storage of sponsor logos in an isolated directory.
type LogoStorage struct {
	baseDir string
}

func NewLogoStorage(baseDir string) (*LogoStorage, error) {
	if baseDir == "" {
		baseDir = filepath.Join("data", "sponsors", "logos")
	}
	absDir, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("resolve logo storage dir: %w", err)
	}
	if err := os.MkdirAll(absDir, 0755); err != nil {
		return nil, fmt.Errorf("create logo storage dir: %w", err)
	}
	return &LogoStorage{baseDir: absDir}, nil
}

func (s *LogoStorage) BaseDir() string {
	return s.baseDir
}

// SaveLogo validates MIME, rejects dangerous files, sanitizes SVGs, and stores the file.
// Returns relative public path, MIME type, and error.
func (s *LogoStorage) SaveLogo(data []byte, filename string) (string, string, error) {
	if len(data) == 0 {
		return "", "", ErrEmptyFile
	}
	if len(data) > MaxLogoSizeBytes {
		return "", "", ErrFileTooLarge
	}

	mimeType, err := DetectMimeType(data)
	if err != nil {
		return "", "", err
	}

	var toSave []byte
	var ext string

	switch mimeType {
	case MIMETypePNG:
		ext = ".png"
		toSave = data
	case MIMETypeWebP:
		ext = ".webp"
		toSave = data
	case MIMETypeSVG:
		ext = ".svg"
		// Security Check: If dangerous script/XSS is detected, reject it with ErrMaliciousSVG
		if err := CheckMaliciousSVG(data); err != nil {
			return "", "", err
		}
		// Additional sanitization pass for safety
		sanitized, err := SanitizeSVG(data)
		if err != nil {
			return "", "", fmt.Errorf("sanitize svg: %w", err)
		}
		toSave = sanitized
	default:
		return "", "", ErrUnsupportedMimeType
	}

	// Generate safe, content-addressed filename
	hasher := sha256.New()
	hasher.Write(toSave)
	hashHex := hex.EncodeToString(hasher.Sum(nil))[:16]
	safeFilename := fmt.Sprintf("logo_%s%s", hashHex, ext)

	targetPath := filepath.Join(s.baseDir, safeFilename)
	if err := os.WriteFile(targetPath, toSave, 0644); err != nil {
		return "", "", fmt.Errorf("write logo file: %w", err)
	}

	// Relative path for database and public URL
	relPath := filepath.Join("data", "sponsors", "logos", safeFilename)
	return relPath, mimeType, nil
}

// ReadLogo reads a logo from storage.
func (s *LogoStorage) ReadLogo(relPath string) ([]byte, string, error) {
	cleanPath := filepath.Clean(relPath)
	filename := filepath.Base(cleanPath)
	fullPath := filepath.Join(s.baseDir, filename)

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, "", fmt.Errorf("read logo: %w", err)
	}

	mimeType, _ := DetectMimeType(data)
	return data, mimeType, nil
}

// Ensure interface / helper
var _ io.Reader
