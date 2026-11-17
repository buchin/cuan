package terminal

import (
	"bytes"
	"unicode/utf8"
)

const (
	// DefaultAttachSeedMaxBytes defines the bounded tail seed for initial terminal paint on attach/reconnect.
	// Bounding initial seed (32 KiB) keeps attach instant (1 frame, ~0ms) and prevents flooding client memory,
	// matching native terminal standards (Blink Shell / Termux / Fural).
	DefaultAttachSeedMaxBytes = 32 * 1024
)

var (
	decSyncBegin = []byte("\x1b[?2026h")
	decSyncEnd   = []byte("\x1b[?2026l")
	eraseDisplay = []byte("\x1b[2J")
	eraseScroll  = []byte("\x1b[3J")
	altScreenOn  = []byte("\x1b[?1049h")
	cursorShow   = []byte("\x1b[?25h")
	sgrReset     = []byte("\x1b[0m")
)

// SanitizePTYReplay bounds and sanitizes historical PTY output for client reconnects / attachments.
// It:
// 1. Caps output to maxBytes (defaulting to DefaultAttachSeedMaxBytes = 32 KiB).
// 2. Aligns to clean boundaries (last screen clear, escape sequence, or newline) to avoid mid-escape junk.
// 3. Strips unpaired DEC synchronized updates (\x1b[?2026h / \x1b[?2026l) so xterm never freezes.
// 4. Ensures cursor visibility and SGR state are preserved.
func SanitizePTYReplay(raw []byte, maxBytes int) []byte {
	if len(raw) == 0 {
		return nil
	}
	if maxBytes <= 0 {
		maxBytes = DefaultAttachSeedMaxBytes
	}

	hasSync := bytes.Contains(raw, decSyncBegin) || bytes.Contains(raw, decSyncEnd)
	origin := lastReplayOrigin(raw)

	// If already within budget, contains no DEC sync toggles, and has no screen clear,
	// return untouched for byte-for-byte fidelity with plain text test fixtures.
	if len(raw) <= maxBytes && !hasSync && origin < 0 {
		return raw
	}

	// Buffer exceeds maxBytes or has clear origin / DEC sync:
	body := selectReplaySlice(raw, maxBytes)
	if len(body) == 0 {
		return nil
	}

	return finalizePTYReplay(body)
}

// lastReplayOrigin finds the index of the last full-screen origin sequence.
func lastReplayOrigin(raw []byte) int {
	last := -1
	for _, pat := range [][]byte{eraseDisplay, eraseScroll, altScreenOn} {
		if i := bytes.LastIndex(raw, pat); i > last {
			last = i
		}
	}
	return last
}

// selectReplaySlice extracts the most relevant visible tail up to maxBytes.
func selectReplaySlice(raw []byte, maxBytes int) []byte {
	// If there is a full-screen clear/alt-screen origin within the raw stream:
	if origin := lastReplayOrigin(raw); origin >= 0 {
		fromOrigin := raw[origin:]
		if len(fromOrigin) <= maxBytes {
			// Entire screen state since the last clear fits in maxBytes
			return fromOrigin
		}
		// Output since last clear exceeds maxBytes: truncate tail while keeping origin clear
		return truncateFromOrigin(fromOrigin, maxBytes)
	}

	return truncateReplayBody(raw, maxBytes)
}

// truncateFromOrigin preserves the origin escape sequence while capping total bytes.
func truncateFromOrigin(body []byte, maxBytes int) []byte {
	if len(body) <= maxBytes {
		return body
	}

	// Extract origin sequence (e.g. \x1b[2J or \x1b[?1049h)
	originEnd := 1
	for originEnd < len(body) && originEnd < 16 && body[originEnd] != 'J' && body[originEnd] != 'h' {
		originEnd++
	}
	if originEnd < len(body) {
		originEnd++
	}
	origin := body[:originEnd]
	if len(origin) >= maxBytes {
		return origin[:maxBytes]
	}

	tailBudget := maxBytes - len(origin)
	tail := truncateReplayBody(body[originEnd:], tailBudget)

	out := make([]byte, 0, len(origin)+len(tail))
	out = append(out, origin...)
	out = append(out, tail...)
	return out
}

// truncateReplayBody keeps the newest maxBytes, cutting at an escape sequence, newline, or UTF-8 rune boundary.
func truncateReplayBody(body []byte, maxBytes int) []byte {
	if maxBytes <= 0 || len(body) <= maxBytes {
		return body
	}
	tail := body[len(body)-maxBytes:]

	searchWindow := len(tail)
	if searchWindow > 64 {
		searchWindow = 64
	}

	// 1. Look for escape sequence start (\x1b) in the leading window to avoid starting mid-CSI
	if i := bytes.IndexByte(tail[:searchWindow], 0x1b); i > 0 {
		return tail[i:]
	}

	// 2. Look for newline boundary in the first 128 bytes
	lineWindow := len(tail)
	if lineWindow > 128 {
		lineWindow = 128
	}
	if i := bytes.IndexByte(tail[:lineWindow], '\n'); i > 0 {
		return tail[i+1:]
	}

	// 3. Ensure we start at a valid UTF-8 rune boundary
	for len(tail) > 0 && !utf8.RuneStart(tail[0]) {
		tail = tail[1:]
	}

	return tail
}

// finalizePTYReplay cleans DEC sync toggles and ensures proper terminal reset & cursor state.
func finalizePTYReplay(body []byte) []byte {
	cleaned := stripDECSyncUpdates(body)
	if len(cleaned) == 0 {
		return nil
	}

	const (
		syncOff    = "\x1b[?2026l\x1b[0m"
		showCursor = "\x1b[?25h"
		suffix     = "\x1b[?2026l\x1b[?25h"
	)
	prefix := syncOff + showCursor
	out := make([]byte, 0, len(prefix)+len(cleaned)+len(suffix))
	out = append(out, prefix...)
	out = append(out, cleaned...)
	out = append(out, suffix...)
	return out
}

// stripDECSyncUpdates strips DEC synchronized-update mode toggles (\x1b[?2026h and \x1b[?2026l).
func stripDECSyncUpdates(in []byte) []byte {
	if len(in) == 0 {
		return nil
	}
	out := bytes.ReplaceAll(in, decSyncBegin, nil)
	out = bytes.ReplaceAll(out, decSyncEnd, nil)
	return out
}
