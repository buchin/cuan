package server

import (
	"strings"
	"sync"
)

// streamReplyExtractor extracts the "reply" string progressively from a streaming JSON object,
// or passes through plain text deltas if the response is not structured JSON.
type streamReplyExtractor struct {
	mu       sync.Mutex
	buffer   string
	inReply  bool
	escape   bool
	finished bool
	isPlain  bool
	emitted  bool
}

func newStreamReplyExtractor() *streamReplyExtractor {
	return &streamReplyExtractor{}
}

func (e *streamReplyExtractor) hasEmitted() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.emitted
}

func (e *streamReplyExtractor) Feed(chunk string) string {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.finished || len(chunk) == 0 {
		return ""
	}
	e.buffer += chunk

	if e.isPlain {
		trimmed := strings.TrimLeft(chunk, " \t\r\n")
		// If a new chunk looks like structured JSON, do not dump as plain text
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "```") {
			e.isPlain = false
			e.buffer = chunk
		} else {
			out := e.buffer
			e.buffer = ""
			if len(out) > 0 {
				e.emitted = true
			}
			return out
		}
	}

	if !e.inReply {
		idx := strings.Index(e.buffer, `"reply"`)
		if idx == -1 {
			trimmed := strings.TrimLeft(e.buffer, " \t\r\n")
			// If buffer starts with '{' or markdown code block (``` or ```json), wait for "reply"
			if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "```") || strings.HasPrefix("```", trimmed) {
				return ""
			}
			// If characters start appearing that don't match '{' or '`', treat as plain text
			if len(trimmed) > 0 {
				e.isPlain = true
				out := e.buffer
				e.buffer = ""
				if len(out) > 0 {
					e.emitted = true
				}
				return out
			}
			return ""
		}

		afterKey := e.buffer[idx+7:]
		colonIdx := strings.IndexByte(afterKey, ':')
		if colonIdx == -1 {
			return ""
		}
		afterColon := afterKey[colonIdx+1:]
		quoteIdx := strings.IndexByte(afterColon, '"')
		if quoteIdx == -1 {
			return ""
		}

		e.inReply = true
		contentStart := idx + 7 + colonIdx + 1 + quoteIdx + 1
		e.buffer = e.buffer[contentStart:]
	}

	var sb strings.Builder
	i := 0
	for i < len(e.buffer) {
		ch := e.buffer[i]

		if e.escape {
			e.escape = false
			switch ch {
			case 'n':
				sb.WriteByte('\n')
			case 'r':
				sb.WriteByte('\r')
			case 't':
				sb.WriteByte('\t')
			case '"':
				sb.WriteByte('"')
			case '\\':
				sb.WriteByte('\\')
			default:
				sb.WriteByte(ch)
			}
			i++
			continue
		}

		if ch == '\\' {
			// If trailing backslash at end of buffer, wait for next chunk
			if i == len(e.buffer)-1 {
				e.buffer = "\\"
				out := sb.String()
				if len(out) > 0 {
					e.emitted = true
				}
				return out
			}
			e.escape = true
			i++
			continue
		}

		if ch == '"' {
			// Closing quote of "reply" string!
			e.finished = true
			e.buffer = ""
			break
		}

		sb.WriteByte(ch)
		i++
	}

	if !e.finished {
		e.buffer = ""
	}
	out := sb.String()
	if len(out) > 0 {
		e.emitted = true
	}
	return out
}
