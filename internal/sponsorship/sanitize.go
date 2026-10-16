package sponsorship

import (
	"html"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
)

var (
	htmlTagRegex       = regexp.MustCompile(`(?is)<[^>]*>`)
	jsSchemeRegex      = regexp.MustCompile(`(?i)^\s*(?:javascript|vbscript|data):`)
	dangerousCharsReg = regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]`)
)

// SanitizeText removes HTML tags, malicious scripts, and control characters,
// returning safe plaintext for brand names, slogans, and remarks.
func SanitizeText(input string) string {
	if input == "" {
		return ""
	}

	// 1. Strip raw control characters
	cleaned := dangerousCharsReg.ReplaceAllString(input, "")

	// 2. Remove all HTML tags completely (e.g. <script>alert(1)</script> -> alert(1))
	cleaned = htmlTagRegex.ReplaceAllString(cleaned, "")

	// 3. Unescape entities if any were double-encoded, then re-strip in case of nested payloads
	cleaned = html.UnescapeString(cleaned)
	cleaned = htmlTagRegex.ReplaceAllString(cleaned, "")

	// 4. Normalize multiple whitespaces
	cleaned = strings.Join(strings.Fields(cleaned), " ")

	return strings.TrimSpace(cleaned)
}

// ValidateAndFormatURL ensures the URL is safe (anti-XSS), uses http/https,
// and normalizes missing scheme to https://.
func ValidateAndFormatURL(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", ErrInvalidWebsiteURL
	}

	// Reject javascript:, data:, vbscript: explicitly
	if jsSchemeRegex.MatchString(trimmed) || strings.Contains(strings.ToLower(trimmed), "javascript:") {
		return "", ErrInvalidWebsiteURL
	}

	// Prepend https:// if protocol is missing
	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		trimmed = "https://" + trimmed
	}

	parsed, err := url.ParseRequestURI(trimmed)
	if err != nil {
		return "", ErrInvalidWebsiteURL
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", ErrInvalidWebsiteURL
	}

	if parsed.Host == "" || strings.Contains(parsed.Host, " ") {
		return "", ErrInvalidWebsiteURL
	}

	return parsed.String(), nil
}

// ValidateContactEmail ensures valid standard email format.
func ValidateContactEmail(email string) error {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return ErrInvalidEmail
	}

	addr, err := mail.ParseAddress(trimmed)
	if err != nil {
		return ErrInvalidEmail
	}

	parts := strings.Split(addr.Address, "@")
	if len(parts) != 2 || !strings.Contains(parts[1], ".") {
		return ErrInvalidEmail
	}

	return nil
}
