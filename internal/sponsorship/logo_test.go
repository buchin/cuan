package sponsorship

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectMimeType(t *testing.T) {
	// PNG
	validPNG := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
	mime, err := DetectMimeType(validPNG)
	if err != nil || mime != MIMETypePNG {
		t.Fatalf("expected PNG, got %s, err: %v", mime, err)
	}

	// WebP
	validWebP := []byte{'R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'E', 'B', 'P', 'V', 'P', '8'}
	mime, err = DetectMimeType(validWebP)
	if err != nil || mime != MIMETypeWebP {
		t.Fatalf("expected WebP, got %s, err: %v", mime, err)
	}

	// SVG
	validSVG := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><circle cx="50" cy="50" r="40" /></svg>`)
	mime, err = DetectMimeType(validSVG)
	if err != nil || mime != MIMETypeSVG {
		t.Fatalf("expected SVG, got %s, err: %v", mime, err)
	}

	// Disallowed: JPEG
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	_, err = DetectMimeType(jpegHeader)
	if err != ErrUnsupportedMimeType {
		t.Fatalf("expected ErrUnsupportedMimeType for JPEG, got %v", err)
	}

	// Disallowed: Executable ELF / Script / HTML
	htmlContent := []byte(`<!DOCTYPE html><html><body><h1>Disallowed</h1></body></html>`)
	_, err = DetectMimeType(htmlContent)
	if err != ErrUnsupportedMimeType {
		t.Fatalf("expected ErrUnsupportedMimeType for HTML, got %v", err)
	}

	// Empty file
	_, err = DetectMimeType([]byte{})
	if err != ErrEmptyFile {
		t.Fatalf("expected ErrEmptyFile, got %v", err)
	}

	// Too large file (> 2MB)
	largeData := make([]byte, MaxLogoSizeBytes+1)
	copy(largeData, validPNG)
	_, err = DetectMimeType(largeData)
	if err != ErrFileTooLarge {
		t.Fatalf("expected ErrFileTooLarge, got %v", err)
	}
}

func TestCheckMaliciousSVG(t *testing.T) {
	tests := []struct {
		name      string
		svg       string
		malicious bool
	}{
		{
			name:      "Clean SVG",
			svg:       `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><rect width="100" height="100" fill="blue"/></svg>`,
			malicious: false,
		},
		{
			name:      "SVG with script tag",
			svg:       `<svg xmlns="http://www.w3.org/2000/svg"><script>alert('XSS')</script><circle cx="10" cy="10" r="5"/></svg>`,
			malicious: true,
		},
		{
			name:      "SVG with script CDATA",
			svg:       `<svg><script type="text/javascript"><![CDATA[window.location='http://attacker.com';]]></script></svg>`,
			malicious: true,
		},
		{
			name:      "SVG with onload event",
			svg:       `<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"><circle cx="10" cy="10" r="5"/></svg>`,
			malicious: true,
		},
		{
			name:      "SVG with onerror on image",
			svg:       `<svg><image href="invalid.png" onerror="alert('xss')"/></svg>`,
			malicious: true,
		},
		{
			name:      "SVG with javascript link",
			svg:       `<svg><a href="javascript:alert(1)"><text>Click</text></a></svg>`,
			malicious: true,
		},
		{
			name:      "SVG with xlink:href javascript",
			svg:       `<svg xmlns:xlink="http://www.w3.org/1999/xlink"><a xlink:href="javascript:steal()"><rect width="10" height="10"/></a></svg>`,
			malicious: true,
		},
		{
			name:      "SVG with foreignObject",
			svg:       `<svg><foreignObject width="100" height="100"><body xmlns="http://www.w3.org/1999/xhtml"><script>alert(1)</script></body></foreignObject></svg>`,
			malicious: true,
		},
		{
			name:      "SVG with XML entity expansion",
			svg:       `<?xml version="1.0"?><!DOCTYPE svg [<!ENTITY xxe SYSTEM "file:///etc/passwd">]><svg>&xxe;</svg>`,
			malicious: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckMaliciousSVG([]byte(tc.svg))
			if tc.malicious && err == nil {
				t.Fatalf("expected malicious SVG to be rejected, but CheckMaliciousSVG passed: %s", tc.svg)
			}
			if !tc.malicious && err != nil {
				t.Fatalf("expected clean SVG to pass, but got: %v", err)
			}
		})
	}
}

func TestSanitizeSVG(t *testing.T) {
	input := `<svg xmlns="http://www.w3.org/2000/svg" onload="alert('hack')">
		<script>evilCode();</script>
		<a href="javascript:doEvil()">
			<circle cx="50" cy="50" r="40" fill="green" onclick="alert(1)"/>
		</a>
		<foreignObject><div>bypass</div></foreignObject>
	</svg>`

	sanitized, err := SanitizeSVG([]byte(input))
	if err != nil {
		t.Fatalf("SanitizeSVG returned error: %v", err)
	}

	sanStr := string(sanitized)
	if strings.Contains(sanStr, "<script") {
		t.Errorf("sanitized SVG still contains <script: %s", sanStr)
	}
	if strings.Contains(sanStr, "onload") {
		t.Errorf("sanitized SVG still contains onload: %s", sanStr)
	}
	if strings.Contains(sanStr, "onclick") {
		t.Errorf("sanitized SVG still contains onclick: %s", sanStr)
	}
	if strings.Contains(sanStr, "javascript:") {
		t.Errorf("sanitized SVG still contains javascript: URL: %s", sanStr)
	}
	if strings.Contains(sanStr, "foreignObject") {
		t.Errorf("sanitized SVG still contains foreignObject: %s", sanStr)
	}

	// Must retain valid svg
	if !strings.Contains(sanStr, "<svg") || !strings.Contains(sanStr, "<circle") {
		t.Errorf("sanitized SVG lost valid graphic content: %s", sanStr)
	}
}

func TestLogoStorageSaveAndRead(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cuan-logo-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage, err := NewLogoStorage(tempDir)
	if err != nil {
		t.Fatalf("failed to init LogoStorage: %v", err)
	}

	// 1. Save PNG
	validPNG := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x01, 0x02, 0x03}
	relPath, mime, err := storage.SaveLogo(validPNG, "my_brand.png")
	if err != nil {
		t.Fatalf("failed to save PNG: %v", err)
	}
	if mime != MIMETypePNG {
		t.Fatalf("expected MIME image/png, got %s", mime)
	}
	if !strings.HasSuffix(relPath, ".png") {
		t.Fatalf("expected .png extension, got %s", relPath)
	}

	// Read back
	readBytes, readMime, err := storage.ReadLogo(relPath)
	if err != nil {
		t.Fatalf("failed to read back saved logo: %v", err)
	}
	if !bytes.Equal(readBytes, validPNG) || readMime != MIMETypePNG {
		t.Fatalf("read back content mismatch")
	}

	// 2. Save Safe SVG
	cleanSVG := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="50" height="50"><rect width="50" height="50"/></svg>`)
	svgRelPath, svgMime, err := storage.SaveLogo(cleanSVG, "logo.svg")
	if err != nil {
		t.Fatalf("failed to save clean SVG: %v", err)
	}
	if svgMime != MIMETypeSVG {
		t.Fatalf("expected SVG mime, got %s", svgMime)
	}
	if !strings.HasSuffix(svgRelPath, ".svg") {
		t.Fatalf("expected .svg extension, got %s", svgRelPath)
	}

	// 3. Reject Malicious SVG (DoD: Upload file berbahaya ditolak dengan aman)
	badSVG := []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert('XSS')"><text>Dangerous</text></svg>`)
	_, _, err = storage.SaveLogo(badSVG, "dangerous.svg")
	if err != ErrMaliciousSVG {
		t.Fatalf("expected ErrMaliciousSVG rejection, got: %v", err)
	}

	// 4. Reject Non-Allowed Mime (e.g. PHP script pretending to be image)
	phpPayload := []byte(`<?php system($_GET['cmd']); ?>`)
	_, _, err = storage.SaveLogo(phpPayload, "shell.php.png")
	if err != ErrUnsupportedMimeType {
		t.Fatalf("expected ErrUnsupportedMimeType, got: %v", err)
	}

	// 5. Check no path traversal
	_, _, err = storage.ReadLogo("../../../etc/passwd")
	// Since ReadLogo cleans and takes filepath.Base, it will look for "passwd" in storage dir and fail gracefully
	if err == nil {
		t.Fatalf("expected error reading non-existent file, got nil")
	}
	fullCheck := filepath.Join(tempDir, "passwd")
	if _, err := os.Stat(fullCheck); !os.IsNotExist(err) {
		t.Fatalf("directory traversal attempt leaked into file operation")
	}
}
