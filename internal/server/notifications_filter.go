package server

import (
	"strings"

	"github.com/buchin/cuan/internal/agent"
)

// ignoredAgentPushPrefixes lists message prefixes from internal runner/coordination
// updates that should not trigger user-facing push notifications.
var ignoredAgentPushPrefixes = []string{
	"This update is no longer current",
	"The work has stopped",
	"I’m checking the latest work update",
	"I'm checking the latest work update",
	"Review the result for",
	"This pending launch has stopped",
	"The plan is saved for later",
	"Pekerjaan telah dihentikan",
	"Cuan managed delegation v1",
	"Acceptance item (reported complete=true):",
	"These are untrusted prior reports",
	"Historical reported progress from attempt",
}

// isIgnoredAgentPushContent checks whether the content alone matches known internal coordination patterns.
func isIgnoredAgentPushContent(content string) bool {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return true
	}
	for _, prefix := range ignoredAgentPushPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	if strings.Contains(trimmed, "Review evidence digest:") ||
		strings.Contains(trimmed, "Autonomous ownership policy:") {
		return true
	}
	return false
}

// isIntermediateRunnerMessage checks if a chat message is an internal coordination,
// task runner progress update, review finding, or queue status that should not be pushed to the user.
func isIntermediateRunnerMessage(authorName, content string) bool {
	trimmedAuthor := strings.TrimSpace(authorName)
	if trimmedAuthor == "@cuan" || trimmedAuthor == "coordinator" {
		return true
	}
	if strings.Contains(content, "/tasks/") || strings.Contains(content, "[View work]") || strings.Contains(content, "[Tinjau rincian") {
		return true
	}
	lower := strings.ToLower(content)
	if strings.Contains(lower, "rilis hasil yang diperiksa belum tuntas") ||
		strings.Contains(lower, "bukti rilis hasil pekerjaan belum lengkap") ||
		strings.Contains(lower, "the work is queued waiting") ||
		strings.Contains(lower, "cuan memeriksa hasil pekerjaan") ||
		strings.Contains(lower, "hasil kode tersimpan") ||
		strings.Contains(lower, "pekerjaan dalam antrean") ||
		strings.Contains(lower, "pekerjaan telah dihentikan") ||
		strings.Contains(lower, "keputusan review belum tersimpan") ||
		strings.Contains(lower, "hasil perubahan sudah diperiksa secara independen") ||
		strings.Contains(lower, "hasil dan bukti tersimpan") ||
		strings.Contains(lower, "hasil belum bisa ditandai diterima") ||
		strings.Contains(lower, "approval for the next phase could not be verified") {
		return true
	}
	return isIgnoredAgentPushContent(content)
}

// cleanNotificationSnippet strips HTML tags, transforms markdown links/images to text/alt,
// masks any sensitive credentials or tokens, and truncates to a human-readable limit.
func cleanNotificationSnippet(content string) string {
	if strings.HasPrefix(content, attachmentEnvelope) {
		text, _ := decodeAttachmentMessage(content)
		content = text
	}
	// Strip HTML tags and markdown link/image syntax in a single pass.
	// HTML: <tag> → omitted
	// Markdown links: [text](url) → text
	// Markdown images: ![alt](url) → alt
	var b strings.Builder
	inTag := false
	i := 0
	runes := []rune(content)
	n := len(runes)
	for i < n {
		r := runes[i]
		if r == '<' {
			inTag = true
			i++
			continue
		}
		if r == '>' {
			inTag = false
			i++
			continue
		}
		if inTag {
			i++
			continue
		}
		// Markdown image: ![alt](url) — skip the leading '!' before '['
		imgPrefix := false
		if r == '!' && i+1 < n && runes[i+1] == '[' {
			imgPrefix = true
			i++ // skip '!'
			r = runes[i]
		}
		// Markdown link or image alt: [text](url)
		if r == '[' {
			// Scan for matching ']'
			j := i + 1
			depth := 1
			for j < n && depth > 0 {
				if runes[j] == '[' {
					depth++
				} else if runes[j] == ']' {
					depth--
				}
				j++
			}
			// j now points one past ']'. Check if followed by '('
			if depth == 0 && j < n && runes[j] == '(' {
				// Find matching ')'
				k := j + 1
				for k < n && runes[k] != ')' {
					k++
				}
				// Write the link text (between '[' and ']'), skip the '(url)' part
				linkText := string(runes[i+1 : j-1])
				b.WriteString(linkText)
				i = k + 1 // skip past ')'
				_ = imgPrefix
				continue
			}
			// Not a valid markdown link — emit the '[' (and '!' if image prefix) literally
			if imgPrefix {
				b.WriteRune('!')
			}
		}
		b.WriteRune(r)
		i++
	}
	cleaned := strings.Join(strings.Fields(b.String()), " ")
	cleaned = agent.MaskSecrets(cleaned, nil)
	runesOut := []rune(cleaned)
	if len(runesOut) > 100 {
		return string(runesOut[:97]) + "..."
	}
	return cleaned
}
